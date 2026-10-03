package app

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

func newWorkID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "wk_" + hex.EncodeToString(b)
}

func (a *App) handleListWork(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("work", "")
	items = a.visibleRecords("work", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, it := range items {
		out = append(out, it)
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) handleCreateWork(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Title    string `json:"title"`
		Kind     string `json:"kind"`
		Body     string `json:"body"`
		Repo     string `json:"repo"`
		Assignee string `json:"assignee"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeJSON(w, 400, map[string]any{"error": "title_required"})
		return
	}
	if in.Kind == "" {
		in.Kind = "feature"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := newWorkID()
	_, _, err := a.Trestle.CreateRecord("work", map[string]any{
		"id": id, "title": in.Title, "kind": in.Kind, "status": "open",
		"owner": user, "created_at": now, "updated_at": now,
	}, "work-"+id)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	_, _, detailErr := a.Trestle.CreateRecord("work_details", map[string]any{"work_id": id, "body": strings.TrimSpace(in.Body), "repo": strings.TrimSpace(in.Repo), "assignee": strings.TrimSpace(in.Assignee), "updated_at": now}, "work-details-"+id)
	if detailErr != nil {
		writeJSON(w, 502, map[string]any{"error": "work_details_create_failed", "id": id, "metadata_created": true})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "title": in.Title, "kind": in.Kind, "status": "open", "owner": user, "body": in.Body, "repo": in.Repo, "assignee": in.Assignee})
}

func (a *App) handleGetWork(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	items, err := a.Trestle.ListRecords("work", `id = "`+id+`"`)
	items = a.visibleRecords("work", items, a.currentUser(r))
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "work_not_found"})
		return
	}
	out := map[string]any{}
	for k, v := range items[0] {
		out[k] = v
	}
	if ds, _ := a.Trestle.ListRecords("work_details", `work_id = "`+id+`"`); len(ds) > 0 {
		for _, k := range []string{"body", "repo", "assignee"} {
			out[k] = ds[0][k]
		}
	}
	if ats, _ := a.Trestle.ListRecords("attempts", `work_id = "`+id+`"`); len(ats) > 0 {
		ats = a.visibleRecords("attempts", ats, a.currentUser(r))
		out["attempts"] = ats
	} else {
		out["attempts"] = []map[string]any{}
	}
	if prs, _ := a.Trestle.ListRecords("prs", `work_id = "`+id+`"`); len(prs) > 0 {
		prs = a.visibleRecords("prs", prs, a.currentUser(r))
		out["pull_requests"] = prs
	} else {
		out["pull_requests"] = []map[string]any{}
	}
	writeJSON(w, 200, out)
}

func (a *App) handleUpdateWork(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	rid, ver, v, e := a.Trestle.FindRecord("work", `id = "`+id+`"`)
	if e != nil || rid == "" {
		writeJSON(w, 404, map[string]any{"error": "work_not_found"})
		return
	}
	if strOr(v["owner"]) != u {
		writeJSON(w, 403, map[string]any{"error": "work_owner_required"})
		return
	}
	var in struct {
		Status   string  `json:"status"`
		Title    string  `json:"title"`
		Body     *string `json:"body"`
		Assignee *string `json:"assignee"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	patch := map[string]any{"updated_at": time.Now().UTC().Format(time.RFC3339)}
	if strings.TrimSpace(in.Title) != "" {
		patch["title"] = strings.TrimSpace(in.Title)
	}
	if in.Status == "open" || in.Status == "closed" {
		patch["status"] = in.Status
	}
	if err := a.Trestle.PatchRecord("work", rid, ver, patch); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "work_update_failed"})
		return
	}
	if in.Body != nil || in.Assignee != nil {
		drid, dver, _, err := a.Trestle.FindRecord("work_details", filterEq("work_id", id))
		if err != nil || drid == "" {
			writeJSON(w, 502, map[string]any{"error": "work_details_lookup_failed", "metadata_updated": true})
			return
		}
		details := map[string]any{"updated_at": nowStr()}
		if in.Body != nil {
			details["body"] = *in.Body
		}
		if in.Assignee != nil {
			details["assignee"] = *in.Assignee
		}
		if err := a.Trestle.PatchRecord("work_details", drid, dver, details); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "work_details_update_failed", "metadata_updated": true})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) handleListWorkComments(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	xs, e := a.Trestle.ListRecords("work_comments", `work_id = "`+r.PathValue("id")+`"`)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": xs})
}
func (a *App) handleCreateWorkComment(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Body) == "" {
		writeJSON(w, 400, map[string]any{"error": "body_required"})
		return
	}
	id := "cmt_" + newWorkID()[3:]
	vals := map[string]any{"id": id, "work_id": r.PathValue("id"), "author": u, "body": strings.TrimSpace(in.Body), "created_at": time.Now().UTC().Format(time.RFC3339)}
	_, _, e := a.Trestle.CreateRecord("work_comments", vals, "work-comment-"+id)
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	writeJSON(w, 201, vals)
}
