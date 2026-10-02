# Cloudflare Artifacts — Git plane research and round-trip scaffolding

**Status:** live round trip **NOT executed** — no Cloudflare credentials are
available in the environment or on the Linode. The full contract was extracted
from the official Artifacts docs, and the round-trip Worker + scripts are
staged here and ready to run the moment a token is provided.

## What is required to run the round trip

1. A Cloudflare account on the **Workers Paid plan** (Artifacts is Paid-only).
2. An API token with Artifacts permissions:
   `export CLOUDFLARE_API_TOKEN=...` (and the account id
   `export CLOUDFLARE_ACCOUNT_ID=...`). wrangler is installed on the Linode
   (`wrangler 4.146.0`); `wrangler login` device flow also works.
3. An Artifacts namespace (created automatically on first repo, or via
   `POST /artifacts/namespaces`).
4. A single-node Trestle instance to ingest the bridged event.

## Contract (from developers.cloudflare.com/artifacts, open beta, Oct 2026)

### Interfaces

- **Workers binding** (`env.ARTIFACTS`): `create/get/list/import/delete`;
  disposable repo handle (`using`) with `info`, `createToken`, `listTokens`,
  `revokeToken`, `fork`, `log`, `readCommit`, `readTree`, `readBlob`,
  `readFile`. `create()`/`import()` return `remote` + an initial token.
- **REST API** (Bearer `CLOUDFLARE_API_TOKEN`):
  base `https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/artifacts/namespaces/$NS`;
  `POST repos` (name/description/default_branch/read_only) → `{id, remote, token}`;
  `POST repos/:name/fork`, `POST repos/:name/import {url,branch,depth}`;
  repo content reads: `log`, `commit/:hash`, `tree/:hash`, `blob/:hash`,
  `file?ref=&path=`, `raw/:ref/:path`; tokens: `POST tokens {repo,scope,ttl}`,
  `DELETE tokens/:id`. Standard Cloudflare v4 envelope.
- **Git protocol**: remote `https://<ACCOUNT_ID>.artifacts.cloudflare.net/git/<ns>/<repo>.git`;
  auth via `git -c http.extraHeader="Authorization: Bearer $TOKEN"` or Basic in
  URL (`https://x:<secret>@...`). **Push is protocol v1 only** (no v2
  receive-pack); clone/fetch v1+v2 (v2 `ls-refs`+`fetch`); some optional v1
  capabilities (`filter`, `include-tag`) not supported.

### Events (Queues event subscriptions)

- Account-level (`artifacts`): `repo.created`, `repo.deleted`, `repo.forked`,
  `repo.imported`.
- Repo-level (`artifacts.repo`, namespace+repo filter): **`pushed`** (payload:
  `ref`, `before`, `after`, `commits[]` with id/message/author/committer/
  parents, `totalCommitsCount`), `cloned`, `fetched`, `token.created`,
  `token.revoked`.
- All events: `{type, source:{namespace,repoName}, payload, metadata:
  {accountId, eventSubscriptionId, eventSchemaVersion, eventTimestamp}}`.
- Consumption: subscribe via Cloudflare Queues event subscriptions and consume
  from a Worker; or trigger Workers Workflows/Workers Builds on
  `cf.artifacts.repo.pushed` (already used by Cloudflare for CI).

### Limits and pricing (relevant for the control plane + agent scale)

- 2000 control-plane req / 10 s / namespace; 2000 git req / 10 s / repo.
- Max 1 GB per repo; max 32 MB per file/blob; 1 TB per account (raiseable).
- Repos and namespaces unlimited; naming: 2-63 chars, `[A-Za-z0-9._-]`.
- Pricing: Workers Paid only. First 10k operations/mo free, then $0.15/1k;
  first 1 GB-mo free, then $0.50/GB-mo. **Billing begins 2026-10-14.**
- Jurisdiction: namespace-scoped `us`/`eu` (set at namespace creation; immutable).

### Notes vs the competition material

- The competition blog ("next Git platform on Cloudflare") is consistent with
  the current docs; the only delta is that event subscriptions are delivered
  via Queues and the `pushed` event schema matches the docs exactly.
- Workers Builds integration supports only `main` as the production branch
  (branch previews for others) — relevant if Switchyard ever uses it for
  agent-run deploys.
- Artifacts is Durable-Object-backed; repos are isolated units (one repo per
  agent/task/session is the documented pattern).

## Authority boundary (from the architecture review)

- **Artifacts owns Git truth.** Repos, forks, refs, tokens, push/clone events.
- **Trestle owns Switchyard coordination truth** (tasks, runs, provenance,
  integration state, jobs, audit).
- The **Artifacts→Trestle bridge is at-least-once with reconciliation** — it is
  not a distributed transaction. Git truth and coordination truth are two
  stores joined by events, not one transaction.
- Workers are edge glue: mint tokens, forward `pushed` events to Trestle, host
  the static dashboard.

## Staged round-trip (runs when a token is available)

The intended end-to-end slice (see `roundtrip/` for staged code):

1. Baseline repo in an Artifacts namespace (create or `import` from GitHub).
2. Fork it as an "agent task" repo (`POST .../fork`, `default_branch_only`).
3. Mint a short-lived write token (`POST .../tokens {scope:"write", ttl}`).
4. Clone with an ordinary `git` client using the token.
5. Commit + push.
6. Observe `cf.artifacts.repo.pushed` via a Queues event subscription; a
   Worker normalizes the event into Trestle's ingestion shape.
7. Ingest into single-node Trestle as a task/run transition (research code
   only — not the beginning of the Switchyard API).

`roundtrip/worker/` contains a minimal Worker (wrangler config + handler) that
mints tokens and (later) forwards events; `roundtrip/scripts/` contains the
git round-trip steps.

## Credentials blocker

No `CLOUDFLARE_API_TOKEN` / wrangler auth exists on the Linode or the operator
machine. The single required input to finish E and G is Cloudflare access on a
Workers Paid account with an Artifacts namespace. `wrangler login` device flow
or an exported API token both work. This is the only hard blocker to Checkpoint
0 passing unconditionally.