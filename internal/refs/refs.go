// Package refs implements the safe per-ref mutation substrate for Switchyard.
//
// UpdateRef is the CAS primitive: a ref update succeeds only if the current
// ref value still equals the expected value; otherwise the write is STALE and
// the caller reconciles. Git's non-force push provides the expected-old
// enforcement together with an exact advertised-SHA pre-push guard; per-ref serialization
// is an ordering convenience for control-plane writers.
package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/process"
	"sync"
	"time"

	"switchyard/internal/artifacts"
	"switchyard/internal/trestle"
)

type Change struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Delete  bool   `json:"delete,omitempty"`
}

type Result struct {
	Status string `json:"status"` // "ok" | "stale"
	OldSHA string `json:"old_sha"`
	NewSHA string `json:"new_sha"`
	Ref    string `json:"ref"`
}

type Service struct {
	Artifacts  *artifacts.Client
	Trestle    *trestle.Client
	ScratchDir string

	mu         sync.Mutex
	refLocks   map[string]*sync.Mutex
	repoTokens map[string]repoToken
}

type repoToken struct {
	token   string
	expires time.Time
}

func NewService(a *artifacts.Client, t *trestle.Client, scratchDir string) *Service {
	return &Service{Artifacts: a, Trestle: t, ScratchDir: scratchDir, refLocks: map[string]*sync.Mutex{}}
}

func (s *Service) lockKey(repo, ref string) string { return repo + "\x00" + ref }

func (s *Service) lock(repo, ref string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.lockKey(repo, ref)
	if m, ok := s.refLocks[k]; ok {
		return m
	}
	m := &sync.Mutex{}
	s.refLocks[k] = m
	return m
}

// repoToken returns a cached repo-scoped git token, minting a fresh one near
// expiry. Git protocol operations use repo tokens, not the account REST token.
func (s *Service) repoToken(repo string) (string, error) {
	s.mu.Lock()
	if s.repoTokens == nil {
		s.repoTokens = map[string]repoToken{}
	}
	if t, ok := s.repoTokens[repo]; ok && time.Until(t.expires) > 2*time.Minute {
		s.mu.Unlock()
		return t.token, nil
	}
	s.mu.Unlock()
	tok, err := s.Artifacts.MintToken(repo, "write", 900)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.repoTokens[repo] = repoToken{token: tok, expires: time.Now().Add(900 * time.Second)}
	s.mu.Unlock()
	return tok, nil
}

// GitToken exposes a cached repo-scoped git token for read/git operations.
func (s *Service) GitToken(repo string) (string, error) {
	return s.repoToken(repo)
}

func (s *Service) remote(repo string) (string, error) {
	r, err := s.Artifacts.GetRepo(repo)
	if err != nil {
		return "", err
	}
	return r.Remote, nil
}

func (s *Service) authArgs(repo string) ([]string, error) {
	tok, err := s.repoToken(repo)
	if err != nil {
		return nil, err
	}
	return []string{"-c", "http.extraHeader=Authorization: Bearer " + tok}, nil
}

// currentSHA reads the authoritative current SHA of a branch ref.
func (s *Service) currentSHA(repo, remote, branch string) (string, error) {
	tok, err := s.repoToken(repo)
	if err != nil {
		return "", err
	}
	refs, err := artifacts.LsRemote(remote, tok)
	if err != nil {
		return "", err
	}
	sha, ok := refs["refs/heads/"+branch]
	if !ok || sha == "" {
		return "", nil // branch does not exist yet (empty)
	}
	return sha, nil
}

// Update applies file changes to the current branch head and CAS-updates the
// ref via a non-force push. Returns status "ok" or "stale".
func (s *Service) Update(repo, branch, expected string, changes []Change, message, provenance string) (*Result, error) {
	if err := validateBranch(branch); err != nil {
		return nil, err
	}

	for _, c := range changes {
		if err := ValidatePath(c.Path); err != nil {
			return nil, err
		}
	}
	l := s.lock(repo, branch)
	l.Lock()
	defer l.Unlock()

	remote, err := s.remote(repo)
	if err != nil {
		return nil, err
	}
	current, err := s.currentSHA(repo, remote, branch)
	if err != nil {
		return nil, err
	}
	if current != expected {
		return &Result{Status: "stale", OldSHA: expected, NewSHA: current, Ref: "refs/heads/" + branch}, nil
	}
	// if the target branch does not exist yet, base it on the repo default branch
	if current == "" {
		r, err := s.Artifacts.GetRepo(repo)
		if err != nil {
			return nil, err
		}
		current, err = s.currentSHA(repo, remote, r.DefaultBranch)
		if err != nil {
			return nil, err
		}
	}

	// build the commit in a disposable scratch clone
	dir, err := s.buildCommit(repo, remote, branch, current, changes, message)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	newSHA, pushErr := s.push(repo, remote, branch, dir, expected)
	if pushErr != nil {
		// a rejected non-force push means the ref moved; if it did, STALE,
		// otherwise it is a genuine error.
		now, e := s.currentSHA(repo, remote, branch)
		if e == nil && now != "" && now != expected {
			return &Result{Status: "stale", OldSHA: expected, NewSHA: now, Ref: "refs/heads/" + branch}, nil
		}
		return nil, fmt.Errorf("push failed: %w", pushErr)
	}
	// record provenance
	if s.Trestle != nil {
		_, _, _ = s.Trestle.CreateRecord("ref_updates", map[string]any{
			"repo": repo, "branch": branch, "old_sha": current, "new_sha": newSHA,
			"provenance": provenance, "occurred_at": time.Now().UTC().Format(time.RFC3339),
		}, "refupd-"+repo+"-"+branch+"-"+newSHA[:12])
	}
	return &Result{Status: "ok", OldSHA: current, NewSHA: newSHA, Ref: "refs/heads/" + branch}, nil
}

func (s *Service) buildCommit(repo, remote, branch, expected string, changes []Change, message string) (string, error) {
	dir, err := os.MkdirTemp(s.ScratchDir, "scratch-*")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(dir)
		}
	}()
	args, err := s.authArgs(repo)
	if err != nil {
		return "", err
	}
	cloneBranch := branch
	if _, ok, err := s.branchExists(remote, branch); err != nil {
		return "", err
	} else if !ok {
		r, err := s.Artifacts.GetRepo(repo)
		if err != nil {
			return "", err
		}
		cloneBranch = r.DefaultBranch
	}
	cloneArgs := append([]string{"clone", "--quiet"}, args...)
	cloneArgs = append(cloneArgs, "--branch", cloneBranch, remote, dir)
	if err := git(dir, "", cloneArgs...); err != nil {
		return "", fmt.Errorf("scratch clone: %w", err)
	}
	// `git clone -c http.extraHeader=...` persists the header into the new
	// repo's config, which would produce duplicate Authorization headers (and
	// an Artifacts 400) on later ops. Remove it so only the explicitly passed
	// -c header is sent.
	_ = git(dir, "", "config", "--unset-all", "http.extraheader")
	head, err := gitOut(dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(head) != expected {
		return "", fmt.Errorf("branch moved before draft clone; reload and retry")
	}
	for _, c := range changes {
		p, err := ContainedPath(dir, c.Path)
		if err != nil {
			return "", err
		}
		if c.Delete {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, []byte(c.Content), 0644); err != nil {
			return "", err
		}
	}
	if err := git(dir, "", "add", "-A"); err != nil {
		return "", err
	}
	if err := git(dir, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "commit", "--quiet", "-m", message); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}
	success = true
	return dir, nil
}

func (s *Service) branchExists(remote, branch string) (string, bool, error) {
	tok, err := s.repoTokenRemote(remote)
	if err != nil {
		return "", false, err
	}
	refs, err := artifacts.LsRemote(remote, tok)
	if err != nil {
		return "", false, err
	}
	sha, ok := refs["refs/heads/"+branch]
	return sha, ok && sha != "", nil
}

// MergeBranch merges the source branch into the target branch and CAS-updates
// the target (canonical integration primitive). Returns the new target SHA.
func (s *Service) MergeBranch(repo, target, source, message, provenance string) (*Result, error) {
	candidate, err := s.PrepareMerge(repo, target, source, message)
	if err != nil {
		return nil, err
	}
	defer candidate.Close()
	return s.PublishPrepared(candidate)
}

// PreviewMerge tests whether the source branch merges cleanly into the target
// branch without modifying any remote ref. It returns the list of files that
// would conflict (structural/semantic conflict detection for preview
// integration). A clean merge returns an empty list.
func (s *Service) PreviewMerge(repo, target, source string) ([]string, error) {
	l := s.lock(repo, target)
	l.Lock()
	defer l.Unlock()
	remote, err := s.remote(repo)
	if err != nil {
		return nil, err
	}
	if _, ok, err := s.branchExists(remote, source); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("preview: source branch %s does not exist", source)
	}
	dir, err := os.MkdirTemp(s.ScratchDir, "preview-*")
	if err != nil {
		return nil, err
	}
	args, err := s.authArgs(repo)
	if err != nil {
		return nil, err
	}
	c := append([]string{"clone", "--quiet"}, args...)
	c = append(c, "--branch", target, remote, dir)
	if err := git(dir, "", c...); err != nil {
		return nil, fmt.Errorf("preview clone: %w", err)
	}
	_ = git(dir, "", "config", "--unset-all", "http.extraheader")
	f := append([]string{}, args...)
	f = append(f, "fetch", "--quiet", remote, source+":"+"src")
	if err := git(dir, "", f...); err != nil {
		return nil, fmt.Errorf("preview fetch: %w", err)
	}
	if err := git(dir, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "merge", "--no-commit", "--no-ff", "src"); err == nil {
		// clean; abort to avoid leaving a merge in progress
		_ = git(dir, "", "merge", "--abort")
		return nil, nil
	}
	// conflicted: enumerate unmerged files
	out, _ := gitOut(dir, "diff", "--name-only", "--diff-filter=U")
	_ = git(dir, "", "merge", "--abort")
	var conflicts []string
	for _, f := range strings.Fields(out) {
		conflicts = append(conflicts, f)
	}
	return conflicts, nil
}

// PreviewMergedTree returns the scratch directory containing the merged
// working tree of the source branch into the target branch when the merge is
// textually clean, or "" when it conflicts. The caller owns the returned
// directory (remove it after use). Used by the semantic-conflict check: a
// clean Git merge does NOT imply the combined tree is semantically valid.
func (s *Service) PreviewMergedTree(repo, target, source string) (string, error) {
	l := s.lock(repo, target)
	l.Lock()
	defer l.Unlock()
	remote, err := s.remote(repo)
	if err != nil {
		return "", err
	}
	if _, ok, err := s.branchExists(remote, source); err != nil {
		return "", err
	} else if !ok {
		return "", fmt.Errorf("preview-tree: source branch %s does not exist", source)
	}
	dir, err := os.MkdirTemp(s.ScratchDir, "ptree-*")
	if err != nil {
		return "", err
	}
	args, err := s.authArgs(repo)
	if err != nil {
		return "", err
	}
	c := append([]string{"clone", "--quiet"}, args...)
	c = append(c, "--branch", target, remote, dir)
	if err := git(dir, "", c...); err != nil {
		return "", fmt.Errorf("preview-tree clone: %w", err)
	}
	_ = git(dir, "", "config", "--unset-all", "http.extraheader")
	f := append([]string{}, args...)
	f = append(f, "fetch", "--quiet", remote, source+":"+"src")
	if err := git(dir, "", f...); err != nil {
		return "", fmt.Errorf("preview-tree fetch: %w", err)
	}
	if err := git(dir, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "merge", "--no-commit", "--no-ff", "src"); err != nil {
		// textual conflict: no merged tree to validate
		_ = git(dir, "", "merge", "--abort")
		_ = os.RemoveAll(dir)
		return "", nil
	}
	return dir, nil
}

// ResolveIntoSource performs a real three-way merge of the source branch into
// the target branch, resolving any conflicts deterministically (git merge-file
// three-way; if that still conflicts, the source's version wins with a
// recorded resolution), and pushes the resolved merge onto the SOURCE branch.
// This is the conflict-resolver repair loop: the source branch is updated so a
// subsequent integration is clean.
func (s *Service) ResolveIntoSource(repo, target, source, message, provenance string) (*Result, []string, error) {
	l := s.lock(repo, target)
	l.Lock()
	defer l.Unlock()
	filesDir, err := os.MkdirTemp(s.ScratchDir, "resolve-files-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(filesDir)

	remote, err := s.remote(repo)
	if err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp(s.ScratchDir, "resolve-*")
	if err != nil {
		return nil, nil, err
	}
	args, err := s.authArgs(repo)
	if err != nil {
		return nil, nil, err
	}
	c := append([]string{"clone", "--quiet"}, args...)
	c = append(c, "--branch", target, remote, dir)
	if err := git(dir, "", c...); err != nil {
		return nil, nil, fmt.Errorf("resolve clone: %w", err)
	}
	_ = git(dir, "", "config", "--unset-all", "http.extraheader")
	f := append([]string{}, args...)
	f = append(f, "fetch", "--quiet", remote, source+":"+"src")
	if err := git(dir, "", f...); err != nil {
		return nil, nil, fmt.Errorf("resolve fetch: %w", err)
	}
	// capture the source SHA before resolution
	srcSHA, _ := gitOut(dir, "rev-parse", "src")
	srcSHA = strings.TrimSpace(srcSHA)

	merged := git(dir, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "merge", "--no-commit", "--no-ff", "src") == nil
	var resolved []string
	if !merged {
		out, _ := gitOut(dir, "diff", "--name-only", "--diff-filter=U")
		for _, name := range strings.Fields(out) {
			// three-way resolution: current = ours (target), base, other = theirs (source)
			ours, _ := gitOut(dir, "show", ":2:"+name)
			base, _ := gitOut(dir, "show", ":1:"+name)
			theirs, _ := gitOut(dir, "show", ":3:"+name)
			ot := filepath.Join(filesDir, "ours")
			bt := filepath.Join(filesDir, "base")
			tt := filepath.Join(filesDir, "theirs")
			if e := os.WriteFile(ot, []byte(ours), 0600); e != nil {
				return nil, nil, e
			}
			if e := os.WriteFile(bt, []byte(base), 0600); e != nil {
				return nil, nil, e
			}
			if e := os.WriteFile(tt, []byte(theirs), 0600); e != nil {
				return nil, nil, e
			}
			// git merge-file <current> <base> <other> -> writes merged into <current>
			if git(dir, "", "merge-file", "-p", ot, bt, tt) != nil {
				// still conflicting: source's version wins (bounded deterministic policy)
				_ = os.WriteFile(ot, []byte(theirs), 0644)
			}
			mergedBytes, _ := os.ReadFile(ot)
			p, e := ContainedPath(dir, name)
			if e != nil {
				return nil, nil, e
			}
			if e = os.WriteFile(p, mergedBytes, 0644); e != nil {
				return nil, nil, e
			}
			_ = git(dir, "", "add", "--", name)
			resolved = append(resolved, name)
		}
	}
	if err := git(dir, "", "-c", "user.name=switchyard", "-c", "user.email=switchyard@local", "commit", "--quiet", "-m", message); err != nil {
		return nil, nil, fmt.Errorf("resolve commit: %w", err)
	}
	// push the resolution onto the source branch
	if _, err := s.push(repo, remote, source, dir, srcSHA); err != nil {
		return nil, nil, fmt.Errorf("resolve push: %w", err)
	}
	newSHA, _ := gitOut(dir, "rev-parse", "HEAD")
	newSHA = strings.TrimSpace(newSHA)
	if s.Trestle != nil {
		_, _, _ = s.Trestle.CreateRecord("ref_updates", map[string]any{
			"repo": repo, "branch": source, "old_sha": srcSHA, "new_sha": newSHA,
			"provenance": "resolve:" + provenance, "occurred_at": time.Now().UTC().Format(time.RFC3339),
		}, "refupd-"+repo+"-"+source+"-"+newSHA[:12])
	}
	return &Result{Status: "ok", OldSHA: srcSHA, NewSHA: newSHA, Ref: "refs/heads/" + source}, resolved, nil
}

// repoTokenRemote resolves a repo token from a remote URL by fetching repo name
// is not needed: we already have repo tokens cached by repo name; for ls-remote
// by remote URL we look up the token via the artifacts account token. This is a
// fallback used by branchExists when only a remote is available.
func (s *Service) repoTokenRemote(remote string) (string, error) {
	// parse the remote path: /git/<ns>/<name>.git and mint a repo token
	parts := strings.Split(strings.TrimSuffix(remote, ".git"), "/")
	name := parts[len(parts)-1]
	return s.repoToken(name)
}

func (s *Service) push(repo, remote, branch, dir, expected string) (string, error) {
	args, err := s.authArgs(repo)
	if err != nil {
		return "", err
	}
	return pushExpected(dir, remote, branch, expected, args)
}

func git(dir, stdin string, args ...string) error {
	cmd, cancel := process.Command(2*time.Minute, "git", args...)
	defer cancel()
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := process.Capture(cmd, true)
	if err != nil {
		return fmt.Errorf("git: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}
func gitOut(dir string, args ...string) (string, error) {
	cmd, cancel := process.Command(2*time.Minute, "git", args...)
	defer cancel()
	cmd.Dir = dir
	return process.Capture(cmd, false)
}

// DiffRefs returns a unified diff and changed-file list between two refs without
// modifying remote refs. It uses a disposable authenticated clone so Git remains
// the source of truth for comparison semantics.
func (s *Service) DiffRefs(repo, base, head string) (string, []string, error) {
	remote, err := s.remote(repo)
	if err != nil {
		return "", nil, err
	}
	dir := filepath.Join(s.ScratchDir, "compare-"+strings.ReplaceAll(repo+base+head, "/", "_"))
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, err
	}
	defer os.RemoveAll(dir)
	args, err := s.authArgs(repo)
	if err != nil {
		return "", nil, err
	}
	c := append([]string{"clone", "--quiet"}, args...)
	c = append(c, "--branch", base, remote, dir)
	if err := git(dir, "", c...); err != nil {
		return "", nil, fmt.Errorf("compare clone: %w", err)
	}
	_ = git(dir, "", "config", "--unset-all", "http.extraheader")
	f := append([]string{}, args...)
	f = append(f, "fetch", "--quiet", remote, head+":"+"compare-head")
	if err := git(dir, "", f...); err != nil {
		return "", nil, fmt.Errorf("compare fetch: %w", err)
	}
	diff, err := gitOut(dir, "diff", "--no-ext-diff", "--find-renames", base+"...compare-head")
	if err != nil {
		return "", nil, err
	}
	names, _ := gitOut(dir, "diff", "--name-only", base+"...compare-head")
	files := []string{}
	for _, n := range strings.Fields(names) {
		files = append(files, n)
	}
	return diff, files, nil
}
