package app

import (
	"context"
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
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := a.reconcileOnce(); err != nil {
					log.Printf("reconcile: %v", err)
				}
			}
		}
	}()
}

func (a *App) reconcileOnce() error {
	repos, err := a.Artifacts.ListRepos()
	if err != nil {
		return err
	}
	for _, repo := range repos {
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
	for ref, sha := range refsMap {
		if ref != "HEAD" {
			branch := shortRef(ref)
			a.reconcileRef(repo, branch, sha)
		}
	}
	return nil
}

func (a *App) reconcileRef(repo, branch, sha string) {
	// latest recorded observation for this branch
	latest, _ := a.latestRefObs(repo, branch)
	if latest == sha {
		return
	}
	// First observation of this branch: record a baseline observation only,
	// with no domain event. Reconciliation cannot distinguish "branch just
	// created then pushed" from "already existed", so a fabricated before=""
	// transition would collide with the queue event's authoritative before and
	// create a second domain fact. The queue event (if any) carries the true
	// `before`; the baseline here makes later transitions (before -> after)
	// well-defined.
	if latest == "" {
		now := time.Now().UTC().Format(time.RFC3339)
		_, _, _ = a.Trestle.CreateRecord("ref_obs", map[string]any{
			"repo": repo, "branch": branch, "sha": sha, "seen_at": now,
		}, "refobs-"+repo+"-"+branch+"-"+sha+"-baseline-"+now)
		return
	}
	// shared normalized ingest (also used by the queue fast path); domain
	// event is deduplicated by ref-transition identity (repo, ref, before,
	// after), so reconciliation never duplicates what the queue already
	// recorded, and vice versa.
	if _, err := a.observeTransition(repo, branch, latest, sha, "reconciliation"); err != nil {
		log.Printf("reconcile %s/%s: %v", repo, branch, err)
	}
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
		if ts > bestTS {
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