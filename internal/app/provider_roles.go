package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/agent"
	"switchyard/internal/refs"
	"time"
)

func (a *App) requestedProvider(r *http.Request, role string) (context.Context, bool, error) {
	ctx, err := a.agentRequestContext(r, role)
	if err != nil {
		return nil, false, err
	}
	s, err := a.agentSelection(a.currentUser(r), role)
	if err != nil {
		return nil, false, err
	}
	if override, ok := ctx.Value(agentOverrideKey{}).(AgentSelection); ok {
		s = override
	}
	return ctx, s.Provider != "", nil
}
func (a *App) providerReview(ctx context.Context, repo, branch, attempt string) ([]map[string]any, *agent.Execution, error) {
	paths, err := a.changedFiles(repo, branch)
	if err != nil {
		return nil, nil, err
	}
	if len(paths) > 100 {
		return nil, nil, fmt.Errorf("review exceeds 100 changed files")
	}
	head, err := a.repoHead(repo, branch)
	if err != nil {
		return nil, nil, err
	}
	files := map[string]string{}
	total := 0
	for _, path := range paths {
		data, err := a.Artifacts.RawFile(repo, head, path)
		if err != nil {
			return nil, nil, err
		}
		total += len(data)
		if total > 1<<20 {
			return nil, nil, fmt.Errorf("review input exceeds 1 MiB")
		}
		files[path] = string(data)
	}
	input, _ := json.Marshal(files)
	ex, err := a.runViaSubstrateContext(ctx, "reviewer", attempt, repo, branch, "REVIEW.json", string(input), `Review these files. Return files.REVIEW.json containing a JSON array of objects with severity (info/warning/error), message, file. Do not modify repository files.`)
	if err != nil {
		return nil, nil, err
	}
	var findings []map[string]any
	if err = json.Unmarshal([]byte(ex.Result["REVIEW.json"]), &findings); err != nil {
		return nil, nil, fmt.Errorf("reviewer returned invalid findings")
	}
	if len(findings) > 100 {
		return nil, nil, fmt.Errorf("too many review findings")
	}
	for _, f := range findings {
		severity := strOr(f["severity"])
		path := strOr(f["file"])
		message := strOr(f["message"])
		if (severity != "info" && severity != "warning" && severity != "error") || message == "" || len(message) > 4000 {
			return nil, nil, fmt.Errorf("invalid review finding")
		}
		if path != "" {
			if _, ok := files[path]; !ok {
				return nil, nil, fmt.Errorf("review finding outside reviewed files")
			}
		}
	}
	return findings, ex, nil
}
func (a *App) providerConflict(ctx context.Context, repo, base, branch, attempt, user string) (*refs.Result, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	head, err := a.repoHead(repo, branch)
	if err != nil {
		return nil, nil, err
	}
	tree, conflicts, err := a.Refs.PreviewConflictTree(repo, base, branch)
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tree)
	if len(conflicts) > 20 {
		return nil, nil, fmt.Errorf("repair exceeds 20 conflict files")
	}
	changes := []refs.Change{}
	total := 0
	for _, path := range conflicts {
		if err := refs.ValidatePath(path); err != nil {
			return nil, nil, err
		}
		absolute := filepath.Join(tree, filepath.FromSlash(path))
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil || !strings.HasPrefix(resolved, tree+string(os.PathSeparator)) {
			return nil, nil, fmt.Errorf("conflict path escapes preview")
		}
		info, err := os.Lstat(absolute)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return nil, nil, fmt.Errorf("conflict input is not a bounded regular file")
		}
		total += int(info.Size())
		if total > 2<<20 {
			return nil, nil, fmt.Errorf("repair input exceeds 2 MiB")
		}
		bytes, err := os.ReadFile(absolute)
		if err != nil {
			return nil, nil, err
		}
		ex, err := a.runViaSubstrateContext(ctx, "conflict-resolver", attempt, repo, branch, path, string(bytes), "Resolve the merge conflict markers in this file. Return only this file. Preserve the intended changes on both sides.")
		if err != nil {
			return nil, nil, err
		}
		content, ok := ex.Result[path]
		if !ok || len(ex.Result) != 1 || strings.Contains(content, "<<<<<<<") || strings.Contains(content, ">>>>>>>") {
			return nil, nil, fmt.Errorf("resolver did not return one resolved file")
		}
		changes = append(changes, refs.Change{Path: path, Content: content})
	}
	if len(changes) == 0 {
		return nil, nil, fmt.Errorf("no structural conflicts to resolve")
	}
	result, err := a.Refs.Update(repo, branch, head, changes, "provider conflict repair "+attempt, "conflict-resolver:"+user+":"+attempt)
	return result, conflicts, err
}
