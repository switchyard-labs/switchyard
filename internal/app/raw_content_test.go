package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepositoryHTMLAndSVGCannotExecuteSameOrigin(t *testing.T) {
	for _, content := range []string{`<!doctype html><script>fetch('/api/credentials')</script>`, `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(document.cookie)"></svg>`} {
		w := httptest.NewRecorder()
		serveRepositoryContent(w, []byte(content))
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") || w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
			t.Fatal(w.Header())
		}
		if w.Body.String() != content {
			t.Fatal("changed repository bytes")
		}
	}
}
