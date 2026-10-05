package app

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http/httptest"
	"strings"
	"testing"
)

func signupRequest(a *App, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(body))
	a.handleRegister(w, r)
	return w
}
func TestSignupValidationAndPolicy(t *testing.T) {
	t.Setenv("SWITCHYARD_REGISTRATION_POLICY", "open")
	for _, tc := range []struct{ body, error string }{
		{`{"username":"new-user","email":"invalid","password":"password123","confirm_password":"password123"}`, "email_invalid"},
		{`{"username":"new-user","email":"a@example.com","password":"password123","confirm_password":"different"}`, "password_confirmation_mismatch"},
		{`{"username":"new-user","email":"a@example.com","password":"short","confirm_password":"short"}`, "username_or_password_too_short"},
		{`{"username":"www","email":"a@example.com","password":"password123","confirm_password":"password123"}`, "username_invalid"},
	} {
		w := signupRequest(&App{}, tc.body)
		if w.Code != 400 || !strings.Contains(w.Body.String(), tc.error) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	t.Setenv("SWITCHYARD_REGISTRATION_POLICY", "disabled")
	w := signupRequest(&App{}, `{}`)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	t.Setenv("SWITCHYARD_REGISTRATION_POLICY", "unknown")
	if registrationOpen() {
		t.Fatal("unknown policy must fail closed")
	}
}
func TestSignupPersistenceSessionLoginAndDuplicates(t *testing.T) {
	t.Setenv("SWITCHYARD_REGISTRATION_POLICY", "open")
	records := map[string][]map[string]any{}
	a := securityFixture(t, records)
	body := `{"username":"New-User","email":"NEW@example.com","password":"password123","confirm_password":"password123","display_name":"ignored"}`
	w := signupRequest(a, body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	user := records["users"][0]
	if user["username"] != "new-user" || user["email"] != "new@example.com" || user["display_name"] != "new-user" {
		t.Fatal("normalized fields/default name")
	}
	if bcrypt.CompareHashAndPassword([]byte(user["password_hash"].(string)), []byte("password123")) != nil {
		t.Fatal("password hash")
	}
	if len(records["sessions"]) != 1 || len(records["account_emails"]) != 1 || len(records["owner_namespaces"]) != 1 || len(w.Result().Cookies()) != 1 {
		t.Fatal("account/session persistence")
	}
	w = signupRequest(a, body)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "username_taken") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = signupRequest(a, strings.Replace(body, "New-User", "different-user", 1))
	if w.Code != 409 || !strings.Contains(w.Body.String(), "email_taken") {
		t.Fatal(w.Code, w.Body.String())
	}
	login := httptest.NewRecorder()
	a.handleLogin(login, httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"new-user","password":"password123"}`)))
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	public := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/users/new-user", nil)
	r.SetPathValue("username", "new-user")
	a.handleGetUserProfile(public, r)
	var profile map[string]any
	json.Unmarshal(public.Body.Bytes(), &profile)
	if _, exists := profile["email"]; exists {
		t.Fatal("private email leaked")
	}
}
