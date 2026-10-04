package app

import "net/http"

// Intake is an explicit repository policy. Storage failures deny new intake.
func (a *App) proposalIntakeAllowed(meta map[string]any, user string) bool {
	if user == "" || !a.CanRepository(meta, user, ReadRepo) {
		return false
	}
	if a.CanRepository(meta, user, WriteRepo) {
		return true
	}
	_, _, settings, err := a.Trestle.FindRecord("proposal_settings", filterEq("repository_id", strOr(meta["id"])))
	return err == nil && strOr(settings["intake"]) == "readers"
}
func (a *App) handleProposalSettings(w http.ResponseWriter, r *http.Request) {
	meta := a.repositoryMetaByOwner(r.PathValue("owner"), r.PathValue("repo"))
	user := a.currentUser(r)
	if !a.CanRepository(meta, user, ReadRepo) {
		writeJSON(w, 404, map[string]any{"error": "repository_not_found"})
		return
	}
	rid, version, settings, err := a.Trestle.FindRecord("proposal_settings", filterEq("repository_id", strOr(meta["id"])))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": "proposal_settings_unavailable"})
		return
	}
	if r.Method == "GET" {
		intake := strOr(settings["intake"])
		if intake == "" {
			intake = "writers"
		}
		writeJSON(w, 200, map[string]any{"intake": intake, "version": version, "can_create": a.proposalIntakeAllowed(meta, user), "can_configure": user != "" && a.CanRepository(meta, user, AdminRepo)})
		return
	}
	if user == "" || a.isDemoGuest(r) {
		writeJSON(w, 401, map[string]any{"error": "sign_in_required"})
		return
	}
	if !a.CanRepository(meta, user, AdminRepo) {
		writeJSON(w, 403, map[string]any{"error": "proposal_settings_admin_required"})
		return
	}
	var in struct {
		Version string `json:"version"`
		Intake  string `json:"intake"`
	}
	if readJSON(r, &in) != nil || (in.Intake != "writers" && in.Intake != "readers") {
		writeJSON(w, 400, map[string]any{"error": "proposal_intake_invalid"})
		return
	}
	if rid == "" {
		// Initialization never acknowledges the caller's desired policy: after a
		// competing create, the caller must read and CAS the authoritative record.
		_, _, err = a.Trestle.CreateRecord("proposal_settings", map[string]any{"repository_id": meta["id"], "intake": "writers", "updated_by": user, "updated_at": nowStr()}, "proposal-settings-"+strOr(meta["id"]))
		if err != nil {
			writeJSON(w, 502, map[string]any{"error": "proposal_settings_unavailable"})
			return
		}
		writeJSON(w, 409, map[string]any{"error": "proposal_settings_initialized_reload_required"})
		return
	}
	if in.Version == "" || in.Version != version {
		writeJSON(w, 409, map[string]any{"error": "proposal_settings_version_conflict"})
		return
	}
	patch := map[string]any{"intake": in.Intake, "updated_by": user, "updated_at": nowStr()}
	if err = a.Trestle.PatchRecord("proposal_settings", rid, version, patch); err != nil {
		writeJSON(w, 409, map[string]any{"error": "proposal_settings_version_conflict"})
		return
	}
	writeJSON(w, 200, patch)
}
