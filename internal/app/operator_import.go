package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// ImportUserRepository is an explicit host-operator action, never a user API.
// It binds one existing physical repository to an existing user. It does not
// enumerate the Cloudflare account, transfer ownership or overwrite metadata.
func (a *App) ImportUserRepository(artifact, owner, slug, visibility string) (map[string]any, error) {
	if !validOwnerSlug(owner) || !validRepoSlug(slug) || !validRepoSlug(artifact) || (visibility != "public" && visibility != "private") {
		return nil, fmt.Errorf("explicit valid artifact, user, slug and public/private visibility are required")
	}
	_, _, user, err := a.Trestle.FindRecord("users", filterEq("username", owner))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("import owner must be an existing user")
	}
	_, _, existing, err := a.Trestle.FindRecord("repository_meta", filterEq("artifact_name", artifact))
	if err != nil {
		return nil, err
	}
	full := owner + "/" + slug
	if existing != nil {
		if existing["full_name"] != full || existing["owner_type"] != "user" || existing["owner_id"] != owner || existing["visibility"] != visibility {
			return nil, fmt.Errorf("existing repository identity differs; import will not overwrite it")
		}
		return existing, nil
	}
	_, _, collision, err := a.Trestle.FindRecord("repository_meta", filterEq("full_name", full))
	if err != nil || collision != nil {
		return nil, fmt.Errorf("repository name unavailable")
	}
	backend, err := a.Artifacts.GetRepo(artifact)
	if err != nil {
		return nil, fmt.Errorf("cannot verify the selected Artifacts repository")
	}
	if backend.Name != artifact {
		return nil, fmt.Errorf("Artifacts identity mismatch")
	}
	if err = a.ensureOwnerNamespace(owner, "user", owner); err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(a.Artifacts.AccountID + "/" + a.Artifacts.Namespace + "/" + artifact))
	id := "repo_" + hex.EncodeToString(hash[:12])
	values := map[string]any{"id": id, "full_name": full, "owner_type": "user", "owner_id": owner, "owner_slug": owner, "slug": slug, "display_name": slug, "description": backend.Description, "visibility": visibility, "default_branch": backend.DefaultBranch, "artifact_name": artifact, "created_at": nowStr(), "updated_at": nowStr()}
	_, _, err = a.Trestle.CreateRecord("repository_meta", values, "operator-import-"+id)
	if err != nil {
		return nil, err
	}
	return values, nil
}
