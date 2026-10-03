package app

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"switchyard/internal/artifacts"
	"testing"
)

func TestArtifactsRateLimitReachesClient(t *testing.T) {
	w := httptest.NewRecorder()
	writeArtifactsError(w, fmt.Errorf("repository read: %w", &artifacts.Error{Code: "upstream_rate_limited", Status: 429, RetryAfter: "17"}))
	if w.Code != 429 || w.Header().Get("Retry-After") != "17" || !strings.Contains(w.Body.String(), "upstream_rate_limited") {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
func TestOrganizationFallbackUsesIdentity(t *testing.T) {
	a, _, _ := actionFixture(t)
	if _, _, err := a.Trestle.CreateRecord("orgs", map[string]any{"id": "org_fixture", "name": "Railway Labs", "owner": "alice"}, "org-avatar"); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/avatars/org/org_fixture", nil)
	r.SetPathValue("kind", "org")
	r.SetPathValue("id", "org_fixture")
	w := httptest.NewRecorder()
	a.handleAvatar(w, r)
	if !strings.Contains(w.Body.String(), ">R</text>") {
		t.Fatal("organization fallback did not use name")
	}
}
