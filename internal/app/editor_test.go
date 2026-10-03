package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"switchyard/internal/agent"
	"switchyard/internal/trestle"
	"sync"
	"testing"
	"time"
)

type draftFixture struct {
	mu          sync.Mutex
	values      map[string]any
	version     int
	patchStatus int
	reads       int
}

func newDraftFixture(t *testing.T) (*App, *draftFixture) {
	t.Helper()
	f := &draftFixture{version: 1, values: map[string]any{"id": "draft", "user": "alice", "repo": "repo", "branch": "main", "path": "file.go", "content": "original", "revision": "1", "base_sha": "base"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/admin/v1/session" {
			if r.Method == "GET" {
				fmt.Fprint(w, `{"csrfToken":"test"}`)
			} else {
				fmt.Fprint(w, `{}`)
			}
			return
		}
		switch r.Method {
		case "GET":
			f.reads++
			json.NewEncoder(w).Encode(map[string]any{"items": []any{map[string]any{"id": "record", "version": f.version, "values": f.values}}})
		case "PATCH":
			if f.patchStatus != 0 {
				w.WriteHeader(f.patchStatus)
				fmt.Fprint(w, `{"error":"injected"}`)
				return
			}
			if r.Header.Get("If-Match") != strconv.Itoa(f.version) {
				w.WriteHeader(409)
				fmt.Fprint(w, `{"error":"stale"}`)
				return
			}
			var in struct {
				Values map[string]any `json:"values"`
			}
			json.NewDecoder(r.Body).Decode(&in)
			for k, v := range in.Values {
				f.values[k] = v
			}
			f.version++
			fmt.Fprint(w, `{}`)
		case "POST":
			fmt.Fprint(w, `{"id":"execution-record","version":1}`)
		default:
			w.WriteHeader(500)
		}
	}))
	t.Cleanup(srv.Close)
	return &App{Trestle: trestle.New(srv.URL, "test", "test")}, f
}
func saveTestDraft(a *App, user, revision, content string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]any{"repo": "repo", "branch": "main", "path": "file.go", "content": content, "expected_revision": revision})
	r := httptest.NewRequest("POST", "/api/drafts", strings.NewReader(string(body)))
	r = r.WithContext(contextWithUser(r.Context(), user))
	w := httptest.NewRecorder()
	a.handleSaveDraft(w, r)
	return w
}
func TestDraftSaveRequiresRevisionAndOwner(t *testing.T) {
	a, f := newDraftFixture(t)
	for _, rev := range []string{"", "0", "2"} {
		if w := saveTestDraft(a, "alice", rev, "new"); w.Code != 409 {
			t.Fatal(rev, w.Code, w.Body.String())
		}
	}
	if w := saveTestDraft(a, "bob", "1", "new"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if f.values["content"] != "original" {
		t.Fatal("lost content")
	}
}
func TestDraftSaveUsesOneSnapshotAndReportsPatchFailure(t *testing.T) {
	a, f := newDraftFixture(t)
	f.patchStatus = 500
	w := saveTestDraft(a, "alice", "1", "new")
	if w.Code != 502 || f.values["content"] != "original" || f.reads != 1 {
		t.Fatal(w.Code, f.reads, f.values)
	}
	f.patchStatus = 0
	f.reads = 0
	w = saveTestDraft(a, "alice", "1", "new")
	if w.Code != 200 || f.values["revision"] != "2" || f.reads != 1 {
		t.Fatal(w.Code, f.reads, f.values)
	}
}
func TestTwoSessionsOnlyOneRevisionWins(t *testing.T) {
	a, f := newDraftFixture(t)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for _, content := range []string{"human", "agent"} {
		wg.Add(1)
		go func(c string) { defer wg.Done(); statuses <- saveTestDraft(a, "alice", "1", c).Code }(content)
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for code := range statuses {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 || f.values["revision"] != "2" {
		t.Fatal(counts, f.values)
	}
}
func TestCommitCleanupPreservesConcurrentSave(t *testing.T) {
	a, f := newDraftFixture(t)
	if saveTestDraft(a, "alice", "1", "newer").Code != 200 {
		t.Fatal("save")
	}
	if a.finishDraftCommit("record", "1", 1, "new-head") == nil {
		t.Fatal("stale cleanup succeeded")
	}
	if f.values["content"] != "newer" || f.values["revision"] != "2" || f.values["base_sha"] != "base" {
		t.Fatal("overwrote newer draft", f.values)
	}
}
func TestCommitCleanupFailureRetainsContent(t *testing.T) {
	a, f := newDraftFixture(t)
	f.patchStatus = 500
	if a.finishDraftCommit("record", "1", 1, "head") == nil {
		t.Fatal("false success")
	}
	if f.values["content"] != "original" {
		t.Fatal("lost content")
	}
	f.patchStatus = 0
	if err := a.finishDraftCommit("record", "1", 1, "head"); err != nil {
		t.Fatal(err)
	}
	if f.values["content"] != "original" || f.values["base_sha"] != "head" || f.values["revision"] != "2" {
		t.Fatal(f.values)
	}
}
func TestCommitRejectsAnotherOwnerBeforeGit(t *testing.T) {
	a, _ := newDraftFixture(t)
	r := httptest.NewRequest("POST", "/api/drafts/draft/commit", strings.NewReader(`{"expected_revision":"1"}`))
	r.SetPathValue("id", "draft")
	r = r.WithContext(contextWithUser(r.Context(), "bob"))
	w := httptest.NewRecorder()
	a.handleCommitDraft(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
}

type delayedDraftRunner struct{ started, release chan struct{} }

func (d delayedDraftRunner) Run(ctx context.Context, ex *agent.Execution, task agent.Task) error {
	close(d.started)
	select {
	case <-d.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	ex.Status = "succeeded"
	ex.Result = map[string]string{task.File: task.Current + "agent change"}
	return nil
}
func TestInteractiveProposalCannotOverwriteNewerHumanSave(t *testing.T) {
	a, f := newDraftFixture(t)
	runner := delayedDraftRunner{make(chan struct{}), make(chan struct{})}
	a.Runner = runner
	request := httptest.NewRequest("POST", "/api/drafts/draft/agent-propose", strings.NewReader(`{"prompt":"propose a change","expected_revision":"1"}`))
	request.SetPathValue("id", "draft")
	request = request.WithContext(contextWithUser(request.Context(), "alice"))
	result := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { a.handleAgentProposeDraft(result, request); close(done) }()
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		t.Fatal("proposal runner did not start")
	}
	if saved := saveTestDraft(a, "alice", "1", "new human draft"); saved.Code != 200 {
		t.Fatalf("human save failed: %d", saved.Code)
	}
	close(runner.release)
	<-done
	if result.Code != 409 || f.values["content"] != "new human draft" || f.values["revision"] != "2" {
		t.Fatalf("proposal overwrote newer draft: %d %+v", result.Code, f.values)
	}
}

func TestCommittedDraftReloadRetainsRevisionForNextSave(t *testing.T) {
	a, f := newDraftFixture(t)
	f.values["committed_at"] = "2026-10-03T00:00:00Z"
	f.values["revision"] = "2"
	f.values["base_sha"] = "committed-head"
	r := httptest.NewRequest("GET", "/api/drafts/repo/main/file.go", nil)
	r.SetPathValue("repo", "repo")
	r.SetPathValue("branch", "main")
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w := httptest.NewRecorder()
	a.handleGetDraft(w, r)
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result["committed"] != true || result["revision"] != "2" {
		t.Fatalf("reload: %d %s", w.Code, w.Body.String())
	}
	body := `{"repo":"repo","branch":"main","path":"file.go","content":"next edit","expected_revision":"2","base_sha":"new-head"}`
	r = httptest.NewRequest("POST", "/api/drafts", strings.NewReader(body))
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w = httptest.NewRecorder()
	a.handleSaveDraft(w, r)
	if w.Code != 200 || f.values["content"] != "next edit" || f.values["base_sha"] != "new-head" || f.values["committed_at"] != "" {
		t.Fatalf("next save: %d %s", w.Code, w.Body.String())
	}
}
