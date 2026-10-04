package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"switchyard/internal/actions"
)

func actionDefinitionID(repo, revision string) string {
	return fmt.Sprintf("def-%x", sha256.Sum256([]byte(repo+"\x00"+revision)))
}
func decodeAction(value any, out any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
func sameActionJSON(left, right any) bool {
	a, errA := json.Marshal(left)
	b, errB := json.Marshal(right)
	var normalizedA, normalizedB any
	return errA == nil && errB == nil && json.Unmarshal(a, &normalizedA) == nil && json.Unmarshal(b, &normalizedB) == nil && reflect.DeepEqual(normalizedA, normalizedB)
}
func (a *App) actionRecord(collection, id string, values map[string]any) error {
	rid, version, existing, err := a.Trestle.FindRecord(collection, filterEq("id", id))
	if err != nil {
		return err
	}
	if existing == nil {
		_, _, err = a.Trestle.CreateRecord(collection, values, collection+"-"+id)
		return err
	}
	if sameActionJSON(existing, values) {
		return nil
	}
	// Every child identity is immutable; only its normalized state may advance.
	for _, key := range []string{"repo", "run_id", "source_sha", "definition_revision", "job_id", "provider", "provider_id"} {
		if existing[key] != values[key] {
			return fmt.Errorf("Actions identity conflict")
		}
	}
	patch := map[string]any{}
	for _, key := range []string{"state", "status"} {
		if value, ok := values[key]; ok {
			patch[key] = value
		}
	}
	return a.Trestle.PatchRecord(collection, rid, version, patch)
}
func (a *App) syncActionSnapshot(snapshot actions.Snapshot) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return a.syncActionSnapshotContext(ctx, snapshot)
}
func (a *App) syncActionSnapshotContext(ctx context.Context, snapshot actions.Snapshot) error {
	manifest := snapshot.Manifest
	if manifest == nil {
		return fmt.Errorf("Actions manifest pending")
	}
	run := manifest.Run
	if snapshot.ID != run.ID || !eventSHA.MatchString(run.SHA) || run.Owner != a.Artifacts.Namespace || run.ProviderData["namespace"] != a.Artifacts.Namespace {
		return fmt.Errorf("Actions snapshot identity mismatch")
	}
	// Offline curated-demo reset retains terminal external logs, but suppresses
	// rediscovery of explicitly reset run identities. It never hides other repos.
	switch run.Repo {
	case "demo-basic", "demo-agents", "demo-conflict", "demo-semantic", "demo-workflow":
		_, _, excluded, err := a.Trestle.FindRecord("demo_action_exclusions", filterEq("repo", run.Repo)+" && "+filterEq("run_id", run.ID))
		if err != nil {
			return err
		}
		if excluded != nil {
			return nil
		}
	}
	if source := manifest.Source; source != nil {
		if source.Repo != run.Repo || source.SHA != run.SHA || !source.CommitPresent || source.Inspection != "artifacts-worker-binding" {
			return fmt.Errorf("Actions source inspection identity mismatch")
		}
		if config := source.Config; config != nil && (config.Path != "switchyard.actions.js" || config.Bytes < 0 || config.Bytes > 64<<10 || len(config.SHA256) != 64 || strings.Trim(config.SHA256, "0123456789abcdef") != "") {
			return fmt.Errorf("Actions source configuration fingerprint invalid")
		}
	}
	_, _, definition, err := a.Trestle.FindRecord("action_definitions", filterEq("id", actionDefinitionID(run.Repo, run.DefinitionRevision)))
	if err != nil {
		return err
	}
	if definition == nil {
		return fmt.Errorf("Actions definition is not approved")
	}
	var approved actions.Definition
	if err = decodeAction(definition["definition"], &approved); err != nil {
		return err
	}
	if approved.Revision != run.DefinitionRevision || !reflect.DeepEqual(approved.Jobs, run.Jobs) || !reflect.DeepEqual(approved.Release, run.Release) {
		return fmt.Errorf("Actions snapshot definition mismatch")
	}
	status := actions.NormalizeStatus(snapshot.ExternalStatus.Status, manifest.Status)
	if status == "failure" && snapshot.ExternalStatus.Error != nil && strings.Contains(strings.ToLower(snapshot.ExternalStatus.Error.Name), "timeout") {
		status = "timed_out"
	}
	rid, version, existing, err := a.Trestle.FindRecord("action_runs", filterEq("id", run.ID))
	if err != nil {
		return err
	}
	state := map[string]any{"status": status, "manifest": manifest, "external_status": snapshot.ExternalStatus.Status, "finished_at": manifest.FinishedAt}
	if existing != nil {
		for key, value := range map[string]any{"repo": run.Repo, "source_sha": run.SHA, "ref": run.Ref, "definition_revision": run.DefinitionRevision} {
			if existing[key] != value {
				return fmt.Errorf("Actions run identity conflict")
			}
		}
		old, _ := existing["state"].(map[string]any)
		oldStatus := strOf(old["status"])
		if old["cancel_requested"] == true && !actions.Terminal(status) {
			state["cancel_requested"] = true
		}
		if actions.Terminal(oldStatus) && oldStatus != status {
			return fmt.Errorf("Actions terminal result cannot regress")
		}
		if !sameActionJSON(existing["state"], state) {
			if err = a.Trestle.PatchRecord("action_runs", rid, version, map[string]any{"state": state}); err != nil {
				return err
			}
		}
	} else {
		_, _, err = a.Trestle.CreateRecord("action_runs", map[string]any{"id": run.ID, "repo": run.Repo, "source_sha": run.SHA, "ref": run.Ref, "definition_revision": run.DefinitionRevision, "trigger": run.Trigger, "actor": run.Actor, "created_at": manifest.StartedAt, "state": state}, "action-run-"+run.ID)
		if err != nil {
			return err
		}
	}
	// Persist jobs/steps before publishing checks; partial writes fail closed.
	jobViews := map[string]actions.JobState{}
	for _, job := range manifest.Jobs {
		jobViews[job.ID] = job
	}
	for _, job := range run.Jobs {
		view, ok := jobViews[job.ID]
		if !ok {
			view = actions.JobState{ID: job.ID, Name: job.Name, Status: "queued"}
		}
		if err = a.actionRecord("action_jobs", run.ID+"-"+job.ID, map[string]any{"id": run.ID + "-" + job.ID, "repo": run.Repo, "run_id": run.ID, "state": view}); err != nil {
			return err
		}
		for _, step := range view.Steps {
			if err = a.actionRecord("action_steps", run.ID+"-"+job.ID+"-"+step.ID, map[string]any{"id": run.ID + "-" + job.ID + "-" + step.ID, "repo": run.Repo, "run_id": run.ID, "state": step}); err != nil {
				return err
			}
		}
		check := actions.NormalizeStatus("", view.Status)
		if status == "cancelled" || status == "timed_out" {
			check = status
		}
		if status == "failure" && check != "success" {
			check = "failure"
		}
		id := run.ID + "-" + job.ID
		if err = a.actionRecord("action_checks", id, map[string]any{"id": id, "repo": run.Repo, "source_sha": run.SHA, "run_id": run.ID, "job_id": job.ID, "definition_revision": run.DefinitionRevision, "status": check}); err != nil {
			return err
		}
	}
	if err = a.actionRecord("external_executions", run.ID, map[string]any{"id": run.ID, "repo": run.Repo, "run_id": run.ID, "provider": "cloudflare-workflows", "provider_id": run.ID, "state": map[string]any{"status": status}}); err != nil {
		return err
	}
	return a.syncActionRelease(ctx, manifest, strOf(definition["approved_by"]), status)
}
func (a *App) reconcileActions(ctx context.Context) error {
	if a.Actions == nil {
		return nil
	}
	if err := a.recoverActionIntents(ctx); err != nil {
		return err
	}
	cursor := ""
	seen := map[string]bool{}
	for pageNumber := 0; pageNumber < 64; pageNumber++ {
		page, err := a.Actions.List(ctx, cursor)
		if err != nil {
			return err
		}
		for _, manifest := range page.Runs {
			// Unknown repositories/definitions are deliberately not imported.
			_, _, definition, err := a.Trestle.FindRecord("action_definitions", filterEq("id", actionDefinitionID(manifest.Run.Repo, manifest.Run.DefinitionRevision)))
			if err != nil {
				return err
			}
			if definition == nil {
				continue
			}
			var snapshot actions.Snapshot
			if err = a.Actions.Status(ctx, manifest.Run.ID, &snapshot); err != nil {
				return err
			}
			if err = a.syncActionSnapshotContext(ctx, snapshot); err != nil {
				return err
			}
		}
		if page.Cursor == "" {
			return nil
		}
		if seen[page.Cursor] {
			return fmt.Errorf("Actions cursor loop")
		}
		seen[page.Cursor] = true
		cursor = page.Cursor
	}
	return fmt.Errorf("Actions discovery page budget exceeded")
}
func (a *App) StartActions(ctx context.Context, interval time.Duration) {
	if a.Actions == nil {
		return
	}
	if interval < time.Second {
		interval = time.Second
	}
	a.launchWorker(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			poll, cancel := context.WithTimeout(ctx, 2*time.Minute)
			err := a.reconcileActions(poll)
			cancel()
			if err != nil {
				log.Printf("Actions reconciliation: %v", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
}

// The newest run, not merely the newest passing check, is authoritative. A
// queued rerun fences the old success, and an updated definition fences all
// checks from its previous revision, even at the same Git SHA.
func (a *App) requiredActionsPassed(repo, sha string) bool {
	_, _, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", repo))
	if err != nil {
		return false
	}
	if settings == nil {
		return true
	}
	var required []string
	if decodeAction(settings["required_jobs"], &required) != nil {
		return false
	}
	if len(required) == 0 {
		return true
	}
	revision := strOf(settings["revision"])
	runs, err := a.Trestle.ListRecords("action_runs", filterEq("repo", repo)+" && "+filterEq("source_sha", sha)+" && "+filterEq("definition_revision", revision))
	if err != nil {
		return false
	}
	var latest []map[string]any
	var latestTime time.Time
	for _, run := range runs {
		at, err := time.Parse(time.RFC3339Nano, strOf(run["created_at"]))
		if err != nil {
			return false
		}
		if len(latest) == 0 || at.After(latestTime) {
			latest = []map[string]any{run}
			latestTime = at
		} else if at.Equal(latestTime) {
			latest = append(latest, run)
		}
	}
	if len(latest) == 0 {
		return false
	}
	// Equal provider timestamps have no reliable order. Require all tied runs
	// rather than letting an arbitrary list order resurrect an older success.
	for _, run := range latest {
		checks, err := a.Trestle.ListRecords("action_checks", filterEq("run_id", strOf(run["id"])))
		if err != nil {
			return false
		}
		passed := map[string]bool{}
		for _, check := range checks {
			if check["repo"] == repo && check["source_sha"] == sha && check["definition_revision"] == revision && check["status"] == "success" {
				passed[strOf(check["job_id"])] = true
			}
		}
		for _, job := range required {
			if !passed[job] {
				return false
			}
		}
	}
	return true
}
