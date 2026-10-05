package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOriginAndURLSchemes(t *testing.T) {
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.example")
	for _, origin := range []string{"https://evil.example", "null", "http://switchyard.example", "https://switchyard.example.evil"} {
		r := httptest.NewRequest("POST", "https://switchyard.example/api/drafts", nil)
		r.Header.Set("Origin", origin)
		if sameOrigin(r) {
			t.Fatal(origin)
		}
	}
	r := httptest.NewRequest("POST", "https://switchyard.example/api/drafts", nil)
	r.Header.Set("Origin", "https://switchyard.example")
	if !sameOrigin(r) || !secureRequest(r) {
		t.Fatal("same-origin https rejected")
	}
	for _, link := range []string{"javascript:alert(1)", "data:text/html,test", "java\tscript:alert(1)", "//evil.example", "https://u:p@example.com"} {
		if safeWebURL(link) {
			t.Fatal(link)
		}
	}
	if !safeWebURL("https://example.com/path") {
		t.Fatal("https rejected")
	}
}
func TestLoginThrottle(t *testing.T) {
	a := &App{}
	for i := 0; i < 10; i++ {
		if !a.allowLogin("127.0.0.1:123") {
			t.Fatal(i)
		}
	}
	if a.allowLogin("127.0.0.1:456") {
		t.Fatal("port bypass")
	}
	if !a.allowLogin("127.0.0.2:123") {
		t.Fatal("other client")
	}
}

func TestDemoGuestIsNotAuthenticated(t *testing.T) {
	a := &App{}
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	r = r.WithContext(contextWithDemoGuest(contextWithUser(r.Context(), "demo"), true))
	w := httptest.NewRecorder()
	a.handleMe(w, r)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("auth state must not be cached")
	}
	if w.Code != 401 || strings.Contains(w.Body.String(), `"authed":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestAnonymousOrganizationRoutes(t *testing.T) {
	a := &App{}
	for _, path := range []string{"/api/orgs", "/api/orgs/example", "/api/orgs/example/repositories", "/api/orgs/example/invitations", "/api/orgs/example/policies"} {
		r := httptest.NewRequest("GET", path, nil)
		if path != "/api/orgs" {
			r.SetPathValue("id", "example")
		}
		reached := false
		w := httptest.NewRecorder()
		a.authorizeHandler(func(w http.ResponseWriter, r *http.Request) { reached = true })(w, r)
		want := path == "/api/orgs" || path == "/api/orgs/example" || path == "/api/orgs/example/repositories"
		if reached != want {
			t.Fatalf("%s: allowed=%v want=%v", path, reached, want)
		}
	}
}

func TestMutationOriginsThroughMux(t *testing.T) {
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.cx")
	a := &App{}
	for _, path := range []string{"/api/auth/login", "/api/repositories/alice/repo/proposals", "/api/repositories/alice/repo/pages/deploy"} {
		for _, origin := range []string{"https://evil.example", "https://alice.switchyard.cx"} {
			r := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
			r.Header.Set("Origin", origin)
			r.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			if w.Code != 403 || !strings.Contains(w.Body.String(), "cross_origin_request_denied") {
				t.Fatalf("%s %s: %d %s", path, origin, w.Code, w.Body.String())
			}
		}
	}
	for _, origin := range []string{"", "https://switchyard.cx"} {
		reached := false
		handler := mutationOriginGuard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
		r := httptest.NewRequest("POST", "/api/test", nil)
		r.Header.Set("Origin", origin)
		handler.ServeHTTP(httptest.NewRecorder(), r)
		if !reached {
			t.Fatalf("CLI/same-origin denied: %q", origin)
		}
	}
}

func TestProductionSessionCookieDoesNotAcceptLegacyName(t *testing.T) {
	t.Setenv("SWITCHYARD_PUBLIC_URL", "https://switchyard.cx")
	a := &App{}
	r := httptest.NewRequest("GET", "/api/auth/me", nil)
	r.AddCookie(&http.Cookie{Name: "switchyard_session", Value: "attacker"})
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code, w.Body.String())
	}
	if sessionCookieName(r) != "__Host-switchyard_session" {
		t.Fatal("unsafe production cookie")
	}
}
