package app

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// PagesConfig names an explicit source; neither site kind requires a magic repo.
// Public output from private source requires a separate recorded acknowledgement.
type PagesConfig struct {
	RepositoryID       string `json:"repository_id"`
	Owner              string `json:"owner"`
	Project            string `json:"project,omitempty"`
	Ref                string `json:"ref"`
	WorkingDirectory   string `json:"working_directory"`
	BuildCommand       string `json:"build_command"`
	OutputDirectory    string `json:"output_directory"`
	SPAFallback        string `json:"spa_fallback,omitempty"`
	PublicAcknowledged bool   `json:"public_acknowledged"`
}

var pagesOwner = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)
var pagesProject = regexp.MustCompile(`^[a-z0-9_][a-z0-9._-]{0,99}$`)

func pagesRelativePath(value string, allowRoot bool) bool {
	if value == "." {
		return allowRoot
	}
	if value == "" || strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\\x00\r\n") || path.Clean(value) != value {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." || part == "." || part == "" || part == ".git" {
			return false
		}
	}
	return true
}
func (p PagesConfig) validate(privateSource bool) error {
	if !pagesOwner.MatchString(p.Owner) || strings.HasPrefix(p.Owner, "dpl-") {
		return fmt.Errorf("pages_owner_invalid")
	}
	switch p.Owner {
	case "www", "docs", "app", "api", "admin", "assets", "static", "pages", "login", "signup", "sy":
		return fmt.Errorf("pages_owner_reserved")
	}
	if p.Project != "" && (!pagesProject.MatchString(p.Project) || p.Project == ".git") {
		return fmt.Errorf("pages_project_invalid")
	}
	if p.SPAFallback != "" && !pagesRelativePath(p.SPAFallback, false) {
		return fmt.Errorf("pages_spa_fallback_invalid")
	}
	if p.RepositoryID == "" || !pagesSourceRefValid(p.Ref) {
		return fmt.Errorf("pages_source_invalid")
	}
	if !pagesRelativePath(p.WorkingDirectory, true) || !pagesRelativePath(p.OutputDirectory, false) {
		return fmt.Errorf("pages_directory_invalid")
	}
	if strings.TrimSpace(p.BuildCommand) == "" || len(p.BuildCommand) > 4096 || strings.ContainsRune(p.BuildCommand, 0) {
		return fmt.Errorf("pages_build_invalid")
	}
	if privateSource && !p.PublicAcknowledged {
		return fmt.Errorf("pages_public_acknowledgement_required")
	}
	return nil
}
func (p PagesConfig) basePath() string {
	if p.Project == "" {
		return "/"
	}
	return "/" + p.Project + "/"
}

// Configured sources are branches or qualified branch/tag refs, never a mutable
// interpretation of a raw commit ID. Each build resolves this ref once.
func pagesSourceRefValid(value string) bool {
	ref := pagesRef(value)
	if len(ref) > 255 || (!strings.HasPrefix(ref, "refs/heads/") && !strings.HasPrefix(ref, "refs/tags/")) || strings.ContainsAny(ref, " ~^:?*[\\\x00\r\n\t") || strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for _, part := range strings.Split(ref, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasPrefix(part, "-") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return !workspaceSHA.MatchString(value)
}
func pagesResolvedSource(refs map[string]string, ref string) (string, error) {
	if !pagesSourceRefValid(ref) {
		return "", fmt.Errorf("pages_source_invalid")
	}
	key := pagesRef(ref)
	sha := refs[key]
	// Annotated tags resolve to their peeled commit, not the tag object.
	if peeled := refs[key+"^{}"]; peeled != "" {
		sha = peeled
	}
	if !workspaceSHA.MatchString(sha) {
		return "", fmt.Errorf("pages_source_unavailable")
	}
	return sha, nil
}
