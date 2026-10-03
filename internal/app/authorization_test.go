package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRepositoryOwnerMatrix(t *testing.T) {
	a := &App{}
	for _, visibility := range []string{"public", "internal", "private"} {
		meta := map[string]any{"owner_type": "user", "owner_id": "alice", "visibility": visibility}
		for _, user := range []string{"", "demo", "alice", "outsider"} {
			for _, cap := range []RepoCapability{ReadRepo, WriteRepo, AdminRepo, RunAgent, IntegrateRepo} {
				want := user == "alice" || cap == ReadRepo && visibility == "public"
				if got := a.CanRepository(meta, user, cap); got != want {
					t.Errorf("%s %q %d got %v want %v", visibility, user, cap, got, want)
				}
			}
		}
	}
	if a.CanRepository(nil, "alice", ReadRepo) {
		t.Fatal("unknown repo allowed")
	}
}
func TestGrantCapabilities(t *testing.T) {
	for _, p := range []string{"read", "write", "admin", "bogus"} {
		for _, c := range []RepoCapability{ReadRepo, WriteRepo, AdminRepo, RunAgent, IntegrateRepo} {
			want := p == "admin" || p == "write" && c != AdminRepo || p == "read" && c == ReadRepo
			if permissionAllows(p, c) != want {
				t.Fatal(p, c)
			}
		}
	}
}
func TestAnonymousAndDemoPrivateAPIs(t *testing.T) {
	a := &App{}
	for _, path := range []string{"/api/events", "/api/work", "/api/prs", "/api/credentials", "/api/workflow_runs", "/api/queue", "/api/attention", "/api/executions"} {
		for _, demo := range []bool{false, true} {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			user := ""
			if demo {
				user = "demo"
			}
			r = r.WithContext(contextWithDemoGuest(contextWithUser(r.Context(), user), demo))
			w := httptest.NewRecorder()
			called := false
			a.authorizeHandler(func(http.ResponseWriter, *http.Request) { called = true })(w, r)
			if called || w.Code != 401 {
				t.Errorf("%s demo %v status %d called %v", path, demo, w.Code, called)
			}
		}
	}
}
