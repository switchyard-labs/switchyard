package app

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type loginWindow struct {
	Count int
	Until time.Time
}

func (a *App) allowLogin(remote string) bool { return a.allowAuthAttempt(remote, "login") }
func (a *App) allowAuthAttempt(remote, purpose string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	host = purpose + ":" + host
	a.authMu.Lock()
	defer a.authMu.Unlock()
	now := time.Now()
	if a.loginAttempts == nil {
		a.loginAttempts = map[string]loginWindow{}
	}
	for k, v := range a.loginAttempts {
		if now.After(v.Until) {
			delete(a.loginAttempts, k)
		}
	}
	if len(a.loginAttempts) >= 10000 {
		return false
	}
	v := a.loginAttempts[host]
	if now.After(v.Until) {
		v = loginWindow{Until: now.Add(5 * time.Minute)}
	}
	v.Count++
	a.loginAttempts[host] = v
	return v.Count <= 10
}
func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.HasPrefix(os.Getenv("SWITCHYARD_PUBLIC_URL"), "https://")
}
func sameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	expected := os.Getenv("SWITCHYARD_PUBLIC_URL")
	if expected == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		expected = scheme + "://" + r.Host
	}
	u, e := url.Parse(origin)
	v, f := url.Parse(expected)
	return e == nil && f == nil && u.Scheme == v.Scheme && u.Host == v.Host && u.User == nil && u.Path == ""
}
func safeWebURL(raw string) bool {
	if raw == "" {
		return true
	}
	if strings.ContainsAny(raw, "\r\n\t") {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil
}
