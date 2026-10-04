package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Preferences contain only model names and references to this user's encrypted
// credentials. The catalog mirrors Cortex/Warden; model IDs are user editable.
var agentProviders = []map[string]string{
	{"id": "opencode", "label": "OpenCode Zen"},
	{"id": "opencode-go", "label": "OpenCode Go"},
	{"id": "openrouter", "label": "OpenRouter"},
	{"id": "openai", "label": "OpenAI API"},
	{"id": "anthropic", "label": "Anthropic API"},
	{"id": "google", "label": "Google AI"},
	{"id": "deepseek", "label": "DeepSeek API"},
}

type AgentSelection struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	CredentialID string `json:"credential_id"`
}
type agentPreferences struct {
	Roles       map[string]AgentSelection `json:"roles"`
	Provider    string                    `json:"provider"`
	Models      map[string]string         `json:"models"`
	Credentials map[string]string         `json:"credentials"`
}
type agentUserKey struct{}

func agentUserContext(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, agentUserKey{}, user)
}
func knownAgentProvider(id string) bool {
	for _, p := range agentProviders {
		if p["id"] == id {
			return true
		}
	}
	return false
}
func preferenceMap(v any) map[string]string {
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		for k, v := range m {
			out[k] = strOr(v)
		}
	}
	return out
}
func (a *App) agentPreferences(user string) (agentPreferences, error) {
	xs, err := a.Trestle.ListRecords("agent_preferences", filterEq("username", user))
	p := agentPreferences{Models: map[string]string{}, Credentials: map[string]string{}}
	if err != nil {
		return p, err
	}
	if len(xs) > 0 {
		p.Provider = strOr(xs[0]["provider"])
		p.Models = preferenceMap(xs[0]["models"])
		p.Credentials = preferenceMap(xs[0]["credentials"])
		if raw, err := json.Marshal(xs[0]["roles"]); err == nil {
			_ = json.Unmarshal(raw, &p.Roles)
		}
	}
	return p, nil
}
func (a *App) validateAgentPreferences(user string, p agentPreferences) error {
	if p.Provider != "" && !knownAgentProvider(p.Provider) {
		return fmt.Errorf("unknown provider")
	}
	for provider, model := range p.Models {
		if !knownAgentProvider(provider) || len(model) > 200 || strings.ContainsAny(model, "\x00\r\n\t ") {
			return fmt.Errorf("invalid provider or model")
		}
	}
	for provider, id := range p.Credentials {
		if !knownAgentProvider(provider) {
			return fmt.Errorf("unknown credential provider")
		}
		if id == "" {
			continue
		}
		if !a.ownsCredential(id, user) {
			return fmt.Errorf("credential_owner_required")
		}
		meta, err := a.Secrets.Metadata()
		if err != nil {
			return fmt.Errorf("credential unavailable")
		}
		matched := false
		for _, m := range meta {
			if m.ID == id && m.Provider == provider {
				matched = true
			}
		}
		if !matched {
			return fmt.Errorf("credential_provider_mismatch")
		}
	}
	for role, selection := range p.Roles {
		if role != "implementer" && role != "reviewer" && role != "conflict-resolver" && role != "proposer" {
			return fmt.Errorf("unknown agent role")
		}
		if selection.Provider == "" {
			continue
		}
		one := agentPreferences{Provider: selection.Provider, Models: map[string]string{selection.Provider: selection.Model}, Credentials: map[string]string{selection.Provider: selection.CredentialID}}
		if err := a.validateAgentPreferences(user, one); err != nil {
			return err
		}
	}
	if p.Provider != "" && (p.Models[p.Provider] == "" || p.Credentials[p.Provider] == "") {
		return fmt.Errorf("selected provider requires a model and personal credential")
	}
	return nil
}
func (a *App) agentSelection(user, role string) (AgentSelection, error) {
	p, err := a.agentPreferences(user)
	if err != nil {
		return AgentSelection{}, err
	}
	if err = a.validateAgentPreferences(user, p); err != nil {
		return AgentSelection{}, err
	}
	if s, ok := p.Roles[role]; ok && s.Provider != "" {
		return s, nil
	}
	return AgentSelection{p.Provider, p.Models[p.Provider], p.Credentials[p.Provider]}, nil
}
func (a *App) handleAgentSettings(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	if r.Method == http.MethodPut {
		var p agentPreferences
		if err := readJSON(r, &p); err != nil {
			writeJSON(w, 400, map[string]any{"error": "invalid preferences"})
			return
		}
		if err := a.validateAgentPreferences(user, p); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		id, version, _, err := a.Trestle.FindRecord("agent_preferences", filterEq("username", user))
		values := map[string]any{"username": user, "provider": p.Provider, "models": p.Models, "credentials": p.Credentials, "roles": p.Roles}
		if err == nil {
			if id == "" {
				_, _, err = a.Trestle.CreateRecord("agent_preferences", values, "agent-preferences-"+user)
			} else {
				err = a.Trestle.PatchRecord("agent_preferences", id, version, values)
			}
		}
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "preferences persistence failed"})
			return
		}
	}
	p, err := a.agentPreferences(user)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "preferences unavailable"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"preferences": p, "providers": agentProviders, "runner_available": a.ProviderRunner != nil})
}

type agentOverrideKey struct{}

// A task override contains identifiers only. It is validated against the
// authenticated owner's credentials before any execution starts.
func (a *App) agentRequestContext(r *http.Request, role string) (context.Context, error) {
	ctx := agentUserContext(r.Context(), a.currentUser(r))
	raw := r.Header.Get("X-Switchyard-Agent")
	if raw == "" {
		return ctx, nil
	}
	if len(raw) > 1024 {
		return nil, fmt.Errorf("agent override too large")
	}
	var selection AgentSelection
	if err := json.Unmarshal([]byte(raw), &selection); err != nil {
		return nil, fmt.Errorf("invalid agent override")
	}
	if selection.Provider == "" {
		return nil, fmt.Errorf("provider required")
	}
	p := agentPreferences{Roles: map[string]AgentSelection{role: selection}}
	if err := a.validateAgentPreferences(a.currentUser(r), p); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, agentOverrideKey{}, selection), nil
}
