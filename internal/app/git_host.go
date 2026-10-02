package app

import (
	"net/http"
	"sort"
	"strings"
)

// handleRepositoryOverview assembles familiar Git-host repository metadata from
// the canonical Switchyard identity plus immutable Git truth in Artifacts.
func (a *App) handleRepositoryOverview(w http.ResponseWriter, r *http.Request) {
	meta, artifact, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	backend, err := a.Artifacts.GetRepo(artifact)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	tok, err := a.Refs.GitToken(artifact)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	refsMap, err := a.Artifacts.LsRemoteWithToken(backend.Remote, tok)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
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
	commits, _ := a.Artifacts.Log(artifact, strOr(meta["default_branch"]), 50)
	var latest any = nil
	if len(commits) > 0 {
		latest = commits[0]
	}
	out := map[string]any{
		"repository":          meta,
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
