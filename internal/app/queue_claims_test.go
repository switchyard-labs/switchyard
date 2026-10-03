package app

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func fixtureEffect(t *testing.T, effect queueEffect) (*App, *draftFixture) {
	t.Helper()
	a, f := newDraftFixture(t)
	encoded, err := json.Marshal(effect)
	if err != nil {
		t.Fatal(err)
	}
	var state any
	if err = json.Unmarshal(encoded, &state); err != nil {
		t.Fatal(err)
	}
	f.values = map[string]any{"queue_id": effect.QueueID, "state": state}
	return a, f
}

func TestQueueClaimSingleOwnerAndExpiry(t *testing.T) {
	a, f := fixtureEffect(t, queueEffect{QueueID: "queue", Phase: "queued"})
	var wg sync.WaitGroup
	claims := make(chan *queueClaim, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claim, _ := a.claimQueue("queue")
			if claim != nil {
				claims <- claim
			}
		}()
	}
	wg.Wait()
	close(claims)
	var owner *queueClaim
	count := 0
	for claim := range claims {
		owner = claim
		count++
	}
	if count != 1 {
		t.Fatalf("expected one owner, got %d", count)
	}
	if err := owner.save("validating"); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	current, err := decodeQueueEffect(f.values)
	if err != nil {
		t.Fatal(err)
	}
	current.LeaseUntil = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	b, _ := json.Marshal(current)
	var state any
	json.Unmarshal(b, &state)
	f.values["state"] = state
	f.version++
	f.mu.Unlock()
	replacement, err := a.claimQueue("queue")
	if err != nil || replacement == nil {
		t.Fatalf("expired claim not reclaimed: %v", err)
	}
	if replacement.effect.Owner == owner.effect.Owner {
		t.Fatal("reused expired owner")
	}
	if err := owner.save("publishing"); err == nil {
		t.Fatal("expired owner advanced state")
	}
	if err := owner.release(); err == nil {
		t.Fatal("expired owner released new claim")
	}
	if err := replacement.save("publishing"); err != nil {
		t.Fatal(err)
	}
	if err := replacement.save("reconciling"); err != nil {
		t.Fatal(err)
	}
	if err := replacement.save("done"); err != nil {
		t.Fatal(err)
	}
	if err := replacement.release(); err != nil {
		t.Fatal(err)
	}
	if claim, err := a.claimQueue("queue"); err != nil || claim != nil {
		t.Fatalf("completed effect reclaimed: %v", err)
	}
}

func TestQueueClaimPersistenceFailureDoesNotGrantOwnership(t *testing.T) {
	a, f := fixtureEffect(t, queueEffect{QueueID: "queue", Phase: "queued"})
	f.patchStatus = 503
	if claim, err := a.claimQueue("queue"); err == nil || claim != nil {
		t.Fatal("failed claim write granted ownership")
	}
}
