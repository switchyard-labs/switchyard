package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

func proposalValues(p Proposal) map[string]any {
	b, _ := json.Marshal(p)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

// Repository-scoped routes always resolve the durable identity first. Never
// accept a repository/principal from client input as mutation authority.
func (a *App) handleProposals(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if !a.CanRepository(meta, user, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	repoID := strOr(meta["id"])
	if r.Method == http.MethodPost {
		if user == "" || a.isDemoGuest(r) {
			writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
			return
		}
		if !a.proposalIntakeAllowed(meta, user) {
			writeJSON(w, 403, map[string]any{"error": "proposal_intake_permission_required"})
			return
		}
		var p Proposal
		if readJSON(r, &p) != nil {
			writeJSON(w, 400, map[string]any{"error": "bad_request"})
			return
		}
		// Lifecycle and provenance are server-owned on creation. Import/Agent
		// adapters will need authenticated evidence, not arbitrary JSON claims.
		findingID := strOr(p.Provenance["finding_id"])
		p.ID = "prop_" + randHex(12)
		p.RepositoryID = repoID
		p.AuthorPrincipal = user
		p.State, p.ClosureOutcome = "open", ""
		p.Provenance = map[string]any{"source": "human", "principal": user}
		if findingID != "" {
			evidence, err := a.proposalFindingEvidence(meta, user, findingID)
			if err != nil {
				writeJSON(w, proposalErrorStatus(err), map[string]any{"error": err.Error()})
				return
			}
			p.Provenance = evidence
		}

		p.CreatedAt, p.UpdatedAt = nowStr(), nowStr()
		if p.Type == "" {
			p.Type = "Other"
		}
		if p.Labels == nil {
			p.Labels = []string{}
		}
		if p.Type == "Security" && strOr(meta["visibility"]) != "private" {
			writeJSON(w, 400, map[string]any{"error": "restricted_security_intake_not_available"})
			return
		}
		if err := p.validate(); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		values := proposalValues(p)
		values["history"] = []map[string]any{{"kind": "created", "actor": user, "at": p.CreatedAt, "state": "open"}}
		if _, _, err := a.Trestle.CreateRecord("proposals", values, "proposal-"+p.ID); err != nil {
			writeJSON(w, 502, map[string]any{"error": "proposal_create_failed"})
			return
		}
		writeJSON(w, 201, values)
		return
	}
	items, err := a.Trestle.ListRecords("proposals", filterEq("repository_id", repoID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposals_unavailable"})
		return
	}
	_, _, graph, graphErr := a.Trestle.FindRecord("proposal_graphs", filterEq("repository_id", repoID))
	if graphErr != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_links_unavailable"})
		return
	}
	applySupersession := func(item map[string]any) {
		edges, _ := graph["edges"].([]any)
		for _, raw := range edges {
			edge, ok := raw.(map[string]any)
			if ok && edge["relation"] == "supersedes" && edge["target_id"] == item["id"] {
				item["state"] = "closed"
				item["closure_outcome"] = "superseded"
				item["superseded_by"] = edge["proposal_id"]
			}
		}
	}
	if id := r.PathValue("id"); id != "" {
		for _, item := range items {
			if strOr(item["id"]) == id {
				_, version, current, findErr := a.Trestle.FindRecord("proposals", filterEq("id", id))
				if findErr != nil {
					writeJSON(w, 502, map[string]any{"error": "proposals_unavailable"})
					return
				}
				applySupersession(current)
				current["version"] = version
				writeJSON(w, 200, current)
				return
			}
		}
		writeJSON(w, 404, map[string]any{"error": "proposal_not_found"})
		return
	}
	out := []map[string]any{}
	for _, item := range items {
		applySupersession(item)
		matches := true
		for _, key := range []string{"state", "type", "author_principal", "priority"} {
			if wanted := r.URL.Query().Get(key); wanted != "" && strOr(item[key]) != wanted {
				matches = false
			}
		}
		if wanted := r.URL.Query().Get("label"); wanted != "" {
			found := false
			labels, _ := item["labels"].([]any)
			for _, label := range labels {
				if strOr(label) == wanted {
					found = true
				}
			}
			if !found {
				matches = false
			}
		}
		if wanted := r.URL.Query().Get("provenance"); wanted != "" {
			provenance, _ := item["provenance"].(map[string]any)
			if strOr(provenance["source"]) != wanted {
				matches = false
			}
		}
		if query := strings.ToLower(r.URL.Query().Get("q")); query != "" && !strings.Contains(strings.ToLower(strOr(item["title"])), query) {
			matches = false
		}
		if matches {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i]["created_at"] == out[j]["created_at"] {
			return strOr(out[i]["id"]) < strOr(out[j]["id"])
		}
		return strOr(out[i]["created_at"]) > strOr(out[j]["created_at"])
	})
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	const size = 20
	total := len(out)
	// Bound before multiplying to avoid overflow from arbitrary query values.
	if page > total/size+1 {
		page = total/size + 1
	}
	start := (page - 1) * size
	end := start + size
	if end > total {
		end = total
	}
	writeJSON(w, 200, map[string]any{"items": out[start:end], "total": total, "page": page, "page_size": size})
}

func (a *App) handleUpdateProposal(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	if !a.CanRepository(meta, user, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	rid, ver, values, err := a.Trestle.FindRecord("proposals", filterEq("id", r.PathValue("id")))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposals_unavailable"})
		return
	}
	if rid == "" || strOr(values["repository_id"]) != strOr(meta["id"]) {
		writeJSON(w, 404, map[string]any{"error": "proposal_not_found"})
		return
	}
	var in struct {
		Version        string    `json:"version"`
		Title          *string   `json:"title"`
		Description    *string   `json:"description"`
		Type           *string   `json:"type"`
		State          *string   `json:"state"`
		ClosureOutcome *string   `json:"closure_outcome"`
		Priority       *string   `json:"priority"`
		Severity       *string   `json:"severity"`
		Labels         *[]string `json:"labels"`
		Reason         string    `json:"reason"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if !a.CanRepository(meta, user, WriteRepo) {
		if strOr(values["author_principal"]) != user {
			writeJSON(w, 403, map[string]any{"error": "proposal_author_required"})
			return
		}
		if in.State != nil || in.ClosureOutcome != nil || in.Priority != nil || in.Severity != nil {
			writeJSON(w, 403, map[string]any{"error": "proposal_maintainer_required"})
			return
		}
	}
	if in.Version == "" {
		writeJSON(w, 400, map[string]any{"error": "proposal_version_required"})
		return
	}
	if in.Version != ver {
		writeJSON(w, 409, map[string]any{"error": "proposal_version_conflict"})
		return
	}
	data, _ := json.Marshal(values)
	var p Proposal
	_ = json.Unmarshal(data, &p)
	oldState, oldOutcome := p.State, p.ClosureOutcome
	for _, field := range []struct {
		value *string
		dst   *string
	}{{in.Title, &p.Title}, {in.Description, &p.Description}, {in.Type, &p.Type}, {in.State, &p.State}, {in.ClosureOutcome, &p.ClosureOutcome}, {in.Priority, &p.Priority}, {in.Severity, &p.Severity}} {
		if field.value != nil {
			*field.dst = *field.value
		}
	}
	if in.Labels != nil {
		p.Labels = *in.Labels
	}
	if in.State != nil && p.State != "closed" {
		p.ClosureOutcome = ""
	}
	// Supersession must create its checked relationship, not just a state label.
	if p.ClosureOutcome == "superseded" && oldOutcome != "superseded" {
		writeJSON(w, 400, map[string]any{"error": "supersession_relationship_required"})
		return
	}
	if p.Type == "Security" && strOr(meta["visibility"]) != "private" {
		writeJSON(w, 400, map[string]any{"error": "restricted_security_intake_not_available"})
		return
	}
	if len(in.Reason) > 4096 {
		writeJSON(w, 400, map[string]any{"error": "proposal_reason_too_large"})
		return
	}
	if err := p.validate(); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	p.UpdatedAt = nowStr()
	// Keep the audit in the same CAS write as the mutation, avoiding a changed
	// lifecycle with a lost separate event write after a process crash.
	history, _ := values["history"].([]any)
	if len(history) >= 1000 {
		writeJSON(w, 409, map[string]any{"error": "proposal_history_limit"})
		return
	}
	patch := proposalValues(p)
	patch["closure_outcome"] = p.ClosureOutcome
	patch["history"] = append(history, map[string]any{"kind": "updated", "actor": user, "at": p.UpdatedAt, "previous_state": oldState, "previous_outcome": oldOutcome, "state": p.State, "outcome": p.ClosureOutcome, "reason": in.Reason})
	if err := a.Trestle.PatchRecord("proposals", rid, ver, patch); err != nil {
		writeJSON(w, 409, map[string]any{"error": "proposal_update_conflict"})
		return
	}
	writeJSON(w, 200, patch)
}
