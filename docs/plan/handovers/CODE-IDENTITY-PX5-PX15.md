# Switchyard Code Surfaces + Identity batch — PX5 through PX15

## Batch boundary

This handover closes the second and third product-quality phases from
`PRODUCT-UX-CAMPAIGN.md`:

- **Code Surfaces:** PX5–PX9
- **Identity:** PX10–PX15

The next planned checkpoint is **PX16 — Repository overview + metadata + clone UX**.
No PX16+ implementation was started in this batch.

## Status summary

| Checkpoint | Status in this workspace | Commit |
|---|---|---|
| PX5 shared language/highlighting | PASS | `7df5ee4` |
| PX6 repository source viewer 2.0 | CONDITIONAL PASS | `c794280` |
| PX7 Markdown/README/image renderer | PASS | `1718e90` |
| PX8 editor/workbench quality | CONDITIONAL PASS | `ee93e6f` |
| PX9 interactive Agent-to-draft | CONDITIONAL PASS | `4c0dc1b` |
| PX10 user profile/avatar model | CONDITIONAL PASS | `c0a8443` |
| PX11 user profile/README/activity | CONDITIONAL PASS | `a532ac0` |
| PX12 user settings | CONDITIONAL PASS | `3c7a24c` |
| PX13 organization profile/avatar | CONDITIONAL PASS | `3be853b` |
| PX14 members/teams/invitations/access | CONDITIONAL PASS | `8325ba1` |
| PX15 organization settings | CONDITIONAL PASS | `8a1d8b5` |

Follow-up commits preserve checkpoint history rather than amending it:

- `8743550` — PX15 invitation surface fix.
- `8f409ff` — identity integration/schema/navigation corrections + Organizations index.
- `ece9ebe` — PX14 Trestle-filter compatibility correction.

`CONDITIONAL PASS` means the implementation and offline/static gates are complete,
but the supplied runner cannot execute the normal live Trestle + Cloudflare
Artifacts + authenticated Chromium gates. It does **not** mean the feature was
silently accepted without a gate requirement.

---

# Code Surfaces

## PX5 — shared code presentation

One language registry now drives file identity and read-only tokens for:

- repository source;
- Markdown fences;
- future diff/review snippets;
- editor language/status identity.

Initial coverage includes Go, C/C++, Rust, JS/TS/JSX/TSX, HTML, CSS, JSON,
YAML, TOML, Markdown, shell, Python, SQL, Nift and Strut, with safe plaintext
fallback.

No runtime CDN dependency was introduced.

## PX6 — repository source viewer

The old plain `<pre>` path has been replaced with a Git-host source surface:

- language highlighting;
- line numbers + anchors;
- branch selector;
- breadcrumbs;
- file size/language metadata;
- Copy path / Copy file / Raw / Edit actions;
- MIME-aware image preview;
- responsive horizontal source scrolling.

Canonical owner/repo APIs now expose refs. Legacy flat repo routes remain for
compatibility.

## PX7 — sanitized README/Markdown

Added a reusable Markdown renderer for repository/profile content:

- raw HTML is escaped, not passed through;
- `javascript:`/`data:` destinations are blocked;
- headings/anchors, lists, tables, blockquotes, fenced code, links and images;
- fenced code uses PX5 tokens;
- relative repository links/images resolve against repo/ref/path.

Repository root README precedence currently is:

1. `README.md`
2. `README.markdown`
3. `README`
4. `docs/README.md`

This renderer is reused for user/org profile READMEs.

## PX8 — editor/workbench

The editor is now materially closer to the Warden quality bar while preserving
Switchyard semantics:

- Files / Search / Source modes;
- integrated repo file tree;
- multi-file tabs;
- editor status bar (branch/language/Ln+Col);
- filename filtering/search;
- new-file drafting;
- CAS-backed rename/delete;
- Ctrl/Cmd-S;
- Diff / Findings / Agent rail;
- responsive `Files | Editor | Agent` mobile modes;
- commit modal + inline status instead of relying on `prompt()`/`alert()` as the
  primary UX.

No terminal/server-management surface was imported from Warden.

## PX9 — interactive Agent assistance

Interactive assistance is now distinct from formal autonomous Attempts:

```text
Draft revision R
→ Agent proposal against R
→ re-read current draft
→ CAS apply only if still R
→ show diff
→ human edits/accepts/rejects
→ explicit human Commit
```

A newer human save wins over a stale Agent result. Interactive assistance never
commits Git implicitly. Formal Attempt Agent execution remains available as its
own mode.

Mixed provenance keeps the Agent execution attached to the later human commit
without falsely attributing the final commit solely to the Agent.

---

# Identity

## PX10 — users + avatars

Authentication records remain separate from profile records.

User profile data now supports:

- display name;
- bio;
- location;
- website/social metadata;
- pinned repositories;
- avatar metadata.

Avatar pipeline:

- PNG/JPEG/WebP/GIF;
- server-side MIME sniff;
- 2 MiB cap;
- no remote-avatar SSRF path;
- file storage under Switchyard data dir, 0600;
- generated graphite/amber fallback SVG;
- stable `/api/avatars/user/{username}` URL.

## PX11 — user profile product

`/{username}` renders a Git-host-style profile:

- large avatar;
- display name + handle;
- bio/location/site;
- repositories;
- pinned repositories;
- honest Work/Git activity;
- profile README.

Profile README convention is intentionally Git-native:

```text
username/username/README.md
```

Relative images and links use the same PX7 renderer.

No fake contribution heatmap was created.

## PX12 — user settings

Added account settings for:

- Profile;
- Appearance/density;
- Password;
- Sessions / revoke others;
- Agent provider credentials;
- Organization invitations;
- explicit notification-status/deferred surface;
- explicit danger-zone/deferred account deletion.

Provider credentials remain distinct from account authentication, Git
credentials and Switchyard repository capabilities.

## PX13 — organization profile product

Organization policy/fleet records now have a real profile layer:

- display name;
- description;
- location;
- website/contact;
- visibility;
- large avatar;
- organization repositories;
- member presentation.

Organization README convention follows familiar GitHub semantics:

```text
org/.github/profile/README.md
```

Again, PX7 handles Markdown, code and relative images.

## PX14 — collaboration semantics

Added normalized records for:

- memberships;
- invitations;
- teams;
- team members;
- repository access assignments.

Server-side role enforcement exists for owner/member/admin. The primary owner
cannot be removed. Invitations must be accepted/declined by the invited user.
Canonical owner/repo read access now observes visibility/membership rather than
relying on CSS hiding.

Legacy CP12 `members` arrays remain a compatibility fallback while the normalized
model becomes authoritative.

## PX15 — organization settings

Added `/organizations/{org}/settings` with:

- General/profile + avatar;
- Members/invitations;
- Teams;
- Repositories;
- Policies;
- Audit;
- Security guidance;
- Danger zone.

Existing Switchyard policy/risk/audit capabilities are now presented as an
organization administration product rather than disconnected APIs.

Policy creation requires org ownership; fleet requires membership; org audit
requires admin/owner.

Rename/transfer/delete remain deliberately disabled until namespace and
repository ownership-transfer semantics are independently certified.

A global Organizations index/create surface is also present at `/organizations`.

---

# Shared identity/navigation integration

The normal application account chip now uses the user avatar and links directly
to the user profile. The full-screen Account navigation links to Organizations
and Settings rather than labelling identity as a future phase.

The underlying public identity remains `owner/repo`; physical Cloudflare
Artifacts repository names remain an implementation detail.

---

# Validation performed in this runner

Passed:

- PX5 language/token fixture: 18/18;
- PX6 repository-view static gate;
- PX7 Markdown sanitization/render fixture;
- PX8 workbench static gate;
- PX9 Agent/draft CAS contract static gate;
- PX11 profile static gate;
- PX12 settings static gate;
- PX13 org profile static gate;
- PX14 collaboration/permission static gate;
- PX15 org settings static gate;
- `node --check` on all changed application JS;
- `gofmt` on changed Go files;
- `git diff --check`.

## Runner limitation

The archive declares **Go 1.26**, while this isolated runner does not have the
Go 1.26 toolchain or network/module cache. `go test ./...` therefore cannot be
executed here without modifying the project toolchain or fetching dependencies.
That limitation is preserved honestly rather than changing `go.mod`.

The normal development environment must run before PX16:

```bash
go test ./...
go vet ./...
```

and then exercise the live gates below.

---

# Required normal-environment live gates before PX16

1. **PX6:** browse `demo-basic` and Switchyard-self via canonical owner/repo URLs;
   exercise branch selector/source/image/raw/copy/edit.
2. **PX8:** fresh 1440x900, 1280x800 and 390x844 Chromium screenshots; compare
   Files/Search/Source, tabs, status and Agent rail against Warden reference.
3. **PX9:** deterministic/real runner race where human saves R+1 while Agent
   proposal started at R; newer draft must survive.
4. **PX10/11:** two real users with different avatar formats + `user/user`
   profile README repos containing relative images.
5. **PX12:** password/session/provider flows + mobile settings permission/error
   states.
6. **PX13:** curated `switchyard-labs` org, large avatar, multiple owned repos and
   `.github/profile/README.md` with relative image.
7. **PX14:** owner/member/outsider permission matrix; invitation acceptance;
   member removal; team membership; private repository access denial.
8. **PX15:** owner can mutate organization settings; ordinary member gets 403 on
   owner-only actions; audit/policy surfaces display real records.

Do not call the Identity phase release-certified until these live gates run.

---

# Resume point

Proceed next with **PX16 — Repository overview + metadata + clone UX** after the
normal-environment batch gate above is green or bounded with explicit evidence.
