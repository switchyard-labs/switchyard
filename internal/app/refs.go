package app

import (
	"net/http"

	"switchyard/internal/artifacts"
	"switchyard/internal/refs"
)

// handleRepoRefs returns the current refs of a repository (read path for CAS).
func (a *App) handleRepoRefs(w http.ResponseWriter, r *http.Request) {
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
	tok, err := a.Refs.GitToken(name)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	refsMap, err := artifacts.LsRemote(repo.Remote, tok)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"refs": refsMap})
}

// handleRefUpdate is the CAS ref mutation endpoint. The caller supplies an
// expected_sha; the update only succeeds if it is still current.
func (a *App) handleRefUpdate(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Repo     string         `json:"repo"`
		Branch   string         `json:"branch"`
		Expected string         `json:"expected_sha"`
		Changes  []refs.Change  `json:"changes"`
		Message  string         `json:"message"`
	}
	if err := readJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if in.Repo == "" || in.Branch == "" || in.Message == "" {
		writeJSON(w, 400, map[string]any{"error": "repo/branch/message required"})
		return
	}
	res, err := a.Refs.Update(in.Repo, in.Branch, in.Expected, in.Changes, in.Message, "user:"+user)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}
