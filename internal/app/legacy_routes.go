package app

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"switchyard/internal/refs"
)

// Resolve physical-name bookmarks through registered identity, never by
// guessing an owner or granting access to account-wide Artifacts repositories.
func (a *App) redirectLegacyRepository(w http.ResponseWriter, r *http.Request) bool {
	section, legacy := map[string]string{"/repo.html": "", "/history.html": "commits", "/repo-settings.html": "settings", "/edit.html": "edit"}[r.URL.Path]
	if !legacy || r.URL.Query().Get("name") == "" {
		return false
	}
	meta := a.repositoryMetaByArtifact(r.URL.Query().Get("name"))
	user := ""
	if cookie, err := r.Cookie(sessionCookieName(r)); err == nil {
		if u, ok := a.userForSession(r, cookie.Value); ok {
			user = u
		}
	}
	if meta == nil || !a.CanRepository(meta, user, ReadRepo) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Repository unavailable · Switchyard</title><link rel="stylesheet" href="/assets/css/theme.css"><link rel="stylesheet" href="/assets/css/components.css"><main class="site-main"><h1>Repository unavailable</h1><p>This bookmark needs a registered repository you can access.</p><a class="btn" href="/repositories">Browse repositories</a> <a class="btn" href="/signin">Sign in</a></main></html>`))
		return true
	}
	target, err := legacyRepositoryTarget(meta, r.URL.Query(), section)
	if err != nil {
		http.Error(w, "Invalid repository bookmark", http.StatusBadRequest)
		return true
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusFound)
	return true
}

func legacyRepositoryTarget(meta map[string]any, query url.Values, section string) (string, error) {
	owner, slug := strOf(meta["owner_slug"]), strOf(meta["slug"])
	if !validOwnerSlug(owner) || !validRepoSlug(slug) {
		return "", fmt.Errorf("invalid repository identity")
	}
	base := "/" + url.PathEscape(owner) + "/" + url.PathEscape(slug)
	ref, path := query.Get("ref"), query.Get("path")
	if ref == "" {
		ref = strOf(meta["default_branch"])
	}
	if ref == "" {
		ref = "main"
	}
	if section == "settings" {
		return base + "/settings", nil
	}
	if section == "commits" {
		return base + "/commits/" + url.PathEscape(ref), nil
	}
	if section == "edit" {
		target := base + "/edit/" + url.PathEscape(ref)
		if path != "" {
			if err := refs.ValidatePath(path); err != nil {
				return "", err
			}
			for _, part := range strings.Split(path, "/") {
				target += "/" + url.PathEscape(part)
			}
		}
		if attempt := query.Get("attempt"); attempt != "" {
			target += "?" + url.Values{"attempt": {attempt}}.Encode()
		}
		return target, nil
	}
	if path != "" {
		if err := refs.ValidatePath(path); err != nil {
			return "", err
		}
		parts := strings.Split(path, "/")
		for i := range parts {
			parts[i] = url.PathEscape(parts[i])
		}
		return base + "/blob/" + url.PathEscape(ref) + "/" + strings.Join(parts, "/"), nil
	}
	if query.Get("ref") != "" {
		return base + "/tree/" + url.PathEscape(ref), nil
	}
	return base, nil
}
