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
	repo := pr["repo"].(string)
	branch := pr["branch"].(string)
	base := pr["base"].(string)
	risk := "low"
	if changed, err := a.changedFiles(repo, branch); err == nil {
		if len(changed) > 8 {
			risk = "high"
		} else if len(changed) > 2 {
			risk = "medium"
		}
	}
	qid := "iq_" + randHex(10)
	at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, _, err := a.Trestle.CreateRecord("iq", map[string]any{
		"id": qid, "pr_id": prID, "repo": repo, "base": base, "branch": branch,
		"status": "queued", "risk": risk, "policy": "check_pass + preview_clean",
		"attempts": "0", "error": "", "created_at": at, "updated_at": nowStr(),
	}, "iq-"+qid)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	log.Printf("integration queue: %s enqueued (risk=%s) by %s", qid, risk, user)
	writeJSON(w, 201, map[string]any{"id": qid, "pr_id": prID, "status": "queued", "risk": risk, "policy": "check_pass + preview_clean"})
}

func (a *App) handleListQueue(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("iq", "")
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
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "item_not_found"})
		return
	}
	a.patchQueue(qid, map[string]any{"status": "queued", "updated_at": nowStr()})
	log.Printf("integration queue: %s re-queued by %s", qid, user)
	writeJSON(w, 200, map[string]any{"id": qid, "status": "queued"})
}

func (a *App) patchQueue(id string, values map[string]any) {
	if rid, ver, _, err := a.Trestle.FindRecord("iq", `id = "`+id+`"`); err == nil && rid != "" {
		_ = a.Trestle.PatchRecord("iq", rid, ver, values)
	}
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
		if st != "queued" {
			continue
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
	// mark running
	a.patchQueue(it.id, map[string]any{"status": "running", "updated_at": nowStr()})

	// policy gate 1: PR check must pass
	if !a.checkPassed(it.prID) {
		a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "check_not_passed", "updated_at": nowStr()})
		return nil
	}
	// policy gate 2: preview integration must be clean (structural/semantic)
	conflicts, err := a.Refs.PreviewMerge(it.repo, it.base, it.branch)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		for _, f := range conflicts {
			a.addFinding(it.prID, "error", "integration preview conflict in "+f, f)
		}
		// route the underlying attempt to conflict so the conflict-resolver
		// repair loop is uniform with the preview endpoint.
		if prs, _ := a.Trestle.ListRecords("prs", `id = "`+it.prID+`"`); len(prs) > 0 {
			if aid, _ := prs[0]["attempt_id"].(string); aid != "" {
				a.patchAttempt(aid, map[string]any{"status": "conflict", "updated_at": nowStr()})
			}
		}
		a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "preview_conflict: " + fmt.Sprint(conflicts), "updated_at": nowStr()})
		return nil
	}
	// git merge is textually clean: validate repo contracts on the merged tree.
	// A clean merge can still be a semantic conflict -> block.
	sem, err := a.semanticFindings(it.repo, it.branch)
	if err != nil {
		return err
	}
	if len(sem) > 0 {
		for _, f := range sem {
			a.addFinding(it.prID, f["severity"].(string), f["message"].(string), f["file"].(string))
		}
		if prs, _ := a.Trestle.ListRecords("prs", `id = "`+it.prID+`"`); len(prs) > 0 {
			if aid, _ := prs[0]["attempt_id"].(string); aid != "" {
				a.patchAttempt(aid, map[string]any{"status": "semantic_conflict", "updated_at": nowStr()})
			}
		}
		a.patchQueue(it.id, map[string]any{"status": "blocked", "error": "semantic_conflict", "updated_at": nowStr()})
		return nil
	}
	// policy gate: risk-based escalation (CP12)
	policyDecision, policyReason := a.policyGateIntegrate(it.repo, it.risk)
	if policyDecision == "escalate" {
		a.patchQueue(it.id, map[string]any{"status": "blocked", "error": policyReason, "updated_at": nowStr()})
		a.addFinding(it.prID, "error", policyReason, "")
		return nil
	}
	if policyDecision == "deny" {
		a.patchQueue(it.id, map[string]any{"status": "blocked", "error": policyReason, "updated_at": nowStr()})
		return nil
	}
	// integrate (CAS-guarded canonical-head freshness)
	res, err := a.Refs.MergeBranch(it.repo, it.base, it.branch, "integration queue "+it.prID, "queue:"+it.id+":"+it.prID)
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
		attempts := a.queueAttempts(it.id)
		if attempts+1 >= maxQueueAttempts {
			a.patchQueue(it.id, map[string]any{"status": "failed", "error": "stale after " + itoa(attempts+1) + " attempts", "updated_at": nowStr()})
			return nil
		}
		a.patchQueue(it.id, map[string]any{"status": "queued", "attempts": itoa(attempts + 1), "error": "stale; requeued", "updated_at": nowStr()})
		return nil
	}
	// mark PR integrated
	if id, ver, _, e := a.Trestle.FindRecord("prs", `id = "`+it.prID+`"`); e == nil && id != "" {
		_ = a.Trestle.PatchRecord("prs", id, ver, map[string]any{"status": "integrated", "check_status": "pass", "integrated_at": nowStr()})
	}
	a.patchQueue(it.id, map[string]any{"status": "done", "error": "", "updated_at": nowStr()})
	log.Printf("integration queue: %s integrated %s -> %s", it.id, it.branch, res.NewSHA[:12])
	return nil
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