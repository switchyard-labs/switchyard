package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"switchyard/internal/artifacts"
	"switchyard/internal/trestle"
)

func releaseCollections() [][2]any {
	fields := []trestle.CollectionField{}
	for _, name := range []string{"id", "identity", "repo_id", "tag", "target_sha", "title", "body", "draft", "prerelease", "author", "created_at", "updated_at", "published_at", "assets_json"} {
		fields = append(fields, trestle.CollectionField{Name: name, Type: "text", Unique: name == "id" || name == "identity"})
	}
	assets := []trestle.CollectionField{}
	for _, name := range []string{"id", "identity", "release_id", "name", "size", "content_type", "sha256", "storage_key", "created_at"} {
		assets = append(assets, trestle.CollectionField{Name: name, Type: "text", Unique: name == "id" || name == "identity"})
	}
	return [][2]any{{"releases", fields}, {"release_assets", assets}}
}

func validReleaseTag(tag string) bool {
	for _, char := range tag {
		if char < 32 || char == 127 {
			return false
		}
	}
	if tag == "" || len(tag) > 200 || strings.HasPrefix(tag, "-") || strings.HasPrefix(tag, "/") || strings.HasSuffix(tag, "/") || strings.HasSuffix(tag, ".") || strings.ContainsAny(tag, " ~^:?*[\\\x00\r\n\t") || strings.Contains(tag, "..") || strings.Contains(tag, "@{") || strings.Contains(tag, "//") {
		return false
	}
	for _, part := range strings.Split(tag, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

func releaseIdentity(repo, tag string) string { return sha256Hex([]byte(repo + "\x00" + tag)) }

func (a *App) releaseView(meta, record map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"id", "tag", "target_sha", "title", "body", "draft", "prerelease", "author", "created_at", "updated_at", "published_at"} {
		out[key] = record[key]
	}
	base := "/" + url.PathEscape(strOf(meta["owner_slug"])) + "/" + url.PathEscape(strOf(meta["slug"]))
	assets, err := releaseAssets(record)
	if err == nil {
		out["assets"] = assets
	}
	out["source_zip"] = base + "/archive/" + strOf(record["target_sha"]) + ".zip"
	out["source_tar_gz"] = base + "/archive/" + strOf(record["target_sha"]) + ".tar.gz"
	return out
}

func (a *App) handleReleases(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	canWrite := a.currentUser(r) != "" && !a.isDemoGuest(r) && a.CanRepository(meta, a.currentUser(r), WriteRepo)
	if r.Method == "GET" {
		rows, err := a.Trestle.ListRecords("releases", filterEq("repo_id", strOf(meta["id"])))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "releases_unavailable"})
			return
		}
		items := []map[string]any{}
		for _, row := range rows {
			if strOf(row["draft"]) == "true" && !canWrite {
				continue
			}
			items = append(items, a.releaseView(meta, row))
		}
		sort.Slice(items, func(i, j int) bool { return strOf(items[i]["created_at"]) > strOf(items[j]["created_at"]) })
		writeJSON(w, 200, map[string]any{"items": items, "can_write": canWrite})
		return
	}
	if !canWrite {
		writeJSON(w, 403, map[string]any{"error": "release_maintainer_required"})
		return
	}
	var in struct {
		Tag        string `json:"tag"`
		Title      string `json:"title"`
		Body       string `json:"body"`
		Prerelease bool   `json:"prerelease"`
	}
	if readJSON(r, &in) != nil || !validReleaseTag(in.Tag) || len(in.Title) > 300 || len(in.Body) > 65536 {
		writeJSON(w, 400, map[string]any{"error": "invalid_release"})
		return
	}
	// Existing Git tags are authoritative. Resolve once and retain the commit
	// identity independently of subsequent tag movement.
	commits, err := a.releaseTagCommit(r.Context(), repo, in.Tag)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	if len(commits) != 1 || !workspaceSHA.MatchString(commits[0].Hash) {
		writeJSON(w, 404, map[string]any{"error": "release_tag_not_found"})
		return
	}
	identity := releaseIdentity(strOf(meta["id"]), in.Tag)
	rid, _, _, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "releases_unavailable"})
		return
	}
	if rid != "" {
		writeJSON(w, 409, map[string]any{"error": "release_tag_exists"})
		return
	}
	if in.Title == "" {
		in.Title = in.Tag
	}
	record := map[string]any{"id": "rel_" + randHex(12), "identity": identity, "repo_id": meta["id"], "tag": in.Tag, "target_sha": commits[0].Hash, "title": in.Title, "body": in.Body, "draft": "true", "prerelease": fmt.Sprint(in.Prerelease), "author": a.currentUser(r), "created_at": nowStr(), "updated_at": nowStr(), "published_at": ""}
	if err = a.audit(strOf(meta["owner_slug"]), "release.create", strOf(record["id"]), "allowed", "draft creation intent", a.currentUser(r)); err != nil {
		writeJSON(w, 502, map[string]any{"error": "release_audit_failed"})
		return
	}
	if _, _, err = a.Trestle.CreateRecord("releases", record, "release-"+identity); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "release_create_failed"})
		return
	}
	writeJSON(w, 201, a.releaseView(meta, record))
}

func (a *App) handleRelease(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	canWrite := a.currentUser(r) != "" && !a.isDemoGuest(r) && a.CanRepository(meta, a.currentUser(r), WriteRepo)
	identity := releaseIdentity(strOf(meta["id"]), r.PathValue("tag"))
	rid, version, record, err := a.Trestle.FindRecord("releases", filterEq("identity", identity))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "releases_unavailable"})
		return
	}
	if rid == "" || (strOf(record["draft"]) == "true" && !canWrite) {
		writeJSON(w, 404, map[string]any{"error": "release_not_found"})
		return
	}
	if r.Method == "GET" {
		writeJSON(w, 200, a.releaseView(meta, record))
		return
	}
	if !canWrite {
		writeJSON(w, 403, map[string]any{"error": "release_maintainer_required"})
		return
	}
	if r.Method == "DELETE" {
		assets, e := releaseAssets(record)
		if e != nil || len(assets) > 0 {
			writeJSON(w, 409, map[string]any{"error": "release_assets_must_be_deleted"})
			return
		}
	}
	if r.Method == "DELETE" && strOf(record["draft"]) != "true" {
		writeJSON(w, 409, map[string]any{"error": "published_release_cannot_be_deleted"})
		return
	}
	action := "release.edit"
	patch := map[string]any{"updated_at": nowStr()}
	if r.Method == "DELETE" {
		action = "release.delete_draft"
	} else {
		var in struct {
			Title      *string `json:"title"`
			Body       *string `json:"body"`
			Prerelease *bool   `json:"prerelease"`
			Publish    bool    `json:"publish"`
		}
		if readJSON(r, &in) != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid_release"})
			return
		}
		if in.Title != nil {
			if len(*in.Title) > 300 {
				writeJSON(w, 400, map[string]any{"error": "invalid_release_title"})
				return
			}
			patch["title"] = *in.Title
		}
		if in.Body != nil {
			if len(*in.Body) > 65536 {
				writeJSON(w, 400, map[string]any{"error": "invalid_release_body"})
				return
			}
			patch["body"] = *in.Body
		}
		if in.Prerelease != nil {
			patch["prerelease"] = fmt.Sprint(*in.Prerelease)
		}
		if in.Publish && strOf(record["draft"]) == "true" {
			if !releaseAssetsReady(record) {
				writeJSON(w, 409, map[string]any{"error": "release_assets_pending"})
				return
			}
			commits, e := a.releaseTagCommit(r.Context(), repo, strOf(record["tag"]))
			if e != nil {
				writeArtifactsError(w, e)
				return
			}
			if len(commits) != 1 || commits[0].Hash != strOf(record["target_sha"]) {
				writeJSON(w, 409, map[string]any{"error": "release_tag_moved"})
				return
			}
			patch["draft"] = "false"
			patch["published_at"] = nowStr()
			action = "release.publish"
		}
	}
	if err = a.audit(strOf(meta["owner_slug"]), action, strOf(record["id"]), "allowed", "release mutation intent", a.currentUser(r)); err != nil {
		writeJSON(w, 502, map[string]any{"error": "release_audit_failed"})
		return
	}
	if r.Method == "DELETE" {
		err = a.Trestle.DeleteRecord("releases", rid, version)
	} else {
		err = a.Trestle.PatchRecord("releases", rid, version, patch)
	}
	if err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "release_update_failed"})
		return
	}
	if r.Method == "DELETE" {
		writeJSON(w, 200, map[string]any{"deleted": true})
		return
	}
	for key, value := range patch {
		record[key] = value
	}
	writeJSON(w, 200, a.releaseView(meta, record))
}

// The Artifacts log API accepts a short ref, which can collide with a branch.
// Read the exact Git tag ref first, peel annotated tags, then request its SHA.
func (a *App) releaseTagCommit(ctx context.Context, repo, tag string) ([]artifacts.Commit, error) {
	backend, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		return nil, err
	}
	token, err := a.Refs.GitToken(repo)
	if err != nil {
		return nil, err
	}
	refs, err := a.Artifacts.LsRemoteWithToken(backend.Remote, token)
	if err != nil {
		return nil, err
	}
	sha := releaseTagSHA(refs, tag)
	if !workspaceSHA.MatchString(sha) {
		return nil, nil
	}
	return a.Artifacts.LogContext(ctx, repo, sha)
}
func releaseTagSHA(refs map[string]string, tag string) string {
	ref := "refs/tags/" + tag
	if refs[ref] == "" {
		return ""
	}
	if peeled := refs[ref+"^{}"]; peeled != "" {
		return peeled
	}
	return refs[ref]
}
