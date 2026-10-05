package app

import (
	"context"
	"encoding/json"
	"fmt"
	"switchyard/internal/refs"
)

type workflowGitEffect struct {
	Candidate *refs.PreparedMerge `json:"candidate"`
	Dir       string              `json:"dir"`
	Result    map[string]any      `json:"result"`
}

func (e *wfExec) durableAgentUpdateRole(repo, branch, file, prompt, role string) (any, error) {
	auth := publicationAuthority{principal: e.actor, role: role, kind: publicationAgent}
	if err := e.a.authorizePublication(auth, repo, branch); err != nil {
		return nil, err
	}
	effectID := e.runID + "|" + e.currentKey
	_, _, values, err := e.a.Trestle.FindRecord("workflow_git_effects", filterEq("effect_id", effectID))
	if err != nil {
		return nil, &workflowDurabilityError{err}
	}
	var effect workflowGitEffect
	if values != nil {
		b, err := json.Marshal(values["state"])
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &effect); err != nil {
			return nil, err
		}
		if effect.Candidate == nil {
			return nil, fmt.Errorf("invalid Git effect")
		}
		if err = e.a.Refs.RestorePrepared(effect.Candidate, effect.Dir); err != nil {
			return nil, err
		}
	} else {
		head, err := e.a.repoHead(repo, branch)
		if err != nil {
			return nil, err
		}
		content, err := e.a.Artifacts.RawFile(repo, head, file)
		if err != nil {
			return nil, err
		}
		ctx := e.context
		if ctx == nil {
			ctx = context.Background()
		}
		execution, err := e.a.runViaSubstrateContext(agentUserContext(ctx, e.actor), role, "wf:"+e.runID, repo, branch, file, string(content), prompt)
		if err != nil {
			return nil, err
		}
		next, ok := execution.Result[file]
		if !ok {
			return nil, fmt.Errorf("agent produced no file result")
		}
		candidate, err := e.a.Refs.PrepareUpdate(repo, branch, head, []refs.Change{{Path: file, Content: next}}, "workflow effect "+effectID)
		if err != nil {
			return nil, err
		}
		effect = workflowGitEffect{Candidate: candidate, Dir: candidate.Dir, Result: map[string]any{"execution": execution.ID, "new_sha": candidate.CommitSHA, "status": "ok", "output": execution.Output}}
		if _, _, err = e.a.Trestle.CreateRecord("workflow_git_effects", map[string]any{"effect_id": effectID, "state": effect}, "workflow-git-"+effectID); err != nil {
			return nil, &workflowDurabilityError{err}
		}
	}
	published, err := e.a.Refs.CandidatePublished(effect.Candidate)
	if err != nil {
		return nil, err
	}
	if !published {
		if err := e.checkCancel(); err != nil {
			return nil, err
		}
		e.a.workflowCheckpoint("before_git_publication")
		result, err := e.a.publishAgentCandidate(auth, effect.Candidate)
		if err != nil {
			return nil, err
		}
		if result.Status != "ok" {
			return nil, fmt.Errorf("workflow Git effect stale; explicit reconciliation required")
		}
	}
	e.a.workflowCheckpoint("after_git_publication")
	return effect.Result, nil
}
