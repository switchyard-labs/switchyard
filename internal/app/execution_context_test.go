package app

import (
	"context"
	"switchyard/internal/agent"
	"testing"
)

type executionContextRunner struct{ change func() }

func (r executionContextRunner) Run(ctx context.Context, ex *agent.Execution, task agent.Task) error {
	r.change()
	ex.Status = "succeeded"
	return nil
}

func TestExecutionContextSurvivesLaterAttemptReassociation(t *testing.T) {
	a, _, _ := actionFixture(t)
	for collection, values := range map[string][]map[string]any{
		"attempts": {{"id": "attempt1", "work_id": "original-work"}},
		"prs":      {{"id": "pr1", "attempt_id": "attempt1"}, {"id": "pr2", "attempt_id": "attempt1"}},
	} {
		for _, record := range values {
			if _, _, err := a.Trestle.CreateRecord(collection, record, collection+strOf(record["id"])); err != nil {
				t.Fatal(err)
			}
		}
	}
	a.Runner = executionContextRunner{change: func() {
		id, version, _, err := a.Trestle.FindRecord("attempts", filterEq("id", "attempt1"))
		if err != nil {
			t.Fatal(err)
		}
		if err = a.Trestle.PatchRecord("attempts", id, version, map[string]any{"work_id": "later-work"}); err != nil {
			t.Fatal(err)
		}
	}}
	execution, err := a.runViaSubstrateContext(context.Background(), "reviewer", "attempt1", "repo", "branch", "file.go", "original", "review")
	if err != nil {
		t.Fatal(err)
	}
	_, _, record, err := a.Trestle.FindRecord("execution_metadata", filterEq("execution_id", execution.ID))
	if err != nil {
		t.Fatal(err)
	}
	metadata := record["metadata"].(map[string]any)
	if metadata["attempt_id"] != "attempt1" || metadata["work_id"] != "original-work" {
		t.Fatal("execution context followed later mutable association")
	}
	ids := metadata["pr_ids"].([]any)
	if len(ids) != 2 {
		t.Fatal("related PR context missing")
	}
}
