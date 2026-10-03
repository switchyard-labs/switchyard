package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const zeroSHA = "0000000000000000000000000000000000000000"

var eventSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

func normalizeZero(sha string) string {
	if sha == zeroSHA {
		return ""
	}
	return sha
}

// Persist the first immutable envelope BEFORE advancing the ref observation.
// Queue and reconciliation retries use that exact body, not a new timestamp.
func (a *App) observeTransition(repo, branch, before, after, source string) (bool, error) {
	before, after = normalizeZero(before), normalizeZero(after)
	ref := branch
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + ref
	}
	if repo == "" || (!strings.HasPrefix(ref, "refs/heads/") && !strings.HasPrefix(ref, "refs/tags/")) || strings.Contains(ref, "..") || (before != "" && !eventSHA.MatchString(before)) || (after != "" && !eventSHA.MatchString(after)) {
		return false, fmt.Errorf("invalid ref transition")
	}
	id := fmt.Sprintf("gitref-%x", sha256.Sum256([]byte(repo+"\x00"+ref+"\x00"+before+"\x00"+after)))
	_, _, receipt, err := a.Trestle.FindRecord("event_receipts", filterEq("id", id))
	if err != nil {
		return false, err
	}
	if receipt == nil {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		envelope := map[string]any{"type": "git.ref_changed", "repo_name": repo, "payload": map[string]any{"ref": ref, "before": before, "after": after, "source": source, "seen_at": now}, "occurred_at": now}
		_, _, err = a.Trestle.CreateRecord("event_receipts", map[string]any{"id": id, "repo": repo, "envelope": envelope}, "event-intent-"+id+"-"+randHex(8))
		var readErr error
		_, _, receipt, readErr = a.Trestle.FindRecord("event_receipts", filterEq("id", id))
		if readErr != nil {
			return false, readErr
		}
		if receipt == nil {
			return false, fmt.Errorf("event intent unavailable: %v", err)
		}
	}
	encoded, err := json.Marshal(receipt["envelope"])
	if err != nil {
		return false, err
	}
	var envelope map[string]any
	if err = json.Unmarshal(encoded, &envelope); err != nil {
		return false, err
	}
	payload, ok := envelope["payload"].(map[string]any)
	if !ok || envelope["type"] != "git.ref_changed" || envelope["repo_name"] != repo || payload["ref"] != ref || payload["before"] != before || payload["after"] != after {
		return false, fmt.Errorf("event receipt identity mismatch")
	}
	eventID, replayed, err := a.Trestle.CreateRecord("events", envelope, id)
	if err != nil {
		return false, err
	}
	if !replayed && a.Hub != nil {
		a.Hub.Publish(map[string]any{"id": eventID, "type": "git.ref_changed", "repo": repo, "payload": envelope["payload"], "replayed": false})
	}
	_, _, err = a.Trestle.CreateRecord("ref_obs", map[string]any{"repo": repo, "branch": branch, "sha": after, "seen_at": envelope["occurred_at"]}, "ref-observation-"+id)
	return replayed, err
}
