package app

import (
	"context"
	"fmt"
	"net/http"
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

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
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
	if ns, _ := a.Trestle.ListRecords("owner_namespaces", `slug = "`+normalizeOwnerSlug(in.Username)+`"`); len(ns) > 0 {
		writeJSON(w, 409, map[string]any{"error": "username_taken"})
		return
	}
	existing, err := a.Trestle.ListRecords("users", `username = "`+in.Username+`"`)
	if err == nil && len(existing) > 0 {
		writeJSON(w, 409, map[string]any{"error": "username_taken"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "internal"})
		return
	}
	display := in.DisplayName
	if display == "" {
		display = in.Username
	}
	_, _, err = a.Trestle.CreateRecord("users", map[string]any{
		"username":      in.Username,
		"password_hash": string(hash),
		"display_name":  display,
	}, "user-"+in.Username)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "create_user_failed"})
		return
	}
	if err := a.ensureOwnerNamespace(in.Username, "user", in.Username); err != nil {
		writeJSON(w, 409, map[string]any{"error": "owner_namespace_conflict"})
		return
	}
	if err := a.startSession(w, r, in.Username); err != nil {
		writeJSON(w, 502, map[string]any{"error": "session_persistence_failed"})
		return
	}
	writeJSON(w, 201, map[string]any{"ok": true, "user": in.Username})
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
	if err != nil || len(items) == 0 {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOhi8nVZzQEjU1C5bG1fT3UJwWzM6bLfW"), []byte(in.Password))
		writeJSON(w, 401, map[string]any{"error": "invalid_credentials"})
		return
	}
	hash, _ := items[0]["password_hash"].(string)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		writeJSON(w, 401, map[string]any{"error": "invalid_credentials"})
		return
	}
	if err := a.startSession(w, r, in.Username); err != nil {
		writeJSON(w, 502, map[string]any{"error": "session_persistence_failed"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "user": in.Username})
}

func (a *App) startSession(w http.ResponseWriter, r *http.Request, username string) error {
	record := a.userRecord(username)
	if record == nil {
		return fmt.Errorf("user not found")
	}
	token := newToken() + "_" + sha256Hex([]byte(strOr(record["password_hash"])))[:16]
	_, _, err := a.Trestle.CreateRecord("sessions", map[string]any{
		"token":      token,
		"username":   username,
		"expires_at": time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}, "sess-"+token)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: "switchyard_session", Value: token, Path: "/",
		HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600,
	})
	return nil
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("switchyard_session"); err == nil {
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

	http.SetCookie(w, &http.Cookie{Name: "switchyard_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
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
