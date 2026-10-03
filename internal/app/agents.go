package app

import (
	"net/http"
)

// handleCreateCredential stores a provider credential (encrypted at rest).
// The secret is never returned.
func (a *App) handleCreateCredential(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Name     string `json:"name"`
		Provider string `json:"provider"`
		Secret   string `json:"secret"`
		Scope    string `json:"scope"`
	}
	if err := readJSON(r, &in); err != nil || in.Name == "" || in.Provider == "" || in.Secret == "" {
		writeJSON(w, 400, map[string]any{"error": "name/provider/secret required"})
		return
	}
	if in.Scope == "" {
		in.Scope = "personal"
	}
	id, err := a.Secrets.Create(in.Name, in.Provider, in.Scope, in.Secret)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "provider": in.Provider, "scope": in.Scope, "notice": "credential stored securely; the secret cannot be viewed again"})
}

func (a *App) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	meta, err := a.Secrets.Metadata()
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": meta})
}

func (a *App) handleRotateCredential(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	var in struct {
		Secret string `json:"secret"`
	}
	if err := readJSON(r, &in); err != nil || in.Secret == "" {
		writeJSON(w, 400, map[string]any{"error": "secret required"})
		return
	}
	if err := a.Secrets.Rotate(id, in.Secret); err != nil {
		writeJSON(w, 404, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) handleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	if err := a.Secrets.Delete(r.PathValue("id")); err != nil {
		writeJSON(w, 404, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) handleListRoles(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	writeJSON(w, 200, map[string]any{"items": a.Roles})
}

func (a *App) handleListExecutions(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("executions", "")
	items = a.visibleRecords("executions", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}
