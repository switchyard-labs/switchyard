package app

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRoutesDoNotFallBackToDashboard(t *testing.T) {
	a := &App{StaticDir: filepath.Join("..", "..", "public")}
	for path, marker := range map[string]string{"/work": `id="work-list"`, "/work/": `id="work-list"`, "/alice/railway/work": `id="work-list"`, "/alice/railway/commit/abcdef": `id="commit-detail"`, "/pulls": `id="pull-list"`, "/signin": `id="signin-form"`} {
		response := httptest.NewRecorder()
		a.serveStatic(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), marker) || strings.Contains(response.Body.String(), `id="home-greeting"`) {
			t.Fatalf("workspace route %s served the wrong surface", path)
		}
	}
}
