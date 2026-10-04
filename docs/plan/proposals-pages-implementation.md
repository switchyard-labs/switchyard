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
