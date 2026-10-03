package refs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
