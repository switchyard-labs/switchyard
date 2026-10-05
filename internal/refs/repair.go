package refs

import (
	"fmt"
	"strings"
)

// PublishRepairTree commits the inspected merged preview onto the source branch.
// Both source and canonical changes remain ancestors, preventing a repair from
// losing the canonical side or merely rewriting an unchanged source file.
func (s *Service) PublishRepairTree(repo, branch, expected, tree, message, provenance string) (*Result, error) {
	if err := validateBranch(branch); err != nil {
		return nil, err
	}
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
	head, err := gitOut(tree, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	treeSHA, err := gitOut(tree, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return nil, err
	}
	candidate := &PreparedMerge{Repo: repo, Base: branch, BaseSHA: expected, CommitSHA: strings.TrimSpace(head), TreeSHA: strings.TrimSpace(treeSHA), Dir: tree, remote: remote}
	return s.PublishPreparedTracked(candidate, provenance)
}
