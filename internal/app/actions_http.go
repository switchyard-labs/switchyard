package app

import "net/http"

func (a *App) canonicalActionsRepo(r *http.Request) string {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	return strOf(meta["artifact_name"])
}
func (a *App) handleActions(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	items, err := a.Trestle.ListRecords("action_runs", filterEq("repo", repo))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_unavailable"})
		return
	}
	_, _, settings, err := a.Trestle.FindRecord("action_settings", filterEq("repo", repo))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_settings_unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items, "settings": settings, "enabled": a.Actions != nil})
}
func (a *App) handleActionRun(w http.ResponseWriter, r *http.Request) {
	repo := a.canonicalActionsRepo(r)
	id := r.PathValue("id")
	_, _, run, err := a.Trestle.FindRecord("action_runs", filterEq("id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_unavailable"})
		return
	}
	if run == nil || run["repo"] != repo {
		writeJSON(w, 404, map[string]any{"error": "action_not_found"})
		return
	}
	jobs, err := a.Trestle.ListRecords("action_jobs", filterEq("run_id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_unavailable"})
		return
	}
	checks, err := a.Trestle.ListRecords("action_checks", filterEq("run_id", id))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "actions_unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"run": run, "jobs": jobs, "checks": checks, "enabled": a.Actions != nil})
}
