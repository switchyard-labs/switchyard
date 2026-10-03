package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"switchyard/internal/trestle"
)

// Durable workflows (CP7).
//
// Authoring model: JavaScript. A workflow is a JS function `run(ctx)` where
// `ctx.agent/check/review/integrate/wait_approval/spawn/...` are Switchyard
// operations. The durable workflow state is NOT the lifetime of a JS process:
// each Switchyard operation is a durable step recorded in `wf_steps`, keyed by
// (run_id, step_key). On executor death / restart the script is re-run from the
// top (Temporal-style replay); completed steps replay their recorded result
// instead of re-executing, so external effects are never duplicated.
//
// Constrained surface (no closures/stack serialization): the JS may do ordinary
// local computation, but only Switchyard operations are durable boundaries.
// Determinism constraint: workflow scripts must be deterministic (no wall-clock
// in control flow; use ctx.random(seed) for seeded randomness).

type Workflow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Script    string `json:"script"`
	CreatedAt string `json:"created_at"`
}

// ---- collections ----

func workflowCollections() [][2]any {
	return [][2]any{
		{"workflow_git_effects", []trestle.CollectionField{{Name: "effect_id", Type: "text", Unique: true}, {Name: "state", Type: "json"}}},
		{"workflow_claims", []trestle.CollectionField{{Name: "queue_id", Type: "text", Unique: true}, {Name: "state", Type: "json"}}},
		{"workflow_owners", []trestle.CollectionField{{Name: "workflow_id", Type: "text", Unique: true}, {Name: "username", Type: "text"}}},
		{"workflows", []trestle.CollectionField{
			{Name: "id", Type: "text", Unique: true}, {Name: "name", Type: "text"},
			{Name: "script", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"workflow_runs", []trestle.CollectionField{
			{Name: "id", Type: "text", Unique: true}, {Name: "workflow_id", Type: "text"},
			{Name: "params", Type: "json"}, {Name: "script", Type: "text"},
			{Name: "status", Type: "text"}, {Name: "step_count", Type: "text"},
			{Name: "budget_steps", Type: "text"}, {Name: "output", Type: "json"},
			{Name: "error", Type: "text"}, {Name: "parent_run_id", Type: "text"},
			{Name: "depth", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"wf_steps", []trestle.CollectionField{
			{Name: "step_id", Type: "text", Unique: true}, {Name: "run_id", Type: "text"}, {Name: "step_key", Type: "text"},
			{Name: "op", Type: "text"}, {Name: "args_hash", Type: "text"},
			{Name: "status", Type: "text"}, {Name: "result", Type: "json"},
			{Name: "attempts", Type: "text"}, {Name: "executed_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
	}
}

// ---- API ----

func (a *App) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Name   string `json:"name"`
		Script string `json:"script"`
	}
	if err := readJSON(r, &in); err != nil || in.Name == "" || in.Script == "" {
		writeJSON(w, 400, map[string]any{"error": "name_and_script_required"})
		return
	}
	if err := validateWorkflowScript(in.Script); err != nil {
		writeJSON(w, 400, map[string]any{"error": "script_invalid: " + err.Error()})
		return
	}
	id := "wf_" + randHex(10)
	_, _, err := a.Trestle.CreateRecord("workflows", map[string]any{
		"id": id, "name": in.Name, "script": in.Script, "created_at": time.Now().UTC().Format(time.RFC3339),
	}, "workflow-"+id)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if _, _, err := a.Trestle.CreateRecord("workflow_owners", map[string]any{"workflow_id": id, "username": user}, "workflow-owner-"+id); err != nil {
		writeJSON(w, 502, map[string]any{"error": "workflow_owner_persistence_failed"})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name})
}

func (a *App) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("workflows", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, it := range items {
		if a.workflowOwner(strOr(it["id"])) == a.currentUser(r) {
			out = append(out, it)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) handleStartRun(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	wfID := r.PathValue("id")
	items, err := a.Trestle.ListRecords("workflows", `id = "`+wfID+`"`)
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "workflow_not_found"})
		return
	}
	var in struct {
		Params      map[string]any `json:"params"`
		BudgetSteps int            `json:"budget_steps"`
	}
	_ = readJSON(r, &in)
	if in.BudgetSteps == 0 {
		in.BudgetSteps = 25
	}
	if in.Params == nil {
		in.Params = map[string]any{}
	}
	in.Params["_actor"] = user
	if !a.repoAccess(strOr(in.Params["repo"]), user, RunAgent) {
		writeJSON(w, 403, map[string]any{"error": "repository_access_denied"})
		return
	}
	runID := "wfr_" + randHex(10)
	now := time.Now().UTC().Format(time.RFC3339)
	script, _ := items[0]["script"].(string)
	_, _, err = a.Trestle.CreateRecord("workflow_runs", map[string]any{
		"id": runID, "workflow_id": wfID, "params": in.Params, "script": script,
		"status": "pending", "step_count": "0", "budget_steps": itoa(in.BudgetSteps),
		"output": nil, "error": "", "parent_run_id": "", "depth": "0",
		"created_at": now, "updated_at": now,
	}, "wfrun-"+runID)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	log.Printf("workflow run %s started (%s) by %s", runID, wfID, user)
	writeJSON(w, 201, map[string]any{"id": runID, "workflow_id": wfID, "status": "pending"})
}

func (a *App) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("workflow_runs", "")
	items = a.visibleRecords("workflow_runs", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) handleGetRun(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	run := a.runByID(r.PathValue("id"))
	if run == nil {
		writeJSON(w, 404, map[string]any{"error": "run_not_found"})
		return
	}
	steps, _ := a.Trestle.ListRecords("wf_steps", `run_id = "`+r.PathValue("id")+`"`)
	writeJSON(w, 200, map[string]any{"run": run, "steps": steps})
}

func (a *App) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	runID := r.PathValue("id")
	run := a.runByID(runID)
	if run == nil {
		writeJSON(w, 404, map[string]any{"error": "run_not_found"})
		return
	}
	status, _ := run["status"].(string)
	if status == "completed" || status == "cancelled" {
		writeJSON(w, 409, map[string]any{"error": "run_terminal"})
		return
	}
	a.patchRun(runID, map[string]any{"status": "cancelling", "updated_at": nowStr()})
	log.Printf("workflow run %s cancellation requested by %s", runID, user)
	writeJSON(w, 200, map[string]any{"id": runID, "status": "cancelling"})
}

func (a *App) handleApproveRun(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	runID := r.PathValue("id")
	run := a.runByID(runID)
	if run == nil {
		writeJSON(w, 404, map[string]any{"error": "run_not_found"})
		return
	}
	status, _ := run["status"].(string)
	if status != "waiting_approval" {
		writeJSON(w, 409, map[string]any{"error": "run_not_waiting_approval"})
		return
	}
	steps, err := a.Trestle.ListRecords("wf_steps", filterEq("run_id", runID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	approved := false
	for _, step := range steps {
		if step["op"] == "wait_approval" && step["status"] == "waiting" {
			e := &wfExec{a: a}
			if err := e.updateStep(strOf(step["step_id"]), map[string]any{"status": "completed", "result": map[string]any{"approved": true, "approved_by": user}, "updated_at": nowStr()}); err != nil {
				writeJSON(w, 502, map[string]any{"error": err.Error()})
				return
			}
			approved = true
			break
		}
	}
	if !approved {
		writeJSON(w, 409, map[string]any{"error": "approval_effect_not_waiting"})
		return
	}
	if err := a.patchRun(runID, map[string]any{"status": "running", "updated_at": nowStr()}); err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	log.Printf("workflow run %s approved by %s", runID, user)
	writeJSON(w, 200, map[string]any{"id": runID, "status": "running", "approved_by": user})
}

func (a *App) handleRetryRun(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	runID := r.PathValue("id")
	run := a.runByID(runID)
	if run == nil {
		writeJSON(w, 404, map[string]any{"error": "run_not_found"})
		return
	}
	status, _ := run["status"].(string)
	if status != "failed" && status != "cancelled" {
		writeJSON(w, 409, map[string]any{"error": "run_not_retryable"})
		return
	}
	// completed durable steps replay; failed/incomplete steps re-execute
	a.patchRun(runID, map[string]any{"status": "pending", "error": "", "updated_at": nowStr()})
	log.Printf("workflow run %s retry requested by %s", runID, user)
	writeJSON(w, 200, map[string]any{"id": runID, "status": "pending"})
}

func (a *App) runByID(id string) map[string]any {
	items, err := a.Trestle.ListRecords("workflow_runs", `id = "`+id+`"`)
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (a *App) patchRun(id string, values map[string]any) error {
	rid, ver, _, err := a.Trestle.FindRecord("workflow_runs", filterEq("id", id))
	if err != nil {
		return err
	}
	if rid == "" {
		return fmt.Errorf("workflow run missing")
	}
	return a.Trestle.PatchRecord("workflow_runs", rid, ver, values)
}

// ---- runner loop ----

// StartWorkflowRunner executes pending/running workflows. It is the durable
// execution queue: on restart it resumes incomplete runs by replaying the
// workflow script from the top (completed durable steps replay; nothing
// external is re-executed). Runs execute concurrently; each run is guarded by
// an in-flight set so replay never races itself. Budgets, cancellation,
// spawn/child promotion and waiting-for-approval parking are handled in the
// executor.
func (a *App) StartWorkflowRunner(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	log.Printf("workflow runner started (interval %s)", interval)
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				a.promoteWaitingParents()
				a.pumpRuns()
			}
		}
	}()
}

func (a *App) pumpRuns() {
	items, err := a.Trestle.ListRecords("workflow_runs", "")
	if err != nil {
		return
	}
	for _, it := range items {
		status, _ := it["status"].(string)
		if status != "pending" && status != "running" && status != "cancelling" {
			continue
		}
		id, _ := it["id"].(string)
		if status == "cancelling" {
			a.patchRun(id, map[string]any{"status": "cancelled", "updated_at": nowStr()})
			continue
		}
		a.wfMu.Lock()
		if a.wfInFlight[id] {
			a.wfMu.Unlock()
			continue
		}
		a.wfInFlight[id] = true
		a.wfMu.Unlock()
		go func(runID string) {
			defer func() {
				a.wfMu.Lock()
				delete(a.wfInFlight, runID)
				a.wfMu.Unlock()
			}()
			if err := a.executeRun(runID); err != nil {
				log.Printf("workflow run %s: %v", runID, err)
			}
		}(id)
	}
}

// promoteWaitingParents resumes workflow runs that were waiting on a child
// workflow once the child has reached a terminal state.
func (a *App) promoteWaitingParents() {
	items, err := a.Trestle.ListRecords("workflow_runs", "")
	if err != nil {
		return
	}
	for _, run := range items {
		status := strOf(run["status"])
		if status != "waiting_child" && status != "waiting_approval" {
			continue
		}
		id := strOf(run["id"])
		steps, err := a.Trestle.ListRecords("wf_steps", filterEq("run_id", id))
		if err != nil {
			continue
		}
		for _, step := range steps {
			result, _ := step["result"].(map[string]any)
			resume := status == "waiting_approval" && step["op"] == "wait_approval" && step["status"] == "completed" && result["approved"] == true
			if status == "waiting_child" && step["op"] == "spawn" && step["status"] == "waiting" {
				child := a.runByID(strOf(result["child_run_id"]))
				if child == nil {
					continue
				}
				switch child["status"] {
				case "completed", "failed", "cancelled":
					resume = true
				}
			}
			if status == "waiting_child" && step["op"] == "integrate" && step["status"] == "waiting" {
				_, _, item, err := a.Trestle.FindRecord("iq", filterEq("id", strOf(result["queue_id"])))
				if err == nil && item != nil && (item["status"] == "done" || item["status"] == "failed") {
					resume = true
				}
			}

			if resume {
				if err := a.patchRun(id, map[string]any{"status": "running", "updated_at": nowStr()}); err != nil {
					log.Printf("resume workflow %s: %v", id, err)
				}
				break
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func nowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}
