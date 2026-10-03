package process

import (
	"strings"
	"testing"
	"time"
)

func TestGitCredentialLivesOnlyInChildEnvironment(t *testing.T) {
	cmd, cancel := Command(time.Second, "git", "clone", "-c", "http.extraHeader=Authorization: Bearer fixture-secret", "--quiet", "remote", "target")
	defer cancel()
	if strings.Contains(strings.Join(cmd.Args, " "), "fixture-secret") {
		t.Fatal("credential remains in argv")
	}
	if !strings.Contains(strings.Join(cmd.Env, "\n"), "GIT_CONFIG_VALUE_0=Authorization: Bearer fixture-secret") {
		t.Fatal("child environment configuration missing")
	}
	if strings.Contains(strings.Join(cmd.Args, " "), "http.extraHeader") {
		t.Fatal("clone could persist credential option")
	}
}
