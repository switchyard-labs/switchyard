package app

import (
	"net/http"
	"os"
	"path/filepath"
	"switchyard/internal/artifacts"
	"testing"
)

func TestOperatorImportPreservesOwnershipAndIsIdempotent(t *testing.T) {
	a, store, _ := actionFixture(t)
	root := t.TempDir()
	helper := filepath.Join(root, "fixture-token")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	a.Artifacts = artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: queueArtifactTransport{}})
	if _, err := a.ImportUserRepository("repo", "alice", "railway", "public"); err == nil {
		t.Fatal("unknown owner accepted")
	}
	_, _, err := a.Trestle.CreateRecord("users", map[string]any{"username": "alice"}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.ImportUserRepository("repo", "alice", "railway", "public")
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ImportUserRepository("repo", "alice", "railway", "public")
	if err != nil || first["id"] != second["id"] {
		t.Fatalf("idempotent import: %v", err)
	}
	for _, args := range [][3]string{{"alice", "other", "public"}, {"alice", "railway", "private"}} {
		if _, err := a.ImportUserRepository("repo", args[0], args[1], args[2]); err == nil {
			t.Fatal("existing identity overwritten")
		}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.records["repository_meta"]) != 1 {
		t.Fatal("duplicate import record")
	}
}
