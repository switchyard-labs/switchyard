package refs

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ValidatePath accepts canonical repository-relative paths only. URL decoding
// belongs to HTTP, never to the filesystem; encoded aliases are rejected too.
func ValidatePath(name string) error {
	if name == "" || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return fmt.Errorf("invalid repository path")
	}
	decoded, err := url.PathUnescape(name)
	if err != nil || decoded != name {
		return fmt.Errorf("encoded repository path")
	}
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." || strings.EqualFold(p, ".git") || strings.Contains(p, ":") {
			return fmt.Errorf("reserved repository path")
		}
	}
	return nil
}

// ContainedPath rejects every symlink component, including internal aliases to
// Git metadata. Callers operate only inside private, disposable clones.
func ContainedPath(root, name string) (string, error) {
	if err := ValidatePath(name); err != nil {
		return "", err
	}
	p := root
	for _, part := range strings.Split(name, "/") {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink repository path")
		}
	}
	return p, nil
}
