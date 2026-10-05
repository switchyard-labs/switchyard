package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"switchyard/internal/maildelivery"
	"switchyard/internal/trestle"
	"sync"
	"testing"
	"time"
)

type captureMailer struct {
	mu       sync.Mutex
	messages []maildelivery.Message
	fail     bool
}

func (m *captureMailer) Send(_ context.Context, msg maildelivery.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	if m.fail {
		return errors.New("provider outage")
	}
	return nil
}
func recoveryFixture(t *testing.T) (*App, *captureMailer, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/admin/v1/session" {
			w.Write([]byte(`{"csrfToken":"test"}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/account-auth") {
			calls++
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			if in["action"] == "issue" {
				w.WriteHeader(201)
				json.NewEncoder(w).Encode(map[string]any{"token": strings.Repeat("a", 43), "expires_at": time.Now().Add(time.Hour)})
				return
			}
			if in["token"] == strings.Repeat("a", 43) {
				w.Write([]byte(`{"ok":true}`))
				return
			}
			w.WriteHeader(400)
			w.Write([]byte(`{}`))
			return
		}
		filter := r.URL.Query().Get("filter")
		items := []any{}
		if strings.Contains(filter, "known@example.test") || strings.Contains(filter, "alice") {
			items = append(items, map[string]any{"id": "rec_alice", "version": 1, "values": map[string]any{"email": "known@example.test", "username": "alice", "password_hash": "$2a$10$example"}})
		}
		if strings.Contains(filter, "disabled@example.test") {
			items = append(items, map[string]any{"id": "rec_disabled", "version": 1, "values": map[string]any{"email": "disabled@example.test", "disabled_at": "2026-10-01"}})
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
	}))
	t.Cleanup(srv.Close)
	mailer := &captureMailer{}
	return &App{Trestle: trestle.New(srv.URL, "admin", "secret"), Mailer: mailer}, mailer, &calls
}
func TestResetRequestEnumerationAndMailFailure(t *testing.T) {
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.example")
	a, mailer, calls := recoveryFixture(t)
	mailer.fail = true
	var expected string
	for _, email := range []string{"known@example.test", "missing@example.test", "disabled@example.test"} {
		r := httptest.NewRequest("POST", "/api/auth/password-reset/request", strings.NewReader(`{"email":"`+email+`"}`))
		w := httptest.NewRecorder()
		start := time.Now()
		a.handleResetRequest(w, r)
		if w.Code != 202 {
			t.Fatalf("status %d", w.Code)
		}
		if expected == "" {
			expected = w.Body.String()
		} else if w.Body.String() != expected {
			t.Fatal("enumerating response")
		}
		if time.Since(start) < 700*time.Millisecond {
			t.Fatal("missing minimum latency")
		}
	}
	if err := a.DrainWorkers(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 || len(mailer.messages) != 1 {
		t.Fatal("disabled/unknown issuance")
	}
	msg := mailer.messages[0]
	if !strings.Contains(msg.Text, "https://switchyard.example/reset-password#token=") || !strings.Contains(msg.HTML, "Reset your password") || strings.Contains(msg.Text, "$2a") {
		t.Fatal("unsafe email")
	}
}
func TestVerificationAndResetDelegation(t *testing.T) {
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.example")
	a, mailer, calls := recoveryFixture(t)
	r := httptest.NewRequest("POST", "/api/auth/resend-verification", strings.NewReader(`{}`))
	r = r.WithContext(contextWithUser(r.Context(), "alice"))
	w := httptest.NewRecorder()
	a.handleResendVerification(w, r)
	if w.Code != 202 || len(mailer.messages) != 1 {
		t.Fatal("resend failed", w.Code)
	}
	r = httptest.NewRequest("POST", "/api/auth/verify-email", strings.NewReader(`{"token":"rec_alice.`+strings.Repeat("a", 43)+`"}`))
	w = httptest.NewRecorder()
	a.handleVerifyEmail(w, r)
	if w.Code != 200 {
		t.Fatal("verification failed", w.Code)
	}
	before := *calls
	r = httptest.NewRequest("POST", "/api/auth/password-reset/complete", strings.NewReader(`{"token":"rec_alice.`+strings.Repeat("a", 43)+`","new_password":"new-password","confirm_password":"different"}`))
	w = httptest.NewRecorder()
	a.handleResetComplete(w, r)
	if w.Code != 400 || *calls != before {
		t.Fatal("confirmation delegated incorrectly")
	}
	r = httptest.NewRequest("POST", "/api/auth/password-reset/complete", strings.NewReader(`{"token":"rec_alice.`+strings.Repeat("a", 43)+`","new_password":"new-password","confirm_password":"new-password"}`))
	w = httptest.NewRecorder()
	a.handleResetComplete(w, r)
	if w.Code != 200 {
		t.Fatal("reset failed", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "switchyard_session" || cookies[0].MaxAge != -1 {
		t.Fatal("session cookie not cleared")
	}
}
func TestMailBudgetsAndConfiguration(t *testing.T) {
	a := &App{}
	r := httptest.NewRequest("POST", "/", nil)
	r.RemoteAddr = "192.0.2.1:123"
	for i := 0; i < 3; i++ {
		if !a.allowAccountMail(r, "alice@example.test") {
			t.Fatal("legitimate budget denied")
		}
	}
	if a.allowAccountMail(r, "alice@example.test") {
		t.Fatal("email budget exceeded")
	}
	t.Setenv("SWITCHYARD_RESET_TOKEN_SECONDS", "7201")
	if _, err := tokenLifetime("password_reset"); err == nil {
		t.Fatal("unsafe lifetime")
	}
}
func TestLoginCannotBindStalePasswordToNewSession(t *testing.T) {
	records := map[string][]map[string]any{"users": {{"username": "alice", "password_hash": "replacement"}}}
	a := securityFixture(t, records)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	if a.startSession(w, r, "alice", "original") == nil || len(records["sessions"]) != 0 || len(w.Result().Cookies()) != 0 {
		t.Fatal("stale authentication created session")
	}
}

func TestAccountTokenPreflight(t *testing.T) {
	a, _, calls := recoveryFixture(t)
	for _, purpose := range []string{"email_verification", "password_reset"} {
		r := httptest.NewRequest("POST", "/api/auth/account-token/validate?purpose="+purpose, strings.NewReader(`{"token":"rec_alice.`+strings.Repeat("a", 43)+`"}`))
		w := httptest.NewRecorder()
		a.handleAccountTokenValidation(w, r)
		if w.Code != 200 || len(w.Result().Cookies()) != 0 {
			t.Fatalf("preflight: status %d, cookies %v", w.Code, w.Result().Cookies())
		}
	}
	if *calls != 2 {
		t.Fatalf("expected two read-only validations, got %d", *calls)
	}
	for _, path := range []string{"?purpose=other", "?purpose=password_reset"} {
		w := httptest.NewRecorder()
		a.handleAccountTokenValidation(w, httptest.NewRequest("POST", "/api/auth/account-token/validate"+path, strings.NewReader(`{"token":"invalid"}`)))
		if w.Code != 400 {
			t.Fatalf("invalid preflight status %d", w.Code)
		}
	}
}
