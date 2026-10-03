package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSemanticContractFailsClosedAndPreservesTypes(t *testing.T) {
	for _, test := range []struct {
		name, contract, a, b   string
		wantError, wantFinding bool
	}{
		{"absent", "", "", "", false, false},
		{"equal", `{"rules":[{"kind":"field_equals","a":"a.json:value","b":"b.json:value"}]}`, `{"value":1}`, `{"value":1}`, false, false},
		{"different_types", `{"rules":[{"kind":"field_equals","a":"a.json:value","b":"b.json:value"}]}`, `{"value":1}`, `{"value":"1"}`, false, true},
		{"missing_file", `{"rules":[{"kind":"field_equals","a":"a.json:value","b":"b.json:value"}]}`, `{"value":1}`, "", false, true},
		{"unknown_rule", `{"rules":[{"kind":"ignore_me"}]}`, "", "", true, false},
		{"malformed", `{"rules":`, "", "", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range map[string]string{"switchyard.contract.json": test.contract, "a.json": test.a, "b.json": test.b} {
				if content != "" {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			findings, err := semanticFindingsInTree(dir)
			if (err != nil) != test.wantError || (len(findings) > 0) != test.wantFinding {
				t.Fatalf("findings=%v error=%v", findings, err)
			}
		})
	}
}
