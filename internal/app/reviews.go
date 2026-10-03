package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// CP8: structured review findings + draft contention + preview-based conflict
// detection + conflict routing to the conflict-resolver role (bounded repair
// loop). Review is deterministic (the real provider-backed reviewer is gated on
// CP6's bounded condition); findings are durable and machine-actionable.

// handleReviewAttempt runs the reviewer role against an attempt branch and
// records structured findings (severity, message, file).
func (a *App) handleReviewAttempt(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	attemptID := r.PathValue("id")
	attempt := a.attemptByID(r, attemptID)
	if attempt == nil {
		writeJSON(w, 404, map[string]any{"error": "attempt_not_found"})
		return
	}
	repo := attempt["repo"].(string)
	branch := attempt["branch"].(string)
	findings, err := a.deterministicReview(repo, branch, attemptID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	for _, f := range findings {
		if err := a.addFinding(attemptID, f["severity"].(string), f["message"].(string), f["file"].(string)); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
			return
		}
	}
	// record the review execution through the substrate (reviewer role)
	ex := &ExecutionRecord{ID: "exe_" + randHex(8), Role: "reviewer", AttemptID: attemptID, Adapter: "deterministic", Status: "succeeded", Output: "reviewed", Started: time.Now().UTC()}
	if err := a.recordExecution(ex); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "findings": findings, "execution": ex.ID, "reviewed_by": user})
}

// deterministicReview produces machine findings for an attempt branch:
// structural (changed-file inventory + marker quality) and semantic (empty
// marker / scope size). It returns finding maps {severity, message, file}.
func (a *App) deterministicReview(repo, branch, attemptID string) ([]map[string]any, error) {
	changed, err := a.changedFiles(repo, branch)
	if err != nil {
		return nil, err
	}
	findings := []map[string]any{}
	if len(changed) == 0 {
		return findings, nil
	}
	findings = append(findings, map[string]any{"severity": "info", "message": "attempt touches " + itoa(len(changed)) + " file(s)", "file": ""})
	for _, f := range changed {
		data, err := a.Artifacts.RawFile(repo, branch, f)
		if err != nil {
			findings = append(findings, map[string]any{"severity": "warning", "message": "file could not be read", "file": f})
			continue
		}
		if len(data) > 32<<10 {
			findings = append(findings, map[string]any{"severity": "info", "message": "file approaches the 32 MB Artifacts limit", "file": f})
		}
		if f == "ATTEMPT.md" {
			lines := len(strings.Split(strings.TrimSpace(string(data)), "\n"))
			if lines < 2 {
				findings = append(findings, map[string]any{"severity": "warning", "message": "ATTEMPT.md lacks an init header + run line (structural)", "file": f})
			}
		}
	}
	return findings, nil
}

// handlePreview runs preview integration: does the attempt branch merge
// cleanly into the current base? If not, structural/semantic conflict findings
// are recorded and the attempt is routed to conflict.
func (a *App) handlePreview(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	attemptID := r.PathValue("id")
	attempt := a.attemptByID(r, attemptID)
	if attempt == nil {
		writeJSON(w, 404, map[string]any{"error": "attempt_not_found"})
		return
	}
	repo := attempt["repo"].(string)
	branch := attempt["branch"].(string)
	rr, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	base := rr.DefaultBranch
	previewStarted := time.Now()
	mergedTree, conflicts, err := a.Refs.PreviewMergedTreeWithConflicts(repo, base, branch)
	previewDuration := time.Since(previewStarted)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if mergedTree != "" {
		defer removeAll(mergedTree)
	}
	if len(conflicts) > 0 {
		for _, f := range conflicts {
			if err := a.addFinding(attemptID, "error", "merge conflict with "+base+" in "+f, f); err != nil {
				writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
				return
			}
		}
		if err := a.patchAttempt(attemptID, map[string]any{"status": "conflict", "updated_at": nowStr()}); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
			return
		}
		writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "conflict": true, "conflicts": conflicts, "status": "conflict", "resolved_by": user})
		return
	}
	// git merge is textually clean: validate repo contracts on the merged tree
	// (semantic-conflict detection). A clean merge can still be incompatible.
	semanticStarted := time.Now()
	sem, err := semanticFindingsInTree(mergedTree)
	w.Header().Set("Server-Timing", fmt.Sprintf("git_preview;dur=%.3f, semantic;dur=%.3f", float64(previewDuration.Microseconds())/1000, float64(time.Since(semanticStarted).Microseconds())/1000))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "semantic check failed: " + err.Error()})
		return
	}
	if len(sem) > 0 {
		for _, f := range sem {
			if err := a.addFinding(attemptID, f["severity"].(string), f["message"].(string), f["file"].(string)); err != nil {
				writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
				return
			}
		}
		if err := a.patchAttempt(attemptID, map[string]any{"status": "semantic_conflict", "updated_at": nowStr()}); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
			return
		}
		writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "conflict": false, "semantic_conflict": true, "findings": sem, "status": "semantic_conflict"})
		return
	}
	writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "conflict": false, "conflicts": []string{}, "semantic_conflict": false, "status": "ok"})
}

// handleResolveConflict routes a conflicted attempt to the conflict-resolver
// role: a deterministic three-way merge (git merge-file) repairs the source
// branch so a subsequent preview/integration is clean. Findings are closed as
// resolved. Bounded repair loop: one resolution pass.
func (a *App) handleResolveConflict(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	attemptID := r.PathValue("id")
	attempt := a.attemptByID(r, attemptID)
	if attempt == nil {
		writeJSON(w, 404, map[string]any{"error": "attempt_not_found"})
		return
	}
	status, _ := attempt["status"].(string)
	if status != "conflict" {
		writeJSON(w, 409, map[string]any{"error": "attempt_not_in_conflict"})
		return
	}
	repo := attempt["repo"].(string)
	branch := attempt["branch"].(string)
	rr, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	base := rr.DefaultBranch
	res, resolved, err := a.Refs.ResolveIntoSource(repo, base, branch, "resolve attempt "+attemptID, "conflict-resolver:"+user+":"+attemptID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	for _, f := range resolved {
		if err := a.closeFinding(attemptID, f, "resolved by conflict-resolver three-way merge"); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
			return
		}
	}
	if err := a.patchAttempt(attemptID, map[string]any{"status": "resolved", "message": "resolved by conflict-resolver", "updated_at": nowStr()}); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
		return
	}
	ex := &ExecutionRecord{ID: "exe_" + randHex(8), Role: "conflict-resolver", AttemptID: attemptID, Adapter: "deterministic", Status: "succeeded", Output: "resolved " + itoa(len(resolved)) + " file(s)", Started: time.Now().UTC()}
	if err := a.recordExecution(ex); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "review_persistence_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "status": "resolved", "resolved_files": resolved, "new_sha": res.NewSHA, "resolved_by": user})
}

// handleListFindings returns findings for a target (attempt id) or all.
func (a *App) handleListFindings(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("findings", "")
	items = a.visibleRecords("findings", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) addFinding(target, severity, message, file string) error {
	_, _, err := a.Trestle.CreateRecord("findings", map[string]any{
		"id": "fnd_" + randHex(10), "target": target, "severity": severity,
		"message": message, "file": file, "status": "open", "created_at": nowStr(), "resolved_at": "",
	}, "finding-"+target+"-"+randHex(6))
	return err
}

func (a *App) closeFinding(target, file, note string) error {
	items, err := a.Trestle.ListRecords("findings", filterEq("target", target))
	if err != nil {
		return err
	}
	for _, it := range items {
		if it["file"] != file || it["status"] != "open" {
			continue
		}
		id, ver, current, err := a.Trestle.FindRecord("findings", filterEq("id", strOr(it["id"])))
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("finding disappeared")
		}
		if current["status"] != "open" {
			continue
		}
		if err := a.Trestle.PatchRecord("findings", id, ver, map[string]any{"status": "resolved", "resolved_at": nowStr()}); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) patchAttempt(id string, values map[string]any) error {
	rid, ver, _, err := a.Trestle.FindRecord("attempts", filterEq("id", id))
	if err != nil {
		return err
	}
	if rid == "" {
		return fmt.Errorf("attempt not found")
	}
	return a.Trestle.PatchRecord("attempts", rid, ver, values)
}

// changedFiles lists files the attempt branch changed relative to base via a
// scratch diff (name-status only).
func (a *App) changedFiles(repo, branch string) ([]string, error) {
	rr, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		return nil, err
	}
	base := rr.DefaultBranch
	dir := scratchDir(a.DataDir, "diff-"+repo+"-"+branch)
	removeAll(dir)
	mkdirAll(dir, 0755)
	tok, err := a.Refs.GitToken(repo)
	if err != nil {
		return nil, err
	}
	auth := []string{"-c", "http.extraHeader=Authorization: Bearer " + tok}
	if err := runGit(dir, "", append(auth, "clone", "--quiet", "--no-checkout", "--branch", base, rr.Remote, dir)...); err != nil {
		return nil, err
	}
	if err := runGit(dir, "", append(auth, "fetch", "--quiet", rr.Remote, branch+":refs/remotes/src")...); err != nil {
		return nil, err
	}
	out, err := runGitOut(dir, append(auth, "diff", "--name-status", base, "src")...)
	if err != nil {
		return nil, err
	}
	changed := []string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] != "D" {
			changed = append(changed, fields[len(fields)-1])
		}
	}
	return changed, nil
}
