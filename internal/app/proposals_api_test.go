package app

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProposalRepositoryReadIsolation(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"repository_meta": {
			{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "visibility": "public"},
			{"id": "priv", "full_name": "alice/private", "owner_type": "user", "owner_id": "alice", "visibility": "private"},
		},
		"proposals": {{"id": "secret", "repository_id": "priv", "title": "Private evidence"}, {"id": "visible", "repository_id": "pub", "title": "Public intake", "state": "open", "type": "Question"}},
	})
	for _, tc := range []struct {
		repo, id string
		status   int
	}{{"public", "", 200}, {"public", "secret", 404}, {"private", "", 404}} {
		r := httptest.NewRequest("GET", "/api/repositories/alice/"+tc.repo+"/proposals", nil)
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", tc.repo)
		r.SetPathValue("id", tc.id)
		w := httptest.NewRecorder()
		a.handleProposals(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s/%s: %d %s", tc.repo, tc.id, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "Private evidence") {
			t.Fatal("cross-repository evidence leaked")
		}
	}
}

func TestProposalGuestCannotForgePrincipal(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"repository_meta": {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "visibility": "public"}},
	})
	r := httptest.NewRequest("POST", "/api/repositories/alice/public/proposals", strings.NewReader(`{"title":"x","type":"Bug","author_principal":"alice","provenance":{"source":"agent"}}`))
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "public")
	w := httptest.NewRecorder()
	a.handleProposals(w, r)
	if w.Code < 400 {
		t.Fatal("anonymous forged mutation accepted")
	}
}

func TestProposalDiscussionVersionAndOwnership(t *testing.T) {
	a := securityFixture(t, map[string][]map[string]any{
		"repository_meta": {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "artifact_name": "public", "visibility": "public"}},
		"sessions":        {{"token": "bob-token", "username": "bob", "expires_at": "2099-01-01T00:00:00Z"}},
		"proposals":       {{"id": "p", "repository_id": "pub", "discussion": []any{map[string]any{"id": "c", "author_principal": "alice", "body": "original"}}}},
	})
	for _, tc := range []struct {
		method, comment, body string
		status                int
	}{
		{"POST", "", `{"version":"0","body":"stale"}`, 409},
		{"PATCH", "c", `{"version":"1","body":"forged edit"}`, 403},
		{"DELETE", "c", `{"version":"1"}`, 403},
		{"POST", "", `{"version":"1","body":"reader discussion"}`, 200},
	} {
		r := httptest.NewRequest(tc.method, "/api/repositories/alice/public/proposals/p/comments", strings.NewReader(tc.body))
		r = r.WithContext(contextWithUser(r.Context(), "bob"))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "public")
		r.SetPathValue("id", "p")
		r.SetPathValue("comment", tc.comment)
		w := httptest.NewRecorder()
		a.handleProposalDiscussion(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.method, w.Code, w.Body.String())
		}
	}
}

func TestProposalWorkRetryAndStandaloneDecision(t *testing.T) {
	records := map[string][]map[string]any{
		"repository_meta": {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "artifact_name": "public", "visibility": "public"}},
		"proposals":       {{"id": "p", "repository_id": "pub", "title": "Question", "type": "Question", "state": "accepted", "author_principal": "alice", "history": []any{}}},
	}
	a := securityFixture(t, records)
	call := func(body string) int {
		r := httptest.NewRequest("POST", "/api/repositories/alice/public/proposals/p/work", strings.NewReader(body))
		r = r.WithContext(contextWithUser(r.Context(), "alice"))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "public")
		r.SetPathValue("id", "p")
		w := httptest.NewRecorder()
		a.handleProposalWork(w, r)
		return w.Code
	}
	body := `{"version":"1","operation_id":"retry-key-123","title":"Investigate","body":"Evidence","kind":"investigation"}`
	if got := call(body); got != 200 {
		t.Fatalf("first request: %d", got)
	}
	if got := call(body); got != 200 {
		t.Fatalf("retry: %d", got)
	}
	if len(records["work"]) != 1 || len(records["work_details"]) != 1 {
		t.Fatal("retry duplicated Work")
	}
	// Repair a failed second write without another Work identity.
	records["work_details"] = nil
	if got := call(body); got != 200 {
		t.Fatalf("repair: %d", got)
	}
	if len(records["work"]) != 1 || len(records["work_details"]) != 1 {
		t.Fatal("partial write not repaired")
	}
	if got := call(strings.Replace(body, "Evidence", "Changed", 1)); got != 409 {
		t.Fatalf("input-changing replay: %d", got)
	}
	if got := call(strings.Replace(body, "retry-key-123", "second-key-123", 1)); got != 200 {
		t.Fatalf("second Work: %d", got)
	}
	if len(records["work"]) != 2 {
		t.Fatal("Proposal cannot have multiple Work items")
	}
	if records["proposals"][0]["state"] != "accepted" {
		t.Fatal("Work creation changed Proposal lifecycle")
	}
}

func TestProposalGraphIsolationAndSupersession(t *testing.T) {
	records := map[string][]map[string]any{
		"repository_meta": {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "artifact_name": "public", "visibility": "public"}},
		"proposals":       {{"id": "a", "repository_id": "pub", "state": "open"}, {"id": "b", "repository_id": "pub", "state": "open"}, {"id": "private", "repository_id": "other"}},
		"proposal_graphs": {{"repository_id": "pub", "edges": []any{}}},
	}
	a := securityFixture(t, records)
	post := func(source, target string) int {
		body := `{"version":"1","relation":"supersedes","target_kind":"proposal","target_id":"` + target + `"}`
		r := httptest.NewRequest("POST", "/links", strings.NewReader(body))
		r = r.WithContext(contextWithUser(r.Context(), "alice"))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "public")
		r.SetPathValue("id", source)
		w := httptest.NewRecorder()
		a.handleProposalLinks(w, r)
		return w.Code
	}
	if got := post("a", "private"); got != 400 {
		t.Fatalf("cross-repo link: %d", got)
	}
	if got := post("a", "b"); got != 201 {
		t.Fatalf("supersession: %d", got)
	}
	if got := post("b", "a"); got != 409 {
		t.Fatalf("cycle: %d", got)
	}
	r := httptest.NewRequest("GET", "/proposal", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "public")
	r.SetPathValue("id", "b")
	w := httptest.NewRecorder()
	a.handleProposals(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"closure_outcome":"superseded"`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestProposalIntakeAndAuthorBoundaries(t *testing.T) {
	records := map[string][]map[string]any{
		"repository_meta":   {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "visibility": "public"}},
		"proposal_settings": {{"repository_id": "pub", "intake": "writers"}},
		"proposals":         {{"id": "owned", "repository_id": "pub", "title": "Own consideration", "type": "Question", "state": "open", "author_principal": "bob"}},
	}
	a := securityFixture(t, records)
	call := func(method, actor, body string) int {
		r := httptest.NewRequest(method, "/proposal", strings.NewReader(body))
		r = r.WithContext(contextWithUser(r.Context(), actor))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "public")
		r.SetPathValue("id", "owned")
		w := httptest.NewRecorder()
		if method == "POST" {
			a.handleProposals(w, r)
		} else {
			a.handleUpdateProposal(w, r)
		}
		return w.Code
	}
	if got := call("POST", "bob", `{"title":"Reader intake","type":"Question"}`); got != 403 {
		t.Fatal(got)
	}
	records["proposal_settings"][0]["intake"] = "readers"
	if got := call("POST", "bob", `{"title":"Reader intake","type":"Question","author_principal":"agent:forged"}`); got != 201 {
		t.Fatal(got)
	}
	if records["proposals"][1]["author_principal"] != "bob" {
		t.Fatal("forged principal accepted")
	}
	if got := call("PATCH", "bob", `{"version":"1","title":"Clarification"}`); got != 200 {
		t.Fatal(got)
	}
	if got := call("PATCH", "bob", `{"version":"1","state":"accepted"}`); got != 403 {
		t.Fatal(got)
	}
	if got := call("PATCH", "charlie", `{"version":"1","title":"Hijack"}`); got != 403 {
		t.Fatal(got)
	}
	if got := call("POST", "bob", `{"title":"Confidential","type":"Security"}`); got != 400 {
		t.Fatal(got)
	}
}

func TestProposalCompletedAgentRecoveryKeepsIdentityAndPrincipal(t *testing.T) {
	records := map[string][]map[string]any{
		"repository_meta":    {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "artifact_name": "public", "visibility": "public"}},
		"executions":         {{"id": "exe_completed", "role": "proposer", "status": "succeeded", "finished_at": "2026-10-05T01:00:00Z"}},
		"execution_metadata": {{"execution_id": "exe_completed", "repo": "public", "metadata": map[string]any{"principal": "alice", "role": "proposer", "status": "succeeded", "source_sha": strings.Repeat("a", 40), "branch": "main", "proposal_result": `{"title":"Provider consideration","type":"Question","description":"Evidence"}`}}},
	}
	a := securityFixture(t, records)
	call := func(actor string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/generate", strings.NewReader(`{"execution_id":"exe_completed"}`))
		r = r.WithContext(contextWithUser(r.Context(), actor))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "public")
		w := httptest.NewRecorder()
		a.handleGenerateProposal(w, r)
		return w
	}
	first := call("alice")
	if first.Code != 201 {
		t.Fatal(first.Code, first.Body.String())
	}
	second := call("alice")
	if second.Code != 201 || first.Body.String() != second.Body.String() {
		t.Fatal(second.Code, second.Body.String())
	}
	if len(records["proposals"]) != 1 {
		t.Fatal("recovery duplicated proposal")
	}
	if records["proposals"][0]["author_principal"] != "agent:exe_completed" {
		t.Fatal("agent principal missing")
	}
	if call("bob").Code == 201 {
		t.Fatal("another principal recovered execution")
	}
}

func TestAgentDiscussionRecoveryDoesNotDuplicateOrCrossTarget(t *testing.T) {
	records := map[string][]map[string]any{
		"repository_meta":    {{"id": "pub", "full_name": "alice/public", "owner_type": "user", "owner_id": "alice", "artifact_name": "public", "visibility": "public"}},
		"proposals":          {{"id": "target", "repository_id": "pub", "title": "Consider this", "description": "Evidence", "state": "open"}, {"id": "other", "repository_id": "pub", "title": "Other"}},
		"executions":         {{"id": "exe_discuss", "finished_at": "2026-10-05T01:00:00Z"}},
		"execution_metadata": {{"execution_id": "exe_discuss", "repo": "public", "metadata": map[string]any{"principal": "alice", "role": "proposer", "status": "succeeded", "source_sha": strings.Repeat("a", 40), "branch": "main", "proposal_id": "target", "proposal_result": `{"description":"Agent discussion consideration"}`}}},
	}
	a := securityFixture(t, records)
	call := func(target string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/generate", strings.NewReader(`{"proposal_id":"`+target+`","execution_id":"exe_discuss"}`))
		r = r.WithContext(contextWithUser(r.Context(), "alice"))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "public")
		w := httptest.NewRecorder()
		a.handleGenerateProposal(w, r)
		return w
	}
	if first := call("target"); first.Code != 201 {
		t.Fatal(first.Code, first.Body.String())
	}
	if call("target").Code != 200 {
		t.Fatal("comment replay failed")
	}
	comments := records["proposals"][0]["discussion"].([]any)
	if len(comments) != 1 || comments[0].(map[string]any)["author_principal"] != "agent:exe_discuss" {
		t.Fatal(comments)
	}
	if call("other").Code != 404 {
		t.Fatal("cross-target recovery accepted")
	}
	if len(records["proposals"]) != 2 {
		t.Fatal("discussion created extra proposal")
	}
}
