package app

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// handleIngestEvent accepts a normalized Artifacts event and writes it to the
// coordination truth with an Idempotency-Key (derived from the event identity),
// so at-least-once redelivery is deduplicated at the Trestle boundary.
func (a *App) handleIngestEvent(w http.ResponseWriter, r *http.Request) {
	var ev struct {
		Type      string         `json:"type"`
		RepoName  string         `json:"repo_name"`
		Namespace string         `json:"namespace"`
		Payload   map[string]any `json:"payload"`
		Occurred  string         `json:"occurred_at"`
	}
	if err := readJSON(r, &ev); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if ev.Type == "" {
		writeJSON(w, 400, map[string]any{"error": "type_required"})
		return
	}
	if ev.Occurred == "" {
		ev.Occurred = time.Now().UTC().Format(time.RFC3339)
	}
	// idempotency key from event identity (type + repo + occurrence)
	h := sha256.Sum256([]byte(ev.Type + "|" + ev.RepoName + "|" + ev.Occurred))
	key := "evt-" + hex.EncodeToString(h[:16])
	id, replayed, err := a.Trestle.CreateRecord("events", map[string]any{
		"type": ev.Type, "repo_name": ev.RepoName, "payload": ev.Payload, "occurred_at": ev.Occurred,
	}, key)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "replayed": replayed})
}

func (a *App) handleListEvents(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("events", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}