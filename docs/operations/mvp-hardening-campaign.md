# Bounded MVP hardening campaign

This campaign follows the approved F1–F14 review. No tags, releases or normal
pushes are authorized until final reconciliation. Checkpoints are logical commit
boundaries, not additional approval gates.

## CP1: security isolation

Fleet repository and queue reads are scoped by requested organisation and current
repository permissions. Legacy workflow/escalation repository associations live
inside JSON; those lists are filtered before deriving counts. Their global read
cost remains a timing/workload limitation, not a constant-time isolation claim.
Every mutating API request passes the common origin boundary before session lookup.
Missing Origin remains supported for CLI/API clients; explicit foreign and sibling
Pages origins are rejected. HTTPS production uses `__Host-switchyard_session`
(Secure, HttpOnly, Path=/, no Domain). Legacy-name cookies are not accepted in
production, so existing browser sessions need a fresh login at rollout. Plain HTTP
development uses the separate legacy-name cookie.

Validation: full application tests and focused tenant/origin regressions passed.
These are local fixture tests, not a live deployment certification.

## CP2: immutable source intent

Queue schema version 14 adds optional `source_sha`, `enqueue_base_sha`,
`approved_by`, and `approval_kind` text fields. New enqueue requests explicitly
name the source SHA viewed by the caller. The queue persists that SHA and the
principal's source-enqueue decision. This decision is not a claim of an independent
review approval; model findings remain review evidence, not approval decisions.
Configured checks and integration policy apply to the pinned source and prepared
merged candidate. No additional mandatory-review feature is introduced.

Source movement blocks the old queue item and requires a fresh enqueue for the
new source. Base movement permits fresh preparation/validation of the same source.
Legacy unpinned intents are blocked, never backfilled from the current branch tip.
Deterministic review reads at a pinned SHA and records that identity in findings
and execution metadata; model review already pins its source inputs.

Before live schema mutation, inventory active/legacy operations and schemas, stop
application workers/requests, take a consistent coordination/local-scratch backup,
retain the prior binary/key/config, run `-schema-plan`, apply only reviewed additive
fields with `-schema-migrate`, then verify data/ledger and restart. A failure may
leave additive fields; do not delete fields or invent identities to roll back.
An older binary refuses the newer ledger: rollback requires the retained compatible
state/binary or a verified restore of the exclusive pre-migration snapshot.

Publication recovery and final rollout remain subsequent checkpoints. No live
schema mutation or deployment has been performed by these checkpoints yet.
