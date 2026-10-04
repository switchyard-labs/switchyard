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
	gate := make(chan struct{}, concurrency)
	a.ProviderRunner = func(selection AgentSelection) (agent.Runner, error) {
		return &personalProviderRunner{a: a, binary: binary, selection: selection, gate: gate}, nil
	}
	return nil
}

type personalProviderRunner struct {
	a         *App
	binary    string
	selection AgentSelection
	gate      chan struct{}
}

func (p *personalProviderRunner) Run(ctx context.Context, ex *agent.Execution, task agent.Task) error {
	p.a.Metrics.Adjust("agent_queued", 1)
	select {
	case p.gate <- struct{}{}:
		p.a.Metrics.Adjust("agent_queued", -1)
		p.a.Metrics.Adjust("agent_running", 1)
		defer func() { <-p.gate; p.a.Metrics.Adjust("agent_running", -1) }()
	case <-ctx.Done():
		p.a.Metrics.Adjust("agent_queued", -1)
		return ctx.Err()
	}
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
	status := 200
	if err != nil {
		status = 500
	}
	p.a.Metrics.Record("agent_execution", time.Since(started), status)
	switch ex.FailureCode {
	case "runner_timeout", "runner_cancelled", "runner_resource_memory", "runner_exit_nonzero", "runner_invalid_output", "runner_start_failed":
		p.a.Metrics.Record("agent_"+ex.FailureCode, time.Since(started), 500)
	}
	ex.Adapter = task.Provider + "/" + task.Model
	return err
}
