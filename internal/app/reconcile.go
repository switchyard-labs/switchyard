package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// StartReconciler observes Artifacts Git truth and reconciles it into Trestle
// coordination state. It detects external ref movement (human git pushes,
// branch create/delete, out-of-band changes) and emits durable normalized
// events that are also broadcast to live browsers. The reconciliation loop is
// the at-least-once + reconcilable safety net.
func (a *App) StartReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	log.Printf("reconciler started (interval %s)", interval)
	a.launchWorker(func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := a.reconcileOnceContext(ctx); err != nil {
					log.Printf("reconcile: %v", err)
				}
			}
		}
	})
}

func (a *App) reconcileOnce() error {
	return a.reconcileOnceContext(context.Background())
}

// Cancellation stops the next repository, after an already admitted repository
// has finished its durable observation/event boundary.
func (a *App) reconcileOnceContext(ctx context.Context) (resultErr error) {
	started := time.Now()
	defer func() {
		status := 200
		if resultErr != nil {
			status = 500
		}
		a.Metrics.Record("reconciliation", time.Since(started), status)
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	repos, err := a.Artifacts.ListRepos()
	if err != nil {
		return err
	}
	for _, repo := range repos {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := a.reconcileRepo(repo.Name); err != nil {
			log.Printf("reconcile %s: %v", repo.Name, err)
		}
	}
	return nil
}

func (a *App) reconcileRepo(repo string) error {
	refsMap, err := a.repoRefs(repo)
	if err != nil {
		return err
	}
	current := map[string]string{}
	for ref, sha := range refsMap {
		if ref != "HEAD" {
			current[shortRef(ref)] = sha
		}
	}
	// Retain known refs after deletion, including pre-cursor observations.
	for _, collection := range []string{"ref_current", "ref_obs"} {
		known, err := a.Trestle.ListRecords(collection, filterEq("repo", repo))
		if err != nil {
			return err
		}
		for _, row := range known {
			branch := strOr(row["branch"])
			if _, exists := current[branch]; !exists {
				current[branch] = ""
			}
		}
	}
	for branch, sha := range current {
		if err := a.reconcileRef(repo, branch, sha); err != nil {
			return err
		}
	}

	return nil
}

// The current cursor is separate from immutable, deduplicated event receipts.
// A->B->A->B must advance the cursor even when the A->B receipt already exists.
func (a *App) reconcileRef(repo, branch, sha string) error {
	id := fmt.Sprintf("ref-%x", sha256.Sum256([]byte(repo+"\x00"+branch)))
	rid, version, cursor, err := a.Trestle.FindRecord("ref_current", filterEq("id", id))
	if err != nil {
		return err
	}
	if cursor == nil {
		previous, err := a.latestRefObs(repo, branch)
		if err != nil {
			return err
		}
		baseline := previous
		if baseline == "" {
			baseline = sha
		}
		_, _, err = a.Trestle.CreateRecord("ref_current", map[string]any{"id": id, "repo": repo, "branch": branch, "sha": baseline, "seen_at": time.Now().UTC().Format(time.RFC3339Nano)}, "ref-current-"+id)
		rid, version, cursor, err = a.Trestle.FindRecord("ref_current", filterEq("id", id))
		if err != nil {
			return err
		}
		if cursor == nil {
			return fmt.Errorf("ref cursor unavailable")
		}
	}
	previous := strOr(cursor["sha"])
	if previous == sha {
		return nil
	}
	if _, err := a.observeTransition(repo, branch, previous, sha, "reconciliation"); err != nil {
		return err
	}
	return a.Trestle.PatchRecord("ref_current", rid, version, map[string]any{"sha": sha, "seen_at": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (a *App) latestRefObs(repo, branch string) (string, error) {
	items, err := a.Trestle.ListRecords("ref_obs", `repo = "`+repo+`"`)
	if err != nil {
		return "", err
	}
	// Trestle list order is not guaranteed to be newest-first; pick the max
	// seen_at for this branch.
	best := ""
	var bestTS string
	for _, it := range items {
		if it["branch"] != branch {
			continue
		}
		ts, _ := it["seen_at"].(string)
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		bestTime, _ := time.Parse(time.RFC3339Nano, bestTS)
		if err == nil && (bestTS == "" || parsed.After(bestTime)) {
			bestTS = ts
			best, _ = it["sha"].(string)
		}
	}
	return best, nil
}

func shortRef(ref string) string {
	const p = "refs/heads/"
	if len(ref) > len(p) && ref[:len(p)] == p {
		return ref[len(p):]
	}
	return ref
}

// handleEventStream is the browser realtime SSE endpoint (authenticated).
func (a *App) handleEventStream(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]any{"error": "no flush"})
		return
	}
	ch := a.Hub.Subscribe()
	defer a.Hub.Unsubscribe(ch)
	// initial ready frame
	w.Write([]byte("event: ready\ndata: {}\n\n"))
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			var event map[string]any
			if json.Unmarshal(b, &event) != nil || !a.recordAccess("events", event, a.currentUser(r), ReadRepo) {
				continue
			}
			w.Write([]byte("event: event\ndata: "))
			w.Write(b)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-heartbeat.C:
			w.Write([]byte("event: heartbeat\ndata: {}\n\n"))
			flusher.Flush()
		}
	}
}
