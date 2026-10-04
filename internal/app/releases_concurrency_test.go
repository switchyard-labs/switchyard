package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/artifacts"
	"switchyard/internal/refs"
	"sync"
	"testing"
	"time"
)

type releaseGitTransport struct{ remote, sha string }

func (f releaseGitTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.Contains(r.URL.Path, "/log") {
		body, _ := json.Marshal(map[string]any{"result": []map[string]any{{"hash": f.sha}}})
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	}
	return (queueArtifactTransport{f.remote}).RoundTrip(r)
}

// Hold both final CAS writes until both handlers have read the original release.
// Real local Git supplies the tag; the versioned HTTP store arbitrates the race.
func TestReleaseConcurrentEditAndPublish(t *testing.T) {
	for _, second := range []string{`{"title":"edited"}`, `{"publish":true}`} {
		t.Run(second, func(t *testing.T) {
			a, store, _ := actionFixture(t)
			root := t.TempDir()
			remote := filepath.Join(root, "repo.git")
			queueGit(t, root, "init", "--bare", remote)
			local := filepath.Join(root, "local")
			queueGit(t, root, "init", "-b", "main", local)
			if err := os.WriteFile(filepath.Join(local, "file"), []byte("release\n"), 0600); err != nil {
				t.Fatal(err)
			}
			queueGit(t, local, "add", ".")
			queueGit(t, local, "commit", "-m", "release")
			sha := queueGit(t, local, "rev-parse", "HEAD")
			queueGit(t, local, "push", remote, "HEAD:refs/tags/v1")
			helper := filepath.Join(root, "token.sh")
			if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700); err != nil {
				t.Fatal(err)
			}
			a.Artifacts = artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: releaseGitTransport{remote, sha}})
			a.Refs = refs.NewService(a.Artifacts, a.Trestle, filepath.Join(root, "scratch"))
			for collection, record := range map[string]map[string]any{
				"repository_meta": {"id": "repo-id", "full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "artifact_name": "repo"},
				"releases":        {"id": "release-id", "identity": releaseIdentity("repo-id", "v1"), "repo_id": "repo-id", "tag": "v1", "target_sha": sha, "draft": "true", "title": "original"},
			} {
				if _, _, err := a.Trestle.CreateRecord(collection, record, collection); err != nil {
					t.Fatal(err)
				}
			}
			var mu sync.Mutex
			arrived := 0
			gate := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/releases/") {
					mu.Lock()
					arrived++
					if arrived == 2 {
						close(gate)
					}
					mu.Unlock()
					select {
					case <-gate:
					case <-time.After(10 * time.Second):
						http.Error(w, "race barrier timeout", 504)
						return
					}
				}
				store.ServeHTTP(w, r)
			}))
			defer server.Close()
			a.Trestle.BaseURL = server.URL
			results := make(chan *httptest.ResponseRecorder, 2)
			for _, body := range []string{`{"publish":true}`, second} {
				go func(body string) {
					w := httptest.NewRecorder()
					a.handleRelease(w, releaseRequest("PATCH", "/api/repositories/alice/demo/releases/v1", body))
					results <- w
				}(body)
			}
			codes := map[int]int{}
			for i := 0; i < 2; i++ {
				w := <-results
				codes[w.Code]++
				if w.Code != 200 && w.Code != 409 {
					t.Fatalf("unexpected response %d %s", w.Code, w.Body.String())
				}
			}
			if codes[200] != 1 || codes[409] != 1 {
				t.Fatalf("race results %v", codes)
			}
			// Retry the publish after re-reading the winning version.
			w := httptest.NewRecorder()
			a.handleRelease(w, releaseRequest("PATCH", "/api/repositories/alice/demo/releases/v1", `{"publish":true}`))
			if w.Code != 200 {
				t.Fatalf("publish retry %d %s", w.Code, w.Body.String())
			}
			_, _, record, err := a.Trestle.FindRecord("releases", filterEq("identity", releaseIdentity("repo-id", "v1")))
			if err != nil {
				t.Fatal(err)
			}
			if record["target_sha"] != sha || record["draft"] != "false" || strOf(record["published_at"]) == "" {
				t.Fatalf("invalid published identity %v", record)
			}
			publishedAt := record["published_at"]
			w = httptest.NewRecorder()
			a.handleRelease(w, releaseRequest("PATCH", "/api/repositories/alice/demo/releases/v1", `{"publish":true}`))
			if w.Code != 200 {
				t.Fatalf("duplicate publish %d %s", w.Code, w.Body.String())
			}
			_, _, record, err = a.Trestle.FindRecord("releases", filterEq("identity", releaseIdentity("repo-id", "v1")))
			if err != nil || record["target_sha"] != sha || record["published_at"] != publishedAt {
				t.Fatal("duplicate publication changed immutable identity or publication date")
			}
			store.mu.Lock()
			count := len(store.records["releases"])
			store.mu.Unlock()
			if count != 1 {
				t.Fatalf("duplicate release records %d", count)
			}
		})
	}
}
