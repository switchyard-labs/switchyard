# Application product requirements

The authenticated Switchyard application (GitHub-like product UI) lives in the
`switchyard-labs/switchyard` repository. These are product requirements, not an
implementation plan.

## Established product invariants

- **Dark mode** is the primary intended visual design: dark, restrained,
  developer-tool density, high readability, minimal decoration, GitHub-esque
  familiarity. Avoid generic blue-heavy SaaS styling, glassmorphism, excessive
  gradients, and "AI product" visual clichés. Code, diffs, status, and work
  dominate.
- **Page/document title is exactly `Switchyard` on every page** — never
  "Repository · Switchyard", "PR #18 · Switchyard", "Settings · Switchyard".
- **Favicon** derived from the Switchyard identity/logo; never ship with a
  default/no favicon.
- **Mobile primary navigation** collapses behind a **hamburger**; activating it
  shows/hides a **full-screen navigation menu** (keyboard accessible, dismissible
  with Escape, focus trapped while open, scroll-safe, easy to operate on touch).
- **Agent panel** is showable/hidden; the product works with it fully hidden.
- **No terminal** in the Switchyard editor.
- Progressive complexity: agent-native capability appears only when useful.

## Repository navigation

Not final (terminology and granularity still open), current lean:

```
Code · Work · Pull Requests · Workflows · Queue · Artifacts · Activity · Settings
```

- `Work` replaces a pure `Issues` list (Issue is a Work subtype; benchmark
  campaigns, investigations, releases are Work, not issues).
- `Pull Requests` retained as the familiar proposed-integration surface.
- `Queue` (Integration Queue) is **progressive**: hidden at level 1, appears when
  protection/agents/concurrent changes/queued integrations exist.
- `Agents` is contextual rather than a permanent tab in the simple case; a fleet
  view appears at organisation scale.
- `Changes` (as a noun replacing Pull Requests) was rejected on terminology
  grounds (overloaded word); Pull Requests stays.

## Global / organisation navigation (level 4)

Repositories · Work (across) · Fleet/Agents · Integration Queue overview ·
Policy · Audit · Budgets · Needs Attention (across) · Settings. Aggregates
repo-local primitives; nothing new is invented at org level.

## Repository home

Familiar elements (name, visibility, description, branch/ref selector, latest
canonical commit, file tree, README, clone/access) plus a thin live strip:
active work, agents affecting the repo, candidates awaiting integration, queue
status, blocked/conflicting work, failing validation, human attention required.
Avoid an operations-dashboard noise problem: the strip stays thin and quiet when
idle.

## Code view

Ordinary Git browsing must not regress: browse files, inspect commits, view
diffs, inspect history, copy clone URLs. The canonical source tree stays
familiar.

## Change / Pull Request page

A familiar PR layout with agent-native depth revealed progressively:

```
Overview | Files changed | Reviews | Checks | Conflicts | Artifacts | Provenance
```

plus an "Advanced" disclosure (attempts, competing candidates, queue position,
semantic conflict status) that appears only when relevant. The same page serves
the simple one-human change and a concurrent multi-attempt change.

## Integration Queue

First-class and more prominent than GitHub's merge queue: queue position,
dependencies, why something cannot integrate, staleness, revalidation, priority,
affected subsystems, human escalation. Users must be able to understand why
canonical is not changing.

## Work

Work list (list/board/dependency-graph views) and work item pages: intent,
agents, attempts, affected areas, dependencies, status, human intervention
required.

## Reviews

Familiar diff review interactions (unified/split, files changed, inline
findings, resolved findings) plus structured agent findings with category,
severity, confidence, status, and repair requests — not endless comment threads.

## Conflicts

Visually distinguish textual / structural / semantic / intent / policy conflict;
show what autonomous resolution already tried before asking a human to
intervene.

## Provenance

A familiar GitHub-style timeline of meaningful engineering events (work
requested, planner decomposed, attempt started, candidate produced, tests
passed, review repaired, benchmark compared, rejected, queued, preview passed,
integrated). Default to concise summaries with expansion; never raw transcripts.

## Live concurrent-work awareness

Contextual indicators that active work affects this file/directory/API/contract/
package. Lightweight warnings ("⚠ Attempt #82 also modifies this file"); no
collaborative multi-agent editing.

## Needs Attention

First-class surface for human escalations, rendered as **decision packets**:
original intent, options, evidence, what automation/agents already tried, the
exact unresolved question, and consequences of each choice. A human must not
need to reconstruct hours of agent activity.

## Notifications / inbox

Humans notified only for direct mentions, requested approvals, unresolved
escalations, workflow failures exceeding policy, and releases/deployments
requiring approval. Agents have their own machine-oriented event/subscription
interface.

## Search

Global search across repositories, source, commits, work, pull requests, agents,
reviews, conflicts, artifacts, provenance, workflow runs. Semantic/AI search is
longer-term; provenance architecture must not preclude it.

## Realtime surfaces

Queue state, agent activity, concurrent-work indicators, change checks/conflicts,
Needs Attention count, workflow-run status. See `architecture/realtime.md`.

## Editor + Agent panel

See `architecture/editor.md`. The editor is a mode within Code, not a top-level
`IDE` tab: "Edit" on a file, "Open editor" from Code/Work/PR/Conflict.

## Mobile

Competent responsive behaviour; serious editor is desktop-oriented; on mobile,
panes become mutually selectable views (Files | Editor | Agent). Mobile
priorities: checking status, reviewing an escalation, approving/rejecting,
reading work/provenance, small edits.