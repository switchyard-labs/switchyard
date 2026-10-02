package app

import "testing"

func TestRepositoryRouteParsing(t *testing.T) {
	cases := []struct{ path, owner, repo, kind, ref, file string }{
		{"/nick/demo", "nick", "demo", "", "", ""},
		{"/switchyard-labs/switchyard/blob/main/internal/app/server.go", "switchyard-labs", "switchyard", "blob", "main", "internal/app/server.go"},
		{"/acme/widget/tree/feature-x/docs", "acme", "widget", "tree", "feature-x", "docs"},
	}
	for _, tc := range cases {
		r, ok := parseRepositoryRoute(tc.path)
		if !ok || r.Owner != tc.owner || r.Repo != tc.repo || r.Kind != tc.kind || r.Ref != tc.ref || r.Path != tc.file {
			t.Fatalf("parseRepositoryRoute(%q) = %#v,%v", tc.path, r, ok)
		}
	}
	for _, bad := range []string{"/assets/css/app.css", "/api/repos/x", "/settings/profile", "/only-one"} {
		if _, ok := parseRepositoryRoute(bad); ok {
			t.Fatalf("reserved/non-repo path parsed: %s", bad)
		}
	}
}

func TestOwnerAndRepoSlugs(t *testing.T) {
	if !validOwnerSlug("switchyard-labs") || validOwnerSlug("assets") || validOwnerSlug("Bad_Name") {
		t.Fatal("owner slug validation")
	}
	if !validRepoSlug("switchyard.v2") || !validRepoSlug("repo_name") || validRepoSlug("bad repo") {
		t.Fatal("repo slug validation")
	}
}
