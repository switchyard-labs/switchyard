package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"switchyard/internal/actions"
	"testing"
)

func TestActionLogBoundsAndStreamLabels(t *testing.T) {
	lines, truncated := boundedActionLines(actionLogCapture{Stdout: actionLogOutput{Text: strings.Repeat("x", 9000) + "\nsecond\n"}, Stderr: actionLogOutput{Text: "error\n"}})
	if !truncated || len(lines) != 3 || len(lines[0].Text) > 8250 || lines[2].ID != 3 || lines[2].Stream != "stderr" {
		t.Fatalf("unsafe capture pagination: %v %d", truncated, len(lines))
	}
	lines, truncated = boundedActionLines(actionLogCapture{Stdout: actionLogOutput{Text: strings.Repeat("line\n", 10001)}})
	if !truncated || len(lines) != 10000 {
		t.Fatal("line cap not enforced")
	}
}

// The provider is unreachable until canonical repository/run/step grants pass.
type logTestProvider struct {
	actions.Provider
	capture actionLogCapture
	calls   int
}

func (p *logTestProvider) Logs(_ context.Context, _, _ string, out any) error {
	p.calls++
	data, _ := json.Marshal(p.capture)
	return json.Unmarshal(data, out)
}
func TestActionLogScopeSHAAndResume(t *testing.T) {
	a, _, snapshot := actionFixture(t)
	if err := a.syncActionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	_, _, err := a.Trestle.CreateRecord("repository_meta", map[string]any{"id": "meta", "artifact_name": "repo", "full_name": "alice/rail", "owner_type": "user", "owner_id": "alice", "visibility": "public"}, "meta")
	if err != nil {
		t.Fatal(err)
	}
	provider := &logTestProvider{capture: actionLogCapture{SHA: snapshot.Manifest.Run.SHA, Kind: "captured", Stdout: actionLogOutput{Text: "first\nsecond\n"}}}
	a.Actions = provider
	request := func(path, step string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "rail")
		r.SetPathValue("id", "run1")
		if step != "" {
			q := r.URL.Query()
			q.Set("step", step)
			r.URL.RawQuery = q.Encode()
		}
		w := httptest.NewRecorder()
		a.authorizeHandler(a.handleActionLogs)(w, r)
		return w
	}
	w := request("/api/repositories/alice/rail/actions/run1/logs/stream?cursor=1", "verify-test")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"text":"first"`) || !strings.Contains(w.Body.String(), `"text":"second"`) || !strings.Contains(w.Body.String(), "event: complete") {
		t.Fatalf("resume: %d %s", w.Code, w.Body.String())
	}
	before := provider.calls
	if w = request("/api/repositories/alice/rail/actions/run1/logs", "unknown-step"); w.Code != 404 || provider.calls != before {
		t.Fatal("unapproved step reached provider")
	}
	provider.capture.SHA = strings.Repeat("b", 40)
	if w = request("/api/repositories/alice/rail/actions/run1/logs", "verify-test"); w.Code != 502 {
		t.Fatal("wrong SHA exposed")
	}
	provider.capture.SHA = snapshot.Manifest.Run.SHA
	if w = request("/api/repositories/alice/rail/actions/run1/logs?download=1", "verify-test"); w.Header().Get("Content-Disposition") == "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("download not inert")
	}
	r := httptest.NewRequest("GET", "/api/repositories/alice/private/actions/run1/logs", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "private")
	r.SetPathValue("id", "run1")
	before = provider.calls
	w = httptest.NewRecorder()
	a.authorizeHandler(a.handleActionLogs)(w, r)
	if w.Code != 403 || provider.calls != before {
		t.Fatal("unknown repository reached provider")
	}
}
