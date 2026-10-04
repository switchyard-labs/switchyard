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
	if in.Scope != "" && in.Scope != "personal" {
		writeJSON(w, 400, map[string]any{"error": "organization_credential_delegation_not_configured"})
		return
	}
	in.Scope = "personal"
	id, err := a.Secrets.Create(in.Name, in.Provider, in.Scope, in.Secret)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if _, _, err := a.Trestle.CreateRecord("credential_owners", map[string]any{"credential_id": id, "username": user}, "credential-owner-"+id); err != nil {
		writeJSON(w, 502, map[string]any{"error": "credential_owner_persistence_failed"})
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
	out := meta[:0]
	for _, m := range meta {
		if a.ownsCredential(m.ID, a.currentUser(r)) {
			out = append(out, m)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) handleRotateCredential(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id := r.PathValue("id")
	if !a.ownsCredential(id, a.currentUser(r)) {
		writeJSON(w, 403, map[string]any{"error": "credential_owner_required"})
		return
	}
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
	if !a.ownsCredential(r.PathValue("id"), a.currentUser(r)) {
		writeJSON(w, 403, map[string]any{"error": "credential_owner_required"})
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
	metadata, err := a.Trestle.ListRecords("execution_metadata", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "execution_metadata_unavailable"})
		return
	}
	byID := map[string]any{}
	for _, record := range a.visibleRecords("execution_metadata", metadata, a.currentUser(r)) {
		byID[strOr(record["execution_id"])] = record["metadata"]
	}
	for _, item := range items {
		if meta, ok := byID[strOr(item["id"])]; ok {
			item["metadata"] = meta
		}
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) ownsCredential(id, user string) bool {
	if user == "" {
		return false
	}
	xs, err := a.Trestle.ListRecords("credential_owners", filterEq("credential_id", id))
	return err == nil && len(xs) == 1 && strOr(xs[0]["username"]) == user
}
