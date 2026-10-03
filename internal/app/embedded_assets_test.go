package app

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedAssetsWithoutFilesystemDirectory(t *testing.T) {
	a := &App{}
	for path, marker := range map[string]string{"/signin": `id="signin-form"`, "/work": `id="work-list"`, "/alice": `id="pinned-repos"`, "/assets/js/app.js": "Switchyard"} {
		response := httptest.NewRecorder()
		a.serveStatic(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || !strings.Contains(response.Body.String(), marker) {
			t.Fatalf("embedded %s: status %d, missing marker", path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/assets/js/app.js", nil)
	request.Header.Set("Range", "bytes=0-9")
	a.serveStatic(response, request)
	if response.Code != 206 || response.Body.Len() != 10 {
		t.Fatalf("range: %d, %d bytes", response.Code, response.Body.Len())
	}
	response = httptest.NewRecorder()
	a.serveStatic(response, httptest.NewRequest("HEAD", "/assets/js/app.js", nil))
	if response.Code != 200 || response.Body.Len() != 0 {
		t.Fatalf("HEAD: %d", response.Code)
	}
}

func TestExplicitDevelopmentAssetOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("development override"), 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{StaticDir: dir}
	response := httptest.NewRecorder()
	a.serveStatic(response, httptest.NewRequest("GET", "/", nil))
	if response.Code != 200 || response.Body.String() != "development override" {
		t.Fatal("explicit development override ignored")
	}
}
