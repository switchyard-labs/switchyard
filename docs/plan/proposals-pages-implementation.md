# Proposal and Pages implementation ledger

Accepted 5 October 2026: implement Proposals first, certify them, then Pages. The accepted [feasibility report](proposals-pages-feasibility.md) remains the architecture contract. No pushes authorized. Do not reopen the product decision absent a concrete contradiction.

## Current state — live activation certified, 5 October 2026

Proposal + Pages are implemented and live on Linode/Cloudflare. Native Git push
→ approved Actions build → immutable R2 export → ready deployment → explicit
promotion and rollback passed with the real services. DNS/TLS are active, not
conditional. Signed-out surfaces default to public exploration or sign-in prompts
without flashing Alice's workspace. Public repositories remain public.

See [A–U handover](proposals-pages-live-handover.md) and
[live evidence](../evidence/proposals-pages/live/). Deployed app source is
`2ce7c717b22bed1d6f33e1d448ca968fa2bc1fb3`; the later `c5cefb4` adds a regression
test only. CLI `85f7683` passed the live reads and promotion/rollback sequence.
No campaign source was pushed, tagged or released. Disposable fixture Git pushes
were performed as authorized. Production fixture mapping is generation 6, owner
and project both restored to A; later ready previews remain immutable.

Reserved infrastructure names are now checked during registration and namespace
creation as well as Pages configuration. The first smoke exposed a missing
registration guard; its exact newly created www user/namespace/session were
removed, the guard fixed, and subsequent live reserved registrations rejected.

Remaining scope is deliberately deferred: custom domains, confidential Security
intake, imports, rename migration, automated retention/quota/billing, larger
history archival, and broader Actions repository rollout. Commercial proposer
quality remains a separate provider qualification question; no live coding Agent
was run on Linode during this static Pages certification.

## Historical checkpoints

The dated checkpoints below describe earlier local states. Their PARTIAL,
CONDITIONAL, pending and undeployed statements are historical and superseded by
the current state above; they are retained to preserve implementation decisions.

## Foundation checkpoint

Implemented locally:

- Schema version 9 adds Proposal metadata, comments, links and intake settings without changing Work parentage.
- Built-in types and bounded labels/priority/severity; lifecycle open/accepted/deferred/closed with explicit closure outcome.
- Repository-scoped list/get/create/update APIs; public reads follow repository visibility, mutations initially require repository write permission.
- Creation principal and provenance are server-owned; client claims cannot impersonate Agents/imports.
- Updates require the current record version. Audit history and lifecycle update share one conditional record write, avoiding a missing audit after a crash between separate writes.
- Paginated lists, deterministic ordering, state/type/author/priority and title filters.
- Security submissions are explicitly unavailable until restricted intake is implemented; no false confidentiality promise.
- Superseded closure cannot be manufactured without its checked relationship.

This is **not** a completed Proposal release. Collections for discussion/links/settings are provisioned but their mutation APIs remain pending. Agent adapters, findings promotion, supersession, retry-safe Work creation, browser surfaces, sy, broader intake permissions and end-to-end dogfood remain to implement. Imports remain a later optional checkpoint. The bounded inline history is an initial atomic audit implementation; it caps at 1,000 mutations and needs archival before exceeding that limit.

Pages remains unimplemented. Continue through the approved checkpoints, including naming inventory, explicit build configuration, artifact exporter, Worker prefix serving, promotion/rollback, UI/CLI and final certification. No production deployment or DNS changes in this checkpoint.

Validation: lifecycle/metadata tests and full app regression tests; HTTP fixtures use disposable loopback servers and require execution outside the restricted filesystem sandbox's socket policy. Do not interpret a socket denial as an application regression.

## Discussion, relationships and Work checkpoint — PARTIAL

- Repository graph is one CAS record per repository, allowing transitive supersession-cycle checks across competing processes. Superseded closure is derived from this graph; it does not require two atomic Proposal writes. Graph entries carry actor/time. One replacement per superseded Proposal.
- Discussion create/edit/delete uses Proposal CAS with same-write audit, own-author editing and maintainer deletion. Deleted text is removed; audit preserves its hash instead of exposing removed text.
- Explicit Work creation persists an input-bound operation intent first. Replays repair missing Work/details without duplicate identity; changed replay input conflicts. Multiple independent Work requests preserve Proposal state.
- Canonical list/detail UI, initial relationship controls and `sy proposal` list/view/create/decision/work commands implemented locally. Browser matrix and complete CLI surface still pending.
- New provider review Findings now snapshot the reviewed SHA and execution ID. Proposal creation can attach checked Finding evidence without closing/deleting that Finding; historical unknown SHA remains unknown.
- Tests cover reader discussion versus own-comment moderation, private-repo evidence denial, cycle denial, Work replay/input conflict/partial-write repair, and multiple Work items. Full app regression and vet passed before the final supersession additions; rerun required for checkpoint commit.
- No deployment or push. Agent-generated output adapters, restricted Security intake, intake settings, browser certification and Pages remain pending.

## Intake, Agent entry point and UI/CLI follow-up — PARTIAL

- Added explicit writers/readers intake policy with administrator CAS updates. Default is writers; initialization returns a reload conflict rather than falsely acknowledging a competing policy. Authors may update their text; lifecycle/priority/severity remain maintainer decisions. Permission tests cover reader opt-in, forged principal denial and author/maintainer separation.
- Security Proposals are accepted only inside private repositories. Public Security intake remains unavailable; there is no separately confidential public-repository reporting channel.
- Added explicit bounded `proposer` provider execution and normal Proposal creation with server-owned Agent/execution/source SHA provenance. Real provider dogfood, durable generation recovery and Agent discussion remain pending; this is not certified Agent intake.
- Discussion edit/delete UI, evidence details and author/label/priority/provenance filters added. CLI metadata update/comment/link/links added. Full app and CLI command regressions pass; app vet passes.
- Chromium fixture matrix renders list/detail at 1600/1280/1024/768/430/390. Visual inspection found and fixed insecure-origin `randomUUID` failure. Fixture rendering is not a real backend/API/CLI combined dogfood certification. Browser evidence is temporary at `/tmp/switchyard-proposal-browser`; final campaign evidence must use the real isolated runtime.
- Remaining Proposal work includes settings UI, metadata editing UI, complete supersession behavior, generation recovery/certification, Finding action, Agent participation, durable CAS concurrency tests and real runtime dogfood.

## Pages independent groundwork — PARTIAL / native domain CONDITIONAL

Nick confirmed `switchyard.cx` has not yet been purchased/configured and requested all independent work continue. Domain purchase, authoritative DNS and native TLS certification remain CONDITIONAL; they do not block implementation or local routing tests. No Cloudflare resources were created and no Worker was deployed.

- Added explicit configuration validation for owner/project, source ref, working/build/output directories and public-output acknowledgement for private sources. This model is not yet connected to settings/storage/API.
- Added Worker serving primitive for owner roots, dotted project prefixes and immutable preview identities. Claimed project failures cannot fall through to owner root. Supports conditional requests, HEAD, ranges, scoped index/404/SPA fallback and restricted response headers.
- Manifest hash is required and verified before serving/promotion; manifest entry count/size/path/type are bounded. Promotion validates payload sizes, uses one conditional owner-map write, preserves unrelated mappings and binds retry receipts to the complete target identity.
- Trusted Node exporter rejects traversal, symlinks and nonregular files; hashes payloads and writes the immutable manifest last. No production pointer changes during export. File count/size/path limits apply.
- Local tests cover root/project/preview routing, unsafe paths, ETag/ranges, promotion conflicts, competing promotion acknowledgements, changed replay input, corrupt manifests, exporter containment and upload failure. These tests do not certify live R2, Actions integration, real deployment records, retention or permissions.
- Pages APIs/schema, approved Actions integration, persistent deployment/projection/recovery coordinator, preview publication, UI/CLI, rename reservations, retention and full combined dogfood remain pending. Custom domains remain phase 2.

Deployment: all of these changes remain local; no push, Linode deployment or hostname routing change.

Follow-up to `5c052c4` (app) / `d40c420` (sy): Finding conversion entry point added to PR Findings, with the server continuing to resolve and authorize evidence. Superseded lifecycle updates now return an explicit conflict; metadata/discussion remain independent. Trusted static export anchors traversal to Linux open directory descriptors with `O_NOFOLLOW`, including output ancestors and nested directories. App regressions and Worker/export tests pass after these changes. Native DNS **and TLS** remain conditional on purchase/configuration; broader independent campaign items above remain unfinished.

## Actions integration and product surfaces — local checkpoint in progress

Implemented since `c330ad4` (not pushed or deployed):

- Schema 11 adds permanent mount reservations, versioned explicit site configs and SHA-bound deployment records. Owner-root approval requires owner/org authority; project config requires repo administration. Private source requires explicit public-output acknowledgement and promotion rechecks it.
- Pages uses typed static output on approved Actions jobs. A generated trusted exporter produces a grant-scoped, checksum-bound bundle; the signed Actions control plane verifies successful job/run/source identity before immutable publication. Builds cannot choose destination keys or production mappings.
- Manual build intents freeze exact source SHA; branch completion uses the existing Actions reconciliation path. Deployment identity binds site/run/job/definition. Saved deployment config survives settings edits. Failed/cancelled runs never publish.
- Publication verifies the entire bounded bundle before writes, immutable payload checksums, then a manifest written last. Publication retries repair partial upload and converge. Production remains a separate conditional owner-map pointer.
- Configuration/history/view/build/promote/rollback APIs and Pages project/owner UI; `sy pages` status/deployments/view/configure/deploy/promote/rollback. Explicit generation/version/operation IDs, no unsafe automatic mutation retries.
- Qualified branch and tag refs resolve once, including peeled annotated tags. Raw SHA config is deliberately unsupported. Tag Pages can preserve an existing tag-only Release definition; branch Pages cannot silently broaden that policy.
- Existing activity events record deployment states and promotion/rollback provenance. No automatic artifact deletion is implemented: production, rollback and all previews are conservatively retained.
- Proposal intake and metadata UI, explicit Agent generation UI, and completed-execution recovery with stable Proposal identity and delegated-principal checks. This does not certify provider completion across a crash before execution evidence was saved.

Validation so far: full app Go tests PASS; CLI command tests PASS; Actions typecheck PASS; Pages/export/control-plane Worker tests PASS; 12 Proposal and 12 Pages browser **fixture** checks PASS at 1600/1280/1024/768/430/390. A control-plane Worker fixture traces Action receipt → preview A → production A → B → rollback A with unchanged artifact/SHA and audit actor/action. Fixture evidence lives under `/tmp/switchyard-{proposal,pages}-browser`.

Still required before final acceptance: API permission/concurrent recovery certification, CLI Pages mutation dogfood, generated-export drift check, real isolated-runtime combined flow and broader viewport checks, shared-origin browser certification, naming/base-path/Nift matrix, final build/vet/race/Worker regression and release instructions. Agent discussion support remains pending. Public DNS/TLS remains CONDITIONAL; no public hosting claim, domain purchase, new paid resources, push or deployment.

Additional certification: actual app session issuance test verifies host-only, Secure, HttpOnly cookies. Chromium interception verifies no app session cookie on owner/preview hosts and confirms owner project localStorage is shared. Static responses have no Set-Cookie or Service-Worker-Allowed override and preserve nosniff/MIME/HEAD lengths across HTML, CSS, JS/MJS, JSON, SVG, PNG, JPEG, WebP, WOFF/WOFF2, TXT, XML and WASM. This is explicitly a same-owner content trust boundary, not per-project CSP/storage isolation.

## Final local certification — 2026-10-05

This checkpoint supersedes the pending implementation lists above. Proposal and Pages locally testable product implementation is complete. Public activation remains **CONDITIONAL**. No push, Linode deployment, domain purchase, DNS mutation or paid resource creation occurred.

### Finished product behavior

- Proposal intake settings, metadata, relationships/supersession, discussion/moderation, checked Finding evidence, multiple optional Work links, proposer generation and Agent discussion are implemented. Completed-execution recovery preserves identity and principal without invoking the model again. UI and CLI expose saved proposer defaults and explicit per-request provider/model/personal credential references.
- Pages configuration, deployment list/view, manual build, approved branch completion, immutable publication, preview links, production promotion and rollback are implemented through existing Actions. Site approval history preserves builds completing after settings change. Deployment states are finite; uploading intent precedes remote publication; delayed snapshots cannot regress it. Production is a separate conditional mapping, never a mutable deployment state.
- Exact ref/SHA, run/job/revision, artifact identity, approver and actor provenance are retained. Stable operation identities and CAS protect manual replay, concurrent completions and promotion. Creation/state/promotion events use the existing events store and replay repairs missing audit records.
- Owner root and repository paths select the backing repository explicitly. Prefixes are permanently reserved; dotted/hyphenated/underscored repository names are covered. SPA fallback is explicit in UI/config/CLI and remains inside the output. Project 404s never fall into the owner root. No HTML rewriting, arbitrary raw-SHA configuration or magic repository name.
- Export is descriptor-anchored, bounded and rejects symlinks/nonregular/traversal. Worker publication verifies the bounded bundle before immutable writes and writes the manifest last. All artifacts are retained; no deletion can remove production or rollback targets.

### Evidence and boundaries

`docs/evidence/proposals-pages/` holds final browser JSON and representative screenshots. Reproduction: `scripts/proposals-browser-regression.mjs`, `pages-browser-regression.mjs`, `work-pages-browser-regression.mjs`, `pages-origin-certification.mjs`, `pages-service-worker-certification.mjs` (set the installed Playwright module path when needed).

- Final full app and sy tests/build/vet PASS. Affected app tests PASS with the race detector. Actions 28 Node tests and typecheck PASS; Pages 15 Node tests PASS. Both Worker dry-run builds PASS. Nift app generation PASS.
- Chromium fixture rendering: 12 Proposal list/detail, 12 Pages owner/project, six Work preview checks PASS at 1600/1280/1024/768/430/390. Pages checks include card-internal width, not just document overflow. Mutation controls and direct deployment View routing are exercised against controlled API responses. These are rendering/contract checks, not live cloud browser certification.
- Actual application HTTP handlers plus disposable real bare Git and queue integration trace Proposal acceptance → Work → two deterministic Agent candidates → exact-SHA Action completion → Pages previews → review → selected candidate → real Git integration → production-branch deployment creation. Standalone Work and manual builds bypass Proposal; branch completion bypasses Agents. Proposer model transport is controlled but uses the normal execution substrate; recovery and discussion are verified. Worker control tests separately trace artifact publication → promote A → B → rollback A. This is composed local certification, **not** one live provider/Trestle/R2 production run.
- Naming matrix checks n-ham/strut-labs and foo-bar/foo.js/foo_bar/foo-1.2 across routing, Proposal, Pages, Releases/archive and real Git clone/fetch/push. Existing full suites cover broader product handlers and CLI. The index.html route check found and fixed ServeFile's redirect behavior for application source routes.
- Real app cookie issuance verifies host-only Secure/HttpOnly attributes; browser interception confirms no app auth cookie reaches Pages/preview hosts. Real loopback browser service-worker test denies a project scope of `/` and proves a sibling is uncontrolled. LocalStorage and IndexedDB sharing between owner project paths is explicitly demonstrated. Authored scripts/CSP do **not** isolate same-owner projects. Owner-root scripts remain trusted across that owner origin.

### Remaining release prerequisites and intentional limits

- Native switchyard.cx purchase, authoritative DNS/wildcard routes and TLS: **CONDITIONAL**, not attempted. Public preview URLs remain planned addresses until activation.
- Live signed Actions/Pages transport and real R2 conditional-write semantics, live Trestle metadata recovery, and commercial-provider runtime/quality must be smoke-tested after explicit release authorization. Local substitutes cannot certify their production behavior.
- Actions has an existing restricted repository allowlist. Release must explicitly approve the intended repositories; dry-run success does not widen it. Use existing configured bucket/resources. Follow `workers/pages/ACTIVATION.md` and deploy coherent app/Actions/Pages versions together.
- Generation cannot promise exactly-once provider completion if the process dies before execution evidence is saved. Recover only completed saved execution evidence.
- Conservative retention only; quota/billing/cleanup, imports, custom domains, confidential Security intake on public repos and rename migration are later scope. Bounded Proposal history (1,000), discussion (200), relationship graph (10,000) and site approval history (1,000) require archival before exceeding their limits.

### Release sequence (prepared, not executed)

Review local app and sy commits; rerun environment-specific signed transport smoke; authorize intended Actions repositories; migrate app schema and deploy compatible Actions control/Pages serving/app/CLI; verify local-host owner/project/preview routing and rollback; acquire/configure domain only under separate authorization; verify app host-only cookies and native wildcard TLS before announcing public Pages. Preserve existing production mappings throughout. Never promote from an unverified or failed deployment.

Local commit record: app `dbfcb0e` (implementation, tests, browser evidence); sy `85f7683` (Pages CLI, proposer generation/recovery contract). Both remain unpushed and undeployed. This ledger-only follow-up records their final identities.
