package app

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

func (a *App) handleFollowUser(w http.ResponseWriter, r *http.Request) {
	viewer, target := a.currentUser(r), normalizeOwnerSlug(r.PathValue("username"))
	if viewer == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	if viewer == target {
		writeJSON(w, 400, map[string]any{"error": "cannot_follow_yourself"})
		return
	}
	if a.userRecord(target) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	key := sha256Hex([]byte(viewer + "\x00" + target))
	rid, ver, _, err := a.Trestle.FindRecord("user_follows", filterEq("id", key))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "connections_unavailable"})
		return
	}
	if r.Method == "PUT" && rid == "" {
		_, _, err = a.Trestle.CreateRecord("user_follows", map[string]any{"id": key, "follower": viewer, "following": target, "created_at": nowStr()}, "follow-"+key)
		if err != nil { // A simultaneous identical request may have created the unique relationship.
			existing, _, _, lookupErr := a.Trestle.FindRecord("user_follows", filterEq("id", key))
			if lookupErr == nil && existing != "" {
				err = nil
			}
		}
	} else if r.Method == "DELETE" && rid != "" {
		err = a.Trestle.DeleteRecord("user_follows", rid, ver)
	}
	if err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "follow_update_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"following": r.Method == "PUT"})
}

func (a *App) handleUserConnections(w http.ResponseWriter, r *http.Request) {
	target := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(target) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	followers, err := a.Trestle.ListRecords("user_follows", filterEq("following", target))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "connections_unavailable"})
		return
	}
	following, err := a.Trestle.ListRecords("user_follows", filterEq("follower", target))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "connections_unavailable"})
		return
	}
	viewerFollows := false
	for _, f := range followers {
		if strOf(f["follower"]) == a.currentUser(r) {
			viewerFollows = true
		}
	}
	kind := r.URL.Query().Get("kind")
	records, field := followers, "follower"
	if kind == "following" {
		records, field = following, "following"
	}
	sort.SliceStable(records, func(i, j int) bool { return strOf(records[i][field]) < strOf(records[j][field]) })
	limit, offset := activityPageBounds(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
	total := len(records)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	items := []map[string]any{}
	for _, f := range records[offset:end] {
		username := strOf(f[field])
		items = append(items, map[string]any{"username": username, "avatar_url": "/api/avatars/user/" + username})
	}
	writeJSON(w, 200, map[string]any{"items": items, "followers": len(followers), "following": len(following), "viewer_follows": viewerFollows, "offset": offset, "total": total, "has_more": end < total})
}

func (a *App) handleUserOrganizations(w http.ResponseWriter, r *http.Request) {
	target := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(target) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	orgs, err := a.Trestle.ListRecords("orgs", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "organizations_unavailable"})
		return
	}
	items := []map[string]any{}
	for _, org := range orgs {
		if a.orgRole(org, target) == "" {
			continue
		}
		p := a.orgProfile(org)
		if strOf(p["visibility"]) != "public" && a.orgRole(org, a.currentUser(r)) == "" {
			continue
		}
		items = append(items, map[string]any{"slug": p["slug"], "name": p["display_name"], "avatar_url": p["avatar_url"]})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func attributedRef(provenance, username string) bool {
	fields := strings.Fields(provenance)
	if len(fields) == 0 {
		return false
	}
	parts := strings.Split(fields[0], ":")
	if len(parts) < 2 {
		return false
	}
	if parts[0] == "resolve" {
		parts = parts[1:]
	}
	if len(parts) < 2 {
		return false
	}
	switch parts[0] {
	case "user", "editor", "browser-file-operation", "escalation":
		return parts[1] == username
	}
	return false
}
func (a *App) handleUserContributions(w http.ResponseWriter, r *http.Request) {
	target := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(target) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	start := today.AddDate(-1, 0, 1)
	days := map[string]int{}
	events := []map[string]any{}
	add := func(at, kind, title, url string) {
		stamp, err := time.Parse(time.RFC3339, at)
		if err != nil || stamp.Before(start) || stamp.After(today.Add(24*time.Hour)) {
			return
		}
		day := stamp.UTC().Format("2006-01-02")
		days[day]++
		events = append(events, map[string]any{"at": at, "type": kind, "title": title, "url": url})
	}
	records, err := a.Trestle.ListRecords("ref_updates", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "contributions_unavailable"})
		return
	}
	seen := map[string]bool{}
	// Request-local memoization keeps permission checks fresh across requests.
	// Reject unrelated/out-of-window events before remote metadata reads.
	metadata := map[string]map[string]any{}
	allowed := map[string]bool{}
	for _, record := range records {
		if !attributedRef(strOf(record["provenance"]), target) {
			continue
		}
		sha := strOf(record["new_sha"])
		identity := strOf(record["repo"]) + ":" + sha
		if sha == "" || strings.Trim(sha, "0") == "" || seen[identity] {
			continue
		}

		stamp, parseErr := time.Parse(time.RFC3339, strOf(record["occurred_at"]))
		if parseErr != nil || stamp.Before(start) || !stamp.Before(today.Add(24*time.Hour)) {
			continue
		}
		repo := strOf(record["repo"])
		meta, loaded := metadata[repo]
		if !loaded {
			meta = a.repositoryMetaByArtifact(repo)
			metadata[repo] = meta
			allowed[repo] = a.CanRepository(meta, a.currentUser(r), ReadRepo)
		}
		if !allowed[repo] {
			continue
		}
		seen[identity] = true
		add(strOf(record["occurred_at"]), "commit", "Committed to "+strOf(meta["full_name"]), "/"+strOf(meta["owner_slug"])+"/"+strOf(meta["slug"]))
	}
	work, err := a.Trestle.ListRecords("work", filterEq("owner", target))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "contributions_unavailable"})
		return
	}
	for _, record := range a.visibleRecords("work", work, a.currentUser(r)) {
		add(strOf(record["created_at"]), "work", "Created "+strOf(record["title"]), "/work/"+strOf(record["id"]))
	}
	sort.SliceStable(events, func(i, j int) bool { return strOf(events[i]["at"]) > strOf(events[j]["at"]) })
	total := 0
	for _, count := range days {
		total += count
	}
	limit, offset := activityPageBounds(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
	if offset > len(events) {
		offset = len(events)
	}
	end := offset + limit
	if end > len(events) {
		end = len(events)
	}
	writeJSON(w, 200, map[string]any{"days": days, "total": total, "start": start.Format("2006-01-02"), "end": today.Format("2006-01-02"), "items": events[offset:end], "offset": offset, "has_more": end < len(events), "definition": "Recorded commits and new Work items in repositories visible to you; imported Git history is not counted."})
}
