package app

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"switchyard/internal/refs"
)

// CP10: browser editor backend. Draft state is durable (Save != Commit): a
// draft survives reload and is only applied to Git when committed via the
// shared ref substrate.

func (a *App) handleSaveDraft(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Repo             string `json:"repo"`
		Branch           string `json:"branch"`
		Path             string `json:"path"`
		Content          string `json:"content"`
		ExpectedRevision string `json:"expected_revision"`
		BaseSHA          string `json:"base_sha"`
	}
	if err := readJSON(r, &in); err != nil || in.Repo == "" || in.Branch == "" || in.Path == "" {
		writeJSON(w, 400, map[string]any{"error": "repo_branch_path_required"})
		return
	}
	if err := refs.ValidatePath(in.Path); err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_repository_path"})
		return
	}
	draftID := "draft_" + sha256Hex([]byte(user + "|" + in.Repo + "|" + in.Branch + "|" + in.Path))[:10]
	at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	rid, ver, existing, err := a.Trestle.FindRecord("drafts", filterEq("id", draftID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "draft_read_failed"})
		return
	}
	if rid != "" {
		if existing["user"] != user {
			writeJSON(w, 403, map[string]any{"error": "draft_owner_required"})
			return
		}
		currentRev := draftRevision(existing)
		if in.ExpectedRevision == "" || atoiOr(in.ExpectedRevision, -1) != currentRev {
			writeJSON(w, 409, map[string]any{"error": "draft_stale", "current_revision": itoa(currentRev)})
			return
		}
		base := strOr(existing["base_sha"])
		if base == "" {
			base = in.BaseSHA
		}
		newRev := currentRev + 1
		if err := a.Trestle.PatchRecord("drafts", rid, ver, map[string]any{"content": in.Content, "revision": itoa(newRev), "base_sha": base, "updated_at": at, "committed_at": ""}); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "draft_save_failed"})
			return
		}
		writeJSON(w, 200, map[string]any{"id": draftID, "revision": newRev, "saved": true, "committed": false})
		return
	}
	if in.ExpectedRevision != "" {
		writeJSON(w, 409, map[string]any{"error": "draft_stale"})
		return
	}
	if in.BaseSHA == "" {
		writeJSON(w, 400, map[string]any{"error": "draft_base_sha_required"})
		return
	}
	_, _, err = a.Trestle.CreateRecord("drafts", map[string]any{
		"id": draftID, "repo": in.Repo, "branch": in.Branch, "path": in.Path,
		"content": in.Content, "user": user, "revision": "1", "base_sha": in.BaseSHA,
		"updated_at": at, "committed_at": "",
	}, "draft-"+draftID)
	if err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "draft_create_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"id": draftID, "revision": 1, "saved": true, "committed": false})
}

func draftRevision(d map[string]any) int {
	if s, ok := d["revision"].(string); ok && s != "" {
		return atoiOr(s, 1)
	}
	return 1
}

func atoiOr(s string, def int) int {
	n := 0
	neg := false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	return n
}

func (a *App) handleGetDraft(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	repo := r.PathValue("repo")
	branch := r.PathValue("branch")
	path := strings.TrimPrefix(r.URL.Path, "/api/drafts/"+repo+"/"+branch+"/")
	items, err := a.Trestle.ListRecords("drafts", `repo = "`+repo+`"`)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	for _, it := range items {
		if it["branch"] == branch && it["path"] == path && it["user"] == user {
			writeJSON(w, 200, map[string]any{
				"id": it["id"], "repo": repo, "branch": branch, "path": path, "content": it["content"],
				"revision": it["revision"], "base_sha": it["base_sha"], "last_agent_execution": it["last_agent_execution"], "updated_at": it["updated_at"],
			})
			return
		}
	}
	writeJSON(w, 404, map[string]any{"error": "no_draft"})
}

// handleCommitDraft applies a saved draft to Git via the shared ref substrate
// (Save != Commit is honored: only this endpoint changes the repository).
func (a *App) handleCommitDraft(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	draftID := r.PathValue("id")
	var in struct {
		Message          string `json:"message"`
		ExpectedRevision string `json:"expected_revision"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if in.Message == "" {
		in.Message = "edit via browser editor"
	}
	rid, ver, d, err := a.Trestle.FindRecord("drafts", filterEq("id", draftID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "draft_read_failed"})
		return
	}
	if rid == "" {
		writeJSON(w, 404, map[string]any{"error": "draft_not_found"})
		return
	}
	if d["user"] != user {
		writeJSON(w, 403, map[string]any{"error": "draft_owner_required"})
		return
	}
	revision := draftRevision(d)
	if in.ExpectedRevision == "" || atoiOr(in.ExpectedRevision, -1) != revision {
		writeJSON(w, 409, map[string]any{"error": "draft_stale", "current_revision": itoa(revision)})
		return
	}
	repo := d["repo"].(string)
	branch := d["branch"].(string)
	path := d["path"].(string)
	content := d["content"].(string)
	head, err := a.repoHead(repo, branch)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	// Race C: external branch movement — the draft was based on base_sha; if
	// the branch moved since, surface reconcile guidance instead of silently
	// committing on top of an unexpected base (the ref CAS substrate would
	// still reject, but the user needs to reconcile the draft, not lose it).
	if baseSHA, _ := d["base_sha"].(string); baseSHA == "" || baseSHA != head {
		writeJSON(w, 409, map[string]any{
			"error": "stale_base", "draft_base_sha": baseSHA, "current_head": head,
			"guidance": "the branch moved after your draft was based; reload the committed content and re-apply your edit",
		})
		return
	}
	prov := "editor:" + user + ":" + draftID
	if ae, _ := d["last_agent_execution"].(string); ae != "" {
		prov += " agent:" + ae
	}
	res, err := a.Refs.Update(repo, branch, head, []refs.Change{{Path: path, Content: content}}, in.Message, prov)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if res.Status != "ok" {
		writeJSON(w, 409, map[string]any{"error": "stale_base", "current_head": res.NewSHA})
		return
	}
	// PATCH against the exact snapshot used for Git. A concurrent save wins.
	cleanupErr := a.finishDraftCommit(rid, ver, revision, res.NewSHA)
	cleanup := "complete"
	if cleanupErr != nil {
		cleanup = "pending"
	}
	writeJSON(w, 200, map[string]any{"draft": draftID, "status": res.Status, "new_sha": res.NewSHA, "message": in.Message, "draft_cleanup": cleanup, "revision": revision + 1})
}

// handleDiff returns a unified diff between two file states (old vs new) using
// git diff --no-index in a scratch dir. This is the shared diff surface for the
// editor and the Agent panel.
func (a *App) handleDiff(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := readJSON(r, &in); err != nil || in.Path == "" {
		writeJSON(w, 400, map[string]any{"error": "path_required"})
		return
	}
	dir := scratchDir(a.DataDir, "diff-"+randHex(6))
	mkdirAll(dir, 0755)
	defer removeAll(dir)
	oldF := filepath.Join(dir, "old")
	newF := filepath.Join(dir, "new")
	_ = os.WriteFile(oldF, []byte(in.Old), 0644)
	_ = os.WriteFile(newF, []byte(in.New), 0644)
	out, err := runGitOut(dir, "diff", "--no-index", "--no-color", "old", "new")
	_ = out
	if err != nil {
		// git diff returns exit 1 for differences; exit 0 when identical
		gerr, ok := err.(*gitError)
		if !ok {
			writeJSON(w, 502, map[string]any{"error": "diff failed"})
			return
		}
		out = gerr.Out
	}
	writeJSON(w, 200, map[string]any{"path": in.Path, "diff": out})
}

var _ = time.RFC3339

// handleAgentProposeDraft runs the shared Agent runner against an explicit
// draft revision and CAS-applies the result back to working state. It never
// commits Git. Formal Attempts remain a separate autonomous path.
func (a *App) handleAgentProposeDraft(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	draftID := r.PathValue("id")
	var in struct {
		Prompt           string `json:"prompt"`
		ExpectedRevision string `json:"expected_revision"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Prompt) == "" {
		writeJSON(w, 400, map[string]any{"error": "prompt_required"})
		return
	}
	items, err := a.Trestle.ListRecords("drafts", `id = "`+draftID+`"`)
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "draft_not_found"})
		return
	}
	d := items[0]
	if d["user"] != user {
		writeJSON(w, 403, map[string]any{"error": "draft_owner_required"})
		return
	}
	startRev := draftRevision(d)
	if in.ExpectedRevision == "" || atoiOr(in.ExpectedRevision, -1) != startRev {
		writeJSON(w, 409, map[string]any{"error": "draft_stale", "current_revision": itoa(startRev)})
		return
	}
	path, _ := d["path"].(string)
	cur, _ := d["content"].(string)
	appendLine := "\n// Agent proposal: " + strings.TrimSpace(in.Prompt) + "\n"
	ex, err := a.runViaSubstrate("implementer", "draft:"+draftID, strOr(d["repo"]), strOr(d["branch"]), path, cur, appendLine)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	proposed := ""
	if ex.Result != nil {
		proposed = ex.Result[path]
	}
	if proposed == "" {
		writeJSON(w, 502, map[string]any{"error": "agent_produced_no_change"})
		return
	}
	// Re-read after the potentially long Agent call. A newer human save wins.
	rid, ver, vals, err := a.Trestle.FindRecord("drafts", `id = "`+draftID+`"`)
	if err != nil || rid == "" {
		writeJSON(w, 404, map[string]any{"error": "draft_not_found"})
		return
	}
	currentRev := draftRevision(vals)
	if currentRev != startRev {
		writeJSON(w, 409, map[string]any{"error": "draft_stale", "expected_revision": itoa(startRev), "current_revision": itoa(currentRev), "execution": ex.ID})
		return
	}
	newRev := currentRev + 1
	if err := a.Trestle.PatchRecord("drafts", rid, ver, map[string]any{"content": proposed, "revision": itoa(newRev), "last_agent_execution": ex.ID, "last_agent_prompt": strings.TrimSpace(in.Prompt), "updated_at": nowStr()}); err != nil {
		writeJSON(w, 409, map[string]any{"error": "draft_stale", "execution": ex.ID})
		return
	}
	writeJSON(w, 200, map[string]any{"id": draftID, "revision": newRev, "content": proposed, "execution": ex.ID, "status": "proposed", "committed": false})
}

func (a *App) finishDraftCommit(id, version string, revision int, sha string) error {
	// Keep content for recovery instead of destroying an edit. The revision advances
	// only with this conditional write; failure leaves the entire draft untouched.
	return a.Trestle.PatchRecord("drafts", id, version, map[string]any{"committed_at": nowStr(), "base_sha": sha, "revision": itoa(revision + 1)})
}
func draftPersistenceStatus(err error) int {
	if strings.Contains(err.Error(), ": 409 ") || strings.Contains(err.Error(), ": 412 ") {
		return 409
	}
	return 502
}
