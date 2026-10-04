package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"switchyard/internal/agent"
	"time"
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
		Prompt      string `json:"prompt"`
		Branch      string `json:"branch"`
		ExecutionID string `json:"execution_id"`
		ProposalID  string `json:"proposal_id"`
	}
	if readJSON(r, &in) != nil || (strings.TrimSpace(in.Prompt) == "" && in.ExecutionID == "") || len(in.Prompt) > 16384 {
		writeJSON(w, 400, map[string]any{"error": "proposal_prompt_invalid"})
		return
	}
	if in.Branch == "" {
		in.Branch = strOr(meta["default_branch"])
	}
	if in.ProposalID != "" {
		target := a.scopedRecord("proposals", in.ProposalID)
		if target == nil || target["repository_id"] != meta["id"] {
			writeJSON(w, 404, map[string]any{"error": "proposal_not_found"})
			return
		}
		in.Prompt = "Proposal: " + strOf(target["title"]) + "\n" + strOf(target["description"]) + "\nDiscussion request: " + in.Prompt
	}
	var ex *agent.Execution
	var sha string
	var err error
	if in.ExecutionID != "" {
		_, _, record, e := a.Trestle.FindRecord("execution_metadata", filterEq("execution_id", in.ExecutionID))
		metadata, _ := record["metadata"].(map[string]any)
		if e != nil || record == nil || record["repo"] != meta["artifact_name"] || metadata["role"] != "proposer" || metadata["principal"] != user || metadata["status"] != "succeeded" || strOf(metadata["proposal_id"]) != in.ProposalID {
			writeJSON(w, 404, map[string]any{"error": "proposal_execution_not_available"})
			return
		}
		sha = strOf(metadata["source_sha"])
		in.Branch = strOf(metadata["branch"])
		execution := a.scopedRecord("executions", in.ExecutionID)
		finished, _ := time.Parse(time.RFC3339, strOf(execution["finished_at"]))
		if finished.IsZero() || !workspaceSHA.MatchString(sha) {
			writeJSON(w, 502, map[string]any{"error": "proposal_execution_evidence_incomplete"})
			return
		}
		ex = &agent.Execution{ID: in.ExecutionID, Finished: finished, Result: map[string]string{"PROPOSAL.json": strOf(metadata["proposal_result"])}}
	} else {
		ctx, configured, e := a.requestedProvider(r, "proposer")
		if e != nil || !configured {
			writeJSON(w, 400, map[string]any{"error": "proposal_provider_required"})
			return
		}
		sha, err = a.repoHead(strOr(meta["artifact_name"]), in.Branch)
		if err != nil || !workspaceSHA.MatchString(sha) {
			writeJSON(w, 400, map[string]any{"error": "proposal_source_unavailable"})
			return
		}
		ctx = context.WithValue(ctx, agentSourceSHAKey{}, sha)
		ctx = context.WithValue(ctx, agentProposalTargetKey{}, in.ProposalID)
		ex, err = a.runViaSubstrateContext(ctx, "proposer", "", strOr(meta["artifact_name"]), in.Branch, "PROPOSAL.json", in.Prompt, `Use only the supplied evidence. Return files.PROPOSAL.json as a JSON object with title, description, type, labels. Type is Bug, Suggestion, Feature, Improvement, Refactor, Documentation, Security, Performance, Experiment, Question or Other. Record a consideration only; do not write repository files, run commands or create Work.`)

		if err != nil {
			writeAgentExecutionError(w, ex, err)
			return
		}
	}
	var p Proposal
	if json.Unmarshal([]byte(ex.Result["PROPOSAL.json"]), &p) != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_agent_output_invalid", "execution_id": ex.ID})
		return
	}
	if in.ProposalID != "" {
		a.saveAgentProposalComment(w, r, meta, user, in.ProposalID, ex, sha, in.Branch, p.Description)
		return
	}
	p.ID = "prop_" + sha256Hex([]byte(ex.ID))[:24]
	_, _, previous, readErr := a.Trestle.FindRecord("proposals", filterEq("id", p.ID))
	if readErr != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_recovery_unavailable", "execution_id": ex.ID})
		return
	}
	if previous != nil {
		if previous["repository_id"] != meta["id"] || previous["author_principal"] != "agent:"+ex.ID {
			writeJSON(w, 409, map[string]any{"error": "proposal_execution_identity_conflict"})
			return
		}
		writeJSON(w, 201, previous)
		return
	}
	p.RepositoryID = strOr(meta["id"])
	p.State = "open"
	p.ClosureOutcome = ""
	p.AuthorPrincipal = "agent:" + ex.ID
	p.CreatedAt = ex.Finished.UTC().Format(time.RFC3339)
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
	stored := a.scopedRecord("proposals", p.ID)
	if stored == nil || stored["repository_id"] != meta["id"] {
		writeJSON(w, 502, map[string]any{"error": "proposal_create_pending", "execution_id": ex.ID})
		return
	}
	writeJSON(w, 201, stored)
}

type agentProposalTargetKey struct{}

func (a *App) saveAgentProposalComment(w http.ResponseWriter, r *http.Request, meta map[string]any, user, target string, ex *agent.Execution, sha, branch, body string) {
	rid, version, p, err := a.Trestle.FindRecord("proposals", filterEq("id", target))
	if err != nil || p == nil || p["repository_id"] != meta["id"] {
		writeJSON(w, 404, map[string]any{"error": "proposal_not_found"})
		return
	}
	comments, _ := p["discussion"].([]any)
	id := "pc_" + sha256Hex([]byte(target + "\x00" + ex.ID))[:24]
	for _, raw := range comments {
		comment, _ := raw.(map[string]any)
		if comment["id"] == id {
			writeJSON(w, 200, map[string]any{"id": target, "comment": comment, "version": version})
			return
		}
	}
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 16384 {
		writeJSON(w, 502, map[string]any{"error": "proposal_agent_comment_invalid", "execution_id": ex.ID})
		return
	}
	if len(comments) >= 200 {
		writeJSON(w, 409, map[string]any{"error": "proposal_discussion_limit", "execution_id": ex.ID})
		return
	}
	at := ex.Finished.UTC().Format(time.RFC3339)
	principal := "agent:" + ex.ID
	comment := map[string]any{"id": id, "author_principal": principal, "body": body, "created_at": at, "provenance": map[string]any{"source": "agent", "execution_id": ex.ID, "delegated_by": user, "source_sha": sha, "source_ref": branch}}
	history, _ := p["history"].([]any)
	if len(history) >= 1000 {
		writeJSON(w, 409, map[string]any{"error": "proposal_history_limit", "execution_id": ex.ID})
		return
	}
	history = append(history, map[string]any{"kind": "comment_created", "actor": principal, "delegated_by": user, "comment_id": id, "at": at})
	if err = a.Trestle.PatchRecord("proposals", rid, version, map[string]any{"discussion": append(comments, comment), "history": history, "updated_at": nowStr()}); err != nil {
		writeJSON(w, 409, map[string]any{"error": "proposal_agent_comment_pending", "execution_id": ex.ID})
		return
	}
	writeJSON(w, 201, map[string]any{"id": target, "comment": comment})
}
