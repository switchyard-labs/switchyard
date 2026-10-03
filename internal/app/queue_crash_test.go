package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"switchyard/internal/artifacts"
	"switchyard/internal/refs"
	"switchyard/internal/trestle"
	"sync"
	"testing"
	"time"
)

type queueRecord struct {
	id      string
	version int
	values  map[string]any
}
type queueStore struct {
	mu                 sync.Mutex
	records            map[string][]*queueRecord
	next               int
	failStepCompletion bool
}

func (f *queueStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/admin/v1/session" {
		fmt.Fprint(w, `{"csrfToken":"fixture"}`)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 {
		w.WriteHeader(404)
		return
	}
	collection := parts[3]
	if r.Method == "GET" {
		items := []any{}
		for _, record := range f.records[collection] {
			matches := true
			for _, predicate := range strings.Split(r.URL.Query().Get("filter"), " AND ") {
				if predicate == "" {
					continue
				}
				pair := strings.SplitN(predicate, " = ", 2)
				if len(pair) != 2 {
					matches = false
					break
				}
				value, err := strconv.Unquote(pair[1])
				if err != nil || record.values[pair[0]] != value {
					matches = false
					break
				}
			}
			if matches {
				items = append(items, map[string]any{"id": record.id, "version": record.version, "values": record.values})
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
		return
	}
	var body struct {
		Values map[string]any `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(422)
		return
	}
	if r.Method == "POST" {
		f.next++
		record := &queueRecord{id: strconv.Itoa(f.next), version: 1, values: body.Values}
		f.records[collection] = append(f.records[collection], record)
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"id": record.id, "version": record.version, "values": record.values})
		return
	}
	if r.Method == "PATCH" && len(parts) == 6 {
		if f.failStepCompletion && collection == "wf_steps" && body.Values["status"] == "completed" {
			w.WriteHeader(503)
			return
		}
		for _, record := range f.records[collection] {
			if record.id != parts[5] {
				continue
			}
			if r.Header.Get("If-Match") != strconv.Itoa(record.version) {
				w.WriteHeader(409)
				return
			}
			for k, v := range body.Values {
				record.values[k] = v
			}
			record.version++
			fmt.Fprint(w, `{}`)
			return
		}
	}
	w.WriteHeader(404)
}

func queueGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@local", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@local")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

type queueArtifactTransport struct{ remote string }

func (f queueArtifactTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body := `{"result":{"plaintext":"fixture"}}`
	if r.Method == "GET" {
		b, _ := json.Marshal(map[string]any{"result": map[string]any{"name": "repo", "remote": f.remote, "default_branch": "main"}})
		body = string(b)
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}
func queueCrashApp(url, remote, root string) *App {
	tc := trestle.New(url, "fixture", "fixture")
	ac := artifacts.NewWithHTTP("fixture", "fixture", filepath.Join(root, "token.sh"), &http.Client{Transport: queueArtifactTransport{remote}})
	return &App{Trestle: tc, Artifacts: ac, Refs: refs.NewService(ac, tc, filepath.Join(root, "scratch"))}
}

func TestQueueCrashWorker(t *testing.T) {
	if os.Getenv("SWITCHYARD_QUEUE_CRASH_CHILD") != "yes" {
		return
	}
	a := queueCrashApp(os.Getenv("SWITCHYARD_QUEUE_FIXTURE_URL"), os.Getenv("SWITCHYARD_QUEUE_FIXTURE_REMOTE"), os.Getenv("SWITCHYARD_QUEUE_FIXTURE_ROOT"))
	a.queueCrashHook = func(phase string) {
		if phase == os.Getenv("SWITCHYARD_QUEUE_CRASH_PHASE") {
			os.Exit(91)
		}
	}
	if err := a.processQueueItem(&queueItem{id: "queue", prID: "pr", repo: "repo", base: "main", branch: "source", risk: "low"}); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash phase was not reached")
}

func TestQueueRecoversAfterProcessDeath(t *testing.T) {
	for _, phase := range []string{"after_claim", "after_checks", "after_preview", "after_semantic", "before_publication", "after_publication", "before_pr_completion", "before_queue_completion"} {
		t.Run(phase, func(t *testing.T) {
			root := t.TempDir()
			remote := filepath.Join(root, "remote.git")
			local := filepath.Join(root, "local")
			queueGit(t, root, "init", "--bare", remote)
			queueGit(t, root, "init", "-b", "main", local)
			os.WriteFile(filepath.Join(local, "base"), []byte("base"), 0600)
			queueGit(t, local, "add", ".")
			queueGit(t, local, "commit", "-m", "base")
			queueGit(t, local, "push", remote, "main")
			queueGit(t, local, "checkout", "-b", "source")
			os.WriteFile(filepath.Join(local, "source"), []byte("source"), 0600)
			queueGit(t, local, "add", ".")
			queueGit(t, local, "commit", "-m", "source")
			queueGit(t, local, "push", remote, "source")
			source := queueGit(t, local, "rev-parse", "HEAD")
			os.Mkdir(filepath.Join(root, "scratch"), 0700)
			os.WriteFile(filepath.Join(root, "token.sh"), []byte("#!/bin/sh\nprintf fixture\n"), 0700)
			store := &queueStore{records: map[string][]*queueRecord{}, next: 10}
			for collection, values := range map[string]map[string]any{
				"iq":              {"id": "queue", "pr_id": "pr", "repo": "repo", "base": "main", "branch": "source", "status": "queued"},
				"prs":             {"id": "pr", "repo": "repo", "base": "main", "branch": "source", "status": "open"},
				"commit_checks":   {"pr_id": "pr", "repo": "repo", "source_sha": source, "status": "pass", "created_at": "2026-10-03T00:00:00Z"},
				"repository_meta": {"id": "repo-meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice"},
			} {
				store.next++
				store.records[collection] = []*queueRecord{{id: strconv.Itoa(store.next), version: 1, values: values}}
			}
			server := httptest.NewServer(store)
			defer server.Close()
			cmd := exec.Command(os.Args[0], "-test.run=^TestQueueCrashWorker$")
			cmd.Env = append(os.Environ(), "SWITCHYARD_QUEUE_CRASH_CHILD=yes", "SWITCHYARD_QUEUE_CRASH_PHASE="+phase, "SWITCHYARD_QUEUE_FIXTURE_URL="+server.URL, "SWITCHYARD_QUEUE_FIXTURE_REMOTE="+remote, "SWITCHYARD_QUEUE_FIXTURE_ROOT="+root)
			output, err := cmd.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 91 {
				t.Fatalf("expected actual process death: %v %s", err, output)
			}
			before := queueGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
			store.mu.Lock()
			effects := store.records["integration_effects"]
			if len(effects) != 2 {
				store.mu.Unlock()
				t.Fatal("durable claim missing")
			}
			var effect queueEffect
			var effectRecord *queueRecord
			for _, record := range effects {
				current, e := decodeQueueEffect(record.values)
				if e != nil {
					store.mu.Unlock()
					t.Fatal(e)
				}
				current.LeaseUntil = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
				record.values["state"] = current
				record.version++
				if current.QueueID == "queue" {
					effectRecord = record
					effect = current
				}
			}
			if effectRecord == nil {
				store.mu.Unlock()
				t.Fatal("queue effect absent")
			}

			store.mu.Unlock()
			a := queueCrashApp(server.URL, remote, root)
			if err := a.processQueueItem(&queueItem{id: "queue", prID: "pr", repo: "repo", base: "main", branch: "source", risk: "low"}); err != nil {
				t.Fatal(err)
			}
			after := queueGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
			if phase == "after_publication" || phase == "before_pr_completion" || phase == "before_queue_completion" {
				if before != after {
					t.Fatal("recovery duplicated canonical publication")
				}
			}
			if count := queueGit(t, root, "--git-dir="+remote, "rev-list", "--count", "main"); count != "3" {
				t.Fatalf("canonical commit count %s", count)
			}
			store.mu.Lock()
			defer store.mu.Unlock()
			effect, err = decodeQueueEffect(effectRecord.values)
			if err != nil || effect.Phase != "done" || store.records["iq"][0].values["status"] != "done" || store.records["prs"][0].values["status"] != "integrated" {
				t.Fatalf("recovery incomplete: effect=%+v err=%v", effect, err)
			}
		})
	}
}
