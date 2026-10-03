package app

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"switchyard/internal/artifacts"
	"time"
)

// handleRepositoryOverview assembles familiar Git-host repository metadata from
// the canonical Switchyard identity plus immutable Git truth in Artifacts.
func (a *App) handleRepositoryOverview(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	meta, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	coordinationDuration := time.Since(started)
	started = time.Now()
	backend, err := a.Artifacts.GetRepo(artifact)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	metadataDuration := time.Since(started)
	started = time.Now()
	tok, err := a.Refs.GitToken(artifact)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	tokenDuration := time.Since(started)
	started = time.Now()
	refsMap, err := a.Artifacts.LsRemoteWithToken(backend.Remote, tok)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	gitDuration := time.Since(started)
	branches, tags := []string{}, []string{}
	for ref := range refsMap {
		if strings.HasPrefix(ref, "refs/heads/") {
			branches = append(branches, strings.TrimPrefix(ref, "refs/heads/"))
		}
		if strings.HasPrefix(ref, "refs/tags/") && !strings.HasSuffix(ref, "^{}") {
			tags = append(tags, strings.TrimPrefix(ref, "refs/tags/"))
		}
	}
	sort.Strings(branches)
	sort.Strings(tags)
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = strOr(meta["default_branch"])
	}
	started = time.Now()
	var commits []artifacts.Commit
	if len(refsMap) > 0 {
		commits, err = a.Artifacts.Log(artifact, ref, 50)
	}
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	historyDuration := time.Since(started)
	w.Header().Set("Server-Timing", fmt.Sprintf("coordination;dur=%.2f, artifacts_metadata;dur=%.2f, git_token;dur=%.2f, git_refs;dur=%.2f, artifacts_history;dur=%.2f", float64(coordinationDuration)/float64(time.Millisecond), float64(metadataDuration)/float64(time.Millisecond), float64(tokenDuration)/float64(time.Millisecond), float64(gitDuration)/float64(time.Millisecond), float64(historyDuration)/float64(time.Millisecond)))
	var latest any = nil
	if len(commits) > 0 {
		latest = commits[0]
	}
	out := map[string]any{
		"repository":          meta,
		"ref":                 ref,
		"clone_https":         backend.Remote,
		"default_branch":      meta["default_branch"],
		"branch_count":        len(branches),
		"tag_count":           len(tags),
		"branches":            branches,
		"tags":                tags,
		"commit_count_sample": len(commits),
		"latest_commit":       latest,
		"read_only":           backend.ReadOnly,
		"source":              backend.Source,
	}
	writeJSON(w, 200, out)
}

func (a *App) handleRepositoryCommits(w http.ResponseWriter, r *http.Request) {
	_, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	ref := r.URL.Query().Get("ref")
	if ref == "" {
		ref = "main"
	}
	commits, err := a.Artifacts.Log(artifact, ref, 100)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ref": ref, "items": commits})
}

func (a *App) handleRepositoryCommit(w http.ResponseWriter, r *http.Request) {
	_, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	sha := r.PathValue("sha")
	commits, err := a.Artifacts.Log(artifact, sha, 1)
	if err != nil || len(commits) == 0 {
		writeJSON(w, 404, map[string]any{"error": "commit_not_found"})
		return
	}
	prov, _ := a.Trestle.ListRecords("ref_updates", `new_sha = "`+sha+`"`)
	writeJSON(w, 200, map[string]any{"commit": commits[0], "provenance": prov})
}

func (a *App) handleRepositoryCompare(w http.ResponseWriter, r *http.Request) {
	_, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	base, head := r.URL.Query().Get("base"), r.URL.Query().Get("head")
	if base == "" || head == "" {
		writeJSON(w, 400, map[string]any{"error": "base_head_required"})
		return
	}
	diff, files, err := a.Refs.DiffRefs(artifact, base, head)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"base": base, "head": head, "files": files, "diff": diff})
}
