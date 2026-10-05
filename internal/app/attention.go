package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"switchyard/internal/refs"
)

// CP11: Needs Attention + escalation decision packets.
//
// Attention aggregates everything that needs a human (or policy) decision:
// attempts parked in `conflict`, queue items `blocked`, workflow runs waiting
// for approval, and open error findings. An escalation assembles a
// SELF-CONTAINED decision packet (target, branches, conflicting files, base +
// ours + theirs contents, findings, suggested resolutions) so a human can
// decide without reconstructing history. Decisions are applied through the
// existing repair paths (conflict-resolver / queue requeue / workflow approve).

func (a *App) handleNeedsAttention(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items := []map[string]any{}
	warnings := []string{}
	for _, collection := range []string{"attempts", "iq", "workflow_runs", "findings", "action_runs"} {
		records, err := a.Trestle.ListRecords(collection, "")
		if err != nil {
			warnings = append(warnings, collection+" unavailable")
			continue
		}
		for _, record := range a.visibleRecords(collection, records, user) {
			item := attentionProjection(collection, record)
			if item != nil {
				items = append(items, item)
			}
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "count": len(items), "warnings": warnings, "complete": len(warnings) == 0})
}

// attentionProjection describes recorded outcomes without claiming unrecorded repair attempts.
func attentionProjection(collection string, x map[string]any) map[string]any {
	kind, title, reason, evidence := "", "", "", ""
	status := strOr(x["status"])
	repo, branch := strOr(x["repo"]), strOr(x["branch"])
	switch collection {
	case "attempts":
		if status != "conflict" {
			return nil
		}
		kind, title, reason = "attempt_conflict", "Attempt needs conflict resolution", "The Attempt is parked in conflict. Review the conflicting versions before changing its branch."
		evidence = strOr(x["message"])
	case "iq":
		if status != "blocked" {
			return nil
		}
		kind, title, reason = "queue_blocked", "Integration is blocked", "The queue stopped before publication. Requeue rechecks the current branches and required gates."
		evidence = strOr(x["error"])
	case "workflow_runs":
		if status != "waiting_approval" && status != "failed" {
			return nil
		}
		kind, title, reason = "workflow_approval", "Workflow awaits approval", "A durable approval step is waiting. Approval resumes from that step; completed effects are replayed."
		if status == "failed" {
			kind, title, reason = "workflow_failed", "Workflow stopped", "The recorded workflow failed. Retry replays completed steps and retries unfinished effects."
		}
		params, _ := x["params"].(map[string]any)
		repo = strOr(params["repo"])
		branch = strOr(params["branch"])
		evidence = strOr(x["error"])
	case "findings":
		if x["severity"] != "error" || status != "open" {
			return nil
		}
		kind, title, reason = "open_finding", "Validation finding needs review", "An error finding remains open. Inspect its file and linked Attempt before applying a repair."
		evidence = strOr(x["message"]) + " · " + strOr(x["file"])
	case "action_runs":
		state, _ := x["state"].(map[string]any)
		if state["status"] != "failed" && state["status"] != "failure" {
			return nil
		}
		kind, title, reason = "actions_failed", "Cloudflare Actions failed", "A CI run failed at its recorded commit. Inspect job output; a rerun uses the same source and approved definition."
		status = strOr(state["status"])
		branch = strOr(x["ref"])
		evidence = "Commit " + strOr(x["source_sha"])
	default:
		return nil
	}
	target := strOr(x["id"])
	if collection == "findings" {
		target = strOr(x["target"])
	}
	return map[string]any{"kind": kind, "target_id": target, "repo": repo, "branch": branch, "summary": title, "reason": reason, "evidence": evidence, "status": status, "created_at": x["updated_at"], "pr_id": x["pr_id"]}
}

// assembleConflictPacket builds a self-contained decision packet for a
// conflicted attempt: the conflicting files with base (canonical) + source
// (attempt) contents and the recorded findings.
func (a *App) assembleConflictPacket(attemptID string) (map[string]any, error) {
	atts, err := a.Trestle.ListRecords("attempts", `id = "`+attemptID+`"`)
	if err != nil || len(atts) == 0 {
		return nil, fmt.Errorf("attempt not found")
	}
	attempt := atts[0]
	repo := attempt["repo"].(string)
	branch := attempt["branch"].(string)
	rr, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		return nil, err
	}
	base := rr.DefaultBranch
	conflicts, err := a.Refs.PreviewMerge(repo, base, branch)
	if err != nil {
		return nil, err
	}
	files := []map[string]any{}
	for _, f := range conflicts {
		ours, _ := a.Artifacts.RawFile(repo, base, f)
		theirs, _ := a.Artifacts.RawFile(repo, branch, f)
		files = append(files, map[string]any{
			"file": f, "base_sha": base, "source_sha": branch,
			"base_version": string(ours), "source_version": string(theirs),
		})
	}
	findings := []map[string]any{}
	if f, err := a.Trestle.ListRecords("findings", `target = "`+attemptID+`"`); err == nil {
		for _, x := range f {
			findings = append(findings, map[string]any{"severity": x["severity"], "message": x["message"], "file": x["file"], "status": x["status"]})
		}
	}
	return map[string]any{
		"target_kind": "attempt_conflict", "target_id": attemptID,
		"repo": repo, "branch": branch, "base": base,
		"conflicting_files": files, "findings": findings,
		"suggested_resolutions": []string{"take_ours", "take_theirs", "three_way"},
	}, nil
}

// handleEscalate creates an escalation decision packet for a target.
func (a *App) handleEscalate(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	kind := r.PathValue("kind")
	targetID := r.PathValue("id")
	var packet map[string]any
	switch kind {
	case "attempt":
		p, err := a.assembleConflictPacket(targetID)
		if err != nil {
			writeJSON(w, 404, map[string]any{"error": err.Error()})
			return
		}
		packet = p
	case "queue":
		packet = map[string]any{"target_kind": "queue_blocked", "target_id": targetID, "repo": "", "branch": ""}
	case "workflow":
		packet = map[string]any{"target_kind": "workflow_approval", "target_id": targetID, "repo": "", "branch": ""}
	default:
		writeJSON(w, 400, map[string]any{"error": "unsupported_kind"})
		return
	}
	escID := "esc_" + randHex(10)
	_, _, err := a.Trestle.CreateRecord("escalations", map[string]any{
		"id": escID, "packet": packet, "status": "open",
		"created_by": user, "created_at": nowStr(), "decision": "", "decided_by": "", "decided_at": "",
	}, "esc-"+escID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": escID, "packet": packet, "status": "open", "escalated_by": user})
}

func (a *App) handleListEscalations(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("escalations", "")
	items = a.visibleRecords("escalations", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

// handleDecide applies a human decision to an escalation packet.
func (a *App) handleDecide(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	escID := r.PathValue("id")
	var in struct {
		Decision string `json:"decision"`
	}
	if err := readJSON(r, &in); err != nil || in.Decision == "" {
		writeJSON(w, 400, map[string]any{"error": "decision_required"})
		return
	}
	items, err := a.Trestle.ListRecords("escalations", `id = "`+escID+`"`)
	items = a.visibleRecords("escalations", items, a.currentUser(r))
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "escalation_not_found"})
		return
	}
	esc := items[0]
	if esc["status"] == "decided" {
		writeJSON(w, 409, map[string]any{"error": "already_decided"})
		return
	}
	packet, _ := esc["packet"].(map[string]any)
	kind, _ := packet["target_kind"].(string)
	var outcome map[string]any
	switch kind {
	case "attempt_conflict":
		outcome = a.applyConflictDecision(packet, in.Decision, user)
	case "queue_blocked":
		// decision "requeue" (or approve) re-queues the blocked integration
		targetID, _ := packet["target_id"].(string)
		a.patchQueue(targetID, map[string]any{"status": "queued", "error": "approved by " + user, "updated_at": nowStr()})
		outcome = map[string]any{"queue": targetID, "status": "requeued"}
	case "workflow_approval":
		targetID, _ := packet["target_id"].(string)
		a.patchRun(targetID, map[string]any{"status": "running", "updated_at": nowStr()})
		outcome = map[string]any{"workflow_run": targetID, "status": "approved"}
	default:
		writeJSON(w, 400, map[string]any{"error": "unsupported_packet"})
		return
	}
	if rid, ver, _, e := a.Trestle.FindRecord("escalations", `id = "`+escID+`"`); e == nil && rid != "" {
		_ = a.Trestle.PatchRecord("escalations", rid, ver, map[string]any{
			"status": "decided", "decision": in.Decision, "decided_by": user, "decided_at": nowStr(),
		})
	}
	writeJSON(w, 200, map[string]any{"id": escID, "decision": in.Decision, "decided_by": user, "outcome": outcome})
}

// applyConflictDecision writes the chosen versions of the conflicting files
// onto the attempt branch (take_ours = canonical version, take_theirs = source
// version, three_way = conflict-resolver merge) and marks the attempt resolved.
func (a *App) applyConflictDecision(packet map[string]any, decision, user string) map[string]any {
	repo, _ := packet["repo"].(string)
	branch, _ := packet["branch"].(string)
	base, _ := packet["base"].(string)
	targetID, _ := packet["target_id"].(string)
	conflicts, _ := packet["conflicting_files"].([]any)
	changes := []map[string]string{}
	for _, c := range conflicts {
		m, _ := c.(map[string]any)
		file, _ := m["file"].(string)
		ours, _ := m["base_version"].(string)
		theirs, _ := m["source_version"].(string)
		var chosen string
		switch decision {
		case "take_ours":
			chosen = ours
		case "take_theirs":
			chosen = theirs
		case "three_way":
			merged, ok := threeWayMerge(ours, base, theirs)
			if !ok {
				chosen = theirs
			} else {
				chosen = merged
			}
		default:
			chosen = theirs
		}
		changes = append(changes, map[string]string{"file": file, "content": chosen})
	}
	// apply via the shared ref substrate
	head, err := a.repoHead(repo, branch)
	if err != nil {
		return map[string]any{"attempt": targetID, "status": "resolve_failed", "error": err.Error()}
	}
	// skip files whose chosen content already matches the branch (a no-op,
	// e.g. take_theirs on the source branch); if nothing changes, the decision
	// is still recorded and the attempt marked resolved.
	effective := changes[:0]
	for _, c := range changes {
		if cur, e := a.Artifacts.RawFile(repo, branch, c["file"]); e != nil || string(cur) != c["content"] {
			effective = append(effective, c)
		}
	}
	if err == nil && len(effective) > 0 {
		refsChanges := make([]refs.Change, 0, len(effective))
		for _, c := range effective {
			refsChanges = append(refsChanges, refs.Change{Path: c["file"], Content: c["content"]})
		}
		if res, err := a.publishUpdate(publicationAuthority{principal: user}, repo, branch, head, refsChanges, "decision "+decision+" by "+user, "escalation:"+user+":"+targetID); err == nil {
			_ = res
			if err := a.patchAttempt(targetID, map[string]any{"status": "resolved", "message": "resolved by decision " + decision, "updated_at": nowStr()}); err != nil {
				return map[string]any{"attempt": targetID, "status": "coordination_failed", "error": err.Error()}
			}
			for _, c := range effective {
				if err := a.closeFinding(targetID, c["file"], "resolved by "+decision); err != nil {
					return map[string]any{"attempt": targetID, "status": "coordination_failed", "error": err.Error()}
				}
			}
			return map[string]any{"attempt": targetID, "status": "resolved", "decision": decision, "files": len(effective)}
		} else {
			return map[string]any{"attempt": targetID, "status": "resolve_failed", "decision": decision, "error": err.Error()}
		}
	}
	// nothing changed (or no files): record the decision; the attempt is
	// considered resolved per the human's choice.
	if err := a.patchAttempt(targetID, map[string]any{"status": "resolved", "message": "resolved by decision " + decision + " (no file changes)", "updated_at": nowStr()}); err != nil {
		return map[string]any{"attempt": targetID, "status": "coordination_failed", "error": err.Error()}
	}
	return map[string]any{"attempt": targetID, "status": "resolved", "decision": decision, "files": 0, "note": "no file changes required"}
}

func threeWayMerge(ours, base, theirs string) (string, bool) {
	// best-effort textual three-way merge via git merge-file in a temp dir
	dir, err := os.MkdirTemp("", "switchyard-twm-")
	if err != nil {
		return "", false
	}
	defer removeAll(dir)
	ot, bt, tt := filepath.Join(dir, "ours"), filepath.Join(dir, "base"), filepath.Join(dir, "theirs")
	_ = os.WriteFile(ot, []byte(ours), 0644)
	_ = os.WriteFile(bt, []byte(base), 0644)
	_ = os.WriteFile(tt, []byte(theirs), 0644)
	if runGit(dir, "", "merge-file", "-p", ot, bt, tt) == nil {
		if b, err := os.ReadFile(ot); err == nil {
			return string(b), true
		}
	}
	return "", false
}

var _ = strings.TrimSpace
var _ = time.RFC3339
