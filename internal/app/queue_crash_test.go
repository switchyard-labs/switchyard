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
	failProvenance     int
	failStepCompletion bool
	enforceUnique      bool
	idempotency        map[string]*queueRecord
	rejectEventReplay  bool
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
			for _, predicate := range strings.Split(r.URL.Query().Get("filter"), " && ") {
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
	if r.Method == "DELETE" && len(parts) == 6 {
		for i, record := range f.records[collection] {
			if record.id == parts[5] {
				if r.Header.Get("If-Match") != strconv.Itoa(record.version) {
					w.WriteHeader(409)
					return
				}
				f.records[collection] = append(f.records[collection][:i], f.records[collection][i+1:]...)
				w.WriteHeader(204)
				return
			}
		}
		w.WriteHeader(404)
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
		if collection == "ref_updates" && f.failProvenance > 0 {
			f.failProvenance--
			w.WriteHeader(500)
			return
		}
		key := collection + "/" + r.Header.Get("Idempotency-Key")
		if f.enforceUnique {
			if old := f.idempotency[key]; old != nil && r.Header.Get("Idempotency-Key") != "" {
				if collection == "events" && f.rejectEventReplay {
					w.WriteHeader(http.StatusConflict)
					fmt.Fprint(w, `{"error":"idempotency_input_conflict"}`)
					return
				}
				w.Header().Set("Idempotency-Replayed", "true")
				json.NewEncoder(w).Encode(map[string]any{"id": old.id, "version": old.version, "values": old.values})
				return
			}
			if id, ok := body.Values["id"].(string); ok && id != "" {
				for _, old := range f.records[collection] {
					if old.values["id"] == id {
						w.WriteHeader(409)
						return
					}
				}
			}
		}
		f.next++
		record := &queueRecord{id: strconv.Itoa(f.next), version: 1, values: body.Values}
		f.records[collection] = append(f.records[collection], record)
		if f.enforceUnique && r.Header.Get("Idempotency-Key") != "" {
			if f.idempotency == nil {
				f.idempotency = map[string]*queueRecord{}
			}
			f.idempotency[key] = record
		}
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
	a.Refs.PublicationCheckpoint = a.queueCheckpoint
	if err := a.processQueueItem(&queueItem{id: "queue", prID: "pr", repo: "repo", base: "main", branch: "source", risk: "low"}); err != nil {
		t.Fatal(err)
	}
	t.Fatal("crash phase was not reached")
}

func TestQueueRecoversAfterProcessDeath(t *testing.T) {
	for _, phase := range []string{"after_publication_intent", "before_git_push", "after_git_push", "after_provenance_receipt", "after_claim", "after_checks", "after_preview", "after_semantic", "before_publication", "after_publication", "before_pr_completion", "before_queue_completion"} {
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
			store := &queueStore{records: map[string][]*queueRecord{}, next: 10, enforceUnique: true, idempotency: map[string]*queueRecord{}}
			for collection, values := range map[string]map[string]any{
				"iq":              {"source_sha": source, "approved_by": "alice", "id": "queue", "pr_id": "pr", "repo": "repo", "base": "main", "branch": "source", "status": "queued"},
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
			if len(store.records["ref_updates"]) != 1 {
				t.Fatalf("provenance count %d", len(store.records["ref_updates"]))
			}
			effect, err = decodeQueueEffect(effectRecord.values)
			if err != nil || effect.Phase != "done" || store.records["iq"][0].values["status"] != "done" || store.records["prs"][0].values["status"] != "integrated" {
				t.Fatalf("recovery incomplete: effect=%+v err=%v", effect, err)
			}
		})
	}
}

func TestQueueIntentCannotRetargetCheckedSource(t *testing.T) {
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
	os.WriteFile(filepath.Join(local, "source"), []byte("A"), 0600)
	queueGit(t, local, "add", ".")
	queueGit(t, local, "commit", "-m", "A")
	queueGit(t, local, "push", remote, "source")
	sourceA := queueGit(t, local, "rev-parse", "HEAD")
	os.Mkdir(filepath.Join(root, "scratch"), 0700)
	os.WriteFile(filepath.Join(root, "token.sh"), []byte("#!/bin/sh\nprintf fixture\n"), 0700)
	store := &queueStore{records: map[string][]*queueRecord{
		"prs":             {{id: "pr", version: 1, values: map[string]any{"id": "pr", "repo": "repo", "branch": "source", "base": "main"}}},
		"commit_checks":   {{id: "check-a", version: 1, values: map[string]any{"pr_id": "pr", "repo": "repo", "source_sha": sourceA, "status": "pass", "created_at": nowStr()}}},
		"repository_meta": {{id: "meta", version: 1, values: map[string]any{"id": "meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice"}}},
	}, next: 10}
	server := httptest.NewServer(store)
	defer server.Close()
	a := queueCrashApp(server.URL, remote, root)
	a.DataDir = root
	pr := store.records["prs"][0].values
	idA, err := a.enqueuePR(pr, sourceA, "alice")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(local, "source"), []byte("B"), 0600)
	queueGit(t, local, "add", ".")
	queueGit(t, local, "commit", "-m", "B")
	queueGit(t, local, "push", remote, "source")
	sourceB := queueGit(t, local, "rev-parse", "HEAD")
	store.mu.Lock()
	store.records["commit_checks"] = append(store.records["commit_checks"], &queueRecord{id: "check-b", version: 1, values: map[string]any{"pr_id": "pr", "repo": "repo", "source_sha": sourceB, "status": "pass", "created_at": nowStr()}})
	store.mu.Unlock()
	before := queueGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
	if err := a.processQueueItem(&queueItem{id: idA, prID: "pr", repo: "repo", base: "main", branch: "source"}); err != nil {
		t.Fatal(err)
	}
	if after := queueGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main"); after != before {
		t.Fatal("changed source integrated under old intent")
	}
	_, _, intent, err := a.Trestle.FindRecord("iq", filterEq("id", idA))
	if err != nil || intent["status"] != "blocked" || intent["source_sha"] != sourceA {
		t.Fatal(intent, err)
	}
	if _, err := a.enqueuePR(pr, sourceA, "alice"); err == nil {
		t.Fatal("stale enqueue accepted")
	}
	idB, err := a.enqueuePR(pr, sourceB, "alice")
	if err != nil || idB == idA {
		t.Fatal(idA, idB, err)
	}

	// Normal base movement must re-preview B without retargeting its source.
	queueGit(t, local, "checkout", "main")
	os.WriteFile(filepath.Join(local, "base-two"), []byte("canonical movement"), 0600)
	queueGit(t, local, "add", ".")
	queueGit(t, local, "commit", "-m", "base moved")
	queueGit(t, local, "push", remote, "main")
	baseMoved := queueGit(t, local, "rev-parse", "HEAD")
	store.mu.Lock()
	store.enforceUnique = true
	store.idempotency = map[string]*queueRecord{}
	store.failProvenance = 1
	store.mu.Unlock()
	itemB := &queueItem{id: idB, prID: "pr", repo: "repo", base: "main", branch: "source", risk: "low"}
	if err := a.processQueueItem(itemB); err == nil {
		t.Fatal("lost provenance acknowledgement reported success")
	}
	published := queueGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main")
	if published == baseMoved {
		t.Fatal("fault did not happen after Git publication")
	}
	// Revocation after the Git effect permits historical acknowledgement only.
	store.mu.Lock()
	store.records["repository_settings"] = []*queueRecord{{id: "settings", version: 1, values: map[string]any{"repo_id": "meta", "archived": "true"}}}
	store.mu.Unlock()
	if err := a.processQueueItem(itemB); err != nil {
		t.Fatal("publication recovery", err)
	}
	if after := queueGit(t, root, "--git-dir="+remote, "rev-parse", "refs/heads/main"); after != published {
		t.Fatal("recovery duplicated publication")
	}
	_, _, done, _ := a.Trestle.FindRecord("iq", filterEq("id", idB))
	if done["source_sha"] != sourceB || done["status"] != "done" {
		t.Fatal(done)
	}
	facts, err := a.Trestle.ListRecords("ref_updates", "")
	if err != nil || len(facts) != 1 {
		t.Fatal("publication fact count", len(facts), err)
	}
	store.mu.Lock()
	delete(store.records, "repository_settings")
	store.mu.Unlock()
	// A permanently broken source ref gets a bounded terminal state.
	_, _, err = a.Trestle.CreateRecord("iq", map[string]any{"id": "poison", "pr_id": "pr", "repo": "repo", "base": "missing-base", "branch": "source", "source_sha": sourceB, "approved_by": "alice", "status": "queued"}, "poison")
	if err != nil {
		t.Fatal(err)
	}
	poison := &queueItem{id: "poison", prID: "pr", repo: "repo", base: "missing-base", branch: "source"}
	for i := 0; i < maxQueueAttempts; i++ {
		if err := a.processQueueItem(poison); err == nil {
			t.Fatal("broken ref unexpectedly succeeded")
		}
	}
	_, _, blocked, _ := a.Trestle.FindRecord("iq", filterEq("id", "poison"))
	if blocked["status"] != "blocked" || blocked["attempts"] != itoa(maxQueueAttempts) {
		t.Fatal(blocked)
	}
	// Legacy records cannot acquire identity from the current source.
	_, _, err = a.Trestle.CreateRecord("iq", map[string]any{"id": "legacy", "status": "queued"}, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.processQueueItem(&queueItem{id: "legacy", prID: "pr", repo: "repo", base: "main", branch: "source"}); err != nil {
		t.Fatal(err)
	}
	_, _, legacy, _ := a.Trestle.FindRecord("iq", filterEq("id", "legacy"))
	if legacy["status"] != "blocked" {
		t.Fatal(legacy)
	}
}
