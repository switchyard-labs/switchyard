package app

import (
	"net/http"
)

// handleWorkProvenance assembles the durable provenance chain for a Work item:
// intent -> attempts -> runs -> PRs -> ref mutations -> normalized events.
// It is a derived view over existing coordination records (no raw model text).
func (a *App) handleWorkProvenance(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	workID := r.PathValue("id")
	chain := []map[string]any{}
	work, err := a.Trestle.ListRecords("work", `id = "`+workID+`"`)
	if err == nil && len(work) > 0 {
		chain = append(chain, map[string]any{"kind": "work", "id": workID, "title": work[0]["title"], "owner": work[0]["owner"], "status": work[0]["status"]})
	}
	// attempts
	if attempts, err := a.Trestle.ListRecords("attempts", ""); err == nil {
		for _, at := range attempts {
			if at["work_id"] == workID {
				chain = append(chain, map[string]any{"kind": "attempt", "id": at["id"], "repo": at["repo"], "branch": at["branch"], "status": at["status"]})
				aid, _ := at["id"].(string)
				// runs
				if runs, err := a.Trestle.ListRecords("runs", ""); err == nil {
					for _, run := range runs {
						if run["attempt_id"] == aid {
							chain = append(chain, map[string]any{"kind": "run", "attempt_id": aid, "new_sha": run["new_sha"], "message": run["message"], "at": run["created_at"]})
						}
					}
				}
				// PRs from this attempt
				if prs, err := a.Trestle.ListRecords("prs", ""); err == nil {
					for _, pr := range prs {
						if pr["attempt_id"] == aid {
							chain = append(chain, map[string]any{"kind": "pr", "id": pr["id"], "repo": pr["repo"], "branch": pr["branch"], "base": pr["base"], "status": pr["status"], "check_status": pr["check_status"]})
						}
					}
				}
			}
		}
	}
	// ref mutations for any repo touched by this work
	repos := map[string]bool{}
	if attempts, err := a.Trestle.ListRecords("attempts", ""); err == nil {
		for _, at := range attempts {
			if at["work_id"] == workID {
				repos[at["repo"].(string)] = true
			}
		}
	}
	if refs, err := a.Trestle.ListRecords("ref_updates", ""); err == nil {
		for _, ru := range refs {
			if repos[ru["repo"].(string)] {
				chain = append(chain, map[string]any{"kind": "ref_update", "repo": ru["repo"], "branch": ru["branch"], "old_sha": ru["old_sha"], "new_sha": ru["new_sha"], "provenance": ru["provenance"]})
			}
		}
	}
	// recent normalized events for those repos
	if evs, err := a.Trestle.ListRecords("events", ""); err == nil {
		for _, ev := range evs {
			if ev["repo_name"] != nil && repos[ev["repo_name"].(string)] {
				chain = append(chain, map[string]any{"kind": "event", "type": ev["type"], "repo": ev["repo_name"], "at": ev["occurred_at"]})
			}
		}
	}
	writeJSON(w, 200, map[string]any{"work_id": workID, "chain": chain})
}
