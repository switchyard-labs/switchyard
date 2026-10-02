# Product / UX principles

The product philosophy, applied across Gantry projects and to Switchyard
specifically: **simple defaults, excellent ergonomics, serious depth when you
need it, no enterprise tax on the basic case.**

## Core principle: simple case first, enterprise scale without changing mental models

> Switchyard should feel like a very good GitHub alternative in the simple case,
> while revealing its agent-native machinery progressively as concurrency,
> automation, review complexity, and scale increase.

A human using Switchyard with one repository, no agents, no concurrent work, and
ordinary Git branches/commits should find the product extremely familiar. They
should never be required to learn: attempts, provenance graphs, semantic
contention, conflict agents, integration schedulers, durable workflows, agent
leases, candidate comparison, or queue sharding — unless those become useful.

**Product litmus test:** if you hand Switchyard to a developer who has never
heard of agent workflows, does it still feel like excellent Git hosting? If the
answer isn't yes, the agent side is over-designed.

## Progressive disclosure (the layer model)

- **Level 1 — ordinary Git hosting:** broadly GitHub-like UX. Repos, code,
  commits, diffs, issues/work, pull requests, checks, settings. No agent
  concepts visible.
- **Level 2 — single-agent assistance:** the familiar change page gains agent
  status, plan, automated review, test evidence, provenance, artifacts — but it
  still looks and behaves like a normal proposed change.
- **Level 3 — concurrent agent engineering:** only now expose multiple attempts,
  competing implementations, semantic overlap, the integration queue,
  autonomous review/repair, conflict resolution, active-work awareness.
- **Level 4 — organisation/enterprise:** policy, risk classes, approvals,
  compliance/audit, quotas/budgets, org-wide workflow templates, access
  controls, repository fleets, observability, distributed operation, high-scale
  integration scheduling.

Same product, same primitives, no migration to a different enterprise
architecture.

## Graceful trivial cases

Every advanced primitive has a trivial case:

- one Work item + one Attempt → looks like an ordinary issue/change;
- one candidate → no comparison UI;
- no semantic conflict → conflict machinery invisible;
- empty Integration Queue → feels like an immediate merge;
- no custom workflow → a good built-in default;
- one human developer → no agent-coordination UI;
- no enterprise policy → no policy bureaucracy.

> Agent-native capability must not make human-native Git workflows worse.

## GitHub familiarity, agent-native underneath

Use familiar repository layout, breadcrumbs, code browser, history, diffs,
review interactions, statuses/checks, settings, keyboard/command-palette
conventions. **Depart from GitHub only when the agent-native model genuinely
improves the workflow.** Novelty is not a goal.

Familiar UX is not the same as legacy semantics: a PR-looking page can represent
something genuinely different underneath (competing attempts, structured
evidence, autonomous review, provenance, integration proposals).

## UX/DX quality is a first-class architecture requirement

Treat API ergonomics, CLI ergonomics, web UX, error messages, defaults,
discoverability, docs, agent discoverability, and observability as architecture
concerns, not post-implementation polish. The system should be pleasant at every
scale, not only after extensive configuration.

## Test every abstraction at every scale

For each proposed Switchyard abstraction ask:

1. What does this look like with one human and no agents?
2. With one agent?
3. With 50 concurrent agents?
4. At enterprise scale?
5. Does the user need to learn a new concept before it is useful?
6. Can it remain hidden in the trivial case?
7. Does adding scale preserve the same mental model?

Target: **familiar simplicity at the bottom, agent-native concurrency in the
middle, serious enterprise operation at the top — without discontinuities
between them.**

## Terminology

If a concept is genuinely a Git concept, use the Git term; preserve familiar
Git-hosting terminology when semantics remain truthful; introduce new terms only
for genuinely new Switchyard concepts (see `architecture/domain-model.md`).
Do not rename for novelty.