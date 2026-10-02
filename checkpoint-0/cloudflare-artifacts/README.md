# Cloudflare Artifacts — Git plane research and round-trip scaffolding

**Status:** live round trip **EXECUTED**. The full Git-plane loop was exercised
against a real account on the Workers Paid plan: repo create → push → REST
reads → fork → scoped tokens → git clone/commit/push → **`pushed` event
delivered via a Queues event subscription** → normalized → ingested into a
single-node Trestle instance with **idempotent re-delivery**. See the
"Live validation" section and `evidence-live/`. The account-eligibility blocker
was resolved by the operator upgrading to Workers Paid.

## Live validation (executed 2026-10-02, after plan upgrade)

Driven from the Linode with the official `cf` CLI (`npm install -g cf`) and the
REST API (OAuth token). Evidence outputs are in `evidence-live/`.

### What actually worked (observed behavior, not docs)

| Step | Result |
| --- | --- |
| Namespace create | `switchyard-cp0`, `unrestricted` jurisdiction, created instantly |
| Repo create | `POST /repos {name, default_branch}` → `{id, remote, token}`, token `art_v2_x_...?expires=...` |
| Import (public GitHub repo) | `octocat/Hello-World` imported → `default_branch: main`, `source` recorded; ready within seconds |
| Push content | `git init + remote + push` and `git clone + push` both work; ordinary git client, protocol v2 upload-pack / v1 receive-pack |
| REST reads | `log`, `commit/:hash`, `tree/:hash`, `file?ref&path`, `raw/:ref/:path` all return the expected objects/bytes |
| Fork | `POST /repos/:name/fork {default_branch_only}` → returns `objects` count; measured latency **~1.9–2.2 s** |
| Scoped tokens | `POST /tokens {scope, ttl}` → read and write tokens; `GET /repos/:name/tokens` lists them with state/expiry |
| Git UX | clone/fetch/push with `http.extraHeader="Authorization: Bearer $TOKEN"` work out of the box; read token clones, write token pushes |
| **Event delivery** | `pushed` event delivered to a Queues event subscription and observed via queue `peek`; payload exactly matches the documented schema (before/after hashes matched the real push) |
| **Trestle ingest** | normalized event inserted into single-node Trestle (port 7340) as a record; `_trestle_events` and `_trestle_audit` both = 1 (the standalone contrast to the clustered finding) |
| **Idempotent re-delivery** | re-POSTing the same event with the same `Idempotency-Key` returned the same record (version 1, no duplicate) |
| Failure modes | bad token → git clone `403`; nonexistent repo → `Repository not found`; nonexistent ref/path → `File not found`; bad repo name → regex validation error; delete then get → `Repository not found` |

### Findings and rough edges (observed, beyond the docs)

1. **Account eligibility was the real gate.** Before the operator's plan
   upgrade: zero subscriptions, zero entitlements, `10004 Access denied` on
   every Artifacts call — with a token that already had `artifacts.read/write`
   scopes. Token scopes are necessary but not sufficient.
2. **The `cf` CLI's event-subscription source enum does not include
   `artifacts`/`artifacts.repo`** (`cf queues subscriptions create --source-type`
   lists only images/kv/r2/vectorize/workersAi/workersBuilds/workers.script/
   workflows). The subscription had to be created via the REST API
   (`POST /accounts/{id}/event_subscriptions/subscriptions`), which accepts
   `source.type = "artifacts.repo"`.
3. **The subscription `events` list uses short names** (`"pushed"`), not the
   delivered `type` string (`cf.artifacts.repo.pushed`). The docs' examples
   show only the delivered event's `type`; sending `cf.artifacts.repo.pushed`
   as the subscription event fails with "Unrecognized event types".
4. **Repo-level `pushed` events require a specific `repo_name`** in the
   subscription source (`source.repo_name` is a required field). There is no
   namespace-wide `pushed` subscription; subscribe per repo (or use the
   account-level `artifacts` source for created/deleted/forked/imported).
5. **HTTP pull consumers require an explicit enable step.** Creating an
   `http_pull` consumer (`cf queues consumers create --type http_pull`) does
   NOT enable pull mode; the docs require `wrangler queues consumer http add`
   or the dashboard. `cf` has no such subcommand yet. Push-based Worker
   consumers are the default/recommended path and need no extra enablement.
   Queue pull/ack lease semantics work as documented once enabled.
6. **Occasional transient git clone failures under bursty scripted activity.**
   Several scripted runs saw `git clone` fail silently on the first attempt
   (and a naive retry reusing the target dir compounded it — git leaves a
   partial dir). Isolated clones (15+ in a row, including `-q`) all succeeded
   in ~300–500 ms. Root cause not isolated; treat as a transient to retry with
   a fresh target dir. Worth watching under real agent traffic.
7. **Edge Workers cannot reach a self-hosted Trestle on loopback.** The staged
   Worker's `/ingest` points at `127.0.0.1:7333` on the Linode; from the
   Cloudflare edge that is unreachable. The bridge therefore ran as
   **control-plane pull** (queue → Linode → Trestle), which is also the more
   robust shape: the edge writes durable state (queue/R2), and the control
   plane pulls and ingests with idempotency. If a direct edge→Trestle push is
   ever desired, Trestle must be reachable (reverse proxy / tunnel), not on
   loopback.
8. **`git clone` of a freshly created (empty) repo** prints the standard
   "empty repository" warning; `git init + remote add + push` is the reliable
   bootstrap path (the initial token from `POST /repos` is a write token).

### Opinion after dogfooding: does "Artifacts = Git truth" still hold?

**Yes.** Artifacts behaved as a faithful Git server for the whole round trip:
objects/refs/commits were exactly what a normal `git` client wrote, REST reads
returned the same bytes, forks copied object sets, and the push event carried
the exact before/after SHAs. Nothing observed suggests we should hold Git state
anywhere else. The at-least-once assumption for the Artifacts → Worker →
Trestle bridge is also supported: messages persist in the queue until acked
(redelivery possible), and the Trestle ingest boundary dedupes via
`Idempotency-Key` (verified). Two planning consequences, not architecture
changes: (a) per-repo `pushed` subscriptions mean the bridge must either create
a subscription per task repo or use the account-level source plus its own
filtering; (b) the bridge should be pull-based (queue → control plane) given a
self-hosted control plane, or a Worker consumer writing to durable storage.

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

## Live observations (account-level, 2026-10-02) — historical, pre-upgrade

Recorded **before** the operator upgraded the account to Workers Paid. Kept as
evidence of the eligibility diagnosis. After the upgrade, every item below
flipped to working (see Live validation).

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

## Credentials and account state (resolved)

- Authentication: `cf` OAuth device flow; token stored on the Linode at
  `/root/.config/cloudflare/config/default.json` (0600). No credentials are in
  this repository.
- Account: `b7f20353ee8a9e5d2003f52c74ba795e`, **Workers Paid** as of
  2026-10-02 (operator action). Artifacts namespace `switchyard-cp0`.
- The API token in `~/.bashrc` still lacks Artifacts permissions; use `cf`'s
  OAuth profile on the Linode (`env -u CLOUDFLARE_API_TOKEN cf ...`).