package app

import (
	"fmt"
	"net/http"
)

// Evidence is looked up by durable ID and authorization, never trusted from a
// client-supplied SHA or principal. Missing historical SHA stays unknown.
func (a *App) proposalFindingEvidence(meta map[string]any, user, id string) (map[string]any, error) {
	_, _, f, err := a.Trestle.FindRecord("findings", filterEq("id", id))
	if err != nil {
		return nil, fmt.Errorf("proposal_evidence_unavailable")
	}
	if f == nil {
		return nil, fmt.Errorf("proposal_finding_not_found")
	}
	attempt := a.scopedRecord("attempts", strOr(f["target"]))
	if attempt == nil || strOr(attempt["repo"]) != strOr(meta["artifact_name"]) || !a.recordAccess("attempts", attempt, user, ReadRepo) {
		return nil, fmt.Errorf("proposal_finding_not_found")
	}
	return map[string]any{"source": "review", "finding_id": id, "attempt_id": f["target"], "source_sha": f["source_sha"], "review_execution_id": f["review_execution_id"], "file": f["file"], "finding_message": f["message"], "recorded_at": f["created_at"], "submitted_by": user}, nil
}

func (a *App) addReviewFinding(target, severity, message, file, sourceSHA, executionID, principal string) error {
	id := "fnd_" + randHex(10)
	_, _, err := a.Trestle.CreateRecord("findings", map[string]any{"id": id, "target": target, "severity": severity, "message": message, "file": file, "status": "open", "created_at": nowStr(), "resolved_at": "", "source_sha": sourceSHA, "review_execution_id": executionID, "principal": principal}, "finding-"+id)
	return err
}

func proposalErrorStatus(err error) int {
	if err.Error() == "proposal_evidence_unavailable" {
		return http.StatusBadGateway
	}
	return http.StatusBadRequest
}
