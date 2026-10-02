package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExecutionRecord is the durable record of an Agent/check execution.
type ExecutionRecord struct {
	ID         string
	Role       string
	AttemptID  string
	Adapter    string
	Status     string
	Output     string
	Started    time.Time
	Finished   time.Time
}

func (a *App) recordExecution(ex *ExecutionRecord) {
	_, _, _ = a.Trestle.CreateRecord("executions", map[string]any{
		"id": ex.ID, "role": ex.Role, "attempt_id": ex.AttemptID, "adapter": ex.Adapter,
		"status": ex.Status, "output": ex.Output,
		"started_at": ex.Started.Format(time.RFC3339), "finished_at": ex.Finished.Format(time.RFC3339),
	}, "exec-"+ex.ID)
}

func scratchDir(dataDir, name string) string {
	return filepath.Join(dataDir, "scratch", name)
}

func removeAll(dir string) { _ = os.RemoveAll(dir) }

func mkdirAll(dir string, perm os.FileMode) { _ = os.MkdirAll(dir, perm) }

func runGit(dir, stdin string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return &gitError{Out: strings.TrimSpace(string(out))}
	}
	return nil
}

func runGitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type gitError struct{ Out string }

func (e *gitError) Error() string { return "git: " + e.Out }