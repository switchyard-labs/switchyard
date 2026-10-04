package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"switchyard/internal/actions"
)

type actionBuildStore interface {
	releaseAssetStore
	GetBuildAsset(context.Context, string, string, string, string) (*http.Response, error)
}

// Approved tag automation owns a deterministic release identity. It never
// adopts a manually-created draft, nor changes an existing release's source.
func (a *App) syncActionRelease(ctx context.Context, manifest *actions.Manifest, approvedBy, status string) error {
	run := manifest.Run
	if run.Release == nil {
		return nil
	}
	if !strings.HasPrefix(run.Ref, "refs/tags/") {
		return fmt.Errorf("release automation requires tag")
	}
	tag := strings.TrimPrefix(run.Ref, "refs/tags/")
	if !validReleaseTag(tag) {
		return fmt.Errorf("invalid automated release tag")
	}
	metas, err := a.Trestle.ListRecords("repository_meta", filterEq("artifact_name", run.Repo))
	if err != nil || len(metas) != 1 {
		return fmt.Errorf("release repository unavailable")
	}
	meta := metas[0]
	if approvedBy == "" || !a.CanRepository(meta, approvedBy, WriteRepo) {
		return fmt.Errorf("release approval authority expired")
	}
	identity := releaseIdentity(strOf(meta["id"]), tag)
	expectedID := "rel_action_" + sha256Hex([]byte(identity+"\x00"+run.SHA+"\x00"+run.DefinitionRevision))
	rid, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	if err != nil {
		return err
	}
	if record == nil {
		commits, e := a.releaseTagCommit(ctx, run.Repo, tag)
		if e != nil || len(commits) != 1 || commits[0].Hash != run.SHA {
			return fmt.Errorf("automated release tag moved or unavailable")
		}
		record = map[string]any{"id": expectedID, "identity": identity, "repo_id": meta["id"], "tag": tag, "target_sha": run.SHA, "title": tag, "body": "Built by Actions at " + run.SHA, "draft": "true", "prerelease": fmt.Sprint(run.Release.Prerelease), "author": approvedBy, "created_at": nowStr(), "updated_at": nowStr(), "published_at": ""}
		if err = a.audit(strOf(meta["owner_slug"]), "release.actions.create", expectedID, "allowed", "approved tag build draft", approvedBy); err != nil {
			return err
		}
		if _, _, err = a.Trestle.CreateRecord("releases", record, "release-"+identity); err != nil {
			return err
		}
		rid, _, record, err = a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if err != nil {
			return err
		}
	}
	if strOf(record["id"]) != expectedID || strOf(record["target_sha"]) != run.SHA {
		return fmt.Errorf("release automation identity conflict")
	}
	// Failure and cancellation keep the source-bound draft; partial build output
	// stays staged in R2 and cannot be attached or published by this path.
	if status != "success" || strOf(record["draft"]) != "true" {
		return nil
	}
	store, ok := a.Actions.(actionBuildStore)
	if !ok {
		return fmt.Errorf("Actions release storage unavailable")
	}
	expected := map[string]actions.BuildAssetReceipt{}
	for _, job := range run.Jobs {
		var view *actions.JobState
		for i := range manifest.Jobs {
			if manifest.Jobs[i].ID == job.ID {
				view = &manifest.Jobs[i]
			}
		}
		if view == nil || view.Status != "succeeded" {
			return fmt.Errorf("release build job incomplete")
		}
		for _, asset := range job.Assets {
			var receipt *actions.BuildAssetReceipt
			for i := range view.Assets {
				if view.Assets[i].Name == asset.Name {
					receipt = &view.Assets[i]
				}
			}
			nameHash := sha256.Sum256([]byte(asset.Name))
			key := "build-assets/" + run.Repo + "/" + run.ID + "/" + job.ID + "/" + hex.EncodeToString(nameHash[:])
			// A failed-job rerun may reuse a successful parent's output lane.
			if view.ReusedFrom != "" {
				key = "build-assets/" + run.Repo + "/" + view.ReusedFrom + "/" + job.ID + "/" + hex.EncodeToString(nameHash[:])
			}
			if receipt == nil || !validAssetName(asset.Name) || receipt.SourceSHA != run.SHA || receipt.JobID != job.ID || receipt.Key != key || receipt.Size < 1 || receipt.Size > actions.ReleaseAssetLimit || len(receipt.SHA256) != 64 || strings.Trim(receipt.SHA256, "0123456789abcdef") != "" {
				return fmt.Errorf("release build receipt invalid")
			}
			expected[asset.Name] = *receipt
		}
	}
	// Reserve all names on the same release CAS before the first external copy.
	for tries := 0; tries < 8; tries++ {
		var version string
		rid, version, record, err = a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if err != nil {
			return err
		}
		if record == nil || strOf(record["draft"]) != "true" {
			return fmt.Errorf("release changed during asset reservation")
		}
		assets, e := releaseAssets(record)
		if e != nil {
			return e
		}
		for _, old := range assets {
			want, exists := expected[old.Name]
			if !exists || old.SHA256 != want.SHA256 || old.Size != want.Size || old.State == "deleting" {
				return fmt.Errorf("automated release asset conflict")
			}
		}
		changed := false
		for name, want := range expected {
			found := false
			for _, old := range assets {
				if old.Name == name {
					found = true
				}
			}
			if !found {
				changed = true
				id := "ast_" + sha256Hex([]byte(expectedID + "\x00" + name + "\x00" + want.SHA256))[:24]
				assets = append(assets, releaseAsset{ID: id, StorageKey: "release-assets/" + run.Repo + "/" + id, Name: name, Size: want.Size, SHA256: want.SHA256, ContentType: "application/octet-stream", CreatedAt: nowStr(), State: "pending"})
			}
		}
		if !changed {
			break
		}
		raw, _ := json.Marshal(assets)
		err = a.Trestle.PatchRecord("releases", rid, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
		if err == nil {
			record["assets_json"] = string(raw)
			break
		}
		if draftPersistenceStatus(err) != 409 || tries == 7 {
			return err
		}
	}
	assets, err := releaseAssets(record)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if asset.State == "ready" {
			continue
		}
		want := expected[asset.Name]
		sourceRun := run.ID
		parts := strings.Split(want.Key, "/")
		if len(parts) != 5 {
			return fmt.Errorf("invalid output key")
		}
		sourceRun = parts[2]
		response, e := store.GetBuildAsset(ctx, run.Repo, sourceRun, want.JobID, want.Name)
		if e != nil {
			return e
		}
		if response.Body == nil || response.ContentLength != want.Size || response.Header.Get("X-Checksum-SHA256") != want.SHA256 || response.Header.Get("X-Source-SHA") != run.SHA {
			if response.Body != nil {
				response.Body.Close()
			}
			return fmt.Errorf("build output transport identity mismatch")
		}
		e = store.UploadReleaseAsset(ctx, run.Repo, asset.ID, want.SHA256, want.Size, io.LimitReader(response.Body, want.Size+1))
		response.Body.Close()
		if e != nil {
			return e
		}
		for tries := 0; tries < 8; tries++ {
			id, version, current, e := a.Trestle.FindRecord("releases", filterEq("identity", identity))
			if e != nil {
				return e
			}
			values, e := releaseAssets(current)
			if e != nil {
				return e
			}
			found := false
			for i := range values {
				if values[i].ID == asset.ID && values[i].State != "deleting" {
					values[i].State = "ready"
					found = true
				}
			}
			if !found {
				return fmt.Errorf("release output reservation disappeared")
			}
			raw, _ := json.Marshal(values)
			e = a.Trestle.PatchRecord("releases", id, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
			if e == nil {
				break
			}
			if draftPersistenceStatus(e) != 409 || tries == 7 {
				return e
			}
		}
	}
	if !run.Release.Publish {
		return nil
	}
	commits, err := a.releaseTagCommit(ctx, run.Repo, tag)
	if err != nil || len(commits) != 1 || commits[0].Hash != run.SHA {
		return fmt.Errorf("automated release tag moved before publish")
	}
	rid, version, current, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	if err != nil {
		return err
	}
	if strOf(current["draft"]) != "true" {
		return nil
	}
	if !releaseAssetsReady(current) {
		return fmt.Errorf("release output pending")
	}
	finalAssets, err := releaseAssets(current)
	if err != nil || len(finalAssets) != len(expected) {
		return fmt.Errorf("automated release outputs changed")
	}
	for _, asset := range finalAssets {
		want, ok := expected[asset.Name]
		if !ok || asset.Size != want.Size || asset.SHA256 != want.SHA256 {
			return fmt.Errorf("automated release outputs changed")
		}
	}
	if err = a.audit(strOf(meta["owner_slug"]), "release.actions.publish", expectedID, "allowed", "successful approved exact-SHA build", approvedBy); err != nil {
		return err
	}
	return a.Trestle.PatchRecord("releases", rid, version, map[string]any{"draft": "false", "published_at": nowStr(), "updated_at": nowStr()})
}
