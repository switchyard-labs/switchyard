package refs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/artifacts"
	"switchyard/internal/trestle"
	"testing"
)

func TestPublicationReceiptConvergence(t *testing.T) {
	for _, mode := range []string{"normal", "ambiguous", "stale", "noop", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			remote := filepath.Join(root, "remote.git")
			local := filepath.Join(root, "local")
			fixtureGit(t, root, "init", "--bare", remote)
			fixtureGit(t, root, "init", "-b", "main", local)
			os.WriteFile(filepath.Join(local, "base"), []byte("base"), 0600)
			fixtureGit(t, local, "add", ".")
			fixtureGit(t, local, "commit", "-m", "base")
			fixtureGit(t, local, "push", remote, "main")
			fixtureGit(t, local, "checkout", "-b", "source")
			os.WriteFile(filepath.Join(local, "source"), []byte("source"), 0600)
			fixtureGit(t, local, "add", ".")
			fixtureGit(t, local, "commit", "-m", "source")
			fixtureGit(t, local, "push", remote, "source")
			helper := filepath.Join(root, "token.sh")
			os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture\n"), 0700)
			scratch := filepath.Join(root, "scratch")
			os.Mkdir(scratch, 0700)
			records := map[string][]map[string]any{}
			facts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/admin/v1/session" {
					fmt.Fprint(w, `{"csrfToken":"fixture"}`)
					return
				}
				parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
				collection := parts[3]
				if r.Method == "GET" {
					items := []any{}
					for i, v := range records[collection] {
						items = append(items, map[string]any{"id": fmt.Sprint(i), "version": 1, "values": v})
					}
					json.NewEncoder(w).Encode(map[string]any{"items": items})
					return
				}
				var body struct {
					Values map[string]any `json:"values"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				if r.Method == "PATCH" {
					records[collection][0] = body.Values
					fmt.Fprint(w, `{}`)
					return
				}
				if collection == "ref_updates" {
					facts++
					if mode == "conflict" {
						records[collection] = []map[string]any{{"repo": "repo", "branch": "main", "new_sha": body.Values["new_sha"], "provenance": "someone-else"}}
						w.WriteHeader(409)
						return
					}
				}
				records[collection] = append(records[collection], body.Values)
				json.NewEncoder(w).Encode(map[string]any{"id": "0", "version": 1, "values": body.Values})
			}))
			defer server.Close()
			ac := artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: artifactFixtureTransport{remote}})
			service := NewService(ac, trestle.New(server.URL, "fixture", "fixture"), scratch)
			candidate, err := service.PrepareMerge("repo", "main", "source", "candidate")
			if err != nil {
				t.Fatal(err)
			}
			defer candidate.Close()
			if mode == "noop" {
				candidate.CommitSHA = candidate.BaseSHA
			}
			pushes := 0
			result, err := service.trackedPublication(candidate, "test", func() (*Result, error) {
				pushes++
				if mode == "stale" {
					return &Result{Status: "stale"}, nil
				}
				result, err := service.PublishPrepared(candidate)
				if err == nil && mode == "ambiguous" {
					return nil, fmt.Errorf("simulated lost push response")
				}
				return result, err
			})
			if mode == "conflict" {
				if err == nil {
					t.Fatal("conflicting receipt accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "stale" || mode == "noop" {
				if facts != 0 {
					t.Fatal("non-movement invented provenance")
				}
				if mode == "noop" && pushes != 0 {
					t.Fatal("noop pushed")
				}
				return
			}
			if result.Status != "ok" || pushes != 1 || facts != 1 {
				t.Fatal(result, pushes, facts)
			}
			if _, err = service.trackedPublication(candidate, "test", func() (*Result, error) { t.Fatal("retry pushed again"); return nil, nil }); err != nil {
				t.Fatal(err)
			}
			if facts != 1 {
				t.Fatal("duplicate fact", facts)
			}
		})
	}
}
