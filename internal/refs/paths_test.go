package refs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepositoryPaths(t *testing.T) {
	for _, name := range []string{"../escape", "/tmp/x", "a/../../x", ".git/config", "a/.GiT/hooks/x", "a//b", "a/./b", "a\\b", "C:/x", "%2e%2e/x", "%252e%252e/x", "a\x00b"} {
		if ValidatePath(name) == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"README.md", "a/b/c.go", ".github/workflows/ci.js", "space name.txt"} {
		if err := ValidatePath(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
func TestSymlinksCannotWriteOrRead(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".git"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"escape/secret", "escape", "alias/config"} {
		if _, err := ContainedPath(root, p); err == nil {
			t.Errorf("accepted %s", p)
		}
	}
	if _, err := ContainedPath(root, "new/directory/file.go"); err != nil {
		t.Fatal(err)
	}
}
