# Proposal and Pages implementation ledger

Accepted 5 October 2026: implement Proposals first, certify them, then Pages. The accepted [feasibility report](proposals-pages-feasibility.md) remains the architecture contract. No pushes authorized. Do not reopen the product decision absent a concrete contradiction.

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
