package app

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"switchyard/internal/artifacts"
	"testing"
)

type archiveTransport struct {
	t         *testing.T
	sha       string
	link      string
	oversized bool
	refs      int
}

func (f *archiveTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	var data []byte
	switch {
	case strings.Contains(r.URL.Path, "/log"):
		ref := r.URL.Query().Get("ref")
		if ref == "main" {
			f.refs++
		} else if ref != f.sha {
			f.t.Fatalf("mutable read %s", ref)
		}
		data, _ = json.Marshal(map[string]any{"result": []map[string]any{{"hash": f.sha, "treeHash": "tree"}}})
	case strings.Contains(r.URL.Path, "/tree/"):
		data, _ = json.Marshal(map[string]any{"result": []map[string]any{{"name": "hello.txt", "mode": "100755", "type": "blob"}, {"name": "link", "mode": "120000", "type": "blob"}}})
	case strings.Contains(r.URL.Path, "/raw/"):
		if !strings.Contains(r.URL.Path, "/raw/"+f.sha+"/") {
			f.t.Fatal("blob not SHA pinned")
		}
		data = []byte("hello exact tree\n")
		if strings.HasSuffix(r.URL.Path, "/link") {
			data = []byte(f.link)
		}
		if f.oversized {
			data = bytes.Repeat([]byte("x"), int(archiveBlobLimit)+1)
		}
	default:
		f.t.Fatalf("unexpected request %s", r.URL.Path)
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data))}, nil
}
func archiveFixture(t *testing.T, visibility string) (*App, *archiveTransport) {
	t.Helper()
	a := securityFixture(t, map[string][]map[string]any{"repository_meta": {{"full_name": "alice/demo", "owner_slug": "alice", "slug": "demo", "owner_type": "user", "owner_id": "alice", "visibility": visibility, "artifact_name": "physical"}}})
	helper := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\nprintf fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	f := &archiveTransport{t: t, sha: strings.Repeat("a", 40), link: "hello.txt"}
	a.Artifacts = artifacts.NewWithHTTP("fixture", "fixture", helper, &http.Client{Transport: f})
	return a, f
}
func requestArchive(a *App, user, format string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/alice/demo/archive/main."+format, nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("repo", "demo")
	r.SetPathValue("archive", "main."+format)
	r = r.WithContext(contextWithUser(r.Context(), user))
	w := httptest.NewRecorder()
	a.handleRepositoryArchive(w, r)
	return w
}
func TestArchivePermissionsAndExactSHA(t *testing.T) {
	for _, visibility := range []string{"public", "private", "internal"} {
		for _, user := range []string{"", "alice", "outsider"} {
			a, f := archiveFixture(t, visibility)
			w := requestArchive(a, user, "zip")
			allowed := visibility == "public" || user == "alice"
			if !allowed {
				if w.Code != 403 || f.refs != 0 {
					t.Fatal(visibility, user, w.Code, f.refs)
				}
				continue
			}
			if w.Code != 200 || f.refs != 1 || w.Header().Get("X-Switchyard-Commit") != f.sha {
				t.Fatal(visibility, user, w.Code, w.Body.String())
			}
			z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
			if err != nil {
				t.Fatal(err)
			}
			if len(z.File) != 2 || z.File[0].Name != "demo-aaaaaaaaaaaa/hello.txt" || z.File[0].Mode().Perm() != 0755 || z.File[1].Mode()&os.ModeSymlink == 0 {
				t.Fatal("zip tree/mode mismatch")
			}
			file, err := z.File[0].Open()
			if err != nil {
				t.Fatal(err)
			}
			contents, _ := io.ReadAll(file)
			file.Close()
			if string(contents) != "hello exact tree\n" {
				t.Fatal("wrong blob")
			}
		}
	}
	a, _ := archiveFixture(t, "public")
	w := requestArchive(a, "", "tar.gz")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	gz, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	h, err := tr.Next()
	if err != nil || h.Mode != 0755 {
		t.Fatal(h, err)
	}
	data, _ := io.ReadAll(tr)
	if string(data) != "hello exact tree\n" {
		t.Fatal("tar blob mismatch")
	}
	h, err = tr.Next()
	if err != nil || h.Typeflag != tar.TypeSymlink || h.Linkname != "hello.txt" {
		t.Fatal(h, err)
	}
}
func TestArchivesRejectUnsafeSymlinksAndOversizedBlobs(t *testing.T) {
	for _, link := range []string{"../../escape", "/etc/passwd", ".git/config", "hello\\bad"} {
		a, f := archiveFixture(t, "public")
		f.link = link
		w := requestArchive(a, "", "zip")
		if w.Code != 422 {
			t.Fatal(link, w.Code)
		}
	}
	a, f := archiveFixture(t, "public")
	f.oversized = true
	w := requestArchive(a, "", "zip")
	if w.Code != 422 {
		t.Fatal(w.Code)
	}
}

func TestArchiveRoutesRegisterWithoutMuxConflicts(t *testing.T) {
	a, _ := archiveFixture(t, "public")
	_ = a.Handler()
}
