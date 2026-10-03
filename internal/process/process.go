// Package process supplies cancellation and bounded output for trusted Git
// transport helpers. Repository programs use the stronger Agent sandbox.
package process

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

func Command(timeout time.Duration, name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	return cmd, cancel
}

type Output struct {
	mu        sync.Mutex
	data      bytes.Buffer
	Limit     int
	Truncated bool
}

func (o *Output) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(data)
	limit := o.Limit
	if limit <= 0 {
		limit = 16 << 20
	}
	remaining := limit - o.data.Len()
	if n > remaining {
		data = data[:max(remaining, 0)]
		o.Truncated = true
	}
	o.data.Write(data)
	return n, nil
}
func (o *Output) String() string { o.mu.Lock(); defer o.mu.Unlock(); return o.data.String() }
func Capture(cmd *exec.Cmd, combined bool) (string, error) {
	var output, stderr Output
	cmd.Stdout = &output
	if combined {
		cmd.Stderr = &output
	} else {
		cmd.Stderr = &stderr
	}
	err := cmd.Run()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	if output.Truncated || stderr.Truncated {
		return "", errors.New("process output exceeds 16 MiB")
	}
	return output.String(), err
}
