package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"switchyard/internal/refs"
	"sync"
	"syscall"
	"time"
)

const maxResultBytes = 8 << 20

type Task struct {
	Role     string `json:"role,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Repo     string `json:"repo"`
	Branch   string `json:"branch"`
	File     string `json:"file"`
	Current  string `json:"current"`
	Prompt   string `json:"prompt"`
}

func (t Task) Validate() error {
	if err := refs.ValidatePath(t.File); err != nil {
		return err
	}
	if len(t.Current)+len(t.Prompt) > maxResultBytes {
		return errors.New("agent task exceeds limit")
	}
	return nil
}

type RunnerEvent struct {
	Kind   string    `json:"kind"`
	Stream string    `json:"stream,omitempty"`
	Text   string    `json:"text,omitempty"`
	At     time.Time `json:"at"`
}
type RunnerLimits struct {
	Timeout         time.Duration
	OutputBytes     int
	MemoryBytes     uint64
	CPUSeconds      int
	Cgroup          bool
	PIDs            int
	CPUQuotaPercent int
}

// CLIRunner executes a configured adapter inside bubblewrap's private mount,
// PID and user namespaces. Missing sandbox/limit tools fail closed. Its JSON
// stdin request and JSON stdout result are an adapter protocol, not a shell.
type CLIRunner struct {
	gateOnce      sync.Once
	gate          chan struct{}
	RuntimeBinary string
	Bin           string
	Args          []string
	WorkDir       string
	Credential    func() (string, error)
	Limits        RunnerLimits
	AllowNetwork  bool
	OnEvent       func(RunnerEvent)
}

func (r *CLIRunner) Run(ctx context.Context, ex *Execution, task Task) (runErr error) {
	defer func() {
		if runErr != nil && ex.FailureCode == "" {
			if ex.Adapter != "cli-sandbox" {
				ex.FailureCode = "runner_start_failed"
			} else {
				ex.FailureCode = "runner_invalid_output"
			}
		}
	}()
	if err := task.Validate(); err != nil {
		return err
	}
	r.gateOnce.Do(func() { r.gate = make(chan struct{}, 1) })
	select {
	case r.gate <- struct{}{}:
		defer func() { <-r.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if os.Geteuid() == 0 {
		return errors.New("sandbox runner requires a dedicated non-root service user")
	}
	if !filepath.IsAbs(r.Bin) {
		return errors.New("agent binary must be an absolute configured path")
	}
	binary, err := filepath.EvalSymlinks(r.Bin)
	if err != nil {
		return err
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return errors.New("Agent sandbox unavailable: bubblewrap required")
	}
	limiter, err := exec.LookPath("prlimit")
	if err != nil {
		return errors.New("Agent resource limiter unavailable: prlimit required")
	}
	dir := r.WorkDir
	if dir == "" {
		dir, err = os.MkdirTemp("", "switchyard-agent-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
	}
	if !filepath.IsAbs(dir) {
		return errors.New("agent workdir must be absolute")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("agent workdir must be a real private directory")
	}
	if info.Mode().Perm()&0077 != 0 {
		return errors.New("agent workdir must be private")
	}
	credential := ""
	secret := ""
	if r.Credential != nil {
		credential, err = r.Credential()
		if err != nil {
			return err
		}
		if credential != "" {
			defer os.Remove(credential)
			info, err := os.Lstat(credential)
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
				return errors.New("credential file must be private and regular")
			}
			data, err := os.ReadFile(credential)
			if err != nil {
				return err
			}
			if len(data) > 64<<10 {
				return errors.New("credential exceeds limit")
			}
			secret = string(data)
		}
	}
	timeout := r.Limits.Timeout
	if timeout <= 0 || timeout > 10*time.Minute {
		timeout = 2 * time.Minute
	}
	outputLimit := r.Limits.OutputBytes
	if outputLimit <= 0 || outputLimit > 8<<20 {
		outputLimit = 1 << 20
	}
	memory := r.Limits.MemoryBytes
	if memory == 0 || memory > 4<<30 {
		memory = 2 << 30
	}
	cpu := r.Limits.CPUSeconds
	if cpu <= 0 || cpu > 600 {
		cpu = 120
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"--cpu=" + fmt.Sprint(cpu), "--nofile=128", "--core=0", "--fsize=16777216", "--", bwrap, "--die-with-parent", "--new-session", "--unshare-all", "--clearenv"}
	if !r.Limits.Cgroup {
		args = append([]string{"--as=" + fmt.Sprint(memory)}, args...)
	}
	if r.AllowNetwork {
		args = append(args, "--share-net")
	}
	for _, path := range []string{"/usr", "/bin", "/lib", "/lib64"} {
		if _, err := os.Stat(path); err == nil {
			args = append(args, "--ro-bind", path, path)
		}
	}
	args = append(args, "--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--dir", "/run", "--dir", "/runner", "--dir", "/home", "--dir", "/home/agent", "--bind", dir, "/workspace", "--ro-bind", binary, "/runner/agent", "--chdir", "/workspace", "--setenv", "PATH", "/usr/bin:/bin", "--setenv", "HOME", "/home/agent", "--setenv", "LANG", "C.UTF-8")
	if r.AllowNetwork {
		for _, path := range []string{"/etc/resolv.conf", "/etc/hosts", "/etc/ssl"} {
			if _, err := os.Stat(path); err == nil {
				args = append(args, "--ro-bind", path, path)
			}
		}
	}
	if r.RuntimeBinary != "" {
		if !filepath.IsAbs(r.RuntimeBinary) {
			return errors.New("OpenCode runtime must be an absolute configured path")
		}
		runtime, err := filepath.EvalSymlinks(r.RuntimeBinary)
		if err != nil {
			return err
		}
		args = append(args, "--ro-bind", runtime, "/runner/opencode", "--setenv", "SWITCHYARD_OPENCODE_BIN", "/runner/opencode")
	}
	if credential != "" {
		args = append(args, "--ro-bind", credential, "/run/credential", "--setenv", "SWITCHYARD_CREDENTIAL_FILE", "/run/credential")
	}
	if r.Limits.Cgroup {
		args = append(args, "--", "/runner/agent")
	} else {
		args = append(args, "--", limiter, "--nproc=256", "--", "/runner/agent")
	}
	args = append(args, r.Args...)
	command := limiter
	unit := ""
	if r.Limits.Cgroup {
		pids := r.Limits.PIDs
		if pids == 0 {
			pids = 128
		}
		quota := r.Limits.CPUQuotaPercent
		if quota == 0 {
			quota = 200
		}
		if pids < 16 || pids > 1024 || quota < 10 || quota > 800 {
			return errors.New("invalid cgroup PID/CPU policy")
		}
		systemdRun, lookupErr := exec.LookPath("systemd-run")
		if lookupErr != nil {
			return errors.New("cgroup runner requires systemd-run user service support")
		}
		random := make([]byte, 12)
		if _, err := rand.Read(random); err != nil {
			return err
		}
		unit = "switchyard-agent-" + hex.EncodeToString(random) + ".service"
		args = append([]string{"--user", "--wait", "--pipe", "--quiet", "--collect", "--service-type=exec", "--unit=" + unit, "--property=MemoryMax=" + fmt.Sprint(memory), "--property=MemorySwapMax=0", "--property=TasksMax=" + fmt.Sprint(pids), "--property=CPUQuota=" + fmt.Sprint(quota) + "%", "--property=RuntimeMaxSec=" + fmt.Sprint(int(timeout.Seconds())+2), "--property=KillMode=control-group", "--property=Slice=app.slice", "--", limiter}, args...)
		command = systemdRun
	}
	stopUnit := func() {
		if unit == "" {
			return
		}
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		stop := exec.CommandContext(stopCtx, "systemctl", "--user", "stop", unit)
		stop.Env = []string{"PATH=/usr/bin:/bin", "XDG_RUNTIME_DIR=/run/user/" + fmt.Sprint(os.Getuid()), "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/" + fmt.Sprint(os.Getuid()) + "/bus"}
		_ = stop.Run()
	}
	defer stopUnit()
	cmd := exec.CommandContext(runCtx, command, args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	if unit != "" {
		cmd.Env = append(cmd.Env, "XDG_RUNTIME_DIR=/run/user/"+fmt.Sprint(os.Getuid()), "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/"+fmt.Sprint(os.Getuid())+"/bus")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	cmd.Cancel = func() error {
		stopUnit()
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	request, err := json.Marshal(task)
	if err != nil {
		return err
	}
	cmd.Stdin = strings.NewReader(string(request))
	output := &runnerOutput{limit: outputLimit, secret: secret}
	cmd.Stdout = &runnerStream{output: output, stream: "stdout"}
	cmd.Stderr = &runnerStream{output: output, stream: "stderr"}
	ex.Adapter = "cli-sandbox"
	ex.Status = "running"
	ex.Started = time.Now().UTC()
	emit := func(kind, stream, text string) {
		if r.OnEvent != nil {
			r.OnEvent(RunnerEvent{Kind: kind, Stream: stream, Text: text, At: time.Now().UTC()})
		}
	}
	defer func() {
		if runErr != nil && ex.Status == "succeeded" {
			ex.Status = "failed"
		}
		emit("finished", "", ex.Status)
	}()
	emit("started", "", "")
	var finishAccounting func() map[string]string
	if unit != "" {
		finishAccounting = watchCgroup(unit)
	}
	err = cmd.Run()
	if finishAccounting != nil {
		ex.ResourceUsage = finishAccounting()
	}
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	ex.Finished = time.Now().UTC()
	ex.Stdout = output.text("stdout")
	ex.Stderr = output.text("stderr")
	ex.Output = ex.Stdout + ex.Stderr
	ex.OutputTruncated = output.truncated
	ex.ExitCode = -1
	if cmd.ProcessState != nil {
		ex.ExitCode = cmd.ProcessState.ExitCode()
		ex.CPUTime = cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
	}
	if usec := resourceCounter(ex.ResourceUsage["cpu.stat"], "usage_usec"); usec > 0 {
		ex.CPUTime = time.Duration(usec) * time.Microsecond
	}
	emit("output", "stdout", ex.Stdout)
	emit("output", "stderr", ex.Stderr)
	switch {
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		ex.Status = "timed_out"
		ex.FailureCode = "runner_wall_timeout"
		err = context.DeadlineExceeded
	case errors.Is(runCtx.Err(), context.Canceled):
		ex.Status = "cancelled"
		ex.FailureCode = "runner_cancelled"
		err = context.Canceled
	case err != nil:
		ex.Status = "failed"
		ex.FailureCode = resourceFailureCode(ex.ResourceUsage)
	default:
		ex.Status = "succeeded"
	}
	if err != nil {
		return fmt.Errorf("Agent %s (%s, exit %d): %w", ex.Status, ex.FailureCode, ex.ExitCode, err)
	}
	var result struct {
		Files map[string]string `json:"files"`
	}
	if output.truncated {
		ex.FailureCode = "runner_invalid_output"
		return errors.New("agent result output was truncated")
	}
	if err := json.Unmarshal([]byte(output.rawStdout()), &result); err != nil {
		ex.FailureCode = "runner_invalid_output"
		return fmt.Errorf("invalid Agent JSON result: %w", err)
	}
	total := 0
	for path, content := range result.Files {
		if secret != "" && strings.Contains(content, secret) {
			return errors.New("Agent result contains scoped credential")
		}
		if err := refs.ValidatePath(path); err != nil {
			return err
		}
		total += len(content)
		if total > maxResultBytes {
			return errors.New("agent result exceeds limit")
		}
	}
	if len(result.Files) == 0 {
		return errors.New("agent result has no files")
	}
	ex.Result = result.Files
	return nil
}

type runnerOutput struct {
	mu             sync.Mutex
	limit, used    int
	secret         string
	stdout, stderr strings.Builder
	truncated      bool
}
type runnerStream struct {
	output *runnerOutput
	stream string
}

func (s *runnerStream) Write(data []byte) (int, error) {
	n := len(data)
	o := s.output
	o.mu.Lock()
	defer o.mu.Unlock()
	remaining := o.limit - o.used
	if len(data) > remaining {
		data = data[:max(remaining, 0)]
		o.truncated = true
	}
	if s.stream == "stdout" {
		o.stdout.Write(data)
	} else {
		o.stderr.Write(data)
	}
	o.used += len(data)
	return n, nil
}
func (o *runnerOutput) text(stream string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	text := o.stdout.String()
	if stream == "stderr" {
		text = o.stderr.String()
	}
	if o.secret != "" {
		text = strings.ReplaceAll(text, o.secret, "[REDACTED]")
	}
	return text
}
func (o *runnerOutput) rawStdout() string { o.mu.Lock(); defer o.mu.Unlock(); return o.stdout.String() }
