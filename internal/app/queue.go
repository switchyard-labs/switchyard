package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// QueueConsumer is the event-driven fast path. Cloudflare Artifacts delivers
// `cf.artifacts.repo.pushed` events to a Cloudflare Queue; the Switchyard
// control plane pulls batches from that queue (pull-based consumer, consistent
// with the CP0 decision that a self-hosted control plane cannot be reached from
// the Cloudflare edge), ingests each one through the SAME normalized path as
// reconciliation (observeTransition), then acknowledges the message. Messages
// that fail to ingest are retried (returned to the queue) so delivery is
// at-least-once.
//
// Reconciliation remains the safety net: it repairs anything the queue missed
// while the control plane was down or the subscription lagged. Because both
// paths share the ref-transition idempotency identity, the same git movement
// observed by both converges onto one durable domain event and one broadcast.
//
// Invariant: events accelerate convergence; reconciliation guarantees
// eventual convergence.
type QueueConsumer struct {
	AccountID string
	QueueID   string
	Token     func() (string, error)
	http      *http.Client
}

func NewQueueConsumer(accountID, queueID string, token func() (string, error)) *QueueConsumer {
	return &QueueConsumer{AccountID: accountID, QueueID: queueID, Token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

func (q *QueueConsumer) pullURL() string {
	return "https://api.cloudflare.com/client/v4/accounts/" + q.AccountID + "/queues/" + q.QueueID + "/messages/pull"
}

func (q *QueueConsumer) ackURL() string {
	return "https://api.cloudflare.com/client/v4/accounts/" + q.AccountID + "/queues/" + q.QueueID + "/messages/ack"
}

type queueMessage struct {
	ID          string          `json:"id"`
	LeaseID     string          `json:"lease_id"`
	Attempts    int             `json:"attempts"`
	TimestampMs int64           `json:"timestamp_ms"`
	Body        json.RawMessage `json:"body"`
}

// Pull fetches a batch of messages (leased for visibility_timeout_ms).
func (q *QueueConsumer) Pull(batchSize int, visibilityMS int64) ([]queueMessage, error) {
	tok, err := q.Token()
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]any{"batch_size": batchSize, "visibility_timeout_ms": visibilityMS})
	req, _ := http.NewRequest(http.MethodPost, q.pullURL(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Result struct {
			Messages []queueMessage `json:"messages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if !out.Success || len(out.Errors) > 0 {
		if len(out.Errors) > 0 {
			return nil, fmt.Errorf("queue pull: %s", out.Errors[0].Message)
		}
		return nil, fmt.Errorf("queue pull: not success")
	}
	return out.Result.Messages, nil
}

// Ack acknowledges (or retries) leased messages by lease_id.
func (q *QueueConsumer) Ack(acks []string, retries []string) error {
	tok, err := q.Token()
	if err != nil {
		return err
	}
	ackObjs := make([]map[string]string, 0, len(acks))
	for _, l := range acks {
		ackObjs = append(ackObjs, map[string]string{"lease_id": l})
	}
	retryObjs := make([]map[string]any, 0, len(retries))
	for _, l := range retries {
		retryObjs = append(retryObjs, map[string]any{"lease_id": l, "delay_seconds": 1})
	}
	body, _ := json.Marshal(map[string]any{"acks": ackObjs, "retries": retryObjs})
	req, _ := http.NewRequest(http.MethodPost, q.ackURL(), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := q.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("queue ack: %d %s", resp.StatusCode, strings.TrimSpace(string(rb)))
	}
	return nil
}

// consumeOnce pulls one batch from the Cloudflare queue, ingests every
// `cf.artifacts.repo.pushed` event through observeTransition, and acknowledges
// successfully-ingested messages (retrying failed ones).
func (a *App) consumeOnce() (resultErr error) {
	started := time.Now()
	defer func() {
		status := 200
		if resultErr != nil {
			status = 500
		}
		a.Metrics.Record("event_consumer", time.Since(started), status)
	}()
	msgs, err := a.Queue.Pull(10, 30000)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return nil
	}
	acked := []string{}
	retried := []string{}
	ingested := 0
	for _, m := range msgs {
		var ev struct {
			Type   string `json:"type"`
			Source struct {
				RepoName string `json:"repoName"`
			} `json:"source"`
			Payload struct {
				Ref    string `json:"ref"`
				Before string `json:"before"`
				After  string `json:"after"`
			} `json:"payload"`
		}
		// Cloudflare queue message bodies are JSON-encoded strings (the CP0
		// peek shows "body":"{\"type\":...}"), so decode the string first,
		// then the envelope.
		var bodyStr string
		if err := json.Unmarshal(m.Body, &bodyStr); err == nil {
			m.Body = json.RawMessage(bodyStr)
		}
		if err := json.Unmarshal(m.Body, &ev); err != nil {
			// malformed message: ack (nothing useful to retry forever)
			log.Printf("queue consumer: malformed message %s: %v", m.ID, err)
			acked = append(acked, m.LeaseID)
			continue
		}
		if ev.Type != "cf.artifacts.repo.pushed" || ev.Source.RepoName == "" {
			acked = append(acked, m.LeaseID)
			continue
		}
		branch := shortRef(ev.Payload.Ref)
		if branch == "" || ev.Payload.After == "" {
			acked = append(acked, m.LeaseID)
			continue
		}
		if _, err := a.observeTransition(ev.Source.RepoName, branch, ev.Payload.Before, ev.Payload.After, "queue"); err != nil {
			log.Printf("queue ingest %s/%s: %v", ev.Source.RepoName, branch, err)
			retried = append(retried, m.LeaseID)
			continue
		}
		ingested++
		acked = append(acked, m.LeaseID)
	}
	if len(acked) > 0 || len(retried) > 0 {
		if err := a.Queue.Ack(acked, retried); err != nil {
			return err
		}
	}
	if ingested > 0 {
		log.Printf("queue consumer: ingested %d pushed event(s)", ingested)
	}
	return nil
}

// StartEventConsumer runs the queue fast path on a short interval. A queue id
// is required; without one the fast path is disabled and reconciliation alone
// provides convergence.
func (a *App) StartEventConsumer(ctx context.Context, interval time.Duration) {
	if a.Queue == nil {
		log.Printf("event consumer disabled (no queue id configured)")
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	log.Printf("event consumer started (queue=%s, interval %s)", a.Queue.QueueID, interval)
	a.launchWorker(func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		// first pass immediately
		if err := a.consumeOnce(); err != nil {
			log.Printf("event consumer: %v", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := a.consumeOnce(); err != nil {
					log.Printf("event consumer: %v", err)
				}
			}
		}
	})
}
