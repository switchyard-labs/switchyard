# C39: sy API handoff

Implementation contract at C38 (`f079eeb`), 2026-10-04. Cookie authentication
(`switchyard_session`) remains supported; PAT/device login is not implemented.
Do not require a user to supply an Artifacts account token.

## Identity and scoped clone

`GET /api/repositories` returns canonical registered repositories. Use their
`owner_slug`, `name`, `full_name`, `artifact_name`, `visibility` and `id` fields;
never derive public visibility from the provider's `read_only` flag. Canonical
browser route is `/{owner}/{repo}`. Physical Artifacts namespace/name need not
match Switchyard owner/name. Escape each path component.

`POST /api/repositories/{owner}/{repo}/git-credential`
body `{"scope":"read","ttl_seconds":600}`. TTL 60–900 seconds, default600;
scope read/write. Response200: `{remote,token,expires_at,permissions,repository}`.
Token is opaque and includes provider expiry; use it intact as Bearer auth.
Authentication required even for a public repository. No-store; credential
returned once, never stored by Switchyard. Audit contains metadata only.
Read requires repository access; write requires write permission and is rejected
for archived/read-only repos. Repository-wide direct write credentials are
blocked409 when protected refs exist. Git token must never be in argv, origin
URL, `.git/config`, CLI JSON output or logs. Limit its HTTP header to the exact
HTTPS remote; reject redirects, malformed remotes and control characters.

## Actions

Root: `/api/repositories/{owner}/{repo}/actions`.

| Method/path | Request | Response |
|---|---|---|
| GET root | none | `{items,settings,enabled,can_run,can_admin,deployments_enabled}` |
| GET `/{id}` | none | `{run,jobs,checks,enabled,can_run,can_admin,deployments_enabled}` |
| POST `/dispatch` | `{request_id,sha,ref,pr_id?}` |202 `{id,sha,definition_revision}` |
| POST `/{id}/rerun` | `{request_id,mode:"all" or "failed"}` |202 same |
| POST `/{id}/cancel` | `{}` |202 `{id,status:"cancellation_requested"}` |
| GET `/{id}/logs` | query `step=jobID-stepID&cursor=0` | `{lines:[{id,stream,text}],cursor,complete,truncated,kind,sha}` |
| GET `/{id}/logs/stream` | same step/cursor or Last-Event-ID | SSE below |

Run fields: `id,repo,source_sha,ref,definition_revision,trigger,actor,created_at,
state`. `state.status`: queued/running/success/failure/cancelled/timed_out.
`state.manifest` contains immutable run definition, jobs/steps, timestamps and
optional Worker source inspection. Missing source inspection does not mean
verified. Jobs use provider status succeeded/failed (normalize for display).

Dispatch requires exact lower-case40hex SHA and full `refs/heads/...` or
`refs/tags/...`; optional PR resolves its current source and rejects changed SHA.
Request IDs16–64 alphanumeric/underscore/hyphen; preserve across an ambiguous
retry, never blindly auto-retry a mutation. Rerun requires terminal state and
current approved definition; failed mode reuses passing exact-source jobs.
Controls require repository write access. Definition approval requires admin.

Logs are bounded captured output, not an interactive shell/live process stream.
JSON pages ≤100 lines/64KiB; stable cursor max10000. Pending capture202
`{error:"logs_pending",pending:true}`; terminal missing capture is an error.
SSE events: capture(metadata), line(id/data line object), complete(cursor/kind/
truncated), log_error(error). Stream waits at most2min and ends after captured
output; reconnect with cursor. Completion means capture drained, not necessarily
whole run terminal. `sy actions logs --follow` must poll run status and other
steps until terminal, then drain each available capture; Ctrl-C cancels following
only. No cancel POST on signal. Download query`download=1` returns text attachment.
stdout and stderr are grouped; interleaved process ordering is not claimed.

## Attempts and errors

No top-level GET `/api/attempts` or GET `/api/attempts/{id}` exists at this
checkpoint. Work detail `/api/work/{id}` has authorized `attempts[]`. Existing
bounded derived listing remains necessary and should expose scan limits.
POST `/api/attempts/{id}/run` and `/pr` remain available. Deterministic execution
is not a certified real external coding provider.

Server errors may be `{error:"code"}` or legacy structured error envelopes.
Preserve status/code and Retry-After. Upstream429 is HTTP429
`upstream_rate_limited`; real404 remains repository_not_found. Upstream401/403
become502 authentication failure;408/504→504;503→503; others→502. Retry only safe
GET/HEAD with bounded, context-cancellable backoff honoring Retry-After. No
unsafe automatic mutation retry. Do not reinterpret404 as rate limiting.

## Cloudflare deployment qualification

Generic Actions use real Artifacts native CI triggers → Worker immutable binding
inspection → Workflows/@cloudflare/ci Sandbox → R2 manifests/logs. Parallel
Artifacts subscription → Queue → Go/Trestle/SSE is certified in C36. Required
checks authorize only exact commit and current definition. Worker Builds/Preview
setup remains conditional; deployments_enabled=false. No fake preview URL or
SSH option. See C36 evidence and Cloudflare setup documentation.
