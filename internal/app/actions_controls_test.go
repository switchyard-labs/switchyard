package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"switchyard/internal/actions"
	"testing"
)

type controlTestProvider struct {
	actions.Provider
	calls     []actions.Run
	fail      bool
	cancelled []string
}

func (p *controlTestProvider) Dispatch(_ context.Context, run actions.Run) error {
	p.calls = append(p.calls, run)
	if p.fail {
		return errors.New("provider temporarily unavailable")
	}
	return nil
}
func (p *controlTestProvider) Cancel(_ context.Context, id string) error {
	p.cancelled = append(p.cancelled, id)
	return nil
}
func TestActionDispatchIntentReplayAndExactRerun(t *testing.T) {
	a, store, snapshot := actionFixture(t)
	_, _, err := a.Trestle.CreateRecord("repository_meta", map[string]any{"id": "meta", "artifact_name": "repo", "full_name": "alice/rail", "owner_type": "user", "owner_id": "alice", "visibility": "public"}, "meta")
	if err != nil {
		t.Fatal(err)
	}
	provider := &controlTestProvider{fail: true}
	a.Actions = provider
	invoke := func(body, parent string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/repositories/alice/rail/actions/dispatch", strings.NewReader(body))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "rail")
		r.SetPathValue("id", parent)
		w := httptest.NewRecorder()
		a.handleActionDispatch(w, r)
		return w
	}
	body := `{"request_id":"fixture-request-0001","sha":"` + snapshot.Manifest.Run.SHA + `","ref":"refs/heads/main"}`
	if w := invoke(body, ""); w.Code != 502 || len(store.records["action_runs"]) != 1 {
		t.Fatalf("intent lost: %d", w.Code)
	}
	provider.fail = false
	if w := invoke(body, ""); w.Code != 202 || len(store.records["action_runs"]) != 1 || len(provider.calls) != 2 || provider.calls[0].ID != provider.calls[1].ID {
		t.Fatalf("retry changed identity: %d", w.Code)
	}
	if w := invoke(strings.Replace(body, snapshot.Manifest.Run.SHA, strings.Repeat("b", 40), 1), ""); w.Code != 409 || len(provider.calls) != 2 {
		t.Fatal("idempotency key reused for different source")
	}
	if a.requiredActionsPassed("repo", snapshot.Manifest.Run.SHA) {
		t.Fatal("queued dispatch passed required checks")
	}
	if err = a.syncActionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if w := invoke(`{"request_id":"fixture-request-0002","mode":"failed"}`, "run1"); w.Code != 409 {
		t.Fatal("successful run offered failed-job rerun")
	}
	snapshot.Manifest.Status = "failed"
	snapshot.Manifest.Jobs[0].Status = "failed"
	snapshot.ExternalStatus.Status = "errored"
	snapshot.ID = "failure1"
	snapshot.Manifest.Run.ID = "failure1"
	if err = a.syncActionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if w := invoke(`{"request_id":"fixture-request-0003","mode":"failed"}`, "failure1"); w.Code != 202 {
		t.Fatalf("failed rerun: %d %s", w.Code, w.Body.String())
	}
	run := provider.calls[len(provider.calls)-1]
	if run.SHA != snapshot.Manifest.Run.SHA || run.RerunOf != "failure1" || len(run.SelectedJobs) != 1 || run.SelectedJobs[0] != "verify" {
		t.Fatal("rerun source or selection changed")
	}
	r := httptest.NewRequest("POST", "/api/repositories/alice/rail/actions/dispatch", strings.NewReader(body))
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "rail")
	w := httptest.NewRecorder()
	before := len(provider.calls)
	a.authorizeHandler(a.handleActionDispatch)(w, r)
	if (w.Code != 403 && w.Code != 401) || len(provider.calls) != before {
		t.Fatal("anonymous dispatch reached provider")
	}
}

func (p *controlTestProvider) Status(_ context.Context, id string, out any) error {
	data, _ := json.Marshal(map[string]any{"id": id, "external_status": map[string]any{"status": "terminated"}, "manifest": nil})
	return json.Unmarshal(data, out)
}
func TestCancelledActionBeforeManifestRecovers(t *testing.T) {
	a, store, snapshot := actionFixture(t)
	a.Actions = &controlTestProvider{}
	run := snapshot.Manifest.Run
	run.ID = "cancel-before-start"
	snapshot.Manifest.Run = run
	snapshot.Manifest.Status = "queued"
	_, _, err := a.Trestle.CreateRecord("action_runs", map[string]any{"id": run.ID, "repo": run.Repo, "source_sha": run.SHA, "ref": run.Ref, "definition_revision": run.DefinitionRevision, "created_at": snapshot.Manifest.StartedAt, "state": map[string]any{"status": "queued", "manifest": snapshot.Manifest, "dispatch_pending": true, "cancel_requested": true}}, "cancel-intent")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.recoverActionIntents(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, _, record, err := a.Trestle.FindRecord("action_runs", filterEq("id", run.ID))
	if err != nil || record["state"].(map[string]any)["status"] != "cancelled" {
		t.Fatal("cancelled workflow stayed queued")
	}
	if len(store.records["action_checks"]) != 1 || store.records["action_checks"][0].values["status"] != "cancelled" {
		t.Fatal("cancelled workflow did not fence checks")
	}
	before := len(a.Actions.(*controlTestProvider).calls)
	if err = a.recoverActionIntents(context.Background()); err != nil || len(a.Actions.(*controlTestProvider).calls) != before {
		t.Fatal("terminal workflow redispatched")
	}
}
