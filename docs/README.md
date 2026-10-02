# Switchyard documentation

This is the durable design/planning home for **Switchyard**, an agent-native Git
collaboration platform: concurrent autonomous agents work in isolated Git state,
and integration into canonical repositories flows through deterministic,
policy-controlled, serialized integration.

The documentation is organised in three layers:

| Area | Purpose |
| --- | --- |
| [`architecture/`](architecture/README.md) | Consolidated architecture handover, domain model, Git/ref model, agent model, editor, security, realtime, open questions |
| [`product/`](product/ux-principles.md) | Product/UX requirements, application surface, brochure boundary |
| [`plan/`](plan/README.md) | Master checkpoint gameplan, experimental verification, upstream dependencies |

## Relationship to Checkpoint 0

[`checkpoint-0/`](../checkpoint-0/README.md) is **historical evidence** — the
feasibility/substrate validation phase (Linode environment, Trestle clustering
defect, Strut surface validation, Cloudflare Artifacts live round trip). It is
treated as evidence/history and is not rewritten. The architecture/planning docs
in this directory **reference** CP0 findings where relevant and build the
current design on top of them.

## Authority split (the core of the architecture)

- **Cloudflare Artifacts = Git truth.** Authoritative for repositories, commits,
  refs, trees/files, and repo-scoped Git credentials/tokens.
- **Trestle = coordination/application truth.** Authoritative for Work, Attempts,
  executions, leases, reviews/findings, workflow state, integration state,
  provenance, policy/application state, and normalized Git events.
- **Strut = bounded/disposable execution.** Strut workers execute steps; they do
  not own durable orchestration state.
- **Nift** builds the Switchyard web product/docs layer.

The Artifacts ↔ Trestle connection is an **at-least-once, idempotent,
reconcilable** bridge (validated live in Checkpoint 0), never a distributed
transaction.

## Reading order

1. `architecture/README.md` — the consolidated handover.
2. `architecture/domain-model.md` — Work/Attempt/Pull Request and terminology.
3. `architecture/git-and-ref-model.md` — Git truth, safe ref mutation, queues.
4. `architecture/agents-and-workflows.md` — Agent roles, runner adapters, workflows.
5. `architecture/security-and-credentials.md` — secrets, capabilities.
6. `architecture/editor.md` — the browser editor and draft state.
7. `architecture/realtime.md`, `architecture/open-questions.md`.
8. `product/*` then `plan/*` for requirements and the implementation gameplan.