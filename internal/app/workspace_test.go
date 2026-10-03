package app

import "testing"

func TestWorkspaceMovePlanRejectsAliasesCollisionsAndUnsupportedEntries(t *testing.T) {
	entries := []workspaceEntry{{"src/main.go", "100755", "blob"}, {"docs/guide.md", "100644", "blob"}, {"linked", "120000", "blob"}}
	for _, test := range []struct{ source, target, operation string }{{"../src", "new", "move"}, {"src", "src/new", "move"}, {"src", "docs", "move"}, {"src", "docs/guide.md/child", "move"}, {"linked", "moved", "move"}, {"missing", "new", "delete"}, {"src", "new", "copy"}} {
		if _, err := workspaceMovePlan(entries, test.source, test.target, test.operation); err == nil {
			t.Fatalf("unsafe operation accepted: %+v", test)
		}
	}
	plan, err := workspaceMovePlan(entries, "src", "lib", "move")
	if err != nil || len(plan) != 1 || plan[0].Path != "src/main.go" || !plan[0].Delete {
		t.Fatalf("valid directory move rejected: %+v %v", plan, err)
	}
}
