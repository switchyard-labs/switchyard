# Proposal and Pages feasibility review

Date: 5 October 2026. Status: investigation only; awaiting review.

This report addresses the supplied Proposal/Pages brief and naming audit, including the preferred owner-subdomain/project-path URL model. It changes no application behavior, identity validation, DNS, infrastructure, or production deployment. Recommendations below are proposed work, not certification that either feature exists.

## A. Proposal recommendation — BUILD WITH CHANGES

Build a lightweight repository intake object called **Proposal**, with optional links to execution. Do not also create an independent Issues system. Keep Work usable without Proposal and allow Proposal to conclude without Work, code, review, or integration.

The important change to the brief is to avoid making the Proposal state machine represent every downstream activity. Investigation can be Work of kind investigation; a decision can be a recorded discussion/event. Neither needs an independent CRUD subsystem initially.

There is a naming collision in the earlier [Agent design exploration](agent-native-design-exploration.md): it uses Proposal for an exact-SHA change/review manifest. The new request uses Proposal for intake. Adopt the new meaning explicitly; call the earlier concept a candidate/checkpoint or change proposal in prose, not another entity named Proposal. Existing Attempts and PRs already cover code candidates.

## B. Proposed Proposal model

Core collections, using stable IDs rather than mutable owner/repository strings:

| Collection | Essential fields |
| --- | --- |
| `proposals` | id, repository_id, type, title, description, state, closure_outcome, author_principal, created_at, updated_at, version |
| `proposal_comments` | id, proposal_id, author_principal, body, created_at, optional source reference |
| `proposal_links` | id, proposal_id, relation, target_kind, target_id, created_by, created_at, idempotency_key |
| `proposal_events` | id, proposal_id, actor, event_kind, previous/new state, reason, operation_id, timestamp |

Types: Bug, Suggestion, Feature, Improvement, Refactor, Documentation, Security, Performance, Experiment, Question, Other. Add labels, optional severity and priority as bounded metadata. Defer confidence scoring, custom types, epics, estimates, sprints, and dependency scheduling.

States: `open`, `accepted`, `deferred`, `closed`. Closed outcome: `rejected`, `answered`, `documented`, `resolved_without_code`, `superseded`, or `completed`. Reopening records an event and clears the current outcome while preserving history. Acceptance records intent; it does not create Work automatically. “Investigating”, “active”, and “blocked” are derived badges from linked activity, not extra Proposal states. Linking Work also does not automatically close the Proposal.

Links initially support related/supersedes Proposal, spawned Work, source Finding, and evidence artifact/execution. One Proposal can spawn several Work items; Work can be linked to several Proposals. Prevent self-supersession and supersession cycles. Do not turn ordinary related links into a workflow dependency graph.

Reuse repository read authorization for visibility. Submission requires authentication plus an explicit repository intake policy; a public reader may submit when intake is enabled, without gaining WriteRepo. Authors may edit their own discussion; maintainers triage, supersede and accept/reject. Work creation retains its existing execution permissions. Security intake should default to restricted visibility or be disabled until restricted intake is implemented; do not label a public submission “Security” and imply confidentiality.

Agent submissions are normal Proposals with provenance: delegated human/service principal, execution ID, role, provider/model, exact source SHA when relevant, evidence IDs, and deduplication key. An Agent cannot invent a new authority, accept its own suggestion as a maintainer, or trigger deployment merely by creating intake. Apply rate limits and evidence-size limits.

## C. Proposal versus Issues

Proposal gives broader, explicit consideration/decision semantics while still supporting conventional bug reports and questions. This is an incremental improvement over Issues, not inherently an Agent breakthrough. Its value comes from evidence and execution links, not terminology.

Import GitHub/GitLab Issues into Proposal with external instance/project/issue ID as an idempotent source identity. Preserve title, body, labels, comments, original author attribution and timestamps separately from the importing principal. Infer type only from configured labels/templates; fallback Other. Map open to open and closed to closed, with imported/unknown resolution metadata rather than guessing “completed”. Preserve external links and issue numbers without requiring them to become internal IDs. Attachments need a separately authorized bounded import; do not blindly fetch URLs embedded in comments. Imports come after native intake dogfood.

## D. Optional progression and stopping semantics

All of these are legal:

```text
Proposal → closed(rejected)
Proposal → comment answering question → closed(answered)
Proposal → investigation Work → deferred
Proposal → decision event → closed(resolved_without_code)
Proposal → accepted → several Work items
Work → Attempt → PR → integration, without any Proposal
Proposal → documentation Work → closed(documented)
Proposal → Work cancelled; Proposal remains open or explicitly closes
Proposal → superseded by another Proposal
```

Use object-specific meanings: Proposal rejected is a decision; investigation abandoned is cancelled Work; an Attempt withdrawn is an execution outcome; review requests changes/rejects a candidate; a stale queue candidate is blocked/invalidated and may be regenerated. A queue rejection must not reject the underlying idea automatically. Current Work only supports open/closed, so adding explicit cancellation outcome is new work, not existing behavior.

Finding promotion should be an explicit “Create follow-up Proposal” action with a reviewable title/body. Copy evidence and retain a source link; do not resolve the Finding automatically. Snapshot source SHA and review execution provenance at promotion. Current Finding records are attached to Attempts and do not independently store the complete reviewed-SHA provenance; historical evidence must not be fabricated.

## E. Pages recommendation — BUILD WITH CHANGES

Build static Pages after Proposal. Use explicit configuration that compiles into an approved Action; Pages consumes its immutable static artifact. This is the brief’s Model 3 as product UX, implemented through Model 2 internally. Do not create a second build scheduler or use the small control-plane Linode for builds.

Preferred permanent URLs:

```text
https://{owner}.switchyard.cx/
https://{owner}.switchyard.cx/{repo}/
```

The owner chooses the backing repository explicitly. `strut-labs/strut-website`, branch main, working directory `.`, command `nift build`, output `public` can back the owner root. `strut-labs/docs` can independently back `/docs/`. No special repository name is required.

## F. Pages architecture and configuration

```text
Artifacts exact source SHA
  → approved Action definition + isolated build
  → verified static-site artifact + manifest
  → immutable PagesDeployment in Trestle
  → immutable file objects in R2
  → conditional serving-map publication
  → wildcard-routed Worker
```

Extend the existing approved Actions configuration convention rather than adding another executable JS configuration. A typed Pages section specifies source branch/ref, working directory, build command and output directory. Settings hold the owner/site mapping, publication permission and explicit public exposure approval. Repository-controlled changes to commands or export paths require renewed configuration approval. Store the approved revision/hash in every deployment.

Resolve ref to SHA once before scheduling. Production selected-branch push, manual deployment and exact-SHA Attempt/PR previews are trigger policies for the same Action machinery. A branch moving while a build runs does not change its source. Action completion consumes a verified receipt; workflow-produced artifacts must satisfy the same contract.

Working/output directories are relative, normalized paths within the workspace. Output is relative to the configured working directory. Reject absolute paths, traversal and symlink escape. Support a bounded prebuilt-site mode without a command; do not pretend a no-op command proves a build occurred. Nift is a happy path, not a framework requirement. The brochure already uses output-aware `@path` links, but this does not certify arbitrary Nift projects or automatic prefix conversion.

Artifacts owns Git truth. Trestle owns configuration, deployment metadata, permission decisions, promotion intent/history and a projection of serving state. R2 owns immutable content and the actual serving-map commit point. This deliberately refines the brief’s “Trestle production pointer”: avoid two independent authoritative pointers with an impossible cross-store atomic update.

Recommended R2 layout:

```text
pages/{site_id}/{deployment_id}/files/{manifest_file_id}
pages/{site_id}/{deployment_id}/manifest.json
pages-routing/{owner_id}.json
```

Manifest entries map normalized URL paths to opaque file keys, hash, size and approved MIME metadata. Do not form storage authority from user-controlled slug concatenation. Use a separate Pages bucket and read-only serving binding where practical; reuse release upload/checksum code, not release publication semantics or its credentials.

Cloudflare R2 supports conditional writes, ranged reads and HTTP metadata, making this serving design feasible. [R2 Workers API](https://developers.cloudflare.com/r2/api/workers/workers-api-reference/)

Serve GET/HEAD, conditional ETags, a single validated range, explicit MIME types and `nosniff`. Resolve directory indexes only inside the selected site. Keep absent-file 404 responses at 404 status. Do not expose bucket listing. Compression should use verified precompressed variants or platform behavior with correct Vary/ETag handling; defer clever on-the-fly transforms. Mutable production URLs need revalidation; immutable preview content can cache by deployment ID. Do not assign long immutable browser caching to unversioned production asset paths.

## G. Security and naming prerequisites

### Identity audit

Actual code in [repository_identity.go](../../internal/app/repository_identity.go), [auth.go](../../internal/app/auth.go), [policy.go](../../internal/app/policy.go), [profiles.go](../../internal/app/profiles.go), and [CLI Git detection](../../../sy/internal/gitutil/git.go) establishes:

- Owners already support hyphens, normalize to lowercase, and use a shared user/org `owner_namespaces` collection with unique slug.
- Owner validation permits 1–39 characters; signup separately requires at least three. Standardize new user/org creation at 3–39, preserving existing shorter identities rather than silently renaming them.
- Owner dots/underscores and leading/trailing hyphens are forbidden. Consecutive hyphens are currently permitted and can remain permitted except reserved prefixes.
- Repository validation permits lowercase letters, numbers, dots, underscores and hyphens, 1–100 characters. It also permits `.` and `..`, which must be rejected before Pages.
- Logical repository identity is separate from physical `artifact_name`. Allowing a logical slug does not prove every provider/import/clone endpoint accepts the same physical name.
- CLI inference removes a trailing `.git`; literal names ending `.git` require an explicit ambiguity policy.
- Signup and org creation write their identity record before ensuring the shared namespace; a race/failure can leave an orphan record. Preflight reads do not replace a durable unique claim. There is lazy user namespace repair, but Pages needs an explicit inventory/backfill/recovery pass.

Recommended **new** repository policy retains `[a-z0-9._-]{1,100}` but additionally rejects `.`/`..`, a leading dot or hyphen, and a trailing dot or `.git` suffix. This is a conservative path/clone policy, not a DNS requirement. Existing incompatible names need a compatibility/remediation inventory; never apply a global regex change that makes existing repositories disappear.

| Input | Current validator | Proposed new-name policy |
| --- | --- | --- |
| `foo-bar`, `foo.bar`, `foo_bar`, `foo-1.2` | accepts | accept |
| `foo..bar`, `foo-` | accepts | accept; one literal path segment |
| `.`, `..`, `.git`, `.trailing` | accepts | reject |
| `-leading`, `leading.` | accepts | reject |
| `foo.git` | accepts | reject initially; revisit after clone interoperability tests |
| `foo/`, `foo/bar`, `foo\\bar`, `foo%2Fbar` | rejects | reject, including encoded separators |

Reserve existing infrastructure names plus `www`, `pages`, `static`, `admin`, `login`, `signup`, `sy`, and the entire `dpl-` prefix. Inventory collisions before activation; preserve existing ownership and fail closed on ambiguous hosts. Never silently evict a current account to reserve infrastructure. Reject encoded slash/backslash, malformed escapes, control characters and dot segments before URL normalization can hide them; decode once and validate the canonical result. Test double encoding as well.

Audit every boundary in a prerequisite checkpoint: registration/import, canonical routes, Git clone URLs, archive/release routes, Actions resource IDs, workspace paths, browser forms and sy. Backends/workspaces should use stable IDs or hashed identifiers rather than raw names. Renames are not certified by this review. Initially freeze Pages alias renaming or implement explicit migrations with retained/tombstoned old aliases; do not silently derive new hosts from display-name edits.

### Build and publication security

Reuse CI Sandbox execution only after verifying non-root execution, CPU/memory/PID/time limits, isolated workspace, minimal environment and absence of control-plane filesystem access. Agent sandbox certification does not certify the CI build environment automatically. Build code receives no general R2/Cloudflare deployment credentials. A trusted uploader receives a short-lived capability for one artifact/run/site, capped by bytes/files/expiry.

Current trusted build exporter verifies individual regular files; it is not a recursive static-site exporter. Add a bounded exporter that rejects symlinks and special files and captures a stable output snapshot after build processes terminate. Hash/upload the same opened data, not separate path opens vulnerable to replacement. Upload keyed files directly; do not extract arbitrary archives inside the serving Worker.

Proposed initial policy limits, subject to G0 measurement: 10,000 files, 32 MiB per file, 256 MiB total, path length 1,024 bytes, depth 32, manifest 2 MiB, ten-minute build timeout. These are product proposals, not Cloudflare limits. Explicitly exclude secrets, `.git` and configured sensitive files from publication; inspect output before enabling public exposure.

V1 supports explicitly approved **public** preview/production only. Repository public visibility does not imply publication permission; private repository publication requires an affirmative disclosure decision. Public preview URLs are not confidential merely because the ID is difficult to guess. Restricted previews require a later authenticated access design.

Shared owner origin is accepted for v1: project paths are routing boundaries, not browser security boundaries. Storage, scripts and service workers can interact across an owner’s sites. Treat all publishers under one owner as an origin trust group; warn org administrators when delegating publication. Strip artifact-controlled Set-Cookie and unsafe response metadata. Preserve host-only control-plane cookies; current session cookie has no Domain attribute. Prefer `__Host-` cookies once HTTPS deployment allows it and never broaden them to `.switchyard.cx`. Pages must not serve authenticated control-plane APIs or inherit their session authority.

## H. Preview, production, precedence and atomic promotion

Use globally unique single-label `dpl-{opaque-id}.switchyard.cx` preview hosts. Reserve the prefix; IDs are independent of repo names. Do not use `repo.owner.switchyard.cx`.

In a full Cloudflare zone, Universal SSL covers the apex and first-level subdomains; deeper hostnames need additional certificate handling. Use proxied wildcard DNS plus a Worker Route, with explicit infrastructure-host exclusions. Workers Custom Domains match exact hostnames and do not provide wildcard matching. Zone ownership, DNS setup and active certificate coverage have not been verified in this pass. [Universal SSL limitations](https://developers.cloudflare.com/ssl/edge-certificates/universal-ssl/limitations/), [Workers Custom Domains](https://developers.cloudflare.com/workers/configuration/routing/custom-domains/), [Worker Routes](https://developers.cloudflare.com/workers/configuration/routing/routes/)

### Base path

Record `base_path` in the artifact contract and expose `SWITCHYARD_PAGES_BASE_PATH` to the build. Owner site uses `/`; repository site uses `/{repo}/`. Generators must consume the value explicitly where needed. Do not rewrite arbitrary HTML/JS or assume root-absolute asset links work under a prefix.

A project preview must serve the **same prefix** as production: `https://dpl-ID.switchyard.cx/docs/`, with `/` redirecting to `/docs/`. That permits exact-artifact promotion. A root-site artifact cannot be promoted to `/docs/` without a new compatible build; repo renames can similarly require a rebuild or retaining the old prefix. Client routing is an explicit site-local fallback rule, never an owner-wide fallback.

### Routing precedence

1. Validate host and resolve owner/deployment ID from trusted registry.
2. For owner hosts, match the first complete URL segment against claimed project prefixes.
3. A claimed prefix selects that site exclusively, including disabled/tombstoned prefixes. Missing project files return its own 404; never fall through to the owner root.
4. If no claimed prefix matches, select the owner root site, if configured; otherwise 404.
5. `/docs` redirects to `/docs/`; `/docs2` does not match `/docs/`.

Owner root may contain content under unclaimed paths. Claiming a project prefix explicitly shadows that subtree; show a collision warning before claiming it. Reserve a claimed prefix on disable until deliberately released. Directory index, redirects and SPA fallback operate only within the selected site. V1 local redirect rules must remain in that prefix; external HTTPS redirects need separate explicit rules. No automatic “try owner index.html” after project failure.

### Atomic publication

Maintain one bounded R2 route-map object per owner (initially at most 100 sites/128 KiB). Each entry binds a stable site/repository ID, mount prefix, deployment ID, manifest hash and generation. Only a trusted publication service may write it.

Record a durable promotion intent and actor in Trestle; verify permission, artifact completeness, base path, expected previous deployment and configuration revision. Read the owner map and conditionally replace it using its ETag, changing only the selected entry and preserving other entries. On contention reread/revalidate; same-site stale operations return conflict, while independent sites can retry. Include operation ID so an ambiguous response can be recovered. After success, project the committed mapping into Trestle. Readers select one complete map version per request. A reconciliation loop repairs pending projections; it must not roll back a successful serving update because the database acknowledgement failed.

R2 provides strong consistency, but cached copies can remain stale. For v1, do not cache the mutable routing map or public aliases; cache immutable payloads by deployment identity. An in-flight request can finish against the prior deployment, and this design does not promise an atomic multi-request browser session. [R2 consistency](https://developers.cloudflare.com/r2/reference/consistency/)

Do not use Workers KV as the sole production pointer: its propagation can take 60 seconds or longer. If measured uncached map reads are too expensive or slow, evaluate a Durable Object coordinator with explicit serving consistency, rather than quietly weakening promotion semantics. [KV consistency](https://developers.cloudflare.com/kv/concepts/how-kv-works/), [Workers storage options](https://developers.cloudflare.com/workers/platform/storage-options/)

A repository publisher cannot claim another repository prefix or the owner root. Root selection/reassignment requires user ownership or org administration. Rollback is a new audited map update pointing to a retained immutable deployment, with expected-generation protection; no rebuild.

## I. Attempt/PR/Agent integration

Deployment links to its Action run, source repository/SHA, artifact and optional Attempt/PR IDs. Work is derived through Attempt rather than copied into every record unless historical snapshot semantics require it. Display Preview beside Changes, Actions and Findings, showing exact SHA and whether it matches the current candidate. Stale preview remains inspectable with a stale badge, not relabelled as current.

A reviewer can compare competing Attempt previews without merging either. Agents may request permitted preview Actions, but production promotion remains a separately authorized effect. Existing workflow durable effect/idempotency mechanisms can orchestrate optional build/promotion approvals; no mandatory Proposal→Pages pipeline is introduced.

## J. Custom domains — later phase

Cloudflare for SaaS is the natural product to investigate for customer-owned hostnames. It provides a custom-hostname lifecycle; DNS instructions, ownership verification and certificate readiness must all be checked. Do not activate routing until both ownership and TLS are ready. [Cloudflare for SaaS setup](https://developers.cloudflare.com/cloudflare-for-platforms/cloudflare-for-saas/start/getting-started/), [Hostname validation](https://developers.cloudflare.com/cloudflare-for-platforms/cloudflare-for-saas/domain-support/hostname-validation/)

Maintain globally unique domain claims by site ID, audit attachment/release, reverify ownership on transfer and retain takeover-resistant release history. Native-domain renames should not detach customer domains implicitly. Custom domains commonly mount at `/`; a project artifact built for `/docs/` is not automatically compatible. Require a root-base build or explicitly keep its prefix. Account entitlement, cost, quotas, actual domain control and operational renewal behavior remain unverified external prerequisites. No custom-domain provisioning in v1.

## K. Existing code reuse and gaps

| Current component | Reuse | Required extension/limit |
| --- | --- | --- |
| `internal/app/work.go` | Work, details, discussion, optional Attempts | Proposal intake/links/outcomes; recover multi-write operations |
| `internal/app/server.go`, `schema.go` | Collection definitions, versioned migrations | New collections/indexes/backfill; not cross-store transactions |
| `repository_identity.go`, `authorization.go` | Stable logical/physical repo mapping, permissions | Naming audit, explicit intake/publication capabilities |
| `reviews.go`, `provider_roles.go` | Findings, exact-source execution evidence, role models | Explicit promotion and provenance snapshot |
| Workflow effect/approval machinery | Durable optional orchestration | New authorized Proposal/Pages effect types |
| Actions definitions/settings/runs/jobs/checks | Approved builds, exact SHA, logs, cancellation | Typed static-site artifact receipts/export and CI sandbox certification |
| `actions_releases.go`, `release_assets.go` | Scoped file upload, checksum verification, R2 access | Static manifest/serving adapter; separate lifecycle and credentials |
| `workers/actions/src/build-export.mjs` | Trusted regular-file exporter patterns | Bounded recursive stable snapshot; no direct directory support today |
| sy repository resolution/JSON conventions | Canonical `-R owner/repo`, machine-readable output | New commands and `.git` inference edge-case tests |

There is no first-class intake Proposal implementation in the inspected schema/API. Current Work already supports a body and does not automatically create an Attempt or PR; the distinction is consideration versus execution intent, not “Work always means code”. Existing releases are versioned distribution events; Action artifacts are build output/evidence; Pages deployments expose a static artifact through serving mappings. Keep these three separate.

Compare alternatives before committing infrastructure: managed Cloudflare Pages or Workers Static Assets can reduce bespoke HTTP behavior, but introduce their own deployment/project lifecycle and credentials. R2/Worker is recommended here as an architectural inference because it matches the existing artifact provenance and dynamic owner-prefix model. G0 must validate latency/cost and compare operational burden; it is not a claim that custom hosting is always cheaper.

## L. New schema, API, UI and CLI work

Pages needs `action_artifacts`, `pages_sites`, `pages_deployments`, and `pages_operations`; later `pages_domains`. Deployment metadata includes repository/site IDs, ref/SHA, Action run/definition revision, configuration snapshot, base path, manifest hash, size/file count, creator, readiness/failure, creation time and visibility approval. Preview/production are serving assignments; immutable deployment identity does not change when promoted. Keep mutable upload/readiness state distinct from immutable sealed artifact metadata.

Provisional APIs follow existing repository authorization and canonical identity resolution: repo-scoped Proposal list/create, Proposal get/update/comments/links, idempotent Work creation from Proposal; repo-scoped Pages config/status/deployments, deployment creation, promote and rollback with expected version/generation. Precise route spelling should follow the current API during implementation, not become a contract from examples in this report. Reject replayed/unauthorized operations before invoking effects.

Proposal-to-Work creation uses an operation ID and deterministic link identity: current Work/detail writes are separate, so retries must repair partial creation rather than create duplicates. State/event writes need the same recoverability.

UI: compact paginated Proposals list and detail/discussion/evidence/linked-Work panels, with permission-aware triage actions. Use repository secondary navigation/overflow if tabs become crowded. Pages settings provide explicit source/working/build/output fields, root-versus-project assignment, public exposure warning, deployment history, preview and promotion/rollback. Follow the established bounded dashboard/card scrolling conventions.

CLI candidates: `sy proposal list/view/create/accept/reject/defer/work` and `sy pages status/deploy/deployments/promote/rollback`, with existing stable JSON, error/exit-code conventions, explicit repository selection and idempotency options. No commands implemented during this review.

## M. Failure, retention, observability and risks

Build failure, cancellation, incomplete upload, failed verification and deployment creation failure leave production unchanged. Publish only after manifest seal and serving readiness checks. A promotion whose map write fails leaves the previous mapping; an ambiguous acknowledgement is reconciled by operation ID. Routing deployment failure must not fall through to the control-plane origin. Partial uploads are quarantined and garbage collected after a lease expires.

Proposed retention: unpinned previews for 30 days, retain active Attempt/PR previews subject to quota, and pin current production plus at least two rollback deployments. Mark for deletion, recheck all routing/pin references, wait a grace interval, then delete content and reconcile metadata. Do not configure a blind bucket TTL that can delete live production or shared evidence. Site deletion first disables/tombstones the mapping; identity/domain claims have a separate release policy.

Record build/upload/verify/deploy/promote/rollback/expire events with operation IDs, actor, SHA and hashes. Track build duration, upload duration, bytes/files, queue age, failures, storage and aggregate serving latency/errors. Avoid invasive visitor analytics in v1. Logs must redact secrets; public deployment metadata must not reveal private source paths, logs or credentials.

Largest risks are shared-origin publisher trust, path-prefix incompatibility, identity takeover, partial durable effects, storage growth, untrusted output/export races, and accidentally building a second Actions or Releases system. Proposal risks are duplicate intake, vocabulary confusion and bureaucratic state expansion. None justifies a mandatory end-to-end pipeline.

## N. Implementation checkpoints, after review only

| Checkpoint | Deliverable and exit evidence |
| --- | --- |
| P0 | Approve terminology, state/outcome contract, permission matrix and schemas |
| P1 | Model/API/comments/events; authorization, concurrency and partial-write recovery tests |
| P2 | Paginated list/detail UI; legal no-code terminal flows dogfooded |
| P3 | One/many Work links, standalone Work unchanged, retry-safe creation |
| P4 | Finding promotion and delegated Agent provenance; no implicit Finding resolution |
| P5 | sy commands and native human/Agent dogfood |
| P6 | Optional GitHub/GitLab imports with preserved attribution and repeatable dedup |
| G0 | Naming inventory and route matrix; disposable DNS/TLS/R2 conditional-write spike; CI non-root/limits and hosting alternative/cost comparison |
| G1 | Static artifact contract, bounded exporter, sealed manifest, scoped upload capabilities |
| G2 | Explicit Pages configuration compiled to approved Actions; exact-SHA build evidence |
| G3 | Isolated Worker serving, MIME/ETag/range/index/404 behavior, prefix-safe routing |
| G4 | Single-label previews using production base path; Attempt/PR attachments and stale indication |
| G5 | Root/project claims, CAS promotion/rollback, parallel distinct-site promotions and crash recovery |
| G6 | Settings/history UI, sy, retention, quota and observability |
| G7 | Full regression/dogfood; only then native public production rollout |
| G8 | Separate custom-domain design/approval and provisioning campaign |

Mandatory Pages acceptance cases: hyphen owner; dotted/underscored repository; root and `/repo/` coexistence; `/repo` redirect; `/repo2` nonmatch; nested relative assets; root-absolute asset failure diagnosed; SPA fallback scoped; project 404 never owner fallback; disabled prefix stays blocked; encoded traversal/separator rejection; publisher cannot overwrite another site/root; same-site stale promotion conflict; different-site concurrent promotions preserve both; lost acknowledgement recovery; immutable rollback; unchanged base path through preview promotion; rename/tombstone takeover prevention; private source never implicitly public.

The feasibility pass inspected code and primary documentation. It did not run these future acceptance tests, provision wildcard DNS/TLS, exercise a Pages Worker, implement an exporter, or prove production-scale serving cost/latency.

## O. Priority recommendation — Proposal first, Pages second

Proposal closes a core intake/follow-up gap with relatively little new infrastructure. Implement P0–P5 before import breadth. Then proceed with Pages G0 as a bounded disposable feasibility gate, followed by G1–G7 only if its evidence passes. Do not run both full implementation campaigns in parallel or postpone all useful product work while chasing perfect Agent scaling. Pages builds belong on separate bounded CI capacity, consistent with the earlier control-plane/worker separation.

## P. Product and competition impact

Proposal is valuable chiefly when evidence becomes actionable durable intake and can stop cleanly without execution. A renamed Issues list alone adds little differentiation. Pages is more visually compelling: reviewers compare live competing Agent Attempts at recorded SHAs, and promotion/rollback demonstrates the existing provenance model. Static hosting alone is commodity functionality. Keep the distinctive connection to Attempt evidence and approved artifacts, without making hosting a prerequisite for ordinary Git collaboration.

Decision requested after this report: approve or amend the lightweight Proposal model and Pages architecture/checkpoints before implementation. The preferred permanent URL model is adopted throughout.
