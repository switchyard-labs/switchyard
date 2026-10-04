package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"switchyard/internal/actions"
	"time"
)

var actionRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

func readActionRequest(w http.ResponseWriter, r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return fmt.Errorf("trailing request data")
	}
	return nil
}
func (a *App) handleActionDefinition(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	if a.Actions == nil {
		writeJSON(w, 503, map[string]any{"error": "actions_not_configured"})
		return
	}
	var input struct {
		Source           string   `json:"source"`
		ExpectedRevision string   `json:"expected_revision"`
		Required         []string `json:"required_jobs"`
	}
	if readActionRequest(w, r, &input) != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid_definition_request"})
		return
	}
	definition, err := actions.CompileDefinition(input.Source)
	if err != nil {
		writeJSON(w, 422, map[string]any{"error": "invalid_definition", "detail": err.Error()})
		return
	}
	valid := map[string]bool{}
	for _, job := range definition.Jobs {
		valid[job.ID] = true
	}
	seen := map[string]bool{}
	for _, job := range input.Required {
		if !valid[job] || seen[job] {
			writeJSON(w, 422, map[string]any{"error": "invalid_required_jobs"})
			return
		}
		seen[job] = true
	}
	rid, version, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", repo))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_settings_unavailable"})
		return
	}
	if strOf(settings["revision"]) != input.ExpectedRevision {
		writeJSON(w, 409, map[string]any{"error": "definition_changed"})
		return
	}
	if err = a.Actions.PutDefinition(r.Context(), repo, definition); err != nil {
		writeJSON(w, 502, map[string]any{"error": "definition_not_published"})
		return
	}
	id := actionDefinitionID(repo, definition.Revision)
	_, _, existing, err := a.Trestle.FindRecord("action_definitions", filterEq("id", id))
	if err == nil && existing == nil {
		_, _, err = a.Trestle.CreateRecord("action_definitions", map[string]any{"id": id, "repo": repo, "revision": definition.Revision, "definition": definition, "approved_by": a.currentUser(r), "created_at": nowStr()}, id)
	}
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "definition_not_saved"})
		return
	}
	values := map[string]any{"repo": repo, "revision": definition.Revision, "required_jobs": input.Required}
	if settings == nil {
		_, _, err = a.Trestle.CreateRecord("action_settings", values, "action-settings-"+repo)
	} else {
		err = a.Trestle.PatchRecord("action_settings", rid, version, values)
	}
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": "definition_settings_conflict"})
		return
	}
	writeJSON(w, 200, map[string]any{"revision": definition.Revision, "required_jobs": input.Required})
}
func (a *App) handleActionDispatch(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	if a.Actions == nil {
		writeJSON(w, 503, map[string]any{"error": "actions_not_configured"})
		return
	}
	var input struct {
		RequestID string `json:"request_id"`
		SHA       string `json:"sha"`
		Ref       string `json:"ref"`
		PRID      string `json:"pr_id"`
		Mode      string `json:"mode"`
	}
	if readActionRequest(w, r, &input) != nil || !actionRequestID.MatchString(input.RequestID) {
		writeJSON(w, 400, map[string]any{"error": "invalid_dispatch_request"})
		return
	}
	_, _, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", repo))
	if err != nil || settings == nil {
		writeJSON(w, 409, map[string]any{"error": "definition_not_configured"})
		return
	}
	_, _, record, err := a.Trestle.FindRecord("action_definitions", filterEq("id", actionDefinitionID(repo, strOf(settings["revision"]))))
	var definition actions.Definition
	if err != nil || record == nil || decodeAction(record["definition"], &definition) != nil {
		writeJSON(w, 502, map[string]any{"error": "definition_unavailable"})
		return
	}
	run := actions.Run{Provider: "cloudflare-artifacts", ProviderData: map[string]string{"namespace": a.Artifacts.Namespace}, Owner: a.Artifacts.Namespace, Repo: repo, SHA: input.SHA, Ref: input.Ref, Trigger: "manual", Actor: a.currentUser(r), DefinitionRevision: definition.Revision, Jobs: definition.Jobs, Release: definition.Release}
	parentID := r.PathValue("id")
	if parentID != "" {
		_, _, parent, err := a.Trestle.FindRecord("action_runs", filterEq("id", parentID))
		var state struct {
			Status   string           `json:"status"`
			Manifest actions.Manifest `json:"manifest"`
		}
		if err != nil || parent == nil || parent["repo"] != repo || decodeAction(parent["state"], &state) != nil {
			writeJSON(w, 404, map[string]any{"error": "action_not_found"})
			return
		}
		if !actions.Terminal(state.Status) || parent["definition_revision"] != definition.Revision {
			writeJSON(w, 409, map[string]any{"error": "rerun_requires_terminal_current_definition"})
			return
		}
		run.SHA = strOf(parent["source_sha"])
		run.Ref = strOf(parent["ref"])
		run.Trigger = "rerun"
		run.RerunOf = parentID
		if input.Mode == "failed" {
			for _, job := range definition.Jobs {
				passed := false
				for _, view := range state.Manifest.Jobs {
					if view.ID == job.ID && view.Status == "succeeded" {
						passed = true
					}
				}
				if !passed {
					run.SelectedJobs = append(run.SelectedJobs, job.ID)
				}
			}
			if len(run.SelectedJobs) == 0 {
				writeJSON(w, 409, map[string]any{"error": "no_failed_jobs"})
				return
			}
		}
		if input.Mode != "failed" && input.Mode != "all" {
			writeJSON(w, 400, map[string]any{"error": "invalid_rerun_mode"})
			return
		}
	} else if input.PRID != "" {
		_, _, pr, err := a.Trestle.FindRecord("prs", filterEq("id", input.PRID))
		if err != nil || pr == nil || pr["repo"] != repo {
			writeJSON(w, 404, map[string]any{"error": "pull_request_not_found"})
			return
		}
		_, sha, err := a.Refs.Snapshot(repo, strOf(pr["base"]), strOf(pr["branch"]))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "source_snapshot_unavailable"})
			return
		}
		if input.SHA != "" && input.SHA != sha {
			writeJSON(w, 409, map[string]any{"error": "source_changed"})
			return
		}
		run.SHA = sha
		run.Ref = "refs/heads/" + strOf(pr["branch"])
		run.Trigger = "pull_request"
	}
	validRef := regexp.MustCompile(`^refs/(heads|tags)/[A-Za-z0-9][A-Za-z0-9._/-]{0,250}$`).MatchString(run.Ref)

	if !eventSHA.MatchString(run.SHA) || !validRef || strings.Contains(run.Ref, "..") {
		writeJSON(w, 422, map[string]any{"error": "invalid_source_identity"})
		return
	}
	if run.Release != nil && (!strings.HasPrefix(run.Ref, "refs/tags/") || !slices.Contains(definition.Refs, run.Ref)) {
		writeJSON(w, 422, map[string]any{"error": "release_requires_approved_tag"})
		return
	}
	run.ID = fmt.Sprintf("manual-%x", sha256.Sum256([]byte(repo+"\x00"+run.Actor+"\x00"+input.RequestID)))
	_, _, existing, err := a.Trestle.FindRecord("action_runs", filterEq("id", run.ID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "dispatch_unavailable"})
		return
	}
	if existing != nil {
		var previous struct {
			Manifest actions.Manifest `json:"manifest"`
		}
		if decodeAction(existing["state"], &previous) != nil || !reflect.DeepEqual(previous.Manifest.Run, run) {
			writeJSON(w, 409, map[string]any{"error": "request_identity_conflict"})
			return
		}
	} else {
		manifest := actions.Manifest{Run: run, Status: "queued", StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		_, _, err = a.Trestle.CreateRecord("action_runs", map[string]any{"id": run.ID, "repo": repo, "source_sha": run.SHA, "ref": run.Ref, "definition_revision": run.DefinitionRevision, "trigger": run.Trigger, "actor": run.Actor, "created_at": manifest.StartedAt, "state": map[string]any{"status": "queued", "manifest": manifest, "dispatch_pending": true}}, "action-run-"+run.ID)
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "dispatch_intent_not_saved"})
			return
		}
	}
	// Immutable intent exists before the provider effect. Retrying this request
	// uses the same workflow ID; reconciliation recovers a lost HTTP response.
	if err = a.Actions.Dispatch(r.Context(), run); err != nil {
		writeJSON(w, 502, map[string]any{"error": "dispatch_pending", "id": run.ID})
		return
	}
	writeJSON(w, 202, map[string]any{"id": run.ID, "sha": run.SHA, "definition_revision": run.DefinitionRevision})
}
func (a *App) handleActionCancel(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	id := r.PathValue("id")
	rid, version, run, err := a.Trestle.FindRecord("action_runs", filterEq("id", id))
	var state map[string]any
	if err != nil || run == nil || run["repo"] != repo || decodeAction(run["state"], &state) != nil {
		writeJSON(w, 404, map[string]any{"error": "action_not_found"})
		return
	}
	if actions.Terminal(strOf(state["status"])) {
		writeJSON(w, 409, map[string]any{"error": "run_already_finished"})
		return
	}
	if a.Actions == nil {
		writeJSON(w, 503, map[string]any{"error": "actions_not_configured"})
		return
	}
	state["cancel_requested"] = true
	if err = a.Trestle.PatchRecord("action_runs", rid, version, map[string]any{"state": state}); err != nil {
		writeJSON(w, 409, map[string]any{"error": "cancel_intent_not_saved"})
		return
	}
	if err = a.Actions.Cancel(r.Context(), id); err != nil {
		writeJSON(w, 502, map[string]any{"error": "cancel_pending"})
		return
	}
	writeJSON(w, 202, map[string]any{"id": id, "status": "cancellation_requested"})
}
func (a *App) recoverActionIntents(ctx context.Context) error {
	rows, err := a.Trestle.ListRecords("action_runs", "")
	if err != nil {
		return err
	}
	for _, row := range rows {
		var state struct {
			DispatchPending bool             `json:"dispatch_pending"`
			CancelRequested bool             `json:"cancel_requested"`
			Status          string           `json:"status"`
			Manifest        actions.Manifest `json:"manifest"`
		}
		if decodeAction(row["state"], &state) != nil {
			return fmt.Errorf("invalid Actions intent")
		}
		if actions.Terminal(state.Status) {
			continue
		}
		if state.DispatchPending {
			if err = a.Actions.Dispatch(ctx, state.Manifest.Run); err != nil {
				return err
			}
		}
		if state.CancelRequested {
			if err = a.Actions.Cancel(ctx, strOf(row["id"])); err != nil {
				return err
			}
		}
		if state.DispatchPending || state.CancelRequested {
			var snapshot actions.Snapshot
			if err = a.Actions.Status(ctx, strOf(row["id"]), &snapshot); err != nil {
				return err
			}
			if snapshot.Manifest == nil && snapshot.ExternalStatus.Status == "terminated" {
				snapshot.Manifest = &state.Manifest
				snapshot.Manifest.Status = "cancelled"
			}
			if snapshot.Manifest != nil {
				if err = a.syncActionSnapshot(snapshot); err != nil {
					return err
				}
			}
		}

	}
	return nil
}

func (a *App) handleActionConfiguration(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	_, _, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", repo))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_settings_unavailable"})
		return
	}
	var definition map[string]any
	if settings != nil {
		_, _, definition, err = a.Trestle.FindRecord("action_definitions", filterEq("id", actionDefinitionID(repo, strOf(settings["revision"]))))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "definition_unavailable"})
			return
		}
	}
	writeJSON(w, 200, map[string]any{"settings": settings, "definition": definition, "enabled": a.Actions != nil, "secret_references": actions.WorkerSecretReferences(), "deployment_setup": "A verified Cloudflare Workers Builds connection and build token are required for preview and deployment links."})
}
