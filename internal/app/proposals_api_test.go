package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProposalRepositoryReadIsolation(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"repository_meta": {
			{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "visibility": "public"},
			{"id": "priv", "full_name": "alice/private", "owner_type": "user", "owner_id": "alice", "visibility": "private"},
		},
		"proposals": {{"id": "secret", "repository_id": "priv", "title": "Private evidence"}, {"id": "visible", "repository_id": "pub", "title": "Public intake", "state": "open", "type": "Question"}},
	})
	for _, tc := range []struct {
		repo, id string
		status   int
	}{{"public", "", 200}, {"public", "secret", 404}, {"private", "", 404}} {
		r := httptest.NewRequest("GET", "/api/repositories/alice/"+tc.repo+"/proposals", nil)
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", tc.repo)
		r.SetPathValue("id", tc.id)
		w := httptest.NewRecorder()
		a.handleProposals(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s/%s: %d %s", tc.repo, tc.id, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Private evidence") {
			t.Fatal("cross-repository evidence leaked")
		}
	}
}

func TestProposalGuestCannotForgePrincipal(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"repository_meta": {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "visibility": "public"}},
	})
	r := httptest.NewRequest("POST", "/api/repositories/alice/public/proposals", strings.NewReader(`{"title":"x","type":"Bug","author_principal":"alice","provenance":{"source":"agent"}}`))
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "public")
	w := httptest.NewRecorder()
	a.handleProposals(w, r)
	if w.Code < 400 {
		t.Fatal("anonymous forged mutation accepted")
	}
}
