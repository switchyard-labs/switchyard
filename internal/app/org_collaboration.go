package app

import (
	"net/http"
	"strings"

	"switchyard/internal/trestle"
)

func collaborationCollections() [][2]any {
	return [][2]any{
		{"org_memberships", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "org_id", Type: "text"}, {Name: "username", Type: "text"}, {Name: "role", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"org_invitations", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "org_id", Type: "text"}, {Name: "username", Type: "text"}, {Name: "role", Type: "text"}, {Name: "status", Type: "text"}, {Name: "invited_by", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "decided_at", Type: "text"}}},
		{"org_teams", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "org_id", Type: "text"}, {Name: "slug", Type: "text"}, {Name: "name", Type: "text"}, {Name: "description", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"org_team_members", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "org_id", Type: "text"}, {Name: "team_id", Type: "text"}, {Name: "username", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"org_repo_access", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "org_id", Type: "text"}, {Name: "repo_id", Type: "text"}, {Name: "subject_type", Type: "text"}, {Name: "subject_id", Type: "text"}, {Name: "permission", Type: "text"}, {Name: "created_at", Type: "text"}}},
	}
}

func (a *App) orgRole(org map[string]any, username string) string {
	if org == nil || username == "" {
		return ""
	}
	if strOr(org["owner"]) == username {
		return "owner"
	}
	id := strOr(org["id"])
	it, e := a.Trestle.ListRecords("org_memberships", `org_id = "`+id+`"`)
	if e == nil {
		for _, m := range it {
			if strOr(m["username"]) == username {
				return strOr(m["role"])
			}
		}
	}
	// compatibility with CP12's old members array.
	if ms, ok := org["members"].([]any); ok {
		for _, m := range ms {
			if strOr(m) == username {
				return "member"
			}
		}
	}
	return ""
}
func roleRank(r string) int {
	switch r {
	case "owner":
		return 3
	case "admin":
		return 2
	case "member":
		return 1
	}
	return 0
}
func (a *App) requireOrgRole(w http.ResponseWriter, r *http.Request, key string, min string) (map[string]any, bool) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return nil, false
	}
	o := a.orgBySlugOrID(key)
	if o == nil {
		writeJSON(w, 404, map[string]any{"error": "org_not_found"})
		return nil, false
	}
	if roleRank(a.orgRole(o, u)) < roleRank(min) {
		writeJSON(w, 403, map[string]any{"error": "org_permission_denied"})
		return nil, false
	}
	return o, true
}

func (a *App) handleOrgMembers(w http.ResponseWriter, r *http.Request) {
	o := a.orgBySlugOrID(r.PathValue("id"))
	if o == nil {
		writeJSON(w, 404, map[string]any{"error": "org_not_found"})
		return
	}
	id := strOr(o["id"])
	out := []map[string]any{{"username": o["owner"], "role": "owner", "avatar_url": "/api/avatars/user/" + strOr(o["owner"])}}
	it, _ := a.Trestle.ListRecords("org_memberships", `org_id = "`+id+`"`)
	for _, m := range it {
		if strOr(m["username"]) == strOr(o["owner"]) {
			continue
		}
		out = append(out, map[string]any{"username": m["username"], "role": m["role"], "avatar_url": "/api/avatars/user/" + strOr(m["username"])})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}
func (a *App) handleInviteOrgMember(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok {
		return
	}
	var in struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if readJSON(r, &in) != nil || a.userRecord(in.Username) == nil {
		writeJSON(w, 400, map[string]any{"error": "valid_username_required"})
		return
	}
	role := strings.ToLower(in.Role)
	if role == "" {
		role = "member"
	}
	if role != "member" && role != "admin" {
		writeJSON(w, 400, map[string]any{"error": "role_invalid"})
		return
	}
	id := "inv_" + randHex(10)
	vals := map[string]any{"id": id, "org_id": o["id"], "username": in.Username, "role": role, "status": "pending", "invited_by": a.currentUser(r), "created_at": nowStr(), "decided_at": ""}
	_, _, e := a.Trestle.CreateRecord("org_invitations", vals, "org-invite-"+id)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, vals)
}
func (a *App) handleListOrgInvitations(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok {
		return
	}
	it, e := a.Trestle.ListRecords("org_invitations", `org_id = "`+strOr(o["id"])+`"`)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": it})
}
func (a *App) handleDecideOrgInvitation(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	rid, ver, v, e := a.Trestle.FindRecord("org_invitations", `id = "`+r.PathValue("id")+`"`)
	if e != nil || rid == "" || strOr(v["username"]) != u || strOr(v["status"]) != "pending" {
		writeJSON(w, 404, map[string]any{"error": "invitation_not_found"})
		return
	}
	var in struct {
		Decision string `json:"decision"`
	}
	_ = readJSON(r, &in)
	if in.Decision != "accept" && in.Decision != "decline" {
		writeJSON(w, 400, map[string]any{"error": "decision_invalid"})
		return
	}
	_ = a.Trestle.PatchRecord("org_invitations", rid, ver, map[string]any{"status": in.Decision + "ed", "decided_at": nowStr()})
	if in.Decision == "accept" {
		mid := "mem_" + randHex(10)
		_, _, _ = a.Trestle.CreateRecord("org_memberships", map[string]any{"id": mid, "org_id": v["org_id"], "username": u, "role": v["role"], "created_at": nowStr()}, "org-membership-"+strOr(v["org_id"])+"-"+u)
	}
	writeJSON(w, 200, map[string]any{"status": in.Decision + "ed"})
}
func (a *App) handleRemoveOrgMember(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok {
		return
	}
	u := r.PathValue("username")
	if u == strOr(o["owner"]) {
		writeJSON(w, 409, map[string]any{"error": "cannot_remove_primary_owner"})
		return
	}
	rid, ver := "", ""
	if items, e := a.Trestle.ListRecords("org_memberships", `org_id = "`+strOr(o["id"])+`"`); e == nil {
		for _, m := range items {
			if strOr(m["username"]) == u {
				rid, ver, _, _ = a.Trestle.FindRecord("org_memberships", `id = "`+strOr(m["id"])+`"`)
				break
			}
		}
	}
	if rid == "" {
		writeJSON(w, 404, map[string]any{"error": "member_not_found"})
		return
	}
	if e := a.Trestle.DeleteRecord("org_memberships", rid, ver); e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (a *App) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok {
		return
	}
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Name) == "" {
		writeJSON(w, 400, map[string]any{"error": "name_required"})
		return
	}
	slug := normalizeOwnerSlug(strings.ReplaceAll(in.Name, " ", "-"))
	if !ownerSlugRE.MatchString(slug) {
		writeJSON(w, 400, map[string]any{"error": "team_slug_invalid"})
		return
	}
	id := "team_" + randHex(9)
	vals := map[string]any{"id": id, "org_id": o["id"], "slug": slug, "name": in.Name, "description": in.Description, "created_at": nowStr()}
	_, _, e := a.Trestle.CreateRecord("org_teams", vals, "org-team-"+strOr(o["id"])+"-"+slug)
	if e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, vals)
}
func (a *App) handleListTeams(w http.ResponseWriter, r *http.Request) {
	o := a.orgBySlugOrID(r.PathValue("id"))
	if o == nil {
		writeJSON(w, 404, map[string]any{"error": "org_not_found"})
		return
	}
	it, e := a.Trestle.ListRecords("org_teams", `org_id = "`+strOr(o["id"])+`"`)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": it})
}
func (a *App) handleAddTeamMember(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok {
		return
	}
	var in struct {
		Username string `json:"username"`
	}
	if readJSON(r, &in) != nil || a.orgRole(o, in.Username) == "" {
		writeJSON(w, 400, map[string]any{"error": "org_member_required"})
		return
	}
	tid := r.PathValue("team")
	id := "tm_" + randHex(9)
	_, _, e := a.Trestle.CreateRecord("org_team_members", map[string]any{"id": id, "org_id": o["id"], "team_id": tid, "username": in.Username, "created_at": nowStr()}, "team-member-"+tid+"-"+in.Username)
	if e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"ok": true})
}
func (a *App) handleSetRepoAccess(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok {
		return
	}
	meta := a.repositoryMetaByOwner(normalizeOwnerSlug(strOr(o["name"])), r.PathValue("repo"))
	if meta == nil {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	var in struct {
		SubjectType string `json:"subject_type"`
		SubjectID   string `json:"subject_id"`
		Permission  string `json:"permission"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if in.Permission != "read" && in.Permission != "write" && in.Permission != "admin" {
		writeJSON(w, 400, map[string]any{"error": "permission_invalid"})
		return
	}
	id := "ra_" + randHex(9)
	_, _, e := a.Trestle.CreateRecord("org_repo_access", map[string]any{"id": id, "org_id": o["id"], "repo_id": meta["id"], "subject_type": in.SubjectType, "subject_id": in.SubjectID, "permission": in.Permission, "created_at": nowStr()}, "repo-access-"+strOr(meta["id"])+"-"+in.SubjectType+"-"+in.SubjectID)
	if e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"ok": true})
}

func (a *App) canAccessRepository(meta map[string]any, username string, write bool) bool {
	cap := ReadRepo
	if write {
		cap = WriteRepo
	}
	return a.CanRepository(meta, username, cap)
}

func (a *App) handleMyOrgInvitations(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	it, e := a.Trestle.ListRecords("org_invitations", `username = "`+u+`"`)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	out := []map[string]any{}
	for _, x := range it {
		if strOr(x["status"]) == "pending" {
			if o := a.orgBySlugOrID(strOr(x["org_id"])); o != nil {
				x["org_name"] = o["name"]
			}
			out = append(out, x)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}
func (a *App) handleListOrgPolicies(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "member")
	if !ok {
		return
	}
	it, e := a.Trestle.ListRecords("policies", `org = "`+strOr(o["id"])+`"`)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": it})
}
func (a *App) handleOrgAudit(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "admin")
	if !ok {
		return
	}
	it, e := a.Trestle.ListRecords("audit", `org = "`+strOr(o["id"])+`"`)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": it})
}
