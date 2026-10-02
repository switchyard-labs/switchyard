package app

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

func newWorkID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "wk_" + hex.EncodeToString(b)
}

func (a *App) handleListWork(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("work", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, it := range items {
		out = append(out, it)
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) handleCreateWork(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		writeJSON(w, 400, map[string]any{"error": "title_required"})
		return
	}
	if in.Kind == "" {
		in.Kind = "feature"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := newWorkID()
	_, _, err := a.Trestle.CreateRecord("work", map[string]any{
		"id": id, "title": in.Title, "kind": in.Kind, "status": "open",
		"owner": user, "created_at": now, "updated_at": now,
	}, "work-"+id)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "title": in.Title, "kind": in.Kind, "status": "open", "owner": user})
}

func (a *App) handleGetWork(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	items, err := a.Trestle.ListRecords("work", `id = "`+id+`"`)
	if err != nil || len(items) == 0 {
		writeJSON(w, 404, map[string]any{"error": "work_not_found"})
		return
	}
	writeJSON(w, 200, items[0])
}
