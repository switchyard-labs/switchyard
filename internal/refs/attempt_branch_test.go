package refs

import (
	"net/http"
	"os"
	"path/filepath"
	"switchyard/internal/artifacts"
	"testing"
)

func TestAttemptBranchesPreserveSourceTreeAndCommit(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	fixtureGit(t, root, "init", "--bare", remote)
	fixtureGit(t, root, "init", "-b", "main", local)
	os.WriteFile(filepath.Join(local, "code.txt"), []byte("real source"), 0600)
	fixtureGit(t, local, "add", ".")
	fixtureGit(t, local, "commit", "-m", "base")
	fixtureGit(t, local, "push", remote, "main")
	head := fixtureGit(t, local, "rev-parse", "HEAD")
	helper := filepath.Join(root, "token.sh")
	os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700)
	client := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
	scratch := filepath.Join(root, "scratch")
	os.Mkdir(scratch, 0700)
	service := NewService(client, nil, scratch)
	for _, branch := range []string{"attempt-a", "attempt-b"} {
		result, err := service.Update("repo", branch, "", nil, "Attempt metadata only", "test")
		if err != nil || result.Status != "ok" || result.NewSHA != head {
			t.Fatalf("branch %s: %+v %v", branch, result, err)
		}
	}
	if actual := fixtureGit(t, root, "--git-dir", remote, "ls-tree", "--name-only", "attempt-a"); actual != "code.txt" {
		t.Fatalf("synthetic file added: %s", actual)
	}
	if actual := fixtureGit(t, root, "--git-dir", remote, "rev-parse", "main"); actual != head {
		t.Fatal("main changed")
	}
	result, err := service.Update("repo", "attempt-a", "", nil, "collision", "test")
	if err != nil || result.Status != "stale" {
		t.Fatal("existing branch accepted as new")
	}
}
