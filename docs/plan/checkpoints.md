# Master checkpoint gameplan

Living checklist, not a waterfall. Gates at the end of every checkpoint:
**PASS / CONDITIONAL PASS / FAIL**, with evidence. Favour vertical slices.

## CP0 — Feasibility / substrate validation

- **Status: PASS** (completed, historical).
- Evidence: [`checkpoint-0/`](../../checkpoint-0/README.md) — Linode environment,
  Trestle clustering defect reproduction, Strut surface validation, Cloudflare
  Artifacts live round trip + event bridge.
- Do not renumber or rewrite.

## CP1 — Architecture synthesis + contract verification

- **Status: PASS** (handover: `handovers/CP01.md`; evidence: `experiments/cp1/`).

**Objective:** turn the design handover into a durable architecture and close the
highest-risk assumptions before building. **No product implementation.**

**Prerequisites:** CP0 evidence; this docs tree.

**Tasks:**
1. Consolidate architecture/planning docs (this tree); reconcile contradictions.
2. Four **independent Codex review threads**, each producing a bounded written
   contract/recommendation, not code:
   - T1 ref-update primitive / Artifacts contention semantics;
   - T2 Agent runner adapter contract;
   - T3 interactive-draft → Attempt takeover semantics;
   - T4 role configuration precedence + repo→org-secret resolution.
3. **Experimental verification** (each may be a sub-checkpoint with its own
   gate if that yields cleaner review):
   - E1 Artifacts per-ref contention/CAS behaviour;
   - E2 draft recovery/sync prototype;
   - E3 three-way merge for human/Agent stale edits;
   - E4 secret resolution → executor path.

**Expected artifacts:** finalised architecture docs; four Codex contract
documents; four experiment reports; updated `open-questions.md`.

**Tests/evidence:** every experiment has hypothesis + measurement + decision +
cleanup rule.

**Review gate:** architecture and the four contracts reviewed collectively
(ChatGPT + DeepSeek + Codex); experiments recorded. **CONDITIONAL PASS** is
acceptable with bounded items.

**Stop conditions:** unresolved E1 (Artifacts CAS) or E4 (secret delivery)
contradicts a foundational assumption → stop and re-plan that layer.

**Non-goals:** no product endpoints, no UI, no schema migrations for product.

## CP2 — Control-plane foundation

- **Status: PASS** (handover: `handovers/CP02.md`; gate: `experiments/cp2/cp2_gate.sh`).

**Objective:** authentication (users/orgs), Artifacts repository integration,
repo browser + read surfaces, and event ingestion/reconciliation on single-node
Trestle.

**Tasks:** app skeleton (Nift-built shell + islands); auth/users/orgs; repo
registry backed by Artifacts; repo browser (files/commits/diffs read-only);
Artifacts event subscription → Trestle ingestion; reconciliation job;
Work items (create/list/status).

**Gate:** a real repo is browsable; a real `git push` lands as a normalized
Trestle event and reconciles; a Work item can be created and viewed. No secrets
leaked in logs/provenance.

## CP3 — Git/ref mutation substrate

- **Status: PASS** (handover: `handovers/CP03.md`; gate: `experiments/cp3/cp3_gate.sh`).

**Objective:** the safe per-ref mutation primitive and direct human Git
compatibility.

**Tasks:** `UpdateRef(repo, ref, expected_sha, new_sha, provenance)` service;
per-ref CAS + stale reconciliation; observe external ref movement; direct
`git clone/fetch/pull/push` against Artifacts through the control plane;
policy hook for protected refs.

**Gate:** concurrent writers to a shared ref yield exactly one success and clean
stale handling; a local `git push` outside the UI is observed and reconciled;
agent principals hold no canonical-write credential.

## CP4 — Deterministic end-to-end vertical slice

- **Status: PASS** (handover: `handovers/CP04.md`; gate: `experiments/cp4/cp4_gate.sh`).

**Objective:** the first full loop, **deterministic (no AI model)**:

```
Work → Attempt → Strut executor → deterministic file edit → Git commit → PR
→ check → Integration Queue → preview → merge
```

**Why deterministic first:** proves Switchyard itself; removes model variability;
makes failures attributable; validates control-plane/event/Git/ref semantics.

**Gate:** the full loop runs with a scripted worker; every step is attributable
in provenance; integration lands only via the queue.

## CP5 — Events, provenance, realtime

- **Status: PASS** (handover: `handovers/CP05.md`; gate: `experiments/cp5/cp5_gate.sh`).

**Objective:** full event bridge; provenance graph; browser realtime.

**Tasks:** Artifacts event bridge at-least-once + idempotent (extend CP2);
provenance query surface; realtime via Worker/DO SSE bridge.

**Gate:** a change's full provenance chain is queryable; the dashboard reflects
live state; a forced outage converges via reconciliation within a bounded window.

## CP6 — Agent execution + credentials

- **Status: CONDITIONAL PASS** (handover: `handovers/CP06.md`; bounded: real-LLM
  CLI adapter requires a provider credential + CLI on the Linode).

**Objective:** pluggable runner adapter (v1: one coding-agent CLI adapter),
Agent Role/Profile/Principal/Execution, provider credentials, SecretStore
contract, first interactive Agent pass.

**Gate:** a real coding-agent CLI executes an Attempt through the same substrate
as a scripted one; secrets resolve only at execution; child agents inherit
reduced credentials; no secret in Git/logs/provenance.

## CP7 — Durable workflows

**Objective:** JS→step durable interpreter over Trestle + Strut; execution queue;
budgets/cancellation.

**Gate:** a workflow survives executor death and resumes idempotently; a
workflow can spawn a child workflow and wait on an approval.

## CP8 — Reviews/findings + conflict

**Objective:** structured Review Findings; draft contention; preview-based
structural/semantic detection; conflict routing.

**Gate:** two competing Attempts produce comparable PRs; a semantic conflict is
caught by preview integration; findings have a bounded repair loop.

## CP9 — Integration Queue full semantics

**Objective:** ordering, canonical-head freshness, policy, repair loops, risk
classes, preview.

**Gate:** queue behaviour under churn is correct and observable in the UI;
canonical changes only when policy allows.

## CP10 — Browser editor + Agent panel

**Objective:** CM6 editor, draft state, Save ≠ Commit, shared diff surface,
findings gutters, hideable Agent panel; commit via the shared substrate.

**Gate:** human → Agent → inspect diff → commit loop works end-to-end in the
browser; draft survives reload; Save ≠ Commit is honored.

## CP11 — Needs Attention + escalation

**Objective:** escalation decision packets; human approval flows; policy-driven
escalation.

**Gate:** an unresolved conflict produces a self-contained decision packet; a
human can decide without reconstructing history.

## CP12 — Organisation/enterprise + policy/risk

**Objective:** risk model (blast radius × uncertainty), org policies, budgets,
audit, fleet view; per-domain queues where justified.

**Gate:** policy constrains an org's agents without breaking the simple repo
case.

## CP13 — Dogfooding maturity

**Objective:** Switchyard builds, reviews, and integrates its own work (see
dogfooding progression below).

**Gate:** the dogfood progression is complete; Switchyard operates on Switchyard
repositories end-to-end.

## CP14 — Hardening & release

**Objective:** security review, performance/concurrency, failure/recovery,
self-hosted deployment, docs, release certification.

**Gate:** v1 release certified; no known secret-leakage or recovery defects;
self-hosted install documented and exercised.

## Proposed v1 boundary

- **Required for v1:** CP2–CP9 core (control plane, Git/ref correctness,
  deterministic slice, Agent execution via one adapter, basic workflows,
  review/findings, integration queue, provenance, realtime) + a usable basic
  browser editor (CP10 core) + self-hosted single-node deployment.
- **Strongly desired for v1:** Needs Attention (CP11), basic org policy, the
  interactive Agent panel (CP10), dark app shell with mobile menu.
- **Post-v1 / enterprise expansion:** full policy/risk model, enterprise
  credentials + audit + fleet, per-domain queues, semantic/intent conflict
  agents, competing-implementation synthesis, clustered Trestle (gated on the
  upstream fix), GitHub interop, mobile editor polish.

## Dogfooding progression

- **Dogfood 0:** current external DeepSeek/Codex work + handovers (in progress).
- **Dogfood 1:** Switchyard tracks its own Work (CP2).
- **Dogfood 2:** Switchyard launches a **scripted** Attempt against itself (CP4).
- **Dogfood 3:** a real Agent creates a Switchyard PR (CP6).
- **Dogfood 4:** Switchyard review/reconciliation processes its own PR (CP8).
- **Dogfood 5:** Switchyard Integration Queue integrates Switchyard work (CP9).
- **Dogfood 6:** full loop + editor drives Switchyard work (CP10+).

Target: **Switchyard is built using Switchyard.**

## Requirements → checkpoint map

| Requirement | Checkpoint |
| --- | --- |
| GitHub-like code browser / repo hosting | CP2 |
| Work | CP2 (create/list) → CP4 (full loop) |
| Attempts | CP4 |
| Pull Requests | CP4 |
| Safe ref mutation / CAS | CP3 |
| Integration Queue | CP4 (basic) → CP9 (full) |
| Direct human Git | CP3 |
| Events + reconciliation | CP2 (basic) → CP5 (full) |
| Provenance | CP5 |
| Realtime | CP5 |
| Agent execution + provider credentials | CP6 |
| Durable workflows | CP7 |
| Review/findings | CP8 |
| Conflict resolution | CP8 |
| Browser editor | CP10 |
| Agent panel | CP6 (basic) → CP10 (full) |
| Needs Attention | CP11 |
| Policy/risk/autonomy | CP12 |
| Enterprise org controls | CP12 |
| Mobile full-screen menu / dark / favicon / title | CP2 app shell (polish at CP10/CP14) |
| Dogfooding | CP13 (staged) |
| Self-hosting / release | CP14 |