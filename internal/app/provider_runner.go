package app

import (
	"context"
	"fmt"
	"os"
	"switchyard/internal/agent"
)

// ConfigureProviderRunner accepts only an administrator-installed JSON adapter,
// never a command supplied by a user. Every execution has a fresh workspace
// and personal credential file. The shared gate bounds concurrent invocations.
func (a *App) ConfigureProviderRunner(binary string, concurrency int) {
	if concurrency < 1 || concurrency > 4 {
		concurrency = 2
	}
	gate := make(chan struct{}, concurrency)
	a.ProviderRunner = func(selection AgentSelection) (agent.Runner, error) {
		return &personalProviderRunner{a: a, binary: binary, selection: selection, gate: gate}, nil
	}
}

type personalProviderRunner struct {
	a         *App
	binary    string
	selection AgentSelection
	gate      chan struct{}
}

func (p *personalProviderRunner) Run(ctx context.Context, ex *agent.Execution, task agent.Task) error {
	select {
	case p.gate <- struct{}{}:
		defer func() { <-p.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	dir, err := os.MkdirTemp("", "switchyard-provider-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	runner := agent.CLIRunner{Bin: p.binary, AllowNetwork: true, Credential: func() (string, error) { return p.a.Secrets.ResolveForExecution(p.selection.CredentialID, dir) }}
	task.Provider, task.Model = p.selection.Provider, p.selection.Model
	if task.Provider == "" || task.Model == "" {
		return fmt.Errorf("provider and model required")
	}
	err = runner.Run(ctx, ex, task)
	ex.Adapter = task.Provider + "/" + task.Model
	return err
}
