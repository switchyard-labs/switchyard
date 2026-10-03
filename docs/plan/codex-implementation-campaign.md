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

C0 complete: this checkpoint commit records baseline and repairs formatting detection. C1–C4 in progress. C5–C25 pending.

## C1 — contained repository filesystem

Repository paths now reject traversal, absolute/noncanonical paths, encoded aliases, Git metadata, Windows separators/drive syntax and symlink components. Applied at the ref mutation substrate, draft creation, conflict-resolution writes and semantic contract reads. Mutation scratch clones use unique temporary directories; failed commit builds clean up their clones. Raw repository responses are inert text with nosniff and sandbox CSP, including HTML/SVG. Adversarial traversal and symlink tests pass with the full Go suite. No live writes or deployment. Existing generated trailing whitespace from the C0 Nift build was normalized. C5 still owns Git expected-SHA publication semantics.

## C2 — repository authorization

Central capability/grant interpreter covers read, write, admin, Agent execution and integration. Private organization repos no longer grant all members read; internal repos do. Collaborator/user/team grants are interpreted with organization boundaries. All API handlers pass through authorization after route resolution; indirect Work/Attempt/PR/queue/draft/run associations and request-supplied repos are checked before effects. Public canonical reads are permitted anonymously; operational guest routes are denied. Global repository/Work/PR/event/run/finding lists and SSE are scoped. Workflows have a separate owner collection, server-assigned actor and per-host-operation permission checks; existing unowned definitions/runs fail closed pending explicit operator migration. User-driven import of account-wide Artifacts repositories is disabled because account credentials cannot prove a caller owns an unregistered repository. An operator import flow remains a product limitation. Owner/visibility/capability and anonymous operational API regression tests pass. Live deployment/migration is deferred, preserving existing data.

C0 commit: 94717ff. C1 commit: a440691.

## C3 — credentials and sessions

Credentials use a separate owner registry (new collection); unowned legacy credentials are inaccessible until operator adoption. Organization delegation is rejected until a scoped trust model exists. Rotation performs a conditional ciphertext PATCH without delete/create; failure retains the original secret. Malformed ciphertext returns errors. Key loading fails closed on invalid/insecure/unreadable keys and checks all creation errors; local encryption does not protect against host compromise. CLI credential files are unique/private and removed, environment inheritance is replaced by an allowlist, output is bounded and the scoped credential is redacted. Full process cancellation remains C8.

Sessions now bind to the current password hash, so password changes invalidate all prior tokens even if record cleanup fails. Legacy session tokens expire at upgrade and users sign in again. Logout deletes the durable session and reports persistence failures. Session creation reports errors; usernames are normalized; login is throttled by client IP. Secure cookies use actual TLS or explicitly configured `SWITCHYARD_PUBLIC_URL=https://…` (forwarded headers are not blindly trusted). Mutation Origin/Sec-Fetch-Site checks reject cross-site requests. Profile website/social URLs allow only HTTP(S), fallback avatar text is escaped, and public user activity is scoped. Tests cover malformed ciphertext, failed rotation, key retention/privacy, output bounds, URL schemes, Origin and throttling. Full Go suite passes. New owner collections must be provisioned before deployment; no live schema/data changes made.

C2 commit: e9faf30.
