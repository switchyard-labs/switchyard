package app

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"switchyard/internal/agent"
	"time"
)

// ConfigureProviderRunner accepts only an administrator-installed JSON adapter,
// never a command supplied by a user. Every execution has a fresh workspace
// and personal credential file. The shared gate bounds concurrent invocations.
func (a *App) ConfigureProviderRunner(binary string, concurrency int) error {
	if raw := os.Getenv("SWITCHYARD_AGENT_CONCURRENCY"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 4 {
			return fmt.Errorf("SWITCHYARD_AGENT_CONCURRENCY must be 1–4")
		}
		concurrency = n
	}
	if concurrency < 1 || concurrency > 4 {
		concurrency = 2
	}
	queueDepth := 16
	if raw := os.Getenv("SWITCHYARD_AGENT_QUEUE_DEPTH"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 256 {
			return fmt.Errorf("SWITCHYARD_AGENT_QUEUE_DEPTH must be 1–256")
		}
		queueDepth = n
	}
	limits, err := providerLimits()
	if err != nil {
		return err
	}
	a.AgentPolicy = map[string]any{"max_concurrent_agents": concurrency, "queue_depth": queueDepth, "memory_bytes": limits.MemoryBytes, "pids": limits.PIDs, "cpu_percent": limits.CPUQuotaPercent, "wall_timeout_seconds": int(limits.Timeout.Seconds()), "output_bytes": limits.OutputBytes}
	gate := make(chan struct{}, concurrency)
	admission := make(chan struct{}, concurrency+queueDepth)
	a.ProviderRunner = func(selection AgentSelection) (agent.Runner, error) {
		return &personalProviderRunner{a: a, binary: binary, selection: selection, gate: gate, admission: admission}, nil
	}
	return nil
}

type personalProviderRunner struct {
	a         *App
	binary    string
	selection AgentSelection
	gate      chan struct{}
	admission chan struct{}
}

func (p *personalProviderRunner) Run(ctx context.Context, ex *agent.Execution, task agent.Task) error {
	select {
	case p.admission <- struct{}{}:
		defer func() { <-p.admission }()
	default:
		ex.Status = "failed"
		ex.FailureCode = "runner_queue_full"
		return fmt.Errorf("Agent queue is full")
	}
	queueStarted := time.Now()
	p.a.Metrics.Adjust("agent_queued", 1)
	select {
	case p.gate <- struct{}{}:
		p.a.Metrics.Adjust("agent_queued", -1)
		p.a.Metrics.Adjust("agent_running", 1)
		defer func() { <-p.gate; p.a.Metrics.Adjust("agent_running", -1) }()
	case <-ctx.Done():
		p.a.Metrics.Adjust("agent_queued", -1)
		ex.Status = "cancelled"
		ex.FailureCode = "runner_cancelled"
		if ctx.Err() == context.DeadlineExceeded {
			ex.Status = "timed_out"
			ex.FailureCode = "runner_wall_timeout"
		}
		return ctx.Err()
	}
	queueWait := int64(time.Since(queueStarted))
	dir, err := os.MkdirTemp("", "switchyard-provider-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	runner := agent.CLIRunner{Bin: p.binary, RuntimeBinary: os.Getenv("SWITCHYARD_OPENCODE_BIN"), AllowNetwork: true, Credential: func() (string, error) { return p.a.Secrets.ResolveForExecution(p.selection.CredentialID, dir) }}
	if runner.RuntimeBinary != "" {
		runner.Args = []string{"agent-adapter"}
		limits, err := providerLimits()
		if err != nil {
			return err
		}
		runner.Limits = limits
	}
	task.Provider, task.Model = p.selection.Provider, p.selection.Model
	if task.Provider == "" || task.Model == "" {
		return fmt.Errorf("provider and model required")
	}
	p.a.Metrics.Record("agent_starts", 0, 200)
	started := time.Now()
	err = runner.Run(ctx, ex, task)
	if ex.TimingsNS == nil {
		ex.TimingsNS = map[string]int64{}
	}
	ex.TimingsNS["queue_wait_ns"] = queueWait
	status := 200
	if err != nil {
		status = 500
	}
	p.a.Metrics.Record("agent_execution", time.Since(started), status)
	switch ex.FailureCode {
	case "provider_auth_failed", "provider_rate_limited", "provider_timeout", "provider_request_failed", "provider_invalid_response", "runner_wall_timeout", "runner_cancelled", "runner_memory_limit", "runner_pid_limit", "runner_exit_nonzero", "runner_invalid_output", "runner_start_failed":
		p.a.Metrics.Record("agent_"+ex.FailureCode, time.Since(started), 500)
	}
	ex.Adapter = task.Provider + "/" + task.Model
	return err
}
