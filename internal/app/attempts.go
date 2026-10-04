package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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
	var input struct {
		File   string `json:"file"`
		Prompt string `json:"prompt"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &input); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid agent task"})
			return
		}
	}
	file := input.File
	if file == "" {
		file = "ATTEMPT.md"
	}
	if refs.ValidatePath(file) != nil || len(input.Prompt) > 20000 {
		writeJSON(w, 400, map[string]any{"error": "invalid agent file or prompt"})
		return
	}
	if err := a.policyGateAgent("implementer", repo, file); err != nil {
		writeJSON(w, 403, map[string]any{"error": err.Error()})
		return
	}

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

	agentCtx, err := a.agentRequestContext(r, "implementer")
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	prompt := input.Prompt
	if prompt == "" {
		prompt = "\n// run by " + user + " at " + time.Now().UTC().Format(time.RFC3339) + "\n"
	}
	ex, res, err := a.runAgentStepContext(agentCtx, "implementer", attemptID, repo, branch, file, string(cur), prompt, head, "user:"+user)
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
	return a.runViaSubstrateContext(context.Background(), role, attemptID, repo, branch, file, currentContent, appendLine)
}

func (a *App) runViaSubstrateContext(parent context.Context, role, attemptID, repo, branch, file, currentContent, appendLine string) (*agent.Execution, error) {
	now := time.Now().UTC()
	ex := &agent.Execution{ID: "exe_" + randHex(8), Role: role, AttemptID: attemptID, Adapter: "deterministic", Status: "running", Started: now}
	task := agent.Task{Role: role, Repo: repo, Branch: branch, File: file, Current: currentContent, Prompt: appendLine}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if a.Runner == nil {
		return nil, fmt.Errorf("Agent runner unavailable")
	}
	runner := a.Runner
	credentialProfile := ""
	principal, _ := parent.Value(agentUserKey{}).(string)
	if user, _ := parent.Value(agentUserKey{}).(string); user != "" {
		selection, selectionErr := a.agentSelection(user, role)
		if override, ok := parent.Value(agentOverrideKey{}).(AgentSelection); ok {
			selection, selectionErr = override, nil
		}
		if selectionErr != nil {
			return nil, selectionErr
		}
		if selection.Provider != "" {
			if a.ProviderRunner == nil {
				return nil, fmt.Errorf("provider runner is not configured on this server")
			}
			runner, selectionErr = a.ProviderRunner(selection)
			if selectionErr != nil {
				return nil, selectionErr
			}
			task.Provider, task.Model = selection.Provider, selection.Model
			credentialProfile = selection.CredentialID
			ex.Adapter = selection.Provider
		}
	}
	err := runner.Run(ctx, ex, task)
	ex.Finished = time.Now().UTC()
	status := ex.Status
	if status == "" || status == "running" {
		status = "succeeded"
	}
	if err != nil && status == "succeeded" {
		status = "failed"
	}
	ex.Status = status
	_, _, persistErr := a.Trestle.CreateRecord("executions", map[string]any{
		"id": ex.ID, "role": ex.Role, "attempt_id": ex.AttemptID, "adapter": ex.Adapter,
		"status": status, "output": ex.Output, "started_at": ex.Started.Format(time.RFC3339),
		"finished_at": ex.Finished.Format(time.RFC3339),
	}, "exec-"+ex.ID)
	if persistErr == nil {
		_, _, persistErr = a.Trestle.CreateRecord("execution_metadata", map[string]any{"execution_id": ex.ID, "repo": repo, "metadata": map[string]any{"branch": branch, "file": file, "status": ex.Status, "resource_usage": ex.ResourceUsage, "failure_code": ex.FailureCode, "credential_profile": credentialProfile, "principal": principal, "exit_code": ex.ExitCode, "cpu_time_ns": int64(ex.CPUTime), "output_truncated": ex.OutputTruncated, "duration_ns": int64(ex.Finished.Sub(ex.Started)), "sandbox": task.Provider != "" || ex.Adapter == "cli-sandbox", "provider": task.Provider, "model": task.Model, "role": role}}, "execution-meta-"+ex.ID)
	}

	if persistErr != nil {
		return nil, fmt.Errorf("execution persistence: %w", persistErr)
	}
	if err != nil {
		return nil, err
	}
	return ex, nil
}

// runAgentStep runs an Agent execution through the substrate and commits the
// produced change to the branch via the ref substrate. It is the shared
// executor for the attempts API and the durable workflow engine. When
// expectedHead is non-empty the commit is CAS-guarded; an already-applied
// identical commit (same message) is treated as idempotent success.
func (a *App) runAgentStep(role, attemptID, repo, branch, file, currentContent, appendLine, expectedHead, provenance string) (*agent.Execution, *refs.Result, error) {
	return a.runAgentStepContext(context.Background(), role, attemptID, repo, branch, file, currentContent, appendLine, expectedHead, provenance)
}

func (a *App) runAgentStepContext(ctx context.Context, role, attemptID, repo, branch, file, currentContent, appendLine, expectedHead, provenance string) (*agent.Execution, *refs.Result, error) {
	ex, err := a.runViaSubstrateContext(ctx, role, attemptID, repo, branch, file, currentContent, appendLine)
	if err != nil {
		return nil, nil, err
	}
	newContent, ok := ex.Result[file]
	if !ok {
		return nil, nil, fmt.Errorf("agent produced no file result")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	msg := "attempt " + attemptID + " run (" + ex.Adapter + ") execution " + ex.ID
	res, err := a.Refs.Update(repo, branch, expectedHead, []refs.Change{{Path: file, Content: newContent}}, msg, provenance)
	if err != nil {
		// Recover an ambiguous publication only for this exact execution.
		// A previous execution on the same Attempt is not evidence of success.
		if head, e2 := a.repoHead(repo, branch); e2 == nil && a.commitHasMessage(repo, branch, head, msg) {
			return ex, &refs.Result{Status: "ok", NewSHA: head}, nil
		}
		return nil, nil, err
	}
	return ex, res, nil
}

func (a *App) repoHead(repo, branch string) (string, error) {
	refsMap, err := a.repoRefs(repo)
	if err != nil {
		return "", err
	}
	return refsMap["refs/heads/"+branch], nil
}

func (a *App) commitHasMessage(repo, branch, head, want string) bool {
	if head == "" {
		return false
	}
	logs, err := a.Artifacts.Log(repo, branch, 5)
	if err != nil {
		return false
	}
	for _, c := range logs {
		if c.Hash == head {
			return strings.TrimSpace(c.Message) == want
		}
	}
	return false
}

func randHex(n int) string {
	if n < 1 {
		panic("invalid random ID length")
	}
	bytes := make([]byte, (n+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		panic("secure random ID unavailable")
	}
	return hex.EncodeToString(bytes)[:n]
}
