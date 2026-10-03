package app

import "testing"

func TestAttentionProjectionOnlyRecordedStops(t *testing.T) {
	for _, status := range []string{"running", "completed", "cancelled"} {
		if got := attentionProjection("workflow_runs", map[string]any{"status": status}); got != nil {
			t.Fatal("invented decision", got)
		}
	}
	got := attentionProjection("workflow_runs", map[string]any{"id": "run", "status": "waiting_approval", "params": map[string]any{"repo": "repo", "branch": "main"}})
	if got == nil || got["repo"] != "repo" || got["kind"] != "workflow_approval" {
		t.Fatal(got)
	}
	failed := attentionProjection("action_runs", map[string]any{"id": "ci", "source_sha": "sha", "state": map[string]any{"status": "failed"}})
	if failed == nil || failed["kind"] != "actions_failed" || failed["status"] != "failed" {
		t.Fatal(failed)
	}
}
