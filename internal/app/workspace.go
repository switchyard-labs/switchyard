package app

import (
	"context"
	"fmt"
	coreeditor "github.com/gantry-tools/gantry-core/editor"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"switchyard/internal/refs"
	"time"
	"unicode/utf8"
)

var workspaceSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

type workspaceEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
}

func (a *App) workspaceTree(ctx context.Context, repo, sha string) ([]workspaceEntry, error) {
	commits, err := a.Artifacts.LogContext(ctx, repo, sha)
	if err != nil {
		return nil, err
	}
	if len(commits) != 1 || commits[0].Hash != sha {
		return nil, fmt.Errorf("immutable commit unavailable")
	}
	entries := []workspaceEntry{}
	trees := 0
	var walk func(string, string, int) error
	walk = func(hash, prefix string, depth int) error {
		if depth > 32 || trees >= 1000 {
			return fmt.Errorf("tree limit exceeded")
		}
		trees++
		children, err := a.Artifacts.TreeContext(ctx, repo, hash)
		if err != nil {
			return err
		}
		for _, child := range children {
			path := prefix + child.Name
			if refs.ValidatePath(path) != nil {
				return fmt.Errorf("invalid tree path")
			}
			if child.Type == "tree" {
				if err := walk(child.Hash, path+"/", depth+1); err != nil {
					return err
				}
			} else {
				entries = append(entries, workspaceEntry{path, child.Mode, child.Type})
				if len(entries) > 10000 {
					return fmt.Errorf("file limit exceeded")
				}
			}
		}
		return nil
	}
	if err = walk(commits[0].TreeHash, "", 0); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}
func regularWorkspaceFile(entry workspaceEntry) bool {
	return entry.Type == "blob" && (entry.Mode == "100644" || entry.Mode == "100755")
}

type workspaceMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Preview string `json:"preview"`
	Kind    string `json:"kind"`
}

func (a *App) handleWorkspaceSearch(w http.ResponseWriter, r *http.Request) {
	_, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	sha, query := r.URL.Query().Get("sha"), r.URL.Query().Get("q")
	if !workspaceSHA.MatchString(sha) || len(query) > 256 || query == "" {
		writeJSON(w, 400, map[string]any{"error": "valid_sha_query_required"})
		return
	}
	re, err := coreeditor.Compile(coreeditor.Query{Pattern: query, Regex: r.URL.Query().Get("regex") == "1", CaseSensitive: r.URL.Query().Get("case") == "1"})
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_search_pattern"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	entries, err := a.workspaceTree(ctx, repo, sha)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "search_snapshot_unavailable"})
		return
	}
	matches := []workspaceMatch{}
	scanned, skipped, total := 0, 0, 0
	truncated := false
	for _, entry := range entries {
		if len(matches) >= 200 || scanned >= 100 || total >= 20<<20 || ctx.Err() != nil {
			truncated = true
			break
		}
		if re.MatchString(entry.Path) {
			matches = append(matches, workspaceMatch{entry.Path, 0, 0, entry.Path, "filename"})
		}
		if !regularWorkspaceFile(entry) {
			skipped++
			continue
		}
		scanned++
		data, err := a.Artifacts.ReadBounded(ctx, repo, sha, entry.Path, 1<<20)
		if err != nil {
			skipped++
			continue
		}
		total += len(data)
		if coreeditor.LooksBinary(data) || !utf8.Valid(data) {
			skipped++
			continue
		}
		for number, line := range strings.Split(string(data), "\n") {
			index := re.FindStringIndex(line)
			if index == nil {
				continue
			}
			column := utf8.RuneCountInString(line[:index[0]]) + 1
			preview := []rune(line)
			if len(preview) > 300 {
				start := column - 81
				if start < 0 {
					start = 0
				}
				if start+300 > len(preview) {
					start = len(preview) - 300
				}
				preview = preview[start : start+300]
			}
			matches = append(matches, workspaceMatch{entry.Path, number + 1, column, string(preview), "content"})
			if len(matches) >= 200 {
				truncated = true
				break
			}
		}
	}
	writeJSON(w, 200, map[string]any{"sha": sha, "items": matches, "scanned_files": scanned, "skipped_files": skipped, "truncated": truncated})
}
func pathWithin(path, root string) bool { return path == root || strings.HasPrefix(path, root+"/") }
func workspaceMovePlan(entries []workspaceEntry, source, target, operation string) ([]refs.Change, error) {
	if refs.ValidatePath(source) != nil {
		return nil, fmt.Errorf("invalid path")
	}
	if operation != "move" && operation != "delete" {
		return nil, fmt.Errorf("invalid operation")
	}
	if operation == "move" && (refs.ValidatePath(target) != nil || pathWithin(target, source) || pathWithin(source, target)) {
		return nil, fmt.Errorf("invalid destination")
	}
	selected := []refs.Change{}
	existing := map[string]bool{}
	for _, entry := range entries {
		existing[entry.Path] = true
		if pathWithin(entry.Path, source) {
			if !regularWorkspaceFile(entry) {
				return nil, fmt.Errorf("unsupported Git entry")
			}
			selected = append(selected, refs.Change{Path: entry.Path, Delete: true})
		}
	}
	if len(selected) == 0 || len(selected) > 100 {
		return nil, fmt.Errorf("operation file limit or missing source")
	}
	if operation == "move" {
		for _, entry := range entries {
			if pathWithin(entry.Path, target) || pathWithin(target, entry.Path) {
				return nil, fmt.Errorf("destination exists")
			}
		}
		for _, change := range selected {
			destination := target + strings.TrimPrefix(change.Path, source)
			if refs.ValidatePath(destination) != nil || existing[destination] {
				return nil, fmt.Errorf("destination exists")
			}
		}
	}
	return selected, nil
}
func (a *App) handleWorkspaceMutation(w http.ResponseWriter, r *http.Request) {
	meta, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	var in struct {
		Branch      string `json:"branch"`
		ExpectedSHA string `json:"expected_sha"`
		Source      string `json:"source"`
		Target      string `json:"target"`
		Operation   string `json:"operation"`
		Message     string `json:"message"`
	}
	if readJSON(r, &in) != nil || !workspaceSHA.MatchString(in.ExpectedSHA) || strings.TrimSpace(in.Message) == "" {
		writeJSON(w, 400, map[string]any{"error": "explicit_commit_required"})
		return
	}
	if !a.allowDirectWorkspaceWrite(w, meta, in.Branch) {
		return
	}
	head, err := a.repoHead(repo, in.Branch)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "mutation_head_unavailable"})
		return
	}
	if head != in.ExpectedSHA {
		writeJSON(w, 409, map[string]any{"error": "branch_moved", "current_sha": head})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	entries, err := a.workspaceTree(ctx, repo, in.ExpectedSHA)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "mutation_snapshot_unavailable"})
		return
	}
	changes, err := workspaceMovePlan(entries, in.Source, in.Target, in.Operation)
	if err != nil {
		writeJSON(w, 422, map[string]any{"error": "invalid_file_operation"})
		return
	}
	drafts, err := a.Trestle.ListRecords("drafts", filterEq("repo", repo))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "draft_read_failed"})
		return
	}
	for _, draft := range drafts {
		if strOr(draft["branch"]) == in.Branch && strOr(draft["committed_at"]) == "" && (pathWithin(strOr(draft["path"]), in.Source) || (in.Operation == "move" && pathWithin(strOr(draft["path"]), in.Target))) {
			writeJSON(w, 409, map[string]any{"error": "affected_draft_present"})
			return
		}
	}
	if in.Operation == "move" {
		original := append([]refs.Change(nil), changes...)
		total := 0
		for _, change := range original {
			data, err := a.Artifacts.ReadBounded(ctx, repo, in.ExpectedSHA, change.Path, 2<<20)
			total += len(data)
			if err != nil || total > 4<<20 || !utf8.Valid(data) || coreeditor.LooksBinary(data) {
				writeJSON(w, 422, map[string]any{"error": "move_content_limit"})
				return
			}
			mode := "100644"
			for _, entry := range entries {
				if entry.Path == change.Path {
					mode = entry.Mode
					break
				}
			}
			changes = append(changes, refs.Change{Path: in.Target + strings.TrimPrefix(change.Path, in.Source), Content: string(data), Mode: mode})
		}
	}
	result, err := a.publishUpdate(publicationAuthority{principal: a.currentUser(r)}, repo, in.Branch, in.ExpectedSHA, changes, in.Message, "browser-file-operation:"+a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "file_operation_failed"})
		return
	}
	if result.Status != "ok" {
		writeJSON(w, 409, map[string]any{"error": "branch_moved", "current_sha": result.NewSHA})
		return
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "new_sha": result.NewSHA, "affected_files": len(changes), "drafts_preserved": true})
}

// Browser commits must respect archival and protected-ref rules, including
// the legacy direct-ref and draft commit surfaces.
func (a *App) allowDirectWorkspaceWrite(w http.ResponseWriter, meta map[string]any, branch string) bool {
	status, code := a.directPublicationPolicy(meta, branch)
	if status != 0 {
		writeJSON(w, status, map[string]any{"error": code})
		return false
	}
	return true
}
func (a *App) directPublicationPolicy(meta map[string]any, branch string) (int, string) {
	if meta == nil {
		return 403, "repository_write_denied"
	}
	_, _, settings, err := a.Trestle.FindRecord("repository_settings", filterEq("repo_id", strOr(meta["id"])))
	if err != nil {
		return 502, "write_policy_unavailable"
	}
	if strOr(settings["archived"]) == "true" {
		return 403, "repository_archived"
	}
	rules, err := a.Trestle.ListRecords("repo_protected_refs", filterEq("repo_id", strOr(meta["id"])))
	if err != nil {
		return 502, "write_policy_unavailable"
	}
	for _, rule := range rules {
		pattern := strOr(rule["pattern"])
		first, e1 := filepath.Match(pattern, branch)
		second, e2 := filepath.Match(pattern, "refs/heads/"+branch)
		if e1 != nil || e2 != nil {
			return 502, "write_policy_unavailable"
		}
		if (first || second) && (strOr(rule["require_pr"]) == "true" || strOr(rule["require_queue"]) == "true") {
			return 403, "protected_ref_requires_review"
		}
	}
	return 0, ""
}

func (a *App) handleWorkspaceContent(w http.ResponseWriter, r *http.Request) {
	_, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	sha, path := r.URL.Query().Get("sha"), r.URL.Query().Get("path")
	if !workspaceSHA.MatchString(sha) || refs.ValidatePath(path) != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_snapshot_path"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	data, err := a.Artifacts.ReadBounded(ctx, repo, sha, path, 2<<20)
	if err != nil {
		writeJSON(w, 422, map[string]any{"error": "file_unavailable_or_too_large"})
		return
	}
	if !utf8.Valid(data) || coreeditor.LooksBinary(data) {
		writeJSON(w, 422, map[string]any{"error": "binary_file_not_editable"})
		return
	}
	serveRepositoryContent(w, data)
}

func (a *App) handleWorkspaceTree(w http.ResponseWriter, r *http.Request) {
	_, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	sha := r.URL.Query().Get("sha")
	if !workspaceSHA.MatchString(sha) {
		writeJSON(w, 400, map[string]any{"error": "immutable_sha_required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	entries, err := a.workspaceTree(ctx, repo, sha)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "workspace_tree_unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"sha": sha, "tree": entries})
}
