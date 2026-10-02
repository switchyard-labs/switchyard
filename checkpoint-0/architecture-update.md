# Phase-1 architecture, revised from Checkpoint 0 evidence

This updates the earlier architecture review with what Checkpoint 0 actually
established. It is still design — no Switchyard product code exists yet.

## Confirmed boundary (kept from the architecture review)

- **Artifacts owns Git truth.** Repos (one per task/agent/session), forks,
  repo-scoped tokens, and `pushed`/`cloned`/`forked` events.
- **Trestle owns Switchyard coordination truth.** Users, service accounts,
  tasks, runs, provenance, integration state, durable coordination jobs, audit.
  Single-node in the earliest slice.
- **Strut workers execute bounded domain/workflow steps.** They are disposable
  step-executors, not a long-lived in-memory workflow authority. Workflow state
  survives worker death because it lives in Trestle.
- **Workers are edge glue**: mint Artifacts tokens, forward Artifacts events
  into Trestle, host the static Nift-built dashboard, and terminate the
  browser realtime bridge.
- **Nift builds the dashboard/docs** (runtime-free output).
- **The Artifacts↔Trestle bridge is at-least-once with reconciliation** — not a
  distributed transaction.

## What Checkpoint 0 changed

### Trestle

- The clustered side-effect gap is **confirmed** and becomes a gating
  dependency for the clustered phase (see `trestle-clustering-repro/`).
  **Phase 1 uses single-node Trestle by design.** Clustering is not deferred
  forever — it is a separately planned Trestle development campaign that
  Switchyard is intended to dogfood.
- Cluster formation today requires raft ServerID == Gantry `node_id`
  (undocumented; found during reproduction). Not needed for single-node phase
  1, but required before any clustered work.

### Strut

- **Step-executor viability is confirmed at main** (`c4356bea`): HTTPS, SSE
  streaming, JSON, processes/pipes, git CLI, concurrency, filesystem,
  crypto/HMAC, cancellation, child-failure recovery, graceful shutdown all
  pass. No hard blocker.
- The engine must not rely on the v0.0.3 release; it depends on a future
  release that includes the current-main surface (HTTP streaming/SSE,
  cancellable processes/pipes, crypto, cancellation). Release-gate per the
  agreed Strut process.
- Architecture still avoids Strut's weak spots by design: no HTTP server
  breadth (workers are clients), no outbound WS (Trestle is SSE), no typed
  SQLite rows (JSON bookkeeping), no long-lived actor/async loops
  (state-in-Trestle step execution).

### Cloudflare

- Round-trip scaffolding is staged but **not executed** (credential gap).
  Nothing about the authority boundary changed; the docs confirm the planned
  seams (tokens are repo-scoped, events are structured, push is v1-only,
  limits are generous for agent scale).

### Realtime

- Browser realtime = Worker/DO SSE bridge to Trestle (see
  `realtime-decision.md`); Trestle user-SSE is a candidate Trestle dev item.

## Smallest Checkpoint 1 slice (unchanged, re-validated)

One agent, one task, one integration, all real:

```
Nift dashboard+docs ──► static (Workers/R2)
Browser ──► Workers API ──► Artifacts (fork/mint token) + push events ──► Trestle
Strut step-executor (Linode) ── spawns agent (git+LLM CLI), streams,
   on push ──► review step ──► policy ──► merge in Artifacts ──► provenance
```

No issues, PRs, orgs, review UI, or GitHub replacement in this phase.

## Risks carried forward

1. Trestle clustered side effects (blocking for clustered phase; single-node OK).
2. Strut release gate (main-only surface must ship and stabilize).
3. Browser realtime bridge auth hand-off (unresolved detail).
4. Cloudflare live round trip unexecuted (credential gap).
5. `backend_baseline_certification.py` flake on 1-vCPU (reproduce before
   trusting CI-signal on constrained hardware).