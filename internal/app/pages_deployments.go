package app

import (
	"net/http"
	"sort"
	"strconv"
	"switchyard/internal/actions"
)

func (a *App) handlePagesDeployments(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if !a.CanRepository(meta, user, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	if id := r.PathValue("id"); id != "" {
		record := a.scopedRecord("pages_deployments", id)
		if record == nil || record["repository_id"] != meta["id"] {
			writeJSON(w, 404, map[string]any{"error": "pages_deployment_not_found"})
			return
		}
		writeJSON(w, 200, record)
		return
	}
	items, err := a.Trestle.ListRecords("pages_deployments", filterEq("repository_id", strOf(meta["id"])))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_deployments_unavailable"})
		return
	}
	if kind := r.URL.Query().Get("kind"); kind == "project" || kind == "owner" {
		siteID := pagesSiteID(strOf(meta["id"]), kind)
		filtered := []map[string]any{}
		for _, item := range items {
			if item["site_id"] == siteID {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i]["created_at"] == items[j]["created_at"] {
			return strOf(items[i]["id"]) > strOf(items[j]["id"])
		}
		return strOf(items[i]["created_at"]) > strOf(items[j]["created_at"])
	})
	// Deployment content is immutable. Production is an authoritative storage map,
	// not a second mutable deployment lifecycle flag.
	var production map[string]any
	mappingStatus := "unavailable"
	if provider, ok := a.Actions.(pagesProvider); ok {
		production, err = provider.PagesMapping(r.Context(), r.PathValue("owner"))
		if err == nil {
			mappingStatus = "available"
		}
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	total := len(items)
	start := (page - 1) * 20
	if start > total || start < 0 {
		start = total
	}
	end := start + 20
	if end > total {
		end = total
	}
	items = items[start:end]
	writeJSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": 20, "production": production, "production_status": mappingStatus})
}
func (a *App) handlePagesDeploy(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if user == "" || a.isDemoGuest(r) || !a.CanRepository(meta, user, WriteRepo) {
		writeJSON(w, 403, map[string]any{"error": "pages_deploy_permission_required"})
		return
	}
	if a.Actions == nil {
		writeJSON(w, 503, map[string]any{"error": "actions_not_configured"})
		return
	}
	var input struct {
		OperationID string `json:"operation_id"`
		Version     string `json:"version"`
	}
	if readJSON(r, &input) != nil || !actionRequestID.MatchString(input.OperationID) {
		writeJSON(w, 400, map[string]any{"error": "pages_operation_id_invalid"})
		return
	}
	siteID := pagesSiteID(strOf(meta["id"]), pagesKind(r))
	_, version, site, err := a.Trestle.FindRecord("pages_sites", filterEq("id", siteID))
	if err != nil || site == nil {
		writeJSON(w, 404, map[string]any{"error": "pages_not_configured"})
		return
	}
	if input.Version == "" || input.Version != version {
		writeJSON(w, 409, map[string]any{"error": "pages_config_conflict"})
		return
	}
	if site["enabled"] != "true" {
		writeJSON(w, 409, map[string]any{"error": "pages_builds_disabled"})
		return
	}
	var config PagesConfig
	if decodeAction(site["config"], &config) != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_config_invalid"})
		return
	}
	var definition actions.Definition
	_, _, record, err := a.Trestle.FindRecord("action_definitions", filterEq("id", actionDefinitionID(strOf(meta["artifact_name"]), strOf(site["definition_revision"]))))
	if err != nil || record == nil || decodeAction(record["definition"], &definition) != nil {
		writeJSON(w, 502, map[string]any{"error": "definition_unavailable"})
		return
	}
	id := "pages-" + sha256Hex([]byte(siteID + "\x00" + user + "\x00" + input.OperationID))
	_, _, previous, err := a.Trestle.FindRecord("action_runs", filterEq("id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_dispatch_unavailable"})
		return
	}
	var manifest actions.Manifest
	if previous != nil {
		state, _ := previous["state"].(map[string]any)
		if decodeAction(state["manifest"], &manifest) != nil || manifest.Run.DefinitionRevision != definition.Revision {
			writeJSON(w, 409, map[string]any{"error": "pages_operation_input_conflict"})
			return
		}
	} else {
		sourceRefs, e := a.repoRefs(strOf(meta["artifact_name"]))
		sha := ""
		if e == nil {
			sha, e = pagesResolvedSource(sourceRefs, config.Ref)
		}
		if e != nil {
			writeJSON(w, 422, map[string]any{"error": "pages_source_unavailable"})
			return
		}
		run := actions.Run{ID: id, Provider: "cloudflare-artifacts", ProviderData: map[string]string{"namespace": a.Artifacts.Namespace}, Owner: a.Artifacts.Namespace, Repo: strOf(meta["artifact_name"]), SHA: sha, Ref: pagesRef(config.Ref), Trigger: "manual", Actor: user, DefinitionRevision: definition.Revision, Jobs: definition.Jobs}
		manifest = actions.Manifest{Run: run, Status: "queued", StartedAt: nowStr()}
		values := map[string]any{"id": id, "repo": run.Repo, "source_sha": sha, "ref": run.Ref, "definition_revision": definition.Revision, "trigger": "manual", "actor": user, "created_at": manifest.StartedAt, "state": map[string]any{"status": "queued", "dispatch_pending": true, "manifest": manifest}}
		if _, _, err = a.Trestle.CreateRecord("action_runs", values, "action-run-"+id); err != nil {
			writeJSON(w, 502, map[string]any{"error": "pages_dispatch_intent_failed"})
			return
		}
		// Resolve an ambiguous or competing create by reading its exact saved source.
		_, _, saved, e := a.Trestle.FindRecord("action_runs", filterEq("id", id))
		if e != nil || saved == nil {
			writeJSON(w, 502, map[string]any{"error": "pages_dispatch_intent_unavailable"})
			return
		}
		state, _ := saved["state"].(map[string]any)
		if decodeAction(state["manifest"], &manifest) != nil || manifest.Run.DefinitionRevision != definition.Revision {
			writeJSON(w, 409, map[string]any{"error": "pages_operation_input_conflict"})
			return
		}
	}
	if err = a.syncActionPages(r.Context(), &manifest, "queued"); err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_deployment_intent_pending", "run_id": id})
		return
	}
	if err = a.Actions.Dispatch(r.Context(), manifest.Run); err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_dispatch_pending", "run_id": id})
		return
	}
	writeJSON(w, 202, map[string]any{"run_id": id, "source_sha": manifest.Run.SHA, "source_ref": manifest.Run.Ref, "deployment_id": pagesDeploymentID(siteID, id, strOf(site["job_id"]), manifest.Run.DefinitionRevision)})
}
func (a *App) handlePagesPromote(w http.ResponseWriter, r *http.Request) {
	if action := r.PathValue("action"); action != "promote" && action != "rollback" {
		writeJSON(w, 404, map[string]any{"error": "pages_action_not_found"})
		return
	}
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	deployment := a.scopedRecord("pages_deployments", r.PathValue("id"))
	if deployment == nil || deployment["repository_id"] != meta["id"] {
		writeJSON(w, 404, map[string]any{"error": "pages_deployment_not_found"})
		return
	}
	site := a.scopedRecord("pages_sites", strOf(deployment["site_id"]))
	kind := "project"
	if site != nil && site["project"] == "" {
		kind = "owner"
	}
	if a.isDemoGuest(r) || !a.pagesAuthority(meta, user, kind) {
		writeJSON(w, 403, map[string]any{"error": "pages_publish_permission_required"})
		return
	}
	if site == nil || site["owner"] != r.PathValue("owner") {
		writeJSON(w, 409, map[string]any{"error": "pages_site_identity_changed"})
		return
	}
	var config PagesConfig
	if decodeAction(site["config"], &config) != nil || config.validate(strOf(meta["visibility"]) == "private") != nil {
		writeJSON(w, 409, map[string]any{"error": "pages_publication_policy_changed"})
		return
	}
	var input struct {
		Generation  int64  `json:"generation"`
		OperationID string `json:"operation_id"`
	}
	if readJSON(r, &input) != nil || !actionRequestID.MatchString(input.OperationID) || input.Generation < 0 {
		writeJSON(w, 400, map[string]any{"error": "pages_promotion_invalid"})
		return
	}
	state, _ := deployment["state"].(map[string]any)
	artifact, _ := state["artifact"].(map[string]any)
	if state["status"] != "ready" || artifact == nil {
		writeJSON(w, 409, map[string]any{"error": "pages_deployment_not_ready"})
		return
	}
	provider, ok := a.Actions.(pagesProvider)
	if !ok {
		writeJSON(w, 503, map[string]any{"error": "pages_storage_unavailable"})
		return
	}
	action := "promote"
	if r.PathValue("action") == "rollback" {
		action = "rollback"
	}
	result, err := provider.PromotePages(r.Context(), map[string]any{"owner": site["owner"], "project": site["project"], "target": artifact, "generation": input.Generation, "operation_id": input.OperationID, "actor": user, "action": action})
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": "pages_promotion_pending_or_conflicted", "operation_id": input.OperationID})
		return
	}
	_, _, err = a.Trestle.CreateRecord("events", map[string]any{"type": "pages." + map[string]string{"promote": "promoted", "rollback": "rolled_back"}[action], "repo_name": deployment["repo"], "occurred_at": nowStr(), "payload": map[string]any{"deployment_id": deployment["id"], "source_sha": deployment["source_sha"], "actor": user, "operation_id": input.OperationID, "production": result}}, "pages-production-"+sha256Hex([]byte(strOf(site["owner"])+"\x00"+input.OperationID)))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_promotion_audit_pending", "operation_id": input.OperationID})
		return
	}
	writeJSON(w, 200, map[string]any{"production": result, "deployment_id": deployment["id"], "action": action, "source_sha": deployment["source_sha"], "public_hosting": "conditional_dns_tls"})
}
