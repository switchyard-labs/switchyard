# Pending independent Codex review threads (T1–T4)

These four contract areas were identified during CP1. They are **explicitly
pending external review** and must not be marked complete by this DeepSeek
campaign. When a later checkpoint touches one of these contract areas, re-read
the pending question and record whether implementation evidence strengthened or
weakened the current decision, so the later Codex review is more valuable.

Each review should produce a bounded written contract/recommendation, not code.

## T1 — Ref-update primitive / Artifacts contention semantics

**Status of current decision (provisional):** `UpdateRef(repo, ref,
expected_sha, new_sha, provenance)` implemented as: read current SHA via
`git ls-remote` (auth) → push new SHA **non-force** → Git rejects stale writers
("fetch first") → STALE → reconcile (rebase draft diff or surface). Per-ref
serialization is a small ordering convenience. **Force-push is policy-gated and
never used by the integration path.** E1 supported this.

**Ask Codex:** review the reconciliation contract; the force-push policy
mechanism; whether per-ref in-process serialization is even needed given Git
CAS; the exact stale-detection/retry semantics for queued integration under
external movement.

## T2 — Agent runner adapter contract

**Status of current decision (provisional):** pluggable runner adapter under one
substrate: capabilities, context, credential resolution, cancellation,
execution events, budgets, provenance, output classification (reuse
`gantry-core/agent`). v1 ships one adapter (coding-agent CLI). E2/E4 provided no
contradiction.

**Ask Codex:** a second opinion on the substrate interface (context in, events
out, cancellation, budget, output classification) before it hardens; whether the
CLI adapter and a provider-API adapter can share it without distortion.

## T3 — Interactive-draft → Attempt takeover semantics

**Status of current decision (provisional):** at takeover the draft is
materialized as a checkpoint Git commit on a new attempt branch; Work is created
implicitly if absent; the Attempt's base is the checkpoint; provenance records
contributors and `origin: interactive-editor-takeover`. E2 supports that an
Attempt cannot rest on a draft (Artifacts alone must reconstruct the repo).

**Ask Codex:** checkpoint-commit-before-Attempt vs initial-working-tree; who is
attributed for the initial state; implicit Work creation heuristics; whether an
Attempt can ever exist without a Work item.

## T4 — Role configuration precedence + repo→org secret resolution

**Status of current decision (provisional):** built-in roles (implementer/
reviewer/conflict-resolver) usable immediately; user personal roles;
repository-defined roles (`.switchyard/agents/…`); org policy roles as the
constraining layer. Precedence: built-in < personal < repo < org (org
constrains, never escalates). Repo config references roles/provider-profiles
whose credentials resolve through org/user secret stores **without ever
containing a secret**. E4 supports the no-secret-in-repo/provenance boundary.

**Ask Codex:** the exact precedence and portability semantics; how repo config
selects a role whose credential only exists in the org/user store without
leakage or a confusing indirection layer; what happens when a repo config
references a role the org forbids.