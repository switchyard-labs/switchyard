package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReleaseTagValidation(t *testing.T) {
	for _, tag := range []string{"v1.2.3", "release/v1", "2026-10-04"} {
		if !validReleaseTag(tag) {
			t.Errorf("rejected %q", tag)
		}
	}
	for _, tag := range []string{"", "-upload-pack", "../main", "a..b", "a.lock", "a@{b", "a b", "a\nb", "/v1", "v1/", "a//b"} {
		if validReleaseTag(tag) {
			t.Errorf("accepted %q", tag)
		}
	}
}

func TestReleaseDraftVisibilityAndImmutableArchives(t *testing.T) {
	sha := strings.Repeat("a", 40)
	meta := map[string]any{"id": "repo-id", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "artifact_name": "physical"}
	rows := []map[string]any{
		{"id": "draft", "repo_id": "repo-id", "identity": releaseIdentity("repo-id", "v2"), "tag": "v2", "draft": "true", "target_sha": sha},
		{"id": "published", "repo_id": "repo-id", "identity": releaseIdentity("repo-id", "v1"), "tag": "v1", "draft": "false", "target_sha": sha},
	}
	a := securityFixture(t, map[string][]map[string]any{"repository_meta": {meta}, "releases": rows})
	for _, tc := range []struct {
		user  string
		count int
	}{{"", 1}, {"bob", 1}, {"alice", 2}} {
		r := httptest.NewRequest("GET", "/api/repositories/alice/demo/releases", nil)
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "demo")
		r = r.WithContext(contextWithUser(r.Context(), tc.user))
		w := httptest.NewRecorder()
		a.handleReleases(w, r)
		var result struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || len(result.Items) != tc.count {
			t.Fatalf("%q status=%d count=%d", tc.user, w.Code, len(result.Items))
		}
		for _, item := range result.Items {
			if !strings.Contains(strOf(item["source_zip"]), sha+".zip") {
				t.Fatal("archive follows mutable tag")
			}
		}
	}
	r := httptest.NewRequest("GET", "/api/repositories/alice/demo/releases/v2", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "demo")
	r.SetPathValue("tag", "v2")
	w := httptest.NewRecorder()
	a.handleRelease(w, r)
	if w.Code != 404 {
		t.Fatalf("anonymous draft status=%d", w.Code)
	}
}

func TestPublishedReleaseCannotBeDeleted(t *testing.T) {
	meta := map[string]any{"id": "repo-id", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "artifact_name": "physical"}
	a := securityFixture(t, map[string][]map[string]any{"repository_meta": {meta}, "releases": {{"id": "published", "identity": releaseIdentity("repo-id", "v1"), "draft": "false"}}})
	r := httptest.NewRequest("DELETE", "/api/repositories/alice/demo/releases/v1", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "demo")
	r.SetPathValue("tag", "v1")
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w := httptest.NewRecorder()
	a.handleRelease(w, r)
	if w.Code != 409 {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestReleasePendingAssetBlocksPublication(t *testing.T) {
	meta := map[string]any{"id": "repo-id", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "artifact_name": "physical"}
	a := securityFixture(t, map[string][]map[string]any{"repository_meta": {meta}, "releases": {{"id": "draft", "identity": releaseIdentity("repo-id", "v1"), "draft": "true", "assets_json": "[{\"id\":\"ast_x\",\"state\":\"pending\"}]"}}})
	r := httptest.NewRequest("PATCH", "/api/repositories/alice/demo/releases/v1", strings.NewReader(`{"publish":true}`))
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "demo")
	r.SetPathValue("tag", "v1")
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w := httptest.NewRecorder()
	a.handleRelease(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "release_assets_pending") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
func TestReleaseAssetReservationValidation(t *testing.T) {
	for _, name := range []string{"../escape", "a/b", "a\\b", ".", "", "a\x00b", "a\x7fb"} {
		if validAssetName(name) {
			t.Errorf("accepted %q", name)
		}
	}
	if !validAssetName("build-linux.tar.gz") {
		t.Fatal("normal asset rejected")
	}
	for _, raw := range []string{`[{"state":"pending"}]`, `[{"state":"deleting"}]`, `invalid`} {
		if releaseAssetsReady(map[string]any{"assets_json": raw}) {
			t.Fatalf("publish allowed for %s", raw)
		}
	}
	if !releaseAssetsReady(map[string]any{"assets_json": `[{"state":"ready"}]`}) {
		t.Fatal("ready assets rejected")
	}
}

func TestReleaseTagUsesExactRefAndPeelsAnnotatedTags(t *testing.T) {
	refs := map[string]string{"refs/heads/v1": strings.Repeat("a", 40), "refs/tags/v1": strings.Repeat("b", 40), "refs/tags/v1^{}": strings.Repeat("c", 40)}
	if releaseTagSHA(refs, "v1") != strings.Repeat("c", 40) {
		t.Fatal("annotated tag was not peeled")
	}
	if releaseTagSHA(refs, "missing") != "" {
		t.Fatal("missing tag resolved")
	}
	delete(refs, "refs/tags/v1^{}")
	if releaseTagSHA(refs, "v1") != strings.Repeat("b", 40) {
		t.Fatal("branch collision changed tag")
	}
}
