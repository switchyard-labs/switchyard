package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// Generation is an explicit bounded execution, not a client assertion of
// Agent authorship. It produces the same Proposal used for human intake.
func (a *App) handleGenerateProposal(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	if !a.CanRepository(meta, user, RunAgent) || !a.CanRepository(meta, user, WriteRepo) {
		writeJSON(w, 403, map[string]any{"error": "proposal_agent_permission_required"})
		return
	}
	var in struct {
		Prompt string `json:"prompt"`
		Branch string `json:"branch"`
	}
	if readJSON(r, &in) != nil || strings.TrimSpace(in.Prompt) == "" || len(in.Prompt) > 16384 {
		writeJSON(w, 400, map[string]any{"error": "proposal_prompt_invalid"})
		return
	}
	if in.Branch == "" {
		in.Branch = strOr(meta["default_branch"])
	}
	ctx, configured, err := a.requestedProvider(r, "proposer")
	if err != nil || !configured {
		writeJSON(w, 400, map[string]any{"error": "proposal_provider_required"})
		return
	}
	sha, err := a.repoHead(strOr(meta["artifact_name"]), in.Branch)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "proposal_source_unavailable"})
		return
	}
	ctx = context.WithValue(ctx, agentSourceSHAKey{}, sha)
	ex, err := a.runViaSubstrateContext(ctx, "proposer", "", strOr(meta["artifact_name"]), in.Branch, "PROPOSAL.json", in.Prompt, `Use only the supplied evidence. Return files.PROPOSAL.json as a JSON object with title, description, type, labels. Type is Bug, Suggestion, Feature, Improvement, Refactor, Documentation, Security, Performance, Experiment, Question or Other. Record a consideration only; do not write repository files, run commands or create Work.`)
	if err != nil {
		writeAgentExecutionError(w, ex, err)
		return
	}
	var p Proposal
	if json.Unmarshal([]byte(ex.Result["PROPOSAL.json"]), &p) != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_agent_output_invalid", "execution_id": ex.ID})
		return
	}
	p.ID = "prop_" + randHex(12)
	p.RepositoryID = strOr(meta["id"])
	p.State = "open"
	p.ClosureOutcome = ""
	p.AuthorPrincipal = "agent:" + ex.ID
	p.CreatedAt = nowStr()
	p.UpdatedAt = p.CreatedAt
	p.Provenance = map[string]any{"source": "agent", "execution_id": ex.ID, "role": "proposer", "delegated_by": user, "source_sha": sha, "source_ref": in.Branch}
	if p.Type == "Security" && strOr(meta["visibility"]) != "private" {
		writeJSON(w, 400, map[string]any{"error": "restricted_security_intake_not_available"})
		return
	}
	if err := p.validate(); err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_agent_output_invalid", "execution_id": ex.ID})
		return
	}
	values := proposalValues(p)
	values["history"] = []any{map[string]any{"kind": "created", "actor": p.AuthorPrincipal, "delegated_by": user, "at": p.CreatedAt}}
	if _, _, err := a.Trestle.CreateRecord("proposals", values, "proposal-execution-"+ex.ID); err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_create_failed", "execution_id": ex.ID})
		return
	}
	writeJSON(w, 201, values)
}
