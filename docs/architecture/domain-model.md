# Domain model

The user-facing domain model is deliberately small. Git concepts keep Git names;
familiar Git-hosting concepts keep their meaning where the semantics are
truthful; new words exist only for genuinely new Switchyard concepts.

## Terminology principle

> If a concept is genuinely a Git concept, use the Git term. Preserve familiar
> Git-hosting terminology when semantics remain truthful. New terminology is
> justified only for genuinely new Switchyard concepts.

- **Git terms preserved:** repository, branch, commit, ref, tag, fork, remote,
  merge, rebase, clone, fetch, pull, push.
- **Git-hosting terms preserved where truthful:** Pull Request, Issue (as a Work
  subtype), Review, Check, Workflow (word preserved; semantics broader than
  Actions), branch protection.
- **New Switchyard terms (genuinely new):** Work, Attempt, Review Finding,
  semantic/intent/policy conflict (subtypes; textual stays "merge conflict"),
  Provenance, Needs Attention, Integration Queue, Agent principal/role, Claim/Lease.

## Primary user-facing objects

### Work

Durable intent/coordination container. Represents a feature, bug, investigation,
benchmark campaign, release, migration, or any piece of work. Can be
user-created, workflow-created, or **automatically created**. Users are never
forced to manually create Work before asking an Agent to do something — asking
an Agent to implement something with no existing Work creates one implicitly.

### Pull Request

Kept as the established term for the **proposed-integration object**. A PR
exists when an implementation is actually being proposed for integration into a
canonical/head branch. Not every Attempt becomes a PR. PRs survive merging as
history/provenance objects.

### Review

Familiar outer collaboration concept.

### Review Finding

Structured, Switchyard-native review result: category, severity, confidence,
status, location, evidence, proposed repair, verification result. Findings have
a lifecycle (open → addressed → verified → resolved/dismissed) with bounded
repair rounds.

## Durable internal objects

### Attempt

One human/agent execution against a Work item. An Attempt may own:
- an isolated branch/ref/fork;
- a plan (a typed durable Artifact);
- commits;
- artifacts/evidence;
- associated Agent executions.

Several competing Attempts may exist for one Work item. Failed or losing
Attempts remain valuable provenance (retained refs, one-line reason).

### Artifact

Content-addressed outputs/evidence (test results, benchmarks, reports, logs,
screenshots, plans). Attached to an Attempt/PR; producer + reference + checksum.

### Provenance

Structured evidence/decision graph: intent → decomposition → Attempt → evidence →
decision. See `security-and-credentials.md` for the pragmatic attribution level.

### Agent principal/role; Claim/Lease

See `agents-and-workflows.md`.

## Conflict categories (distinct — no magical universal conflict object)

| Category | Nature | Where it surfaces |
| --- | --- | --- |
| Draft edit contention | stale/overlapping editor edits | transient, inline in the editor |
| Git textual conflict | actual merge conflict | Git operation / PR |
| Structural conflict | API/schema/build change | contract/API-compat gates, preview integration |
| Semantic conflict | clean merge, incompatible assumptions | preview integration + tests |
| Intent conflict | two pieces of Work attempt contradictory outcomes | overlap detection / comparison |
| Policy conflict | change violates policy | policy gate |

Draft edit contention is **ephemeral editor state**, not a durable Finding or
Conflict record; only unresolved contention that needs durable routing escalates.
The other conflict types feed the review/conflict infrastructure, which is a set
of mechanisms (deterministic + test-based + bounded LLM judgment), not one agent.

## Cardinality

**One Work → 0..N Pull Requests.** Usually 0–1. Repair iterations update the
same PR (additive commits, no force-push); genuinely divergent outcomes become
separate PRs; rejected attempts stay Attempts and never need to become PRs.

## What is deliberately NOT a domain object

- **"Change"** — rejected as a noun: the word is overloaded ("this change
  contains three changes") and the concept had no stable identity that Pull
  Request + Work + Attempt do not already express.
- **Candidate / Integration Proposal** — resolved into PR states; the Integration
  Queue is a view over PRs in integration states plus a thin queue entry.
- **Draft** — recoverable transient working state (see `editor.md`), not a
  top-level collaboration object.

## Derived views (no independent identity)

Integration Queue, Activity timeline (provenance), Agent activity, Needs
Attention (escalation decision packets), commit history (Git view of integrated
PRs), competing-implementation set (PRs of one Work).

## Ephemeral runtime state

Editor draft, step-executor run state, leases, validation-cache entries, live
presence.