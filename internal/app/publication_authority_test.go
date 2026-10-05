package app

import (
	"os"
	"strings"
	"testing"
)

func TestSharedPublicationAuthority(t *testing.T) {
	records := map[string][]map[string]any{
		"repository_meta":     {{"id": "meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "default_branch": "trunk"}},
		"repo_protected_refs": {{"repo_id": "meta", "pattern": "release/*", "require_queue": "true"}},
	}
	a := securityFixture(t, records)
	human := publicationAuthority{principal: "alice"}
	agent := publicationAuthority{principal: "alice", role: "implementer", kind: publicationAgent}
	for _, branch := range []string{"trunk", "main", "master", "release/stable"} {
		if err := a.authorizePublication(agent, "repo", branch); err == nil {
			t.Fatal("agent wrote canonical/protected", branch)
		}
	}
	if err := a.authorizePublication(agent, "repo", "attempt-one"); err != nil {
		t.Fatal(err)
	}
	if err := a.authorizePublication(human, "repo", "trunk"); err != nil {
		t.Fatal("allowed human write denied", err)
	}
	if err := a.authorizePublication(human, "repo", "release/stable"); err == nil {
		t.Fatal("protected human write allowed")
	}
	if err := a.authorizePublication(publicationAuthority{principal: "mallory"}, "repo", "attempt-one"); err == nil {
		t.Fatal("unrelated principal allowed")
	}
	agent.role = "reviewer"
	if err := a.authorizePublication(agent, "repo", "attempt-one"); err == nil {
		t.Fatal("read-only role published")
	}
	// A policy change between execution preflight and publication is re-read.
	agent.role = "conflict-resolver"
	if err := a.authorizePublication(agent, "repo", "attempt-one"); err != nil {
		t.Fatal(err)
	}
	records["repository_settings"] = []map[string]any{{"repo_id": "meta", "archived": "true"}}
	if err := a.authorizePublication(agent, "repo", "attempt-one"); err == nil {
		t.Fatal("revoked policy ignored")
	}
}

func TestWorkflowRejectsCanonicalAndUnsupportedRoleBeforeExecution(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{"repository_meta": {{"id": "meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "default_branch": "main"}}})
	e := &wfExec{a: a, runID: "test", actor: "alice", params: map[string]any{"repo": "repo", "branch": "main"}, budget: 10}
	if _, err := e.opAgent(map[string]any{"file": "file.go", "role": "implementer"}); err == nil {
		t.Fatal("canonical agent accepted")
	}
	if _, err := e.opAgent(map[string]any{"file": "file.go", "role": "reviewer"}); err == nil {
		t.Fatal("unsupported role silently became implementer")
	}
}

// Guard against a new HTTP/background call bypassing the single application
// boundary. The refs package remains the mechanical Git substrate.
func TestApplicationPublicationCallsUseBoundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "publication_authority.go" {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"Refs.Update(", "Refs.PublishPrepared(", "Refs.PublishRepairTree(", "Refs.ResolveIntoSource(", "Refs.MergeBranch("} {
			if strings.Contains(string(data), method) {
				t.Fatalf("%s bypasses publication boundary with %s", name, method)
			}
		}
	}
}
