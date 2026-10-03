package app

import (
	"net/http"
	"strings"
	"switchyard/internal/trestle"
)

func repositorySettingsCollections() [][2]any {
	return [][2]any{
		{"repository_settings", []trestle.CollectionField{{Name: "repo_id", Type: "text", Unique: true}, {Name: "archived", Type: "text"}, {Name: "integration_policy", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"repo_collaborators", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "repo_id", Type: "text"}, {Name: "username", Type: "text"}, {Name: "permission", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"repo_protected_refs", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "repo_id", Type: "text"}, {Name: "pattern", Type: "text"}, {Name: "require_pr", Type: "text"}, {Name: "require_queue", Type: "text"}, {Name: "created_at", Type: "text"}}},
	}
}

func (a *App) canAdminRepository(meta map[string]any, user string) bool {
	return a.CanRepository(meta, user, AdminRepo)
}

func (a *App) resolveRepositoryAdmin(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return nil, false
	}
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	if meta == nil {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return nil, false
	}
	if !a.canAdminRepository(meta, u) {
		writeJSON(w, 403, map[string]any{"error": "repository_admin_required"})
		return nil, false
	}
	return meta, true
}

func (a *App) handleRepositorySettings(w http.ResponseWriter, r *http.Request) {
	meta, ok := a.resolveRepositoryAdmin(w, r)
	if !ok {
		return
	}
	settings := map[string]any{"archived": "false", "integration_policy": "queue"}
	if xs, _ := a.Trestle.ListRecords("repository_settings", `repo_id = "`+strOr(meta["id"])+`"`); len(xs) > 0 {
		for k, v := range xs[0] {
			settings[k] = v
		}
	}
	collabs, _ := a.Trestle.ListRecords("repo_collaborators", `repo_id = "`+strOr(meta["id"])+`"`)
	protected, _ := a.Trestle.ListRecords("repo_protected_refs", `repo_id = "`+strOr(meta["id"])+`"`)
	writeJSON(w, 200, map[string]any{"repository": meta, "settings": settings, "collaborators": collabs, "protected_refs": protected})
}

func (a *App) handleUpdateRepositorySettings(w http.ResponseWriter, r *http.Request) {
	meta, ok := a.resolveRepositoryAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Description       string `json:"description"`
		Visibility        string `json:"visibility"`
		DefaultBranch     string `json:"default_branch"`
		Slug              string `json:"slug"`
		Archived          *bool  `json:"archived"`
		IntegrationPolicy string `json:"integration_policy"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	rid, ver, _, e := a.Trestle.FindRecord("repository_meta", `id = "`+strOr(meta["id"])+`"`)
	if e != nil || rid == "" {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	patch := map[string]any{"updated_at": nowStr()}
	if in.Description != "" || r.URL.Query().Get("allow_empty_description") == "1" {
		patch["description"] = strings.TrimSpace(in.Description)
	}
	if in.Visibility != "" {
		if in.Visibility != "public" && in.Visibility != "private" && in.Visibility != "internal" {
			writeJSON(w, 400, map[string]any{"error": "visibility_invalid"})
			return
		}
		patch["visibility"] = in.Visibility
	}
	if in.DefaultBranch != "" {
		patch["default_branch"] = strings.TrimSpace(in.DefaultBranch)
	}
	if in.Slug != "" && normalizeRepoSlug(in.Slug) != strOr(meta["slug"]) {
		slug := normalizeRepoSlug(in.Slug)
		if !validRepoSlug(slug) {
			writeJSON(w, 400, map[string]any{"error": "repo_slug_invalid"})
			return
		}
		if x := a.repositoryMetaByOwner(strOr(meta["owner_slug"]), slug); x != nil {
			writeJSON(w, 409, map[string]any{"error": "repository_name_taken"})
			return
		}
		patch["slug"] = slug
		patch["display_name"] = slug
		patch["full_name"] = strOr(meta["owner_slug"]) + "/" + slug
	}
	if err := a.Trestle.PatchRecord("repository_meta", rid, ver, patch); err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	if in.Archived != nil || in.IntegrationPolicy != "" {
		sid, sver, _, _ := a.Trestle.FindRecord("repository_settings", `repo_id = "`+strOr(meta["id"])+`"`)
		sp := map[string]any{"updated_at": nowStr()}
		if in.Archived != nil {
			if *in.Archived {
				sp["archived"] = "true"
			} else {
				sp["archived"] = "false"
			}
		}
		if in.IntegrationPolicy != "" {
			if in.IntegrationPolicy != "queue" && in.IntegrationPolicy != "direct" && in.IntegrationPolicy != "pr" {
				writeJSON(w, 400, map[string]any{"error": "integration_policy_invalid"})
				return
			}
			sp["integration_policy"] = in.IntegrationPolicy
		}
		if sid == "" {
			sp["repo_id"] = meta["id"]
			_, _, _ = a.Trestle.CreateRecord("repository_settings", sp, "repo-settings-"+strOr(meta["id"]))
		} else {
			_ = a.Trestle.PatchRecord("repository_settings", sid, sver, sp)
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "repository": patch})
}

func (a *App) handleAddRepositoryCollaborator(w http.ResponseWriter, r *http.Request) {
	meta, ok := a.resolveRepositoryAdmin(w, r)
	if !ok {
		return
	}
	if strOr(meta["owner_type"]) != "user" {
		writeJSON(w, 409, map[string]any{"error": "org_repositories_use_org_access"})
		return
	}
	var in struct {
		Username   string `json:"username"`
		Permission string `json:"permission"`
	}
	if readJSON(r, &in) != nil || a.userRecord(in.Username) == nil {
		writeJSON(w, 400, map[string]any{"error": "valid_username_required"})
		return
	}
	if in.Permission != "read" && in.Permission != "write" && in.Permission != "admin" {
		writeJSON(w, 400, map[string]any{"error": "permission_invalid"})
		return
	}
	id := "rc_" + randHex(9)
	vals := map[string]any{"id": id, "repo_id": meta["id"], "username": in.Username, "permission": in.Permission, "created_at": nowStr()}
	_, _, e := a.Trestle.CreateRecord("repo_collaborators", vals, "repo-collab-"+strOr(meta["id"])+"-"+in.Username)
	if e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, vals)
}

func (a *App) handleAddProtectedRef(w http.ResponseWriter, r *http.Request) {
	meta, ok := a.resolveRepositoryAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Pattern      string `json:"pattern"`
		RequirePR    bool   `json:"require_pr"`
		RequireQueue bool   `json:"require_queue"`
	}
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Pattern) == "" {
		writeJSON(w, 400, map[string]any{"error": "pattern_required"})
		return
	}
	id := "prot_" + randHex(9)
	vals := map[string]any{"id": id, "repo_id": meta["id"], "pattern": strings.TrimSpace(in.Pattern), "require_pr": boolText(in.RequirePR), "require_queue": boolText(in.RequireQueue), "created_at": nowStr()}
	_, _, e := a.Trestle.CreateRecord("repo_protected_refs", vals, "repo-protected-"+id)
	if e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, vals)
}
func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
