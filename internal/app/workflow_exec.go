package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dop251/goja"
)

const (
	maxWfDepth    = 4
	maxWfRetries  = 3
	maxWfParallel = 16
)

// validateWorkflowScript ensures the JS compiles and exposes a callable `run`.
func validateWorkflowScript(script string) error {
	vm := goja.New()
	if _, err := vm.RunString(script); err != nil {
		return err
	}
	f, ok := goja.AssertFunction(vm.Get("run"))
	if !ok {
		return fmt.Errorf("script must define function run(ctx)")
	}
	_ = f
	return nil
}

// wfExec drives a single workflow run. Its state is per-execution (the script
// is replayed from the top on every pump); durability lives in wf_steps.
type wfExec struct {
	a          *App
	runID      string
	script     string
	params     map[string]any
	actor      string
	budget     int
	depth      int
	counter    int    // deterministic step-key counter, reset per execution
	baseN      int    // durable step count at execution start
	execN      int    // newly executed steps this pass
	currentKey string // step key of the step currently executing
	park       string // set when the run parks (approval/child)
	status     string // run status observed at start
}

// executeRun runs (or replays) a workflow run. Called by the runner pump.
func (a *App) executeRun(runID string) error {
	run := a.runByID(runID)
	if run == nil {
		return fmt.Errorf("run not found")
	}
	status, _ := run["status"].(string)
	if status != "pending" && status != "running" && status != "cancelling" {
		return nil
	}
	script, _ := run["script"].(string)
	params, _ := run["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}
	budget := 25
	if bs, ok := run["budget_steps"].(string); ok && bs != "" {
		fmt.Sscanf(bs, "%d", &budget)
	}
	depth := 0
	if ds, ok := run["depth"].(string); ok && ds != "" {
		fmt.Sscanf(ds, "%d", &depth)
	}
	baseN := 0
	if sc, ok := run["step_count"].(string); ok && sc != "" {
		fmt.Sscanf(sc, "%d", &baseN)
	}
	if d, _ := a.policyGateWorkflow(strOr(params["repo"]), budget); d == "deny" {
		a.patchRun(runID, map[string]any{"status": "failed", "error": "policy denied workflow", "updated_at": nowStr()})
		return nil
	}
	e := &wfExec{
		a: a, runID: runID, script: script, params: params, actor: strOr(params["_actor"]),
		budget: budget, depth: depth, baseN: baseN, status: status,
	}
	// mark running (unless we are a fresh pending run; pending -> running)
	a.patchRun(runID, map[string]any{"status": "running", "updated_at": nowStr()})

	out, err := e.run()
	if err != nil {
		if e.park != "" {
			// park: set the appropriate waiting status and stop this pass.
			switch e.park {
			case "approval":
				a.patchRun(runID, map[string]any{"status": "waiting_approval", "step_count": itoa(e.baseN + e.execN), "updated_at": nowStr()})
			default:
				a.patchRun(runID, map[string]any{"status": "waiting_child", "step_count": itoa(e.baseN + e.execN), "updated_at": nowStr()})
			}
			return nil
		}
		msg := err.Error()
		if strings.Contains(msg, "cancelled") {
			a.patchRun(runID, map[string]any{"status": "cancelled", "error": "cancelled", "step_count": itoa(e.baseN + e.execN), "updated_at": nowStr()})
			return nil
		}
		if strings.Contains(msg, "budget exceeded") {
			a.patchRun(runID, map[string]any{"status": "failed", "error": "budget exceeded", "step_count": itoa(e.baseN + e.execN), "updated_at": nowStr()})
			return nil
		}
		a.patchRun(runID, map[string]any{"status": "failed", "error": msg, "step_count": itoa(e.baseN + e.execN), "updated_at": nowStr()})
		log.Printf("workflow run %s failed: %v", runID, err)
		return nil
	}
	a.patchRun(runID, map[string]any{"status": "completed", "output": out, "step_count": itoa(e.baseN + e.execN), "error": "", "updated_at": nowStr()})
	log.Printf("workflow run %s completed (%d executed steps, %d total)", runID, e.execN, e.baseN+e.execN)
	return nil
}

// run executes the workflow script. It returns the JS `run` return value and
// an error describing the control flow outcome (park/cancel/fail).
func (e *wfExec) run() (map[string]any, error) {
	vm := goja.New()
	ctx := vm.NewObject()
	ctx.Set("agent", e.opAgent)
	ctx.Set("check", e.opCheck)
	ctx.Set("review", e.opReview)
	ctx.Set("integrate", e.opIntegrate)
	ctx.Set("wait_approval", e.opWaitApproval)
	ctx.Set("spawn", e.opSpawn)
	ctx.Set("sleep", e.opSleep)
	ctx.Set("note", e.opNote)
	ctx.Set("parallel", func(call goja.FunctionCall) goja.Value { return e.opParallel(vm, call) })
	ctx.Set("params", vm.ToValue(e.params))
	// deterministic workflows: no wall-clock/entropy in control flow
	if m, ok := vm.Get("Math").(*goja.Object); ok {
		_ = m.Set("random", func() float64 { return 0.5 })
	}
	if d, ok := vm.Get("Date").(*goja.Object); ok {
		_ = d.Set("now", func() int64 { return 0 })
	}
	ctx.Set("random", func(seed int) int {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", e.runID, seed)))
		return int(h[0])
	})
	vm.Set("ctx", ctx)

	if _, err := vm.RunString(e.script); err != nil {
		return nil, err
	}
	fn, ok := goja.AssertFunction(vm.Get("run"))
	if !ok {
		return nil, fmt.Errorf("script must define function run(ctx)")
	}
	res, err := fn(goja.Undefined(), ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if b, e2 := json.Marshal(res.Export()); e2 == nil && string(b) != "null" {
		_ = json.Unmarshal(b, &out)
	}
	return out, nil
}

// checkCancel returns true if the run is being cancelled.
func (e *wfExec) checkCancel() error {
	run := e.a.runByID(e.runID)
	if run == nil {
		return fmt.Errorf("cancelled")
	}
	st, _ := run["status"].(string)
	if st == "cancelling" || st == "cancelled" {
		return fmt.Errorf("cancelled")
	}
	return nil
}

// step is the durable-step framework. On replay (same step_key + args_hash)
// the recorded result is returned instead of re-executing. Re-execution is
// bounded by maxWfRetries per step.
func (e *wfExec) step(op string, args map[string]any, exec func() (any, error)) (any, error) {
	repo, _ := e.defaultRepoBranch(args)
	if !e.a.repoAccess(repo, e.actor, RunAgent) {
		return nil, fmt.Errorf("repository_access_denied")
	}

	if err := e.checkCancel(); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("s%d", e.counter)
	e.counter++
	e.currentKey = key
	ab, _ := json.Marshal(args)
	hash := sha256Hex(ab)
	stepID := e.runID + "|" + key

	existing := e.findStep(stepID)
	if existing != nil {
		st, _ := existing["status"].(string)
		ah, _ := existing["args_hash"].(string)
		if st == "completed" && ah == hash {
			return existing["result"], nil // replay
		}
		if st == "completed" {
			return nil, fmt.Errorf("non-deterministic replay at %s (args changed)", key)
		}
		attempts := 0
		fmt.Sscanf(stepStr(existing["attempts"]), "%d", &attempts)
		if st == "failed" && attempts >= maxWfRetries {
			return nil, fmt.Errorf("step %s failed after %d attempts: %v", key, attempts, existing["result"])
		}
		// crash recovery / retry: proceed to re-execute (attempts incremented below)
	}
	isNew := existing == nil
	attempts := 0
	if existing != nil {
		fmt.Sscanf(stepStr(existing["attempts"]), "%d", &attempts)
	}
	attempts++

	// durable intent: running
	if existing == nil {
		if _, _, cerr := e.a.Trestle.CreateRecord("wf_steps", map[string]any{
			"step_id": stepID, "run_id": e.runID, "step_key": key, "op": op, "args_hash": hash,
			"status": "running", "result": nil, "attempts": itoa(attempts),
			"executed_at": nowStr(), "updated_at": nowStr(),
		}, "wfstep-"+stepID); cerr != nil {
			log.Printf("workflow step create %s/%s: %v", e.runID, key, cerr)
			return nil, fmt.Errorf("durable step record failed: %v", cerr)
		}
	} else {
		e.updateStep(stepID, map[string]any{"status": "running", "attempts": itoa(attempts), "updated_at": nowStr()})
	}

	var result any
	var err error
	for attempt := attempts; attempt <= maxWfRetries; attempt++ {
		result, err = exec()
		if err == nil {
			break
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond) // backoff
	}
	if err != nil {
		e.updateStep(stepID, map[string]any{"status": "failed", "result": err.Error(), "attempts": itoa(attempts), "updated_at": nowStr()})
		return nil, fmt.Errorf("step %s (%s) failed: %v", key, op, err)
	}
	e.updateStep(stepID, map[string]any{"status": "completed", "result": result, "attempts": itoa(attempts), "updated_at": nowStr()})
	if isNew {
		e.execN++
		if e.baseN+e.execN > e.budget {
			return nil, fmt.Errorf("budget exceeded (%d steps)", e.budget)
		}
	}
	return result, nil
}

func (e *wfExec) findStep(stepID string) map[string]any {
	items, err := e.a.Trestle.ListRecords("wf_steps", `run_id = "`+e.runID+`"`)
	if err != nil {
		return nil
	}
	for _, it := range items {
		if it["step_id"] == stepID {
			return it
		}
	}
	return nil
}

func (e *wfExec) updateStep(stepID string, values map[string]any) {
	if id, ver, _, err := e.a.Trestle.FindRecord("wf_steps", `step_id = "`+stepID+`"`); err == nil && id != "" {
		_ = e.a.Trestle.PatchRecord("wf_steps", id, ver, values)
	}
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:16])
}

func stepStr(v any) string {
	s, _ := v.(string)
	return s
}

// ---- host ops ----

func strOr(v any) string {
	s, _ := v.(string)
	return s
}

func (e *wfExec) defaultRepoBranch(args map[string]any) (repo, branch string) {
	repo, _ = args["repo"].(string)
	branch, _ = args["branch"].(string)
	if repo == "" {
		repo, _ = e.params["repo"].(string)
	}
	if branch == "" {
		branch, _ = e.params["branch"].(string)
	}
	return repo, branch
}

func (e *wfExec) opAgent(args map[string]any) (map[string]any, error) {
	res, err := e.step("agent", args, func() (any, error) {
		repo, branch := e.defaultRepoBranch(args)
		file, _ := args["file"].(string)
		appendLine, _ := args["append"].(string)
		if repo == "" || branch == "" || file == "" {
			return nil, fmt.Errorf("agent: repo, branch, file required")
		}
		if err := e.a.policyGateAgent("implementer", repo, file); err != nil {
			return nil, err
		}
		current := ""
		if raw, err := e.a.Artifacts.RawFile(repo, branch, file); err == nil {
			current = string(raw)
		}
		head, err := e.a.repoHead(repo, branch)
		if err != nil {
			return nil, err
		}
		ex, res, err := e.a.runAgentStep("implementer", "wf:"+e.runID, repo, branch, file, current, appendLine, head, "workflow:"+e.runID+":"+e.currentKey)
		if err != nil {
			return nil, err
		}
		return map[string]any{"execution": ex.ID, "new_sha": res.NewSHA, "status": res.Status, "output": ex.Output}, nil
	})
	if err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opCheck(args map[string]any) (map[string]any, error) {
	res, err := e.step("check", args, func() (any, error) {
		repo, branch := e.defaultRepoBranch(args)
		data, err := e.a.Artifacts.RawFile(repo, branch, "ATTEMPT.md")
		if err != nil {
			return map[string]any{"status": "fail", "reason": "marker file missing"}, nil
		}
		lines := len(strings.Split(strings.TrimSpace(string(data)), "\n"))
		if lines < 2 {
			return map[string]any{"status": "fail", "lines": lines, "detail": "need >= 2 lines"}, nil
		}
		return map[string]any{"status": "pass", "lines": lines, "detail": "marker file valid"}, nil
	})
	if err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opReview(args map[string]any) (map[string]any, error) {
	res, err := e.step("review", args, func() (any, error) {
		repo, branch := e.defaultRepoBranch(args)
		data, err := e.a.Artifacts.RawFile(repo, branch, "ATTEMPT.md")
		if err != nil {
			return map[string]any{"approved": false, "findings": []map[string]any{{"severity": "error", "message": "no ATTEMPT.md"}}}, nil
		}
		lines := len(strings.Split(strings.TrimSpace(string(data)), "\n"))
		findings := []map[string]any{}
		approved := true
		if lines < 2 {
			findings = append(findings, map[string]any{"severity": "warning", "message": "ATTEMPT.md too short"})
			approved = false
		}
		return map[string]any{"approved": approved, "findings": findings, "lines": lines}, nil
	})
	if err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opIntegrate(args map[string]any) (map[string]any, error) {
	res, err := e.step("integrate", args, func() (any, error) {
		repo, branch := e.defaultRepoBranch(args)
		base, _ := args["base"].(string)
		if base == "" {
			base, _ = e.params["base"].(string)
		}
		if repo == "" || branch == "" || base == "" {
			return nil, fmt.Errorf("integrate: repo, branch, base required")
		}
		// deterministic PR id so a crash between PR creation and step
		// completion re-creates the SAME PR (idempotent).
		prID := "pr_" + sha256Hex([]byte(e.runID + "|" + e.currentKey))[:10]
		_, replayed, err := e.a.Trestle.CreateRecord("prs", map[string]any{
			"id": prID, "attempt_id": "wf:" + e.runID, "work_id": "wf:" + e.runID,
			"repo": repo, "branch": branch, "base": base, "title": "workflow integrate " + e.runID,
			"status": "open", "check_status": "pending", "created_at": nowStr(), "integrated_at": "",
		}, "pr-"+prID)
		if err != nil {
			return nil, err
		}
		if replayed {
			// PR already exists from a prior attempt: if already integrated,
			// return that result.
			if items, _ := e.a.Trestle.ListRecords("prs", `id = "`+prID+`"`); len(items) > 0 && items[0]["status"] == "integrated" {
				return map[string]any{"pr": prID, "status": "integrated", "new_base_sha": items[0]["integrated_at"]}, nil
			}
		}
		_, sourceSHA, err := e.a.Refs.Snapshot(repo, base, branch)
		if err != nil {
			return nil, err
		}
		data, err := e.a.Artifacts.RawFile(repo, sourceSHA, "ATTEMPT.md")
		ok := err == nil && len(strings.Split(strings.TrimSpace(string(data)), "\n")) >= 2
		status := "fail"
		if ok {
			status = "pass"
		}
		if err := e.a.setCommitCheck(prID, repo, sourceSHA, status, "workflow deterministic check"); err != nil {
			return nil, err
		}
		if !ok {
			return map[string]any{"pr": prID, "status": "check_failed"}, nil
		}
		qid, err := e.a.enqueuePR(map[string]any{"id": prID, "repo": repo, "branch": branch, "base": base})
		if err != nil {
			return nil, err
		}
		return map[string]any{"pr": prID, "status": "queued", "queue_id": qid}, nil

	})
	if err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opWaitApproval(args map[string]any) (map[string]any, error) {
	// durable step: parks this pass on first execution; replays as a no-op
	// (continuing past the gate) once the run has been approved.
	res, err := e.step("wait_approval", args, func() (any, error) {
		e.park = "approval"
		return map[string]any{"approved": false, "reason": args["reason"], "run_id": e.runID}, nil
	})
	if err != nil {
		return nil, err
	}
	if e.park == "approval" {
		return nil, fmt.Errorf("park:approval")
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opSpawn(args map[string]any) (map[string]any, error) {
	wfName, _ := args["workflow"].(string)
	if wfName == "" {
		return nil, fmt.Errorf("spawn: workflow required")
	}
	res, err := e.step("spawn", args, func() (any, error) {
		if e.depth+1 > maxWfDepth {
			return nil, fmt.Errorf("spawn: max depth %d exceeded", maxWfDepth)
		}
		childParams, _ := args["params"].(map[string]any)
		if childParams == nil {
			childParams = map[string]any{}
		}
		childParams["_actor"] = e.actor
		if !e.a.repoAccess(strOr(childParams["repo"]), e.actor, RunAgent) {
			return nil, fmt.Errorf("repository_access_denied")
		}
		items, err := e.a.Trestle.ListRecords("workflows", "")
		if err != nil {
			return nil, err
		}
		var def map[string]any
		for _, it := range items {
			if it["name"] == wfName && e.a.workflowOwner(strOr(it["id"])) == e.actor {
				def = it
				break
			}
		}
		if def == nil {
			return nil, fmt.Errorf("spawn: workflow %q not found", wfName)
		}
		// deterministic child id from (parent, step) so a crash between child
		// creation and step-completion re-creates the SAME child (idempotent).
		childID := "wfr_" + sha256Hex([]byte(e.runID + "|" + e.currentKey))[:10]
		script, _ := def["script"].(string)
		_, replayed, err := e.a.Trestle.CreateRecord("workflow_runs", map[string]any{
			"id": childID, "workflow_id": def["id"], "params": childParams, "script": script,
			"status": "pending", "step_count": "0", "budget_steps": "25",
			"output": nil, "error": "", "parent_run_id": e.runID, "depth": itoa(e.depth + 1),
			"created_at": nowStr(), "updated_at": nowStr(),
		}, "wfrun-"+childID)
		if err != nil {
			return nil, err
		}
		_ = replayed
		e.park = "child"
		return map[string]any{"child_run_id": childID, "parent_run_id": e.runID, "spawned_fresh": !replayed}, nil
	})
	if err != nil {
		return nil, err
	}
	if e.park == "child" {
		return nil, fmt.Errorf("park:child:%s", e.currentKey)
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opSleep(args map[string]any) (map[string]any, error) {
	res, err := e.step("sleep", args, func() (any, error) {
		ms := 0
		if msf, ok := args["ms"].(int64); ok {
			ms = int(msf)
		} else if msf, ok := args["ms"].(int); ok {
			ms = msf
		}
		if ms > 60000 {
			ms = 60000
		}
		if ms < 0 {
			ms = 0
		}
		// cancellable sleep: check the run flag in small chunks so
		// cancellation and executor death have a bounded reaction window.
		for ms > 0 {
			chunk := 500
			if ms < chunk {
				chunk = ms
			}
			time.Sleep(time.Duration(chunk) * time.Millisecond)
			ms -= chunk
			if err := e.checkCancel(); err != nil {
				return nil, err
			}
		}
		return map[string]any{"slept_ms": ms}, nil
	})
	if err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

func (e *wfExec) opNote(args map[string]any) (map[string]any, error) {
	res, err := e.step("note", args, func() (any, error) {
		return map[string]any{"note": args["text"]}, nil
	})
	if err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

// opParallel is a control-flow wrapper (not itself a durable step): it invokes
// each branch function, whose Switchyard operations are independently durable
// with their own step keys. Branches run sequentially on the single-node
// executor but are independently durable/resumable (each branch's steps replay
// individually), so a distributed executor could run them concurrently.
func (e *wfExec) opParallel(vm *goja.Runtime, call goja.FunctionCall) goja.Value {
	if len(call.Arguments) != 1 {
		panic(vm.ToValue("parallel: expected one array of branch functions"))
	}
	arr, ok := call.Arguments[0].(*goja.Object)
	if !ok {
		panic(vm.ToValue("parallel: expected an array of branch functions"))
	}
	n := int(arr.Get("length").ToInteger())
	if n == 0 || n > maxWfParallel {
		panic(vm.ToValue(fmt.Sprintf("parallel: need 1..%d branches", maxWfParallel)))
	}
	results := make([]any, 0, n)
	for i := 0; i < n; i++ {
		fn, ok := goja.AssertFunction(arr.Get(itoa(i)))
		if !ok {
			panic(vm.ToValue("parallel: branch must be a function"))
		}
		v, err := fn(goja.Undefined(), call.This)
		if err != nil {
			panic(err)
		}
		results = append(results, v.Export())
	}
	return vm.ToValue(results)
}
