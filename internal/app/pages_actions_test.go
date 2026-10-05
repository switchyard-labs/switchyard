package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"switchyard/internal/actions"
	"sync"
	"testing"
)

type pagesTestProvider struct {
	actions.Provider
	mu         sync.Mutex
	calls      int
	promotions int
	fail       bool
}

func (p *pagesTestProvider) PublishPages(_ context.Context, in map[string]any) (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.fail {
		return nil, errors.New("publication timeout")
	}
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	out["artifact_sha256"] = strings.Repeat("b", 64)
	out["manifest_hash"] = strings.Repeat("c", 64)
	return out, nil
}
func (p *pagesTestProvider) PagesMapping(context.Context, string) (map[string]any, error) {
	return nil, nil
}
func (p *pagesTestProvider) PromotePages(_ context.Context, input map[string]any) (map[string]any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.promotions++
	if p.fail {
		return nil, errors.New("promotion conflict or timeout")
	}
	return map[string]any{"generation": 1, "projects": map[string]any{strOf(input["project"]): input["target"]}}, nil
}
func pagesActionFixture(t *testing.T) (*App, *queueStore, *actions.Manifest, *pagesTestProvider) {
	a, store, s := actionFixture(t)
	store.enforceUnique = true
	config := PagesConfig{RepositoryID: "meta", Owner: "alice", Project: "docs", Ref: "main", WorkingDirectory: ".", BuildCommand: "npm run build", OutputDirectory: "public"}
	for collection, values := range map[string]map[string]any{
		"repository_meta": {"id": "meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "full_name": "alice/docs"},
		"pages_sites":     {"id": "site_fixture1", "repository_id": "meta", "owner": "alice", "project": "docs", "enabled": "true", "approved_by": "alice", "config": config, "job_id": "pages-project"},
	} {
		if _, _, err := a.Trestle.CreateRecord(collection, values, collection); err != nil {
			t.Fatal(err)
		}
	}
	s.Manifest.Run.Jobs = []actions.Job{pagesBuildJob(config, "project")}
	s.Manifest.Run.Actor = "alice"
	s.Manifest.Jobs = []actions.JobState{{ID: "pages-project", Status: "succeeded", Static: &actions.BuildAssetReceipt{Name: "pages-static.bundle.json", SourceSHA: s.Manifest.Run.SHA, JobID: "pages-project", SHA256: strings.Repeat("b", 64)}}}
	provider := &pagesTestProvider{}
	a.Actions = provider
	return a, store, s.Manifest, provider
}
func TestPagesCompletionReplayAndFrozenConfig(t *testing.T) {
	a, store, m, p := pagesActionFixture(t)
	if err := a.syncActionPages(context.Background(), m, "queued"); err != nil {
		t.Fatal(err)
	}
	rid, v, site, err := a.Trestle.FindRecord("pages_sites", filterEq("id", "site_fixture1"))
	if err != nil {
		t.Fatal(err)
	}
	var changed PagesConfig
	if decodeAction(site["config"], &changed) != nil {
		t.Fatal("config")
	}
	changed.BuildCommand = "new build"
	if err = a.Trestle.PatchRecord("pages_sites", rid, v, map[string]any{"config": changed}); err != nil {
		t.Fatal(err)
	}
	p.fail = true
	if a.syncActionPages(context.Background(), m, "success") == nil {
		t.Fatal("timeout ignored")
	}
	p.fail = false
	if err = a.syncActionPages(context.Background(), m, "success"); err != nil {
		t.Fatal(err)
	}
	if err = a.syncActionPages(context.Background(), m, "success"); err != nil {
		t.Fatal(err)
	}
	if len(store.records["pages_deployments"]) != 1 || p.calls != 2 {
		t.Fatal("duplicate effects", p.calls)
	}
	record := store.records["pages_deployments"][0].values
	state := record["state"].(map[string]any)
	if state["status"] != "ready" || record["source_sha"] != m.Run.SHA {
		t.Fatal(record)
	}
}
func TestPagesFailureCancellationAndRevokedApproval(t *testing.T) {
	for _, status := range []string{"failure", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			a, store, m, p := pagesActionFixture(t)
			if err := a.syncActionPages(context.Background(), m, status); err != nil {
				t.Fatal(err)
			}
			if p.calls != 0 || len(store.records["pages_deployments"]) != 1 {
				t.Fatal("failure published")
			}
			if err := a.syncActionPages(context.Background(), m, "success"); err != nil {
				t.Fatal(err)
			}
			if p.calls != 0 {
				t.Fatal("terminal status regressed")
			}
		})
	}
	a, _, m, p := pagesActionFixture(t)
	rid, v, _, _ := a.Trestle.FindRecord("pages_sites", filterEq("id", "site_fixture1"))
	if err := a.Trestle.PatchRecord("pages_sites", rid, v, map[string]any{"approved_by": "mallory"}); err != nil {
		t.Fatal(err)
	}
	if a.syncActionPages(context.Background(), m, "success") == nil || p.calls != 0 {
		t.Fatal("revoked approver published")
	}
}

func (p *pagesTestProvider) PutDefinition(context.Context, string, actions.Definition) error {
	return nil
}
func (p *pagesTestProvider) Dispatch(context.Context, actions.Run) error { return nil }

func TestPagesConfigPermissionCASAndOwnerReservation(t *testing.T) {
	a, store, _, _ := pagesActionFixture(t)
	store.mu.Lock()
	store.records["pages_sites"] = nil
	store.mu.Unlock()
	call := func(actor, kind, repo, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/config?kind="+kind, strings.NewReader(body))
		r = r.WithContext(contextWithUser(r.Context(), actor))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", repo)
		w := httptest.NewRecorder()
		a.handlePagesConfig(w, r)
		return w
	}
	body := `{"enabled":true,"config":{"ref":"main","working_directory":".","build_command":"npm run build","output_directory":"public"}}`
	if got := call("mallory", "project", "docs", body); got.Code != 403 {
		t.Fatal(got.Code, got.Body.String())
	}
	first := call("alice", "owner", "docs", body)
	if first.Code != 200 {
		t.Fatal(first.Code, first.Body.String())
	}
	if got := call("alice", "owner", "docs", body); got.Code != 409 {
		t.Fatal("stale config accepted", got.Code)
	}
	if _, _, err := a.Trestle.CreateRecord("repository_meta", map[string]any{"id": "meta-other", "artifact_name": "other", "owner_type": "user", "owner_id": "alice", "visibility": "public", "full_name": "alice/other"}, "other"); err != nil {
		t.Fatal(err)
	}
	if got := call("alice", "owner", "other", body); got.Code != 409 || !strings.Contains(got.Body.String(), "pages_prefix_reserved") {
		t.Fatal("root taken over", got.Code, got.Body.String())
	}
	if len(store.records["pages_mounts"]) != 1 {
		t.Fatal("duplicate mount")
	}
}

func TestAppSessionCookieDoesNotAuthorizePagesHosts(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{"users": {{"username": "alice", "password_hash": "fixture"}}})
	r := httptest.NewRequest("POST", "https://app.switchyard.cx/login", nil)
	w := httptest.NewRecorder()
	if err := a.startSession(w, r, "alice"); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Domain != "" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != "/" {
		t.Fatal("app cookie scope changed", cookies)
	}
}

func TestPagesConcurrentCompletionConverges(t *testing.T) {
	a, store, m, p := pagesActionFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = a.syncActionPages(context.Background(), m, "success") }()
	}
	wg.Wait()
	if err := a.syncActionPages(context.Background(), m, "success"); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.records["pages_deployments"]) != 1 || len(store.records["events"]) != 2 {
		t.Fatal("duplicate projection/events", len(store.records["pages_deployments"]), len(store.records["events"]))
	}
	state := store.records["pages_deployments"][0].values["state"].(map[string]any)
	if state["status"] != "ready" {
		t.Fatal(state)
	}
	if p.calls < 1 {
		t.Fatal("not published")
	}
}

func TestPagesCompletionUsesHistoricalApproval(t *testing.T) {
	a, store, m, _ := pagesActionFixture(t)
	rid, version, site, err := a.Trestle.FindRecord("pages_sites", filterEq("id", "site_fixture1"))
	if err != nil {
		t.Fatal(err)
	}
	var original PagesConfig
	if decodeAction(site["config"], &original) != nil {
		t.Fatal("config")
	}
	changed := original
	changed.BuildCommand = "new command"
	history := map[string]any{m.Run.DefinitionRevision: map[string]any{"config": original, "approved_by": "alice"}}
	if err = a.Trestle.PatchRecord("pages_sites", rid, version, map[string]any{"config": changed, "definition_revision": "new-revision", "approvals": history}); err != nil {
		t.Fatal(err)
	}
	if err = a.syncActionPages(context.Background(), m, "success"); err != nil {
		t.Fatal(err)
	}
	if len(store.records["pages_deployments"]) != 1 {
		t.Fatal("in-flight completion lost after config edit")
	}
	var frozen PagesConfig
	if decodeAction(store.records["pages_deployments"][0].values["config"], &frozen) != nil || frozen.BuildCommand != original.BuildCommand {
		t.Fatal("completion used new config")
	}
}

func TestPagesPromotionPermissionsFailuresAndProvenance(t *testing.T) {
	a, store, m, p := pagesActionFixture(t)
	if err := a.syncActionPages(context.Background(), m, "success"); err != nil {
		t.Fatal(err)
	}
	id := strOf(store.records["pages_deployments"][0].values["id"])
	promote := func(user, action string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/promotion", strings.NewReader(`{"generation":0,"operation_id":"promotion-fixture-`+action+`"}`))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "docs")
		r.SetPathValue("id", id)
		r.SetPathValue("action", action)
		r = r.WithContext(contextWithUser(r.Context(), user))
		w := httptest.NewRecorder()
		a.handlePagesPromote(w, r)
		return w
	}
	if w := promote("bob", "promote"); w.Code != 403 || p.promotions != 0 {
		t.Fatal("reader publication", w.Code)
	}
	if w := promote("alice", "delete"); w.Code != 404 || p.promotions != 0 {
		t.Fatal("unknown action accepted", w.Code)
	}
	p.fail = true
	if w := promote("alice", "promote"); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(store.records["events"]) != 2 {
		t.Fatal("failed promotion created audit success")
	}
	p.fail = false
	for _, action := range []string{"promote", "rollback"} {
		w := promote("alice", action)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result map[string]any
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result["source_sha"] != m.Run.SHA {
			t.Fatal("lost exact source", w.Body.String())
		}
	}
	if store.records["pages_deployments"][0].values["state"].(map[string]any)["status"] != "ready" {
		t.Fatal("promotion mutated immutable deployment")
	}
	if len(store.records["events"]) != 4 {
		t.Fatal("same owner operation created duplicate audit", len(store.records["events"]))
	}
	store.rejectEventReplay = true
	for _, action := range []string{"promote", "rollback"} {
		if w := promote("alice", action); w.Code != 200 {
			t.Fatal("replay did not recover its existing audit", w.Code, w.Body.String())
		}
	}
	if len(store.records["events"]) != 4 {
		t.Fatal("replay duplicated audit")
	}
}
