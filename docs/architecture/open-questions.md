# Open questions and decision classification

The intent of this document is to stop free-form brainstorming and make the
remaining decisions explicit and tractable. Free-form architecture discussion
has reached diminishing returns; the next step is CP1 (synthesis + verification +
bounded independent review), not more unbounded design.

## Classification legend

- **Settled enough to synthesize** — further brainstorming has diminishing
  returns; adopt as axioms.
- **Needs independent review** — a second architecture proposal is genuinely
  useful (Codex threads).
- **Must be proven experimentally** — should not be decided by prose.
- **Deferred to implementation** — no architectural resolution needed now.

## Settled enough to synthesize

- Work + Attempt + Pull Request domain model, with implicit Work auto-creation.
- Save ≠ Commit; recoverable transient draft; no staging/index in the web UI.
- One shared diff/review surface across editor/PR/findings/conflict.
- CodeMirror 6 (provisional; stop bikeshedding).
- Agent Role / Provider Profile / Principal / Execution / Worker distinct; only
  Role and Execution prominent.
- Pluggable runner adapter under one substrate; v1 ships one adapter.
- Ref-mutation substrate: per-ref CAS + per-ref serialization policy +
  Integration Queue (high-level canonical gate).
- Queue taxonomy: execution queue / ref-mutation primitive / Integration Queue.
- SecretStore interface contract; backend deferred.
- Personal + org credentials for v1; roles + policy express repo boundaries.
- Provenance at pragmatic file/commit granularity (no line-level authorship).
- Draft contention is ephemeral editor state, distinct from Git/semantic/intent/
  policy conflicts.
- Terminology constraint; progressive complexity; product invariants (dark,
  favicon, `Switchyard` title, hamburger).

## Needs independent Codex review (bounded threads)

- **T1 — ref-update primitive / Artifacts contention semantics:** the exact
  contract of `UpdateRef` and whether/how Artifacts' Git protocol exposes
  expected-old/CAS under contention; the reconciliation contract.
- **T2 — Agent runner adapter contract:** a second opinion on the substrate
  interface (context in, events out, cancellation, budget, output
  classification) before it hardens.
- **T3 — interactive-draft → Attempt takeover semantics:** checkpoint-commit-
  before-Attempt vs initial-working-tree; attribution of the initial state;
  implicit Work creation.
- **T4 — role configuration precedence + repo→org-secret resolution:** how repo
  config selects roles whose credentials live in org/user stores without
  leakage; precedence between built-in/personal/repo/org layers.

## Must be proven experimentally (see plan/verification.md)

- **E1 — Artifacts per-ref contention/CAS** (stale non-force push rejects cleanly).
- **E2 — draft recovery/sync** (measure real latency; bounded recent-edit loss;
  multi-device).
- **E3 — three-way merge** for human/Agent stale edits (correct vs surface
  contention).
- **E4 — secret resolution → executor path** (latency, expiry, child
  inheritance, no leakage).

## Deferred to implementation

- Draft blob storage backend (R2 vs Artifacts blob namespace); TTL/GC.
- Concrete SecretStore backend.
- CodeMirror extension/diff-gutter composition; mobile editor pane details.
- Findings-in-editor gutter mechanics; per-file vs per-hunk revert.
- Agent role configuration syntax and precedence encoding.
- GitHub/GitLab interop depth.
- Nift island composition details for realtime panels.

## Materially blocking the plan?

Nothing in the deferred set blocks CP1. The blocking items are the CP1
synthesis itself and the experimental gates E1–E4 (and, for the clustered
future, the Trestle clustering fix — see `plan/dependencies.md`).