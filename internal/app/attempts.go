package app

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"switchyard/internal/refs"
)

// handleCreateAttempt creates an Attempt and its isolated branch on a repo.
func (a *App) handleCreateAttempt(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	workID := r.PathValue("id")
	var in struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
	}
	if err := readJSON(r, &in); err != nil || in.Repo == "" {
		writeJSON(w, 400, map[string]any{"error": "repo required"})
		return
	}
	attemptID := newWorkID()
	if in.Branch == "" {
		in.Branch = "attempt-" + strings.TrimPrefix(attemptID, "wk_")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	// create the isolated branch from the repo default with a marker commit
	res, err := a.Refs.Update(in.Repo, in.Branch, "", []refs.Change{
		{Path: "ATTEMPT.md", Content: "# Attempt " + attemptID + "\n"},
	}, "attempt "+attemptID+" init", "user:"+user)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	_, _, err = a.Trestle.CreateRecord("attempts", map[string]any{
		"id": attemptID, "work_id": workID, "repo": in.Repo, "branch": in.Branch,
		"status": "created", "owner": user, "created_at": now, "updated_at": now, "message": res.Status,
	}, "attempt-"+attemptID)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": attemptID, "work_id": workID, "repo": in.Repo, "branch": in.Branch, "status": "created"})
}

// handleRunAttempt runs the deterministic Strut worker against the attempt
// branch and applies the produced change through the ref substrate.
func (a *App) handleRunAttempt(w http.ResponseWriter, r *http.Request) {
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
	file := "ATTEMPT.md"

	// current head + current file content
	refsMap, err := a.repoRefs(repo)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	head := refsMap["refs/heads/"+branch]
	cur, err := a.Artifacts.RawFile(repo, branch, file)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "file_not_found"})
		return
	}

	// run the Strut worker deterministically
	tmp, err := os.MkdirTemp(a.DataDir, "worker-")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	defer os.RemoveAll(tmp)
	input := filepath.Join(tmp, "in.txt")
	output := filepath.Join(tmp, "out.txt")
	if err := os.WriteFile(input, cur, 0644); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	task := map[string]any{"input_path": input, "output_path": output, "append": "\n// run by " + user + " at " + time.Now().UTC().Format(time.RFC3339) + "\n"}
	tb, _ := json.Marshal(task)
	taskFile := filepath.Join(tmp, "task.json")
	if err := os.WriteFile(taskFile, tb, 0644); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	cmd := exec.Command(a.StrutBin)
	cmd.Env = append(os.Environ(), "SWITCHYARD_TASK="+taskFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		writeJSON(w, 502, map[string]any{"error": "worker: " + strings.TrimSpace(string(out))})
		return
	}
	newContent, err := os.ReadFile(output)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "worker output missing"})
		return
	}

	res, err := a.Refs.Update(repo, branch, head, []refs.Change{{Path: file, Content: string(newContent)}},
		"attempt "+attemptID+" run", "user:"+user)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _, _ = a.Trestle.CreateRecord("runs", map[string]any{
		"attempt_id": attemptID, "repo": repo, "branch": branch, "new_sha": res.NewSHA,
		"message": "deterministic run", "created_at": now,
	}, "run-"+attemptID+"-"+res.NewSHA[:10])
	writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "status": res.Status, "new_sha": res.NewSHA, "new_len": len(newContent)})
}

func (a *App) attemptByID(r *http.Request, id string) map[string]any {
	items, err := a.Trestle.ListRecords("attempts", `id = "`+id+`"`)
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (a *App) repoRefs(repo string) (map[string]string, error) {
	r, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		return nil, err
	}
	tok, err := a.Refs.GitToken(repo)
	if err != nil {
		return nil, err
	}
	return a.Artifacts.LsRemoteWithToken(r.Remote, tok)
}