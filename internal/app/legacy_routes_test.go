package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestLegacyRepositoryBookmarkUsesCanonicalIdentity(t *testing.T) {
	meta := map[string]any{"owner_slug": "alice", "slug": "railway", "default_branch": "main"}
	for _, tc := range []struct{ query, section, want string }{
		{"name=physical", "", "/alice/railway"},
		{"name=physical&ref=feature-x", "", "/alice/railway/tree/feature-x"},
		{"name=physical&path=docs%2Fhello+world.md", "", "/alice/railway/blob/main/docs/hello%20world.md"},
		{"name=physical", "commits", "/alice/railway/commits/main"},
		{"name=physical", "settings", "/alice/railway/settings"},
		{"name=physical&ref=feature-x&path=src%2Fhello+world.mjs&attempt=wk_123", "edit", "/alice/railway/edit/feature-x/src/hello%20world.mjs?attempt=wk_123"},
		{"name=physical", "edit", "/alice/railway/edit/main"},
	} {
		q, _ := url.ParseQuery(tc.query)
		got, err := legacyRepositoryTarget(meta, q, tc.section)
		if err != nil || got != tc.want {
			t.Fatalf("%s: %q %v", tc.query, got, err)
		}
	}
	q := url.Values{"path": {"../private"}}
	if _, err := legacyRepositoryTarget(meta, q, ""); err == nil {
		t.Fatal("traversal bookmark accepted")
	}
	if _, err := legacyRepositoryTarget(map[string]any{"owner_slug": "https://evil", "slug": "x"}, nil, ""); err == nil {
		t.Fatal("external identity accepted")
	}
}

func TestLegacyBookmarkGuestPublicAndPrivateBoundary(t *testing.T) {
	a, _, _ := actionFixture(t)
	for _, visibility := range []string{"public", "private"} {
		_, _, err := a.Trestle.CreateRecord("repository_meta", map[string]any{"artifact_name": visibility, "owner_slug": "alice", "slug": visibility, "visibility": visibility, "owner_type": "user", "owner_id": "alice", "default_branch": "main"}, visibility)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"public", "private", "unknown"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/history.html?name="+name, nil)
		if !a.redirectLegacyRepository(w, r) {
			t.Fatal("bookmark was not handled")
		}
		if name == "public" {
			if w.Code != http.StatusFound || w.Header().Get("Location") != "/alice/public/commits/main" {
				t.Fatal("public history unavailable")
			}
		} else if w.Code != http.StatusNotFound || w.Header().Get("Location") != "" {
			t.Fatal("private or unregistered identity exposed")
		}
	}
}
