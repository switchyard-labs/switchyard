package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"switchyard/internal/actions"
	"testing"
)

type gatedAssetStore struct {
	actions.Provider
	started chan struct{}
	finish  chan struct{}
}

func (s *gatedAssetStore) UploadReleaseAsset(_ context.Context, repo, id, hash string, size int64, body io.Reader) error {
	close(s.started)
	<-s.finish
	_, err := io.Copy(io.Discard, body)
	return err
}
func (s *gatedAssetStore) GetReleaseAsset(context.Context, string, string) (*http.Response, error) {
	return nil, nil
}
func (s *gatedAssetStore) DeleteReleaseAsset(context.Context, string, string) error { return nil }
func releaseRequest(method, path, body string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "demo")
	r.SetPathValue("tag", "v1")
	return r.WithContext(contextWithUser(r.Context(), "alice"))
}
func TestReleaseUploadFencesPublicationAndConflictingName(t *testing.T) {
	a, _, _ := actionFixture(t)
	meta := map[string]any{"id": "repo-id", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "artifact_name": "repo"}
	for collection, record := range map[string]map[string]any{"repository_meta": meta, "releases": {"id": "release-id", "identity": releaseIdentity("repo-id", "v1"), "repo_id": "repo-id", "tag": "v1", "draft": "true"}} {
		if _, _, err := a.Trestle.CreateRecord(collection, record, collection); err != nil {
			t.Fatal(err)
		}
	}
	storage := &gatedAssetStore{started: make(chan struct{}), finish: make(chan struct{})}
	a.Actions = storage
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		a.handleReleaseAssetUpload(w, releaseRequest("POST", "/api/repositories/alice/demo/release-assets?tag=v1&name=build.zip", "abc"))
		result <- w
	}()
	<-storage.started
	w := httptest.NewRecorder()
	a.handleRelease(w, releaseRequest("PATCH", "/api/repositories/alice/demo/releases/v1", `{"publish":true}`))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "release_assets_pending") {
		t.Fatalf("publication escaped pending reservation: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	a.handleReleaseAssetUpload(w, releaseRequest("POST", "/api/repositories/alice/demo/release-assets?tag=v1&name=build.zip", "different"))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "asset_name_exists") {
		t.Fatalf("conflicting name accepted: %d %s", w.Code, w.Body.String())
	}
	close(storage.finish)
	uploaded := <-result
	if uploaded.Code != 201 {
		t.Fatalf("upload %d %s", uploaded.Code, uploaded.Body.String())
	}
	_, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", releaseIdentity("repo-id", "v1")))
	if err != nil {
		t.Fatal(err)
	}
	if !releaseAssetsReady(record) {
		t.Fatal("completed upload not attached")
	}
	assets, _ := releaseAssets(record)
	if len(assets) != 1 || assets[0].Name != "build.zip" || assets[0].Size != 3 {
		t.Fatal("wrong asset metadata")
	}
}
