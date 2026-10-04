package refs

import (
	"fmt"
	"strings"
	"time"
)

// PublishRepairTree commits the inspected merged preview onto the source branch.
// Both source and canonical changes remain ancestors, preventing a repair from
// losing the canonical side or merely rewriting an unchanged source file.
func (s *Service) PublishRepairTree(repo, branch, expected, tree, message, provenance string) (*Result, error) {
	if err := validateBranch(branch); err != nil {
		return nil, err
	}
	lock := s.lock(repo, branch)
	lock.Lock()
	defer lock.Unlock()
	mergeHead, err := gitOut(tree, "rev-parse", "MERGE_HEAD")
	if err != nil || strings.TrimSpace(mergeHead) != expected {
		return nil, fmt.Errorf("repair preview source does not match expected SHA")
	}
	if err = git(tree, "", "add", "-A"); err != nil {
		return nil, err
	}
	if err = git(tree, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "commit", "--quiet", "-m", message); err != nil {
		return nil, &StageError{Code: "git_commit_failed", Err: err}
	}
	remote, err := s.remote(repo)
	if err != nil {
		return nil, err
	}
	sha, err := s.push(repo, remote, branch, tree, expected)
	if err != nil {
		return nil, &StageError{Code: "git_push_failed", Err: err}
	}
	if s.Trestle != nil {
		if _, _, err = s.Trestle.CreateRecord("ref_updates", map[string]any{"repo": repo, "branch": branch, "old_sha": expected, "new_sha": sha, "provenance": provenance, "occurred_at": time.Now().UTC().Format(time.RFC3339)}, "refupd-"+repo+"-"+branch+"-"+sha[:12]); err != nil {
			return nil, fmt.Errorf("repair provenance persistence failed: %w", err)
		}
	}
	return &Result{Status: "ok", OldSHA: expected, NewSHA: sha, Ref: "refs/heads/" + branch}, nil
}
