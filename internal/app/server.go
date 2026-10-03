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
	workflowCrashHook func(string)
	queueCrashHook    func(string) // test-only injection, unset by constructors

	Trestle   *trestle.Client
	Artifacts *artifacts.Client
	Refs      *refs.Service
	StaticDir string
	DataDir   string
	Hub       *Hub
	Secrets   *agent.CredentialStore
	Runner    agent.Runner
	Roles     []agent.Role
	Queue     *QueueConsumer
	DemoMode  bool

	authMu        sync.Mutex
	loginAttempts map[string]loginWindow
	wfMu          sync.Mutex
	wfInFlight    map[string]bool
	iqBusy        bool
}

func New(t *trestle.Client, a *artifacts.Client, staticDir, dataDir string) *App {
	scratch := filepath.Join(dataDir, "scratch")
	os.MkdirAll(scratch, 0700)
	return &App{Trestle: t, Artifacts: a, Refs: refs.NewService(a, t, scratch), StaticDir: staticDir, DataDir: dataDir, Hub: NewHub(), DemoMode: strings.EqualFold(strings.TrimSpace(os.Getenv("SWITCHYARD_DEMO_MODE")), "true"), wfInFlight: map[string]bool{}}
}

// Provision creates the Switchyard coordination collections if missing.
func switchyardCollections() [][2]any {
	collections := [][2]any{
		{"users", []trestle.CollectionField{{Name: "username", Type: "text", Unique: true}, {Name: "password_hash", Type: "text", Required: true}, {Name: "display_name", Type: "text"}}},
		{"sessions", []trestle.CollectionField{{Name: "token", Type: "text", Unique: true}, {Name: "username", Type: "text"}, {Name: "expires_at", Type: "text"}}},
		{"repos", []trestle.CollectionField{{Name: "name", Type: "text", Unique: true}, {Name: "default_branch", Type: "text"}, {Name: "remote", Type: "text"}, {Name: "registered_at", Type: "text"}}},
		{"owner_namespaces", []trestle.CollectionField{{Name: "slug", Type: "text", Unique: true}, {Name: "owner_type", Type: "text"}, {Name: "owner_id", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"repository_meta", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "full_name", Type: "text", Unique: true}, {Name: "owner_type", Type: "text"}, {Name: "owner_id", Type: "text"}, {Name: "owner_slug", Type: "text"}, {Name: "slug", Type: "text"}, {Name: "display_name", Type: "text"}, {Name: "description", Type: "text"}, {Name: "visibility", Type: "text"}, {Name: "default_branch", Type: "text"}, {Name: "artifact_name", Type: "text", Unique: true}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"work", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "title", Type: "text"}, {Name: "kind", Type: "text"}, {Name: "status", Type: "text"}, {Name: "owner", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"work_details", []trestle.CollectionField{{Name: "work_id", Type: "text", Unique: true}, {Name: "body", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "assignee", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"work_comments", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "work_id", Type: "text"}, {Name: "author", Type: "text"}, {Name: "body", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"event_receipts", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "repo", Type: "text"}, {Name: "envelope", Type: "json"}}},
		{"events", []trestle.CollectionField{{Name: "type", Type: "text"}, {Name: "repo_name", Type: "text"}, {Name: "payload", Type: "json"}, {Name: "occurred_at", Type: "text"}}},
		{"ref_updates", []trestle.CollectionField{{Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "old_sha", Type: "text"}, {Name: "new_sha", Type: "text"}, {Name: "provenance", Type: "text"}, {Name: "occurred_at", Type: "text"}}},
		{"attempts", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "work_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "status", Type: "text"}, {Name: "owner", Type: "text"}, {Name: "message", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"runs", []trestle.CollectionField{{Name: "attempt_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "new_sha", Type: "text"}, {Name: "message", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"prs", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "attempt_id", Type: "text"}, {Name: "work_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "base", Type: "text"}, {Name: "title", Type: "text"}, {Name: "status", Type: "text"}, {Name: "check_status", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "integrated_at", Type: "text"}}},
		{"commit_checks", []trestle.CollectionField{{Name: "pr_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "source_sha", Type: "text"}, {Name: "status", Type: "text"}, {Name: "detail", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"pr_checks", []trestle.CollectionField{{Name: "pr_id", Type: "text"}, {Name: "status", Type: "text"}, {Name: "detail", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"ref_obs", []trestle.CollectionField{{Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "sha", Type: "text"}, {Name: "seen_at", Type: "text"}}},
		{"credential_owners", []trestle.CollectionField{{Name: "credential_id", Type: "text", Unique: true}, {Name: "username", Type: "text"}}},
		{"credentials", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "name", Type: "text"}, {Name: "provider", Type: "text"}, {Name: "scope", Type: "text"}, {Name: "ciphertext", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "last_used", Type: "text"}}},
		{"execution_metadata", []trestle.CollectionField{{Name: "execution_id", Type: "text", Unique: true}, {Name: "repo", Type: "text"}, {Name: "metadata", Type: "json"}}},
		{"executions", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "role", Type: "text"}, {Name: "attempt_id", Type: "text"}, {Name: "adapter", Type: "text"}, {Name: "status", Type: "text"}, {Name: "output", Type: "text"}, {Name: "started_at", Type: "text"}, {Name: "finished_at", Type: "text"}}},
		{"findings", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "target", Type: "text"}, {Name: "severity", Type: "text"}, {Name: "message", Type: "text"}, {Name: "file", Type: "text"}, {Name: "status", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "resolved_at", Type: "text"}}},
		{"integration_effects", []trestle.CollectionField{{Name: "queue_id", Type: "text", Unique: true}, {Name: "state", Type: "json"}}},
		{"iq", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "pr_id", Type: "text"}, {Name: "repo", Type: "text"}, {Name: "base", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "status", Type: "text"}, {Name: "risk", Type: "text"}, {Name: "policy", Type: "text"}, {Name: "attempts", Type: "text"}, {Name: "error", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "updated_at", Type: "text"}}},
		{"drafts", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "repo", Type: "text"}, {Name: "branch", Type: "text"}, {Name: "path", Type: "text"}, {Name: "content", Type: "text"}, {Name: "user", Type: "text"}, {Name: "revision", Type: "text"}, {Name: "base_sha", Type: "text"}, {Name: "last_agent_execution", Type: "text"}, {Name: "last_agent_prompt", Type: "text"}, {Name: "updated_at", Type: "text"}, {Name: "committed_at", Type: "text"}}},
		{"escalations", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "packet", Type: "json"}, {Name: "status", Type: "text"}, {Name: "created_by", Type: "text"}, {Name: "created_at", Type: "text"}, {Name: "decision", Type: "text"}, {Name: "decided_by", Type: "text"}, {Name: "decided_at", Type: "text"}}},
	}
	for _, additional := range [][][2]any{repositorySettingsCollections(), collaborationCollections(), profileCollections(), policyCollections(), workflowCollections()} {
		collections = append(collections, additional...)
	}
	return collections
}

func (a *App) Provision() error { return a.provisionSchema() }

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()

	// static app (Nift build output)
	mux.HandleFunc("/", a.serveStatic)

	// auth
	mux.HandleFunc("POST /api/auth/register", a.authorizeHandler(a.handleRegister))
	mux.HandleFunc("POST /api/auth/login", a.authorizeHandler(a.handleLogin))
	mux.HandleFunc("POST /api/auth/logout", a.authorizeHandler(a.handleLogout))
	mux.HandleFunc("GET /api/auth/me", a.authorizeHandler(a.handleMe))
	mux.HandleFunc("GET /api/demo", a.authorizeHandler(a.handleDemoStatus))
	mux.HandleFunc("GET /api/owners/{slug}", a.authorizeHandler(a.handleGetOwnerProfile))
	mux.HandleFunc("GET /api/users/{username}", a.authorizeHandler(a.handleGetUserProfile))
	mux.HandleFunc("GET /api/users/{username}/repositories", a.authorizeHandler(a.handleUserRepositories))
	mux.HandleFunc("GET /api/users/{username}/activity", a.authorizeHandler(a.handleUserActivity))
	mux.HandleFunc("PATCH /api/settings/profile", a.authorizeHandler(a.handleUpdateUserProfile))
	mux.HandleFunc("POST /api/settings/avatar", a.authorizeHandler(a.handleUploadUserAvatar))
	mux.HandleFunc("DELETE /api/settings/avatar", a.authorizeHandler(a.handleDeleteUserAvatar))
	mux.HandleFunc("POST /api/settings/password", a.authorizeHandler(a.handleChangePassword))
	mux.HandleFunc("GET /api/settings/sessions", a.authorizeHandler(a.handleListSessions))
	mux.HandleFunc("POST /api/settings/sessions/revoke-others", a.authorizeHandler(a.handleRevokeOtherSessions))
	mux.HandleFunc("GET /api/avatars/{kind}/{id}", a.authorizeHandler(a.handleAvatar))

	// repositories: legacy flat API plus canonical owner/repository metadata (PX2)
	mux.HandleFunc("GET /api/repositories", a.authorizeHandler(a.handleListRepositoryMeta))
	mux.HandleFunc("POST /api/repositories/register", a.authorizeHandler(a.handleRegisterRepositoryMeta))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}", a.authorizeHandler(a.handleGetRepositoryMeta))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/tree", a.authorizeHandler(a.handleCanonicalRepoTree))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/content", a.authorizeHandler(a.handleCanonicalRepoContent))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/refs", a.authorizeHandler(a.handleCanonicalRepoRefs))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/overview", a.authorizeHandler(a.handleRepositoryOverview))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/commits", a.authorizeHandler(a.handleRepositoryCommits))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/commits/{sha}", a.authorizeHandler(a.handleRepositoryCommit))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/compare", a.authorizeHandler(a.handleRepositoryCompare))
	mux.HandleFunc("GET /api/repositories/{owner}/{repo}/settings", a.authorizeHandler(a.handleRepositorySettings))
	mux.HandleFunc("PATCH /api/repositories/{owner}/{repo}/settings", a.authorizeHandler(a.handleUpdateRepositorySettings))
	mux.HandleFunc("POST /api/repositories/{owner}/{repo}/collaborators", a.authorizeHandler(a.handleAddRepositoryCollaborator))
	mux.HandleFunc("POST /api/repositories/{owner}/{repo}/protected-refs", a.authorizeHandler(a.handleAddProtectedRef))

	// repos (legacy compatibility)
	mux.HandleFunc("GET /api/repos", a.authorizeHandler(a.handleListRepos))
	mux.HandleFunc("GET /api/repos/{name}", a.authorizeHandler(a.handleGetRepo))
	mux.HandleFunc("GET /api/repos/{name}/tree", a.authorizeHandler(a.handleRepoTree))
	mux.HandleFunc("GET /api/repos/{name}/content", a.authorizeHandler(a.handleRepoContent))

	// work
	mux.HandleFunc("GET /api/work", a.authorizeHandler(a.handleListWork))
	mux.HandleFunc("POST /api/work", a.authorizeHandler(a.handleCreateWork))
	mux.HandleFunc("GET /api/work/{id}", a.authorizeHandler(a.handleGetWork))
	mux.HandleFunc("PATCH /api/work/{id}", a.authorizeHandler(a.handleUpdateWork))
	mux.HandleFunc("GET /api/work/{id}/comments", a.authorizeHandler(a.handleListWorkComments))
	mux.HandleFunc("POST /api/work/{id}/comments", a.authorizeHandler(a.handleCreateWorkComment))

	// events (normalized Artifacts events, idempotent ingest)
	mux.HandleFunc("POST /api/events/ingest", a.authorizeHandler(a.handleIngestEvent))
	mux.HandleFunc("GET /api/events", a.authorizeHandler(a.handleListEvents))

	// refs (safe mutation substrate)
	mux.HandleFunc("POST /api/refs/update", a.authorizeHandler(a.handleRefUpdate))
	mux.HandleFunc("GET /api/repos/{name}/refs", a.authorizeHandler(a.handleRepoRefs))

	// attempts + PRs (deterministic vertical slice)
	mux.HandleFunc("POST /api/work/{id}/attempts", a.authorizeHandler(a.handleCreateAttempt))
	mux.HandleFunc("POST /api/attempts/{id}/run", a.authorizeHandler(a.handleRunAttempt))
	mux.HandleFunc("POST /api/attempts/{id}/pr", a.authorizeHandler(a.handleOpenPR))
	mux.HandleFunc("POST /api/prs/{id}/check", a.authorizeHandler(a.handlePRCheck))
	mux.HandleFunc("POST /api/prs/{id}/integrate", a.authorizeHandler(a.handlePRIntegrate))
	mux.HandleFunc("GET /api/prs", a.authorizeHandler(a.handleListPRs))
	mux.HandleFunc("GET /api/prs/{id}", a.authorizeHandler(a.handleGetPR))

	// realtime + provenance (CP5)
	mux.HandleFunc("GET /api/events/stream", a.authorizeHandler(a.handleEventStream))
	mux.HandleFunc("GET /api/work/{id}/provenance", a.authorizeHandler(a.handleWorkProvenance))

	// agents + credentials (CP6)
	mux.HandleFunc("POST /api/credentials", a.authorizeHandler(a.handleCreateCredential))
	mux.HandleFunc("GET /api/credentials", a.authorizeHandler(a.handleListCredentials))
	mux.HandleFunc("POST /api/credentials/{id}/rotate", a.authorizeHandler(a.handleRotateCredential))
	mux.HandleFunc("DELETE /api/credentials/{id}", a.authorizeHandler(a.handleDeleteCredential))
	mux.HandleFunc("GET /api/roles", a.authorizeHandler(a.handleListRoles))
	mux.HandleFunc("GET /api/executions", a.authorizeHandler(a.handleListExecutions))

	// durable workflows (CP7)
	mux.HandleFunc("POST /api/workflows", a.authorizeHandler(a.handleCreateWorkflow))
	mux.HandleFunc("GET /api/workflows", a.authorizeHandler(a.handleListWorkflows))
	mux.HandleFunc("POST /api/workflows/{id}/run", a.authorizeHandler(a.handleStartRun))
	mux.HandleFunc("GET /api/workflow_runs", a.authorizeHandler(a.handleListRuns))
	mux.HandleFunc("GET /api/workflow_runs/{id}", a.authorizeHandler(a.handleGetRun))
	mux.HandleFunc("POST /api/workflow_runs/{id}/cancel", a.authorizeHandler(a.handleCancelRun))
	mux.HandleFunc("POST /api/workflow_runs/{id}/approve", a.authorizeHandler(a.handleApproveRun))
	mux.HandleFunc("POST /api/workflow_runs/{id}/retry", a.authorizeHandler(a.handleRetryRun))

	// reviews / findings / conflict (CP8)
	mux.HandleFunc("POST /api/attempts/{id}/review", a.authorizeHandler(a.handleReviewAttempt))
	mux.HandleFunc("POST /api/attempts/{id}/preview", a.authorizeHandler(a.handlePreview))
	mux.HandleFunc("POST /api/attempts/{id}/resolve", a.authorizeHandler(a.handleResolveConflict))
	mux.HandleFunc("GET /api/findings", a.authorizeHandler(a.handleListFindings))

	// org/policy/risk/audit/fleet (CP12)
	mux.HandleFunc("POST /api/orgs", a.authorizeHandler(a.handleCreateOrg))
	mux.HandleFunc("GET /api/orgs", a.authorizeHandler(a.handleListOrganizations))
	mux.HandleFunc("GET /api/orgs/{id}", a.authorizeHandler(a.handleGetOrgProfile))
	mux.HandleFunc("GET /api/orgs/{id}/repositories", a.authorizeHandler(a.handleOrgRepositories))
	mux.HandleFunc("GET /api/orgs/{id}/members", a.authorizeHandler(a.handleOrgMembers))
	mux.HandleFunc("POST /api/orgs/{id}/invitations", a.authorizeHandler(a.handleInviteOrgMember))
	mux.HandleFunc("GET /api/orgs/{id}/invitations", a.authorizeHandler(a.handleListOrgInvitations))
	mux.HandleFunc("POST /api/org-invitations/{id}/decide", a.authorizeHandler(a.handleDecideOrgInvitation))
	mux.HandleFunc("DELETE /api/orgs/{id}/members/{username}", a.authorizeHandler(a.handleRemoveOrgMember))
	mux.HandleFunc("POST /api/orgs/{id}/teams", a.authorizeHandler(a.handleCreateTeam))
	mux.HandleFunc("GET /api/orgs/{id}/teams", a.authorizeHandler(a.handleListTeams))
	mux.HandleFunc("GET /api/orgs/{id}/policies", a.authorizeHandler(a.handleListOrgPolicies))
	mux.HandleFunc("GET /api/orgs/{id}/audit", a.authorizeHandler(a.handleOrgAudit))
	mux.HandleFunc("GET /api/settings/org-invitations", a.authorizeHandler(a.handleMyOrgInvitations))
	mux.HandleFunc("POST /api/orgs/{id}/teams/{team}/members", a.authorizeHandler(a.handleAddTeamMember))
	mux.HandleFunc("POST /api/orgs/{id}/repositories/{repo}/access", a.authorizeHandler(a.handleSetRepoAccess))
	mux.HandleFunc("PATCH /api/orgs/{id}/profile", a.authorizeHandler(a.handleUpdateOrgProfile))
	mux.HandleFunc("POST /api/orgs/{id}/avatar", a.authorizeHandler(a.handleUploadOrgAvatar))
	mux.HandleFunc("POST /api/orgs/{id}/policies", a.authorizeHandler(a.handleCreatePolicy))
	mux.HandleFunc("GET /api/orgs/{id}/fleet", a.authorizeHandler(a.handleFleet))
	mux.HandleFunc("GET /api/audit", a.authorizeHandler(a.handleAudit))

	// needs attention + escalation (CP11)
	mux.HandleFunc("GET /api/attention", a.authorizeHandler(a.handleNeedsAttention))
	mux.HandleFunc("POST /api/attention/{kind}/{id}/escalate", a.authorizeHandler(a.handleEscalate))
	mux.HandleFunc("GET /api/escalations", a.authorizeHandler(a.handleListEscalations))
	mux.HandleFunc("POST /api/escalations/{id}/decide", a.authorizeHandler(a.handleDecide))

	// editor (CP10)
	mux.HandleFunc("POST /api/drafts", a.authorizeHandler(a.handleSaveDraft))
	mux.HandleFunc("GET /api/drafts/{repo}/{branch}/{path...}", a.authorizeHandler(a.handleGetDraft))
	mux.HandleFunc("POST /api/drafts/{id}/commit", a.authorizeHandler(a.handleCommitDraft))
	mux.HandleFunc("POST /api/drafts/{id}/agent-propose", a.authorizeHandler(a.handleAgentProposeDraft))
	mux.HandleFunc("POST /api/diff", a.authorizeHandler(a.handleDiff))

	// integration queue (CP9)
	mux.HandleFunc("POST /api/prs/{id}/enqueue", a.authorizeHandler(a.handleEnqueuePR))
	mux.HandleFunc("GET /api/queue", a.authorizeHandler(a.handleListQueue))
	mux.HandleFunc("POST /api/queue/{id}/requeue", a.authorizeHandler(a.handleRequeueItem))

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
	if r.URL.Path == "/operations" || r.URL.Path == "/operations.html" {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "operations.html"))
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
	// Root-level product pages that must not be captured by the owner-slug
	// profile route.
	if r.URL.Path == "/repositories" || r.URL.Path == "/repositories.html" {
		http.ServeFile(w, r, filepath.Join(a.StaticDir, "repositories.html"))
		return
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
			demoGuest := false
			if c, err := r.Cookie("switchyard_session"); err == nil {
				if u, ok := a.userForSession(r, c.Value); ok {
					user = u
				}
			}
			if user == "" && a.DemoMode && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				user = "demo"
				demoGuest = true
			}
			if user == "" && a.DemoMode && r.Method != http.MethodGet && r.Method != http.MethodHead && !strings.HasPrefix(r.URL.Path, "/api/auth/") {
				writeJSON(w, 403, map[string]any{"error": "demo_read_only"})
				return
			}
			ctx := contextWithDemoGuest(contextWithUser(r.Context(), user), demoGuest)
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
	if len(token) != 70 || !strings.HasPrefix(token, "sess_") {
		return "", false
	}
	items, err := a.Trestle.ListRecords("sessions", filterEq("token", token))
	if err != nil || len(items) == 0 {
		return "", false
	}
	exp, _ := items[0]["expires_at"].(string)
	if t, err := time.Parse(time.RFC3339, exp); err != nil || time.Now().After(t) {
		return "", false
	}
	u, _ := items[0]["username"].(string)
	record := a.userRecord(u)
	if record == nil || !strings.HasSuffix(token, "_"+sha256Hex([]byte(strOr(record["password_hash"])))[:16]) {
		return "", false
	}
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
