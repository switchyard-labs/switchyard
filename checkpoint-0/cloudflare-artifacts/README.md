# Cloudflare Artifacts — Git plane research and round-trip scaffolding

**Status:** live round trip **still NOT executed**. Authentication is now fully
resolved (OAuth device flow via the official `cf` CLI; the OAuth token carries
`artifacts.read`/`artifacts.write`), but the account is **not eligible** for
Artifacts: it has zero subscriptions and zero entitlements (Workers Free / not
enrolled). The blocker is now precisely characterized in "Live observations"
below. The full contract was extracted from the docs, and the round-trip
Worker + scripts are staged and ready to run the moment the account is eligible.

## What is required to run the round trip

1. A Cloudflare account on the **Workers Paid plan** (Artifacts is Paid-only).
   **This is the current blocker** — see Live observations.
2. Artifacts access for the account (open beta; confirmed scoped token or OAuth
   with `artifacts.read`/`artifacts.write`).
3. An Artifacts namespace (created automatically on first repo, or via
   `POST /artifacts/namespaces`).
4. A single-node Trestle instance to ingest the bridged event.

## Authentication state (resolved)

- The operator's `CLOUDFLARE_API_TOKEN` from `~/.bashrc` and the initial OAuth
  attempts both fail Artifacts with `403 [10004] Access denied`.
- The official Cloudflare **`cf` CLI** (`npm install -g cf`, v1.0.0-beta.11) was
  used instead of wrangler. `cf auth login` uses **OAuth device authorization**
  by default (`--device`), so there is **no localhost callback** and no SSH
  forward is required. Token verification shows the granted OAuth scopes
  include `artifacts.read` and `artifacts.write`.
- OAuth token is stored on the Linode at
  `/root/.config/cloudflare/config/default.json` (0600). No credentials are in
  this repository.

## Live observations (account-level, 2026-10-02)

Facts learned by actually talking to the account, distinct from the docs:

| Observation | Detail |
| --- | --- |
| Account | `b7f20353ee8a9e5d2003f52c74ba795e` ("Nicholas.charles.ham@gmail.com's Account") |
| Token scopes OK | OAuth token has `artifacts.read`, `artifacts.write`, `workers` scopes |
| Artifacts namespace list | `403 Forbidden [10004] Access denied` on `GET /accounts/{id}/artifacts/namespaces` |
| Account subscriptions | `GET /accounts/{id}/subscriptions` → **empty** (no paid subscriptions) |
| Account entitlements | `GET /accounts/{id}/entitlements` → **0 entitlements** |
| Workers subscription route | `GET /accounts/{id}/workers/subscription` → `7003 Could not route …` (no subscription) |
| `cf` tooling | `cf artifacts namespaces list` → same `10004 Access denied` |

**Conclusion:** the account is on the Workers **Free** tier and has no Artifacts
entitlement. Artifacts is documented as **Workers Paid only**. The 403 is an
account-eligibility failure, not a token-scope failure. The fix is an account
billing/enrollment action by the owner (upgrade to Workers Paid and confirm
Artifacts open-beta access), which is out of scope for this phase and should
not be performed silently.

**Not exercised yet (blocked by eligibility):** repo create/import/fork,
token minting, git clone/push, REST reads, `pushed` event delivery, the staged
Worker bridge, Trestle ingestion. The staged `roundtrip/` code is expected to
run unchanged once the account is eligible.

**Implication for Switchyard:** no architecture change is warranted from this
finding — the authority boundary (Artifacts = Git truth) does not depend on
which Cloudflare plan hosts the account. It does add an **operational
prerequisite**: a Workers Paid account with Artifacts access is required before
any live Git-plane validation. Budget note: billing for Artifacts operations
begins 2026-10-14.

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