package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type RepoCapability int

const (
	ReadRepo RepoCapability = iota
	WriteRepo
	AdminRepo
	RunAgent
	IntegrateRepo
)

func filterEq(field, value string) string { return field + " = " + strconv.Quote(value) }
func permissionAllows(permission string, cap RepoCapability) bool {
	switch permission {
	case "admin":
		return true
	case "write":
		return cap != AdminRepo
	case "read":
		return cap == ReadRepo
	}
	return false
}

// CanRepository is the single grant interpreter used by canonical and legacy
// HTTP surfaces. Unknown repositories, failed lookups and unknown grants deny.
func (a *App) CanRepository(meta map[string]any, user string, cap RepoCapability) bool {
	if meta == nil {
		return false
	}
	if cap == ReadRepo && strOr(meta["visibility"]) == "public" {
		return true
	}
	if user == "" || user == "demo" {
		return false
	}
	if strOr(meta["owner_type"]) == "user" && strOr(meta["owner_id"]) == user {
		return true
	}
	org := strOr(meta["owner_id"])
	if strOr(meta["owner_type"]) == "org" {
		role := a.orgRole(a.orgBySlugOrID(org), user)
		if role == "owner" || role == "admin" {
			return true
		}
		if cap == ReadRepo && role == "member" && strOr(meta["visibility"]) == "internal" {
			return true
		}
	}
	id := strOr(meta["id"])
	if id == "" || a.Trestle == nil {
		return false
	}
	grants, err := a.Trestle.ListRecords("repo_collaborators", filterEq("repo_id", id))
	if err == nil {
		for _, g := range grants {
			if strOr(g["username"]) == user && permissionAllows(strOr(g["permission"]), cap) {
				return true
			}
		}
	}
	if strOr(meta["owner_type"]) != "org" {
		return false
	}
	grants, err = a.Trestle.ListRecords("org_repo_access", filterEq("repo_id", id))
	if err != nil {
		return false
	}
	for _, g := range grants {
		if strOr(g["org_id"]) != org || !permissionAllows(strOr(g["permission"]), cap) {
			continue
		}
		if strOr(g["subject_type"]) == "user" && strOr(g["subject_id"]) == user {
			return true
		}
		if strOr(g["subject_type"]) == "team" {
			members, e := a.Trestle.ListRecords("org_team_members", filterEq("team_id", strOr(g["subject_id"])))
			if e == nil {
				for _, m := range members {
					if strOr(m["org_id"]) == org && strOr(m["username"]) == user {
						return true
					}
				}
			}
		}
	}
	return false
}
func (a *App) repoAccess(name, user string, cap RepoCapability) bool {
	var meta map[string]any
	if strings.Contains(name, "/") {
		p := strings.SplitN(name, "/", 2)
		meta = a.repositoryMetaByOwner(p[0], p[1])
	} else {
		meta = a.repositoryMetaByArtifact(name)
	}
	return a.CanRepository(meta, user, cap)
}
func (a *App) scopedRecord(collection, id string) map[string]any {
	if id == "" || a.Trestle == nil {
		return nil
	}
	_, _, record, err := a.Trestle.FindRecord(collection, filterEq("id", id))
	if err != nil {
		return nil
	}
	return record
}
func (a *App) recordAccess(collection string, x map[string]any, user string, cap RepoCapability) bool {
	if x == nil {
		return false
	}
	if collection == "audit" {
		org := a.orgBySlugOrID(strOr(x["org"]))
		role := a.orgRole(org, user)
		return role == "owner" || role == "admin"
	}
	if collection == "repository_meta" {
		return a.CanRepository(x, user, cap)
	}
	if collection == "repos" {
		return a.repoAccess(strOr(x["name"]), user, cap)
	}
	if collection == "drafts" && strOr(x["user"]) != user {
		return false
	}
	if collection == "work" {
		details, err := a.Trestle.ListRecords("work_details", filterEq("work_id", strOr(x["id"])))
		if err != nil {
			return false
		}
		if len(details) > 0 && strOr(details[0]["repo"]) != "" {
			return a.repoAccess(strOr(details[0]["repo"]), user, cap)
		}
		return user != "" && user != "demo" && strOr(x["owner"]) == user
	}
	for _, k := range []string{"repo", "repo_name"} {
		if name := strOr(x[k]); name != "" {
			return a.repoAccess(name, user, cap)
		}
	}
	if p, ok := x["params"].(map[string]any); ok {
		return strOr(p["_actor"]) == user && a.repoAccess(strOr(p["repo"]), user, cap)
	}
	if p, ok := x["packet"].(map[string]any); ok {
		if repo := strOr(p["repo"]); repo != "" {
			return a.repoAccess(repo, user, cap)
		}
		collection := map[string]string{"queue_blocked": "iq", "workflow_approval": "workflow_runs"}[strOr(p["target_kind"])]
		if collection != "" {
			return a.recordAccess(collection, a.scopedRecord(collection, strOr(p["target_id"])), user, cap)
		}
	}
	if collection == "attention" {
		c := map[string]string{"workflow_approval": "workflow_runs", "open_finding": "attempts"}[strOr(x["kind"])]
		if c != "" {
			return a.recordAccess(c, a.scopedRecord(c, strOr(x["target_id"])), user, cap)
		}
	}
	for key, c := range map[string]string{"work_id": "work", "attempt_id": "attempts", "pr_id": "prs", "target": "attempts"} {
		if id := strOr(x[key]); id != "" {
			return a.recordAccess(c, a.scopedRecord(c, id), user, cap)
		}
	}
	return false // unscoped operational records are not public account data
}
func (a *App) visibleRecords(collection string, items []map[string]any, user string) []map[string]any {
	out := []map[string]any{}
	for _, x := range items {
		if a.recordAccess(collection, x, user, ReadRepo) {
			out = append(out, x)
		}
	}
	return out
}

// authorizeHandler runs after ServeMux has resolved PathValues and before any
// effect. Every entity route resolves its own durable repository association.
func (a *App) authorizeHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" && !sameOrigin(r) {
			writeJSON(w, 403, map[string]any{"error": "cross_origin_request_denied"})
			return
		}
		user := a.currentUser(r)
		path := r.URL.Path
		write := r.Method != "GET" && r.Method != "HEAD"
		public := r.Method == "GET" && (path == "/api/repositories" || strings.HasPrefix(path, "/api/repositories/")) || strings.HasPrefix(path, "/api/auth/") || path == "/api/demo" || strings.HasPrefix(path, "/api/avatars/") || strings.HasPrefix(path, "/api/owners/") || strings.HasPrefix(path, "/api/users/")
		if user == "" && !public {
			writeJSON(w, 401, map[string]any{"error": "unauthorized"})
			return
		}
		if a.isDemoGuest(r) && !(path == "/api/demo" || path == "/api/auth/me" || strings.HasPrefix(path, "/api/repositories") || strings.HasPrefix(path, "/api/users/") || strings.HasPrefix(path, "/api/owners/") || strings.HasPrefix(path, "/api/avatars/")) {
			writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
			return
		}
		cap := ReadRepo
		if write {
			cap = WriteRepo
		}
		if strings.Contains(path, "/integrate") || strings.Contains(path, "/enqueue") || strings.Contains(path, "/requeue") {
			cap = IntegrateRepo
		}
		if strings.Contains(path, "/agent-propose") || strings.Contains(path, "/run") {
			if write {
				cap = RunAgent
			}
		}
		denied := false
		if path == "/api/repositories/register" {
			writeJSON(w, 403, map[string]any{"error": "repository_registration_requires_operator_import"})
			return
		}
		if strings.HasPrefix(path, "/api/workflows/") {
			owner := a.workflowOwner(r.PathValue("id"))
			denied = owner == "" || owner != user
		}

		if strings.HasPrefix(path, "/api/attention/") {
			collections := map[string]string{"attempt_conflict": "attempts", "queue_blocked": "iq", "workflow_approval": "workflow_runs", "open_finding": "attempts", "attempt": "attempts", "queue": "iq", "workflow": "workflow_runs"}
			c := collections[r.PathValue("kind")]
			denied = c == "" || !a.recordAccess(c, a.scopedRecord(c, r.PathValue("id")), user, cap)
		}
		if strings.HasPrefix(path, "/api/repositories/") && r.PathValue("owner") != "" {
			meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
			if strings.HasSuffix(path, "/stars") {
				cap = ReadRepo
			}
			if strings.HasSuffix(path, "/git-credential") {
				// The handler validates the requested scope before minting. A POST
				// for a read credential must not require repository write access.
				cap = ReadRepo
			}
			if strings.Contains(path, "/settings") || strings.Contains(path, "/collaborators") || strings.Contains(path, "/protected-refs") {
				cap = AdminRepo
			}
			denied = !a.CanRepository(meta, user, cap)
		}
		if strings.HasPrefix(path, "/api/repos/") {
			denied = !a.repoAccess(r.PathValue("name"), user, cap)
		}
		for prefix, collection := range map[string]string{"/api/work/": "work", "/api/attempts/": "attempts", "/api/prs/": "prs", "/api/queue/": "iq", "/api/workflow_runs/": "workflow_runs", "/api/escalations/": "escalations"} {
			if strings.HasPrefix(path, prefix) {
				denied = denied || !a.recordAccess(collection, a.scopedRecord(collection, r.PathValue("id")), user, cap)
			}
		}
		if strings.HasPrefix(path, "/api/drafts/") {
			if r.PathValue("id") != "" {
				denied = denied || !a.recordAccess("drafts", a.scopedRecord("drafts", r.PathValue("id")), user, cap)
			} else {
				denied = denied || !a.repoAccess(r.PathValue("repo"), user, cap)
			}
		}
		if write && !strings.HasPrefix(path, "/api/auth/") {
			body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
			if err != nil {
				writeJSON(w, 400, map[string]any{"error": "bad_request"})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			var in map[string]any
			_ = json.Unmarshal(body, &in)
			for _, key := range []string{"repo", "repo_name"} {
				if repo := strOr(in[key]); repo != "" {
					denied = denied || !a.repoAccess(repo, user, cap)
				}
			}
			if source, ok := in["source"].(map[string]any); ok {
				if repo := strOr(source["repoName"]); repo != "" {
					denied = denied || !a.repoAccess(repo, user, AdminRepo)
				}
			}
			if path == "/api/events/ingest" {
				repo := strOr(in["repo_name"])
				if src, ok := in["source"].(map[string]any); ok && repo == "" {
					repo = strOr(src["repoName"])
				}
				denied = denied || !a.repoAccess(repo, user, AdminRepo)
			}
			if !denied && path == "/api/refs/update" {
				if !a.allowDirectWorkspaceWrite(w, a.repositoryMetaByArtifact(strOr(in["repo"])), strOr(in["branch"])) {
					return
				}
			}
			if !denied && strings.HasPrefix(path, "/api/drafts/") && strings.HasSuffix(path, "/commit") {
				draft := a.scopedRecord("drafts", r.PathValue("id"))
				if !a.allowDirectWorkspaceWrite(w, a.repositoryMetaByArtifact(strOr(draft["repo"])), strOr(draft["branch"])) {
					return
				}
			}
			if params, ok := in["params"].(map[string]any); ok {
				denied = denied || !a.repoAccess(strOr(params["repo"]), user, RunAgent)
			}
		}
		if denied {
			writeJSON(w, 403, map[string]any{"error": "repository_access_denied"})
			return
		}
		next(w, r)
	}
}

func (a *App) workflowOwner(id string) string {
	if a.Trestle == nil {
		return ""
	}
	items, err := a.Trestle.ListRecords("workflow_owners", filterEq("workflow_id", id))
	if err != nil || len(items) != 1 {
		return ""
	}
	return strOr(items[0]["username"])
}
