package artifacts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Event struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Namespace  string            `json:"namespace"`
	Repo       string            `json:"repo"`
	Ref        string            `json:"ref,omitempty"`
	Before     string            `json:"before,omitempty"`
	After      string            `json:"after,omitempty"`
	OccurredAt string            `json:"occurred_at"`
	Class      string            `json:"class"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

var eventName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,90}$`)
var objectID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// NormalizeEvent accepts only documented fields. Repository content, token
// plaintext, imported URL credentials and arbitrary payload keys are discarded.
func NormalizeEvent(body []byte, namespace string) (Event, error) {
	var raw struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Source struct {
			Namespace string `json:"namespace"`
			Repo      string `json:"repoName"`
		} `json:"source"`
		Payload  map[string]json.RawMessage `json:"payload"`
		Metadata struct {
			Timestamp string `json:"eventTimestamp"`
			Account   string `json:"accountId"`
		} `json:"metadata"`
	}
	var ev Event
	if len(body) > 1<<20 || json.Unmarshal(body, &raw) != nil || raw.Source.Namespace != namespace || !eventName.MatchString(raw.Source.Repo) {
		return ev, fmt.Errorf("invalid_artifacts_event")
	}
	ev = Event{Type: raw.Type, Namespace: namespace, Repo: raw.Source.Repo, OccurredAt: raw.Metadata.Timestamp, Metadata: map[string]string{}}
	if _, err := time.Parse(time.RFC3339Nano, ev.OccurredAt); err != nil {
		return Event{}, fmt.Errorf("event_timestamp_invalid")
	}
	get := func(key string) string { var value string; _ = json.Unmarshal(raw.Payload[key], &value); return value }
	switch raw.Type {
	case "cf.artifacts.repo.pushed":
		ev.Class = "coordination_ci_reconciliation"
		ev.Ref = get("ref")
		ev.Before = get("before")
		ev.After = get("after")
		if (!strings.HasPrefix(ev.Ref, "refs/heads/") && !strings.HasPrefix(ev.Ref, "refs/tags/")) || len(ev.Ref) > 256 || strings.Contains(ev.Ref, "..") || strings.ContainsAny(ev.Ref, "\r\n\t ") || (!objectID.MatchString(ev.Before) && ev.Before != "") || (!objectID.MatchString(ev.After) && ev.After != "") {
			return Event{}, fmt.Errorf("event_ref_invalid")
		}
	case "cf.artifacts.repo.created", "cf.artifacts.repo.imported", "cf.artifacts.repo.deleted":
		ev.Class = "activity_reconciliation"
	case "cf.artifacts.repo.forked":
		ev.Class = "activity_reconciliation"
		for _, key := range []string{"namespace", "repoName"} {
			if value := get(key); eventName.MatchString(value) {
				ev.Metadata["target_"+key] = value
			}
		}
	case "cf.artifacts.repo.cloned", "cf.artifacts.repo.fetched":
		ev.Class = "audit"
	case "cf.artifacts.repo.token.created", "cf.artifacts.repo.token.revoked":
		ev.Class = "audit"
		for _, key := range []string{"tokenId", "scope", "expiresAt"} {
			value := get(key)
			if len(value) <= 128 && !strings.ContainsAny(value, "\r\n\t") {
				ev.Metadata[key] = value
			}
		}
	default:
		return Event{}, fmt.Errorf("event_type_unsupported")
	}
	// Cloudflare examples have no universal event ID. Hash the canonical safe
	// envelope with provider timestamp, not delivery time or subscription ID.
	encoded, _ := json.Marshal(ev)
	identity := raw.Metadata.Account + "\x00" + string(encoded)
	if raw.ID != "" && len(raw.ID) <= 256 {
		identity = raw.Metadata.Account + "\x00" + namespace + "\x00" + raw.ID
	}
	ev.ID = fmt.Sprintf("artifact-event-%x", sha256.Sum256([]byte(identity)))
	return ev, nil
}
