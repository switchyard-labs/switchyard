package app

import (
	"time"
)

// refTransition identity: a git ref movement is the same logical fact
// regardless of which path observed it (Cloudflare queue event or periodic
// reconciliation). The idempotency identity is (repo, ref, before, after);
// it deliberately does NOT include delivery metadata (message id, seen_at) so
// both paths converge onto one durable domain event and one broadcast.
const zeroSHA = "0000000000000000000000000000000000000000"

// normalizeZero maps the all-zero "null" SHAs used for branch create/delete
// events to the empty string so both paths agree on the identity.
func normalizeZero(sha string) string {
	if sha == zeroSHA {
		return ""
	}
	return sha
}

// observeTransition is the single normalized ingest path for git ref movement.
// It records an append-only audit observation (ref_obs) and one logical domain
// event (events, keyed by the ref-transition identity). The domain event is
// deduplicated at the Trestle boundary via its Idempotency-Key; SSE is only
// broadcast for the first creation (replayed == false). The same transition
// observed by the queue fast path and later by reconciliation yields exactly
// one domain event and one broadcast.
func (a *App) observeTransition(repo, branch, before, after, source string) (replayed bool, err error) {
	before = normalizeZero(before)
	after = normalizeZero(after)
	now := time.Now().UTC().Format(time.RFC3339)

	// append-only audit observation (distinct per source+time; not the
	// dedup boundary).
	_, _, _ = a.Trestle.CreateRecord("ref_obs", map[string]any{
		"repo": repo, "branch": branch, "sha": after, "seen_at": now,
	}, "refobs-"+repo+"-"+branch+"-"+after+"-"+source+"-"+now)

	// one logical domain fact: identity = repo + ref + before + after.
	ref := "refs/heads/" + branch
	payload := map[string]any{"ref": ref, "before": before, "after": after, "source": source, "seen_at": now}
	key := "gitref-" + repo + "-" + ref + "-" + before + "-" + after
	id, replayed, err := a.Trestle.CreateRecord("events", map[string]any{
		"type": "git.ref_changed", "repo_name": repo, "payload": payload, "occurred_at": now,
	}, key)
	if err != nil {
		return false, err
	}
	if !replayed {
		a.Hub.Publish(map[string]any{"id": id, "type": "git.ref_changed", "repo": repo, "payload": payload, "replayed": false})
	}
	return replayed, nil
}