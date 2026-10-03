package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// CP9: Integration Queue full semantics.
//
// Ordering: items are processed FIFO per queue (created_at).
// Canonical-head freshness: integration is CAS-guarded (MergeBranch detects a
// moved base and reports stale); stale items are re-queued (repair loop) with
// bounded attempts.
// Policy: an item integrates only when its PR check passed AND preview
// integration is clean (no structural/semantic conflict). Conflict items are
// blocked with error findings until the conflict-resolver repair loop fixes the
// source branch, then they are re-queued.
// Risk classes: computed from change scope (files touched) + conflict state.
// Preview: every integration must be preview-clean.

const maxQueueAttempts = 4

type queueItem struct {
	id     string
	prID   string
	repo   string
	base   string
	branch string
	risk   string
	at     string
}

func (a *App) handleEnqueuePR(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	prID := r.PathValue("id")
	pr := a.prByID(r, prID)
	if pr == nil {
		writeJSON(w, 404, map[string]any{"error": "pr_not_found"})
		return
	}
	if !a.checkPassed(prID) {
		writeJSON(w, 409, map[string]any{"error": "check_not_passed"})
		return
	}
	qid, err := a.enqueuePR(pr)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	log.Printf("integration queue: %s enqueued by %s", qid, user)
	writeJSON(w, 201, map[string]any{"id": qid, "pr_id": prID, "status": "queued"})
}

func (a *App) enqueuePR(pr map[string]any) (string, error) {
	repo, branch, base, prID := strOf(pr["repo"]), strOf(pr["branch"]), strOf(pr["base"]), strOf(pr["id"])
	changed, err := a.changedFiles(repo, branch)
	if err != nil {
		return "", err
	}
	risk := "low"
	if len(changed) > 8 {
		risk = "high"
	} else if len(changed) > 2 {
		risk = "medium"
	}
	_, sourceSHA, err := a.Refs.Snapshot(repo, base, branch)
	if err != nil {
		return "", err
	}
	if !a.checkPassedAt(prID, sourceSHA) {
		return "", fmt.Errorf("exact source check required")
	}
	qid := "iq_" + sha256Hex([]byte(prID + "|" + sourceSHA))[:24]
	existing, _, _, err := a.Trestle.FindRecord("iq", filterEq("id", qid))
	if err != nil {
		return "", err
	}
	if existing != "" {
		return qid, nil
	}
	at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, _, err = a.Trestle.CreateRecord("iq", map[string]any{
		"id": qid, "pr_id": prID, "repo": repo, "base": base, "branch": branch,
		"status": "queued", "risk": risk, "policy": "exact_source_check + immutable_preview + semantic + policy",
		"attempts": "0", "error": "", "created_at": at, "updated_at": at,
	}, "iq-"+qid)
	if err != nil {
		existing, _, _, readErr := a.Trestle.FindRecord("iq", filterEq("id", qid))
		if readErr == nil && existing != "" {
			return qid, nil
		}
	}
	return qid, err
}

func (a *App) handleListQueue(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("iq", "")
	items = a.visibleRecords("iq", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) handleRequeueItem(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	qid := r.PathValue("id")
	items, err := a.Trestle.ListRecords("iq", `id = "`+qid+`"`)
	items = a.visibleRecords("iq", items, a.currentUser(r))
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "item_not_found"})
		return
	}
	if items[0]["status"] == "done" || items[0]["status"] == "running" {
		writeJSON(w, 409, map[string]any{"error": "item_not_requeueable"})
		return
	}
	if err := a.patchQueue(qid, map[string]any{"status": "queued", "updated_at": nowStr()}); err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	log.Printf("integration queue: %s re-queued by %s", qid, user)
	writeJSON(w, 200, map[string]any{"id": qid, "status": "queued"})
}

func (a *App) patchQueue(id string, values map[string]any) error {
	rid, ver, _, err := a.Trestle.FindRecord("iq", filterEq("id", id))
	if err != nil {
		return err
	}
	if rid == "" {
		return fmt.Errorf("queue item not found")
	}
	return a.Trestle.PatchRecord("iq", rid, ver, values)
}

// StartIntegrationQueue processes queued integrations. FIFO by created_at.
// Each item is guarded in-flight; policy gate (check + preview clean) before
// integration; stale bases are re-queued with bounded attempts.
func (a *App) StartIntegrationQueue(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	log.Printf("integration queue worker started (interval %s)", interval)
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a.pumpQueue()
			}
		}
	}()
}

func (a *App) pumpQueue() {
	// strict FIFO: at most one integration in flight globally. This is what
	// gives ordering — a later PR that edits the same file sees the earlier
	// PR's integration during preview, so conflicts are detected and routed to
	// the repair loop instead of surfacing at merge time.
	a.wfMu.Lock()
	if a.iqBusy {
		a.wfMu.Unlock()
		return
	}
	a.iqBusy = true
	a.wfMu.Unlock()

	items, err := a.Trestle.ListRecords("iq", "")
	if err != nil {
		a.wfMu.Lock()
		a.iqBusy = false
		a.wfMu.Unlock()
		return
	}
	var best *queueItem
	for _, it := range items {
		st, _ := it["status"].(string)
		if st != "queued" && st != "running" && st != "publishing" && st != "reconciling" && st != "done" {
			continue
		}
		_, _, effectValues, e := a.Trestle.FindRecord("integration_effects", filterEq("queue_id", strOf(it["id"])))
		if e != nil {
			continue
		}
		if effectValues != nil {
			effect, e := decodeQueueEffect(effectValues)
			if e != nil {
				continue
			}
			until, _ := time.Parse(time.RFC3339Nano, effect.LeaseUntil)
			if effect.Phase == "done" || (effect.Owner != "" && until.After(time.Now())) {
				continue
			}
		}

		itm := &queueItem{
			id: strOf(it["id"]), prID: strOf(it["pr_id"]), repo: strOf(it["repo"]),
			base: strOf(it["base"]), branch: strOf(it["branch"]), risk: strOf(it["risk"]),
			at: strOf(it["created_at"]),
		}
		if best == nil || itm.at < best.at {
			best = itm
		}
	}
	if best == nil {
		a.wfMu.Lock()
		a.iqBusy = false
		a.wfMu.Unlock()
		return
	}
	go func(it *queueItem) {
		defer func() {
			a.wfMu.Lock()
			a.iqBusy = false
			a.wfMu.Unlock()
		}()
		if err := a.processQueueItem(it); err != nil {
			log.Printf("integration queue %s: %v", it.id, err)
		}
	}(best)
}

func (a *App) processQueueItem(it *queueItem) error {
	lane, err := a.claimQueue(queueLane(it.repo, it.base))
	if err != nil || lane == nil {
		return err
	}
	defer func() {
		if err := lane.release(); err != nil {
			log.Printf("release queue lane: %v", err)
		}
	}()

	claim, err := a.claimQueue(it.id)
	if err != nil || claim == nil {
		return err
	}
	a.queueCheckpoint("after_claim")
	defer func() {
		if err := claim.release(); err != nil {
			log.Printf("release queue claim %s: %v", it.id, err)
		}
	}()
	if claim.effect.Candidate != nil {
		candidate := claim.effect.Candidate
		defer func() {
			if claim.effect.Phase == "done" {
				candidate.Close()
			}
		}()
		if err := a.Refs.RestorePrepared(candidate, claim.effect.CandidateDir); err != nil {
			return err
		}
		published, err := a.Refs.CandidatePublished(candidate)
		if err != nil {
			return err
		}
		if published {
			return a.finishQueuePublication(it, claim, candidate.CommitSHA)
		}
		if claim.effect.Phase == "publishing" {
			if err := claim.save("publishing"); err != nil {
				return err
			}
			result, err := a.Refs.PublishPrepared(candidate)
			if err != nil {
				return err
			}
			if result.Status != "ok" {
				claim.effect.Candidate = nil
				claim.effect.CandidateDir = ""
				if err := claim.save("queued"); err != nil {
					return err
				}
				candidate.Close()
				return a.patchQueue(it.id, map[string]any{"status": "queued", "error": "source or base moved; revalidation required", "updated_at": nowStr()})
			}
			return a.finishQueuePublication(it, claim, result.NewSHA)
		}
	}
	if err := claim.save("validating"); err != nil {
		return err
	}
	if err := a.patchQueue(it.id, map[string]any{"status": "running", "updated_at": nowStr()}); err != nil {
		return err
	}
	// policy gate 1: PR check must pass
	if !a.checkPassed(it.prID) {
		return a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "check_not_passed", "updated_at": nowStr()})
	}
	a.queueCheckpoint("after_checks")
	// policy gate 2: preview integration must be clean (structural/semantic)
	candidate, err := a.Refs.PrepareMerge(it.repo, it.base, it.branch, "integration queue "+it.prID)
	if err != nil {
		a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "preview_failed: " + err.Error(), "updated_at": nowStr()})
		return err
	}
	a.queueCheckpoint("after_preview")
	defer func() {
		if claim.effect.Phase == "done" || claim.effect.Phase == "blocked" || claim.effect.Candidate != candidate {
			candidate.Close()
		}
	}()
	if !a.checkPassedAt(it.prID, candidate.SourceSHA) {
		return a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "exact_source_check_required", "updated_at": nowStr()})
	}

	// git merge is textually clean: validate repo contracts on the merged tree.
	// A clean merge can still be a semantic conflict -> block.
	sem, err := semanticFindingsInTree(candidate.Dir)
	if err != nil {
		return err
	}
	if len(sem) > 0 {
		for _, f := range sem {
			if err := a.addFinding(it.prID, f["severity"].(string), f["message"].(string), f["file"].(string)); err != nil {
				return err
			}
		}
		if prs, _ := a.Trestle.ListRecords("prs", `id = "`+it.prID+`"`); len(prs) > 0 {
			if aid, _ := prs[0]["attempt_id"].(string); aid != "" {
				if err := a.patchAttempt(aid, map[string]any{"status": "semantic_conflict", "updated_at": nowStr()}); err != nil {
					return err
				}
			}
		}
		return a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "semantic_conflict", "updated_at": nowStr()})
	}
	a.queueCheckpoint("after_semantic")
	// policy gate: risk-based escalation (CP12)
	changed, err := candidate.ChangedFiles()
	if err != nil {
		return err
	}
	risk := "low"
	if len(changed) > 8 {
		risk = "high"
	} else if len(changed) > 2 {
		risk = "medium"
	}
	policyDecision, policyReason := a.immutableIntegrationPolicy(it.repo, risk)
	if policyDecision == "escalate" {
		if err := a.patchQueue(it.id, map[string]any{"status": "blocked", "error": policyReason, "updated_at": nowStr()}); err != nil {
			return err
		}
		if err := a.addFinding(it.prID, "error", policyReason, ""); err != nil {
			return err
		}
		return nil
	}
	if policyDecision == "deny" {
		return a.patchQueue(it.id, map[string]any{"status": "blocked", "error": policyReason, "updated_at": nowStr()})
	}
	// integrate (CAS-guarded canonical-head freshness)
	claim.effect.Candidate = candidate
	claim.effect.CandidateDir = candidate.Dir
	if err := claim.save("publishing"); err != nil {
		return err
	}
	a.queueCheckpoint("before_publication")
	res, err := a.Refs.PublishPrepared(candidate)
	if err != nil {
		if strings.Contains(err.Error(), "CONFLICT") {
			// canonical moved between preview and merge (or an external push):
			// route to the repair loop as a conflict.
			a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "merge_conflict", "updated_at": nowStr()})
			return nil
		}
		return err
	}
	if res.Status == "stale" {
		claim.effect.Candidate = nil
		claim.effect.CandidateDir = ""
		if err := claim.save("queued"); err != nil {
			return err
		}
		attempts := a.queueAttempts(it.id)
		if attempts+1 >= maxQueueAttempts {
			a.patchQueue(it.id, map[string]any{"status": "failed", "error": "stale after " + itoa(attempts+1) + " attempts", "updated_at": nowStr()})
			return nil
		}
		return a.patchQueue(it.id, map[string]any{"status": "queued", "attempts": itoa(attempts + 1), "error": "stale; requeued", "updated_at": nowStr()})
	}
	a.queueCheckpoint("after_publication")
	return a.finishQueuePublication(it, claim, res.NewSHA)
}

func (a *App) finishQueuePublication(it *queueItem, claim *queueClaim, sha string) error {
	claim.effect.PublishedSHA = sha
	if err := claim.save("reconciling"); err != nil {
		return err
	}
	a.queueCheckpoint("before_pr_completion")
	rid, ver, pr, err := a.Trestle.FindRecord("prs", filterEq("id", it.prID))
	if err != nil || rid == "" {
		return fmt.Errorf("read PR completion: %v", err)
	}
	if pr["status"] != "integrated" {
		if err := a.Trestle.PatchRecord("prs", rid, ver, map[string]any{"status": "integrated", "check_status": "pass", "integrated_at": nowStr()}); err != nil {
			return err
		}
	}
	a.queueCheckpoint("before_queue_completion")
	if err := a.patchQueue(it.id, map[string]any{"status": "done", "error": "", "updated_at": nowStr()}); err != nil {
		return err
	}
	return claim.save("done")
}

func (a *App) queueAttempts(id string) int {
	items, err := a.Trestle.ListRecords("iq", `id = "`+id+`"`)
	if err != nil || len(items) == 0 {
		return 0
	}
	n := 0
	fmt.Sscanf(strOf(items[0]["attempts"]), "%d", &n)
	return n
}

func strOf(v any) string {
	s, _ := v.(string)
	return s
}

func (a *App) queueCheckpoint(phase string) {
	if a.queueCrashHook != nil {
		a.queueCrashHook(phase)
	}
}

func queueLane(repo, base string) string { return "lane_" + sha256Hex([]byte(repo + "|" + base))[:32] }
