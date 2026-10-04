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
