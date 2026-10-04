package actions

import (
	"strings"
	"testing"
)

const sampleDefinition = `const timeout=30000; export default {refs:['refs/heads/main'],jobs:[{id:'test',steps:[{id:'unit',command:'npm test',timeout_ms:timeout}]}]};`

func TestReleaseDefinitionBindsExplicitTagsAndSafeUniqueOutputs(t *testing.T) {
	source := `export default {refs:['refs/tags/v1'],release:{publish:true,prerelease:false},jobs:[{id:'build',assets:[{name:'program.zip',path:'dist/program.zip'}],steps:[{id:'build',command:'make',timeout_ms:30000}]}]};`
	d, err := CompileDefinition(source)
	if err != nil || d.Release == nil || !d.Release.Publish || d.Jobs[0].Assets[0].Path != "dist/program.zip" {
		t.Fatalf("release configuration: %v", err)
	}
	for _, change := range [][2]string{{"refs/tags/v1", "refs/heads/main"}, {"dist/program.zip", "../secret"}, {"dist/program.zip", ".git/config"}, {"program.zip',path", "../program.zip',path"}} {
		if _, err := CompileDefinition(strings.Replace(source, change[0], change[1], 1)); err == nil {
			t.Fatal("unsafe release configuration accepted")
		}
	}
	d.Jobs = append(d.Jobs, Job{ID: "other", Steps: d.Jobs[0].Steps, Assets: d.Jobs[0].Assets})
	if d.Validate() == nil {
		t.Fatal("duplicate asset name across jobs accepted")
	}
}

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

func TestDefinitionCannotConfigurePrivilegedRunnerInputs(t *testing.T) {
	for _, source := range []string{
		`export default {refs:['refs/heads/main'],env:{CF_TOKEN:'value'},jobs:[{id:'test',steps:[{id:'test',command:'true',timeout_ms:1000}]}]}`,
		`export default {refs:['refs/heads/main'],jobs:[{id:'test',steps:[{id:'test',command:'true',timeout_ms:1000,cloudflareCredentials:true}]}]}`,
		`export default {refs:['refs/heads/main'],jobs:[{id:'test',secrets:['org-token'],steps:[{id:'test',command:'true',timeout_ms:1000}]}]}`,
	} {
		if _, err := CompileDefinition(source); err == nil {
			t.Fatal("privileged configuration accepted")
		}
	}
}

func TestStaticDirectoryOutputDoesNotRequireRelease(t *testing.T) {
	source := `export default {refs:["refs/heads/website"],jobs:[{id:"pages",steps:[{id:"build",command:"nift build",timeout_ms:120000}],static:{directory:"public",base_path:"/foo.js/"}}]}`
	d, err := CompileDefinition(source)
	if err != nil {
		t.Fatal(err)
	}
	if d.Release != nil || d.Jobs[0].Static == nil {
		t.Fatal("static artifact became a release")
	}
	for _, directory := range []string{"../secret", "/public", "foo//bar", "foo/.git", "foo\\bar"} {
		output := *d.Jobs[0].Static
		output.Directory = directory
		if output.Validate() == nil {
			t.Fatal("unsafe directory", directory)
		}
	}
	for _, base := range []string{"foo", "/../", "/x/y/", "/.git/", "/foo%2fbar/"} {
		output := *d.Jobs[0].Static
		output.BasePath = base
		if output.Validate() == nil {
			t.Fatal("unsafe base", base)
		}
	}
}
