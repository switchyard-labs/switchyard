package app

import (
	"encoding/json"
	"fmt"
	"switchyard/internal/artifacts"
)

func (a *App) ingestArtifactEvent(ev artifacts.Event, source string) (bool, error) {
	encoded, _ := json.Marshal(ev)
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return false, err
	}
	envelope := map[string]any{"type": ev.Type, "repo_name": ev.Repo, "payload": normalized, "occurred_at": ev.OccurredAt}
	_, _, receipt, err := a.Trestle.FindRecord("event_receipts", filterEq("id", ev.ID))
	if err != nil {
		return false, err
	}
	if receipt == nil {
		_, _, err = a.Trestle.CreateRecord("event_receipts", map[string]any{"id": ev.ID, "repo": ev.Repo, "envelope": envelope}, "artifact-receipt-"+ev.ID)
		if err != nil {
			return false, err
		}
	} else {
		stored, _ := json.Marshal(receipt["envelope"])
		expected, _ := json.Marshal(envelope)
		if string(stored) != string(expected) {
			return false, fmt.Errorf("artifact_event_identity_mismatch")
		}
	}
	if ev.Type == "cf.artifacts.repo.pushed" {
		return a.observeTransition(ev.Repo, shortRef(ev.Ref), ev.Before, ev.After, source)
	}
	id, replayed, err := a.Trestle.CreateRecord("events", envelope, ev.ID)
	if err == nil && !replayed && a.Hub != nil {
		a.Hub.Publish(map[string]any{"id": id, "type": ev.Type, "repo": ev.Repo, "payload": normalized})
	}
	return replayed, err
}
