package refs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"switchyard/internal/artifacts"
)

var shaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

func validateBranch(branch string) error {
	if branch == "" || strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, "\x00\r\n ~^:?*[\\") || strings.Contains(branch, "..") || strings.HasSuffix(branch, "/") || strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("invalid branch")
	}
	return git("", "", "check-ref-format", "refs/heads/"+branch)
}

// pushExpected preserves fast-forward-only publication and checks the exact
// advertised old object in pre-push. Git receive-pack then performs its native
// old-object CAS. No force option is used, even if the remote was rewound.
func pushExpected(dir, remote, branch, expected string, auth []string) (string, error) {
	if err := validateBranch(branch); err != nil {
		return "", err
	}
	if expected != "" && !shaRE.MatchString(expected) {
		return "", fmt.Errorf("invalid expected SHA")
	}
	head, err := gitOut(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	head = strings.TrimSpace(head)
	if expected != "" {
		if err := git(dir, "", "merge-base", "--is-ancestor", expected, head); err != nil {
			return "", fmt.Errorf("publication would rewrite history")
		}
	}
	hooks, err := os.MkdirTemp("", "switchyard-push-hooks-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(hooks)
	script := `#!/bin/sh
set -eu
count=0
while read -r local_ref local_sha remote_ref remote_sha; do
 count=$((count+1))
 test "$remote_ref" = "$SWITCHYARD_EXPECTED_REF" || exit 1
 test "$remote_sha" = "$SWITCHYARD_EXPECTED_SHA" || { echo 'expected SHA changed; publication rejected' >&2; exit 1; }
done
test "$count" = 1 || { echo 'no exact ref update was offered' >&2; exit 1; }
`
	if err := os.WriteFile(filepath.Join(hooks, "pre-push"), []byte(script), 0700); err != nil {
		return "", err
	}
	old := expected
	if old == "" {
		old = strings.Repeat("0", 40)
	}
	args := append([]string{}, auth...)
	args = append(args, "-c", "core.hooksPath="+hooks, "push", "--quiet", remote, "HEAD:refs/heads/"+branch)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "SWITCHYARD_EXPECTED_REF=refs/heads/"+branch, "SWITCHYARD_EXPECTED_SHA="+old)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("exact-SHA push rejected: %s", strings.TrimSpace(string(out)))
	}
	return head, nil
}

// PreparedMerge owns an immutable candidate. Every validator reads this same
// tree. Call Close after publishing or persisting/reconciling the candidate.
type PreparedMerge struct {
	Repo      string `json:"repo"`
	Base      string `json:"base"`
	Source    string `json:"source"`
	BaseSHA   string `json:"base_sha"`
	SourceSHA string `json:"source_sha"`
	CommitSHA string `json:"commit_sha"`
	TreeSHA   string `json:"tree_sha"`
	Dir       string `json:"-"`
	remote    string
}

func (p *PreparedMerge) Close() {
	if p.Dir != "" {
		os.RemoveAll(p.Dir)
	}
}
func (s *Service) Snapshot(repo, base, source string) (string, string, error) {
	if err := validateBranch(base); err != nil {
		return "", "", err
	}
	if err := validateBranch(source); err != nil {
		return "", "", err
	}
	remote, err := s.remote(repo)
	if err != nil {
		return "", "", err
	}
	token, err := s.repoToken(repo)
	if err != nil {
		return "", "", err
	}
	refs, err := artifacts.LsRemote(remote, token)
	if err != nil {
		return "", "", err
	}
	b, h := refs["refs/heads/"+base], refs["refs/heads/"+source]
	if !shaRE.MatchString(b) || !shaRE.MatchString(h) {
		return "", "", fmt.Errorf("base or source does not exist")
	}
	return b, h, nil
}
func (s *Service) PrepareMerge(repo, base, source, message string) (*PreparedMerge, error) {
	b, h, err := s.Snapshot(repo, base, source)
	if err != nil {
		return nil, err
	}
	remote, err := s.remote(repo)
	if err != nil {
		return nil, err
	}
	auth, err := s.authArgs(repo)
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(s.ScratchDir, "candidate-*")
	if err != nil {
		return nil, err
	}
	p := &PreparedMerge{Repo: repo, Base: base, Source: source, BaseSHA: b, SourceSHA: h, Dir: dir, remote: remote}
	success := false
	defer func() {
		if !success {
			p.Close()
		}
	}()
	args := append([]string{}, auth...)
	args = append(args, "clone", "--quiet", "--no-checkout", remote, dir)
	if err := git("", "", args...); err != nil {
		return nil, err
	}
	fetch := append([]string{}, auth...)
	fetch = append(fetch, "fetch", "--quiet", remote, b, h)
	if err := git(dir, "", fetch...); err != nil {
		return nil, err
	}
	if err := git(dir, "", "checkout", "--quiet", "--detach", b); err != nil {
		return nil, err
	}
	if err := git(dir, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "merge", "--no-ff", "--quiet", "-m", message, h); err != nil {
		return nil, fmt.Errorf("preview merge failed: %w", err)
	}
	commit, err := gitOut(dir, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	tree, err := gitOut(dir, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return nil, err
	}
	p.CommitSHA = strings.TrimSpace(commit)
	p.TreeSHA = strings.TrimSpace(tree)
	success = true
	return p, nil
}
func (s *Service) PublishPrepared(p *PreparedMerge) (*Result, error) {
	l := s.lock(p.Repo, p.Base)
	l.Lock()
	defer l.Unlock()
	b, h, err := s.Snapshot(p.Repo, p.Base, p.Source)
	if err != nil {
		return nil, err
	}
	if b != p.BaseSHA || h != p.SourceSHA {
		return &Result{Status: "stale", OldSHA: p.BaseSHA, NewSHA: b, Ref: "refs/heads/" + p.Base}, nil
	}
	tree, err := gitOut(p.Dir, "rev-parse", "HEAD^{tree}")
	if err != nil || strings.TrimSpace(tree) != p.TreeSHA {
		return nil, fmt.Errorf("validated candidate tree changed")
	}
	head, err := gitOut(p.Dir, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(head) != p.CommitSHA {
		return nil, fmt.Errorf("validated candidate commit changed")
	}
	auth, err := s.authArgs(p.Repo)
	if err != nil {
		return nil, err
	}
	if err := git(p.Dir, "", "diff", "--quiet", "HEAD", "--"); err != nil {
		return nil, fmt.Errorf("validated candidate working tree changed")
	}
	untracked, err := gitOut(p.Dir, "ls-files", "--others", "--exclude-standard")
	if err != nil || strings.TrimSpace(untracked) != "" {
		return nil, fmt.Errorf("validated candidate has untracked files")
	}
	if p.CommitSHA == p.BaseSHA {
		return &Result{Status: "ok", OldSHA: p.BaseSHA, NewSHA: p.CommitSHA, Ref: "refs/heads/" + p.Base}, nil
	}
	sha, err := pushExpected(p.Dir, p.remote, p.Base, p.BaseSHA, auth)
	if err != nil {
		return nil, err
	}
	return &Result{Status: "ok", OldSHA: p.BaseSHA, NewSHA: sha, Ref: "refs/heads/" + p.Base}, nil
}

// ChangedFiles derives scope from the immutable source/base pair, including
// names containing spaces or newlines without splitting them into fake paths.
func (p *PreparedMerge) ChangedFiles() ([]string, error) {
	out, err := gitOut(p.Dir, "diff", "--name-only", "-z", p.BaseSHA+"..."+p.SourceSHA, "--")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00"), nil
}

// RestorePrepared accepts only private candidates beneath this service's
// scratch root. The remote is resolved again, never accepted from persisted input.
func (s *Service) RestorePrepared(p *PreparedMerge, dir string) error {
	rel, err := filepath.Rel(s.ScratchDir, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("candidate outside scratch root")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("candidate directory unavailable")
	}
	p.Dir = dir
	p.remote, err = s.remote(p.Repo)
	return err
}

// CandidatePublished also recognizes a later canonical descendant. Recovery
// must not publish again after the first push succeeded but its response was lost.
func (s *Service) CandidatePublished(p *PreparedMerge) (bool, error) {
	remote, err := s.remote(p.Repo)
	if err != nil {
		return false, err
	}
	current, err := s.currentSHA(p.Repo, remote, p.Base)
	if err != nil {
		return false, err
	}
	if current == p.CommitSHA {
		return true, nil
	}
	if current == p.BaseSHA {
		return false, nil
	}
	auth, err := s.authArgs(p.Repo)
	if err != nil {
		return false, err
	}
	args := append(auth, "fetch", "--quiet", remote, current)
	if err = git(p.Dir, "", args...); err != nil {
		return false, err
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", p.CommitSHA, current)
	cmd.Dir = p.Dir
	if err = cmd.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
