package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func scriptRunner(t *testing.T, body string) *CLIRunner {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "adapter")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return &CLIRunner{Bin: path, Limits: RunnerLimits{Timeout: time.Second}}
}
func fixtureTask() Task {
	return Task{Repo: "repo", Branch: "main", File: "file.go", Current: "old", Prompt: "new"}
}

func TestDeterministicTypedRunnerParityAndCancellation(t *testing.T) {
	r := &DeterministicRunner{}
	for _, test := range []struct{ current, prompt string }{{"", ""}, {"plain", " append"}, {"λ日本語", "⚡\n"}} {
		ex := &Execution{}
		task := fixtureTask()
		task.Current = test.current
		task.Prompt = test.prompt
		if err := r.Run(context.Background(), ex, task); err != nil || ex.Result[task.File] != test.current+test.prompt {
			t.Fatalf("parity: %v %v", ex, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Run(ctx, &Execution{}, fixtureTask()); !errors.Is(err, context.Canceled) {
		t.Fatal("ignored cancellation")
	}
}

func TestSandboxRunnerJSONEnvironmentAndFilesystem(t *testing.T) {
	secretOutside := filepath.Join(t.TempDir(), "host-secret")
	if err := os.WriteFile(secretOutside, []byte("host secret"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SWITCHYARD_HOST_SECRET", "host environment secret")
	r := scriptRunner(t, `test -z "${SWITCHYARD_HOST_SECRET-}"
test ! -e "`+secretOutside+`"
cat >/workspace/request.json
printf 'diagnostic\n' >&2
printf '{"files":{"file.go":"new content"}}'
`)
	ex := &Execution{}
	events := []RunnerEvent{}
	r.OnEvent = func(event RunnerEvent) { events = append(events, event) }
	if err := r.Run(context.Background(), ex, fixtureTask()); err != nil {
		t.Fatal(err)
	}
	if ex.Result["file.go"] != "new content" || ex.Stderr != "diagnostic\n" || ex.ExitCode != 0 || ex.Status != "succeeded" {
		t.Fatalf("result: %+v", ex)
	}
	if len(events) < 4 || events[0].Kind != "started" || events[len(events)-1].Kind != "finished" {
		t.Fatal("missing attribution events")
	}
}

func TestSandboxRunnerTimeoutOutputLimitAndCredentialCleanup(t *testing.T) {
	for _, mode := range []string{"timeout", "output", "secret"} {
		t.Run(mode, func(t *testing.T) {
			body := `sleep 20 & wait`
			if mode == "output" {
				body = `head -c 1048576 /dev/zero | tr '\000' x`
			}
			if mode == "secret" {
				body = `cat "$SWITCHYARD_CREDENTIAL_FILE" >&2; printf '{"files":{"file.go":"safe"}}'`
			}
			r := scriptRunner(t, body)
			r.Limits.Timeout = 100 * time.Millisecond
			r.Limits.OutputBytes = 1024
			credential := filepath.Join(t.TempDir(), "credential")
			if err := os.WriteFile(credential, []byte("scoped-secret"), 0600); err != nil {
				t.Fatal(err)
			}
			r.Credential = func() (string, error) { return credential, nil }
			ex := &Execution{}
			started := time.Now()
			err := r.Run(context.Background(), ex, fixtureTask())
			if time.Since(started) > 3*time.Second {
				t.Fatal("unbounded child process cleanup")
			}
			if _, err := os.Stat(credential); !os.IsNotExist(err) {
				t.Fatal("credential survived execution")
			}
			if strings.Contains(ex.Output, "scoped-secret") {
				t.Fatal("credential leaked in output")
			}
			switch mode {
			case "timeout":
				if !errors.Is(err, context.DeadlineExceeded) || ex.Status != "timed_out" {
					t.Fatalf("timeout: %v %+v", err, ex)
				}
			case "output":
				if err == nil || !ex.OutputTruncated || len(ex.Output) > 1024 || ex.Status != "failed" {
					t.Fatalf("output: %v %+v", err, ex)
				}
			case "secret":
				if err != nil || !strings.Contains(ex.Stderr, "[REDACTED]") {
					t.Fatalf("redaction: %v %+v", err, ex)
				}
			}
		})
	}
}

func TestSandboxRunnerKillsBackgroundDescendants(t *testing.T) {
	r := scriptRunner(t, `(sleep 0.4; printf survived >/workspace/escaped) & sleep 20`)
	r.WorkDir = t.TempDir()
	if err := os.Chmod(r.WorkDir, 0700); err != nil {
		t.Fatal(err)
	}
	r.Limits.Timeout = 100 * time.Millisecond
	if err := r.Run(context.Background(), &Execution{}, fixtureTask()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(r.WorkDir, "escaped")); !os.IsNotExist(err) {
		t.Fatal("background descendant survived sandbox termination")
	}
}

func TestSandboxRunnerRejectsCredentialInFileResult(t *testing.T) {
	r := scriptRunner(t, `printf '{"files":{"file.go":"scoped-secret"}}'`)
	credential := filepath.Join(t.TempDir(), "credential")
	if err := os.WriteFile(credential, []byte("scoped-secret"), 0600); err != nil {
		t.Fatal(err)
	}
	r.Credential = func() (string, error) { return credential, nil }
	ex := &Execution{}
	if err := r.Run(context.Background(), ex, fixtureTask()); err == nil || ex.Status != "failed" || ex.Result != nil || strings.Contains(ex.Output, "scoped-secret") {
		t.Fatalf("unsafe result: %v %+v", err, ex)
	}
}
