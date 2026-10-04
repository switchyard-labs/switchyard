package app

import "net/http"

// All Proposal edges within a repository share one CAS record. This makes
// cycle checking safe across competing processes, not just one Go mutex.
func proposalCycle(edges []any, source, target string) bool {
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(id string) bool {
		if id == source {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		for _, raw := range edges {
			e, ok := raw.(map[string]any)
			if ok && e["relation"] == "supersedes" && e["proposal_id"] == id {
				if walk(strOr(e["target_id"])) {
					return true
				}
			}
		}
		return false
	}
	return source == target || walk(target)
}

func (a *App) handleProposalLinks(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	cap := ReadRepo
	if r.Method != "GET" {
		cap = WriteRepo
	}
	if !a.CanRepository(meta, user, cap) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	if r.Method != "GET" && (user == "" || a.isDemoGuest(r)) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	_, _, p, err := a.Trestle.FindRecord("proposals", filterEq("id", r.PathValue("id")))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposals_unavailable"})
		return
	}
	if p == nil || p["repository_id"] != meta["id"] {
		writeJSON(w, 404, map[string]any{"error": "proposal_not_found"})
		return
	}
	repoID := strOr(meta["id"])
	rid, ver, graph, err := a.Trestle.FindRecord("proposal_graphs", filterEq("repository_id", repoID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_links_unavailable"})
		return
	}
	edges, _ := graph["edges"].([]any)
	if edges == nil {
		edges = []any{}
	}
	if r.Method == "GET" {
		out := []any{}
		for _, raw := range edges {
			e, ok := raw.(map[string]any)
			if ok && (e["proposal_id"] == p["id"] || e["target_kind"] == "proposal" && e["target_id"] == p["id"]) {
				out = append(out, e)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "version": ver})
		return
	}
	var in struct {
		Version    string `json:"version"`
		Relation   string `json:"relation"`
		TargetKind string `json:"target_kind"`
		TargetID   string `json:"target_id"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if in.Version != ver {
		writeJSON(w, 409, map[string]any{"error": "proposal_graph_conflict"})
		return
	}
	if in.TargetKind == "proposal" && in.Relation == "superseded_by" {
		_, _, replacement, e := a.Trestle.FindRecord("proposals", filterEq("id", in.TargetID))
		if e != nil || replacement == nil || replacement["repository_id"] != repoID {
			writeJSON(w, 400, map[string]any{"error": "proposal_target_invalid"})
			return
		}
		in.TargetID = strOr(p["id"])
		p = replacement
		in.Relation = "supersedes"
	}
	valid := in.TargetKind == "proposal" && (in.Relation == "related" || in.Relation == "duplicates" || in.Relation == "supersedes") || in.TargetKind == "work" && in.Relation == "work"
	if !valid || in.TargetID == "" {
		writeJSON(w, 400, map[string]any{"error": "proposal_relation_invalid"})
		return
	}
	if in.TargetKind == "proposal" {
		_, _, target, e := a.Trestle.FindRecord("proposals", filterEq("id", in.TargetID))
		if e != nil || target == nil || target["repository_id"] != repoID || in.TargetID == p["id"] {
			writeJSON(w, 400, map[string]any{"error": "proposal_target_invalid"})
			return
		}
		if in.Relation == "supersedes" && proposalCycle(edges, strOr(p["id"]), in.TargetID) {
			writeJSON(w, 409, map[string]any{"error": "proposal_supersession_cycle"})
			return
		}
	} else {
		_, _, detail, e := a.Trestle.FindRecord("work_details", filterEq("work_id", in.TargetID))
		if e != nil || detail == nil || strOr(detail["repo"]) != strOr(meta["artifact_name"]) {
			writeJSON(w, 400, map[string]any{"error": "proposal_work_target_invalid"})
			return
		}
	}
	for _, raw := range edges {
		edge, ok := raw.(map[string]any)
		if ok && edge["proposal_id"] == p["id"] && edge["relation"] == in.Relation && edge["target_kind"] == in.TargetKind && edge["target_id"] == in.TargetID {
			writeJSON(w, 200, edge)
			return
		}
	}
	if in.Relation == "supersedes" {
		for _, raw := range edges {
			edge, ok := raw.(map[string]any)
			if ok && edge["relation"] == "supersedes" && edge["target_id"] == in.TargetID {
				writeJSON(w, 409, map[string]any{"error": "proposal_already_superseded"})
				return
			}
		}
	}
	if len(edges) >= 10000 {
		writeJSON(w, 409, map[string]any{"error": "proposal_graph_limit"})
		return
	}
	edge := map[string]any{"id": "pl_" + randHex(12), "proposal_id": p["id"], "relation": in.Relation, "target_kind": in.TargetKind, "target_id": in.TargetID, "created_by": user, "created_at": nowStr()}
	patch := map[string]any{"repository_id": repoID, "edges": append(edges, edge)}
	if rid == "" {
		_, _, err = a.Trestle.CreateRecord("proposal_graphs", map[string]any{"repository_id": repoID, "edges": []any{}}, "proposal-graph-"+repoID)
		// Initialization may race; never report an edge written when another
		// request's initialization receipt was returned by idempotency.
		writeJSON(w, 409, map[string]any{"error": "proposal_graph_initialized", "reload_required": true})
		return
	} else {
		err = a.Trestle.PatchRecord("proposal_graphs", rid, ver, patch)
	}
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": "proposal_graph_conflict"})
		return
	}
	writeJSON(w, 201, edge)
}
