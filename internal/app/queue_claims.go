package app

import (
	"encoding/json"
	"fmt"
	"switchyard/internal/refs"
	"time"
)

const queueLease = 10 * time.Minute

type queueEffect struct {
	QueueID      string              `json:"queue_id"`
	Owner        string              `json:"owner"`
	LeaseUntil   string              `json:"lease_until"`
	Phase        string              `json:"phase"`
	Candidate    *refs.PreparedMerge `json:"candidate,omitempty"`
	CandidateDir string              `json:"candidate_dir,omitempty"`
	PublishedSHA string              `json:"published_sha,omitempty"`
}

type queueClaim struct {
	a          *App
	effect     queueEffect
	collection string
}

func decodeQueueEffect(values map[string]any) (queueEffect, error) {
	var effect queueEffect
	b, err := json.Marshal(values["state"])
	if err != nil {
		return effect, err
	}
	err = json.Unmarshal(b, &effect)
	return effect, err
}

func (a *App) claimQueue(id string) (*queueClaim, error) {
	return a.claimExecution("integration_effects", id)
}

func (a *App) claimExecution(collection, id string) (*queueClaim, error) {
	rid, _, values, err := a.Trestle.FindRecord(collection, filterEq("queue_id", id))
	if err != nil {
		return nil, err
	}
	if rid == "" {
		_, _, err = a.Trestle.CreateRecord(collection, map[string]any{"queue_id": id, "state": queueEffect{QueueID: id, Phase: "queued"}}, "integration-effect-"+id)
		if err != nil {
			return nil, err
		}
	}
	rid, ver, values, err := a.Trestle.FindRecord(collection, filterEq("queue_id", id))
	if err != nil || rid == "" {
		return nil, fmt.Errorf("integration effect unavailable: %v", err)
	}
	effect, err := decodeQueueEffect(values)
	if err != nil {
		return nil, err
	}
	if effect.Phase == "done" || effect.Phase == "blocked" {
		return nil, nil
	}
	expires, _ := time.Parse(time.RFC3339Nano, effect.LeaseUntil)
	if effect.Owner != "" && expires.After(time.Now()) {
		return nil, nil
	}
	effect.Owner = randHex(20)
	effect.LeaseUntil = time.Now().Add(queueLease).UTC().Format(time.RFC3339Nano)
	if effect.Phase == "queued" {
		effect.Phase = "claimed"
	}
	if err = a.Trestle.PatchRecord(collection, rid, ver, map[string]any{"state": effect}); err != nil {
		return nil, err
	}
	return &queueClaim{a: a, effect: effect, collection: collection}, nil
}

func (c *queueClaim) save(phase string) error {
	rid, ver, values, err := c.a.Trestle.FindRecord(c.collection, filterEq("queue_id", c.effect.QueueID))
	if err != nil || rid == "" {
		return fmt.Errorf("read integration claim: %v", err)
	}
	current, err := decodeQueueEffect(values)
	if err != nil {
		return err
	}
	expires, _ := time.Parse(time.RFC3339Nano, current.LeaseUntil)
	if current.Owner != c.effect.Owner || !expires.After(time.Now()) {
		return fmt.Errorf("integration claim lost")
	}
	c.effect.Phase = phase
	c.effect.LeaseUntil = time.Now().Add(queueLease).UTC().Format(time.RFC3339Nano)
	return c.a.Trestle.PatchRecord(c.collection, rid, ver, map[string]any{"state": c.effect})
}

func (c *queueClaim) release() error {
	rid, ver, values, err := c.a.Trestle.FindRecord(c.collection, filterEq("queue_id", c.effect.QueueID))
	if err != nil || rid == "" {
		return fmt.Errorf("read integration claim: %v", err)
	}
	current, err := decodeQueueEffect(values)
	if err != nil {
		return err
	}
	if current.Owner != c.effect.Owner {
		return fmt.Errorf("integration claim lost")
	}
	current.Owner = ""
	current.LeaseUntil = ""
	return c.a.Trestle.PatchRecord(c.collection, rid, ver, map[string]any{"state": current})
}
