package app

import (
	"net/http"
	"strings"
)

// Discussion is stored with the Proposal so edits and their audit entries are
// committed together under a single Trestle compare-and-swap.
func (a *App) handleProposalDiscussion(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if !a.CanRepository(meta, user, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	rid, ver, p, err := a.Trestle.FindRecord("proposals", filterEq("id", r.PathValue("id")))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposals_unavailable"})
		return
	}
	if rid == "" || p["repository_id"] != meta["id"] {
		writeJSON(w, 404, map[string]any{"error": "proposal_not_found"})
		return
	}
	comments, _ := p["discussion"].([]any)
	if comments == nil {
		comments = []any{}
	}
	if r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"items": comments, "version": ver})
		return
	}
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	var in struct {
		Version string `json:"version"`
		Body    string `json:"body"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if in.Version != ver {
		writeJSON(w, 409, map[string]any{"error": "proposal_version_conflict"})
		return
	}
	body := strings.TrimSpace(in.Body)
	if r.Method != "DELETE" && (body == "" || len(body) > 16384) {
		writeJSON(w, 400, map[string]any{"error": "proposal_comment_invalid"})
		return
	}
	now := nowStr()
	event := map[string]any{"kind": "comment_created", "actor": user, "at": now}
	if r.Method == "POST" {
		if len(comments) >= 200 {
			writeJSON(w, 409, map[string]any{"error": "proposal_discussion_limit"})
			return
		}
		comment := map[string]any{"id": "pc_" + randHex(12), "author_principal": user, "body": body, "created_at": now, "provenance": map[string]any{"source": "human", "principal": user}}
		event["comment_id"] = comment["id"]
		comments = append(comments, comment)
	} else {
		found := false
		for _, entry := range comments {
			comment, ok := entry.(map[string]any)
			if !ok || strOr(comment["id"]) != r.PathValue("comment") {
				continue
			}
			found = true
			if comment["deleted_at"] != nil {
				writeJSON(w, 409, map[string]any{"error": "proposal_comment_deleted"})
				return
			}
			if strOr(comment["author_principal"]) != user && !(r.Method == "DELETE" && a.CanRepository(meta, user, WriteRepo)) {
				writeJSON(w, 403, map[string]any{"error": "proposal_comment_author_required"})
				return
			}
			event["comment_id"] = comment["id"]
			event["previous_body_sha256"] = sha256Hex([]byte(strOr(comment["body"])))
			if r.Method == "DELETE" {
				comment["body"] = ""
				comment["deleted_at"] = now
				event["kind"] = "comment_deleted"
			} else {
				comment["body"] = body
				comment["edited_at"] = now
				event["kind"] = "comment_edited"
			}
		}
		if !found {
			writeJSON(w, 404, map[string]any{"error": "proposal_comment_not_found"})
			return
		}
	}
	history, _ := p["history"].([]any)
	if len(history) >= 1000 {
		writeJSON(w, 409, map[string]any{"error": "proposal_history_limit"})
		return
	}
	patch := map[string]any{"discussion": comments, "history": append(history, event), "updated_at": now}
	if err := a.Trestle.PatchRecord("proposals", rid, ver, patch); err != nil {
		writeJSON(w, 409, map[string]any{"error": "proposal_update_conflict"})
		return
	}
	writeJSON(w, 200, map[string]any{"items": comments})
}
