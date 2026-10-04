# Codex implementation campaign

Baseline: `ff22794`, main clean and equal to refreshed origin/main, 2026-10-03.
Switchyard and its brochure are authorized for campaign changes; `sy` and all reference projects remain untouched. Local checkpoint commits only; no pushes.

The latest adopted outline requires uninterrupted sequential C5–C25 work, with a local commit at each checkpoint and an early stop only for a genuine blocker. Warden/Gantry Core editor adaptation without an interactive terminal is recorded for C20; collapsible directories and drag/drop moves are acceptance requirements.

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
| Cloudflare Actions | PARTIAL | native CI and distinct domain verified; UI/controls/security gates C12–16 pending |
| Repository/profile/editor/product quality | STILL PRESENT | supplied screenshots and current source; C17–21 |
| Certification gate false positive | STILL PRESENT → addressed in C0 | pipefail/gofmt grep pipeline replaced by captured output |

## C0 — baseline

Go test ./..., go vet ./..., go build ./..., git diff --check and nift build --all passed. Existing coverage is sparse; these are baseline checks, not release certification. Refreshed remote without altering history. Linode switchyard/caddy active, localhost Trestle responded 302. Live browser dashboard still contains the prototype card structure and demo links; product parity remains unfulfilled. Cloudflare-specific runtime certification remains pending; credentials were not printed. Formatting gate now fails reliably on existing drift instead of masking a SIGPIPE failure. No production deployment in this checkpoint.

## Checkpoint ledger

C0–C13 locally implemented and verified. C14 run controls verified; preview/deployment setup remains unavailable. C15 credential boundary verified; C16 alignment review complete; C17 shared design foundation verified; C18 repository redesign verified; C19 workspace and identity redesign verified; C20 workbench verified; C21 decision surfaces verified; C22 Chromium matrix verified; C23–C25 pending. Checkpoint identities are recorded in the batch handover ledger below.

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

## C5 — immutable integration and exact expected SHA

Continuation authorization supersedes the earlier C4 batch stop. Git publication now checks the exact advertised old SHA with a private pre-push hook and retains non-force fast-forward checks; receive-pack supplies the final old-object compare-and-swap. Prepared merges capture base/source/commit/tree identities. Queue preview, semantic validation, change-scope risk and final publication use that same candidate, and publication rejects a changed candidate or moved source/base. Checks are stored separately in `commit_checks` against source SHA; legacy branch-only checks cannot authorize integration. Direct PR integration and workflow integration enqueue instead of bypassing queue validation. Repository archival and organization policy reads fail closed; policy examines all applicable records. Semantic validation rejects unsupported rules, missing referenced files and type mismatches.

Validation: full Go race suite, vet and diff checks pass. Real disposable bare-Git tests cover exact publication, branch creation, remote rewind, changed advertised head and remote movement inside pre-receive after advertisement; rejected publication preserves the external update. Semantic tests cover absent/malformed contracts, equal values, type mismatch, missing files and unsupported rules. These are local protocol tests; Cloudflare-specific publication certification remains part of C25. Provision `commit_checks` before deployment. Durable queue claims/effects and workflow waiting/recovery remain C6/C7; the workflow integration effect now returns a queued result rather than incorrectly claiming a synchronous merge.

C5 commit: `d5bb9b0`.

## C6 — durable Integration Queue ownership and publication recovery

A separate `integration_effects` collection records claim owner, expiry, phase, prepared candidate identities/private scratch location and published SHA. Claims use one read/version CAS; failed writes cannot grant ownership. Expired owners cannot advance or release a replacement's claim. Per-repository/base lanes serialize workers. Queue requests deduplicate by PR/source SHA, and direct/workflow integration follows the queue. Running items are reclaimable; durable candidate publication is reconciled against canonical Git before repeating any effect. PR and queue completion errors are reported and remain recoverable. Completed effects prevent replay even when visible queue state briefly disagrees. No history rewriting is used.

Validation: actual child processes exit at after claim, after checks, after preview, after semantic validation, before publication, after publication, before PR completion and before queue completion. A surviving fixture advances the lease clock, restarts processing, checks PR/queue/effect completion and verifies exactly three canonical history commits. After-publication recovery preserves the same canonical SHA. Competing claims, expiry takeover, stale-owner fencing and failed claim PATCH are covered under the race detector. Additional real-Git tests reject moved source/base and modified candidate commit/working tree. Full race suite, vet and diff checks pass.

Operational limits: prepared candidates live in the private durable scratch directory; losing both coordination and scratch storage requires restore/reconciliation rather than blindly rebuilding a previously publishing candidate. The lease is ten minutes; bounded executor/Git operation deadlines and shutdown integration follow in C8/C24. Cloudflare-specific effects are not claimed certified by local fixtures. Provision `integration_effects` before deployment. No live resources, reference repos, `sy`, push or deployment were changed.

C6 commit: `4860dd9`.

## C7 — workflow replay, gates and recoverable Git effects

Replay identity includes operation and canonical arguments. Budget checks precede durable intent/effects; retry attempts are durably counted rather than hidden inside an inaccurate retry loop. Step/run persistence failures are returned, with completion-write failures leaving the run recoverable. Approvals persist a waiting step; unapproved replay cannot pass, including legacy `approved:false` results. Approval is recorded before resuming the run and a recovery scan closes the intervening gap. Child waiting follows the actual step's child, and integration waits for its queue publication receipt instead of continuing with a queued result or returning a timestamp as a SHA.

Workflow workers use durable claims. Validation is bounded to one second and scripts to 256 KiB; execution has a two-minute wall-clock bound, cancellation polling and a bounded JS call stack. Date constructors and random values use deterministic sources. Agent Git effects persist a prepared candidate/result before publication and reconcile that exact candidate after process death. Claim IDs now use cryptographic randomness. New collections: `workflow_claims`, `workflow_git_effects`.

Validation: real child-process death tests cover intent, effect and completion boundaries for notes/approvals, and before/after real Git publication for prepared Agent effects. Approval remains parked without human approval; Git recovery publishes exactly one update. Tests also cover operation/argument mismatches, budget-before-effect, persisted retry limits, failed completion PATCH, approval/run patch gap, infinite validation/execution and deterministic Date. Full Go race suite, vet and diff checks pass. One fixture race found by these tests was corrected; it was a fixture map write without its mutex. Production subprocess cancellation/resource isolation is C8; Goja timeout/call-stack limits do not claim a hard heap sandbox. No push or deployment.

C7 commit: `4c5e1e1`.

## C8 — typed Go runner and sandboxed subprocess boundary

Runner accepts a context and typed task; executions report status, stdout/stderr, truncation, exit code and CPU time. Deterministic text remains trusted in-process Go, preserving empty/Unicode content. The CLI adapter uses JSON stdin/stdout, bubblewrap mount/PID/user namespaces, environment allowlist, private workspace/home/tmp, default-off network, scoped credentials, deadline/process-group cancellation and prlimit resource bounds. It fails closed without sandbox permission/tools and refuses root execution. A shared CLI instance serializes its workspace. Credential output is redacted and a returned file containing the scoped credential is rejected. HTTP proposals and workflows propagate cancellation. Execution persistence errors cannot report success; resource metadata lives in new `execution_metadata`. Trusted Git helpers now have deadlines, isolated global/system config/environment and bounded output.

Validation: real sandbox scripts verify host file/environment isolation, JSON result/diagnostics, timeout, output cap, credential redaction/removal, rejection of credential-bearing results and background-descendant termination. Deterministic empty/text/Unicode parity and cancellation pass. Full Go race suite, vet and diff checks pass. The process limit moved inside the user namespace after tests exposed shared-host thread accounting; namespace creation and adapter execution now pass. `docs/architecture/runner.md` records bounds and limitations.

Not a live provider certification: no coding-provider CLI/credential was enabled. Output events are attributed captured chunks after exit, not a streaming Cloudflare CI transport. Per-process address-space limits are not an aggregate cgroup memory budget. A dedicated non-root service user and bounded deployment concurrency are prerequisites for real CLI use. No upstream project, `sy`, push or deployment was changed.


## C9 — schema compatibility, migration ledger and typed persistence errors

Startup now reads each collection's complete schema and verifies field types, required/unique constraints and unexpected required fields. It creates missing collections but never silently alters existing schemas. A Switchyard-owned version/fingerprint ledger rejects a newer database or an unversioned manifest change. `-schema-plan` is read-only; `-schema-migrate` explicitly applies only reviewed additive fields/new collections and verifies the result. Existing field IDs/defaults are preserved. Constraint/type changes and new required/unique fields need a separate reviewed backfill, and are refused here. Trestle's upstream schema PATCH lacks CAS, so application and schema administrators must be quiesced in an exclusive maintenance window; the pre-write reread catches stale plans but is not claimed as distributed locking.

The client exposes typed HTTP conflict/validation errors, checks response/marshal failures, bounds response memory and follows cursor pagination rather than silently stopping at 100 records. FindRecord rejects ambiguous matches. Review/finding/attempt, execution, policy audit, Work, profile/avatar, settings, invitation and workflow mutations now propagate persistence errors in the touched paths. Finding closure uses the finding ID rather than a nonunique target. Invitation acceptance creates a stable membership before marking acceptance so a failed final write is retryable; status-only Work edits preserve body/assignee. Ref reconciliation reports failed baseline writes. Broader event ingestion durability is C10 and operational write audits continue in C24.

Validation: full Go race suite and vet pass; schema tests reject incompatible existing collections without writes, preserve additive IDs/defaults, reject stale plans, follow 237 records and detect cursor loops, and classify 409/412/422 errors without returning sensitive error details. No Trestle source, live schema or shared service changed. Operational migration procedure: `docs/architecture/schema-migrations.md`.


## Historical C10 prerequisite investigation (resolved below)

C9 commit: `6d744c5`. Cloudflare account checks confirm Workers/Workflows access but R2 is not enabled (HTTP 403, code 10042). The inspected official CI SDK 0.2.0 snapshots every successful runner to R2, including uncached runners. Existing OAuth can access Builds configuration, but no build token is registered; the standard Artifacts Builds integration has a dashboard setup contract and a connection API shape that is not yet verified for Artifacts. A bounded undocumented-provider probe was rejected without creating a resource; this is not proof of an upstream defect. Evidence and exact continuation prerequisites: `docs/evidence/campaign/C10-cloudflare-prerequisites.md`.

At that earlier stopping point, the sequential campaign awaited account-owner R2 setup before custom CI could be validated. This is a genuine cloud-service prerequisite, not a checkpoint batch boundary. C10 is not marked complete, and C17–C25 visual/editor/brochure work is not claimed done. No push/deployment or live schema change. That request was subsequently answered; see the C10 execution evidence below.

## C10 — R2 enabled; real Cloudflare CI architecture

The earlier R2 prerequisite is resolved by Nick. Implemented and exercised a native Artifacts push → deployed Worker/Workflow → official CI Sandbox runner → R2 snapshot/status/log path. Manual and native runs passed with exact-SHA output. Added approved JavaScript definition compilation, signed bounded control APIs, durable dispatch identity and immutable Git event receipts. Full Go race suite, TypeScript check and four Worker tests passed. Evidence: `docs/evidence/campaign/C10-cloudflare-prerequisites.md`; architecture: `docs/architecture/cloudflare-actions.md`.

C10 custom-CI architecture is complete; standard Workers Builds connection/token setup remains explicitly unavailable and must be reflected in C14 controls and final limitations. C11–C25 remain pending. No project push, shared deployment, sibling/reference edits or billing enrollment.

## C11 — distinct Actions domain and exact-SHA checks

C10 commit: `e2d6abd`. Added definitions/settings, immutable run identities, normalized jobs/steps/checks/external execution records and signed provider polling; schema version 3 is additive. Canonical repository list/detail endpoints authorize before effects. Required Actions checks replace the marker demo gate when configured and must match current definition revision and exact source SHA. Queued reruns fence earlier success; timestamp ties fail closed; terminal snapshots cannot regress. Integration Queue resumption revalidates checks immediately before publication. Worker native event claims deduplicate at the R2 conditional-write boundary; colliding job/step log keys are rejected.

Verification: full Go race suite and vet passed before the final marker-gate change; the final app/CLI race regression run also passed. Worker TypeScript check and five protocol tests passed. Backend fixtures cover recorded real Cloudflare snapshot normalization, exact-SHA/definition invalidation, queued reruns, timestamp ties, replay, terminal regression and failed check persistence. No shared schema/deployment or project push. C12–C25 remain pending; Workers Builds setup remains unavailable and explicit.

### C11 follow-up from actual preview

Real local Trestle polling exposed redundant writes: typed Go snapshots and decoded JSON maps did not compare equal, so unchanged terminal views were patched every poll. Compare their normalized JSON values before persistence. A regression verifies replay leaves all five view record versions unchanged. App race tests passed. This correctness fix is committed separately before the C12 UI checkpoint.

## C12 — canonical Actions list and run details

C11 commit: `802a4c9`; polling follow-up: `bac3d6d`. Added a Nift-built Actions route/tab with a repository sidebar, run search/status/branch filters, status icons, exact commit links, actor/event/timing summary, selectable jobs, expandable steps and secondary provenance. Empty/error states are readable; provider JSON is not presented as product copy. Root-relative Nift asset/navigation paths fix canonical-route loading and are protected by an actual HTTP asset regression.

Verification used the real Go application, an isolated local SQLite Trestle instance and signed Cloudflare provider; the actual native run was imported by the reconciler. Seven permanent staged browser assertions passed (populated list, status/reset, search/reset, run navigation, step disclosure, overflow and genuine empty state). Desktop and 390 px screenshots are under `docs/evidence/campaign/C12`; measured mobile document width 375 <= 390. App race tests, Nift build and JavaScript syntax checks passed. This is not the C22 six-width/full-product certification. Logs and operational buttons follow in C13/C14; no fabricated preview/deployment or build artifact is claimed.

## C13 — bounded read-only Actions logs

Implemented repository/step-authorized exact-SHA log pages, inert downloads and cursor-resumable SSE with disconnect and wait bounds. Warden-inspired ANSI text rendering provides follow/pause, search, copy, capture timing, download and reconnect without stdin or execution. Actual Cloudflare output passed browser search/reconnect checks; evidence and capture limitations are recorded in `docs/evidence/campaign/C13`. Actions/app race tests and ANSI security tests pass. C14–C25 remain pending.

## C14 — durable Actions controls

Implemented canonical authenticated manual dispatch, exact-source PR dispatch, rerun all/failed, idempotent cancellation and administrator JavaScript definition approval. Queued intent is persisted before Cloudflare effects and recovered after ambiguous responses; failed-job reuse is bound to parent SHA, commands and definition. Real disposable manual, browser-selective-rerun and cancellation-before-manifest evidence is in `docs/evidence/campaign/C14`. Actions/app race tests, vet and six Worker protocol tests pass. Preview/deployment links remain unavailable without a verified Workers Builds connection/token, so that portion is not certified. C15–C25 remain pending.

## C15 — CI credential boundaries

Strict approved configuration exports reject arbitrary environment/secret/credential options. Administrator grants control approval; provider rejection occurs before Trestle source persistence. Typed references expose only encrypted Worker binding names/purposes. Runner credential injection remains disabled, and known privileged values/encodings are redacted from captures. A real sandbox probe found no privileged command/readable process environment or persistent Git authorization header; evidence and limits are in `docs/evidence/campaign/C15`. Seven Worker protocol tests and Actions/app race suites pass. Generic CI has no organization-secret or privileged deployment injection; Goja hard heap isolation remains an operational-hardening item. C16–C25 pending.

## C16 — current competition and platform alignment

Read the current challenge, announcement, submission form, rules, Artifacts/custom CI and Builds documentation. `docs/plan/cloudflare-competition-alignment.md` maps actual Worker/Artifacts execution to evidence and remaining gates, records residency/license/manual-entry prerequisites and avoids claiming eligibility or submission. Added source-level runnable README and corrected active authority/runtime descriptions while preserving historical CP0 evidence. Existing MIT LICENSE verified. C17–C25 remain pending; preview/deployment setup and full dogfood/submission materials remain uncertified.

## C17 — consolidated design foundation

Replaced the accumulated shared CSS overrides with one components stylesheet and consolidated page layout declarations. Buttons, icon controls, forms, tabs, badges, tables, panels, dialogs, tooltips, status/empty/loading states, avatars and code surfaces use shared graphite/amber tokens. Removed the undefined page token and hard-coded Actions panel/dialog chrome. One navigation renderer now serves both Nift and hand-written pages, with desktop links, SVG controls, focus trapping, inert background and Escape/focus restoration. Header/menu padding and dimensions share a single rule. Versioned asset references prevent stale token files being combined with new layouts; this failure was observed and corrected in the real browser.

Validation: Nift rebuilt all six generated pages; JavaScript syntax, static accessibility across 17 pages, formatting and whitespace gates passed. Permanent browser shell checks ran on an actual Actions run at 1280px and the actual Alice profile at 390px: 14 checks, exact matching open/close rectangles, no document overflow, background isolation and focus restoration. Evidence is in docs/evidence/campaign/C17. Mobile overlay scrolling initially introduced a second gutter; scroll now belongs to menu content. These are foundation checks, not a claim of product-wide GitHub parity or the C22 matrix.

## C18 — repository browsing and source hierarchy

Replaced the flat source prototype with a directory table, real parent/folder navigation, a collapsible keyboard tree beside files, path search with keyboard selection, canonical breadcrumbs and coherent source controls. The root view presents identity/visibility/description, branch/tag selection, actual latest commit author/message/SHA/time, recent-history link, README and About sidebar. Branch/tag dialogs use real refs. Markdown files offer Preview/Code; source lines have anchors, raster previews are limited to safe MIME types and binary/large previews explain their limits. Content/tree reads use the exact commit displayed in the overview. The overview respects the selected ref, propagates history failures and treats genuinely empty refs as empty; capabilities determine write controls. An unborn branch requires a first Git push before editing.

Validation: actual disposable Artifacts nested fixture pushed with ordinary Git; browser folder/parent navigation and keyboard path search opened actual `src/routes.mjs`. Permanent staged browser checks passed seven source and four 390px empty-state assertions. Two Node hierarchy tests, full Go race suite, vet, Nift and static gates passed. Evidence: docs/evidence/campaign/C18. Per-file commit history and a total commit count are not fabricated; the sample is labeled recent. The slow current overview read is recorded for C23/C24. These layouts establish the repository foundation; C22 owns the broader matrix.

C17 commit: `af3886f`.

## C19 — workspace, identities, settings and authentication

C18 commit: `676a38d`. Replaced prototype dashboard/demo panels with ordinary accessible repositories, pull requests and recent Work. Work creation now uses a focused native dialog; account and organization profiles have real selected tabs, repository search/visibility filters, pins/activity and People/Teams. Account/org/repository settings share section navigation, readable forms and contextual links outside the global header; Actions configuration is reachable from repository settings. Authentication has centered forms, registration name/new-password state, bounded input, disabled submission and friendly errors. Removed the previous competing identity/settings CSS families and use a single identity stylesheet with shared tokens. Fixed canonical Work/Pulls/Sign-in routing and the Work trailing-slash detail collision. Work lists attach authorized repository/assignee detail fields.

Full Go race suite/vet, static product gates and 15 permanent browser assertions passed. Actual isolated fixtures and 1280/390px evidence are in docs/evidence/campaign/C19. Ordinary history author/time rendering and error handling are corrected. Rich PR/diff/editor and decision surfaces continue in C20/C21; full product widths and states remain the C22 gate. No project push, shared deployment or reference edits.

## C20 — Warden-style repository workbench

C19 checkpoint identity: `adde19d`. Added full-height Files/Search/Editor/Agent layout with hierarchical keyboard explorer, persistent directory expansion, multiple per-file undo/selection/scroll states, language-aware CodeMirror, safe context menus, draft-backed file/directory creation and explicit Git move/delete commits. Real drag/drop invokes the same authorized server operation. Gantry Core is a pinned dependency used for query compilation and binary detection; reference repositories are unchanged. No interactive terminal. Unified/split inert diffs serve editor, history and PR surfaces.

Backend immutable tree/content/search reads are bounded; file moves validate containment, modes, collisions, active drafts and expected branch SHA. Browser direct-write routes enforce protected-ref policy. Committed draft reload retains CAS revision metadata; delayed Agent proposal against an older human revision is rejected without overwriting the newer save. Executable mode survives a real Git move regression.

Full Go race suite and vet, three Node hierarchy/diff checks, 17-page static gates and eight permanent workbench browser assertions passed. Actual disposable editor branch browser proof covers explicit draft commit, real drag/drop commit `49f8e2f`, file creation/undo/redo/save/commit `ebb73e5`; server probes cover stale SHA and active-draft blocking. Desktop/390px evidence and adaptation/limits are in `docs/evidence/campaign/C20`. External intelligent Agent execution, full responsive matrix and production deployment remain later gates. C21–C25 pending.

## C21 — human decision surfaces

C20 checkpoint identity: `8528adc`. Replaced fake Inspect links and operational record rows with repository context, stop reasons, recorded evidence, inspection destinations and consequences. Workflow step history loads on demand; approval/retry/cancellation and queue revalidation use explicit consequence dialogs and existing backend authorization. Attention includes failed Actions and failed workflows; partial source reads remain visibly incomplete. The benign local workflow reached waiting approval, was inspected and approved through the browser, then completed with durable step replay. A real malformed-call failure remained inspectable. App race/static checks and permanent browser assertions passed; evidence is under `docs/evidence/campaign/C21`. C22 Chromium matrix verified; C23–C25 pending.

## C22 — browser certification and demo polish

Actual six-width Chromium product/menu matrix, genuine populated/empty fixtures and brochure verification completed; details and limits are in `docs/evidence/campaign/C22/README.md`. Fixed public canonical demo authorization/import, pagination and scrolling ownership, editor selection/shortcuts, persisted stars/follows and contribution profiles. Profile avatars are 250px with rounded square borders; neutral active navigation and primary action colors remain distinct. Linode assets and additive schema updates are synced under existing authorization. C23 performance remains next; architecture exploration is design-only in its separate commit.

## Accepted extension — C45/C46 (October 4)

After the current C22–C44 certification, implement C45 immutable repository ZIP
and tar.gz downloads, then C46 first-class releases. Full accepted requirements:
[codex-c45-c46-archives-releases.md](codex-c45-c46-archives-releases.md).
C45 resolves branch/tag once to a SHA, streams bounded archives through canonical
authorized routes and adds Download ZIP to Code. C46 keeps immutable target SHA,
authorized draft/publish lifecycle and audited metadata in Trestle; binary assets
are separate R2 objects with SHA256/size metadata. Actions integration and later
sy archive/release commands use the same contracts. C45 server ZIP/tar.gz and browser Download ZIP are implemented locally at
67b1992; immutable archive and CLI checksum evidence passed. `sy repo archive`
is implemented locally at d6dbd16. Linode deployment remains pending. C46 is
not implemented or certified. SSH remains deferred; no fake tab is added.

## C25 addition — personal providers and task models

Implemented personal provider/model/credential defaults, independent implementation, review and conflict-resolution selections, and per-invocation pickers in the editor and Work Attempt controls. Workflow implementation inherits the initiating user's selection. Overrides do not overwrite saved defaults; credential ownership and provider matching are enforced server-side. Full Go tests and vet pass; isolated Chromium saved/reloaded distinct role choices and opened the inherited review picker. Evidence and explicit execution limits: `docs/evidence/campaign/C25/provider-settings.md`. Real provider authentication and coding execution now pass locally with the candidate cgroup adapter; non-root production execution and full multi-role certification remain pending. This addition does not mark the remaining C23–C46 campaign complete.

## C25 runner investigation — 4 October 2026

C25 remains PARTIAL / blocked on complete real runner certification. A genuine OpenCode Go credential now exists privately. Local non-root baseline and five harmless filesystem/cgroup sandbox requests returned actual model responses. RLIMIT_AS 2/4 GiB failures were independently reproduced despite a much smaller resident footprint; per-user RLIMIT_NPROC 256 independently fails on the desktop. Candidate cgroup-backed execution preserves filesystem/credential/time/output boundaries. Six sequential coding tasks and two- and three-Agent concurrent runs published verified edits on disposable branches. Four-Agent certification exposed an ambiguous-publication recovery bug; execution-specific recovery is fixed and repeats remain NOT CERTIFIED due to explicit runner failures/upstream timeouts. No Linode capacity conclusion is justified. Evidence: `docs/evidence/campaign/C25/real-agent-runner.md` and `runner-resource-study.json`. Remaining coding/concurrency/Linode/full-flow gates must pass before C25 DONE.

## Accepted continuation — runner stage diagnosis and final campaign gates

Preserve runner commit `80c7de0` and sy archive commit `d6dbd16` without squashing. C25 = **PARTIAL**.

| Gate | Current evidence |
|---|---|
| Real provider authentication | PASS, local |
| One Agent | PASS, six sequential coding tasks |
| Two concurrent Agents | PASS, exact outputs and branch isolation |
| Three concurrent Agents | PASS, exact outputs and branch isolation |
| Four concurrent Agents | NOT CERTIFIED; publication bug fixed, later failures recorded |
| Linode Agent execution | NOT TESTED |
| Full multi-role dogfood | NOT COMPLETE |
| C45 server/browser archives | Implemented and isolated certification passed; Linode pending |
| sy repo archive | Implemented locally, real immutable checksum proof passed |
| C46 releases/R2/CLI | NOT IMPLEMENTED |

Next order: classify failures and instrument stage timings before further stress tests; test 768/896/1024 MiB real-memory budgets with comparable repeated tasks; establish a safe non-root Linode execution boundary and reserved control-plane budget before exactly one Agent test. Do not resize or change production concurrency from local failures. Complete real role-separated review/conflict/re-preview/re-review/queue/Actions flow. Continue C23 genuine semantic timing and C24 restore/reset/drain independently. Deploy only verified committed builds with backup/version/integrity/rollback checks. Complete C46 server/R2/Actions then sy releases; final C42/C43 and brochure refresh certify the final product, not an intermediate state.

The accepted full continuation is the user attachment dated 4 October, `2af2c5c1-fbb8-4d2d-856f-491ed9316c3b/Pasted text.txt`. Final report must cover A–R from that brief, including failure analysis, capacity evidence, complete checkpoint ledger and remaining blockers.
