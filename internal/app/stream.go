package app

import (
	"encoding/json"
	"sync"
	"time"
)

// Hub fans out normalized events to authenticated browser SSE subscribers.
// The control plane is the coordination truth, so it is also the realtime
// source for browsers (no edge -> private-Trestle assumption needed).
type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: map[chan []byte]struct{}{}}
}

func (h *Hub) Publish(v any) {
	b, _ := json.Marshal(v)
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- b:
		default: // slow subscriber: drop this event, keep the connection
		}
	}
}

func (h *Hub) Subscribe() chan []byte {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// broadcast records a normalized event durably (idempotent) and pushes it to
// live subscribers.
func (a *App) broadcast(typ, repoName string, payload map[string]any) {
	id, replayed, err := a.Trestle.CreateRecord("events", map[string]any{
		"type": typ, "repo_name": repoName, "payload": payload,
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
	}, a.eventKey(typ, repoName, payload))
	if err == nil {
		a.Hub.Publish(map[string]any{"id": id, "type": typ, "repo": repoName, "payload": payload, "replayed": replayed})
	}
}

func (a *App) eventKey(typ, repo string, payload map[string]any) string {
	// deterministic identity from type + repo + a distinguishing payload field
	sha, _ := payload["after"].(string)
	if sha == "" {
		sha, _ = payload["new_sha"].(string)
	}
	ts, _ := payload["seen_at"].(string)
	return "evt-" + typ + "-" + repo + "-" + sha + "-" + ts
}