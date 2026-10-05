package app

import (
	"fmt"
	"path/filepath"
	"switchyard/internal/refs"
)

type publicationKind uint8

const (
	publicationHuman publicationKind = iota
	publicationAgent
)

type publicationAuthority struct {
	principal string
	role      string
	kind      publicationKind
}

// The application publication boundary is shared by HTTP handlers and durable
// background effects. No HTTP input can manufacture an integration capability.
func (a *App) authorizePublication(auth publicationAuthority, repo, branch string) error {
	meta := a.repositoryMetaByArtifact(repo)
	cap := WriteRepo
	if auth.kind == publicationAgent {
		cap = RunAgent
	}
	if !a.CanRepository(meta, auth.principal, cap) {
		return fmt.Errorf("publication_repository_denied")
	}
	if auth.kind == publicationAgent {
		if auth.role != "implementer" && auth.role != "conflict-resolver" {
			return fmt.Errorf("unsupported_publication_role")
		}
		canonical := strOr(meta["default_branch"])
		if canonical == "" {
			canonical = "main"
		}
		if branch == canonical || branch == "main" || branch == "master" {
			return fmt.Errorf("agent_canonical_publication_denied")
		}
		rules, err := a.Trestle.ListRecords("repo_protected_refs", filterEq("repo_id", strOr(meta["id"])))
		if err != nil {
			return fmt.Errorf("publication_policy_unavailable")
		}
		for _, rule := range rules {
			match, e := filepath.Match(strOr(rule["pattern"]), branch)
			full, e2 := filepath.Match(strOr(rule["pattern"]), "refs/heads/"+branch)
			if e != nil || e2 != nil {
				return fmt.Errorf("publication_policy_unavailable")
			}
			if match || full {
				return fmt.Errorf("agent_protected_publication_denied")
			}
		}
	}
	// Reuse the existing fail-closed direct-write policy rather than introducing
	// a second interpretation of archived/protected human writes.
	if status, code := a.directPublicationPolicy(meta, branch); status != 0 {
		return fmt.Errorf("publication_policy_denied: %s", code)
	}
	return nil
}

func (a *App) publishUpdate(auth publicationAuthority, repo, branch, expected string, changes []refs.Change, message, provenance string) (*refs.Result, error) {
	if err := a.authorizePublication(auth, repo, branch); err != nil {
		return nil, err
	}
	return a.Refs.Update(repo, branch, expected, changes, message, provenance)
}

func (a *App) publishAgentCandidate(auth publicationAuthority, candidate *refs.PreparedMerge) (*refs.Result, error) {
	if candidate == nil || auth.kind != publicationAgent {
		return nil, fmt.Errorf("invalid_agent_candidate")
	}
	if err := a.authorizePublication(auth, candidate.Repo, candidate.Base); err != nil {
		return nil, err
	}
	return a.Refs.PublishPreparedTracked(candidate, "agent:"+auth.principal+":"+auth.role)
}

// Only the queue processor calls this boundary after preview/semantic checks.
// Re-read durable intent, current permissions and policy immediately before
// publication; callers cannot substitute an arbitrary repository/candidate.
func (a *App) publishQueueCandidate(it *queueItem, candidate *refs.PreparedMerge) (*refs.Result, error) {
	_, _, intent, err := a.Trestle.FindRecord("iq", filterEq("id", it.id))
	if err != nil {
		return nil, err
	}
	if candidate == nil || candidate.Repo != strOr(intent["repo"]) || candidate.Base != strOr(intent["base"]) || candidate.SourceSHA != strOr(intent["source_sha"]) {
		return nil, fmt.Errorf("queue_publication_identity_denied")
	}
	meta := a.repositoryMetaByArtifact(candidate.Repo)
	if !a.CanRepository(meta, strOr(intent["approved_by"]), IntegrateRepo) {
		return nil, fmt.Errorf("queue_publication_authority_revoked")
	}
	_, _, settings, err := a.Trestle.FindRecord("repository_settings", filterEq("repo_id", strOr(meta["id"])))
	if err != nil || strOr(settings["archived"]) == "true" {
		return nil, fmt.Errorf("queue_publication_policy_denied")
	}
	if !a.checkPassedAt(it.prID, candidate.SourceSHA) {
		return nil, fmt.Errorf("queue_checks_revoked")
	}
	changed, err := candidate.ChangedFiles()
	if err != nil {
		return nil, err
	}
	risk := "low"
	if len(changed) > 8 {
		risk = "high"
	} else if len(changed) > 2 {
		risk = "medium"
	}
	decision, reason := a.immutableIntegrationPolicy(candidate.Repo, risk)
	if decision != "allow" {
		return nil, fmt.Errorf("queue_publication_policy_denied: %s", reason)
	}
	return a.Refs.PublishPreparedTracked(candidate, "queue:"+strOr(intent["approved_by"])+":"+it.id)
}

func (a *App) publishRepair(auth publicationAuthority, repo, branch, head, tree, message, provenance string) (*refs.Result, error) {
	if err := a.authorizePublication(auth, repo, branch); err != nil {
		return nil, err
	}
	return a.Refs.PublishRepairTree(repo, branch, head, tree, message, provenance)
}
func (a *App) publishResolution(auth publicationAuthority, repo, base, branch, message, provenance string) (*refs.Result, []string, error) {
	if err := a.authorizePublication(auth, repo, branch); err != nil {
		return nil, nil, err
	}
	return a.Refs.ResolveIntoSource(repo, base, branch, message, provenance)
}
