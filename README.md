# Switchyard

Switchyard coordinates people and coding Agents around ordinary Git. A Work item can have several competing Attempts; each preserves its own branch, execution history, checks and review evidence. An Integration Queue validates one immutable candidate before publishing it to the canonical repository.

Cloudflare Artifacts owns Git. Trestle stores coordination. Native Artifacts pushes start a Cloudflare Worker/Workflow using the official CI SDK and Sandbox; Actions bind results and read-only logs to an exact commit and approved JavaScript definition. Durable engineering workflows coordinate the larger task separately from CI.

This repository contains the Go control plane, Nift-built browser application and `workers/actions` CI Worker. The current implementation campaign is local and unfinished: the complete visual/editor redesign and final dogfood certification are still pending. Standard Workers Builds previews/deployments need a verified connection and build token. The default deterministic Agent adapter is useful for repeatable scenarios; external coding-agent execution is a separate bounded runner capability and is not automatically enabled at startup.

## Run locally

Prerequisites: Go 1.26, Git, a configured single-node Trestle service/admin account, a Cloudflare Artifacts account/namespace and an operator-controlled token helper. Nift rebuilds HTML from `content`/`templates`; checked-in `public` can be served directly. Node 22 or later is used for the Worker tooling.

Provide these settings through your private local environment or service manager:

| Setting | Purpose |
| --- | --- |
| `SWITCHYARD_TRESTLE_URL` | Trestle origin, typically `http://127.0.0.1:7350` |
| `SWITCHYARD_TRESTLE_USER` / `SWITCHYARD_TRESTLE_PASS` | Existing Trestle admin credentials |
| `SWITCHYARD_ARTIFACTS_ACCOUNT` / `SWITCHYARD_ARTIFACTS_NAMESPACE` | Your Cloudflare account and repository namespace |
| `SWITCHYARD_ARTIFACTS_TOKEN_CMD` | Private executable helper returning an account API token on stdout |
| `SWITCHYARD_DATA_DIR` | Private durable application directory outside source control |
| `SWITCHYARD_STATIC_DIR` | Absolute path to this checkout's `public` directory |

Inspect existing schema compatibility first:

```sh
go run ./cmd/switchyard --schema-plan
```

If the plan requires an additive migration, back up Trestle and apply `--schema-migrate` during an exclusive maintenance window. Do not run competing schema writers. Startup provisions a fresh database but rejects incompatible existing schemas.

```sh
go run ./cmd/switchyard --listen 127.0.0.1:8080
```

Open `http://127.0.0.1:8080`, create a local application account and import/register repositories through the authorized operator workflow. No production demo data is required to start. Keep Trestle private and put authenticated TLS termination in front of any non-local deployment. Preserve the generated credential key together with encrypted application state when backing up.

For CI, follow [Cloudflare Actions architecture](docs/architecture/cloudflare-actions.md). The checked-in Wrangler configuration identifies disposable campaign resources; copy and adapt it deliberately for your own account. Provide encrypted Worker secrets outside source control. Configure `SWITCHYARD_ACTIONS_WORKER` and `SWITCHYARD_ACTIONS_CONTROL_KEY_FILE` only after the Worker is deployed and its dedicated key is stored in an existing private regular file. Repository administrators approve definitions through Actions; repository writes do not implicitly approve new CI commands.

## Build and verify

```sh
nift build
go test -race ./...
go vet ./...
node --test scripts/action-ansi.test.mjs
npm ci --prefix workers/actions
npm run typecheck --prefix workers/actions
npm test --prefix workers/actions
```

Browser regression stages live in `scripts/cua-actions-smoke-stages.mjs` and run through the documented Codex browser interface against an actual configured instance. A script file existing is not browser certification. [Campaign evidence](docs/plan/codex-implementation-campaign.md) records what has actually passed and what remains pending.

See [architecture](docs/architecture/README.md), [CI boundaries/evidence](docs/evidence/campaign/C15/README.md) and [competition alignment](docs/plan/cloudflare-competition-alignment.md). Historical Checkpoint 0 material records earlier experiments and dependencies; it does not describe the current runtime. Licensed under [MIT](LICENSE).

## MVP hardening boundaries

Application Agent publication is restricted to permitted candidate/Attempt
branches. Canonical/protected application writes use the separately validated
Integration Queue boundary; intentional direct human writes remain available
where policy permits. Independently-issued Git credentials are outside this
application boundary. Queue enqueue persists the exact source SHA and principal;
base movement revalidates the same source, while source movement requires a new
intent. Reviews record source-bound evidence, not independent approval decisions.

Deterministic semantic checks currently support bounded JSON `field_equals`
contracts only. ZIP/tar.gz branch/tag snapshots resolve their ref at request time;
release links use full SHAs. Submodules and `.gitattributes` export rules are not
supported. Actions control requests are HMAC-authenticated; HTTPS Worker responses
are trusted transport artifacts, not independently signed snapshots.

Real model-backed implementation, review and conflict-resolution runs were
exercised on isolated local workers. The current Linode deployment serves the
control plane and is not sized for routine Agent execution. These runs do not
establish production throughput or distributed-worker scaling. Publication replay
requires retained local candidate scratch; missing scratch fails explicitly with
bounded queue retries. Periodic ref polling cannot reconstruct intermediate pushes.
Pages projects share an owner origin; the host-only `__Host-` authentication cookie
protects Switchyard specifically, not arbitrary parent-domain cookies. Raw `sy api`
HTTP 401 currently exits 1. See `docs/operations/mvp-hardening-campaign.md` for
checkpoint evidence and rollout limitations.
