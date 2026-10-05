package refs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type publicationIntent struct {
	Candidate  *PreparedMerge `json:"candidate"`
	Dir        string         `json:"dir"`
	Provenance string         `json:"provenance"`
	OccurredAt string         `json:"occurred_at"`
	Phase      string         `json:"phase"`
}

func publicationID(p *PreparedMerge, provenance string) string {
	b, _ := json.Marshal([]string{p.Repo, p.Base, p.BaseSHA, p.SourceSHA, p.CommitSHA, provenance})
	sum := sha256.Sum256(b)
	return "pub_" + hex.EncodeToString(sum[:])
}
func (s *Service) publicationIntent(p *PreparedMerge, provenance string, create bool) (string, string, *publicationIntent, error) {
	id := publicationID(p, provenance)
	rid, version, values, err := s.Trestle.FindRecord("publication_intents", `id = "`+id+`"`)
	if err != nil {
		return "", "", nil, err
	}
	if values == nil && create {
		intent := publicationIntent{Candidate: p, Dir: p.Dir, Provenance: provenance, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Phase: "prepared"}
		_, _, err = s.Trestle.CreateRecord("publication_intents", map[string]any{"id": id, "state": intent}, "publication-"+id)
		rid, version, values, err = s.Trestle.FindRecord("publication_intents", `id = "`+id+`"`)
		if err != nil {
			return "", "", nil, err
		}
	}
	if values == nil {
		return "", "", nil, fmt.Errorf("publication intent unavailable")
	}
	var intent publicationIntent
	b, err := json.Marshal(values["state"])
	if err == nil {
		err = json.Unmarshal(b, &intent)
	}
	if err != nil || intent.Candidate == nil || publicationID(intent.Candidate, intent.Provenance) != id || intent.OccurredAt == "" {
		return "", "", nil, fmt.Errorf("publication intent identity conflict")
	}
	return rid, version, &intent, nil
}
func (s *Service) acknowledgePublication(p *PreparedMerge, provenance string) error {
	if s.Trestle == nil {
		return nil
	}
	rid, version, intent, err := s.publicationIntent(p, provenance, false)
	if err != nil {
		return err
	}
	if intent.Phase == "complete" {
		return nil
	}
	// Stable fact bytes come from the durable intent, never a retry's clock.
	fact := map[string]any{"repo": p.Repo, "branch": p.Base, "old_sha": p.BaseSHA, "new_sha": p.CommitSHA, "provenance": provenance, "occurred_at": intent.OccurredAt}
	_, _, err = s.Trestle.CreateRecord("ref_updates", fact, "publication-fact-"+publicationID(p, provenance))
	if err != nil {
		found, _, values, e := s.Trestle.FindRecord("ref_updates", `repo = "`+p.Repo+`" && branch = "`+p.Base+`" && new_sha = "`+p.CommitSHA+`"`)
		if e != nil || found == "" {
			return fmt.Errorf("publication provenance pending: %w", err)
		}
		for key, value := range fact {
			if values[key] != value {
				return fmt.Errorf("publication receipt conflict")
			}
		}
	}
	if s.PublicationCheckpoint != nil {
		s.PublicationCheckpoint("after_provenance_receipt")
	}
	intent.Phase = "complete"
	return s.Trestle.PatchRecord("publication_intents", rid, version, map[string]any{"state": intent})
}

// PublishPreparedTracked records immutable intent before any Git effect. A lost
// push response is reconciled against remote Git truth before recording a fact.
func (s *Service) PublishPreparedTracked(p *PreparedMerge, provenance string) (*Result, error) {
	return s.trackedPublication(p, provenance, func() (*Result, error) { return s.PublishPrepared(p) })
}
func (s *Service) trackedPublication(p *PreparedMerge, provenance string, publish func() (*Result, error)) (*Result, error) {
	if p.BaseSHA != "" && p.CommitSHA == p.BaseSHA {
		remote, err := s.remote(p.Repo)
		if err != nil {
			return nil, err
		}
		current, err := s.currentSHA(p.Repo, remote, p.Base)
		if err != nil {
			return nil, err
		}
		status := "ok"
		if current != p.BaseSHA {
			status = "stale"
		}
		return &Result{Status: status, OldSHA: p.BaseSHA, NewSHA: current, Ref: "refs/heads/" + p.Base}, nil
	}
	if s.Trestle == nil {
		return publish()
	}
	rid, version, intent, err := s.publicationIntent(p, provenance, true)
	if err != nil {
		return nil, err
	}
	if intent.Phase == "aborted" {
		return nil, fmt.Errorf("publication intent aborted; new intent required")
	}
	if s.PublicationCheckpoint != nil {
		s.PublicationCheckpoint("after_publication_intent")
	}
	observed, err := s.CandidatePublished(p)
	if err != nil {
		return nil, err
	}
	if intent.Phase == "complete" && !observed {
		return nil, fmt.Errorf("published ref no longer contains candidate; explicit reconciliation required")
	}
	if !observed {
		if s.PublicationCheckpoint != nil {
			s.PublicationCheckpoint("before_git_push")
		}
		result, pushErr := publish()
		if s.PublicationCheckpoint != nil {
			s.PublicationCheckpoint("after_git_push")
		}
		if pushErr == nil && result.Status != "ok" {
			intent.Phase = "aborted"
			if err := s.Trestle.PatchRecord("publication_intents", rid, version, map[string]any{"state": intent}); err != nil {
				return nil, err
			}
			return result, nil
		}
		if pushErr != nil {
			observed, err = s.CandidatePublished(p)
			if err != nil || !observed {
				return nil, fmt.Errorf("publication outcome requires reconciliation: %w", pushErr)
			}
		}
	}
	if err := s.acknowledgePublication(p, provenance); err != nil {
		return nil, err
	}
	return &Result{Status: "ok", OldSHA: p.BaseSHA, NewSHA: p.CommitSHA, Ref: "refs/heads/" + p.Base}, nil
}

// ReconcilePreparedProvenance never pushes or invents an intent. It is safe after
// authority revocation because it records only an already-observed historical fact.
func (s *Service) ReconcilePreparedProvenance(p *PreparedMerge, provenance string) error {
	if p.BaseSHA != "" && p.CommitSHA == p.BaseSHA {
		return nil
	}
	observed, err := s.CandidatePublished(p)
	if err != nil {
		return err
	}
	if !observed {
		return fmt.Errorf("publication not observed")
	}
	return s.acknowledgePublication(p, provenance)
}

// ReconcilePublicationFacts observes pending intents only. It never retries a
// Git write, so current policy revocation cannot be bypassed by this worker.
func (s *Service) ReconcilePublicationFacts() error {
	items, err := s.Trestle.ListRecords("publication_intents", "")
	if err != nil {
		return err
	}
	for _, item := range items {
		var intent publicationIntent
		b, e := json.Marshal(item["state"])
		if e == nil {
			e = json.Unmarshal(b, &intent)
		}
		if e != nil || intent.Candidate == nil {
			return fmt.Errorf("invalid publication intent")
		}
		if intent.Phase == "complete" || intent.Phase == "aborted" {
			continue
		}
		p := intent.Candidate
		if str, ok := item["id"].(string); !ok || str != publicationID(p, intent.Provenance) {
			return fmt.Errorf("publication journal identity mismatch")
		}
		observed := false
		if e = s.RestorePrepared(p, intent.Dir); e == nil {
			observed, e = s.CandidatePublished(p)
		} else {
			remote, err := s.remote(p.Repo)
			if err != nil {
				continue
			}
			current, err := s.currentSHA(p.Repo, remote, p.Base)
			if err != nil {
				continue
			}
			observed = current == p.CommitSHA
			e = nil
		}
		if e != nil {
			continue
		} // Missing scratch is operator-recoverable, not a new push.
		if observed {
			if e = s.acknowledgePublication(p, intent.Provenance); e != nil {
				return e
			}
		}
	}
	return nil
}
