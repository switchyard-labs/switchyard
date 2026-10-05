package app

import (
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"switchyard/internal/trestle"
)

const avatarMaxBytes = 2 << 20

func profileCollections() [][2]any {
	return [][2]any{
		{"repository_stars", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "repo_id", Type: "text"}, {Name: "username", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"user_follows", []trestle.CollectionField{{Name: "id", Type: "text", Unique: true}, {Name: "follower", Type: "text"}, {Name: "following", Type: "text"}, {Name: "created_at", Type: "text"}}},
		{"user_profiles", []trestle.CollectionField{
			{Name: "username", Type: "text", Unique: true}, {Name: "bio", Type: "text"}, {Name: "location", Type: "text"}, {Name: "website", Type: "text"}, {Name: "social", Type: "json"}, {Name: "pinned_repos", Type: "json"}, {Name: "avatar_file", Type: "text"}, {Name: "updated_at", Type: "text"},
		}},
		{"org_profiles", []trestle.CollectionField{
			{Name: "org_id", Type: "text", Unique: true}, {Name: "display_name", Type: "text"}, {Name: "description", Type: "text"}, {Name: "location", Type: "text"}, {Name: "website", Type: "text"}, {Name: "contact", Type: "text"}, {Name: "avatar_file", Type: "text"}, {Name: "visibility", Type: "text"}, {Name: "updated_at", Type: "text"},
		}},
	}
}

func (a *App) userRecord(username string) map[string]any {
	it, err := a.Trestle.ListRecords("users", `username = "`+username+`"`)
	if err != nil || len(it) == 0 {
		return nil
	}
	return it[0]
}
func (a *App) userProfile(username string) map[string]any {
	p := map[string]any{"username": username, "bio": "", "location": "", "website": "", "social": map[string]any{}, "avatar_url": "/api/avatars/user/" + username}
	if u := a.userRecord(username); u != nil {
		p["display_name"] = u["display_name"]
	}
	if it, err := a.Trestle.ListRecords("user_profiles", `username = "`+username+`"`); err == nil && len(it) > 0 {
		for k, v := range it[0] {
			p[k] = v
		}
	}
	return p
}

func (a *App) handleGetUserProfile(w http.ResponseWriter, r *http.Request) {
	u := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(u) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	writeJSON(w, 200, a.userProfile(u))
}
func (a *App) handleUpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		DisplayName string            `json:"display_name"`
		Bio         string            `json:"bio"`
		Location    string            `json:"location"`
		Website     string            `json:"website"`
		Social      map[string]string `json:"social"`
		PinnedRepos []string          `json:"pinned_repos"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if len(in.Bio) > 500 || len(in.Location) > 120 || len(in.Website) > 300 {
		writeJSON(w, 400, map[string]any{"error": "profile_field_too_long"})
		return
	}

	if !safeWebURL(strings.TrimSpace(in.Website)) {
		writeJSON(w, 400, map[string]any{"error": "website_url_invalid"})
		return
	}
	for _, link := range in.Social {
		if !safeWebURL(link) {
			writeJSON(w, 400, map[string]any{"error": "social_url_invalid"})
			return
		}
	}
	if strings.TrimSpace(in.DisplayName) != "" {
		rid, ver, _, err := a.Trestle.FindRecord("users", filterEq("username", u))
		if err != nil || rid == "" {
			writeJSON(w, 502, map[string]any{"error": "profile_user_lookup_failed"})
			return
		}
		if err := a.Trestle.PatchRecord("users", rid, ver, map[string]any{"display_name": strings.TrimSpace(in.DisplayName)}); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "display_name_save_failed"})
			return
		}
	}
	vals := map[string]any{"username": u, "bio": strings.TrimSpace(in.Bio), "location": strings.TrimSpace(in.Location), "website": strings.TrimSpace(in.Website), "social": in.Social, "pinned_repos": in.PinnedRepos, "updated_at": nowStr()}
	if err := a.saveProfileRecord("user_profiles", filterEq("username", u), vals, "profile-"+u); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "profile_save_failed"})
		return
	}

	writeJSON(w, 200, a.userProfile(u))
}

func avatarExt(header []byte) (string, string, bool) {
	ct := http.DetectContentType(header)
	switch ct {
	case "image/png":
		return ".png", ct, true
	case "image/jpeg":
		return ".jpg", ct, true
	case "image/webp":
		return ".webp", ct, true
	case "image/gif":
		return ".gif", ct, true
	}
	return "", ct, false
}
func saveAvatarFile(dataDir, kind, id string, file multipart.File) (string, error) {
	b, err := io.ReadAll(io.LimitReader(file, avatarMaxBytes+1))
	if err != nil {
		return "", err
	}
	if len(b) > avatarMaxBytes {
		return "", fmt.Errorf("avatar_too_large")
	}
	ext, _, ok := avatarExt(b)
	if !ok {
		return "", fmt.Errorf("avatar_type_invalid")
	}
	dir := filepath.Join(dataDir, "avatars", kind)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	name := id + "-" + randHex(16) + ext
	if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
		return "", err
	}
	return name, nil
}
func (a *App) handleUploadUserAvatar(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	if err := r.ParseMultipartForm(avatarMaxBytes); err != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_multipart"})
		return
	}
	f, _, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": "avatar_required"})
		return
	}
	defer f.Close()
	name, err := saveAvatarFile(a.DataDir, "user", u, f)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	vals := map[string]any{"username": u, "avatar_file": name, "updated_at": nowStr()}
	if err := a.saveProfileRecord("user_profiles", filterEq("username", u), vals, "profile-avatar-"+u); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "profile_save_failed"})
		return
	}

	writeJSON(w, 200, map[string]any{"avatar_url": "/api/avatars/user/" + u})
}
func (a *App) handleDeleteUserAvatar(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	id, ver, profile, err := a.Trestle.FindRecord("user_profiles", filterEq("username", u))
	if err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "avatar_metadata_unavailable"})
		return
	}
	if id != "" {
		if err := a.Trestle.PatchRecord("user_profiles", id, ver, map[string]any{"avatar_file": "", "updated_at": nowStr()}); err != nil {
			writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "avatar_delete_failed"})
			return
		}
		name := strOf(profile["avatar_file"])
		if name != "" && filepath.Base(name) == name {
			if err := os.Remove(filepath.Join(a.DataDir, "avatars", "user", name)); err != nil && !os.IsNotExist(err) {
				writeJSON(w, 200, map[string]any{"ok": true, "cleanup_pending": true})
				return
			}
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true})

}
func fallbackAvatar(label string) string {
	initial := "?"
	if s := strings.TrimSpace(label); s != "" {
		initial = strings.ToUpper(string([]rune(s)[0]))
	}
	return `<svg xmlns="http://www.w3.org/2000/svg" width="256" height="256" viewBox="0 0 256 256"><rect width="256" height="256" rx="10" fill="#202226"/><rect x="16" y="16" width="224" height="224" rx="10" fill="#292c31" stroke="#454a51" stroke-width="2"/><text x="128" y="153" text-anchor="middle" font-family="system-ui,sans-serif" font-size="88" font-weight="700" fill="#d9a45b">` + html.EscapeString(initial) + `</text></svg>`
}
func (a *App) handleAvatar(w http.ResponseWriter, r *http.Request) {
	kind, id := r.PathValue("kind"), r.PathValue("id")
	var name string
	label := id
	if kind == "user" {
		if p := a.userProfile(id); p != nil {
			name, _ = p["avatar_file"].(string)
		}
	} else if kind == "org" {
		if org := a.orgBySlugOrID(id); org != nil {
			label = strOr(org["name"])
			id = strOr(org["id"])
		}
		if it, e := a.Trestle.ListRecords("org_profiles", `org_id = "`+id+`"`); e == nil && len(it) > 0 {
			name, _ = it[0]["avatar_file"].(string)
		}
	}
	if name != "" {
		p := filepath.Join(a.DataDir, "avatars", kind, name)
		if b, e := os.ReadFile(p); e == nil {
			w.Header().Set("Content-Type", http.DetectContentType(b))
			w.Header().Set("Cache-Control", "private, max-age=300")
			_, _ = w.Write(b)
			return
		}
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = w.Write([]byte(fallbackAvatar(label)))
}

func (a *App) handleUserRepositories(w http.ResponseWriter, r *http.Request) {
	u := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(u) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	items, err := a.Trestle.ListRecords("repository_meta", `owner_type = "user"`)
	items = a.visibleRecords("repository_meta", items, a.currentUser(r))
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, it := range items {
		if it["owner_id"] == u || it["owner_slug"] == u {
			out = append(out, it)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}
func (a *App) handleUserActivity(w http.ResponseWriter, r *http.Request) {
	u := normalizeOwnerSlug(r.PathValue("username"))
	if a.userRecord(u) == nil {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	out := []map[string]any{}
	if work, e := a.Trestle.ListRecords("work", `owner = "`+u+`"`); e == nil {
		work = a.visibleRecords("work", work, a.currentUser(r))
		for _, x := range work {
			out = append(out, map[string]any{"type": "work", "title": x["title"], "at": x["updated_at"], "id": x["id"]})
		}
	}
	if refs, e := a.Trestle.ListRecords("ref_updates", ""); e == nil {
		refs = a.visibleRecords("ref_updates", refs, a.currentUser(r))
		for _, x := range refs {
			if attributedRef(strOr(x["provenance"]), u) {
				out = append(out, map[string]any{"type": "git", "repo": x["repo"], "branch": x["branch"], "at": x["occurred_at"], "sha": x["new_sha"]})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if strOf(out[i]["at"]) != strOf(out[j]["at"]) {
			return strOf(out[i]["at"]) > strOf(out[j]["at"])
		}
		return strOf(out[i]["id"])+strOf(out[i]["sha"]) > strOf(out[j]["id"])+strOf(out[j]["sha"])
	})
	limit, offset := activityPageBounds(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
	total := len(out)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	writeJSON(w, 200, map[string]any{"items": out[offset:end], "total": total, "offset": offset, "limit": limit, "has_more": end < total})
}

func (a *App) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if readJSON(r, &in) != nil || len(in.New) < 8 {
		writeJSON(w, 400, map[string]any{"error": "password_too_short"})
		return
	}
	rid, ver, vals, err := a.Trestle.FindRecord("users", `username = "`+u+`"`)
	if err != nil || rid == "" {
		writeJSON(w, 404, map[string]any{"error": "user_not_found"})
		return
	}
	hash, _ := vals["password_hash"].(string)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Current)) != nil {
		writeJSON(w, 403, map[string]any{"error": "current_password_invalid"})
		return
	}
	nh, err := bcrypt.GenerateFromPassword([]byte(in.New), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": "internal"})
		return
	}
	if err := a.Trestle.PatchRecord("users", rid, ver, map[string]any{"password_hash": string(nh)}); err != nil {
		writeJSON(w, 409, map[string]any{"error": "account_changed_retry"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func (a *App) handleListSessions(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	current := ""
	if c, e := r.Cookie(sessionCookieName(r)); e == nil {
		current = c.Value
	}
	it, err := a.Trestle.ListRecords("sessions", `username = "`+u+`"`)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, x := range it {
		tok, _ := x["token"].(string)
		out = append(out, map[string]any{"id": sha256Hex([]byte(tok))[:10], "current": tok == current, "expires_at": x["expires_at"]})
	}
	writeJSON(w, 200, map[string]any{"items": out})
}
func (a *App) handleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	u := a.currentUser(r)
	if u == "" {
		writeJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	current := ""
	if c, e := r.Cookie(sessionCookieName(r)); e == nil {
		current = c.Value
	}
	it, _ := a.Trestle.ListRecords("sessions", `username = "`+u+`"`)
	n := 0
	for _, x := range it {
		tok, _ := x["token"].(string)
		if tok == "" || tok == current {
			continue
		}
		if rid, ver, _, e := a.Trestle.FindRecord("sessions", `token = "`+tok+`"`); e == nil && rid != "" {
			if a.Trestle.DeleteRecord("sessions", rid, ver) == nil {
				n++
			}
		}
	}
	writeJSON(w, 200, map[string]any{"revoked": n})
}

func (a *App) orgBySlugOrID(key string) map[string]any {
	items, err := a.Trestle.ListRecords("orgs", "")
	if err != nil {
		return nil
	}
	key = normalizeOwnerSlug(key)
	for _, o := range items {
		if strOr(o["id"]) == key || normalizeOwnerSlug(strOr(o["name"])) == key {
			return o
		}
	}
	return nil
}
func (a *App) orgProfile(org map[string]any) map[string]any {
	id, name := strOr(org["id"]), strOr(org["name"])
	slug := normalizeOwnerSlug(name)
	p := map[string]any{"id": id, "slug": slug, "name": name, "display_name": name, "description": "", "location": "", "website": "", "contact": "", "visibility": "public", "avatar_url": "/api/avatars/org/" + id, "owner": org["owner"], "members": org["members"]}
	if it, e := a.Trestle.ListRecords("org_profiles", `org_id = "`+id+`"`); e == nil && len(it) > 0 {
		for k, v := range it[0] {
			p[k] = v
		}
	}
	return p
}
func (a *App) handleGetOwnerProfile(w http.ResponseWriter, r *http.Request) {
	slug := normalizeOwnerSlug(r.PathValue("slug"))
	ns, e := a.Trestle.ListRecords("owner_namespaces", `slug = "`+slug+`"`)
	// Accounts created before owner namespaces/profile pages existed should still
	// have a valid profile. Repair that durable namespace lazily instead of
	// rendering "Profile unavailable" for an otherwise valid user.
	if (e != nil || len(ns) == 0) && a.userRecord(slug) != nil {
		if err := a.ensureOwnerNamespace(slug, "user", slug); err == nil {
			ns, e = a.Trestle.ListRecords("owner_namespaces", `slug = "`+slug+`"`)
		}
	}
	if e != nil || len(ns) == 0 {
		writeJSON(w, 404, map[string]any{"error": "owner_not_found"})
		return
	}
	typ, id := strOr(ns[0]["owner_type"]), strOr(ns[0]["owner_id"])
	if typ == "user" {
		p := a.userProfile(id)
		p["owner_type"] = "user"
		writeJSON(w, 200, p)
		return
	}
	if typ == "org" {
		if o := a.orgBySlugOrID(id); o != nil {
			p := a.orgProfile(o)
			if strOr(p["visibility"]) != "public" && a.orgRole(o, a.currentUser(r)) == "" {
				writeJSON(w, 404, map[string]any{"error": "owner_not_found"})
				return
			}
			p["owner_type"] = "org"
			writeJSON(w, 200, p)
			return
		}
	}
	writeJSON(w, 404, map[string]any{"error": "owner_not_found"})
}
func (a *App) handleGetOrgProfile(w http.ResponseWriter, r *http.Request) {
	o := a.orgBySlugOrID(r.PathValue("id"))
	if o == nil {
		writeJSON(w, 404, map[string]any{"error": "org_not_found"})
		return
	}
	if strOr(a.orgProfile(o)["visibility"]) != "public" && a.orgRole(o, a.currentUser(r)) == "" {
		writeJSON(w, 404, map[string]any{"error": "org_not_found"})
		return
	}
	writeJSON(w, 200, a.orgProfile(o))
}
func (a *App) requireOrgOwner(r *http.Request, key string) (map[string]any, bool) {
	u := a.currentUser(r)
	o := a.orgBySlugOrID(key)
	return o, u != "" && o != nil && strOr(o["owner"]) == u
}
func (a *App) handleUpdateOrgProfile(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgOwner(r, r.PathValue("id"))
	if !ok {
		writeJSON(w, 403, map[string]any{"error": "org_owner_required"})
		return
	}
	var in struct {
		DisplayName string `json:"display_name"`
		Description string `json:"description"`
		Location    string `json:"location"`
		Website     string `json:"website"`
		Contact     string `json:"contact"`
		Visibility  string `json:"visibility"`
	}
	if readJSON(r, &in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_request"})
		return
	}
	if len(in.Description) > 1000 {
		writeJSON(w, 400, map[string]any{"error": "description_too_long"})
		return
	}
	id := strOr(o["id"])
	vis := in.Visibility
	if vis == "" {
		vis = "public"
	}
	if !safeWebURL(strings.TrimSpace(in.Website)) {
		writeJSON(w, 400, map[string]any{"error": "website_url_invalid"})
		return
	}
	vals := map[string]any{"org_id": id, "display_name": strings.TrimSpace(in.DisplayName), "description": strings.TrimSpace(in.Description), "location": strings.TrimSpace(in.Location), "website": strings.TrimSpace(in.Website), "contact": strings.TrimSpace(in.Contact), "visibility": vis, "updated_at": nowStr()}
	if vals["display_name"] == "" {
		vals["display_name"] = o["name"]
	}
	if err := a.saveProfileRecord("org_profiles", filterEq("org_id", id), vals, "org-profile-"+id); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "profile_save_failed"})
		return
	}

	writeJSON(w, 200, a.orgProfile(o))
}
func (a *App) handleUploadOrgAvatar(w http.ResponseWriter, r *http.Request) {
	o, ok := a.requireOrgOwner(r, r.PathValue("id"))
	if !ok {
		writeJSON(w, 403, map[string]any{"error": "org_owner_required"})
		return
	}
	if r.ParseMultipartForm(avatarMaxBytes) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad_multipart"})
		return
	}
	f, _, e := r.FormFile("avatar")
	if e != nil {
		writeJSON(w, 400, map[string]any{"error": "avatar_required"})
		return
	}
	defer f.Close()
	id := strOr(o["id"])
	name, e := saveAvatarFile(a.DataDir, "org", id, f)
	if e != nil {
		writeJSON(w, 400, map[string]any{"error": e.Error()})
		return
	}
	vals := map[string]any{"org_id": id, "avatar_file": name, "updated_at": nowStr()}
	if err := a.saveProfileRecord("org_profiles", filterEq("org_id", id), vals, "org-avatar-"+id); err != nil {
		writeJSON(w, draftPersistenceStatus(err), map[string]any{"error": "profile_save_failed"})
		return
	}

	writeJSON(w, 200, map[string]any{"avatar_url": "/api/avatars/org/" + id})
}
func (a *App) handleOrgRepositories(w http.ResponseWriter, r *http.Request) {
	o := a.orgBySlugOrID(r.PathValue("id"))
	if o == nil {
		writeJSON(w, 404, map[string]any{"error": "org_not_found"})
		return
	}
	id := strOr(o["id"])
	items, e := a.Trestle.ListRecords("repository_meta", `owner_type = "org"`)
	items = a.visibleRecords("repository_meta", items, a.currentUser(r))
	if e != nil {
		writeJSON(w, 502, map[string]any{"error": e.Error()})
		return
	}
	out := []map[string]any{}
	for _, x := range items {
		if strOr(x["owner_id"]) == id {
			out = append(out, x)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) handleListOrganizations(w http.ResponseWriter, r *http.Request) {
	items, err := a.Trestle.ListRecords("orgs", "")
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, o := range items {
		p := a.orgProfile(o)
		if strOr(p["visibility"]) == "public" || a.orgRole(o, a.currentUser(r)) != "" {
			out = append(out, p)
		}
	}
	writeJSON(w, 200, map[string]any{"items": out})
}

func (a *App) saveProfileRecord(collection, filter string, values map[string]any, key string) error {
	id, version, old, err := a.Trestle.FindRecord(collection, filter)
	if err != nil {
		return err
	}
	if id != "" {
		if _, explicit := values["avatar_file"]; !explicit {
			if avatar := strOf(old["avatar_file"]); avatar != "" {
				values["avatar_file"] = avatar
			}
		}
		return a.Trestle.PatchRecord(collection, id, version, values)
	}
	_, _, err = a.Trestle.CreateRecord(collection, values, key+"-"+randHex(10))
	return err
}

func activityPageBounds(rawLimit, rawOffset string) (int, int) {
	limit, _ := strconv.Atoi(rawLimit)
	offset, _ := strconv.Atoi(rawOffset)
	if limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
