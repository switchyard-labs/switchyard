package app

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"switchyard/internal/maildelivery"
	"switchyard/internal/trestle"
	"time"
)

const resetRequestMessage = "If an account exists for that email, a reset link has been sent."

func tokenLifetime(purpose string) (int, error) {
	key, value, max := "SWITCHYARD_VERIFY_TOKEN_SECONDS", 86400, 172800
	if purpose == "password_reset" {
		key, value, max = "SWITCHYARD_RESET_TOKEN_SECONDS", 1800, 7200
	}
	if raw := os.Getenv(key); raw != "" {
		v, e := strconv.Atoi(raw)
		if e != nil || v < 60 || v > max {
			return 0, fmt.Errorf("invalid token lifetime")
		}
		value = v
	}
	return value, nil
}
func (a *App) accountMailer() maildelivery.Mailer {
	if a.Mailer != nil {
		return a.Mailer
	}
	return maildelivery.SMTP{TLSMode: os.Getenv("SWITCHYARD_SMTP_TLS_MODE"), Address: os.Getenv("SWITCHYARD_SMTP_ADDRESS"), Username: os.Getenv("SWITCHYARD_SMTP_USERNAME"), Password: os.Getenv("SWITCHYARD_SMTP_PASSWORD"), From: os.Getenv("SWITCHYARD_MAIL_FROM")}
}
func (a *App) allowAccountMail(r *http.Request, email string) bool {
	host, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil {
		host = r.RemoteAddr
	}
	a.authMu.Lock()
	defer a.authMu.Unlock()
	now := time.Now()
	if a.loginAttempts == nil {
		a.loginAttempts = map[string]loginWindow{}
	}
	for k, v := range a.loginAttempts {
		if !now.Before(v.Until) {
			delete(a.loginAttempts, k)
		}
	}
	if len(a.loginAttempts) > 9996 {
		return false
	}
	keys := []struct {
		key string
		max int
	}{{"mail:ip:" + host, 10}, {"mail:email:" + sha256Hex([]byte(email)), 3}, {"mail:global", 100}}
	allowed := true
	for _, item := range keys {
		v := a.loginAttempts[item.key]
		if !now.Before(v.Until) {
			v = loginWindow{Until: now.Add(5 * time.Minute)}
		}
		v.Count++
		a.loginAttempts[item.key] = v
		if v.Count > item.max {
			allowed = false
		}
	}
	return allowed
}
func (a *App) sendAccountToken(ctx context.Context, id, email, purpose string) (result error) {
	started := time.Now()
	defer func() {
		status := 200
		if result != nil {
			status = 503
		}
		a.Metrics.Record("auth_mail_delivery", time.Since(started), status)
	}()
	lifetime, err := tokenLifetime(purpose)
	if err != nil {
		return err
	}
	base, err := url.Parse(os.Getenv("SWITCHYARD_PUBLIC_URL"))
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return maildelivery.ErrUnavailable
	}
	issued, err := a.Trestle.AccountAuth("users", id, map[string]any{"action": "issue", "purpose": purpose, "lifetime_seconds": lifetime})
	if err != nil {
		return err
	}
	if issued.Token == "" {
		return maildelivery.ErrUnavailable
	}
	path, subject, action := "/verify-email", "Verify your Switchyard email", "Verify your email"
	if purpose == "password_reset" {
		path, subject, action = "/reset-password", "Reset your Switchyard password", "Reset your password"
	}
	// Fragment links keep the secret out of HTTP targets, proxy logs and referrers.
	base.Path = path
	base.Fragment = "token=" + url.QueryEscape(id+"."+issued.Token)
	link := base.String()
	expiry := issued.ExpiresAt.UTC().Format(time.RFC1123)
	text := action + " by opening this link:\n\n" + link + "\n\nThis link expires at " + expiry + " and can be used once. If you did not request this, you can ignore this message."
	markup := "<p>" + html.EscapeString(action) + ".</p><p><a href=\"" + html.EscapeString(link) + "\">" + html.EscapeString(action) + "</a></p><p>This link expires at " + html.EscapeString(expiry) + " and can be used once.</p><p>If you did not request this, you can ignore this message.</p>"
	return a.accountMailer().Send(ctx, maildelivery.Message{To: email, Subject: subject, Text: text, HTML: markup})
}
func (a *App) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id, _, rec, err := a.Trestle.FindRecord("users", filterEq("username", user))
	if err != nil || id == "" {
		writeJSON(w, 503, map[string]any{"error": "verification_unavailable"})
		return
	}
	if strOr(rec["verified_at"]) != "" {
		writeJSON(w, 200, map[string]any{"ok": true, "verified": true})
		return
	}
	email := strOr(rec["email"])
	if email == "" {
		writeJSON(w, 400, map[string]any{"error": "account_email_missing"})
		return
	}
	if !a.allowAccountMail(r, email) {
		writeJSON(w, 429, map[string]any{"error": "rate_limited"})
		return
	}
	if a.sendAccountToken(r.Context(), id, email, "email_verification") != nil {
		writeJSON(w, 503, map[string]any{"error": "mail_delivery_failed"})
		return
	}
	writeJSON(w, 202, map[string]any{"ok": true})
}
func (a *App) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	// Enforce the same response floor for every branch. Delivery runs outside
	// the request so provider latency cannot reveal whether an account exists.
	deadline := time.Now().Add(750 * time.Millisecond)
	defer func() {
		if d := time.Until(deadline); d > 0 {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-r.Context().Done():
			}
		}
		writeJSON(w, 202, map[string]any{"message": resetRequestMessage})
	}()
	var in struct {
		Email string `json:"email"`
	}
	if readJSON(r, &in) != nil {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if len(email) > 254 || !a.allowAccountMail(r, email) {
		return
	}
	id, _, rec, err := a.Trestle.FindRecord("users", filterEq("email", email))
	if err != nil || id == "" || strOr(rec["disabled_at"]) != "" {
		return
	}
	ctx := context.WithoutCancel(r.Context())
	a.launchWorker(func() {
		deliveryContext, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		_ = a.sendAccountToken(deliveryContext, id, email, "password_reset")
	})
}
func (a *App) consumeAccountToken(w http.ResponseWriter, r *http.Request, purpose string) {
	a.accountTokenOperation(w, r, purpose, "consume")
}
func (a *App) handleAccountTokenValidation(w http.ResponseWriter, r *http.Request) {
	purpose := r.URL.Query().Get("purpose")
	if purpose != "email_verification" && purpose != "password_reset" {
		writeJSON(w, 400, map[string]any{"error": "account_token_invalid"})
		return
	}
	a.accountTokenOperation(w, r, purpose, "validate")
}
func (a *App) accountTokenOperation(w http.ResponseWriter, r *http.Request, purpose, action string) {
	if !a.allowAuthAttempt(r.RemoteAddr, action+":"+purpose) {
		writeJSON(w, 429, map[string]any{"error": "rate_limited"})
		return
	}
	var in struct {
		Token    string `json:"token"`
		Password string `json:"new_password"`
		Confirm  string `json:"confirm_password"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if action == "consume" && purpose == "password_reset" && (len(in.Password) < 8 || len(in.Password) > 72 || in.Password != in.Confirm) {
		writeJSON(w, 400, map[string]any{"error": "password_confirmation_or_policy"})
		return
	}
	parts := strings.Split(in.Token, ".")
	if len(parts) != 2 || len(parts[0]) > 80 || len(parts[0]) < 5 || len(parts[1]) != 43 || strings.ContainsAny(parts[0], "/\\ \r\n\t") {
		writeJSON(w, 400, map[string]any{"error": "account_token_invalid"})
		return
	}
	_, err := a.Trestle.AccountAuth("users", parts[0], map[string]any{"action": action, "purpose": purpose, "token": parts[1], "new_password": in.Password})
	if err != nil {
		var apiErr *trestle.APIError
		if errors.As(err, &apiErr) && apiErr.Status == 400 {
			writeJSON(w, 400, map[string]any{"error": "account_token_invalid"})
		} else {
			writeJSON(w, 503, map[string]any{"error": "account_operation_unavailable"})
		}
		return
	}
	if action == "consume" && purpose == "password_reset" {
		http.SetCookie(w, &http.Cookie{Name: "switchyard_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteLaxMode})
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (a *App) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	a.consumeAccountToken(w, r, "email_verification")
}
func (a *App) handleResetComplete(w http.ResponseWriter, r *http.Request) {
	a.consumeAccountToken(w, r, "password_reset")
}
func (a *App) handleAccountEmail(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	rec := a.userRecord(user)
	if rec == nil {
		writeJSON(w, 503, map[string]any{"error": "account_unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"email": strOr(rec["email"]), "verified_at": strOr(rec["verified_at"])})
}
