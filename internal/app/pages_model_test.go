package app

import (
	"strings"
	"testing"
)

func TestPagesExplicitSourceAndPrefixes(t *testing.T) {
	p := PagesConfig{RepositoryID: "repo1", Owner: "strut-labs", Ref: "main", WorkingDirectory: ".", BuildCommand: "nift build", OutputDirectory: "public"}
	if err := p.validate(false); err != nil {
		t.Fatal(err)
	}
	if p.basePath() != "/" {
		t.Fatal(p.basePath())
	}
	for _, name := range []string{"foo-bar", "foo.js", "foo_bar", "foo-1.2"} {
		p.Project = name
		if err := p.validate(false); err != nil {
			t.Fatal(name, err)
		}
		if p.basePath() != "/"+name+"/" {
			t.Fatal(name)
		}
	}
	if p.validate(true) == nil {
		t.Fatal("private source silently published")
	}
	p.PublicAcknowledged = true
	if err := p.validate(true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"..", ".", ".git", "x/y", "x%2fy", "x\\y"} {
		p.Project = name
		if p.validate(false) == nil {
			t.Fatal("accepted project", name)
		}
	}
}
func TestPagesDirectoryContainment(t *testing.T) {
	for _, name := range []string{"../public", "foo/../../public", "/public", "foo//bar", "foo/./bar", ".git", "foo/.git", "public\\secret", "public\x00"} {
		if pagesRelativePath(name, false) {
			t.Fatal("accepted", name)
		}
	}
	for _, name := range []string{"public", "dist/site", "docs.v2"} {
		if !pagesRelativePath(name, false) {
			t.Fatal("rejected", name)
		}
	}
	if pagesRelativePath(".", false) || !pagesRelativePath(".", true) {
		t.Fatal("root policy")
	}
}

func TestPagesSourceResolution(t *testing.T) {
	refs := map[string]string{"refs/heads/main": strings.Repeat("a", 40), "refs/heads/website": strings.Repeat("b", 40), "refs/heads/feature/docs": strings.Repeat("c", 40), "refs/tags/v1": strings.Repeat("d", 40), "refs/tags/v1^{}": strings.Repeat("e", 40)}
	for ref, want := range map[string]string{"main": "a", "refs/heads/main": "a", "website": "b", "feature/docs": "c", "refs/tags/v1": "e"} {
		got, err := pagesResolvedSource(refs, ref)
		if err != nil || got != strings.Repeat(want, 40) {
			t.Fatalf("%s: %s %v", ref, got, err)
		}
	}
	for _, ref := range []string{"missing", "refs/tags/missing", "refs/pull/1", strings.Repeat("a", 40), "foo..bar", "foo.lock", "foo bar", "foo@{1}", "foo\\bar", "foo//bar"} {
		if _, err := pagesResolvedSource(refs, ref); err == nil {
			t.Fatal("accepted", ref)
		}
	}
}

func TestPagesSPAFallbackIsExplicitAndContained(t *testing.T) {
	config := PagesConfig{RepositoryID: "repo", Owner: "n-ham", Project: "foo.js", Ref: "website", WorkingDirectory: "site", BuildCommand: "nift build", OutputDirectory: "public", SPAFallback: "index.html"}
	if err := config.validate(false); err != nil {
		t.Fatal(err)
	}
	job := pagesBuildJob(config, "project")
	if err := job.Static.Validate(); err != nil {
		t.Fatal(err)
	}
	if job.Static.SPAFallback != "/index.html" || job.Static.Directory != "site/public" {
		t.Fatal(job.Static)
	}
	config.SPAFallback = "../owner/index.html"
	if config.validate(false) == nil {
		t.Fatal("escaping fallback accepted")
	}
}

func TestPagesInfrastructureOwnersReserved(t *testing.T) {
	p := PagesConfig{RepositoryID: "repo", Ref: "main", WorkingDirectory: ".", BuildCommand: "nift build", OutputDirectory: "public"}
	for _, owner := range []string{"www", "docs", "app", "api", "admin", "assets", "static", "pages", "login", "signup", "sy", "dpl-abc"} {
		p.Owner = owner
		if p.validate(false) == nil {
			t.Fatal("infrastructure owner accepted", owner)
		}
	}
}
