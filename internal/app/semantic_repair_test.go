package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticRepairScopeAndCounterpartContext(t *testing.T) {
	tree := t.TempDir()
	for name, data := range map[string]string{"switchyard.contract.json": `{"rules":[{"kind":"field_equals","a":"api.json:version","b":"consumer.json:requires"}]}`, "api.json": `{"version":2}`, "consumer.json": `{"requires":3}`} {
		if err := os.WriteFile(filepath.Join(tree, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths, prompt, err := semanticRepairInputs(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "api.json" || !strings.Contains(prompt, "consumer.json") {
		t.Fatalf("repair scope/context: %v", paths)
	}
	if err = os.WriteFile(filepath.Join(tree, "api.json"), []byte(`{"version":3}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = semanticRepairInputs(tree); err == nil {
		t.Fatal("accepted already consistent tree")
	}
}
func TestSemanticRepairRejectsSymlinkCounterpart(t *testing.T) {
	tree := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte(`{"requires":3}`), 0600)
	os.WriteFile(filepath.Join(tree, "switchyard.contract.json"), []byte(`{"rules":[{"kind":"field_equals","a":"api.json:version","b":"consumer.json:requires"}]}`), 0600)
	os.WriteFile(filepath.Join(tree, "api.json"), []byte(`{"version":2}`), 0600)
	if err := os.Symlink(outside, filepath.Join(tree, "consumer.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := semanticRepairInputs(tree); err == nil {
		t.Fatal("read symlink counterpart")
	}
}
func TestResolverOutputCannotWidenWriteScope(t *testing.T) {
	for _, result := range []map[string]string{{"other": "value"}, {"api.json": "{}", "other": "value"}, {"api.json": "<<<<<<< left"}, {"api.json": strings.Repeat("a", (1<<20)+1)}} {
		if _, err := validateResolverFile(result, "api.json"); err == nil {
			t.Fatal("accepted invalid output")
		}
	}
}
