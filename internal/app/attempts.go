package app

import (
	"net/http"
	"strings"
	"time"

	"switchyard/internal/agent"
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

// handleRunAttempt runs an Agent execution (role implementer) against the
// attempt branch and applies the produced change through the ref substrate.
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

	ex, err := a.runViaSubstrate("implementer", attemptID, repo, branch, file, string(cur),
		"\n// run by "+user+" at "+time.Now().UTC().Format(time.RFC3339)+"\n")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	newContent := ""
	if ex.Result != nil {
		newContent = ex.Result[file]
	}
	if newContent == "" {
		writeJSON(w, 502, map[string]any{"error": "agent produced no change"})
		return
	}
	res, err := a.Refs.Update(repo, branch, head, []refs.Change{{Path: file, Content: newContent}},
		"attempt "+attemptID+" run", "user:"+user)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _, _ = a.Trestle.CreateRecord("runs", map[string]any{
		"attempt_id": attemptID, "repo": repo, "branch": branch, "new_sha": res.NewSHA,
		"message": "agent run (" + ex.Adapter + ")", "created_at": now,
	}, "run-"+attemptID+"-"+res.NewSHA[:10])
	writeJSON(w, 200, map[string]any{"attempt_id": attemptID, "execution": ex.ID, "status": res.Status, "new_sha": res.NewSHA, "adapter": ex.Adapter, "output": ex.Output})
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

// runViaSubstrate runs an Agent execution through the runner adapter and
// records it durably.
func (a *App) runViaSubstrate(role, attemptID, repo, branch, file, currentContent, appendLine string) (*agent.Execution, error) {
	now := time.Now().UTC()
	ex := &agent.Execution{ID: "exe_" + randHex(8), Role: role, AttemptID: attemptID, Adapter: "deterministic", Status: "running", Started: now}
	task := map[string]any{"file": file, "append": appendLine, "current": currentContent}
	err := a.Runner.Run(ex, task)
	ex.Finished = time.Now().UTC()
	status := "succeeded"
	if err != nil {
		status = "failed"
	}
	_, _, _ = a.Trestle.CreateRecord("executions", map[string]any{
		"id": ex.ID, "role": ex.Role, "attempt_id": ex.AttemptID, "adapter": ex.Adapter,
		"status": status, "output": ex.Output, "started_at": ex.Started.Format(time.RFC3339),
		"finished_at": ex.Finished.Format(time.RFC3339),
	}, "exec-"+ex.ID)
	if err != nil {
		return nil, err
	}
	return ex, nil
}

func randHex(n int) string {
	const d = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = d[(time.Now().UnixNano()>>uint(i*4))%16]
	}
	return string(b)
}