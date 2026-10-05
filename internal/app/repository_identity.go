package app

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"switchyard/internal/artifacts"
)

var ownerSlugRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)
var repoSlugRE = regexp.MustCompile(`^[a-z0-9._-]{1,100}$`)

var reservedOwnerSlugs = map[string]bool{
	"forgot-password": true, "reset-password": true, "verify-email": true,
	"www": true, "docs": true, "app": true, "admin": true, "static": true,
	"pages": true, "login": true, "signup": true, "register": true, "signin": true, "sy": true,
	"api": true, "assets": true, "settings": true, "organizations": true,
	"work": true, "pulls": true, "workflows": true, "attention": true,
	"repositories": true, "operations": true, "history": true, "profile": true,
	"signin.html": true, "repo.html": true, "work.html": true, "edit.html": true,
}

func normalizeOwnerSlug(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func normalizeRepoSlug(s string) string  { return strings.ToLower(strings.TrimSpace(s)) }

func validOwnerSlug(s string) bool {
	s = normalizeOwnerSlug(s)
	return ownerSlugRE.MatchString(s) && !reservedOwnerSlugs[s] && !strings.HasPrefix(s, "dpl-")
}
func validRepoSlug(s string) bool { return repoSlugRE.MatchString(normalizeRepoSlug(s)) }

func (a *App) ensureOwnerNamespace(slug, ownerType, ownerID string) error {
	slug = normalizeOwnerSlug(slug)
	if !validOwnerSlug(slug) {
		return fmt.Errorf("invalid owner slug")
	}
	items, err := a.Trestle.ListRecords("owner_namespaces", `slug = "`+slug+`"`)
	if err != nil {
		return err
	}
	if len(items) > 0 {
		if items[0]["owner_type"] == ownerType && items[0]["owner_id"] == ownerID {
			return nil
		}
		return fmt.Errorf("owner slug already in use")
	}
	_, _, err = a.Trestle.CreateRecord("owner_namespaces", map[string]any{
		"slug": slug, "owner_type": ownerType, "owner_id": ownerID, "created_at": nowStr(),
	}, "owner-namespace-"+slug)
	return err
}

func (a *App) repositoryMetaByOwner(owner, slug string) map[string]any {
	full := normalizeOwnerSlug(owner) + "/" + normalizeRepoSlug(slug)
	items, err := a.Trestle.ListRecords("repository_meta", filterEq("full_name", full))
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (a *App) repositoryMetaByArtifact(name string) map[string]any {
	items, err := a.Trestle.ListRecords("repository_meta", filterEq("artifact_name", name))
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

func (a *App) handleListRepositoryMeta(w http.ResponseWriter, r *http.Request) {
	items, err := a.Trestle.ListRecords("repository_meta", "")
	items = a.visibleRecords("repository_meta", items, a.currentUser(r))
	if a.isDemoGuest(r) {
		filtered := []map[string]any{}
		for _, it := range items {
			if strOr(it["visibility"]) == "public" {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (a *App) handleRegisterRepositoryMeta(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		ArtifactName string `json:"artifact_name"`
		OwnerType    string `json:"owner_type"`
		Owner        string `json:"owner"`
		Slug         string `json:"slug"`
		DisplayName  string `json:"display_name"`
		Description  string `json:"description"`
		Visibility   string `json:"visibility"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.ArtifactName) == "" {
		writeJSON(w, 400, map[string]any{"error": "artifact_name_required"})
		return
	}
	backend, err := a.Artifacts.GetRepo(in.ArtifactName)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "artifact_repo_not_found"})
		return
	}
	ownerType := strings.ToLower(strings.TrimSpace(in.OwnerType))
	if ownerType == "" {
		ownerType = "user"
	}
	ownerID, ownerSlug := "", ""
	switch ownerType {
	case "user":
		ownerID = user
		if in.Owner != "" && in.Owner != user {
			writeJSON(w, 403, map[string]any{"error": "cannot_register_for_other_user"})
			return
		}
		ownerSlug = normalizeOwnerSlug(user)
	case "org":
		orgs, e := a.Trestle.ListRecords("orgs", "")
		if e != nil {
			writeJSON(w, 502, map[string]any{"error": e.Error()})
			return
		}
		for _, org := range orgs {
			id, _ := org["id"].(string)
			name, _ := org["name"].(string)
			if in.Owner == id || strings.EqualFold(in.Owner, name) {
				if org["owner"] != user {
					writeJSON(w, 403, map[string]any{"error": "org_owner_required"})
					return
				}
				ownerID, ownerSlug = id, normalizeOwnerSlug(name)
				break
			}
		}
		if ownerID == "" {
			writeJSON(w, 404, map[string]any{"error": "org_not_found"})
			return
		}
	default:
		writeJSON(w, 400, map[string]any{"error": "owner_type_invalid"})
		return
	}
	if err := a.ensureOwnerNamespace(ownerSlug, ownerType, ownerID); err != nil {
		writeJSON(w, 409, map[string]any{"error": err.Error()})
		return
	}
	slug := normalizeRepoSlug(in.Slug)
	if slug == "" {
		slug = normalizeRepoSlug(backend.Name)
	}
	if !validRepoSlug(slug) {
		writeJSON(w, 400, map[string]any{"error": "repo_slug_invalid"})
		return
	}
	full := ownerSlug + "/" + slug
	if existing := a.repositoryMetaByOwner(ownerSlug, slug); existing != nil {
		writeJSON(w, 409, map[string]any{"error": "repository_name_taken"})
		return
	}
	if existing := a.repositoryMetaByArtifact(backend.Name); existing != nil {
		writeJSON(w, 409, map[string]any{"error": "artifact_repo_already_registered", "repository": existing})
		return
	}
	visibility := strings.ToLower(strings.TrimSpace(in.Visibility))
	if visibility == "" {
		visibility = "private"
	}
	if visibility != "private" && visibility != "public" && visibility != "internal" {
		writeJSON(w, 400, map[string]any{"error": "visibility_invalid"})
		return
	}
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = slug
	}
	id := "repo_" + randHex(12)
	now := time.Now().UTC().Format(time.RFC3339)
	values := map[string]any{
		"id": id, "full_name": full, "owner_type": ownerType, "owner_id": ownerID,
		"owner_slug": ownerSlug, "slug": slug, "display_name": display,
		"description": strings.TrimSpace(in.Description), "visibility": visibility,
		"default_branch": backend.DefaultBranch, "artifact_name": backend.Name,
		"created_at": now, "updated_at": now,
	}
	if _, _, err := a.Trestle.CreateRecord("repository_meta", values, "repository-meta-"+id); err != nil {
		writeArtifactsError(w, err)
		return
	}
	writeJSON(w, 201, values)
}

func (a *App) resolveCanonicalRepository(w http.ResponseWriter, r *http.Request) (map[string]any, string, bool) {
	owner, slug := r.PathValue("owner"), r.PathValue("repo")
	meta := a.repositoryMetaByOwner(owner, slug)
	if meta == nil {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return nil, "", false
	}
	if !a.canAccessRepository(meta, a.currentUser(r), false) {
		writeJSON(w, 403, map[string]any{"error": "repository_access_denied"})
		return nil, "", false
	}
	artifact, _ := meta["artifact_name"].(string)
	if artifact == "" {
		writeJSON(w, 500, map[string]any{"error": "repository_backend_missing"})
		return nil, "", false
	}
	return meta, artifact, true
}

func (a *App) handleGetRepositoryMeta(w http.ResponseWriter, r *http.Request) {
	meta, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	backend, err := a.Artifacts.GetRepo(artifact)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	out := map[string]any{}
	for k, v := range meta {
		out[k] = v
	}
	out["read_only"] = backend.ReadOnly
	out["source"] = backend.Source
	out["can_write"] = !backend.ReadOnly && a.CanRepository(meta, a.currentUser(r), WriteRepo)
	out["can_admin"] = a.CanRepository(meta, a.currentUser(r), AdminRepo)
	writeJSON(w, 200, out)
}

func (a *App) handleCanonicalRepoTree(w http.ResponseWriter, r *http.Request) {
	_, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "main"
	}
	tree, err := a.gitTree(artifact, ref)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ref": ref, "tree": tree})
}

func (a *App) handleCanonicalRepoContent(w http.ResponseWriter, r *http.Request) {
	_, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "main"
	}
	path := r.URL.Query().Get("path")
	data, err := a.Artifacts.RawFile(artifact, ref, path)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	serveRepositoryContent(w, data)
}

type repositoryRoute struct {
	Owner, Repo, Kind, Ref, Path string
}

func parseRepositoryRoute(p string) (repositoryRoute, bool) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(p), "/"), "/")
	if len(parts) < 2 || !validOwnerSlug(parts[0]) || !validRepoSlug(parts[1]) {
		return repositoryRoute{}, false
	}
	r := repositoryRoute{Owner: parts[0], Repo: parts[1]}
	if len(parts) == 2 {
		return r, true
	}
	if len(parts) == 3 && (parts[2] == "branches" || parts[2] == "tags") {
		r.Kind = parts[2]
		return r, true
	}
	if len(parts) >= 4 && (parts[2] == "blob" || parts[2] == "tree") {
		r.Kind, r.Ref = parts[2], parts[3]
		if len(parts) > 4 {
			r.Path = strings.Join(parts[4:], "/")
		}
		return r, true
	}
	return repositoryRoute{}, false
}

func (a *App) handleCanonicalRepoRefs(w http.ResponseWriter, r *http.Request) {
	_, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	repo, err := a.Artifacts.GetRepo(artifact)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	tok, err := a.Refs.GitToken(artifact)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	refsMap, err := artifacts.LsRemote(repo.Remote, tok)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"refs": refsMap})
}
