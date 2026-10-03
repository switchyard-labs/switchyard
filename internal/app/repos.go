package app

import (
	"net/http"
	"strings"
)

func (a *App) handleListRepos(w http.ResponseWriter, r *http.Request) {
	if a.isDemoGuest(r) {
		writeJSON(w, 403, map[string]any{"error": "demo_use_canonical_public_repositories"})
		return
	}
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	repos, err := a.Artifacts.ListRepos()
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	// merge registration metadata from the coordination registry
	regs := map[string]string{}
	if items, err := a.Trestle.ListRecords("repos", ""); err == nil {
		for _, it := range items {
			if n, _ := it["name"].(string); n != "" {
				regs[n], _ = it["registered_at"].(string)
			}
		}
	}
	out := []map[string]any{}
	for _, repo := range repos {
		if !a.repoAccess(repo.Name, a.currentUser(r), ReadRepo) {
			continue
		}
		out = append(out, map[string]any{
			"name": repo.Name, "default_branch": repo.DefaultBranch,
			"remote": repo.Remote, "read_only": repo.ReadOnly,
			"registered": regs[repo.Name] != "", "registered_at": regs[repo.Name],
		})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	if a.isDemoGuest(r) {
		writeJSON(w, 403, map[string]any{"error": "demo_legacy_repository_routes_disabled"})
		return
	}
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	name := r.PathValue("name")
	repo, err := a.Artifacts.GetRepo(name)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "repo_not_found"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"name": repo.Name, "default_branch": repo.DefaultBranch, "remote": repo.Remote,
		"read_only": repo.ReadOnly, "source": repo.Source,
	})
}

// handleRepoTree returns the file tree (via git ls-tree over the git protocol,
// which is the honest read path for a branch). Falls back to the commit log if
// the branch is empty.
func (a *App) handleRepoTree(w http.ResponseWriter, r *http.Request) {
	if a.isDemoGuest(r) {
		writeJSON(w, 403, map[string]any{"error": "demo_legacy_repository_routes_disabled"})
		return
	}
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	name := r.PathValue("name")
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "main"
	}
	tree, err := a.gitTree(name, ref)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ref": ref, "tree": tree})
}

func (a *App) gitTree(name, ref string) ([]map[string]any, error) {
	commits, err := a.Artifacts.Log(name, ref, 1)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return []map[string]any{}, nil
	}
	files := []map[string]any{}
	var walk func(treeHash, prefix string) error
	walk = func(treeHash, prefix string) error {
		entries, err := a.Artifacts.Tree(name, treeHash)
		if err != nil {
			return err
		}
		for _, e := range entries {
			p := e.Name
			if prefix != "" {
				p = prefix + "/" + e.Name
			}
			if e.Type == "tree" {
				if err := walk(e.Hash, p); err != nil {
					return err
				}
			} else {
				files = append(files, map[string]any{"path": p, "type": e.Type})
			}
		}
		return nil
	}
	if err := walk(commits[0].TreeHash, ""); err != nil {
		return nil, err
	}
	return files, nil
}

// handleRepoContent returns a file's bytes at ref/path.
func (a *App) handleRepoContent(w http.ResponseWriter, r *http.Request) {
	if a.isDemoGuest(r) {
		writeJSON(w, 403, map[string]any{"error": "demo_legacy_repository_routes_disabled"})
		return
	}
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	name := r.PathValue("name")
	ref := r.URL.Query().Get("ref")
	path := r.URL.Query().Get("path")
	if ref == "" {
		ref = "main"
	}
	data, err := a.Artifacts.RawFile(name, ref, path)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": strings.TrimPrefix(err.Error(), "raw ")})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Write(data)
}
