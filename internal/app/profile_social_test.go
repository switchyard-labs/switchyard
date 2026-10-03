package app

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRefAttributionUsesExactActor(t *testing.T) {
	for _, tc := range []struct {
		provenance string
		want       bool
	}{{"user:alice", true}, {"editor:alice:draft agent:execution", true}, {"browser-file-operation:alice", true}, {"resolve:editor:alice:draft", true}, {"escalation:alice:item", true}, {"user:malice", false}, {"editor:bob:alice", false}, {"", false}, {"unknown:alice", false}} {
		if got := attributedRef(tc.provenance, "alice"); got != tc.want {
			t.Errorf("%q: got %v", tc.provenance, got)
		}
	}
}
func TestContributionsExcludeUnattributedAndPrivateUpdates(t *testing.T) {
	a, _, _ := actionFixture(t)
	for _, record := range []struct {
		collection string
		values     map[string]any
	}{
		{"users", map[string]any{"username": "alice"}},
		{"repository_meta", map[string]any{"id": "repo", "artifact_name": "repo", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "visibility": "public", "owner_type": "user", "owner_id": "alice"}},
		{"ref_updates", map[string]any{"repo": "repo", "provenance": "user:alice", "new_sha": "abc123", "occurred_at": time.Now().UTC().Format(time.RFC3339)}},
		{"ref_updates", map[string]any{"repo": "repo", "provenance": "editor:alice:draft", "new_sha": "abc123", "occurred_at": time.Now().UTC().Format(time.RFC3339)}},
		{"ref_updates", map[string]any{"repo": "repo", "provenance": "user:malice", "occurred_at": time.Now().UTC().Format(time.RFC3339)}},
		{"ref_updates", map[string]any{"repo": "private", "provenance": "user:alice", "new_sha": "abc123", "occurred_at": time.Now().UTC().Format(time.RFC3339)}},
	} {
		if _, _, err := a.Trestle.CreateRecord(record.collection, record.values, randHex(8)); err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest("GET", "/api/users/alice/contributions", nil)
	r.SetPathValue("username", "alice")
	w := httptest.NewRecorder()
	a.handleUserContributions(w, r)
	var data struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || data.Total != 1 {
		t.Fatalf("status %d total %d", w.Code, data.Total)
	}
}
func TestGuestCannotFollow(t *testing.T) {
	a, _, _ := actionFixture(t)
	r := httptest.NewRequest("PUT", "/api/users/alice/follow", nil)
	r.SetPathValue("username", "alice")
	w := httptest.NewRecorder()
	a.handleFollowUser(w, r)
	if w.Code != 401 {
		t.Fatalf("status %d", w.Code)
	}
}

func TestFollowIsIdempotentAndRemovable(t *testing.T) {
	a, _, _ := actionFixture(t)
	for _, name := range []string{"alice", "bob"} {
		if _, _, err := a.Trestle.CreateRecord("users", map[string]any{"username": name}, name); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []string{"PUT", "PUT", "DELETE", "DELETE"} {
		r := httptest.NewRequest(method, "/api/users/bob/follow", nil)
		r.SetPathValue("username", "bob")
		r = r.WithContext(contextWithUser(r.Context(), "alice"))
		w := httptest.NewRecorder()
		a.handleFollowUser(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", method, w.Code, w.Body.String())
		}
		records, err := a.Trestle.ListRecords("user_follows", "")
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if method == "PUT" {
			want = 1
		}
		if len(records) != want {
			t.Fatalf("%s: %d records", method, len(records))
		}
	}
}
func TestStarsIdempotentAndPrivateCountsHidden(t *testing.T) {
	a, _, _ := actionFixture(t)
	if _, _, err := a.Trestle.CreateRecord("repository_meta", map[string]any{"id": "repo", "artifact_name": "repo", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": "public"}, "repo"); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"PUT", "PUT", "GET", "DELETE", "DELETE"} {
		r := httptest.NewRequest(method, "/api/repositories/alice/demo/stars", nil)
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "demo")
		if method != "GET" {
			r = r.WithContext(contextWithUser(r.Context(), "bob"))
		}
		w := httptest.NewRecorder()
		a.authorizeHandler(a.handleRepositoryStars)(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", method, w.Code, w.Body.String())
		}
		var data struct {
			Count int `json:"count"`
		}
		json.Unmarshal(w.Body.Bytes(), &data)
		want := 1
		if method == "DELETE" {
			want = 0
		}
		if data.Count != want {
			t.Fatalf("%s count %d", method, data.Count)
		}
	}
	rid, ver, _, _ := a.Trestle.FindRecord("repository_meta", filterEq("id", "repo"))
	if err := a.Trestle.PatchRecord("repository_meta", rid, ver, map[string]any{"visibility": "private"}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/repositories/alice/demo/stars", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "demo")
	w := httptest.NewRecorder()
	a.handleRepositoryStars(w, r)
	if w.Code != 404 {
		t.Fatalf("private counts exposed: %d", w.Code)
	}
}
