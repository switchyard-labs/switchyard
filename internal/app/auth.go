package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func contextWithUser(r context.Context, user string) context.Context {
	return context.WithValue(r, userKey, user)
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
	in.Username = strings.TrimSpace(in.Username)
	if len(in.Username) < 3 || len(in.Password) < 8 {
		writeJSON(w, 400, map[string]any{"error": "username_or_password_too_short"})
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
	a.startSession(w, in.Username)
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
	items, err := a.Trestle.ListRecords("users", `username = "`+strings.TrimSpace(in.Username)+`"`)
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
	a.startSession(w, in.Username)
	writeJSON(w, 200, map[string]any{"ok": true, "user": in.Username})
}

func (a *App) startSession(w http.ResponseWriter, username string) {
	token := newToken()
	_, _, _ = a.Trestle.CreateRecord("sessions", map[string]any{
		"token":      token,
		"username":   username,
		"expires_at": time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339),
	}, "sess-"+token)
	http.SetCookie(w, &http.Cookie{
		Name: "switchyard_session", Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 7 * 24 * 3600,
	})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "switchyard_session", Value: "", Path: "/", MaxAge: -1})
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