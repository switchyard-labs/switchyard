package app

import (
	"context"
	"testing"
	"time"
)

func TestShutdownWaitsForDurableClaimBoundary(t *testing.T) {
	a, _ := fixtureEffect(t, queueEffect{QueueID: "queue", Phase: "queued"})
	entered, release, completed := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	if !a.launchWorker(func() {
		claim, err := a.claimQueue("queue")
		if err != nil || claim == nil {
			completed <- err
			close(entered)
			return
		}
		close(entered)
		<-release
		err = claim.save("done")
		if err == nil {
			err = claim.release()
		}
		completed <- err
	}) {
		t.Fatal("worker rejected before shutdown")
	}
	<-entered
	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := a.DrainWorkers(deadline); err == nil {
		t.Fatal("drain returned while claim still active")
	}
	if a.launchWorker(func() { t.Error("work admitted after shutdown fence") }) {
		t.Fatal("shutdown admitted new work")
	}
	close(release)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	if err := a.DrainWorkers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if claim, err := a.claimQueue("queue"); err != nil || claim != nil {
		t.Fatalf("completed effect was abandoned/reclaimed: %v", err)
	}
}
