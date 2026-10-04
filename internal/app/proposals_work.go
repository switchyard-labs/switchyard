package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

func proposalWorkID(proposal, actor, key string) string {
	sum := sha256.Sum256([]byte(proposal + "\x00" + actor + "\x00" + key))
	return "wk_" + hex.EncodeToString(sum[:16])
}

// Persist the intended Work in the Proposal before creating separate Work
// records. Retries reuse its immutable input, then repair missing records.
func (a *App) handleProposalWork(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	if !a.CanRepository(meta, user, WriteRepo) {
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
	var in struct {
		Version     string `json:"version"`
		OperationID string `json:"operation_id"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Kind        string `json:"kind"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if len(in.OperationID) < 8 || len(in.OperationID) > 128 {
		writeJSON(w, 400, map[string]any{"error": "proposal_operation_id_required"})
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = strOr(p["title"])
	}
	if in.Kind == "" {
		in.Kind = "feature"
	}
	if len(in.Title) > 240 || len(in.Body) > 65536 || len(in.Kind) > 64 {
		writeJSON(w, 400, map[string]any{"error": "proposal_work_invalid"})
		return
	}
	id := proposalWorkID(strOr(p["id"]), user, in.OperationID)
	intents, _ := p["work_intents"].([]any)
	var intent map[string]any
	for _, raw := range intents {
		old, ok := raw.(map[string]any)
		if ok && old["id"] == id {
			intent = old
			break
		}
	}
	desired := map[string]any{"title": in.Title, "body": in.Body, "kind": in.Kind}
	if intent != nil {
		old, _ := json.Marshal(intent["input"])
		next, _ := json.Marshal(desired)
		if string(old) != string(next) {
			writeJSON(w, 409, map[string]any{"error": "proposal_operation_input_conflict"})
			return
		}
	} else {
		if in.Version != ver {
			writeJSON(w, 409, map[string]any{"error": "proposal_version_conflict"})
			return
		}
		history, _ := p["history"].([]any)
		if len(intents) >= 100 || len(history) >= 1000 {
			writeJSON(w, 409, map[string]any{"error": "proposal_work_limit"})
			return
		}
		intent = map[string]any{"id": id, "input": desired, "actor": user, "created_at": nowStr(), "proposal_id": p["id"]}
		if err := a.Trestle.PatchRecord("proposals", rid, ver, map[string]any{"work_intents": append(intents, intent), "history": append(history, map[string]any{"kind": "work_requested", "work_id": id, "actor": user, "at": intent["created_at"]}), "updated_at": nowStr()}); err != nil {
			writeJSON(w, 409, map[string]any{"error": "proposal_update_conflict"})
			return
		}
	}
	_, _, work, err := a.Trestle.FindRecord("work", filterEq("id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_work_unavailable"})
		return
	}
	if work == nil {
		work = map[string]any{"id": id, "title": in.Title, "kind": in.Kind, "status": "open", "owner": user, "created_at": intent["created_at"], "updated_at": intent["created_at"]}
		if _, _, err := a.Trestle.CreateRecord("work", work, "proposal-work-"+id); err != nil {
			writeJSON(w, 502, map[string]any{"error": "proposal_work_create_failed", "work_id": id, "retry_safe": true})
			return
		}
	}
	_, _, detail, err := a.Trestle.FindRecord("work_details", filterEq("work_id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_work_unavailable", "work_id": id})
		return
	}
	if detail == nil {
		if _, _, err := a.Trestle.CreateRecord("work_details", map[string]any{"work_id": id, "body": in.Body, "repo": meta["artifact_name"], "assignee": "", "updated_at": intent["created_at"]}, "proposal-work-details-"+id); err != nil {
			writeJSON(w, 502, map[string]any{"error": "proposal_work_details_failed", "work_id": id, "retry_safe": true})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"work": work, "proposal_id": p["id"], "operation_id": in.OperationID})
}
