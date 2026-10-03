package refs

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"switchyard/internal/artifacts"
	"testing"
)

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@local", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@local")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestPublicationRequiresExactRemoteSHA(t *testing.T) {
	for _, movement := range []string{"unchanged", "rewound", "advanced", "created", "receive_race"} {
		t.Run(movement, func(t *testing.T) {
			root := t.TempDir()
			remote := filepath.Join(root, "remote.git")
			local := filepath.Join(root, "local")
			fixtureGit(t, root, "init", "--bare", remote)
			fixtureGit(t, root, "init", "-b", "main", local)
			commit := func(content string) string {
				t.Helper()
				if err := os.WriteFile(filepath.Join(local, "file"), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				fixtureGit(t, local, "add", "file")
				fixtureGit(t, local, "commit", "-m", content)
				return fixtureGit(t, local, "rev-parse", "HEAD")
			}
			first := commit("first")
			fixtureGit(t, local, "push", remote, "HEAD:refs/heads/main")
			expected := commit("second")
			fixtureGit(t, local, "push", remote, "HEAD:refs/heads/main")
			candidate := commit("candidate")
			branch := "main"
			switch movement {
			case "rewound":
				fixtureGit(t, root, "--git-dir="+remote, "update-ref", "refs/heads/main", first)
			case "advanced":
				fixtureGit(t, local, "push", remote, "HEAD:refs/heads/external")
				fixtureGit(t, root, "--git-dir="+remote, "update-ref", "refs/heads/main", candidate)
			case "receive_race":
				hook := "#!/bin/sh\nunset GIT_QUARANTINE_PATH\ngit update-ref refs/heads/main " + first + "\n"
				if err := os.WriteFile(filepath.Join(remote, "hooks", "pre-receive"), []byte(hook), 0700); err != nil {
					t.Fatal(err)
				}
			case "created":
				branch = "new"
				expected = ""
			}
			before := fixtureGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
			got, err := pushExpected(local, remote, branch, expected, nil)
			if movement == "unchanged" || movement == "created" {
				if err != nil || got != candidate {
					t.Fatalf("publication: %s %v", got, err)
				}
				if actual := fixtureGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/"+branch); actual != candidate {
					t.Fatal("candidate not published")
				}
			} else {
				if err == nil {
					t.Fatal("accepted stale expected SHA")
				}
				if actual := fixtureGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main"); actual != before && !(movement == "receive_race" && actual == first) {
					t.Fatal("stale publication changed remote")
				}
			}
		})
	}
}

type artifactFixtureTransport struct{ remote string }

func (f artifactFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body := `{"result":{"plaintext":"fixture"}}`
	if r.Method == "GET" {
		encoded, _ := json.Marshal(map[string]any{"result": map[string]any{"name": "repo", "remote": f.remote, "default_branch": "main"}})
		body = string(encoded)
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func TestPreparedCandidateRejectsMovementAndMutation(t *testing.T) {
	for _, movement := range []string{"none", "base", "source", "worktree", "commit"} {
		t.Run(movement, func(t *testing.T) {
			root := t.TempDir()
			remote := filepath.Join(root, "remote.git")
			local := filepath.Join(root, "local")
			fixtureGit(t, root, "init", "--bare", remote)
			fixtureGit(t, root, "init", "-b", "main", local)
			os.WriteFile(filepath.Join(local, "base"), []byte("base"), 0600)
			fixtureGit(t, local, "add", ".")
			fixtureGit(t, local, "commit", "-m", "base")
			base := fixtureGit(t, local, "rev-parse", "HEAD")
			fixtureGit(t, local, "push", remote, "main")
			fixtureGit(t, local, "checkout", "-b", "source")
			os.WriteFile(filepath.Join(local, "source"), []byte("source"), 0600)
			fixtureGit(t, local, "add", ".")
			fixtureGit(t, local, "commit", "-m", "source")
			fixtureGit(t, local, "push", remote, "source")
			helper := filepath.Join(root, "token.sh")
			os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700)
			client := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
			scratch := filepath.Join(root, "scratch")
			os.Mkdir(scratch, 0700)
			service := NewService(client, nil, scratch)
			candidate, err := service.PrepareMerge("repo", "main", "source", "candidate")
			if err != nil {
				t.Fatal(err)
			}
			defer candidate.Close()
			switch movement {
			case "base", "source":
				fixtureGit(t, local, "checkout", movementBranch(movement))
				os.WriteFile(filepath.Join(local, "external"), []byte("external"), 0600)
				fixtureGit(t, local, "add", ".")
				fixtureGit(t, local, "commit", "-m", "external")
				fixtureGit(t, local, "push", remote, "HEAD:refs/heads/"+movementBranch(movement))
			case "worktree":
				os.WriteFile(filepath.Join(candidate.Dir, "source"), []byte("changed"), 0600)
			case "commit":
				fixtureGit(t, candidate.Dir, "reset", "--hard", base)
			}
			before := fixtureGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
			result, err := service.PublishPrepared(candidate)
			after := fixtureGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
			if movement == "none" {
				if err != nil || result.Status != "ok" || after != candidate.CommitSHA {
					t.Fatalf("publication result=%v err=%v", result, err)
				}
				published, err := service.CandidatePublished(candidate)
				if err != nil || !published {
					t.Fatalf("publication recovery: %v %v", published, err)
				}
			} else {
				if err == nil && result.Status != "stale" {
					t.Fatalf("accepted %s: %v", movement, result)
				}
				if after != before {
					t.Fatal("invalid candidate changed canonical ref")
				}
			}
		})
	}
}
func movementBranch(movement string) string {
	if movement == "base" {
		return "main"
	}
	return "source"
}

func TestFileMovePreservesExecutableMode(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	fixtureGit(t, root, "init", "--bare", remote)
	fixtureGit(t, root, "init", "-b", "main", local)
	if err := os.WriteFile(filepath.Join(local, "run.sh"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, local, "add", ".")
	fixtureGit(t, local, "commit", "-m", "executable")
	base := fixtureGit(t, local, "rev-parse", "HEAD")
	fixtureGit(t, local, "push", remote, "main")
	helper := filepath.Join(root, "token.sh")
	os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700)
	client := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
	scratch := filepath.Join(root, "scratch")
	os.Mkdir(scratch, 0700)
	service := NewService(client, nil, scratch)
	result, err := service.Update("repo", "main", base, []Change{{Path: "run.sh", Delete: true}, {Path: "scripts/run.sh", Content: "#!/bin/sh\nexit 0\n", Mode: "100755"}}, "move executable", "C20-test")
	if err != nil || result.Status != "ok" {
		t.Fatalf("move: %+v %v", result, err)
	}
	actual := fixtureGit(t, root, "--git-dir="+remote, "ls-tree", "main", "scripts/run.sh")
	if !strings.HasPrefix(actual, "100755 blob") {
		t.Fatalf("lost executable mode: %s", actual)
	}
}
