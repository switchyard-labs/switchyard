package app

import "context"

// launchWorker joins both polling loops and their durable effects to shutdown.
// The closing fence prevents a WaitGroup Add after draining starts.
func (a *App) launchWorker(run func()) bool {
	a.workerMu.Lock()
	defer a.workerMu.Unlock()
	if a.closing {
		return false
	}
	a.workers.Add(1)
	go func() { defer a.workers.Done(); run() }()
	return true
}

// DrainWorkers stops admitting new effects and waits for existing work to reach
// its durable boundary. Call after cancelling poll contexts; never clear claims
// just because a deadline expires: their lease/recovery protocol remains truth.
func (a *App) DrainWorkers(ctx context.Context) error {
	a.workerMu.Lock()
	a.closing = true
	a.workerMu.Unlock()
	done := make(chan struct{})
	go func() { a.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
