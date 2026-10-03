package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"switchyard/internal/refs"
)

// Semantic-conflict detection (post-cp13 review experiment).
//
// The claim tested here: "Git says the combined tree is fine, but Switchyard
// knows the two pieces of work are incompatible." A repo may declare a
// deterministic contract (`switchyard.contract.json` at the repo root). After
// preview integration produces a clean merged tree, Switchyard validates the
// contract against that tree. If the merged tree violates the contract, the
// change is a SEMANTIC conflict: a structured error finding + a blocked
// integration, even though `git merge` was clean.
//
// Contract rule (v1 experiment):
//   { "rules": [ { "kind": "field_equals",
//                   "a": "api/version.json:version",
//                   "b": "consumers/consumer.json:requires_version" } ] }
// "a" and "b" are "<path>:<dotted-field>" selectors into JSON files of the
// merged tree; the rule passes when the selected fields are equal.

type contractRule struct {
	Kind string `json:"kind"`
	A    string `json:"a"`
	B    string `json:"b"`
}

// semanticFindings validates repo contracts against the clean merged tree of
// branch into base. Returns nil when there is no contract, when the merge is
// textually conflicted (handled elsewhere), or when the contract passes.
func (a *App) semanticFindings(repo, branch string) ([]map[string]any, error) {
	rr, err := a.Artifacts.GetRepo(repo)
	if err != nil {
		return nil, err
	}
	base := rr.DefaultBranch
	dir, err := a.Refs.PreviewMergedTree(repo, base, branch)
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil // textual conflict: not a semantic check case
	}
	defer removeAll(dir)

	contractPath, err := refs.ContainedPath(dir, "switchyard.contract.json")
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(contractPath)
	if err != nil {
		return nil, nil // no contract declared
	}
	var contract struct {
		Rules []contractRule `json:"rules"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		return nil, fmt.Errorf("semantic contract parse: %v", err)
	}
	findings := []map[string]any{}
	for _, rule := range contract.Rules {
		if rule.Kind != "field_equals" {
			continue
		}
		va, errA := mergedJSONField(dir, rule.A)
		if errA != nil {
			if os.IsNotExist(errA) {
				continue // file absent from the merged tree: rule not applicable
			}
			findings = append(findings, map[string]any{
				"severity": "error", "message": "semantic conflict: cannot read " + rule.A + ": " + errA.Error(), "file": fileA(rule.A),
			})
			continue
		}
		vb, errB := mergedJSONField(dir, rule.B)
		if errB != nil {
			if os.IsNotExist(errB) {
				continue
			}
			findings = append(findings, map[string]any{
				"severity": "error", "message": "semantic conflict: cannot read " + rule.B + ": " + errB.Error(), "file": fileA(rule.B),
			})
			continue
		}
		if va != vb {
			findings = append(findings, map[string]any{
				"severity": "error",
				"message":  "semantic conflict: git merge is clean but " + rule.A + " (" + va + ") != " + rule.B + " (" + vb + ")",
				"file":     fileA(rule.A),
			})
		}
	}
	return findings, nil
}

// mergedJSONField reads a JSON file from the merged tree and returns the
// dotted-field value as a string.
func mergedJSONField(dir, selector string) (string, error) {
	parts := strings.SplitN(selector, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("selector %q must be path:field", selector)
	}
	filePath, err := refs.ContainedPath(dir, parts[0])
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		return "", err
	}
	cur := any(obj)
	for _, f := range strings.Split(parts[1], ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", fmt.Errorf("field %s not an object", f)
		}
		cur, ok = m[f]
		if !ok {
			return "", fmt.Errorf("field %s missing", f)
		}
	}
	switch v := cur.(type) {
	case string:
		return v, nil
	case float64:
		return fmt.Sprintf("%v", v), nil
	case bool:
		return fmt.Sprintf("%v", v), nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

func fileA(selector string) string {
	parts := strings.SplitN(selector, ":", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return selector
}
