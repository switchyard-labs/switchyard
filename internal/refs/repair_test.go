package refs

import (
	"net/http"
	"os"
	"path/filepath"
	"switchyard/internal/artifacts"
	"testing"
)

func TestRepairPreservesBothParentsAndCanonicalRef(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	fixtureGit(t, root, "init", "--bare", remote)
	fixtureGit(t, root, "init", "-b", "main", local)
	os.WriteFile(filepath.Join(local, "api"), []byte("1"), 0600)
	os.WriteFile(filepath.Join(local, "consumer"), []byte("1"), 0600)
	fixtureGit(t, local, "add", ".")
	fixtureGit(t, local, "commit", "-m", "base")
	fixtureGit(t, local, "push", remote, "main")
	fixtureGit(t, local, "checkout", "-b", "source")
	os.WriteFile(filepath.Join(local, "rule"), []byte("api equals consumer"), 0600)
	fixtureGit(t, local, "add", ".")
	fixtureGit(t, local, "commit", "-m", "rule")
	source := fixtureGit(t, local, "rev-parse", "HEAD")
	fixtureGit(t, local, "push", remote, "source")
	fixtureGit(t, local, "checkout", "main")
	os.WriteFile(filepath.Join(local, "api"), []byte("2"), 0600)
	fixtureGit(t, local, "commit", "-am", "api")
	canonical := fixtureGit(t, local, "rev-parse", "HEAD")
	fixtureGit(t, local, "push", remote, "main")
	helper := filepath.Join(root, "token.sh")
	os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700)
	client := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
	scratch := filepath.Join(root, "scratch")
	os.Mkdir(scratch, 0700)
	service := NewService(client, nil, scratch)
	tree, conflicts, err := service.PreviewConflictTree("repo", "main", "source")
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("preview: %v %v", conflicts, err)
	}
	defer os.RemoveAll(tree)
	os.WriteFile(filepath.Join(tree, "consumer"), []byte("2"), 0600)
	if _, err = service.PublishRepairTree("repo", "source", canonical, tree, "bad expected source", "test"); err == nil {
		t.Fatal("accepted wrong source")
	}
	result, err := service.PublishRepairTree("repo", "source", source, tree, "repair", "test")
	if err != nil {
		t.Fatal(err)
	}
	parents := fixtureGit(t, root, "--git-dir", remote, "show", "-s", "--format=%P", result.NewSHA)
	if parents != canonical+" "+source {
		t.Fatalf("parents: %s", parents)
	}
	if got := fixtureGit(t, root, "--git-dir", remote, "rev-parse", "main"); got != canonical {
		t.Fatal("canonical changed during repair")
	}
	if got := fixtureGit(t, root, "--git-dir", remote, "show", "source:api"); got != "2" {
		t.Fatal("canonical change lost")
	}
}
