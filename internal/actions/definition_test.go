package actions

import (
	"strings"
	"testing"
)

const sampleDefinition = `const timeout=30000; export default {refs:['refs/heads/main'],jobs:[{id:'test',steps:[{id:'unit',command:'npm test',timeout_ms:timeout}]}]};`

func TestCompileDefinitionBindsSource(t *testing.T) {
	a, err := CompileDefinition(sampleDefinition)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CompileDefinition(sampleDefinition + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if a.Revision == b.Revision || a.Source != sampleDefinition || a.Jobs[0].Steps[0].TimeoutMS != 30000 {
		t.Fatal("definition lost source identity or JavaScript evaluation")
	}
}
func TestDefinitionRejectsUnboundedOrHostAccess(t *testing.T) {
	for _, source := range []string{`export default process.env;`, `export default {refs:['refs/heads/../../x'],jobs:[]};`, strings.Repeat("x", 65537), `export default {}; while(true){}`} {
		if _, err := CompileDefinition(source); err == nil {
			t.Fatal("unsafe definition accepted")
		}
	}
}
func TestDefinitionRejectsDuplicateStep(t *testing.T) {
	d, err := CompileDefinition(sampleDefinition)
	if err != nil {
		t.Fatal(err)
	}
	d.Jobs[0].Steps = append(d.Jobs[0].Steps, d.Jobs[0].Steps[0])
	if d.Validate() == nil {
		t.Fatal("duplicate step accepted")
	}
}

func TestDefinitionRejectsAmbiguousLogIdentity(t *testing.T) {
	d, err := CompileDefinition(sampleDefinition)
	if err != nil {
		t.Fatal(err)
	}
	d.Jobs = []Job{{ID: "a-b", Steps: []Step{{ID: "c", Command: "true", TimeoutMS: 1000}}}, {ID: "a", Steps: []Step{{ID: "b-c", Command: "true", TimeoutMS: 1000}}}}
	if d.Validate() == nil {
		t.Fatal("colliding log/step keys accepted")
	}
}
