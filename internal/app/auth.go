package app

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const demoGuestKey ctxKey = 1

func contextWithUser(r context.Context, user string) context.Context {
	return context.WithValue(r, userKey, user)
}
func contextWithDemoGuest(r context.Context, v bool) context.Context {
	return context.WithValue(r, demoGuestKey, v)
}
func (a *App) isDemoGuest(r *http.Request) bool {
	v, _ := r.Context().Value(demoGuestKey).(bool)
	return v
}

func (a *App) currentUser(r *http.Request) string {
	if v := r.Context().Value(userKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func registrationOpen() bool {
	policy := strings.ToLower(strings.TrimSpace(os.Getenv("SWITCHYARD_REGISTRATION_POLICY")))
	return policy == "" || policy == "open"
}
func (a *App) handleRegistrationPolicy(w http.ResponseWriter, r *http.Request) {
	mode := "disabled"
	if registrationOpen() {
		mode = "open"
	}
	writeJSON(w, 200, map[string]any{"mode": mode, "email_verification": true})
}
func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !registrationOpen() {
		writeJSON(w, 403, map[string]any{"error": "registration_disabled"})
		return
	}
	if !a.allowAuthAttempt(r.RemoteAddr, "signup") {
		writeJSON(w, 429, map[string]any{"error": "registration_rate_limited"})
		return
	}
	var in struct {
		Username        string `json:"username"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	in.Username = normalizeOwnerSlug(in.Username)
	if len(in.Username) < 3 || len(in.Password) < 8 {
		writeJSON(w, 400, map[string]any{"error": "username_or_password_too_short"})
		return
	}
	if !validOwnerSlug(in.Username) {
		writeJSON(w, 400, map[string]any{"error": "username_invalid"})
		return
	}
	if len(in.Password) > 72 {
		writeJSON(w, 400, map[string]any{"error": "password_too_long"})
		return
	}
	if in.Password != in.ConfirmPassword {
		writeJSON(w, 400, map[string]any{"error": "password_confirmation_mismatch"})
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	address, emailErr := mail.ParseAddress(in.Email)
	if emailErr != nil || address.Address != in.Email || len(in.Email) > 254 || strings.Count(in.Email, "@") != 1 || !strings.Contains(strings.SplitN(in.Email, "@", 2)[1], ".") {
		writeJSON(w, 400, map[string]any{"error": "email_invalid"})
		return
	}
	ns, err := a.Trestle.ListRecords("owner_namespaces", filterEq("slug", in.Username))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
		return
	}
	if len(ns) > 0 {
		writeJSON(w, 409, map[string]any{"error": "username_taken"})
		return
	}
	existing, err := a.Trestle.ListRecords("users", `username = "`+in.Username+`"`)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
		return
	}
	if len(existing) > 0 {
		writeJSON(w, 409, map[string]any{"error": "username_taken"})
		return
	}
	emails, err := a.Trestle.ListRecords("account_emails", filterEq("email", in.Email))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
		return
	}
	if len(emails) > 0 {
		writeJSON(w, 409, map[string]any{"error": "email_taken"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "internal"})
		return
	}
	// Unique email reservations enforce case-insensitive ownership across processes.
	emailID, _, err := a.Trestle.CreateRecord("account_emails", map[string]any{"email": in.Email, "username": in.Username, "created_at": nowStr()}, "signup-email-"+newToken())
	if err != nil {
		matches, lookupErr := a.Trestle.ListRecords("account_emails", filterEq("email", in.Email))
		if lookupErr == nil && len(matches) > 0 {
			writeJSON(w, 409, map[string]any{"error": "email_taken"})
		} else {
			writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
		}
		return
	}
	userID, _, err := a.Trestle.CreateRecord("users", map[string]any{"username": in.Username, "email": in.Email, "password_hash": string(hash), "display_name": in.Username}, "signup-user-"+newToken())
	if err != nil {
		if a.removeSignupRecord("account_emails", emailID) != nil {
			writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
			return
		}
		matches, lookupErr := a.Trestle.ListRecords("users", filterEq("username", in.Username))
		if lookupErr == nil && len(matches) > 0 {
			writeJSON(w, 409, map[string]any{"error": "username_taken"})
		} else {
			writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
		}
		return
	}
	if err := a.ensureOwnerNamespace(in.Username, "user", in.Username); err != nil {
		if a.removeSignupRecord("users", userID) != nil || a.removeSignupRecord("account_emails", emailID) != nil {
			writeJSON(w, 502, map[string]any{"error": "registration_unavailable"})
			return
		}
		writeJSON(w, 409, map[string]any{"error": "owner_namespace_conflict"})
		return
	}
	if err := a.startSession(w, r, in.Username, string(hash)); err != nil {
		writeJSON(w, 502, map[string]any{"error": "session_persistence_failed"})
		return
	}
	delivery := "sent"
	if !a.allowAccountMail(r, in.Email) || a.sendAccountToken(r.Context(), userID, in.Email, "email_verification") != nil {
		delivery = "unavailable"
	}
	writeJSON(w, 201, map[string]any{"ok": true, "user": in.Username, "email_verification": delivery})
}

// Cleanup is restricted to record IDs created by this signup request.
func (a *App) removeSignupRecord(collection, id string) error {
	if id == "" {
		return nil
	}
	return a.Trestle.DeleteRecord(collection, id, "1")
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	in.Username = normalizeOwnerSlug(in.Username)
	if !validOwnerSlug(in.Username) {
		writeJSON(w, 401, map[string]any{"error": "invalid_credentials"})
		return
	}
	if !a.allowLogin(r.RemoteAddr) {
		writeJSON(w, 429, map[string]any{"error": "login_rate_limited"})
		return
	}
	items, err := a.Trestle.ListRecords("users", filterEq("username", in.Username))
	if err != nil || len(items) == 0 || strOr(items[0]["disabled_at"]) != "" {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOhi8nVZzQEjU1C5bG1fT3UJwWzM6bLfW"), []byte(in.Password))
		writeJSON(w, 401, map[string]any{"error": "invalid_credentials"})
		return
	}
	hash, _ := items[0]["password_hash"].(string)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeJSON(w, 401, map[string]any{"error": "invalid_credentials"})
		return
	}
	if err := a.startSession(w, r, in.Username, hash); err != nil {
		writeJSON(w, 502, map[string]any{"error": "session_persistence_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "user": in.Username})
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, username string, authenticatedHash ...string) error {
	record := a.userRecord(username)
	if record == nil || strOr(record["disabled_at"]) != "" {
		return fmt.Errorf("user not found")
	}
	hash := strOr(record["password_hash"])
	// A reset between password validation and session creation must never bind
	// an old-password login to the replacement password's session generation.
	if len(authenticatedHash) > 0 && authenticatedHash[0] != hash {
		return fmt.Errorf("account changed during authentication")
	}
	token := newToken() + "_" + sha256Hex([]byte(hash))[:16]
	_, _, err := a.Trestle.CreateRecord("sessions", map[string]any{
		"token":      token,
		"username":   username,
		"expires_at": time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}, "sess-"+token)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName(r), Value: token, Path: "/",
		HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600,
	})
	return nil
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName(r)); err == nil {
		rid, ver, _, e := a.Trestle.FindRecord("sessions", filterEq("token", c.Value))
		if e != nil {
			writeJSON(w, 502, map[string]any{"error": "logout_persistence_failed"})
			return
		}
		if rid != "" {
			if e = a.Trestle.DeleteRecord("sessions", rid, ver); e != nil {
				writeJSON(w, 502, map[string]any{"error": "logout_persistence_failed"})
				return
			}
		}
	}

	http.SetCookie(w, &http.Cookie{Name: sessionCookieName(r), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"authed": false})
		return
	}
	writeJSON(w, 200, map[string]any{"authed": true, "user": user})
}

func (a *App) handleDemoStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"enabled": a.DemoMode, "guest": a.isDemoGuest(r), "mode": func() string {
		if a.DemoMode {
			return "public-read-only"
		}
		return "off"
	}()})
}
