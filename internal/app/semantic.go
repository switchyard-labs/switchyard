package app

import (
	"encoding/json"
	"fmt"
	"io"
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

	return semanticFindingsInTree(dir)
}

func semanticFindingsInTree(dir string) ([]map[string]any, error) {

	contractPath, err := refs.ContainedPath(dir, "switchyard.contract.json")
	if err != nil {
		return nil, err
	}
	raw, err := boundedSemanticRead(contractPath, 128<<10)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var contract struct {
		Rules []contractRule `json:"rules"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		return nil, fmt.Errorf("semantic contract parse: %v", err)
	}
	if len(contract.Rules) > 100 {
		return nil, fmt.Errorf("semantic rule count exceeds 100")
	}
	// Bound aggregate work as well as individual files. Cache selector results.
	remaining := int64(8 << 20)
	cache := map[string]string{}
	readField := func(selector string) (string, error) {
		if value, ok := cache[selector]; ok {
			return value, nil
		}
		value, err := boundedMergedJSONField(dir, selector, &remaining)
		if err == nil {
			cache[selector] = value
		}
		return value, err
	}
	findings := []map[string]any{}
	for _, rule := range contract.Rules {
		if rule.Kind != "field_equals" {
			return nil, fmt.Errorf("unsupported semantic rule %q", rule.Kind)
		}
		va, errA := readField(rule.A)
		if errA != nil {
			findings = append(findings, map[string]any{
				"severity": "error", "message": "semantic conflict: cannot read " + rule.A + ": " + errA.Error(), "file": fileA(rule.A),
			})
			continue
		}
		vb, errB := readField(rule.B)
		if errB != nil {
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
	remaining := int64(8 << 20)
	return boundedMergedJSONField(dir, selector, &remaining)
}

func boundedMergedJSONField(dir, selector string, remaining *int64) (string, error) {
	parts := strings.SplitN(selector, ":", 2)
	if len(selector) > 1024 || len(parts) != 2 || len(strings.Split(parts[1], ".")) > 32 {
		return "", fmt.Errorf("selector %q must be path:field", selector)
	}
	filePath, err := refs.ContainedPath(dir, parts[0])
	if err != nil {
		return "", err
	}
	b, err := boundedSemanticRead(filePath, min(1<<20, *remaining))
	if err != nil {
		return "", err
	}
	*remaining -= int64(len(b))
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
	encoded, err := json.Marshal(cur)
	return string(encoded), err
}

func fileA(selector string) string {
	parts := strings.SplitN(selector, ":", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	return selector
}

func boundedSemanticRead(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("semantic input exceeds limit or is not a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("semantic input exceeds limit")
	}
	return b, nil
}
