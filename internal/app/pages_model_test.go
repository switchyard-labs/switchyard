package app

import "testing"

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
