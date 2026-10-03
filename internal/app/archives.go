package app

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const archiveBlobLimit int64 = 8 << 20
const archiveTotalLimit int64 = 128 << 20

// archiveLimitWriter bounds disk output as well as the uncompressed inventory.
type archiveLimitWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *archiveLimitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("archive output limit exceeded")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}
func safeArchiveLink(name, target string) bool {
	if target == "" || strings.ContainsAny(target, "\\\x00\r\n") || path.IsAbs(target) {
		return false
	}
	resolved := path.Clean(path.Join(path.Dir(name), target))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return false
	}
	for _, part := range strings.Split(resolved, "/") {
		if strings.EqualFold(part, ".git") {
			return false
		}
	}
	return true
}

// buildRepositoryArchive reads only the pinned SHA. It buffers a single bounded
// blob and stages the complete bounded archive privately before sending headers.
func (a *App) buildRepositoryArchive(ctx context.Context, repo, sha, root, format string, out io.Writer) error {
	entries, err := a.workspaceTree(ctx, repo, sha)
	if err != nil {
		return err
	}
	bounded := &archiveLimitWriter{out, archiveTotalLimit + (8 << 20)}
	var zw *zip.Writer
	var tw *tar.Writer
	var gz *gzip.Writer
	if format == "zip" {
		zw = zip.NewWriter(bounded)
	} else {
		gz = gzip.NewWriter(bounded)
		tw = tar.NewWriter(gz)
	}
	links := map[string]bool{}
	for _, entry := range entries {
		if entry.Mode == "120000" {
			links[entry.Path] = true
		}
	}
	for _, entry := range entries {
		for parent := path.Dir(entry.Path); parent != "."; parent = path.Dir(parent) {
			if links[parent] {
				return fmt.Errorf("archive entry beneath symlink")
			}
		}
	}
	total := int64(0)
	seen := map[string]bool{}
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return err
		}
		for _, part := range strings.Split(entry.Path, "/") {
			if strings.EqualFold(part, ".git") {
				return fmt.Errorf("Git administrative path excluded")
			}
		}
		if seen[entry.Path] {
			return fmt.Errorf("duplicate archive path")
		}
		seen[entry.Path] = true
		if !regularWorkspaceFile(entry) && !(entry.Type == "blob" && entry.Mode == "120000") {
			return fmt.Errorf("unsupported archive object mode")
		}
		data, readErr := a.Artifacts.ReadBounded(ctx, repo, sha, entry.Path, archiveBlobLimit)
		if readErr != nil {
			return readErr
		}
		total += int64(len(data))
		if total > archiveTotalLimit {
			return fmt.Errorf("archive content limit exceeded")
		}
		mode := os.FileMode(0644)
		if entry.Mode == "100755" {
			mode = 0755
		}
		link := entry.Mode == "120000"
		if link {
			if len(data) > 4096 || !safeArchiveLink(entry.Path, string(data)) {
				return fmt.Errorf("unsafe archive symlink")
			}
			mode = os.ModeSymlink | 0777
		}
		name := root + "/" + entry.Path
		if zw != nil {
			h := &zip.FileHeader{Name: name, Method: zip.Deflate}
			h.SetMode(mode)
			w, e := zw.CreateHeader(h)
			if e != nil {
				return e
			}
			if _, e = w.Write(data); e != nil {
				return e
			}
		} else {
			h := &tar.Header{Name: name, Mode: int64(mode.Perm()), Size: int64(len(data)), Typeflag: tar.TypeReg}
			if link {
				h.Typeflag = tar.TypeSymlink
				h.Linkname = string(data)
				h.Size = 0
			}
			if e := tw.WriteHeader(h); e != nil {
				return e
			}
			if !link {
				if _, e := tw.Write(data); e != nil {
					return e
				}
			}
		}
	}
	if zw != nil {
		return zw.Close()
	}
	if err = tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}
func (a *App) handleRepositoryArchive(w http.ResponseWriter, r *http.Request) {
	_, repo, ok := a.resolveCanonicalRepository(w, r)
	if !ok {
		return
	}
	requested := r.PathValue("archive")
	format := "zip"
	ref := ""
	switch {
	case strings.HasSuffix(requested, ".tar.gz"):
		format = "tar.gz"
		ref = strings.TrimSuffix(requested, ".tar.gz")
	case strings.HasSuffix(requested, ".zip"):
		ref = strings.TrimSuffix(requested, ".zip")
	}
	if ref == "" || len(ref) > 256 || strings.ContainsAny(ref, "\x00\r\n") {
		writeJSON(w, 400, map[string]any{"error": "archive_ref_format_required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	commits, err := a.Artifacts.LogContext(ctx, repo, ref)
	if err != nil {
		writeArtifactsError(w, err)
		return
	}
	if len(commits) != 1 || !workspaceSHA.MatchString(commits[0].Hash) {
		writeJSON(w, 404, map[string]any{"error": "archive_commit_not_found"})
		return
	}
	sha := commits[0].Hash
	root := r.PathValue("repo") + "-" + sha[:12]
	tmp, err := os.CreateTemp("", "switchyard-archive-*")
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "archive_staging_unavailable"})
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err = a.buildRepositoryArchive(ctx, repo, sha, root, format, tmp); err != nil {
		writeJSON(w, 422, map[string]any{"error": "archive_unavailable", "message": err.Error()})
		return
	}
	info, err := tmp.Stat()
	if err != nil {
		return
	}
	if _, err = tmp.Seek(0, 0); err != nil {
		return
	}
	mime := "application/zip"
	if format == "tar.gz" {
		mime = "application/gzip"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", root+"."+format))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("X-Switchyard-Commit", sha)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Vary", "Cookie, Authorization")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	io.Copy(w, tmp)
}
