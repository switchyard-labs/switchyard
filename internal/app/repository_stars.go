package app

import "net/http"

func (a *App) handleRepositoryStars(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	viewer := a.currentUser(r)
	if !a.CanRepository(meta, viewer, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	repoID := strOf(meta["id"])
	key := sha256Hex([]byte(repoID + "\x00" + viewer))
	if r.Method != "GET" {
		if viewer == "" || a.isDemoGuest(r) {
			writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
			return
		}
		rid, version, _, err := a.Trestle.FindRecord("repository_stars", filterEq("id", key))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "stars_unavailable"})
			return
		}
		if r.Method == "PUT" && rid == "" {
			_, _, err = a.Trestle.CreateRecord("repository_stars", map[string]any{"id": key, "repo_id": repoID, "username": viewer, "created_at": nowStr()}, "star-"+key)
			if err != nil {
				existing, _, _, lookupErr := a.Trestle.FindRecord("repository_stars", filterEq("id", key))
				if lookupErr == nil && existing != "" {
					err = nil
				}
			}
		} else if r.Method == "DELETE" && rid != "" {
			err = a.Trestle.DeleteRecord("repository_stars", rid, version)
		}
		if err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "star_update_failed"})
			return
		}
	}
	stars, err := a.Trestle.ListRecords("repository_stars", filterEq("repo_id", repoID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "stars_unavailable"})
		return
	}
	starred := false
	for _, star := range stars {
		if strOf(star["username"]) == viewer {
			starred = true
		}
	}
	writeJSON(w, 200, map[string]any{"count": len(stars), "starred": starred})
}
func (a *App) handleUserStars(w http.ResponseWriter, r *http.Request) {
	target := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(target) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	stars, err := a.Trestle.ListRecords("repository_stars", filterEq("username", target))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "stars_unavailable"})
		return
	}
	repos, err := a.Trestle.ListRecords("repository_meta", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "repositories_unavailable"})
		return
	}
	ids := map[string]bool{}
	for _, star := range stars {
		ids[strOf(star["repo_id"])] = true
	}
	items := []map[string]any{}
	for _, repo := range a.visibleRecords("repository_meta", repos, a.currentUser(r)) {
		if ids[strOf(repo["id"])] {
			items = append(items, repo)
		}
	}
	limit, offset := activityPageBounds(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
	if offset > len(items) {
		offset = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	writeJSON(w, 200, map[string]any{"items": items[offset:end], "total": len(items), "offset": offset, "has_more": end < len(items)})
}
