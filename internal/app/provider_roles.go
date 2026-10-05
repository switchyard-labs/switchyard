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

type agentSourceSHAKey struct{}

type providerExecutionError struct {
	execution *agent.Execution
	cause     error
}

func (e *providerExecutionError) Error() string { return "provider execution failed" }
func (e *providerExecutionError) Unwrap() error { return e.cause }

func (a *App) providerReview(ctx context.Context, repo, branch, attempt string) ([]map[string]any, *agent.Execution, error) {
	head, err := a.repoHead(repo, branch)
	if err != nil {
		return nil, nil, err
	}
	ctx = context.WithValue(ctx, agentSourceSHAKey{}, head)
	paths, err := a.changedFiles(repo, head)
	if err != nil {
		return nil, nil, err
	}
	if len(paths) > 100 {
		return nil, nil, fmt.Errorf("review exceeds 100 changed files")
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
	// Include bounded contract counterparts at the same immutable review SHA.
	// They may be unchanged relative to main, but are needed to judge compatibility.
	contract, contractErr := a.Artifacts.RawFile(repo, head, "switchyard.contract.json")
	if contractErr == nil {
		contextPaths, err := reviewContractPaths(contract)
		if err != nil {
			return nil, nil, err
		}
		for _, path := range contextPaths {
			if _, present := files[path]; present {
				continue
			}
			data, err := a.Artifacts.RawFile(repo, head, path)
			if err != nil {
				return nil, nil, fmt.Errorf("review contract counterpart unavailable")
			}
			total += len(data)
			if total > 1<<20 {
				return nil, nil, fmt.Errorf("review input exceeds 1 MiB")
			}
			files[path] = string(data)
		}
	}
	input, _ := json.Marshal(files)
	ex, err := a.runViaSubstrateContext(ctx, "reviewer", attempt, repo, branch, "REVIEW.json", string(input), `Review the supplied JSON map of file paths to complete contents, including unchanged contract counterparts. All review inputs are in this prompt; do not try to read repository paths using tools. Return files.REVIEW.json containing a JSON array of objects with severity (info/warning/error), message, file. Do not modify repository files.`)
	if err != nil {
		return nil, ex, err
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
	for _, finding := range findings {
		finding["source_sha"] = head
	}
	return findings, ex, nil
}

// reviewContractPaths bounds and validates context references before remote reads.
func reviewContractPaths(contract []byte) ([]string, error) {
	var spec struct {
		Rules []contractRule `json:"rules"`
	}
	if len(contract) > 64<<10 || json.Unmarshal(contract, &spec) != nil || len(spec.Rules) > 100 {
		return nil, fmt.Errorf("invalid or oversized review contract")
	}
	paths := []string{"switchyard.contract.json"}
	seen := map[string]bool{"switchyard.contract.json": true}
	for _, rule := range spec.Rules {
		for _, selector := range []string{rule.A, rule.B} {
			parts := strings.SplitN(selector, ":", 2)
			path := parts[0]
			if len(parts) != 2 || parts[1] == "" || path == "" || filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, "../") || strings.ContainsAny(path, "\\\x00") {
				return nil, fmt.Errorf("unsafe review contract path")
			}
			if !seen[path] {
				paths = append(paths, path)
				seen[path] = true
			}
		}
	}
	return paths, nil
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
	keepCandidate := false
	defer func() {
		if !keepCandidate {
			os.RemoveAll(tree)
		}
	}()
	if len(conflicts) > 20 {
		return nil, nil, fmt.Errorf("repair exceeds 20 conflict files")
	}
	ctx = context.WithValue(ctx, agentSourceSHAKey{}, head)
	semantic := len(conflicts) == 0
	prompt := "Resolve the merge conflict markers in this file. Return only this file. Preserve the intended changes on both sides."
	if semantic {
		conflicts, prompt, err = semanticRepairInputs(tree)
		if err != nil {
			return nil, nil, err
		}
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
		ex, err := a.runViaSubstrateContext(ctx, "conflict-resolver", attempt, repo, branch, path, string(bytes), prompt)
		if err != nil {
			return nil, nil, &providerExecutionError{execution: ex, cause: err}
		}
		content, validationErr := validateResolverFile(ex.Result, path)
		if validationErr != nil {
			return nil, nil, validationErr
		}
		if err = os.WriteFile(absolute, []byte(content), 0600); err != nil {
			return nil, nil, err
		}
		changes = append(changes, refs.Change{Path: path, Content: content})
	}
	if len(changes) == 0 {
		return nil, nil, fmt.Errorf("no structural conflicts to resolve")
	}
	if semantic {
		findings, validationErr := semanticFindingsInTree(tree)
		if validationErr != nil {
			return nil, nil, validationErr
		}
		if len(findings) > 0 {
			return nil, nil, fmt.Errorf("resolver proposal still violates semantic contract")
		}
	}
	result, err := a.publishRepair(publicationAuthority{principal: user, role: "conflict-resolver", kind: publicationAgent}, repo, branch, head, tree, "provider conflict repair "+attempt, "conflict-resolver:"+user+":"+attempt)
	if err != nil {
		keepCandidate = true
	}
	return result, conflicts, err
}
