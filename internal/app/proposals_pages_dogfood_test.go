package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/actions"
	"switchyard/internal/agent"
	"testing"
)

// Real application handlers, Git ref publication and runner/reconciliation paths;
// the external Actions/Pages transport is substituted. Its artifact, serving and
// promotion implementation is separately exercised by pages-control.test.mjs.
func TestProposalWorkAttemptsPagesLocalDogfood(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scratch"), 0700); err != nil {
		t.Fatal(err)
	}
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")
	queueGit(t, root, "init", "--bare", remote)
	queueGit(t, root, "init", "-b", "main", local)
	if err := os.WriteFile(filepath.Join(local, "index.html"), []byte("<h1>Base</h1>"), 0600); err != nil {
		t.Fatal(err)
	}
	queueGit(t, local, "add", ".")
	queueGit(t, local, "commit", "-m", "base")
	queueGit(t, local, "push", remote, "main")
	if err := os.WriteFile(filepath.Join(root, "token.sh"), []byte("#!/bin/sh\nprintf fixture\n"), 0700); err != nil {
		t.Fatal(err)
	}
	store := &queueStore{records: map[string][]*queueRecord{}, enforceUnique: true}
	server := httptest.NewServer(store)
	defer server.Close()
	a := queueCrashApp(server.URL, remote, root)
	a.DataDir = root
	a.Runner = &agent.DeterministicRunner{}
	provider := &pagesTestProvider{}
	a.Actions = provider
	_, _, err := a.Trestle.CreateRecord("repository_meta", map[string]any{"id": "meta", "artifact_name": "repo", "owner_type": "user", "owner_id": "alice", "visibility": "public", "full_name": "alice/docs", "default_branch": "main"}, "meta")
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body string, handler http.HandlerFunc, id string) map[string]any {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(contextWithUser(r.Context(), "alice"))
		r.SetPathValue("owner", "alice")
		r.SetPathValue("repo", "docs")
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code < 200 || w.Code >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		var out map[string]any
		if json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatal(w.Body.String())
		}
		return out
	}
	proposal := call("POST", "/proposals", `{"title":"Publish a considered change","type":"Feature"}`, a.handleProposals, "")
	proposalID := strOf(proposal["id"])
	_, version, _, _ := a.Trestle.FindRecord("proposals", filterEq("id", proposalID))
	call("PATCH", "/proposals/"+proposalID, `{"version":"`+version+`","state":"accepted"}`, a.handleUpdateProposal, proposalID)
	_, version, _, _ = a.Trestle.FindRecord("proposals", filterEq("id", proposalID))
	workResult := call("POST", "/work", `{"version":"`+version+`","operation_id":"dogfood-work-op","title":"Publish candidates"}`, a.handleProposalWork, proposalID)
	work := workResult["work"].(map[string]any)
	workID := strOf(work["id"])
	configResult := call("PATCH", "/config", `{"enabled":true,"config":{"ref":"main","working_directory":".","build_command":"mkdir -p public && cp index.html public/index.html","output_directory":"public"}}`, a.handlePagesConfig, "")
	site := configResult["site"].(map[string]any)
	var definition actions.Definition
	record := a.scopedRecord("action_definitions", actionDefinitionID("repo", strOf(site["definition_revision"])))
	if decodeAction(record["definition"], &definition) != nil {
		t.Fatal("definition")
	}
	var selectedAttempt, selectedSHA string
	var completed actions.Snapshot
	for _, branch := range []string{"candidate-a", "candidate-b"} {
		attempt := call("POST", "/attempts", `{"repo":"repo","branch":"`+branch+`"}`, a.handleCreateAttempt, workID)
		attemptID := strOf(attempt["id"])
		head, e := a.repoHead("repo", branch)
		if e != nil {
			t.Fatal(e)
		}
		ctx := agentUserContext(t.Context(), "alice")
		_, result, e := a.runAgentStepContext(ctx, "implementer", attemptID, "repo", branch, "index.html", "<h1>Base</h1>", "\n<p>"+branch+"</p>", head, "user:alice")
		if e != nil {
			t.Fatal(e)
		}
		actual, e := a.repoHead("repo", branch)
		if e != nil || actual != result.NewSHA {
			t.Fatal("published SHA mismatch", e)
		}
		if branch == "candidate-a" {
			selectedAttempt, selectedSHA = attemptID, actual
		}
		run := actions.Run{ID: "dogfood-" + branch, Provider: "cloudflare-artifacts", ProviderData: map[string]string{"namespace": "fixture"}, Owner: "fixture", Repo: "repo", SHA: actual, Ref: "refs/heads/" + branch, Trigger: "manual", Actor: "alice", DefinitionRevision: definition.Revision, Jobs: definition.Jobs}
		job := definition.Jobs[0]
		manifest := &actions.Manifest{Run: run, Status: "succeeded", StartedAt: nowStr(), Jobs: []actions.JobState{{ID: job.ID, Status: "succeeded", Static: &actions.BuildAssetReceipt{Name: "pages-static.bundle.json", JobID: job.ID, SourceSHA: actual, SHA256: strings.Repeat("b", 64)}, Steps: []actions.StepState{{ID: "build", Status: "succeeded"}}}}}
		snapshot := actions.Snapshot{ID: run.ID, Manifest: manifest}
		completed = snapshot
		snapshot.ExternalStatus.Status = "complete"
		if e = a.syncActionSnapshot(snapshot); e != nil {
			t.Fatal(e)
		}
	}
	view := call("GET", "/work/"+workID, "", a.handleGetWork, workID)
	previews := view["pages_previews"].([]any)
	if len(previews) != 2 {
		t.Fatal("missing attempt previews", view)
	}
	if len(store.records["executions"]) != 2 || len(store.records["pages_deployments"]) != 2 {
		t.Fatal("missing execution/deployment provenance")
	}
	for _, preview := range previews {
		p := preview.(map[string]any)
		if !workspaceSHA.MatchString(strOf(p["source_sha"])) || p["hosting"] != "conditional_dns_tls" {
			t.Fatal(p)
		}
	}
	// Review the selected candidate, then run the existing exact-SHA queue.
	call("POST", "/review", "{}", a.handleReviewAttempt, selectedAttempt)
	pr := call("POST", "/pr", `{"title":"Choose candidate A"}`, a.handleOpenPR, selectedAttempt)
	prID := strOf(pr["id"])
	if err := a.setCommitCheck(prID, "repo", selectedSHA, "pass", "local dogfood exact-source validation"); err != nil {
		t.Fatal(err)
	}
	queued := call("POST", "/enqueue", `{"source_sha":"`+selectedSHA+`"}`, a.handleEnqueuePR, prID)
	if err := a.processQueueItem(&queueItem{id: strOf(queued["id"]), prID: prID, repo: "repo", base: "main", branch: "candidate-a", risk: "low"}); err != nil {
		t.Fatal(err)
	}
	productionSHA, err := a.repoHead("repo", "main")
	if err != nil || productionSHA == selectedSHA {
		t.Fatal("integration did not create canonical result", err)
	}
	// Branch-push Actions completion needs neither a Proposal nor an Agent.
	completed.ID = "dogfood-production"
	completed.Manifest.Run.ID = completed.ID
	completed.Manifest.Run.Trigger = "push"
	completed.Manifest.Run.Ref = "refs/heads/main"
	completed.Manifest.Run.SHA = productionSHA
	completed.Manifest.Jobs[0].Static.SourceSHA = productionSHA
	if err := a.syncActionSnapshot(completed); err != nil {
		t.Fatal(err)
	}
	if len(store.records["pages_deployments"]) != 3 {
		t.Fatal("production deployment absent")
	}
	// Work can produce candidate Pages without any Proposal relationship.
	standalone := call("POST", "/work", `{"title":"Standalone site","repo":"repo"}`, a.handleCreateWork, "")
	standaloneID := strOf(standalone["id"])
	standaloneAttempt := call("POST", "/attempts", `{"repo":"repo","branch":"standalone-site"}`, a.handleCreateAttempt, standaloneID)
	standaloneSHA, err := a.repoHead("repo", "standalone-site")
	if err != nil {
		t.Fatal(err)
	}
	completed.ID = "dogfood-standalone"
	completed.Manifest.Run.ID = completed.ID
	completed.Manifest.Run.Trigger = "manual"
	completed.Manifest.Run.Ref = "refs/heads/standalone-site"
	completed.Manifest.Run.SHA = standaloneSHA
	completed.Manifest.Jobs[0].Static.SourceSHA = standaloneSHA
	if err := a.syncActionSnapshot(completed); err != nil {
		t.Fatal(err)
	}
	standaloneView := call("GET", "/work/"+standaloneID, "", a.handleGetWork, standaloneID)
	standalonePreviews := standaloneView["pages_previews"].([]any)
	if len(standalonePreviews) != 1 || standalonePreviews[0].(map[string]any)["attempt_id"] != standaloneAttempt["id"] {
		t.Fatal("standalone Work requires Proposal", standaloneView)
	}
	// Manual Pages dispatch records an exact SHA and a retry keeps that SHA.
	_, configVersion, _, err := a.Trestle.FindRecord("pages_sites", filterEq("id", strOf(site["id"])))
	if err != nil {
		t.Fatal(err)
	}
	requestBody := `{"version":"` + configVersion + `","operation_id":"dogfood-manual-operation"}`
	manual := call("POST", "/deploy", requestBody, a.handlePagesDeploy, "")
	if manual["source_sha"] != productionSHA {
		t.Fatal("manual did not resolve canonical SHA", manual)
	}
	queueGit(t, root, "--git-dir="+remote, "update-ref", "refs/heads/main", selectedSHA)
	replay := call("POST", "/deploy", requestBody, a.handlePagesDeploy, "")
	if replay["run_id"] != manual["run_id"] || replay["source_sha"] != productionSHA {
		t.Fatal("manual retry re-resolved branch", replay)
	}
	queueGit(t, root, "--git-dir="+remote, "update-ref", "refs/heads/main", productionSHA)
	// Exercise proposer intake through the normal execution substrate. Only the
	// model transport is local; this does not certify a live commercial provider.
	_, _, err = a.Trestle.CreateRecord("agent_preferences", map[string]any{"username": "alice", "provider": "openai", "models": map[string]any{"openai": "fixture-model"}, "credentials": map[string]any{"openai": "fixture-credential"}}, "fixture-preferences")
	if err != nil {
		t.Fatal(err)
	}
	model := &proposalDogfoodRunner{}
	_, _, err = a.Trestle.CreateRecord("credential_owners", map[string]any{"username": "alice", "credential_id": "fixture-credential"}, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	a.Secrets = agent.NewCredentialStore(nil, func() ([]map[string]any, error) {
		return []map[string]any{{"id": "fixture-credential", "provider": "openai"}}, nil
	}, nil, make([]byte, 32))
	a.ProviderRunner = func(selection AgentSelection) (agent.Runner, error) {
		if selection.Model != "fixture-model" {
			return nil, fmt.Errorf("wrong model")
		}
		return model, nil
	}
	generated := call("POST", "/generate", `{"prompt":"Consider a smaller site","branch":"main"}`, a.handleGenerateProposal, "")
	provenance := generated["provenance"].(map[string]any)
	if provenance["source_sha"] != productionSHA {
		t.Fatal("proposer lost source identity")
	}
	recovered := call("POST", "/generate", `{"execution_id":"`+strOf(provenance["execution_id"])+`"}`, a.handleGenerateProposal, "")
	if recovered["id"] != generated["id"] || model.calls != 1 {
		t.Fatal("generation recovery reran model")
	}
	call("POST", "/generate", `{"proposal_id":"`+proposalID+`","prompt":"Explain tradeoffs","branch":"main"}`, a.handleGenerateProposal, "")
	if len(store.records["proposals"]) != 2 || model.calls != 2 {
		t.Fatal("discussion created a separate Proposal")
	}
	for _, outcome := range []string{"rejected", "completed"} {
		standaloneProposal := call("POST", "/proposals", `{"title":"Standalone decision","type":"Question"}`, a.handleProposals, "")
		id := strOf(standaloneProposal["id"])
		_, v, _, _ := a.Trestle.FindRecord("proposals", filterEq("id", id))
		if outcome == "completed" {
			call("PATCH", "/decision", `{"version":"`+v+`","state":"accepted"}`, a.handleUpdateProposal, id)
			_, v, _, _ = a.Trestle.FindRecord("proposals", filterEq("id", id))
		}
		call("PATCH", "/decision", `{"version":"`+v+`","state":"closed","closure_outcome":"`+outcome+`"}`, a.handleUpdateProposal, id)
	}
	// Proposal remains consideration; creating/running Work does not complete it.
	if a.scopedRecord("proposals", proposalID)["state"] != "accepted" {
		t.Fatal("mandatory Proposal transition")
	}
}

// Controlled model output tests application execution and persistence, not LLM quality.
type proposalDogfoodRunner struct{ calls int }

func (r *proposalDogfoodRunner) Run(_ context.Context, ex *agent.Execution, task agent.Task) error {
	if task.Role != "proposer" || task.Provider != "openai" || task.File != "PROPOSAL.json" {
		return fmt.Errorf("unexpected proposer task")
	}
	r.calls++
	ex.Result = map[string]string{"PROPOSAL.json": `{"title":"Consider a smaller site","description":"A bounded consideration and its tradeoffs.","type":"Suggestion"}`}
	ex.Status = "succeeded"
	return nil
}
