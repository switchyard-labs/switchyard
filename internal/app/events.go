package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"switchyard/internal/artifacts"
	"time"
)

// handleIngestEvent accepts an Artifacts event envelope (either the Cloudflare
// `cf.artifacts.repo.pushed` shape or a minimal normalized shape) and routes it
// through the same idempotent ingest path as the queue consumer and
// reconciliation. It is a testing/operator hook that preserves one normalized
// ingest boundary.
func (a *App) handleIngestEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	var candidate struct {
		Source struct {
			Namespace string `json:"namespace"`
		} `json:"source"`
	}
	if json.Unmarshal(body, &candidate) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if candidate.Source.Namespace != "" {
		ev, err := artifacts.NormalizeEvent(body, a.Artifacts.Namespace)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid_artifacts_event"})
			return
		}
		replayed, err := a.ingestArtifactEvent(ev, "http")
		if err != nil {
			writeJSON(w, 503, map[string]any{"error": "event_ingest_unavailable"})
			return
		}
		writeJSON(w, 201, map[string]any{"id": ev.ID, "repo": ev.Repo, "type": ev.Type, "replayed": replayed, "occurred_at": ev.OccurredAt})
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var ev struct {
		Type   string `json:"type"`
		Source struct {
			RepoName string `json:"repoName"`
		} `json:"source"`
		RepoName string `json:"repo_name"`
		Payload  struct {
			Ref    string `json:"ref"`
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"payload"`
	}
	if err := readJSON(r, &ev); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	repo := ev.Source.RepoName
	if repo == "" {
		repo = ev.RepoName
	}
	if repo == "" || ev.Type == "" {
		writeJSON(w, 400, map[string]any{"error": "type_and_repo_required"})
		return
	}
	branch := shortRef(ev.Payload.Ref)
	if branch == "" {
		writeJSON(w, 400, map[string]any{"error": "ref_required"})
		return
	}
	replayed, err := a.observeTransition(repo, branch, ev.Payload.Before, ev.Payload.After, "http")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"repo": repo, "branch": branch, "replayed": replayed, "occurred_at": time.Now().UTC().Format(time.RFC3339)})
}

func (a *App) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("events", "")
	items = a.visibleRecords("events", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
