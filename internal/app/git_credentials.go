package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Credentials are returned once, never persisted. Audit records contain only
// actor, repository, scope and expiry. Authentication is required even for
// public repositories; anonymous browsing does not grant credential issuance.
func (a *App) handleGitCredential(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	user := a.currentUser(r)
	if user == "" || user == "demo" {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	var in struct {
		Scope string `json:"scope"`
		TTL   int    `json:"ttl_seconds"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if in.Scope == "" {
		in.Scope = "read"
	}
	if in.TTL == 0 {
		in.TTL = 600
	}
	if (in.Scope != "read" && in.Scope != "write") || in.TTL < 60 || in.TTL > 900 {
		writeJSON(w, 400, map[string]any{"error": "credential_scope_or_ttl_invalid"})
		return
	}
	meta, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	if in.Scope == "write" {
		if !a.CanRepository(meta, user, WriteRepo) {
			writeJSON(w, 403, map[string]any{"error": "repository_access_denied"})
			return
		}
		settings, err := a.Trestle.ListRecords("repository_settings", filterEq("repo_id", strOr(meta["id"])))
		if err != nil {
			writeJSON(w, 503, map[string]any{"error": "repository_policy_unavailable"})
			return
		}
		for _, setting := range settings {
			if strOr(setting["archived"]) == "true" {
				writeJSON(w, 403, map[string]any{"error": "repository_archived"})
				return
			}
		}
		protected, err := a.Trestle.ListRecords("repo_protected_refs", filterEq("repo_id", strOr(meta["id"])))
		if err != nil {
			writeJSON(w, 503, map[string]any{"error": "repository_policy_unavailable"})
			return
		}
		// Artifacts write tokens are repository-wide, not ref-scoped. Do not hand
		// out a token that could bypass this server's protected-ref enforcement.
		if len(protected) > 0 {
			writeJSON(w, 409, map[string]any{"error": "direct_write_credential_blocked_by_protected_refs"})
			return
		}
	}
	backend, err := a.Artifacts.GetRepo(artifact)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	if in.Scope == "write" && backend.ReadOnly {
		writeJSON(w, 403, map[string]any{"error": "repository_read_only"})
		return
	}
	remote, err := url.Parse(backend.Remote)
	if err != nil || remote.Scheme != "https" || remote.User != nil || remote.RawQuery != "" || remote.Fragment != "" || remote.Host != a.Artifacts.AccountID+".artifacts.cloudflare.net" || remote.Path != "/git/"+a.Artifacts.Namespace+"/"+artifact+".git" {
		writeJSON(w, 502, map[string]any{"error": "repository_remote_invalid"})
		return
	}
	token, err := a.Artifacts.MintToken(artifact, in.Scope, in.TTL)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	secret, suffix, found := strings.Cut(token, "?expires=")
	seconds, parseErr := strconv.ParseInt(suffix, 10, 64)
	expiry := time.Unix(seconds, 0).UTC()
	// The secret is opaque: provider token versions can change. Validate the
	// transport boundary and expiry rather than assuming a historical prefix.
	if !found || len(secret) < 20 || len(secret) > 512 || strings.ContainsAny(token, "\r\n\t ") || parseErr != nil || !expiry.After(time.Now()) || expiry.After(time.Now().Add(time.Duration(in.TTL+30)*time.Second)) {
		writeJSON(w, 502, map[string]any{"error": "credential_expiry_invalid"})
		return
	}
	org := ""
	if strOr(meta["owner_type"]) == "org" {
		org = strOr(meta["owner_id"])
	}
	_, _, err = a.Trestle.CreateRecord("audit", map[string]any{"id": "aud_" + randHex(10), "org": org, "action": "git_credential_issued", "subject": strOr(meta["full_name"]), "decision": "allow", "reason": fmt.Sprintf("scope=%s expires_at=%s", in.Scope, expiry.Format(time.RFC3339)), "actor": user, "at": nowStr()}, "git-credential-"+randHex(10))
	if err != nil {
		writeJSON(w, 503, map[string]any{"error": "credential_audit_unavailable"})
		return
	}
	permissions := []string{"read"}
	if in.Scope == "write" {
		permissions = append(permissions, "write")
	}
	writeJSON(w, 200, map[string]any{"remote": backend.Remote, "token": token, "expires_at": expiry.Format(time.RFC3339), "permissions": permissions, "repository": strOr(meta["full_name"])})
}
