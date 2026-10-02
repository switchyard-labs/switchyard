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

	wfMu       sync.Mutex
	wfInFlight map[string]bool
	iqBusy     bool
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
		{"owner_namespaces", []trestle.CollectionField{{Name: "slug", Type: "text", Unique: true}, {Name: "owner_type", Type: "text"}, {Name: "owner_id", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"repository_meta", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "full_name", Type: "text", Unique: true}, {Name: "owner_type", Type: "text"}, {Name: "owner_id", Type: "text"}, {Name: "owner_slug", Type: "text"}, {Name: "slug", Type: "text"}, {Name: "display_name", Type: "text"}, {Name: "description", Type: "text"}, {Name: "visibility", Type: "text"}, {Name: "default_branch", Type: "text"}, {Name: "artifact_name", Type: "text", Unique: true}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"work", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "title", Type: "text"}, {Name: "kind", Type: "text"}, {Name: "status", Type: "text"}, {Name: "owner", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"work_details", []trestle.CollectionField{{Name: "work_id", Type: "text", Unique: true}, {Name: "body", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "assignee", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"work_comments", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "work_id", Type: "text"}, {Name: "author", Type: "text"}, {Name: "body", Type: "text"}, {Name: "created_at", Type: "text"}}},
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
		{"iq", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "pr_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "base", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "status", Type: "text"}, {Name: "risk", Type: "text"}, {Name: "policy", Type: "text"}, {Name: "attempts", Type: "text"}, {Name: "error", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"drafts", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "path", Type: "text"}, {Name: "content", Type: "text"}, {Name: "user", Type: "text"}, {Name: "revision", Type: "text"}, {Name: "base_sha", Type: "text"}, {Name: "last_agent_execution", Type: "text"}, {Name: "last_agent_prompt", Type: "text"}, {Name: "updated_at", Type: "text"}, {Name: "committed_at", Type: "text"}}},
		{"escalations", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "packet", Type: "json"}, {Name: "status", Type: "text"}, {Name: "created_by", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "decision", Type: "text"}, {Name: "decided_by", Type: "text"}, {Name: "decided_at", Type: "text"}}},
	} {
		if err := a.Trestle.EnsureCollection(c[0].(string), c[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	for _, c := range repositorySettingsCollections() {
		if err := a.Trestle.EnsureCollection(c[0].(string), c[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	for _, c := range collaborationCollections() {
		if err := a.Trestle.EnsureCollection(c[0].(string), c[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	for _, c := range profileCollections() {
		if err := a.Trestle.EnsureCollection(c[0].(string), c[1].([]trestle.CollectionField)); err != nil {
			return err
		}
	}
	for _, c := range policyCollections() {
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
	mux.HandleFunc("GET /api/owners/{slug}", a.handleGetOwnerProfile)
	mux.HandleFunc("GET /api/users/{username}", a.handleGetUserProfile)
	mux.HandleFunc("GET /api/users/{username}/repositories", a.handleUserRepositories)
	mux.HandleFunc("GET /api/users/{username}/activity", a.handleUserActivity)
	mux.HandleFunc("PATCH /api/settings/profile", a.handleUpdateUserProfile)
	mux.HandleFunc("POST /api/settings/avatar", a.handleUploadUserAvatar)
	mux.HandleFunc("DELETE /api/settings/avatar", a.handleDeleteUserAvatar)
	mux.HandleFunc("POST /api/settings/password", a.handleChangePassword)
	mux.HandleFunc("GET /api/settings/sessions", a.handleListSessions)
	mux.HandleFunc("POST /api/settings/sessions/revoke-others", a.handleRevokeOtherSessions)
	mux.HandleFunc("GET /api/avatars/{kind}/{id}", a.handleAvatar)

	// repositories: legacy flat API plus canonical owner/repository metadata (PX2)
	mux.HandleFunc("GET /api/repositories", a.handleListRepositoryMeta)
	mux.HandleFunc("POST /api/repositories/register", a.handleRegisterRepositoryMeta)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}", a.handleGetRepositoryMeta)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/tree", a.handleCanonicalRepoTree)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/content", a.handleCanonicalRepoContent)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/refs", a.handleCanonicalRepoRefs)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/overview", a.handleRepositoryOverview)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/commits", a.handleRepositoryCommits)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/commits/{sha}", a.handleRepositoryCommit)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/compare", a.handleRepositoryCompare)
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/settings", a.handleRepositorySettings)
	mux.HandleFunc("PATCH /api/repositories/{owner}/{repo}/settings", a.handleUpdateRepositorySettings)
	mux.HandleFunc("POST /api/repositories/{owner}/{repo}/collaborators", a.handleAddRepositoryCollaborator)
	mux.HandleFunc("POST /api/repositories/{owner}/{repo}/protected-refs", a.handleAddProtectedRef)

	// repos (legacy compatibility)
	mux.HandleFunc("GET /api/repos", a.handleListRepos)
	mux.HandleFunc("GET /api/repos/{name}", a.handleGetRepo)
	mux.HandleFunc("GET /api/repos/{name}/tree", a.handleRepoTree)
	mux.HandleFunc("GET /api/repos/{name}/content", a.handleRepoContent)

	// work
	mux.HandleFunc("GET /api/work", a.handleListWork)
	mux.HandleFunc("POST /api/work", a.handleCreateWork)
	mux.HandleFunc("GET /api/work/{id}", a.handleGetWork)
	mux.HandleFunc("PATCH /api/work/{id}", a.handleUpdateWork)
	mux.HandleFunc("GET /api/work/{id}/comments", a.handleListWorkComments)
	mux.HandleFunc("POST /api/work/{id}/comments", a.handleCreateWorkComment)

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
	mux.HandleFunc("GET /api/prs/{id}", a.handleGetPR)

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

	// org/policy/risk/audit/fleet (CP12)
	mux.HandleFunc("POST /api/orgs", a.handleCreateOrg)
	mux.HandleFunc("GET /api/orgs", a.handleListOrganizations)
	mux.HandleFunc("GET /api/orgs/{id}", a.handleGetOrgProfile)
	mux.HandleFunc("GET /api/orgs/{id}/repositories", a.handleOrgRepositories)
	mux.HandleFunc("GET /api/orgs/{id}/members", a.handleOrgMembers)
	mux.HandleFunc("POST /api/orgs/{id}/invitations", a.handleInviteOrgMember)
	mux.HandleFunc("GET /api/orgs/{id}/invitations", a.handleListOrgInvitations)
	mux.HandleFunc("POST /api/org-invitations/{id}/decide", a.handleDecideOrgInvitation)
	mux.HandleFunc("DELETE /api/orgs/{id}/members/{username}", a.handleRemoveOrgMember)
	mux.HandleFunc("POST /api/orgs/{id}/teams", a.handleCreateTeam)
	mux.HandleFunc("GET /api/orgs/{id}/teams", a.handleListTeams)
	mux.HandleFunc("GET /api/orgs/{id}/policies", a.handleListOrgPolicies)
	mux.HandleFunc("GET /api/orgs/{id}/audit", a.handleOrgAudit)
	mux.HandleFunc("GET /api/settings/org-invitations", a.handleMyOrgInvitations)
	mux.HandleFunc("POST /api/orgs/{id}/teams/{team}/members", a.handleAddTeamMember)
	mux.HandleFunc("POST /api/orgs/{id}/repositories/{repo}/access", a.handleSetRepoAccess)
	mux.HandleFunc("PATCH /api/orgs/{id}/profile", a.handleUpdateOrgProfile)
	mux.HandleFunc("POST /api/orgs/{id}/avatar", a.handleUploadOrgAvatar)
	mux.HandleFunc("POST /api/orgs/{id}/policies", a.handleCreatePolicy)
	mux.HandleFunc("GET /api/orgs/{id}/fleet", a.handleFleet)
	mux.HandleFunc("GET /api/audit", a.handleAudit)

	// needs attention + escalation (CP11)
	mux.HandleFunc("GET /api/attention", a.handleNeedsAttention)
	mux.HandleFunc("POST /api/attention/{kind}/{id}/escalate", a.handleEscalate)
	mux.HandleFunc("GET /api/escalations", a.handleListEscalations)
	mux.HandleFunc("POST /api/escalations/{id}/decide", a.handleDecide)

	// editor (CP10)
	mux.HandleFunc("POST /api/drafts", a.handleSaveDraft)
	mux.HandleFunc("GET /api/drafts/{repo}/{branch}/{path...}", a.handleGetDraft)
	mux.HandleFunc("POST /api/drafts/{id}/commit", a.handleCommitDraft)
	mux.HandleFunc("POST /api/drafts/{id}/agent-propose", a.handleAgentProposeDraft)
	mux.HandleFunc("POST /api/diff", a.handleDiff)

	// integration queue (CP9)
	mux.HandleFunc("POST /api/prs/{id}/enqueue", a.handleEnqueuePR)
	mux.HandleFunc("GET /api/queue", a.handleListQueue)
	mux.HandleFunc("POST /api/queue/{id}/requeue", a.handleRequeueItem)

	return a.withSession(mux)
}

// serveStatic serves the Nift-built public/ directory. Unknown paths that look
// like API calls return 404 JSON; everything else falls back to index.html.
func (a *App) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSON(w, 404, map[string]any{"error": "not_found"})
		return
	}
	if r.URL.Path == "/organizations" || r.URL.Path == "/organizations/" {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "organizations.html"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/organizations/") && strings.Contains(r.URL.Path, "/settings") {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "org-settings.html"))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/work/") {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "work-detail.html"))
		return
	}
	if r.URL.Path == "/settings" || strings.HasPrefix(r.URL.Path, "/settings/") {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "settings.html"))
		return
	}
	parts0 := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts0) == 3 && parts0[2] == "settings" {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "repo-settings.html"))
		return
	}
	if len(parts0) == 3 && parts0[2] == "pulls" {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "pulls.html"))
		return
	}
	if len(parts0) == 4 && parts0[2] == "pull" {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "pull.html"))
		return
	}
	if len(strings.Split(strings.Trim(r.URL.Path, "/"), "/")) >= 3 {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 3 && parts[2] == "commits" {
			http.ServeFile(w, r, filepath.Join(a.StaticDir, "history.html"))
			return
		}
	}
	// Owner profile URLs use /{owner}. Reserved/static paths are filtered by
	// validOwnerSlug; repository routes below take precedence for two segments.
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 1 && validOwnerSlug(parts[0]) {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "profile.html"))
		return
	}
	// Canonical Git-host repository URLs use /{owner}/{repo}[/(blob|tree)/{ref}/...].
	// They render the existing repository shell; the browser resolves the canonical
	// metadata/API path. Physical Artifacts names never appear in the public URL.
	if _, ok := parseRepositoryRoute(r.URL.Path); ok {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "repo.html"))
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
