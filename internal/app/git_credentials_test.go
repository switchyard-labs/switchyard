package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"switchyard/internal/artifacts"
)

type credentialTransport func(*http.Request) (*http.Response, error)

func (f credentialTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGitCredentialAuthorizationScopeAndAudit(t *testing.T) {
	for _, tc := range []struct {
		name, user, scope                      string
		ttl                                    int
		private, protected, readOnly, archived bool
		want                                   int
	}{
		{name: "guest", scope: "read", want: 401},
		{name: "public read", user: "bob", scope: "read", want: 200},
		{name: "owner write", user: "alice", scope: "write", want: 200},
		{name: "read collaborator", user: "bob", scope: "read", private: true, want: 200},
		{name: "read cannot write", user: "bob", scope: "write", private: true, want: 403},
		{name: "protected refs", user: "alice", scope: "write", protected: true, want: 409},
		{name: "upstream read only", user: "alice", scope: "write", readOnly: true, want: 403},
		{name: "archived write", user: "alice", scope: "write", archived: true, want: 403},
		{name: "private nonmember", user: "charlie", scope: "read", private: true, want: 403},
		{name: "protected read", user: "alice", scope: "read", protected: true, want: 200},
		{name: "excess ttl", user: "alice", scope: "read", ttl: 901, want: 400},
		{name: "unknown scope", user: "alice", scope: "admin", want: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, _ := actionFixture(t)
			visibility := "public"
			if tc.private {
				visibility = "private"
			}
			for collection, record := range map[string]map[string]any{
				"repository_meta":    {"id": "repo", "full_name": "alice/demo", "owner_type": "user", "owner_id": "alice", "owner_slug": "alice", "slug": "demo", "artifact_name": "repo", "visibility": visibility},
				"repo_collaborators": {"repo_id": "repo", "username": "bob", "permission": "read"},
			} {
				if _, _, e := a.Trestle.CreateRecord(collection, record, randHex(8)); e != nil {
					t.Fatal(e)
				}
			}
			if tc.protected {
				if _, _, e := a.Trestle.CreateRecord("repo_protected_refs", map[string]any{"repo_id": "repo", "pattern": "main", "require_queue": "true"}, randHex(8)); e != nil {
					t.Fatal(e)
				}
			}
			if tc.archived {
				if _, _, e := a.Trestle.CreateRecord("repository_settings", map[string]any{"repo_id": "repo", "archived": "true"}, randHex(8)); e != nil {
					t.Fatal(e)
				}
			}
			helper := filepath.Join(t.TempDir(), "token-helper")
			if e := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture-account-token"), 0700); e != nil {
				t.Fatal(e)
			}
			minted := 0
			token := "provider_future_" + strings.Repeat("a", 40) + "?expires=" + fmt.Sprint(time.Now().Add(600*time.Second).Unix())
			a.Artifacts = artifacts.NewWithHTTP("fixture", "ns", helper, &http.Client{Transport: credentialTransport(func(r *http.Request) (*http.Response, error) {
				body := fmt.Sprintf(`{"result":{"remote":"https://fixture.artifacts.cloudflare.net/git/ns/repo.git","read_only":%t}}`, tc.readOnly)
				if strings.HasSuffix(r.URL.Path, "/tokens") {
					minted++
					var request struct {
						Repo, Scope string
						TTL         int
					}
					if e := json.NewDecoder(r.Body).Decode(&request); e != nil {
						t.Fatal(e)
					}
					if request.Repo != "repo" || request.Scope != tc.scope || request.TTL != 600 {
						t.Fatal("incorrect mint scope")
					}
					body = fmt.Sprintf(`{"result":{"plaintext":%q}}`, token)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})})
			r := httptest.NewRequest("POST", "/api/repositories/alice/demo/git-credential", strings.NewReader(fmt.Sprintf(`{"scope":%q,"ttl_seconds":%d}`, tc.scope, tc.ttl)))
			r.SetPathValue("owner", "alice")
			r.SetPathValue("repo", "demo")
			r = r.WithContext(contextWithUser(r.Context(), tc.user))
			w := httptest.NewRecorder()
			a.authorizeHandler(a.handleGitCredential)(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d expected %d", w.Code, tc.want)
			}
			if tc.want != 200 {
				if minted != 0 {
					t.Fatal("denied request minted credential")
				}
				return
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("secret response cacheable")
			}
			var response struct {
				Token, Remote, ExpiresAt string
				Permissions              []string
			}
			if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil {
				t.Fatal(e)
			}
			if response.Token != token || response.Remote == "" || minted != 1 {
				t.Fatal("invalid credential response")
			}
			audit, e := a.Trestle.ListRecords("audit", "")
			if e != nil || len(audit) != 1 {
				t.Fatal("missing issuance audit")
			}
			encoded, _ := json.Marshal(audit)
			if strings.Contains(string(encoded), token) || strings.Contains(string(encoded), "fixture-account-token") {
				t.Fatal("audit persisted a secret")
			}
		})
	}
}
