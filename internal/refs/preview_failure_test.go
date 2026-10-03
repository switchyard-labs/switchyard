package refs

import (
	"net/http"
	"os"
	"path/filepath"
	"switchyard/internal/artifacts"
	"testing"
)

func TestPreviewRejectsNonConflictMergeFailureAndCleansScratch(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	fixtureGit(t, root, "init", "--bare", remote)
	fixtureGit(t, root, "init", "-b", "main", local)
	if err := os.WriteFile(filepath.Join(local, "base"), []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, local, "add", ".")
	fixtureGit(t, local, "commit", "-m", "base")
	fixtureGit(t, local, "push", remote, "main")
	fixtureGit(t, local, "checkout", "--orphan", "source")
	fixtureGit(t, local, "commit", "-m", "unrelated")
	fixtureGit(t, local, "push", remote, "source")
	helper := filepath.Join(root, "token.sh")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700); err != nil {
		t.Fatal(err)
	}
	client := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
	scratch := filepath.Join(root, "scratch")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	service := NewService(client, nil, scratch)
	if _, err := service.PreviewMerge("repo", "main", "source"); err == nil {
		t.Fatal("unrelated-history failure accepted as clean merge")
	}
	if _, err := service.PreviewMergedTree("repo", "main", "source"); err == nil {
		t.Fatal("unrelated-history failure accepted as textual conflict")
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("failed previews leaked scratch directories")
	}
}

func TestCombinedPreviewReturnsOneTreeOrConflictAndNeverPublishes(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	fixtureGit(t, root, "init", "--bare", remote)
	fixtureGit(t, root, "init", "-b", "main", local)
	file := filepath.Join(local, "file with spaces.txt")
	if err := os.WriteFile(file, []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, local, "add", ".")
	fixtureGit(t, local, "commit", "-m", "base")
	fixtureGit(t, local, "push", remote, "main")
	fixtureGit(t, local, "checkout", "-b", "source")
	if err := os.WriteFile(file, []byte("source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, local, "commit", "-am", "source")
	fixtureGit(t, local, "push", remote, "source")
	helper := filepath.Join(root, "token.sh")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700); err != nil {
		t.Fatal(err)
	}
	client := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
	scratch := filepath.Join(root, "scratch")
	if err := os.Mkdir(scratch, 0700); err != nil {
		t.Fatal(err)
	}
	service := NewService(client, nil, scratch)
	before := fixtureGit(t, root, "--git-dir", remote, "rev-parse", "refs/heads/main")
	dir, conflicts, err := service.PreviewMergedTreeWithConflicts("repo", "main", "source")
	if err != nil || dir == "" || len(conflicts) != 0 {
		t.Fatalf("clean dir=%q conflicts=%v err=%v", dir, conflicts, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "file with spaces.txt"))
	if err != nil || string(data) != "source\n" {
		t.Fatalf("tree=%q err=%v", data, err)
	}
	entries, _ := os.ReadDir(scratch)
	if len(entries) != 1 {
		t.Fatal("expected exactly one caller-owned scratch tree")
	}
	os.RemoveAll(dir)
	fixtureGit(t, local, "checkout", "main")
	if err := os.WriteFile(file, []byte("main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, local, "commit", "-am", "main")
	fixtureGit(t, local, "push", remote, "main")
	before = fixtureGit(t, root, "--git-dir", remote, "rev-parse", "refs/heads/main")
	dir, conflicts, err = service.PreviewMergedTreeWithConflicts("repo", "main", "source")
	if err != nil || dir != "" || len(conflicts) != 1 || conflicts[0] != "file with spaces.txt" {
		t.Fatalf("conflict dir=%q files=%v err=%v", dir, conflicts, err)
	}
	entries, _ = os.ReadDir(scratch)
	if len(entries) != 0 {
		t.Fatal("conflicted preview leaked scratch")
	}
	after := fixtureGit(t, root, "--git-dir", remote, "rev-parse", "refs/heads/main")
	if after != before {
		t.Fatal("preview changed canonical ref")
	}
}
