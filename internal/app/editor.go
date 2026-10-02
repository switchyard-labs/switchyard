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
		Repo    string `json:"repo"`
		Branch  string `json:"branch"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := readJSON(r, &in); err != nil || in.Repo == "" || in.Branch == "" || in.Path == "" {
		writeJSON(w, 400, map[string]any{"error": "repo_branch_path_required"})
		return
	}
	draftID := "draft_" + sha256Hex([]byte(user+"|"+in.Repo+"|"+in.Branch+"|"+in.Path))[:10]
	at := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	// upsert: unique idempotency key per save; if the deterministic id already
	// exists (unique), patch the existing draft instead.
	_, _, err := a.Trestle.CreateRecord("drafts", map[string]any{
		"id": draftID, "repo": in.Repo, "branch": in.Branch, "path": in.Path,
		"content": in.Content, "user": user, "updated_at": at,
	}, "draft-"+draftID+"-"+at)
	if err != nil {
		if id, ver, _, e := a.Trestle.FindRecord("drafts", `id = "`+draftID+`"`); e == nil && id != "" {
			_ = a.Trestle.PatchRecord("drafts", id, ver, map[string]any{"content": in.Content, "updated_at": at})
			writeJSON(w, 200, map[string]any{"id": draftID, "saved": true, "patched": true, "saved_at": at, "committed": false})
			return
		}
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"id": draftID, "saved": true, "saved_at": at, "committed": false})
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
			writeJSON(w, 200, map[string]any{"repo": repo, "branch": branch, "path": path, "content": it["content"], "updated_at": it["updated_at"]})
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
		Message string `json:"message"`
	}
	_ = readJSON(r, &in)
	if in.Message == "" {
		in.Message = "edit via browser editor"
	}
	items, err := a.Trestle.ListRecords("drafts", `id = "`+draftID+`"`)
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "draft_not_found"})
		return
	}
	d := items[0]
	repo := d["repo"].(string)
	branch := d["branch"].(string)
	path := d["path"].(string)
	content := d["content"].(string)
	head, err := a.repoHead(repo, branch)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	res, err := a.Refs.Update(repo, branch, head, []refs.Change{{Path: path, Content: content}}, in.Message, "editor:"+user+":"+draftID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	// clear the draft after a successful commit
	if id, ver, _, e := a.Trestle.FindRecord("drafts", `id = "`+draftID+`"`); e == nil && id != "" {
		_ = a.Trestle.PatchRecord("drafts", id, ver, map[string]any{"committed_at": nowStr(), "content": ""})
	}
	writeJSON(w, 200, map[string]any{"draft": draftID, "status": res.Status, "new_sha": res.NewSHA, "message": in.Message})
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