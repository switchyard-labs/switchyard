// Package app is the Switchyard control-plane HTTP server: it serves the
// Nift-built web application and the JSON API backed by Trestle (coordination
// truth) and Cloudflare Artifacts (Git truth).
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"switchyard/internal/agent"
	"switchyard/internal/artifacts"
	"switchyard/internal/refs"
	"switchyard/internal/trestle"
)

type App struct {
	Trestle   *trestle.Client
	Artifacts *artifacts.Client
	Refs      *refs.Service
	StrutBin  string
	StaticDir string
	DataDir   string
	Hub       *Hub
	Secrets   *agent.CredentialStore
	Runner    agent.Runner
	Roles     []agent.Role
	Queue     *QueueConsumer

	wfMu      sync.Mutex
	wfInFlight map[string]bool
}

func New(t *trestle.Client, a *artifacts.Client, staticDir, dataDir string) *App {
	scratch := filepath.Join(dataDir, "scratch")
	os.MkdirAll(scratch, 0700)
	return &App{Trestle: t, Artifacts: a, Refs: refs.NewService(a, t, scratch), StaticDir: staticDir, DataDir: dataDir, Hub: NewHub(), wfInFlight: map[string]bool{}}
}

// Provision creates the Switchyard coordination collections if missing.
func (a *App) Provision() error {
	for _, c := range [][2]any{
		{"users", []trestle.CollectionField{{Name: "username", Type: "text", Unique: true}, {Name: "password_hash", Type: "text", Required: true}, {Name: "display_name", Type: "text"}}},
		{"sessions", []trestle.CollectionField{{Name: "token", Type: "text", Unique: true}, {Name: "username", Type: "text"}, {Name: "expires_at", Type: "text"}}},
		{"repos", []trestle.CollectionField{{Name: "name", Type: "text", Unique: true}, {Name: "default_branch", Type: "text"}, {Name: "remote", Type: "text"}, {Name: "registered_at", Type: "text"}}},
		{"work", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "title", Type: "text"}, {Name: "kind", Type: "text"}, {Name: "status", Type: "text"}, {Name: "owner", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"events", []trestle.CollectionField{{Name: "type", Type: "text"}, {Name: "repo_name", Type: "text"}, {Name: "payload", Type: "json"}, {Name: "occurred_at", Type: "text"}}},
		{"ref_updates", []trestle.CollectionField{{Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "old_sha", Type: "text"}, {Name: "new_sha", Type: "text"}, {Name: "provenance", Type: "text"}, {Name: "occurred_at", Type: "text"}}},
		{"attempts", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "work_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "status", Type: "text"}, {Name: "owner", Type: "text"}, {Name: "message", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"runs", []trestle.CollectionField{{Name: "attempt_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "new_sha", Type: "text"}, {Name: "message", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"prs", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "attempt_id", Type: "text"}, {Name: "work_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "base", Type: "text"}, {Name: "title", Type: "text"}, {Name: "status", Type: "text"}, {Name: "check_status", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "integrated_at", Type: "text"}}},
		{"pr_checks", []trestle.CollectionField{{Name: "pr_id", Type: "text"}, {Name: "status", Type: "text"}, {Name: "detail", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"ref_obs", []trestle.CollectionField{{Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "sha", Type: "text"}, {Name: "seen_at", Type: "text"}}},
		{"credentials", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "name", Type: "text"}, {Name: "provider", Type: "text"}, {Name: "scope", Type: "text"}, {Name: "ciphertext", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "last_used", Type: "text"}}},
		{"executions", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "role", Type: "text"}, {Name: "attempt_id", Type: "text"}, {Name: "adapter", Type: "text"}, {Name: "status", Type: "text"}, {Name: "output", Type: "text"}, {Name: "started_at", Type: "text"}, {Name: "finished_at", Type: "text"}}},
		{"findings", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "target", Type: "text"}, {Name: "severity", Type: "text"}, {Name: "message", Type: "text"}, {Name: "file", Type: "text"}, {Name: "status", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "resolved_at", Type: "text"}}},
	} {
		if err := a.Trestle.EnsureCollection(c[0].(string), c[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	for _, c := range workflowCollections() {
		if err := a.Trestle.EnsureCollection(c[0].(string), c[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// static app (Nift build output)
	mux.HandleFunc("/", a.serveStatic)

	// auth
	mux.HandleFunc("POST /api/auth/register", a.handleRegister)
	mux.HandleFunc("POST /api/auth/login", a.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", a.handleLogout)
	mux.HandleFunc("GET /api/auth/me", a.handleMe)

	// repos
	mux.HandleFunc("GET /api/repos", a.handleListRepos)
	mux.HandleFunc("GET /api/repos/{name}", a.handleGetRepo)
	mux.HandleFunc("GET /api/repos/{name}/tree", a.handleRepoTree)
	mux.HandleFunc("GET /api/repos/{name}/content", a.handleRepoContent)

	// work
	mux.HandleFunc("GET /api/work", a.handleListWork)
	mux.HandleFunc("POST /api/work", a.handleCreateWork)
	mux.HandleFunc("GET /api/work/{id}", a.handleGetWork)

	// events (normalized Artifacts events, idempotent ingest)
	mux.HandleFunc("POST /api/events/ingest", a.handleIngestEvent)
	mux.HandleFunc("GET /api/events", a.handleListEvents)

	// refs (safe mutation substrate)
	mux.HandleFunc("POST /api/refs/update", a.handleRefUpdate)
	mux.HandleFunc("GET /api/repos/{name}/refs", a.handleRepoRefs)

	// attempts + PRs (deterministic vertical slice)
	mux.HandleFunc("POST /api/work/{id}/attempts", a.handleCreateAttempt)
	mux.HandleFunc("POST /api/attempts/{id}/run", a.handleRunAttempt)
	mux.HandleFunc("POST /api/attempts/{id}/pr", a.handleOpenPR)
	mux.HandleFunc("POST /api/prs/{id}/check", a.handlePRCheck)
	mux.HandleFunc("POST /api/prs/{id}/integrate", a.handlePRIntegrate)
	mux.HandleFunc("GET /api/prs", a.handleListPRs)

	// realtime + provenance (CP5)
	mux.HandleFunc("GET /api/events/stream", a.handleEventStream)
	mux.HandleFunc("GET /api/work/{id}/provenance", a.handleWorkProvenance)

	// agents + credentials (CP6)
	mux.HandleFunc("POST /api/credentials", a.handleCreateCredential)
	mux.HandleFunc("GET /api/credentials", a.handleListCredentials)
	mux.HandleFunc("POST /api/credentials/{id}/rotate", a.handleRotateCredential)
	mux.HandleFunc("DELETE /api/credentials/{id}", a.handleDeleteCredential)
	mux.HandleFunc("GET /api/roles", a.handleListRoles)

	// durable workflows (CP7)
	mux.HandleFunc("POST /api/workflows", a.handleCreateWorkflow)
	mux.HandleFunc("GET /api/workflows", a.handleListWorkflows)
	mux.HandleFunc("POST /api/workflows/{id}/run", a.handleStartRun)
	mux.HandleFunc("GET /api/workflow_runs", a.handleListRuns)
	mux.HandleFunc("GET /api/workflow_runs/{id}", a.handleGetRun)
	mux.HandleFunc("POST /api/workflow_runs/{id}/cancel", a.handleCancelRun)
	mux.HandleFunc("POST /api/workflow_runs/{id}/approve", a.handleApproveRun)
	mux.HandleFunc("POST /api/workflow_runs/{id}/retry", a.handleRetryRun)

	// reviews / findings / conflict (CP8)
	mux.HandleFunc("POST /api/attempts/{id}/review", a.handleReviewAttempt)
	mux.HandleFunc("POST /api/attempts/{id}/preview", a.handlePreview)
	mux.HandleFunc("POST /api/attempts/{id}/resolve", a.handleResolveConflict)
	mux.HandleFunc("GET /api/findings", a.handleListFindings)

	return a.withSession(mux)
}

// serveStatic serves the Nift-built public/ directory. Unknown paths that look
// like API calls return 404 JSON; everything else falls back to index.html.
func (a *App) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, 404, map[string]any{"error": "not_found"})
		return
	}
	p := filepath.Join(a.StaticDir, filepath.Clean("/"+r.URL.Path))
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	http.ServeFile(w, r, filepath.Join(a.StaticDir, "index.html"))
}

// ---- session middleware ----

type ctxKey int

const userKey ctxKey = 0

func (a *App) withSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			user := ""
			if c, err := r.Cookie("switchyard_session"); err == nil {
				if u, ok := a.userForSession(r, c.Value); ok {
					user = u
				}
			}
			ctx := contextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}

// auth helpers
func (a *App) userForSession(r *http.Request, token string) (string, bool) {
	items, err := a.Trestle.ListRecords("sessions", `token = "`+token+`"`)
	if err != nil || len(items) == 0 {
		return "", false
	}
	exp, _ := items[0]["expires_at"].(string)
	if t, err := time.Parse(time.RFC3339, exp); err != nil || time.Now().After(t) {
		return "", false
	}
	u, _ := items[0]["username"].(string)
	return u, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(out)
}