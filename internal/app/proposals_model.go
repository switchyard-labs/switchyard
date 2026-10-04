package app

import (
	"fmt"
	"strings"
	"switchyard/internal/trestle"
)

// Proposal records consideration, independently of Work and its execution state.
type Proposal struct {
	ID              string         `json:"id"`
	RepositoryID    string         `json:"repository_id"`
	Type            string         `json:"type"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	State           string         `json:"state"`
	ClosureOutcome  string         `json:"closure_outcome,omitempty"`
	AuthorPrincipal string         `json:"author_principal"`
	Priority        string         `json:"priority,omitempty"`
	Severity        string         `json:"severity,omitempty"`
	Labels          []string       `json:"labels"`
	Provenance      map[string]any `json:"provenance"`
	CreatedAt       string         `json:"created_at"`
	UpdatedAt       string         `json:"updated_at"`
}

var proposalTypes = []string{"Bug", "Suggestion", "Feature", "Improvement", "Refactor", "Documentation", "Security", "Performance", "Experiment", "Question", "Other"}

func validProposalLifecycle(state, outcome string) bool {
	switch state {
	case "open", "accepted", "deferred":
		return outcome == ""
	case "closed":
		switch outcome {
		case "rejected", "answered", "documented", "resolved_without_code", "superseded", "completed":
			return true
		}
	}
	return false
}

func (p *Proposal) validate() error {
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" || len(p.Title) > 240 {
		return fmt.Errorf("proposal_title_invalid")
	}
	if len(p.Description) > 65536 {
		return fmt.Errorf("proposal_description_too_large")
	}
	found := false
	for _, kind := range proposalTypes {
		if p.Type == kind {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("proposal_type_invalid")
	}
	if !validProposalLifecycle(p.State, p.ClosureOutcome) {
		return fmt.Errorf("proposal_state_invalid")
	}
	if len(p.Labels) > 20 {
		return fmt.Errorf("proposal_labels_invalid")
	}
	seen := map[string]bool{}
	for _, label := range p.Labels {
		if label != strings.TrimSpace(label) || label == "" || len(label) > 64 || seen[label] {
			return fmt.Errorf("proposal_labels_invalid")
		}
		seen[label] = true
	}
	if p.Priority != "" && p.Priority != "low" && p.Priority != "normal" && p.Priority != "high" && p.Priority != "urgent" {
		return fmt.Errorf("proposal_priority_invalid")
	}
	if p.Severity != "" && p.Severity != "low" && p.Severity != "medium" && p.Severity != "high" && p.Severity != "critical" {
		return fmt.Errorf("proposal_severity_invalid")
	}
	return nil
}

func proposalCollections() [][2]any {
	text := func(name string) trestle.CollectionField { return trestle.CollectionField{Name: name, Type: "text"} }
	id := trestle.CollectionField{Name: "id", Type: "text", Unique: true}
	return [][2]any{
		{"proposals", []trestle.CollectionField{id, text("repository_id"), text("type"), text("title"), text("description"), text("state"), text("closure_outcome"), text("author_principal"), text("priority"), text("severity"), {Name: "labels", Type: "json"}, {Name: "provenance", Type: "json"}, text("created_at"), text("updated_at"), {Name: "history", Type: "json"}}},
		{"proposal_comments", []trestle.CollectionField{id, text("proposal_id"), text("author_principal"), text("body"), text("created_at"), {Name: "source", Type: "json"}}},
		{"proposal_links", []trestle.CollectionField{id, text("proposal_id"), text("relation"), text("target_kind"), text("target_id"), text("created_by"), text("created_at"), {Name: "idempotency_key", Type: "text", Unique: true}, {Name: "evidence", Type: "json"}}},
		{"proposal_settings", []trestle.CollectionField{{Name: "repository_id", Type: "text", Unique: true}, text("intake"), text("updated_by"), text("updated_at")}},
	}
}
