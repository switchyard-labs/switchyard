package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"switchyard/internal/actions"
)

func pagesSiteID(repoID, kind string) string {
	return "site_" + sha256Hex([]byte(repoID + "\x00" + kind))[:24]
}
func (a *App) pagesAuthority(meta map[string]any, user, kind string) bool {
	if user == "" || !a.CanRepository(meta, user, AdminRepo) {
		return false
	}
	if kind != "owner" {
		return true
	}
	if meta["owner_type"] == "user" {
		return meta["owner_id"] == user
	}
	role := a.orgRole(a.orgBySlugOrID(strOf(meta["owner_id"])), user)
	return role == "owner" || role == "admin"
}
func pagesKind(r *http.Request) string {
	if r.URL.Query().Get("kind") == "owner" {
		return "owner"
	}
	return "project"
}
func pagesRef(ref string) string {
	if strings.HasPrefix(ref, "refs/") {
		return ref
	}
	return "refs/heads/" + ref
}
func shellLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
func pagesBuildJob(config PagesConfig, kind string) actions.Job {
	directory := config.OutputDirectory
	fallback := ""
	if config.SPAFallback != "" {
		fallback = "/" + config.SPAFallback
	}
	if config.WorkingDirectory != "." {
		directory = config.WorkingDirectory + "/" + directory
	}
	return actions.Job{ID: "pages-" + kind, Name: "Pages " + kind, Steps: []actions.Step{{ID: "build", Command: "cd " + shellLiteral(config.WorkingDirectory) + " && export SWITCHYARD_PAGES_BASE_PATH=" + shellLiteral(config.basePath()) + " && " + config.BuildCommand, TimeoutMS: 120000}}, Static: &actions.StaticOutput{Directory: directory, BasePath: config.basePath(), SPAFallback: fallback}}
}

// Internal reuse of the existing Actions approval handler preserves its Worker
// publication and definition-setting conflict semantics.
type actionResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *actionResponse) Header() http.Header    { return c.header }
func (c *actionResponse) WriteHeader(status int) { c.status = status }
func (c *actionResponse) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	return c.body.Write(p)
}
func (a *App) handlePagesConfig(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	kind := pagesKind(r)
	if !a.CanRepository(meta, user, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	id := pagesSiteID(strOf(meta["id"]), kind)
	rid, version, site, err := a.Trestle.FindRecord("pages_sites", filterEq("id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_config_unavailable"})
		return
	}
	if r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"site": site, "version": version, "can_configure": a.pagesAuthority(meta, user, kind), "can_deploy": user != "" && a.CanRepository(meta, user, WriteRepo), "native_hosting": "conditional_dns_tls"})
		return
	}
	if a.isDemoGuest(r) || !a.pagesAuthority(meta, user, kind) {
		writeJSON(w, 403, map[string]any{"error": "pages_admin_required"})
		return
	}
	var input struct {
		Version string      `json:"version"`
		Enabled bool        `json:"enabled"`
		Config  PagesConfig `json:"config"`
	}
	if readJSON(r, &input) != nil {
		writeJSON(w, 400, map[string]any{"error": "pages_config_invalid"})
		return
	}
	if input.Version != version {
		writeJSON(w, 409, map[string]any{"error": "pages_config_conflict"})
		return
	}
	config := input.Config
	config.RepositoryID = strOf(meta["id"])
	config.Owner = r.PathValue("owner")
	config.Project = r.PathValue("repo")
	if kind == "owner" {
		config.Project = ""
	}
	if err = config.validate(strOf(meta["visibility"]) == "private"); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	// Reserve mount identity permanently; disable/rename does not release it for
	// another repository. Root authority was checked separately above.
	mountID := "mount_" + sha256Hex([]byte(config.Owner + "\x00" + config.basePath()))[:24]
	_, _, mount, err := a.Trestle.FindRecord("pages_mounts", filterEq("id", mountID))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "pages_mount_unavailable"})
		return
	}
	if mount == nil {
		_, _, err = a.Trestle.CreateRecord("pages_mounts", map[string]any{"id": mountID, "site_id": id, "owner": config.Owner, "base_path": config.basePath(), "created_at": nowStr()}, mountID)
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "pages_mount_unavailable"})
			return
		}
		_, _, mount, err = a.Trestle.FindRecord("pages_mounts", filterEq("id", mountID))
	}
	if err != nil || mount["site_id"] != id {
		writeJSON(w, 409, map[string]any{"error": "pages_prefix_reserved"})
		return
	}
	if site != nil && (site["owner"] != config.Owner || site["project"] != config.Project) {
		writeJSON(w, 409, map[string]any{"error": "pages_rename_requires_reserved_migration"})
		return
	}
	_, _, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", strOf(meta["artifact_name"])))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_settings_unavailable"})
		return
	}
	var definition actions.Definition
	if settings != nil {
		_, _, record, e := a.Trestle.FindRecord("action_definitions", filterEq("id", actionDefinitionID(strOf(meta["artifact_name"]), strOf(settings["revision"]))))
		if e != nil || record == nil || decodeAction(record["definition"], &definition) != nil {
			writeJSON(w, 502, map[string]any{"error": "definition_unavailable"})
			return
		}
	}
	job := pagesBuildJob(config, kind)
	jobs := []actions.Job{}
	for _, old := range definition.Jobs {
		if old.ID != job.ID {
			jobs = append(jobs, old)
		}
	}
	jobs = append(jobs, job)
	refs := definition.Refs
	ref := pagesRef(config.Ref)
	found := false
	for _, old := range refs {
		if old == ref {
			found = true
		}
	}
	if !found {
		refs = append(refs, ref)
	}
	exported := map[string]any{"refs": refs, "jobs": jobs}
	if definition.Release != nil {
		if !strings.HasPrefix(ref, "refs/tags/") {
			writeJSON(w, 409, map[string]any{"error": "pages_branch_conflicts_with_tag_only_release_definition"})
			return
		}
		exported["release"] = definition.Release
	}
	encoded, _ := json.Marshal(exported)
	source := "export default " + string(encoded)
	approved, err := actions.CompileDefinition(source)
	if err != nil {
		writeJSON(w, 422, map[string]any{"error": "pages_definition_invalid"})
		return
	}
	approvals := map[string]any{}
	if site != nil {
		_ = decodeAction(site["approvals"], &approvals)
	}
	if len(approvals) >= 1000 && approvals[approved.Revision] == nil {
		writeJSON(w, 409, map[string]any{"error": "pages_approval_history_limit"})
		return
	}
	approval, _ := json.Marshal(map[string]any{"source": source, "expected_revision": strOf(settings["revision"]), "required_jobs": settings["required_jobs"]})
	cloned := r.Clone(r.Context())
	cloned.Body = http.NoBody
	cloned.Body = io.NopCloser(bytes.NewReader(approval))
	cloned.ContentLength = int64(len(approval))
	capture := &actionResponse{header: http.Header{}}
	a.handleActionDefinition(capture, cloned)
	if capture.status != 200 {
		w.WriteHeader(capture.status)
		_, _ = w.Write(capture.body.Bytes())
		return
	}
	approvals[approved.Revision] = map[string]any{"config": config, "approved_by": user}
	values := map[string]any{"id": id, "repository_id": meta["id"], "owner": config.Owner, "project": config.Project, "enabled": fmt.Sprint(input.Enabled), "approved_by": user, "definition_revision": approved.Revision, "job_id": job.ID, "config": config, "approvals": approvals, "updated_at": nowStr()}
	if rid == "" {
		values["created_at"] = nowStr()
		_, _, err = a.Trestle.CreateRecord("pages_sites", values, "pages-site-"+id)
	} else {
		err = a.Trestle.PatchRecord("pages_sites", rid, version, values)
	}
	if err != nil {
		writeJSON(w, 409, map[string]any{"error": "pages_config_conflict"})
		return
	}
	_, next, current, err := a.Trestle.FindRecord("pages_sites", filterEq("id", id))
	if err != nil || !sameActionJSON(current["config"], config) || current["definition_revision"] != approved.Revision {
		writeJSON(w, 409, map[string]any{"error": "pages_config_conflict"})
		return
	}
	writeJSON(w, 200, map[string]any{"site": current, "version": next, "url": "https://" + config.Owner + ".switchyard.cx" + config.basePath(), "base_path": config.basePath()})
}
