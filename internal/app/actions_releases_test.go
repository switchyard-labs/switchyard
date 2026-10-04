package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"

	"switchyard/internal/actions"
)

type buildReleaseTestStore struct {
	actions.Provider
	uploads int
	reads   int
	fail    bool
}

func (s *buildReleaseTestStore) GetBuildAsset(context.Context, string, string, string, string) (*http.Response, error) {
	s.reads++
	return &http.Response{Body: io.NopCloser(strings.NewReader("abc")), ContentLength: 3, Header: http.Header{"X-Checksum-Sha256": {hex.EncodeToString(testBuildHash[:])}, "X-Source-Sha": {strings.Repeat("a", 40)}}}, nil
}

var testBuildHash = sha256.Sum256([]byte("abc"))

func (s *buildReleaseTestStore) UploadReleaseAsset(_ context.Context, repo, id, hash string, size int64, body io.Reader) error {
	s.uploads++
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if string(data) != "abc" || hash != hex.EncodeToString(testBuildHash[:]) || size != 3 {
		return io.ErrUnexpectedEOF
	}
	if s.fail {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (*buildReleaseTestStore) GetReleaseAsset(context.Context, string, string) (*http.Response, error) {
	panic("unexpected public read")
}
func (*buildReleaseTestStore) DeleteReleaseAsset(context.Context, string, string) error {
	panic("unexpected delete")
}

func actionReleaseFixture(t *testing.T) (*App, *actions.Manifest, *buildReleaseTestStore) {
	a, _, snapshot := actionFixture(t)
	run := &snapshot.Manifest.Run
	run.Ref = "refs/tags/v1"
	run.Release = &actions.ReleasePolicy{}
	run.Jobs[0].Assets = []actions.BuildAsset{{Name: "program.zip", Path: "dist/program.zip"}}
	nameHash := sha256.Sum256([]byte("program.zip"))
	snapshot.Manifest.Jobs[0].Assets = []actions.BuildAssetReceipt{{Name: "program.zip", Size: 3, SHA256: hex.EncodeToString(testBuildHash[:]), SourceSHA: run.SHA, JobID: "verify", Key: "build-assets/repo/run1/verify/" + hex.EncodeToString(nameHash[:])}}
	meta := map[string]any{"id": "meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "owner_slug": "alice", "slug": "rail", "visibility": "public"}
	identity := releaseIdentity("meta", "v1")
	id := "rel_action_" + sha256Hex([]byte(identity+"\x00"+run.SHA+"\x00"+run.DefinitionRevision))
	for collection, record := range map[string]map[string]any{"repository_meta": meta, "releases": {"id": id, "identity": identity, "repo_id": "meta", "tag": "v1", "target_sha": run.SHA, "draft": "true"}} {
		if _, _, err := a.Trestle.CreateRecord(collection, record, collection); err != nil {
			t.Fatal(err)
		}
	}
	store := &buildReleaseTestStore{}
	a.Actions = store
	return a, snapshot.Manifest, store
}

func TestActionReleaseCopiesExactOutputsAndReplayDoesNotCopyAgain(t *testing.T) {
	a, m, store := actionReleaseFixture(t)
	for i := 0; i < 2; i++ {
		if err := a.syncActionRelease(context.Background(), m, "alice", "success"); err != nil {
			t.Fatal(err)
		}
	}
	if store.uploads != 1 || store.reads != 1 {
		t.Fatal("replay copied output again")
	}
	_, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", releaseIdentity("meta", "v1")))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := releaseAssets(record)
	if err != nil || len(assets) != 1 || assets[0].State != "ready" || record["draft"] != "true" {
		t.Fatal("verified draft asset missing")
	}
}
func TestActionReleaseFailureDoesNotAttachAndInvalidReceiptFailsClosed(t *testing.T) {
	a, m, store := actionReleaseFixture(t)
	if err := a.syncActionRelease(context.Background(), m, "alice", "failure"); err != nil {
		t.Fatal(err)
	}
	if store.reads != 0 || store.uploads != 0 {
		t.Fatal("failed build copied output")
	}
	m.Jobs[0].Assets[0].SourceSHA = strings.Repeat("b", 40)
	if a.syncActionRelease(context.Background(), m, "alice", "success") == nil {
		t.Fatal("wrong source receipt accepted")
	}
	if store.reads != 0 {
		t.Fatal("invalid receipt reached storage")
	}
}
func TestActionReleaseCopyFailureRemainsPendingAndRecovers(t *testing.T) {
	a, m, store := actionReleaseFixture(t)
	store.fail = true
	if a.syncActionRelease(context.Background(), m, "alice", "success") == nil {
		t.Fatal("copy failure swallowed")
	}
	_, _, record, _ := a.Trestle.FindRecord("releases", filterEq("identity", releaseIdentity("meta", "v1")))
	if releaseAssetsReady(record) || record["draft"] != "true" {
		t.Fatal("failed copy published")
	}
	store.fail = false
	if err := a.syncActionRelease(context.Background(), m, "alice", "success"); err != nil {
		t.Fatal(err)
	}
}

func TestActionReleaseDoesNotAdoptManualDraftOrRevokedAuthority(t *testing.T) {
	a, m, store := actionReleaseFixture(t)
	if a.syncActionRelease(context.Background(), m, "bob", "success") == nil {
		t.Fatal("non-maintainer approval accepted")
	}
	id, version, _, err := a.Trestle.FindRecord("releases", filterEq("identity", releaseIdentity("meta", "v1")))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Trestle.PatchRecord("releases", id, version, map[string]any{"id": "rel_manual"}); err != nil {
		t.Fatal(err)
	}
	if a.syncActionRelease(context.Background(), m, "alice", "success") == nil {
		t.Fatal("manual draft adopted by automation")
	}
	if store.reads != 0 || store.uploads != 0 {
		t.Fatal("unauthorized automation touched storage")
	}
}
