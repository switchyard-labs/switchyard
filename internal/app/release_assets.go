package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"switchyard/internal/actions"
	"time"
)

// Asset reservations live in the release record: the same CAS which publishes
// the release fences in-flight uploads. An interrupted upload remains pending
// and can be retried with the identical name/content, never silently published.
type releaseAsset struct {
	ID          string `json:"id"`
	StorageKey  string `json:"storage_key"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
	State       string `json:"state"`
}
type releaseAssetStore interface {
	UploadReleaseAsset(context.Context, string, string, string, int64, io.Reader) error
	GetReleaseAsset(context.Context, string, string) (*http.Response, error)
	DeleteReleaseAsset(context.Context, string, string) error
}

func releaseAssets(record map[string]any) ([]releaseAsset, error) {
	result := []releaseAsset{}
	if raw := strOf(record["assets_json"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func releaseAssetsReady(record map[string]any) bool {
	assets, err := releaseAssets(record)
	if err != nil {
		return false
	}
	for _, asset := range assets {
		if asset.State != "ready" {
			return false
		}
	}
	return true
}
func validAssetName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 180 || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, c := range name {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}
func (a *App) handleReleaseAssetUpload(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) || !a.CanRepository(meta, user, WriteRepo) {
		writeJSON(w, 403, map[string]any{"error": "release_maintainer_required"})
		return
	}
	store, ok := a.Actions.(releaseAssetStore)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "release_storage_unavailable"})
		return
	}
	name, tag := r.URL.Query().Get("name"), r.URL.Query().Get("tag")
	if !validAssetName(name) || !validReleaseTag(tag) || r.ContentLength < 1 || r.ContentLength > actions.ReleaseAssetLimit {
		writeJSON(w, 400, map[string]any{"error": "invalid_asset"})
		return
	}
	identity := releaseIdentity(strOf(meta["id"]), tag)
	rid, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	if err != nil || rid == "" {
		writeJSON(w, 404, map[string]any{"error": "release_not_found"})
		return
	}
	if strOf(record["draft"]) != "true" {
		writeJSON(w, 409, map[string]any{"error": "published_assets_immutable"})
		return
	}
	file, err := os.CreateTemp("", "switchyard-release-asset-*")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "asset_staging_failed"})
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(r.Body, actions.ReleaseAssetLimit+1))
	if err != nil || size != r.ContentLength || size > actions.ReleaseAssetLimit {
		writeJSON(w, 400, map[string]any{"error": "asset_size_mismatch"})
		return
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	asset := releaseAsset{ID: "ast_" + sha256Hex([]byte(strOf(record["id"]) + "\x00" + name + "\x00" + checksum))[:24], Name: name, Size: size, SHA256: checksum, ContentType: "application/octet-stream", CreatedAt: nowStr(), State: "pending"}
	asset.StorageKey = "release-assets/" + repo + "/" + asset.ID
	if err = a.audit(strOf(meta["owner_slug"]), "release.asset.upload", asset.ID, "allowed", "asset upload intent", user); err != nil {
		writeJSON(w, 502, map[string]any{"error": "release_audit_failed"})
		return
	}
	// Reserve before touching R2. CAS retries allow independent names while the
	// unique name/content reservation makes ambiguous upload success retryable.
	reserved := false
	for tries := 0; tries < 8; tries++ {
		rid, version, current, e := a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if e != nil || rid == "" {
			break
		}
		if strOf(current["draft"]) != "true" {
			writeJSON(w, 409, map[string]any{"error": "published_assets_immutable"})
			return
		}
		assets, e := releaseAssets(current)
		if e != nil {
			break
		}
		found := false
		for _, old := range assets {
			if old.Name == name {
				if old.ID != asset.ID || old.Size != size {
					writeJSON(w, 409, map[string]any{"error": "asset_name_exists"})
					return
				}
				asset = old
				found = true
				break
			}
		}
		if found {
			reserved = true
			break
		}
		if len(assets) >= 50 {
			writeJSON(w, 409, map[string]any{"error": "release_asset_limit"})
			return
		}
		assets = append(assets, asset)
		raw, _ := json.Marshal(assets)
		e = a.Trestle.PatchRecord("releases", rid, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
		if e == nil {
			reserved = true
			break
		}
		if draftPersistenceStatus(e) != 409 {
			break
		}
	}
	if !reserved {
		writeJSON(w, 409, map[string]any{"error": "asset_reservation_failed"})
		return
	}
	if asset.State == "deleting" {
		writeJSON(w, 409, map[string]any{"error": "asset_deletion_pending"})
		return
	}
	if asset.State == "ready" {
		writeJSON(w, 200, asset)
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		writeJSON(w, 500, map[string]any{"error": "asset_staging_failed"})
		return
	}
	if err = store.UploadReleaseAsset(r.Context(), repo, asset.ID, checksum, size, file); err != nil {
		writeJSON(w, 502, map[string]any{"error": "asset_upload_failed", "retryable": true})
		return
	}
	for tries := 0; tries < 8; tries++ {
		rid, version, current, e := a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if e != nil || rid == "" {
			break
		}
		assets, e := releaseAssets(current)
		if e != nil {
			break
		}
		found := false
		for i := range assets {
			if assets[i].ID == asset.ID {
				if assets[i].State == "ready" {
					writeJSON(w, 200, assets[i])
					return
				}
				if assets[i].State != "pending" {
					writeJSON(w, 409, map[string]any{"error": "asset_deletion_pending"})
					return
				}
				assets[i].State = "ready"
				asset = assets[i]
				found = true
				break
			}
		}
		if !found {
			break
		}
		raw, _ := json.Marshal(assets)
		e = a.Trestle.PatchRecord("releases", rid, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
		if e == nil {
			writeJSON(w, 201, asset)
			return
		}
		if draftPersistenceStatus(e) != 409 {
			break
		}
	}
	writeJSON(w, 502, map[string]any{"error": "asset_attachment_pending", "retryable": true})
}
func (a *App) handleReleaseAssetDownload(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	identity := releaseIdentity(strOf(meta["id"]), r.URL.Query().Get("tag"))
	rid, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	canWrite := a.currentUser(r) != "" && !a.isDemoGuest(r) && a.CanRepository(meta, a.currentUser(r), WriteRepo)
	if err != nil || rid == "" || (strOf(record["draft"]) == "true" && !canWrite) {
		writeJSON(w, 404, map[string]any{"error": "release_not_found"})
		return
	}
	assets, err := releaseAssets(record)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "assets_unavailable"})
		return
	}
	var selected *releaseAsset
	for i := range assets {
		if assets[i].ID == r.PathValue("asset") && assets[i].State == "ready" {
			selected = &assets[i]
			break
		}
	}
	if selected == nil {
		writeJSON(w, 404, map[string]any{"error": "asset_not_found"})
		return
	}
	store, ok := a.Actions.(releaseAssetStore)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "release_storage_unavailable"})
		return
	}
	response, err := store.GetReleaseAsset(r.Context(), repo, selected.ID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "asset_download_failed"})
		return
	}
	defer response.Body.Close()
	if response.ContentLength != selected.Size || response.Header.Get("X-Checksum-SHA256") != selected.SHA256 {
		writeJSON(w, 502, map[string]any{"error": "asset_integrity_mismatch"})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(selected.Size, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": selected.Name}))
	w.Header().Set("X-Checksum-SHA256", selected.SHA256)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, io.LimitReader(response.Body, selected.Size))
}

func (a *App) handleReleaseAssetDelete(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) || !a.CanRepository(meta, user, WriteRepo) {
		writeJSON(w, 403, map[string]any{"error": "release_maintainer_required"})
		return
	}
	store, ok := a.Actions.(releaseAssetStore)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "release_storage_unavailable"})
		return
	}
	identity := releaseIdentity(strOf(meta["id"]), r.URL.Query().Get("tag"))
	id := r.PathValue("asset")
	if err := a.audit(strOf(meta["owner_slug"]), "release.asset.delete", id, "allowed", "draft asset deletion intent", user); err != nil {
		writeJSON(w, 502, map[string]any{"error": "release_audit_failed"})
		return
	}
	reserved := false
	for tries := 0; tries < 8; tries++ {
		rid, version, current, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if err != nil || rid == "" {
			writeJSON(w, 404, map[string]any{"error": "release_not_found"})
			return
		}
		if strOf(current["draft"]) != "true" {
			writeJSON(w, 409, map[string]any{"error": "published_assets_immutable"})
			return
		}
		assets, err := releaseAssets(current)
		if err != nil {
			break
		}
		found := false
		for i := range assets {
			if assets[i].ID == id {
				if assets[i].State == "pending" {
					writeJSON(w, 409, map[string]any{"error": "retry_pending_upload_before_deletion"})
					return
				}
				assets[i].State = "deleting"
				found = true
				break
			}
		}
		if !found {
			writeJSON(w, 200, map[string]any{"deleted": true})
			return
		}
		raw, _ := json.Marshal(assets)
		err = a.Trestle.PatchRecord("releases", rid, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
		if err == nil {
			reserved = true
			break
		}
		if draftPersistenceStatus(err) != 409 {
			break
		}
	}
	if !reserved {
		writeJSON(w, 409, map[string]any{"error": "asset_deletion_reservation_failed"})
		return
	}
	if err := store.DeleteReleaseAsset(r.Context(), repo, id); err != nil {
		writeJSON(w, 502, map[string]any{"error": "asset_deletion_pending", "retryable": true})
		return
	}
	for tries := 0; tries < 8; tries++ {
		rid, version, current, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if err != nil || rid == "" {
			break
		}
		assets, err := releaseAssets(current)
		if err != nil {
			break
		}
		kept := []releaseAsset{}
		for _, asset := range assets {
			if asset.ID != id {
				kept = append(kept, asset)
			}
		}
		raw, _ := json.Marshal(kept)
		err = a.Trestle.PatchRecord("releases", rid, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
		if err == nil {
			writeJSON(w, 200, map[string]any{"deleted": true})
			return
		}
		if draftPersistenceStatus(err) != 409 {
			break
		}
	}
	writeJSON(w, 502, map[string]any{"error": "asset_deletion_pending", "retryable": true})
}

// handleReleaseAssetRecover completes an interrupted attachment from immutable R2
// bytes. It never deletes a pending object or assumes an upload stopped. A missing
// object still requires retrying the identical upload; a recovered ready asset can
// then be deleted through the normal draft lifecycle.
func (a *App) handleReleaseAssetRecover(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) || !a.CanRepository(meta, user, WriteRepo) {
		writeJSON(w, 403, map[string]any{"error": "release_maintainer_required"})
		return
	}
	store, ok := a.Actions.(releaseAssetStore)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "release_storage_unavailable"})
		return
	}
	identity := releaseIdentity(strOf(meta["id"]), r.URL.Query().Get("tag"))
	_, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	if err != nil || record == nil {
		writeJSON(w, 404, map[string]any{"error": "release_not_found"})
		return
	}
	assets, err := releaseAssets(record)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "assets_unavailable"})
		return
	}
	var selected *releaseAsset
	for i := range assets {
		if assets[i].ID == r.PathValue("asset") {
			selected = &assets[i]
			break
		}
	}
	if selected == nil {
		writeJSON(w, 404, map[string]any{"error": "asset_not_found"})
		return
	}
	if selected.State == "ready" {
		writeJSON(w, 200, selected)
		return
	}
	if strOf(record["draft"]) != "true" || selected.State != "pending" {
		writeJSON(w, 409, map[string]any{"error": "asset_not_recoverable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	response, err := store.GetReleaseAsset(ctx, repo, selected.ID)
	if err != nil || response == nil || response.Body == nil {
		writeJSON(w, 502, map[string]any{"error": "asset_recovery_unavailable", "retryable": true})
		return
	}
	defer response.Body.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(response.Body, actions.ReleaseAssetLimit+1))
	if err != nil || size != selected.Size || hex.EncodeToString(hash.Sum(nil)) != selected.SHA256 {
		writeJSON(w, 502, map[string]any{"error": "asset_integrity_mismatch"})
		return
	}
	if err := a.audit(strOf(meta["owner_slug"]), "release.asset.recover", selected.ID, "allowed", "verified immutable R2 payload", user); err != nil {
		writeJSON(w, 502, map[string]any{"error": "release_audit_failed"})
		return
	}
	for tries := 0; tries < 8; tries++ {
		rid, version, current, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
		if err != nil || rid == "" {
			break
		}
		currentAssets, err := releaseAssets(current)
		if err != nil {
			break
		}
		found := false
		for i := range currentAssets {
			asset := &currentAssets[i]
			if asset.ID != selected.ID {
				continue
			}
			if asset.State == "ready" {
				writeJSON(w, 200, asset)
				return
			}
			if strOf(current["draft"]) != "true" || asset.State != "pending" || asset.SHA256 != selected.SHA256 || asset.Size != selected.Size {
				writeJSON(w, 409, map[string]any{"error": "asset_not_recoverable"})
				return
			}
			asset.State = "ready"
			found = true
		}
		if !found {
			writeJSON(w, 404, map[string]any{"error": "asset_not_found"})
			return
		}
		raw, _ := json.Marshal(currentAssets)
		err = a.Trestle.PatchRecord("releases", rid, version, map[string]any{"assets_json": string(raw), "updated_at": nowStr()})
		if err == nil {
			selected.State = "ready"
			writeJSON(w, 200, selected)
			return
		}
		if draftPersistenceStatus(err) != 409 {
			break
		}
	}
	writeJSON(w, 502, map[string]any{"error": "asset_attachment_pending", "retryable": true})
}
