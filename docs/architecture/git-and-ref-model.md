# Git and ref model

Switchyard is built on Git. Cloudflare Artifacts is the Git truth substrate.
This document covers the authority split, the safe ref-mutation model, the queue
taxonomy, direct human Git, and external-movement reconciliation.

## Artifacts = Git truth

Authoritative for: repositories, commits, refs, trees/files, and repo-scoped Git
credentials/tokens. **Any durable collaboration state that claims to be a Git
commit/ref/file history belongs in Artifacts.** Draft storage may hold file bytes
for recovery but is never authoritative for repository history.

Invariant tests:
- When no draft exists, Artifacts alone reconstructs the repository.
- When a draft is lost, committed history is unaffected.
- When a draft is committed, its resulting commit/ref in Artifacts becomes the
  durable truth.

## Trestle = coordination truth

Authoritative for Work, Attempts, executions, leases, reviews/findings, workflow
state, integration state, provenance, policy/application state, and **normalized
Git events**. Single-node in the first release; clustered Switchyard is gated on
the separate Trestle clustering fix.

The Artifacts ↔ Trestle bridge is **at-least-once, idempotent, reconcilable**
(validated in Checkpoint 0). Reconciliation is a scheduled job that diffs
Artifacts' actual head/refs against Trestle's recorded state.

## Queue taxonomy (replacing the vague "thread-safe commit queue")

Three different mechanisms, not one:

1. **Execution queue** — schedules Agent executions, workflow steps, and
   Strut/runner work. Independent of Git.
2. **Safe per-ref mutation** — a low-level primitive/service, *not* a user-facing
   queue:
   ```
   UpdateRef(repo, ref, expected_sha, new_sha, provenance)
   ```
   Accept only when `current(ref) == expected_sha`; otherwise `STALE`, requiring
   reconciliation/retry. **The unit of serialization is the ref, not the
   repository.**
3. **Integration Queue** — higher-level PR/canonical scheduling: ordering,
   canonical-head freshness, preview against latest canonical, required checks,
   review policy, semantic-conflict checks, repair loops, and the final Git
   operation. The ref primitive makes mutation **safe**; the Integration Queue
   decides **whether/when** canonical may change.

"Commit queue" is retired terminology: commits are immutable objects; what is
serialized is ref updates (and commit application from working state).

## Per-ref mutation semantics

- Attempt branches are **single-writer by construction** (isolated forks/branches)
  — no serialization needed.
- Shared non-canonical branches (e.g., a PR branch being repaired by an editor
  and an Agent concurrently) use **CAS + retry/reconcile**.
- Canonical/default branches are **policy-controlled**; when queue-controlled,
  all automated canonical mutations flow through the Integration Queue.
- Tags/releases are a separate protected-ref policy.

**Experimental gate E1:** whether Artifacts' Git protocol exposes the
expected-old/CAS behaviour exactly as needed (Git conceptually does this via
receive-pack's old-ref check, but it must be verified under contention; do not
assume an API shape exists).

## Direct human Git is first-class

A solo developer must be able to use ordinary `git clone/fetch/pull/push` without
caring about agent machinery. Policy may progressively add protected branches,
required PRs, the Integration Queue, and review requirements.

- **Solo/simple repo:** human `git push origin main` may work normally.
  Switchyard observes canonical movement and invalidates/revalidates stale queued
  work.
- **Protected/agent-heavy repo:** canonical requires Switchyard-controlled
  integration.
- **Agents:** generally do NOT receive unrestricted canonical-write credentials.
- **Integration service:** has tightly scoped authority to perform approved
  canonical ref mutations.

## External Git movement reconciliation

The editor and the control plane must assume refs move from outside (local
`git push`, out-of-band changes).

- Every draft and every queued change carries a **base SHA**.
- When a branch moves externally, Switchyard detects `draft.base != branch.current`
  and offers reconciliation: continue against the old base until commit, then
  reconcile at the ref update (rebase or surface conflict); proactively flag
  "branch moved"; or auto three-way merge if clean. The git-native default is:
  your uncommitted working state survives a concurrent push; your commit becomes
  stale at the CAS.
- Queued Changes re-validate against the new base (validation cache limits cost);
  footprint-overlapping external changes re-run affected gates.

## The editor uses the same substrate

Browser commit is not a special one-off write path:

```
draft
  → Git commit object
  → UpdateRef(repo, branch, expected=base, new=commit)
```

The same substrate serves browser commits, Agent commits, repair commits,
conflict-resolution commits, and workflow-generated commits. **UX differs; the
mutation substrate converges.**

## Risk model (integration policy context)

Risk is two axes, not one coarse class:

- **Blast radius** (immutable, categorical): docs, generated, bugfix,
  dependency, benchmark-only, feature, public API, deployment config,
  auth/security, migration/destructive.
- **Uncertainty** (reducible by evidence): confidence the change is correct.

Evidence reduces uncertainty; it never changes blast radius. Autonomy is a
function of (blast radius, uncertainty, policy). Effective risk =
`max(work-declared, repo policy, deterministic change analysis, workflow
override, agent/reviewer escalation)` — escalation only ever raises, never
lowers, the class. Repo policy is version-controlled in the repository.