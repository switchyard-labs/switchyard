package app

import (
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"switchyard/internal/refs"
	"switchyard/internal/trestle"
	"testing"
	"time"
)

func workflowFixture(t *testing.T) (*wfExec, *queueStore) {
	t.Helper()
	store := &queueStore{next: 10, records: map[string][]*queueRecord{
		"repository_meta": {{id: "repo", version: 1, values: map[string]any{"id": "repo", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "visibility": "private"}}},
		"workflow_runs":   {{id: "run", version: 1, values: map[string]any{"id": "run", "status": "running", "params": map[string]any{"repo": "repo", "_actor": "alice"}}}},
	}}
	server := httptest.NewServer(store)
	t.Cleanup(server.Close)
	a := &App{Trestle: trestle.New(server.URL, "fixture", "fixture")}
	return &wfExec{a: a, runID: "run", actor: "alice", params: map[string]any{"repo": "repo"}, budget: 10}, store
}

func TestWorkflowReplayIncludesOperationAndArguments(t *testing.T) {
	e, _ := workflowFixture(t)
	calls := 0
	execute := func() (any, error) { calls++; return map[string]any{"value": "recorded"}, nil }
	if _, err := e.step("note", map[string]any{"text": "one"}, execute); err != nil {
		t.Fatal(err)
	}
	e.counter = 0
	if _, err := e.step("note", map[string]any{"text": "one"}, execute); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("replayed effect")
	}
	for _, test := range []struct{ op, text string }{{"agent", "one"}, {"note", "two"}} {
		e.counter = 0
		if _, err := e.step(test.op, map[string]any{"text": test.text}, execute); err == nil {
			t.Fatal("accepted changed effect identity")
		}
	}
	if calls != 1 {
		t.Fatal("identity mismatch executed effect")
	}
}

func TestWorkflowBudgetAndRetryAccountingBeforeEffect(t *testing.T) {
	e, _ := workflowFixture(t)
	e.budget = 0
	calls := 0
	execute := func() (any, error) { calls++; return nil, errors.New("retry") }
	if _, err := e.step("note", nil, execute); err == nil || calls != 0 {
		t.Fatal("budget permitted effect")
	}
	e.budget = 10
	for range 4 {
		e.counter = 0
		e.step("note", nil, execute)
	}
	if calls != 3 {
		t.Fatalf("retry effects=%d", calls)
	}
}

func TestWorkflowCompletionFailureRemainsRecoverable(t *testing.T) {
	e, store := workflowFixture(t)
	store.failStepCompletion = true
	_, err := e.opNote(map[string]any{"text": "note"})
	var durable *workflowDurabilityError
	if !errors.As(err, &durable) {
		t.Fatalf("completion error lost: %v", err)
	}
	store.mu.Lock()
	store.failStepCompletion = false
	store.mu.Unlock()
	e.counter = 0
	if _, err := e.opNote(map[string]any{"text": "note"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowApprovalNeverReplaysUnapproved(t *testing.T) {
	e, _ := workflowFixture(t)
	args := map[string]any{"reason": "human decision"}
	if _, err := e.opWaitApproval(args); err == nil || e.park != "approval" {
		t.Fatal("approval did not park")
	}
	for range 4 {
		e.counter = 0
		e.park = ""
		if _, err := e.opWaitApproval(args); err == nil || e.park != "approval" {
			t.Fatal("unapproved replay passed gate")
		}
	}
	if err := e.updateStep("run|s0", map[string]any{"status": "completed", "result": map[string]any{"approved": true}}); err != nil {
		t.Fatal(err)
	}
	e.counter = 0
	e.park = ""
	result, err := e.opWaitApproval(args)
	if err != nil || result["approved"] != true || e.park != "" {
		t.Fatalf("approved replay: %v %v", result, err)
	}
}

func TestWorkflowScriptBoundsAndDeterministicDate(t *testing.T) {
	started := time.Now()
	if err := validateWorkflowScript(`while(true){};function run(ctx){}`); err == nil {
		t.Fatal("infinite validation accepted")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("validation exceeded bound")
	}
	e, _ := workflowFixture(t)
	e.timeLimit = 20 * time.Millisecond
	e.script = `function run(ctx){while(true){}}`
	if _, err := e.run(); err == nil {
		t.Fatal("infinite workflow accepted")
	}
	e.script = `function run(ctx){return {date:new Date().valueOf(),random:Math.random()}}`
	result, err := e.run()
	if err != nil || result["date"] != float64(0) || result["random"] != 0.5 {
		t.Fatalf("nondeterministic runtime: %v %v", result, err)
	}
}

func TestWorkflowCrashWorker(t *testing.T) {
	if os.Getenv("SWITCHYARD_WORKFLOW_CRASH_CHILD") != "yes" {
		return
	}
	a := queueCrashApp(os.Getenv("SWITCHYARD_QUEUE_FIXTURE_URL"), os.Getenv("SWITCHYARD_QUEUE_FIXTURE_REMOTE"), os.Getenv("SWITCHYARD_QUEUE_FIXTURE_ROOT"))
	a.workflowCrashHook = func(phase string) {
		if phase == os.Getenv("SWITCHYARD_WORKFLOW_CRASH_PHASE") {
			os.Exit(92)
		}
	}
	e := &wfExec{a: a, runID: "run", actor: "alice", params: map[string]any{"repo": "repo", "branch": "attempt-workflow"}, budget: 10}
	var err error
	switch os.Getenv("SWITCHYARD_WORKFLOW_CRASH_MODE") {
	case "approval":
		_, err = e.opWaitApproval(map[string]any{"reason": "review"})
	case "git":
		_, err = e.opAgent(map[string]any{"file": "file.go", "append": "change"})
	default:
		_, err = e.opNote(map[string]any{"text": "note"})
	}
	t.Fatalf("crash phase not reached: %v", err)
}

func TestWorkflowRecoversAfterProcessDeath(t *testing.T) {
	for _, mode := range []string{"note", "approval", "git"} {
		phases := []string{"after_intent", "after_effect", "after_completion"}
		if mode == "git" {
			phases = []string{"before_git_publication", "after_git_publication"}
		}
		for _, phase := range phases {
			t.Run(mode+"/"+phase, func(t *testing.T) {
				root := t.TempDir()
				remote := filepath.Join(root, "remote.git")
				local := filepath.Join(root, "local")
				queueGit(t, root, "init", "--bare", remote)
				queueGit(t, root, "init", "-b", "main", local)
				if err := os.WriteFile(filepath.Join(local, "file.go"), []byte("original"), 0600); err != nil {
					t.Fatal(err)
				}
				queueGit(t, local, "add", ".")
				queueGit(t, local, "commit", "-m", "base")
				queueGit(t, local, "push", remote, "main")
				queueGit(t, local, "push", remote, "main:attempt-workflow")
				base := queueGit(t, local, "rev-parse", "HEAD")
				os.Mkdir(filepath.Join(root, "scratch"), 0700)
				os.WriteFile(filepath.Join(root, "token.sh"), []byte("#!/bin/sh\nprintf fixture\n"), 0700)
				store := &queueStore{next: 10, records: map[string][]*queueRecord{
					"repository_meta": {{id: "repo", version: 1, values: map[string]any{"id": "repo", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "visibility": "private"}}},
					"workflow_runs":   {{id: "run", version: 1, values: map[string]any{"id": "run", "status": "running", "params": map[string]any{"repo": "repo", "_actor": "alice"}}}},
				}}
				server := httptest.NewServer(store)
				defer server.Close()
				a := queueCrashApp(server.URL, remote, root)
				var plannedSHA string
				if mode == "git" {
					candidate, err := a.Refs.PrepareUpdate("repo", "attempt-workflow", base, []refs.Change{{Path: "file.go", Content: "changed"}}, "workflow effect run|s0")
					if err != nil {
						t.Fatal(err)
					}
					plannedSHA = candidate.CommitSHA
					store.mu.Lock()
					store.records["workflow_git_effects"] = []*queueRecord{{id: "plan", version: 1, values: map[string]any{"effect_id": "run|s0", "state": workflowGitEffect{Candidate: candidate, Dir: candidate.Dir, Result: map[string]any{"status": "ok", "new_sha": candidate.CommitSHA}}}}}
					store.mu.Unlock()
				}
				cmd := exec.Command(os.Args[0], "-test.run=^TestWorkflowCrashWorker$")
				cmd.Env = append(os.Environ(), "SWITCHYARD_WORKFLOW_CRASH_CHILD=yes", "SWITCHYARD_WORKFLOW_CRASH_MODE="+mode, "SWITCHYARD_WORKFLOW_CRASH_PHASE="+phase, "SWITCHYARD_QUEUE_FIXTURE_URL="+server.URL, "SWITCHYARD_QUEUE_FIXTURE_REMOTE="+remote, "SWITCHYARD_QUEUE_FIXTURE_ROOT="+root)
				output, err := cmd.CombinedOutput()
				exit, ok := err.(*exec.ExitError)
				if !ok || exit.ExitCode() != 92 {
					t.Fatalf("expected real process death: %v %s", err, output)
				}
				e := &wfExec{a: a, runID: "run", actor: "alice", params: map[string]any{"repo": "repo", "branch": "attempt-workflow"}, budget: 10}
				switch mode {
				case "approval":
					if _, err := e.opWaitApproval(map[string]any{"reason": "review"}); err == nil || e.park != "approval" {
						t.Fatal("process death bypassed approval")
					}
				case "git":
					result, err := e.opAgent(map[string]any{"file": "file.go", "append": "change"})
					if err != nil || result["new_sha"] != plannedSHA {
						t.Fatalf("Git recovery %v %v", result, err)
					}
					if head := queueGit(t, root, "--git-dir="+remote, "rev-parse", "attempt-workflow"); head != plannedSHA {
						t.Fatal("published different candidate")
					}
					if count := queueGit(t, root, "--git-dir="+remote, "rev-list", "--count", "attempt-workflow"); count != "2" {
						t.Fatal("duplicate workflow commit")
					}
				default:
					if _, err := e.opNote(map[string]any{"text": "note"}); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestWorkflowApprovalCompletionSurvivesRunPatchGap(t *testing.T) {
	e, store := workflowFixture(t)
	e.opWaitApproval(map[string]any{"reason": "review"})
	if err := e.updateStep("run|s0", map[string]any{"status": "completed", "result": map[string]any{"approved": true}}); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.records["workflow_runs"][0].values["status"] = "waiting_approval"
	store.mu.Unlock()
	e.a.promoteWaitingParents()
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.records["workflow_runs"][0].values["status"] != "running" {
		t.Fatal("durable approval stranded run")
	}
}
