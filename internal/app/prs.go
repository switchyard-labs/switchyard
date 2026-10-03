package app

import (
	"net/http"
	"strings"
	"time"
)

func (a *App) handleOpenPR(w http.ResponseWriter, r *http.Request) {
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
	var in struct {
		Title string `json:"title"`
	}
	_ = readJSON(r, &in)
	repo := attempt["repo"].(string)
	branch := attempt["branch"].(string)
	rr, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	prID := newWorkID()
	title := in.Title
	if title == "" {
		title = "Change from attempt " + attemptID
	}
	_, _, err = a.Trestle.CreateRecord("prs", map[string]any{
		"id": prID, "attempt_id": attemptID, "work_id": attempt["work_id"],
		"repo": repo, "branch": branch, "base": rr.DefaultBranch, "title": title,
		"status": "open", "check_status": "pending", "created_at": time.Now().UTC().Format(time.RFC3339), "integrated_at": "",
	}, "pr-"+prID)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": prID, "repo": repo, "branch": branch, "base": rr.DefaultBranch, "title": title, "status": "open", "proposed_by": user})
}

// handlePRCheck runs a deterministic validation: the attempt branch's marker
// file must contain an init header plus at least one run line.
func (a *App) handlePRCheck(w http.ResponseWriter, r *http.Request) {
	prID := r.PathValue("id")
	pr := a.prByID(r, prID)
	if pr == nil {
		writeJSON(w, 404, map[string]any{"error": "pr_not_found"})
		return
	}
	repo := pr["repo"].(string)
	branch := pr["branch"].(string)
	_, sourceSHA, err := a.Refs.Snapshot(repo, strOf(pr["base"]), branch)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	data, err := a.Artifacts.RawFile(repo, sourceSHA, "ATTEMPT.md")
	if err != nil {
		if err := a.setCommitCheck(prID, repo, sourceSHA, "fail", "marker file missing"); err != nil {
			writeJSON(w, 502, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"check": "fail", "reason": "marker file missing"})
		return
	}
	lines := len(strings.Split(strings.TrimSpace(string(data)), "\n"))
	status := "pass"
	detail := "marker file valid"
	if lines < 2 {
		status = "fail"
		detail = "expected at least 2 lines (init + run), got " + string(rune('0'+min(lines, 9)))
	}
	if err := a.setCommitCheck(prID, repo, sourceSHA, status, detail); err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"check": status, "lines": lines, "detail": detail})
}

// handlePRIntegrate merges the PR branch into the base branch (canonical) via
// the ref substrate, only if the deterministic check passed.
func (a *App) handlePRIntegrate(w http.ResponseWriter, r *http.Request) {
	a.handleEnqueuePR(w, r)
}

func (a *App) handleListPRs(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	filter := ""
	if repo := strings.TrimSpace(r.URL.Query().Get("repo")); repo != "" {
		filter = `repo = "` + repo + `"`
	}
	items, err := a.Trestle.ListRecords("prs", filter)
	items = a.visibleRecords("prs", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) prByID(r *http.Request, id string) map[string]any {
	items, err := a.Trestle.ListRecords("prs", `id = "`+id+`"`)
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (a *App) setCommitCheck(prID, repo, sha, status, detail string) error {
	_, _, err := a.Trestle.CreateRecord("commit_checks", map[string]any{
		"pr_id": prID, "repo": repo, "source_sha": sha, "status": status, "detail": detail, "created_at": nowStr(),
	}, "commit-check-"+prID+"-"+sha+"-"+randHex(10))
	return err
}

func (a *App) checkPassedAt(prID, sha string) bool {
	_, _, pr, err := a.Trestle.FindRecord("prs", filterEq("id", prID))
	if err != nil || pr == nil {
		return false
	}
	repo := strOf(pr["repo"])
	_, _, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", repo))
	if err != nil {
		return false
	}
	if settings != nil {
		var required []string
		if decodeAction(settings["required_jobs"], &required) != nil {
			return false
		}
		if len(required) > 0 {
			return a.requiredActionsPassed(repo, sha)
		}
	}
	items, err := a.Trestle.ListRecords("commit_checks", filterEq("pr_id", prID)+" && "+filterEq("source_sha", sha))
	if err != nil {
		return false
	}
	var latest map[string]any
	for _, item := range items {
		if latest == nil || strOf(item["created_at"]) > strOf(latest["created_at"]) {
			latest = item
		}
	}
	if latest == nil || latest["status"] != "pass" {
		return false
	}
	return latest["repo"] == repo
}

func (a *App) checkPassed(prID string) bool {
	items, err := a.Trestle.ListRecords("prs", filterEq("id", prID))
	if err != nil || len(items) != 1 {
		return false
	}
	pr := items[0]
	_, sha, err := a.Refs.Snapshot(strOf(pr["repo"]), strOf(pr["base"]), strOf(pr["branch"]))
	return err == nil && a.checkPassedAt(prID, sha)
}

// handleGetPR exposes a product-shaped pull request snapshot. Familiar Git
// collaboration data is first; Switchyard-specific attempt/findings/queue data
// is layered on top when present.
func (a *App) handleGetPR(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	pr := a.prByID(r, id)
	if pr == nil {
		writeJSON(w, 404, map[string]any{"error": "pr_not_found"})
		return
	}
	checks, _ := a.Trestle.ListRecords("commit_checks", filterEq("pr_id", id))
	findings, _ := a.Trestle.ListRecords("findings", `target = "`+id+`"`)
	queue, _ := a.Trestle.ListRecords("iq", `pr_id = "`+id+`"`)
	attempt := map[string]any(nil)
	if aid := strOr(pr["attempt_id"]); aid != "" {
		if xs, _ := a.Trestle.ListRecords("attempts", `id = "`+aid+`"`); len(xs) > 0 {
			attempt = xs[0]
		}
	}
	work := map[string]any(nil)
	if wid := strOr(pr["work_id"]); wid != "" {
		if xs, _ := a.Trestle.ListRecords("work", `id = "`+wid+`"`); len(xs) > 0 {
			work = xs[0]
		}
	}
	diff, files := "", []string{}
	if repo, base, branch := strOr(pr["repo"]), strOr(pr["base"]), strOr(pr["branch"]); repo != "" && base != "" && branch != "" {
		diff, files, _ = a.Refs.DiffRefs(repo, base, branch)
	}
	actionChecks := []map[string]any{}
	sourceSHA := ""
	var err error
	if a.Refs != nil {
		_, sourceSHA, err = a.Refs.Snapshot(strOf(pr["repo"]), strOf(pr["base"]), strOf(pr["branch"]))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "pr_source_unavailable"})
			return
		}
		actionChecks, err = a.Trestle.ListRecords("action_checks", filterEq("repo", strOf(pr["repo"]))+" && "+filterEq("source_sha", sourceSHA))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "pr_actions_unavailable"})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"pr": pr, "checks": checks, "action_checks": actionChecks, "source_sha": sourceSHA, "findings": findings, "queue": queue, "attempt": attempt, "work": work, "files": files, "diff": diff})
}
