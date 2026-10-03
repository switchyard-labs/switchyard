# Agents and workflows

The Agent substrate is one coherent model, not five unrelated integrations.
Interactive editor Agents, implementation Agents, review hooks, conflict hooks,
and workflow-spawned Agents all share the same role model, capability model,
credential/provider resolution, execution events, provenance, cancellation,
budget accounting, and output classification.

## Agent concepts (distinct)

| Concept | Definition | UI prominence |
| --- | --- | --- |
| **Agent Role** | Intent + capability contract: name, system instructions, Switchyard capabilities (read files, edit draft, create branch, push, open PR, review, spawn, queue, merge), allowed repo/work scopes, budget, timeout, spawn depth, network/tool permissions, execution-backend preference. Workflows/panels reference roles (`agent("reviewer", …)`). | Public |
| **Provider Profile** | How an external model/agent is invoked: provider, model, connection type, credential *reference* (opaque id). Non-secret. | Settings-visible |
| **Principal** | Authorization/audit identity of an invocation (e.g., `agent:execution-981`), derived from role + credential + session. | Internal, shown in provenance |
| **Execution** | One actual run: id, role, principal, provider used, budget consumed, events, outcome. | Per-run views |
| **Worker / Executor** | Infrastructure performing the run (Go deterministic adapter, a bounded agent-CLI process, a Cloudflare CI runner). | Internal ops / fleet view |

Only Role and Execution need to be prominent in normal UX. `credential ≠
profile ≠ execution` is a hard rule.

## Pluggable runner adapter

```
Agent Role
   ↓
Provider Profile
   ↓
Runner Adapter
   ├─ coding-agent CLI (v1 default candidate)
   ├─ provider API
   ├─ local model
   ├─ remote/self-hosted runner
   └─ future adapters
```

All adapters share: capability model, context, credential resolution,
cancellation, execution events, budgets, provenance, output classification
(reuse `gantry-core/agent` run-state/outcome classification).

**Architectural capability vs v1 scope:** the architecture supports multiple
adapters; the first implementation does not need to ship all of them. Current v1
lean is **one coding-agent CLI adapter first** (Warden proves the pattern:
spawn an OpenCode-family CLI with a provider profile and stream events). This is
explicitly provisional pending independent review (Codex thread T2).

## Interactive editor Agent

- Operates on the **same draft** as the human (see `editor.md`); edits are
  attributed to the Agent execution, applied against an explicit draft base, and
  never blindly overwrite newer human edits.
- The Agent does **not** commit unless asked or policy says so (interactive
  panel); formal Attempts commit as part of their own execution.
- Trivial assistance on a human branch creates no Work/Attempt; "take this over"
  explicitly transitions into an Attempt (see `editor.md` §Takeover).

## Durable workflows

- Authoring: a constrained JavaScript API over durable workflow state — steps
  (`agent`, `gate`, `integrate`, `approve`, `spawn`, `waitFor`, `timeout`) with
  no arbitrary side effects.
- Execution: a durable interpreter with state in Trestle, deterministic step
  identities and persisted effect intents; it resumes after a crash. The current
  deterministic adapter is Go, while real coding-agent subprocesses use the
  bounded runner boundary. Strut is not a runtime dependency. Repository CI is
  a separate Cloudflare Worker/Workflow domain.
- Sub-workflows are child state machines; approvals are durable waiting states;
  cancellation is a durable token.
- Workflows and hooks reference **roles, never secrets**.

## Workflow / hook gates

Deterministic gates (compile, tests, contract/API compat, static analysis)
first; bounded LLM review above a risk threshold; structured findings, not
comment threads; capped repair rounds; risk-driven depth and human-approval
policy. "Human attention where value is highest" is the goal, not "humans stop
reviewing."

## Role layering (conceptual)

- **Built-in roles:** `implementer`, `reviewer`, `conflict-resolver` — usable
  immediately.
- **User personal roles:** editor-Agent preferences.
- **Repository-defined roles:** committed config (`.switchyard/agents/…`).
- **Organization policy roles:** enterprise standardization; org policy is the
  constraining layer.

Precedence: built-in defaults overridable by personal → repo → org; org policy
constrains (can restrict, never escalate). A repo config may reference a role or
provider-profile whose credential lives in the org/user secret store **without
ever containing the secret**. Exact precedence/syntax is a Codex-review thread
(T4).

A `reviewer` role must survive changing providers (DeepSeek → Codex → local
model) without rewriting workflows: workflows express intent/capability, not
vendor selection.