package app

import (
	"net/http"
	"path"
	"strings"
	"time"

	"switchyard/internal/trestle"
)

// CP12: organisations + policy/risk + audit + fleet view.
//
// Risk model: risk(class) = blast_radius x uncertainty. Blast radius from the
// change scope (files touched); uncertainty from conflict history/preview
// conflicts. Organisational policies are evaluated against actions (agent
// file writes, workflow budgets, integration risk) and can allow / deny /
// escalate (require human approval). The simple repo case is untouched when no
// policy covers it. Every decision is recorded in the audit trail.

func policyCollections() [][2]any {
	return [][2]any{
		{"orgs", []trestle.CollectionField{
			{Name: "id", Type: "text", Unique: true}, {Name: "name", Type: "text"},
			{Name: "owner", Type: "text"}, {Name: "members", Type: "json"},
			{Name: "repos", Type: "json"}, {Name: "created_at", Type: "text"}}},
		{"policies", []trestle.CollectionField{
			{Name: "id", Type: "text", Unique: true}, {Name: "org", Type: "text"},
			{Name: "name", Type: "text"}, {Name: "rules", Type: "json"}, {Name: "created_at", Type: "text"}}},
		{"audit", []trestle.CollectionField{
			{Name: "id", Type: "text", Unique: true}, {Name: "org", Type: "text"},
			{Name: "action", Type: "text"}, {Name: "subject", Type: "text"},
			{Name: "decision", Type: "text"}, {Name: "reason", Type: "text"},
			{Name: "actor", Type: "text"}, {Name: "at", Type: "text"}}},
	}
}

// riskScore computes blast_radius x uncertainty for a change.
func (a *App) riskScore(repo, branch string) (string, int, error) {
	changed, err := a.changedFiles(repo, branch)
	if err != nil {
		return "low", 1, err
	}
	blast := len(changed) // files touched
	if blast == 0 {
		blast = 1
	}
	uncertainty := 1
	// conflict history / preview conflicts raise uncertainty
	if conflicts, err := a.Refs.PreviewMerge(repo, "main", branch); err == nil && len(conflicts) > 0 {
		uncertainty = 3
	}
	score := blast * uncertainty
	class := "low"
	if score >= 9 {
		class = "high"
	} else if score >= 4 {
		class = "medium"
	}
	return class, score, nil
}

// policyForOrg returns the first policy of an org (simple model: one policy
// per org).
func (a *App) policyForOrg(org string) map[string]any {
	items, err := a.Trestle.ListRecords("policies", `org = "`+org+`"`)
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0]
}

// orgForRepo returns the org id that owns a repo, or "" if none.
func (a *App) orgForRepo(repo string) string {
	items, err := a.Trestle.ListRecords("orgs", "")
	if err != nil {
		return ""
	}
	for _, it := range items {
		repos, _ := it["repos"].([]any)
		for _, r := range repos {
			if r == repo {
				id, _ := it["id"].(string)
				return id
			}
		}
	}
	return ""
}

// policyCheck evaluates an org policy against an action. Returns decision:
// allow / deny / escalate + reason.
func (a *App) policyCheck(org, action, role, repo, filePath string, risk string, budget int) (string, string) {
	if org == "" {
		return "allow", "no policy"
	}
	pol := a.policyForOrg(org)
	if pol == nil {
		return "allow", "no policy"
	}
	rules, _ := pol["rules"].(map[string]any)
	switch action {
	case "agent":
		agentRules, _ := rules["agent"].(map[string]any)
		denyPaths, _ := agentRules["deny_paths"].([]any)
		for _, dp := range denyPaths {
			pat, _ := dp.(string)
			if ok, _ := path.Match(pat, filePath); ok {
				return "deny", "policy: implementer cannot write " + pat
			}
		}
		return "allow", "policy: agent allowed"
	case "integrate":
		ig, _ := rules["integrate"].(map[string]any)
		reqRisk, _ := ig["require_approval_risk"].(string)
		if reqRisk != "" && risk != "" && riskRank(risk) >= riskRank(reqRisk) {
			return "escalate", "policy: integration risk " + risk + " requires approval"
		}
		return "allow", "policy: integration allowed"
	case "workflow":
		wf, _ := rules["workflow"].(map[string]any)
		if maxSteps, ok := wf["budget_steps"].(float64); ok && budget > int(maxSteps) {
			return "deny", "policy: workflow budget exceeds " + itoa(int(maxSteps))
		}
		return "allow", "policy: workflow allowed"
	}
	return "allow", "policy: no rule"
}

func riskRank(r string) int {
	switch r {
	case "high":
		return 3
	case "medium":
		return 2
	}
	return 1
}

func (a *App) audit(org, action, subject, decision, reason, actor string) {
	if org == "" {
		return
	}
	_, _, _ = a.Trestle.CreateRecord("audit", map[string]any{
		"id": "aud_" + randHex(10), "org": org, "action": action, "subject": subject,
		"decision": decision, "reason": reason, "actor": actor, "at": nowStr(),
	}, "aud-"+randHex(8))
}

// ---- API ----

func (a *App) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Name    string   `json:"name"`
		Repos   []string `json:"repos"`
		Members []string `json:"members"`
	}
	if err := readJSON(r, &in); err != nil || in.Name == "" {
		writeJSON(w, 400, map[string]any{"error": "name_required"})
		return
	}
	orgSlug := normalizeOwnerSlug(in.Name)
	if !validOwnerSlug(orgSlug) {
		writeJSON(w, 400, map[string]any{"error": "org_name_invalid"})
		return
	}
	if ns, _ := a.Trestle.ListRecords("owner_namespaces", `slug = "`+orgSlug+`"`); len(ns) > 0 {
		writeJSON(w, 409, map[string]any{"error": "owner_namespace_taken"})
		return
	}
	id := "org_" + randHex(8)
	_, _, err := a.Trestle.CreateRecord("orgs", map[string]any{
		"id": id, "name": in.Name, "owner": user, "members": in.Members, "repos": in.Repos, "created_at": nowStr(),
	}, "org-"+id)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if err := a.ensureOwnerNamespace(orgSlug, "org", id); err != nil {
		writeJSON(w, 409, map[string]any{"error": "owner_namespace_conflict"})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "name": in.Name, "slug": orgSlug, "owner": user})
}

func (a *App) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" { writeJSON(w, 401, map[string]any{"error": "unauthorized"}); return }
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "owner")
	if !ok { return }
	org := strOr(o["id"])
	var in struct {
		Name  string         `json:"name"`
		Rules map[string]any `json:"rules"`
	}
	if err := readJSON(r, &in); err != nil || in.Name == "" {
		writeJSON(w, 400, map[string]any{"error": "name_required"})
		return
	}
	id := "pol_" + randHex(8)
	_, _, err := a.Trestle.CreateRecord("policies", map[string]any{
		"id": id, "org": org, "name": in.Name, "rules": in.Rules, "created_at": nowStr(),
	}, "pol-"+id)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "org": org, "name": in.Name})
}

func (a *App) handleFleet(w http.ResponseWriter, r *http.Request) {
	user := a.currentUser(r)
	if user == "" { writeJSON(w, 401, map[string]any{"error": "unauthorized"}); return }
	o, ok := a.requireOrgRole(w, r, r.PathValue("id"), "member")
	if !ok { return }
	org := strOr(o["id"])
	orgs, _ := a.Trestle.ListRecords("orgs", `id = "`+org+`"`)
	orgView := map[string]any{}
	if len(orgs) > 0 {
		orgView = orgs[0]
	}
	repos, _ := a.Trestle.ListRecords("repos", "")
	queue, _ := a.Trestle.ListRecords("iq", "")
	runs, _ := a.Trestle.ListRecords("workflow_runs", "")
	escs, _ := a.Trestle.ListRecords("escalations", "")
	writeJSON(w, 200, map[string]any{
		"org":           orgView,
		"repos":         len(repos),
		"queue_items":   len(queue),
		"workflow_runs": len(runs),
		"escalations":   len(escs),
		"queue":         queue,
	})
}

func (a *App) handleAudit(w http.ResponseWriter, r *http.Request) {
	if a.currentUser(r) == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	items, err := a.Trestle.ListRecords("audit", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

// policyGateAgent enforces agent-write policy at the run choke point. Returns
// an error when denied.
func (a *App) policyGateAgent(role, repo, filePath string) error {
	org := a.orgForRepo(repo)
	decision, reason := a.policyCheck(org, "agent", role, repo, filePath, "", 0)
	a.audit(org, "agent", repo+":"+filePath, decision, reason, role)
	if decision == "deny" {
		return &policyDenied{Reason: reason}
	}
	return nil
}

// policyGateIntegrate enforces integration policy; escalate means the item
// must be blocked for human approval.
func (a *App) policyGateIntegrate(repo string, risk string) (string, string) {
	org := a.orgForRepo(repo)
	decision, reason := a.policyCheck(org, "integrate", "", repo, "", risk, 0)
	a.audit(org, "integrate", repo, decision, reason, "integration-queue")
	return decision, reason
}

// policyGateWorkflow enforces workflow budget policy.
func (a *App) policyGateWorkflow(repo string, budget int) (string, string) {
	org := a.orgForRepo(repo)
	decision, reason := a.policyCheck(org, "workflow", "", repo, "", "", budget)
	a.audit(org, "workflow", repo, decision, reason, "workflow-runner")
	return decision, reason
}

type policyDenied struct{ Reason string }

func (e *policyDenied) Error() string { return e.Reason }

var _ = strings.TrimSpace
var _ = time.RFC3339
