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
	data, err := a.Artifacts.RawFile(repo, branch, "ATTEMPT.md")
	if err != nil {
		a.setCheck(prID, "fail", "marker file missing: "+err.Error())
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
	a.setCheck(prID, status, detail)
	writeJSON(w, 200, map[string]any{"check": status, "lines": lines, "detail": detail})
}

// handlePRIntegrate merges the PR branch into the base branch (canonical) via
// the ref substrate, only if the deterministic check passed.
func (a *App) handlePRIntegrate(w http.ResponseWriter, r *http.Request) {
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
	res, err := a.Refs.MergeBranch(repo, base, branch, "integrate PR "+prID, "user:"+user+" pr:"+prID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if res.Status == "stale" {
		writeJSON(w, 409, map[string]any{"error": "base moved; retry integration"})
		return
	}
	// mark PR integrated
	if id, ver, _, e := a.Trestle.FindRecord("prs", `id = "`+prID+`"`); e == nil && id != "" {
		_ = a.Trestle.PatchRecord("prs", id, ver, map[string]any{"status": "integrated", "check_status": "pass", "integrated_at": time.Now().UTC().Format(time.RFC3339)})
	}
	writeJSON(w, 200, map[string]any{"pr": prID, "status": "integrated", "new_base_sha": res.NewSHA})
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

func (a *App) setCheck(prID, status, detail string) {
	_, _, _ = a.Trestle.CreateRecord("pr_checks", map[string]any{
		"pr_id": prID, "status": status, "detail": detail, "created_at": time.Now().UTC().Format(time.RFC3339),
	}, "prcheck-"+prID+"-"+time.Now().UTC().Format("20060102150405"))
}

func (a *App) checkPassed(prID string) bool {
	items, err := a.Trestle.ListRecords("pr_checks", `pr_id = "`+prID+`"`)
	if err != nil || len(items) == 0 {
		return false
	}
	// the most recently created check must pass
	latest := items[len(items)-1]
	return latest["status"] == "pass"
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
	checks, _ := a.Trestle.ListRecords("pr_checks", `pr_id = "`+id+`"`)
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
	writeJSON(w, 200, map[string]any{"pr": pr, "checks": checks, "findings": findings, "queue": queue, "attempt": attempt, "work": work, "files": files, "diff": diff})
}
