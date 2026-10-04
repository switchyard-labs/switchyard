package app

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"switchyard/internal/trestle"
	"sync"
	"testing"
	"time"
)

// This fixture exercises the real Trestle client/filter transport and route
// authorization with adversarial identities; no production service is used.
func securityFixture(t *testing.T, records map[string][]map[string]any) *App {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/admin/v1/session" {
			if r.Method == "GET" {
				w.Write([]byte(`{"csrfToken":"fixture"}`))
			} else {
				w.Write([]byte(`{}`))
			}
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 5 {
			w.WriteHeader(404)
			return
		}
		collection := parts[3]
		if r.Method == "GET" {
			filter := strings.SplitN(r.URL.Query().Get("filter"), " = ", 2)
			out := []any{}
			for i, x := range records[collection] {
				if len(filter) == 2 {
					value, err := strconv.Unquote(filter[1])
					if err != nil || x[filter[0]] != value {
						continue
					}
				}
				out = append(out, map[string]any{"id": strconv.Itoa(i), "version": 1, "values": x})
			}
			json.NewEncoder(w).Encode(map[string]any{"items": out})
			return
		}
		if r.Method == "POST" && len(parts) == 5 {
			var body struct {
				Values map[string]any `json:"values"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				w.WriteHeader(400)
				return
			}
			records[collection] = append(records[collection], body.Values)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]any{"id": strconv.Itoa(len(records[collection]) - 1), "version": 1, "values": body.Values})
			return
		}
		if len(parts) == 6 {
			index, err := strconv.Atoi(parts[5])
			if err != nil || index >= len(records[collection]) {
				w.WriteHeader(404)
				return
			}
			switch r.Method {
			case "DELETE":
				records[collection] = append(records[collection][:index], records[collection][index+1:]...)
				w.Write([]byte(`{}`))
				return
			case "PATCH":
				var body struct {
					Values map[string]any `json:"values"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				for k, v := range body.Values {
					records[collection][index][k] = v
				}
				w.Write([]byte(`{}`))
				return
			}
		}
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)
	return &App{Trestle: trestle.New(srv.URL, "fixture", "fixture")}
}
func TestOrganizationCollaboratorAndTeamMatrix(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"orgs":               {{"id": "org1", "name": "org1", "owner": "owner"}},
		"org_memberships":    {{"org_id": "org1", "username": "member", "role": "member"}, {"org_id": "org1", "username": "admin", "role": "admin"}},
		"repo_collaborators": {{"repo_id": "repo", "username": "reader", "permission": "read"}, {"repo_id": "repo", "username": "writer", "permission": "write"}},
		"org_repo_access":    {{"repo_id": "repo", "org_id": "org1", "subject_type": "team", "subject_id": "team1", "permission": "write"}},
		"org_team_members":   {{"team_id": "team1", "org_id": "org1", "username": "teammate"}, {"team_id": "team1", "org_id": "other-org", "username": "cross-org"}},
	})
	meta := map[string]any{"id": "repo", "owner_type": "org", "owner_id": "org1"}
	for _, visibility := range []string{"public", "internal", "private"} {
		meta["visibility"] = visibility
		for _, user := range []string{"", "demo", "outsider", "owner", "admin", "member", "reader", "writer", "teammate", "cross-org"} {
			for _, cap := range []RepoCapability{ReadRepo, WriteRepo, AdminRepo, RunAgent, IntegrateRepo} {
				want := user == "owner" || user == "admin" || (user == "writer" || user == "teammate") && cap != AdminRepo || cap == ReadRepo && (visibility == "public" || user == "reader" || user == "member" && visibility == "internal")
				if got := a.CanRepository(meta, user, cap); got != want {
					t.Errorf("%s %q %d got %v want %v", visibility, user, cap, got, want)
				}
			}
		}
	}
}
func TestIndirectEntityAndLegacyRouteDenial(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"repository_meta": {{"id": "repo", "artifact_name": "private", "full_name": "alice/private", "owner_type": "user", "owner_id": "alice", "visibility": "private"}},
		"work":            {{"id": "work", "owner": "alice"}}, "work_details": {{"work_id": "work", "repo": "private"}},
		"attempts": {{"id": "attempt", "repo": "private", "work_id": "work"}}, "prs": {{"id": "pr", "repo": "private", "attempt_id": "attempt"}},
		"drafts": {{"id": "draft", "repo": "private", "user": "alice"}}, "credential_owners": {{"credential_id": "credential", "username": "alice"}},
	})
	for _, tc := range []struct{ path, id, name string }{{"/api/work/work", "work", ""}, {"/api/attempts/attempt/run", "attempt", ""}, {"/api/prs/pr/integrate", "pr", ""}, {"/api/drafts/draft/commit", "draft", ""}, {"/api/repos/private/content", "", "private"}} {
		r := httptest.NewRequest("POST", tc.path, strings.NewReader(`{}`))
		r.SetPathValue("id", tc.id)
		r.SetPathValue("name", tc.name)
		r = r.WithContext(contextWithUser(r.Context(), "outsider"))
		w := httptest.NewRecorder()
		called := false
		a.authorizeHandler(func(http.ResponseWriter, *http.Request) { called = true })(w, r)
		if called || w.Code != 403 {
			t.Fatal(tc, w.Code, called)
		}
	}
	if !a.ownsCredential("credential", "alice") || a.ownsCredential("credential", "outsider") {
		t.Fatal("credential ownership")
	}
	if len(a.visibleRecords("repository_meta", []map[string]any{a.repositoryMetaByArtifact("private")}, "outsider")) != 0 {
		t.Fatal("private metadata listed")
	}
}
func TestLogoutAndPasswordChangeInvalidateSession(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	token := "sess_" + strings.Repeat("a", 48) + "_" + sha256Hex(hash)[:16]
	records := map[string][]map[string]any{"users": {{"username": "alice", "password_hash": string(hash)}}, "sessions": {{"token": token, "username": "alice", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}}
	a := securityFixture(t, records)
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	if _, ok := a.userForSession(r, token); !ok {
		t.Fatal("session invalid before password change")
	}
	change := httptest.NewRequest("POST", "/api/settings/password", strings.NewReader(`{"current":"old-password","new":"new-password"}`))
	change = change.WithContext(contextWithUser(change.Context(), "alice"))
	w := httptest.NewRecorder()
	a.handleChangePassword(w, change)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, ok := a.userForSession(r, token); ok {
		t.Fatal("old password session survived")
	}
	logout := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logout.AddCookie(&http.Cookie{Name: "switchyard_session", Value: token})
	w = httptest.NewRecorder()
	a.handleLogout(w, logout)
	if w.Code != 200 || len(records["sessions"]) != 0 {
		t.Fatal("durable logout failed", w.Code)
	}
}
