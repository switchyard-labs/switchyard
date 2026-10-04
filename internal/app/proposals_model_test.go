package app

import "testing"

func TestProposalOptionalLifecycle(t *testing.T) {
	// No execution object is required for any intake outcome.
	for _, state := range []string{"open", "accepted", "deferred"} {
		if !validProposalLifecycle(state, "") {
			t.Fatalf("standalone %s rejected", state)
		}
		if validProposalLifecycle(state, "completed") {
			t.Fatalf("nonclosed outcome accepted: %s", state)
		}
	}
	for _, outcome := range []string{"rejected", "answered", "documented", "resolved_without_code", "superseded", "completed"} {
		p := Proposal{Title: "Standalone consideration", Type: "Question", State: "closed", ClosureOutcome: outcome}
		if err := p.validate(); err != nil {
			t.Fatalf("%s requires execution: %v", outcome, err)
		}
	}
	for _, state := range []string{"investigating", "implementation_in_progress", "blocked", ""} {
		if validProposalLifecycle(state, "") {
			t.Fatalf("derived/invalid state persisted: %s", state)
		}
	}
	if validProposalLifecycle("closed", "") {
		t.Fatal("closure must record outcome")
	}
}

func TestProposalBoundedMetadata(t *testing.T) {
	for _, kind := range proposalTypes {
		p := Proposal{Title: " A consideration ", Type: kind, State: "open", Labels: []string{"performance"}}
		if err := p.validate(); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if p.Title != "A consideration" {
			t.Fatal("title not normalized")
		}
	}
	for _, p := range []Proposal{
		{Title: "x", Type: "Epic", State: "open"},
		{Title: "x", Type: "Bug", State: "open", Labels: []string{"a", "a"}},
		{Title: "x", Type: "Bug", State: "open", Labels: []string{" "}},
		{Title: "x", Type: "Bug", State: "open", Priority: "unbounded"},
	} {
		if p.validate() == nil {
			t.Fatalf("invalid metadata accepted: %+v", p)
		}
	}
}
