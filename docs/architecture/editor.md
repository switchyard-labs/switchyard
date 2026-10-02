# Browser editor

A persistent browser editor inspired by the useful subset of Warden — **not** a
full browser IDE and **no terminal**.

```
file tree | editor | hideable Agent panel
```

## Scope

- Repository file tree, folders, tabs/open files, syntax highlighting, line
  numbers, search, dirty-file state, save, create/rename/delete.
- **Diff against branch HEAD and PR base** — the editor raises diff to a
  first-class concern.
- Explicit **Commit**; branch/ref context; discard/revert.
- Review-finding annotations; Agent interaction; keyboard shortcuts.
- No terminal, no debugger/LSP/refactoring, no multi-workspace root management,
  no local-filesystem model.

## Editor library

CodeMirror 6 is the provisional choice, evaluated on its own merits (Warden does
**not** use CodeMirror — it uses a custom textarea/overlay editor, so this is a
fresh decision, not extracted Warden technology). Modular, ~300KB, strong
extension model (custom gutters for findings/diff), good accessibility, works for
light editing on mobile, fits a Nift-built page as an island. Not a settled
dependency until implementation research confirms no blocker. Do not spend more
design rounds on this.

## Save ≠ Commit

```
edit → save working state → inspect modified set/diff → commit
```

The web UI does **not** expose Git's index/staging model. "Choose which modified
files to include in this commit" replaces the index for browser users.

## Draft = recoverable transient working state

A draft is **not**: repository truth, another source-control system, permanent
history, a top-level collaboration object.

A draft may have: base repo/ref/SHA, current revision, changed paths, current
content, create/delete/rename state, limited provenance (per-file last-mutator),
expiry.

A draft must NOT gain: branches, tags, arbitrary history, permanent versions,
independent merge semantics, or its own review lifecycle. The moment state needs
to be durable/branched/historical, it becomes a real Git commit in Artifacts.

**Recovery semantics (no arbitrary latency numbers):** *Explicit Save
synchronously persists the draft revision.* Autosave is a debounced periodic
sync whose real latency is measured, not promised. Requirement: under normal
operation, reload/crash recovery loses at most a small, clearly bounded amount of
the most recent editing; Save guarantees durable persistence.

**Terminology:** durable-but-ephemeral working state — persistent enough for
recovery, not permanent collaboration history, TTL/garbage-collected, not
canonical truth.

## Draft concurrency

- Agent actions operate against an explicit draft revision/base.
- **No last-writer-wins.**
- Different-file edits can usually apply independently.
- Same-file stale edits require merge/reconciliation (per-file three-way merge);
  overlapping edits surface contention.
- External Git movement does not destroy the draft; branch/ref movement is
  reconciled at commit/update time.

Escalation for stale edits: (0) clean apply, silent; (1) per-file clean merge,
applied + noted; (2) overlapping same-file → inline contention ("Review patch /
Retry on latest / Resolve"); (3) only if unresolvable by hand does it route
toward the conflict mechanism. Never a heavyweight conflict agent for an ordinary
stale edit.

## Agent interaction

- The interactive Agent operates on the same draft, attributed to its execution
  id; edits apply against an explicit base and never overwrite newer human edits.
- The Agent does not commit unless asked/policy says so.
- Human interaction is the simplest good model: view diff → accept/revert per
  file (per-hunk revert is a later refinement) → commit. No Copilot-style patch-
  approval system.

## Takeover (interactive → formal Attempt)

At "take this over," the draft is materialized as a **checkpoint Git commit** on a
new attempt branch; Work is created implicitly if absent; a formal Attempt is
created whose base is that checkpoint; provenance records contributors and
`origin: interactive-editor-takeover`. Exact takeover semantics are a Codex-
review thread (T3).

## Commit path (shared substrate)

```
draft
  → Git commit object
  → UpdateRef(repo, branch, expected=base, new=commit)   // same as all writers
```

Same mutation substrate as Agent commits, repair commits, conflict commits, and
workflow commits. Direct-main commit when policy permits; "create branch &
commit" + optional PR when protected.

## Findings in the editor

Inline gutter annotations (`84│ ^ HIGH: nullable return unchecked`) with actions
Explain / Ask agent to fix / Mark resolved / Open review. Collapsible so the
editor isn't cluttered. These are the same structured Review Findings used
elsewhere, not editor-only comments.

## Agent panel

- Hideable; collapses to a status affordance; opens a panel beside the editor.
- Context is explicit: repo, branch, open file, selection, Work/PR, findings,
  diff, concurrent-work warnings.
- The same agent roles/credentials/permissions as automated hooks.

## Mobile

- The main Switchyard site has strong responsive behaviour and a full-screen
  hamburger menu (see `../product/application.md`).
- The editor is desktop-oriented; on mobile, panes become mutually selectable
  views (`Files | Editor | Agent`) rather than squeezed columns. Mobile
  priorities: source browsing, tiny edits, review/findings, agent conversation,
  approve/escalate.