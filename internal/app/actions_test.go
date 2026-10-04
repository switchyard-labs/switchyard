package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"switchyard/internal/actions"
	"switchyard/internal/artifacts"
	"switchyard/internal/trestle"
	"testing"
)

func actionFixture(t *testing.T) (*App, *queueStore, actions.Snapshot) {
	t.Helper()
	store := &queueStore{records: map[string][]*queueRecord{}}
	server := httptest.NewServer(store)
	t.Cleanup(server.Close)
	a := &App{Trestle: trestle.New(server.URL, "fixture", "fixture"), Artifacts: artifacts.New("fixture", "fixture", "")}
	def, err := actions.CompileDefinition(`export default {refs:['refs/heads/main'],jobs:[{id:'verify',steps:[{id:'test',command:'npm test',timeout_ms:30000}]}]};`)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = a.Trestle.CreateRecord("action_definitions", map[string]any{"id": actionDefinitionID("repo", def.Revision), "repo": "repo", "revision": def.Revision, "definition": def}, "def")
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = a.Trestle.CreateRecord("action_settings", map[string]any{"repo": "repo", "revision": def.Revision, "required_jobs": []string{"verify"}}, "settings")
	if err != nil {
		t.Fatal(err)
	}
	s := actions.Snapshot{ID: "run1", Manifest: &actions.Manifest{Run: actions.Run{Provider: "cloudflare-artifacts", ProviderData: map[string]string{"namespace": "fixture"}, Owner: "fixture", Repo: "repo", SHA: strings.Repeat("a", 40), Ref: "refs/heads/main", Trigger: "push", ID: "run1", DefinitionRevision: def.Revision, Jobs: def.Jobs}, Status: "succeeded", StartedAt: "2026-10-03T01:00:00Z", Jobs: []actions.JobState{{ID: "verify", Status: "succeeded", Steps: []actions.StepState{{ID: "test", Status: "succeeded"}}}}}}
	s.ExternalStatus.Status = "complete"
	return a, store, s
}
func TestActionsExactSHAAndQueuedRerunFence(t *testing.T) {
	a, store, s := actionFixture(t)
	if a.requiredActionsPassed("repo", s.Manifest.Run.SHA) {
		t.Fatal("missing run passed")
	}
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if !a.requiredActionsPassed("repo", s.Manifest.Run.SHA) {
		t.Fatal("valid exact SHA denied")
	}
	if a.requiredActionsPassed("repo", strings.Repeat("b", 40)) {
		t.Fatal("stale SHA passed")
	}
	if len(store.records["action_jobs"]) != 1 || len(store.records["action_steps"]) != 1 || len(store.records["external_executions"]) != 1 {
		t.Fatal("domain views missing")
	}
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if len(store.records["action_runs"]) != 1 || len(store.records["action_checks"]) != 1 {
		t.Fatal("replay duplicated result")
	}
	s.ID = "run2"
	s.Manifest.Run.ID = "run2"
	s.Manifest.StartedAt = "2026-10-03T02:00:00Z"
	s.Manifest.Status = "running"
	s.Manifest.Jobs = nil
	s.ExternalStatus.Status = "running"
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if a.requiredActionsPassed("repo", s.Manifest.Run.SHA) {
		t.Fatal("old success authorized queued rerun")
	}
}

func TestDemoResetExcludesOnlyNamedDemoRun(t *testing.T) {
	a, store, snapshot := actionFixture(t)
	_, _, err := a.Trestle.CreateRecord("demo_action_exclusions", map[string]any{
		"id": "reset-fixture", "repo": "demo-basic", "run_id": "run1",
	}, "reset-fixture")
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Manifest.Run.Repo = "demo-basic"
	if err := a.syncActionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if len(store.records["action_runs"]) != 0 || len(store.records["action_checks"]) != 0 {
		t.Fatal("reset demo run was rediscovered")
	}
	// An exclusion cannot suppress a same-ID run in another repository.
	snapshot.Manifest.Run.Repo = "repo"
	_, _, err = a.Trestle.CreateRecord("demo_action_exclusions", map[string]any{
		"id": "non-demo-fixture", "repo": "repo", "run_id": "run1",
	}, "non-demo-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.syncActionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if len(store.records["action_runs"]) != 1 {
		t.Fatal("non-demo run was suppressed")
	}
}
func TestActionsRejectDefinitionAndTerminalRegression(t *testing.T) {
	a, _, s := actionFixture(t)
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	s.ExternalStatus.Status = "running"
	s.Manifest.Status = "running"
	if a.syncActionSnapshot(s) == nil {
		t.Fatal("terminal result regressed")
	}
	s.ExternalStatus.Status = "complete"
	s.Manifest.Status = "succeeded"
	s.Manifest.Run.Jobs[0].Steps[0].Command = "different"
	if a.syncActionSnapshot(s) == nil {
		t.Fatal("definition mismatch accepted")
	}
}
func TestActionsPartialWritesDoNotAuthorizeMerge(t *testing.T) {
	a, store, s := actionFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/action_checks/") {
			w.WriteHeader(503)
			return
		}
		store.ServeHTTP(w, r)
	}))
	defer server.Close()
	a.Trestle = trestle.New(server.URL, "fixture", "fixture")
	if a.syncActionSnapshot(s) == nil {
		t.Fatal("failed check persistence ignored")
	}
	if a.requiredActionsPassed("repo", s.Manifest.Run.SHA) {
		t.Fatal("partial views authorized merge")
	}
}
func TestNativeCloudSnapshotCanBeNormalized(t *testing.T) {
	data, err := os.ReadFile("../../docs/evidence/campaign/C10/native-status.json")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot actions.Snapshot
	if err = json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	a, store, _ := actionFixture(t)
	a.Artifacts.Namespace = "switchyard-cp0"
	run := snapshot.Manifest.Run
	def := actions.Definition{Revision: run.DefinitionRevision, Jobs: run.Jobs, Refs: []string{run.Ref}}
	_, _, err = a.Trestle.CreateRecord("action_definitions", map[string]any{"id": actionDefinitionID(run.Repo, def.Revision), "repo": run.Repo, "definition": def}, "native-def")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.syncActionSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if len(store.records["action_runs"]) != 1 {
		t.Fatal("cloud snapshot not persisted")
	}
}

func TestActionsDefinitionChangeInvalidatesSameSHA(t *testing.T) {
	a, _, s := actionFixture(t)
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	rid, version, _, err := a.Trestle.FindRecord("action_settings", filterEq("repo", "repo"))
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Trestle.PatchRecord("action_settings", rid, version, map[string]any{"revision": strings.Repeat("f", 64)}); err != nil {
		t.Fatal(err)
	}
	if a.requiredActionsPassed("repo", s.Manifest.Run.SHA) {
		t.Fatal("older approved definition authorized merge")
	}
}

func TestActionsEqualTimestampsFailClosed(t *testing.T) {
	a, _, s := actionFixture(t)
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	s.ID = "rerun"
	s.Manifest.Run.ID = s.ID
	s.Manifest.Status = "running"
	s.Manifest.Jobs = nil
	s.ExternalStatus.Status = "running"
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if a.requiredActionsPassed("repo", s.Manifest.Run.SHA) {
		t.Fatal("timestamp tie resurrected older success")
	}
}

func TestRequiredActionsReplaceDemoMarkerGate(t *testing.T) {
	a, _, s := actionFixture(t)
	_, _, err := a.Trestle.CreateRecord("prs", map[string]any{"id": "pr", "repo": "repo"}, "pr")
	if err != nil {
		t.Fatal(err)
	}
	if a.checkPassedAt("pr", s.Manifest.Run.SHA) {
		t.Fatal("missing Actions passed")
	}
	if err = a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if !a.checkPassedAt("pr", s.Manifest.Run.SHA) {
		t.Fatal("real Actions still required a demo marker")
	}
	if a.checkPassedAt("pr", strings.Repeat("b", 40)) {
		t.Fatal("stale Actions passed PR gate")
	}
}

func TestActionsRepeatedPollingDoesNotRewriteViews(t *testing.T) {
	a, store, s := actionFixture(t)
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if err := a.syncActionSnapshot(s); err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{"action_runs", "action_jobs", "action_steps", "action_checks", "external_executions"} {
		for _, record := range store.records[collection] {
			if record.version != 1 {
				t.Fatalf("unchanged %s rewritten", collection)
			}
		}
	}
}

func TestActionsSourceInspectionTrustBoundary(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong repo", "wrong sha", "missing commit", "wrong inspector", "wrong path", "oversize", "bad digest"} {
		t.Run(scenario, func(t *testing.T) {
			a, store, s := actionFixture(t)
			source := &actions.SourceInspection{Repo: s.Manifest.Run.Repo, SHA: s.Manifest.Run.SHA, CommitPresent: true, Inspection: "artifacts-worker-binding", Config: &actions.SourceConfig{Path: "switchyard.actions.js", Bytes: 877, SHA256: strings.Repeat("a", 64)}}
			s.Manifest.Source = source
			switch scenario {
			case "wrong repo":
				source.Repo = "other"
			case "wrong sha":
				source.SHA = strings.Repeat("b", 40)
			case "missing commit":
				source.CommitPresent = false
			case "wrong inspector":
				source.Inspection = "unverified"
			case "wrong path":
				source.Config.Path = "other.js"
			case "oversize":
				source.Config.Bytes = 65537
			case "bad digest":
				source.Config.SHA256 = strings.Repeat("z", 64)
			}
			err := a.syncActionSnapshot(s)
			if scenario == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				encoded, _ := json.Marshal(store.records["action_runs"][0].values)
				if !strings.Contains(string(encoded), "artifacts-worker-binding") {
					t.Fatal("source provenance lost")
				}
			} else {
				if err == nil {
					t.Fatal("untrusted source inspection accepted")
				}
				if len(store.records["action_checks"]) != 0 {
					t.Fatal("untrusted source authorized a check")
				}
			}
		})
	}
}
