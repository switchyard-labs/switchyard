package app

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCampaignNamingRoutesAndReadSurfaces(t *testing.T) {
	for _, owner := range []string{"n-ham", "strut-labs"} {
		for _, repo := range []string{"foo-bar", "foo.js", "foo_bar", "foo-1.2"} {
			t.Run(owner+"/"+repo, func(t *testing.T) {
				if !validOwnerSlug(owner) || !validRepoSlug(repo) {
					t.Fatal("valid identity rejected")
				}
				meta := map[string]any{"id": "meta", "full_name": owner + "/" + repo, "owner_slug": owner, "slug": repo, "owner_type": "user", "owner_id": owner, "artifact_name": "physical", "visibility": "public"}
				a := securityFixture(t, map[string][]map[string]any{"repository_meta": {meta}})
				a.StaticDir = filepath.Join("..", "..", "public")
				for suffix, marker := range map[string]string{"/proposals": `id="proposal-page"`, "/proposals/prop_fixture": `id="proposal-page"`, "/pages": `id="pages-page"`, "/edit/main/index.html": `id="editor-tabs"`} {
					w := httptest.NewRecorder()
					a.serveStatic(w, httptest.NewRequest("GET", "/"+owner+"/"+repo+suffix, nil))
					if w.Code != 200 || !strings.Contains(w.Body.String(), marker) {
						t.Fatalf("%s: %d wrong surface", suffix, w.Code)
					}
				}
				archiveApp, transport := archiveFixture(t, "public")
				archiveApp.Trestle = a.Trestle
				r := httptest.NewRequest("GET", "/"+owner+"/"+repo+"/archive/main.zip", nil)
				r.SetPathValue("owner", owner)
				r.SetPathValue("repo", repo)
				r.SetPathValue("archive", "main.zip")
				w := httptest.NewRecorder()
				archiveApp.handleRepositoryArchive(w, r)
				if w.Code != 200 || w.Header().Get("X-Switchyard-Commit") != transport.sha {
					t.Fatal("named archive", w.Code, w.Body.String())
				}
				root := t.TempDir()
				remote := filepath.Join(root, owner, repo+".git")
				local := filepath.Join(root, "source")
				clone := filepath.Join(root, "clone")
				if err := os.Mkdir(filepath.Join(root, owner), 0700); err != nil {
					t.Fatal(err)
				}
				queueGit(t, root, "init", "--bare", remote)
				queueGit(t, root, "init", "-b", "main", local)
				queueGit(t, local, "commit", "--allow-empty", "-m", "naming fixture")
				queueGit(t, local, "push", remote, "main")
				queueGit(t, root, "clone", "--branch", "main", remote, clone)
				queueGit(t, clone, "checkout", "-b", "website")
				queueGit(t, clone, "push", remote, "website")
				queueGit(t, local, "fetch", remote, "website")
				for _, handler := range []func(*httptest.ResponseRecorder){
					func(w *httptest.ResponseRecorder) {
						r := httptest.NewRequest("GET", "/api/repositories/"+owner+"/"+repo+"/proposals", nil)
						r.SetPathValue("owner", owner)
						r.SetPathValue("repo", repo)
						a.handleProposals(w, r)
					},
					func(w *httptest.ResponseRecorder) {
						r := httptest.NewRequest("GET", "/api/repositories/"+owner+"/"+repo+"/releases", nil)
						r.SetPathValue("owner", owner)
						r.SetPathValue("repo", repo)
						a.handleReleases(w, r)
					},
					func(w *httptest.ResponseRecorder) {
						r := httptest.NewRequest("GET", "/api/repositories/"+owner+"/"+repo+"/pages/deployments", nil)
						r.SetPathValue("owner", owner)
						r.SetPathValue("repo", repo)
						a.handlePagesDeployments(w, r)
					},
				} {
					w := httptest.NewRecorder()
					handler(w)
					if w.Code != 200 {
						t.Fatal(w.Code, w.Body.String())
					}
				}
			})
		}
	}
}
