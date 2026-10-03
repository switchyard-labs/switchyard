package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCanonicalActionsPageAssets(t *testing.T) {
	a := &App{StaticDir: filepath.Join("..", "..", "public")}
	for _, path := range []string{"/alice/railway/actions", "/alice/railway/actions/run-1"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		a.serveStatic(response, request)
		if response.Code != 200 || !strings.Contains(response.Body.String(), `id="actions-page"`) {
			t.Fatalf("Actions route %s did not serve its page", path)
		}
		refs := regexp.MustCompile(`(?:src|href)="([^"]*assets/[^"]+)"`).FindAllStringSubmatch(response.Body.String(), -1)
		if len(refs) < 5 {
			t.Fatal("missing assets")
		}
		base, _ := url.Parse(path)
		for _, ref := range refs {
			relative, _ := url.Parse(ref[1])
			resolved := base.ResolveReference(relative)
			asset := httptest.NewRecorder()
			a.serveStatic(asset, httptest.NewRequest("GET", resolved.String(), nil))
			if asset.Code != 200 || strings.Contains(asset.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("canonical asset %s resolves to an HTML fallback", ref[1])
			}
		}
	}
}
