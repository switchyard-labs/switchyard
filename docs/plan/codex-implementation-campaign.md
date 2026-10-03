# Codex implementation campaign

Baseline: `ff22794`, main clean and equal to refreshed origin/main, 2026-10-03.
Only Switchyard is writable; all reference projects remain untouched. Local checkpoint commits only; no pushes.

The adopted outline requires C0–C4, then a handover before C5. Warden/Gantry Core editor adaptation without an interactive terminal is recorded for C20; collapsible directories and drag/drop moves are acceptance requirements.

## Reconciliation

| Finding | Baseline classification | Evidence / checkpoint |
| --- | --- | --- |
| Product route collision | FIXED BY DEEPSEEK | Explicit /repositories route; existing route tests |
| Strut runtime dependency | FIXED BY DEEPSEEK | Go deterministic adapter, f21261d |
| Menu geometry and design debt | PARTIALLY FIXED | inset changed; product pass still required |
| Filesystem traversal, Git metadata and symlink writes | STILL PRESENT | refs.buildCommit joins unchecked paths; C1 |
| Active same-origin repository HTML/SVG | STILL PRESENT | content handler selects active MIME; C1 |
| Repository authorization and global lists | STILL PRESENT | legacy APIs, grants, profiles, Work/PR; C2 |
| Credential ownership/rotation/session invalidation | STILL PRESENT | global credentials and cookie-only logout; C3 |
| Draft CAS and commit cleanup | STILL PRESENT | two reads, unchecked PATCH, no commit owner guard; C4 |
| Expected SHA and mutable validation | STILL PRESENT | independent clone after head check; C5 |
| Queue leases/recovery | STILL PRESENT | local busy flag and stranded running; C6 |
| Workflow effects/approval/budget | STILL PRESENT | replay completion persistence gaps; C7 |
| Real runner isolation | STILL PRESENT | no context/limits; C8 |
| Schema migrations | STILL PRESENT | existence-only provisioning; C9 |
| Cloudflare Actions | STILL PRESENT | separate CI domain absent; C10–16 |
| Repository/profile/editor/product quality | STILL PRESENT | supplied screenshots and current source; C17–21 |
| Certification gate false positive | STILL PRESENT → addressed in C0 | pipefail/gofmt grep pipeline replaced by captured output |

## C0 — baseline

Go test ./..., go vet ./..., go build ./..., git diff --check and nift build --all passed. Existing coverage is sparse; these are baseline checks, not release certification. Refreshed remote without altering history. Linode switchyard/caddy active, localhost Trestle responded 302. Live browser dashboard still contains the prototype card structure and demo links; product parity remains unfulfilled. Cloudflare-specific runtime certification remains pending; credentials were not printed. Formatting gate now fails reliably on existing drift instead of masking a SIGPIPE failure. No production deployment in this checkpoint.

## Checkpoint ledger

C0–C4 locally implemented and verified. C5–C25 pending. Checkpoint identities are recorded in the batch handover ledger below.

## C1 — contained repository filesystem

Repository paths now reject traversal, absolute/noncanonical paths, encoded aliases, Git metadata, Windows separators/drive syntax and symlink components. Applied at the ref mutation substrate, draft creation, conflict-resolution writes and semantic contract reads. Mutation scratch clones use unique temporary directories; failed commit builds clean up their clones. Raw repository responses are inert text with nosniff and sandbox CSP, including HTML/SVG. Adversarial traversal and symlink tests pass with the full Go suite. No live writes or deployment. Existing generated trailing whitespace from the C0 Nift build was normalized. C5 still owns Git expected-SHA publication semantics.

## C2 — repository authorization

Central capability/grant interpreter covers read, write, admin, Agent execution and integration. Private organization repos no longer grant all members read; internal repos do. Collaborator/user/team grants are interpreted with organization boundaries. All API handlers pass through authorization after route resolution; indirect Work/Attempt/PR/queue/draft/run associations and request-supplied repos are checked before effects. Public canonical reads are permitted anonymously; operational guest routes are denied. Global repository/Work/PR/event/run/finding lists and SSE are scoped. Workflows have a separate owner collection, server-assigned actor and per-host-operation permission checks; existing unowned definitions/runs fail closed pending explicit operator migration. User-driven import of account-wide Artifacts repositories is disabled because account credentials cannot prove a caller owns an unregistered repository. An operator import flow remains a product limitation. Owner/visibility/capability and anonymous operational API regression tests pass. Live deployment/migration is deferred, preserving existing data.

C0 commit: 94717ff. C1 commit: a440691.

## C3 — credentials and sessions

Credentials use a separate owner registry (new collection); unowned legacy credentials are inaccessible until operator adoption. Organization delegation is rejected until a scoped trust model exists. Rotation performs a conditional ciphertext PATCH without delete/create; failure retains the original secret. Malformed ciphertext returns errors. Key loading fails closed on invalid/insecure/unreadable keys and checks all creation errors; local encryption does not protect against host compromise. CLI credential files are unique/private and removed, environment inheritance is replaced by an allowlist, output is bounded and the scoped credential is redacted. Full process cancellation remains C8.

Sessions now bind to the current password hash, so password changes invalidate all prior tokens even if record cleanup fails. Legacy session tokens expire at upgrade and users sign in again. Logout deletes the durable session and reports persistence failures. Session creation reports errors; usernames are normalized; login is throttled by client IP. Secure cookies use actual TLS or explicitly configured `SWITCHYARD_PUBLIC_URL=https://…` (forwarded headers are not blindly trusted). Mutation Origin/Sec-Fetch-Site checks reject cross-site requests. Profile website/social URLs allow only HTTP(S), fallback avatar text is escaped, and public user activity is scoped. Tests cover malformed ciphertext, failed rotation, key retention/privacy, output bounds, URL schemes, Origin and throttling. Full Go suite passes. New owner collections must be provisioned before deployment; no live schema/data changes made.

C2 commit: e9faf30.

## C4 — drafts and editor state

Save reads one durable record/version snapshot, requires an exact logical revision after creation, and conditionally writes that same version. Initial drafts require a base SHA and use a stable creation idempotency key. Read/PATCH/create failures cannot report a successful save. Commit verifies the owner and expected revision, rejects a moved/missing base, checks the cloned HEAD before applying edits, and rejects stale Git results. Commit metadata is patched against the exact snapshot used for Git, retains content for recovery and advances revision/base only on successful cleanup. When Git succeeds but cleanup loses a race or fails, the API explicitly reports `draft_cleanup=pending`; the newer draft remains intact. Agent proposals require expected revision.

The editor stores CodeMirror state, selection/undo history, scroll, content baseline, base SHA, revision and dirty status independently per file. Saves/commits capture their target instead of applying asynchronous results to a newly selected tab. Edits typed during a save remain dirty. Navigation guards warn about unsaved buffers. Browser fixture verified two independent unsaved files survive switching; console warning/error log was empty. This is a correctness change, not the planned Warden/Gantry redesign.

Final audit also closed canonical raw-content serving (shared inert response helper), conflict merge temporary-file symlink aliases, legacy organization-membership fallback, workflow actor mutation, unscoped audit/activity records and filter URL escaping. Raster repository images retain safe MIME types. Existing Artifacts formatting drift was normalized after the repaired gate correctly failed on it.

Validation: `go test -race ./...`, `go vet ./...`, `go build ./...`, all JavaScript syntax checks, static accessibility checks, product certification script and `git diff --check` pass. Tests cover organization owners/admins/members, user/team/collaborator grants, cross-organization denial, private indirect/legacy routes, session logout/password invalidation, two competing draft saves, missing/stale revisions, unauthorized commit, failed PATCH and stale/failed commit cleanup. Real external-push phase races and full Git crash-recovery certification remain C5–C7; helper/HTTP fixture tests are not claims of live crash certification.

## Batch one handover

C0–C4 local implementation complete; stop before C5 as instructed. No pushes or application deployments. Live switchyard/caddy active, Trestle reachable locally, Artifacts HTTP 200 / success / 50 repositories. Reference Warden and Gantry Core checkouts remain clean and untouched. Existing live app still runs ff22794, so these local security improvements are not yet production protection.

Deployment prerequisites: provision new `workflow_owners` and `credential_owners` collections; explicitly adopt legacy records after verifying ownership; re-login after session format upgrade; migrate any legacy array-only org memberships to normalized records. Unowned entities and unregistered repositories fail closed. User-driven account-wide repository import is disabled until ownership can be established. Set `SWITCHYARD_PUBLIC_URL` to the actual HTTPS origin behind a trusted proxy. DNS/TLS is still absent on the provided live IP.

Next: C5–C9 Git/durability/runner/migration batch. GitHub-quality repository/dashboard/profile/layout work remains C17–C21. Warden uses `github.com/gantry-tools/gantry-core/editor` for matching, replacement, revision and atomic-write text operations; its separate browser workbench is in `public/assets/js/script.js` and `public/assets/css/style.css`. Adapt those interactions to Artifacts/draft boundaries without adding a terminal or local-host filesystem access. Collapsible directory explorer, drag/drop moves, proper editor integration, content search and language extensions remain explicit acceptance requirements.

## Checkpoint commit ledger

| Checkpoint | Commit | Status |
| --- | --- | --- |
| C0 | `94717ffd01ca82bb567c434dff40a47bc84857f8` | locally implemented / checks passed |
| C1 | `a440691ebe68012c4191321cf7f6765427ac30bf` | locally implemented / checks passed |
| C2 | `e9faf30b42f10f53e9e865ec02d9acb48495ad26` | locally implemented / checks passed |
| C3 | `2768fa98570a286b75af9055a4072adf9d901c8f` | locally implemented / checks passed |
| C4 | `0c3b1a9e5a76f205538b5657ab23d40fbf9e753e` | locally implemented / checks passed |

These commits are local only. This documentation commit records their immutable identities without amending any checkpoint.
