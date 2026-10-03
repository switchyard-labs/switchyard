# Artifacts event boundary

Current provider events are normalized before queue ingestion. Unknown events,
wrong namespaces, invalid refs/SHAs, oversized bodies and missing/invalid provider
timestamps are rejected. The normalized record includes repository, namespace,
type, provider timestamp, stable identity, classification and available ref
before/after SHAs. No actor is invented: the documented envelopes do not provide
a universal authenticated pusher identity. Commit authors are not push actors.

| Event suffix | Responsibility |
| --- | --- |
| `repo.created`, `repo.imported`, `repo.deleted`, `repo.forked` | Activity and reconciliation hint |
| `repo.pushed` | Ref coordination, CI trigger, reconciliation hint |
| `repo.cloned`, `repo.fetched` | Audit |
| `repo.token.created`, `repo.token.revoked` | Token metadata audit |

Token events retain only ID, scope and expiry. Imports do not retain source URLs
that could contain credentials. Fork events retain safe target repository and
namespace identifiers. Arbitrary payload keys, plaintext tokens and file contents
are excluded. A hint is not an automatic repository ownership grant or deletion.

The provider examples have no universal event ID. With an ID, deduplication uses
account/namespace/ID; otherwise it uses the canonical safe event and provider
timestamp. Delivery subscription identity and delivery time do not affect it.
Persisted receipts fence event identity before projections. Pushes additionally
use the existing repository/ref/before/after transition identity shared with
reconciliation, preserving one domain transition across both paths. Lifecycle
and audit events are durable `events` records, broadcast through the Hub.

The queue consumer acknowledges invalid/unsupported messages, retries persistence
failures and acknowledges successful ingestion. Operators should configure queue
dead-letter retention to investigate rejected messages. Native cloud CI remains
triggered by its separate approved Workflow subscription; lifecycle/audit events
never run CI commands. The HTTP operator hook uses the same normalizer for full
provider envelopes and retains legacy minimal ref-transition compatibility.

Sources reviewed 2026-10-04:
[Artifacts event subscriptions](https://developers.cloudflare.com/artifacts/guides/event-subscriptions/).

Local classification, namespace, secret exclusion and stable-delivery identity
tests cover all nine documented types. Actual subscription delivery beyond
pushes, duplicate delivery and reconciliation bypass certification remain C36
work; code support is not a claim that every event subscription is deployed.
