package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"switchyard/internal/actions"
	"switchyard/internal/trestle"
)

type pagesProvider interface {
	PublishPages(context.Context, map[string]any) (map[string]any, error)
	PagesMapping(context.Context, string) (map[string]any, error)
	PromotePages(context.Context, map[string]any) (map[string]any, error)
}

func pagesCollections() [][2]any {
	text := func(name string) trestle.CollectionField { return trestle.CollectionField{Name: name, Type: "text"} }
	id := trestle.CollectionField{Name: "id", Type: "text", Unique: true}
	return [][2]any{
		{"pages_mounts", []trestle.CollectionField{id, text("site_id"), text("owner"), text("base_path"), text("created_at")}},
		{"pages_sites", []trestle.CollectionField{id, text("repository_id"), text("owner"), text("project"), text("enabled"), text("approved_by"), text("definition_revision"), text("job_id"), {Name: "config", Type: "json"}, {Name: "approvals", Type: "json"}, text("created_at"), text("updated_at")}},
		{"pages_deployments", []trestle.CollectionField{id, {Name: "identity", Type: "text", Unique: true}, text("site_id"), text("repository_id"), text("repo"), text("run_id"), text("job_id"), text("source_sha"), text("source_ref"), text("definition_revision"), text("created_by"), text("approved_by"), {Name: "config", Type: "json"}, text("created_at"), {Name: "state", Type: "json"}}},
	}
}
func pagesDeploymentID(site, run, job, revision string) string {
	return "dpl_" + sha256Hex([]byte(site + "\x00" + run + "\x00" + job + "\x00" + revision))[:32]
}

// Completion is reconciled from the already-authorized Actions snapshot. Stable
// identities and immutable Worker writes repair ambiguous/partial success.
func (a *App) syncActionPages(ctx context.Context, manifest *actions.Manifest, status string) error {
	sites, err := a.Trestle.ListRecords("pages_sites", "")
	if err != nil {
		return err
	}
	for _, site := range sites {
		if site["enabled"] != "true" {
			continue
		}
		meta := a.scopedRecord("repository_meta", strOf(site["repository_id"]))
		if meta == nil || strOf(meta["artifact_name"]) != manifest.Run.Repo {
			continue
		}
		var config PagesConfig
		if decodeAction(site["config"], &config) != nil || config.validate(strOf(meta["visibility"]) == "private") != nil {
			return fmt.Errorf("invalid pages configuration")
		}
		var job *actions.Job
		for i := range manifest.Run.Jobs {
			candidate := &manifest.Run.Jobs[i]
			if candidate.ID == strOf(site["job_id"]) {
				job = candidate
				break
			}
		}
		if job == nil {
			continue
		}
		id := pagesDeploymentID(strOf(site["id"]), manifest.Run.ID, job.ID, manifest.Run.DefinitionRevision)
		rid, version, record, err := a.Trestle.FindRecord("pages_deployments", filterEq("id", id))
		if err != nil {
			return err
		}
		approvedBy := strOf(site["approved_by"])
		if record == nil && site["definition_revision"] != manifest.Run.DefinitionRevision {
			var approvals map[string]struct {
				Config     PagesConfig `json:"config"`
				ApprovedBy string      `json:"approved_by"`
			}
			if decodeAction(site["approvals"], &approvals) == nil {
				if saved, ok := approvals[manifest.Run.DefinitionRevision]; ok {
					config, approvedBy = saved.Config, saved.ApprovedBy
				}
			}
		}
		if record != nil {
			if decodeAction(record["config"], &config) != nil || config.validate(strOf(meta["visibility"]) == "private") != nil {
				return fmt.Errorf("invalid frozen pages configuration")
			}
			approvedBy = strOf(record["approved_by"])
		}
		kind := "project"
		if config.Project == "" {
			kind = "owner"
		}
		if !a.pagesAuthority(meta, approvedBy, kind) {
			return fmt.Errorf("pages approval authority expired")
		}
		if !reflect.DeepEqual(*job, pagesBuildJob(config, kind)) {
			continue
		}
		if (manifest.Run.Trigger == "push" || strings.HasPrefix(manifest.Run.ID, "pages-")) && manifest.Run.Ref != pagesRef(config.Ref) {
			continue
		}
		next := "building"
		switch status {
		case "queued":
			next = "queued"
		case "failure", "timed_out":
			next = "failed"
		case "cancelled":
			next = "cancelled"
		case "success":
			next = "uploading"
		}
		if rid == "" {
			values := map[string]any{"id": id, "identity": id, "site_id": site["id"], "repository_id": site["repository_id"], "repo": manifest.Run.Repo, "run_id": manifest.Run.ID, "job_id": job.ID, "source_sha": manifest.Run.SHA, "source_ref": manifest.Run.Ref, "definition_revision": manifest.Run.DefinitionRevision, "created_by": manifest.Run.Actor, "approved_by": approvedBy, "config": config, "created_at": manifest.StartedAt, "state": map[string]any{"status": next}}
			if _, _, err = a.Trestle.CreateRecord("pages_deployments", values, "pages-deployment-"+id); err != nil {
				return err
			}
			rid, version, record, err = a.Trestle.FindRecord("pages_deployments", filterEq("id", id))
			if err != nil || rid == "" {
				return fmt.Errorf("pages deployment intent unavailable")
			}
		}
		if record["site_id"] != site["id"] || record["source_sha"] != manifest.Run.SHA || record["run_id"] != manifest.Run.ID || record["definition_revision"] != manifest.Run.DefinitionRevision {
			return fmt.Errorf("pages deployment identity conflict")
		}
		// Replay repairs a missing creation audit after the durable intent exists.
		if _, _, err = a.Trestle.CreateRecord("events", map[string]any{"type": "pages.deployment_created", "repo_name": record["repo"], "occurred_at": record["created_at"], "payload": map[string]any{"deployment_id": id, "run_id": record["run_id"], "source_sha": record["source_sha"], "actor": record["created_by"]}}, id+"-created"); err != nil {
			return err
		}
		old, _ := record["state"].(map[string]any)
		oldStatus := strOf(old["status"])
		if oldStatus == "ready" || oldStatus == "failed" || oldStatus == "cancelled" {
			if err = a.pagesDeploymentEvent(record, oldStatus); err != nil {
				return err
			}
			continue
		}
		if oldStatus == "uploading" && (next == "queued" || next == "building") || oldStatus == "building" && next == "queued" {
			continue // Delayed snapshots cannot move a deployment backwards.
		}
		state := map[string]any{"status": next, "transitioned_at": nowStr()}
		if oldStatus == next {
			state = old
		}
		if next == "uploading" {
			if oldStatus != "uploading" {
				if err = a.Trestle.PatchRecord("pages_deployments", rid, version, map[string]any{"state": state}); err != nil {
					return err
				}
				rid, version, record, err = a.Trestle.FindRecord("pages_deployments", filterEq("id", id))
				if err != nil || record == nil {
					return fmt.Errorf("pages uploading intent unavailable")
				}
				old, _ = record["state"].(map[string]any)
				if old["status"] == "ready" {
					if err = a.pagesDeploymentEvent(record, "ready"); err != nil {
						return err
					}
					continue
				}
				if old["status"] != "uploading" {
					return fmt.Errorf("pages uploading intent changed")
				}
			}
			var receipt *actions.BuildAssetReceipt
			for _, view := range manifest.Jobs {
				if view.ID == job.ID && view.Status == "succeeded" {
					receipt = view.Static
				}
			}
			if receipt == nil || receipt.SourceSHA != manifest.Run.SHA || receipt.JobID != job.ID || receipt.Name != "pages-static.bundle.json" {
				return fmt.Errorf("pages artifact receipt missing")
			}
			provider, ok := a.Actions.(pagesProvider)
			if !ok {
				return fmt.Errorf("pages storage provider unavailable")
			}
			result, err := provider.PublishPages(ctx, map[string]any{"repo": manifest.Run.Repo, "run_id": manifest.Run.ID, "job_id": job.ID, "source_sha": manifest.Run.SHA, "definition_revision": manifest.Run.DefinitionRevision, "site_id": site["id"], "deployment_id": id, "base_path": config.basePath()})
			if err != nil {
				return err
			}
			if result["site_id"] != site["id"] || result["deployment_id"] != id || result["source_sha"] != manifest.Run.SHA || result["artifact_sha256"] != receipt.SHA256 || result["base_path"] != config.basePath() {
				return fmt.Errorf("pages publication identity conflict")
			}
			state = map[string]any{"status": "ready", "artifact": result, "ready_at": nowStr()}
		}
		if sameActionJSON(old, state) {
			if err = a.pagesDeploymentEvent(record, next); err != nil {
				return err
			}
			continue
		}
		if err = a.Trestle.PatchRecord("pages_deployments", rid, version, map[string]any{"state": state}); err != nil {
			return err
		}
		record["state"] = state
		if err = a.pagesDeploymentEvent(record, strOf(state["status"])); err != nil {
			return err
		}
	}
	return nil
}

// Existing event storage remains the activity surface. Idempotency keys bind
// state events to immutable deployment provenance; callback retries repair them.
func (a *App) pagesDeploymentEvent(record map[string]any, status string) error {
	kind := map[string]string{"queued": "pages.build_requested", "building": "pages.build_started", "ready": "pages.build_succeeded", "failed": "pages.build_failed", "cancelled": "pages.build_cancelled"}[status]
	if kind == "" {
		return nil
	}
	state, _ := record["state"].(map[string]any)
	at := strOf(state["transitioned_at"])
	if status == "ready" {
		at = strOf(state["ready_at"])
	}
	if at == "" {
		at = strOf(record["created_at"])
	}
	payload := map[string]any{"deployment_id": record["id"], "site_id": record["site_id"], "run_id": record["run_id"], "source_sha": record["source_sha"], "source_ref": record["source_ref"], "actor": record["created_by"], "state": record["state"]}
	_, _, err := a.Trestle.CreateRecord("events", map[string]any{"type": kind, "repo_name": record["repo"], "occurred_at": at, "payload": payload}, strOf(record["id"])+"-"+status)
	return err
}
