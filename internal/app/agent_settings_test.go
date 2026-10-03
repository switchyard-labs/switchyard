package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"switchyard/internal/agent"
	"testing"
)

func agentSettingsFixture(t *testing.T) *App {
	records := map[string][]map[string]any{
		"agent_preferences": {{"username": "alice", "provider": "openai", "models": map[string]any{"openai": "implementation-model"}, "credentials": map[string]any{"openai": "alice-key"}, "roles": map[string]any{"reviewer": map[string]any{"provider": "anthropic", "model": "review-model", "credential_id": "review-key"}}}},
		"credential_owners": {{"username": "alice", "credential_id": "alice-key"}, {"username": "alice", "credential_id": "review-key"}, {"username": "bob", "credential_id": "bob-key"}},
	}
	a := securityFixture(t, records)
	a.Secrets = agent.NewCredentialStore(nil, func() ([]map[string]any, error) {
		return []map[string]any{{"id": "alice-key", "provider": "openai", "ciphertext": "must-never-return"}, {"id": "review-key", "provider": "anthropic"}, {"id": "bob-key", "provider": "openai"}}, nil
	}, nil, make([]byte, 32))
	return a
}
func TestAgentRolePreferencesAndOwnership(t *testing.T) {
	a := agentSettingsFixture(t)
	for role, want := range map[string]string{"implementer": "implementation-model", "reviewer": "review-model", "conflict-resolver": "implementation-model"} {
		s, err := a.agentSelection("alice", role)
		if err != nil || s.Model != want {
			t.Fatalf("%s: %+v %v", role, s, err)
		}
	}
	p := agentPreferences{Provider: "openai", Models: map[string]string{"openai": "model"}, Credentials: map[string]string{"openai": "bob-key"}}
	if err := a.validateAgentPreferences("alice", p); err == nil {
		t.Fatal("foreign credential accepted")
	}
	p.Credentials["openai"] = "review-key"
	if err := a.validateAgentPreferences("alice", p); err == nil {
		t.Fatal("provider mismatch accepted")
	}
	p.Credentials["openai"] = "alice-key"
	p.Models["openai"] = "model\ncommand"
	if err := a.validateAgentPreferences("alice", p); err == nil {
		t.Fatal("malformed model accepted")
	}
}
func TestAgentPreferencesAPIAndTaskOverride(t *testing.T) {
	a := agentSettingsFixture(t)
	r := httptest.NewRequest("GET", "/api/settings/agent", nil)
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w := httptest.NewRecorder()
	a.handleAgentSettings(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "ciphertext") || strings.Contains(w.Body.String(), "must-never-return") {
		t.Fatal(w.Code, w.Body.String())
	}
	for user, want := range map[string]int{"": 401, "alice": 200} {
		r := httptest.NewRequest("GET", "/api/settings/agent", nil)
		r = r.WithContext(contextWithUser(r.Context(), user))
		w := httptest.NewRecorder()
		a.handleAgentSettings(w, r)
		if w.Code != want {
			t.Fatal(user, w.Code)
		}
	}
	r.Header.Set("X-Switchyard-Agent", `{"provider":"openai","model":"one-task-model","credential_id":"alice-key"}`)
	ctx, err := a.agentRequestContext(r, "reviewer")
	if err != nil {
		t.Fatal(err)
	}
	if s := ctx.Value(agentOverrideKey{}).(AgentSelection); s.Model != "one-task-model" {
		t.Fatal(s)
	}
	saved, _ := a.agentSelection("alice", "reviewer")
	if saved.Model != "review-model" {
		t.Fatal("override changed saved default")
	}
	r.Header.Set("X-Switchyard-Agent", `{"provider":"openai","model":"model","credential_id":"bob-key"}`)
	if _, err = a.agentRequestContext(r, "reviewer"); err == nil {
		t.Fatal("foreign override accepted")
	}
	p := agentPreferences{Provider: "openai", Models: map[string]string{"openai": "new-model"}, Credentials: map[string]string{"openai": "alice-key"}, Roles: map[string]AgentSelection{"conflict-resolver": {Provider: "anthropic", Model: "resolver-model", CredentialID: "review-key"}}}
	body, _ := json.Marshal(p)
	r = httptest.NewRequest(http.MethodPut, "/api/settings/agent", strings.NewReader(string(body)))
	r = r.WithContext(contextWithUser(context.Background(), "alice"))
	w = httptest.NewRecorder()
	a.handleAgentSettings(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if s, err := a.agentSelection("alice", "conflict-resolver"); err != nil || s.Model != "resolver-model" {
		t.Fatal(s, err)
	}
}
