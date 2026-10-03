package app

import "strings"

// Fail closed on missing repository identity or unreadable policy state.
// All integration entry points use the queue, including workflow effects.
func (a *App) immutableIntegrationPolicy(repo, risk string) (string, string) {
	field := "artifact_name"
	if strings.Contains(repo, "/") {
		field = "full_name"
	}
	metas, err := a.Trestle.ListRecords("repository_meta", filterEq(field, repo))
	if err != nil || len(metas) != 1 {
		return "deny", "repository identity unavailable"
	}
	meta := metas[0]
	settings, err := a.Trestle.ListRecords("repository_settings", filterEq("repo_id", strOf(meta["id"])))
	if err != nil {
		return "deny", "repository settings unavailable"
	}
	for _, setting := range settings {
		if setting["archived"] == "true" {
			return "deny", "repository is archived"
		}
	}
	if meta["owner_type"] != "org" {
		return "allow", "queue validation required"
	}
	policies, err := a.Trestle.ListRecords("policies", filterEq("org", strOf(meta["owner_id"])))
	if err != nil {
		return "deny", "organization policy unavailable"
	}
	for _, policy := range policies {
		rules, ok := policy["rules"].(map[string]any)
		if !ok {
			return "deny", "invalid organization policy"
		}
		integration, ok := rules["integrate"].(map[string]any)
		if !ok {
			continue
		}
		threshold := strOf(integration["require_approval_risk"])
		if threshold != "" && riskRank(risk) >= riskRank(threshold) {
			return "escalate", "integration risk " + risk + " requires approval"
		}
	}
	return "allow", "immutable queue validation passed"
}
