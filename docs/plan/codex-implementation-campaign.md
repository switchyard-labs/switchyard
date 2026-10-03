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

C0–C13 locally implemented and verified. C14–C25 pending. Checkpoint identities are recorded in the batch handover ledger below.

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
