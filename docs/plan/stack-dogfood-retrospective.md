# Stack dogfood retrospective

Written after building Switchyard through CP1–CP13 and the PX0–PX25 product/UX
campaign on the switchyard-labs stack. This is an **independent, evidence-based**
critique of Nift, Trestle, and Strut from real, prolonged use — including the
small repeated frictions, not just headline conclusions.

## Nift — presentation/build layer

### What I actually did with it
- Built the brochure/docs site (19 tracked pages) and the Switchyard app shell
  (dashboard, repo viewer, work, signin, profile, operations, settings…).
- Iterated templates (`head.html`, `template.html`), content pages, and the
  tracked-page registry.

### Friction log (the small repeated problems)
- **I repeatedly guessed the wrong source directory.** The project has
  `content/`, `templates/`, `public/` (output), and `.nift/` (build state). On
  first contact I treated `public/` as hand-authored and even hand-wrote HTML
  alongside the Nift source. The build output (`public/`) is what's served, so
  "edit the file that's served" is a natural but wrong instinct. Nothing in the
  repo shouts "content/ is source, public/ is generated" until you read
  `HANDOVER.md`.
- **I kept forgetting pages must be tracked.** A `content/foo.html` is NOT built
  until `nift track foo`. The failure mode is silent: the page just doesn't
  appear. I lost real time to this (pages missing from output, then discovering
  the tracked-page registry).
- **I hard-coded asset paths and page links before switching to `@path()`.**
  The templates initially used `/assets/...` and `/repos.html`. Nift accepted
  them silently. `@path()` is documented and correct, but nothing *warns* when
  you link to a tracked page or an asset by its raw output path.
- **Frontmatter vs tracked titles.** I wrote `--- title: … ---` in content,
  expecting it to drive `<title>`. It's inert; Nift titles live in
  `.nift/tracked.json`. Harmless but confusing until learned.

### What was excellent
- **Build loop.** Full builds of 19 pages in ~7 ms. I could rebuild after every
  edit without thinking. This is close to ideal for a coding agent: cheap,
  deterministic, instantaneous feedback.
- **Static output.** The result is plain HTML/CSS/JS served directly by the Go
  control plane. No runtime, no bundler. `@path()` produces correct
  output-relative paths even from nested pages (`../assets/...`).
- **Error quality.** Build diagnostics were terse and pointed at the real
  mistake: "tracked name must stay inside the configured content/output
  directories", "tracked name resolves to a content/output path already managed
  by another tracked name", "dependency changed: …". I could usually recover
  without help.
- **`@path()` once understood** is genuinely good: project-aware, validated at
  build, and correct from any output depth.

### What surprised me
- The tracked-page model is the crux of Nift's simplicity and its discoverability
  gap at the same time. It's a tiny mental model, but it's invisible until read.
- How much an agent's *instinct* to edit served output fights the content/output
  split.

### Agent-facing metadata / introspection
A machine-readable manifest (tracked pages, templates, dependencies, output
paths) would genuinely help: it collapses the first-contact cost. Real tools in
priority order: `nift project-info --json` (one call to learn the project),
`nift paths --json` (map content→output), a `nift doctor`-style check that warns
on (a) untracked content files and (b) hard-coded links to tracked pages or
`public/` assets. `nift graph` / `explain` are lower value for the observed
friction.

### Documentation
`HANDOVER.md` is complete and correct — the problem was discoverability, not
content. An `AGENTS.md` at repo root that says "content/ is source, public/ is
generated, pages are tracked via nift track, use @path()" would have saved the
first-contact mistakes entirely.

### Verdict (Nift)
- **Liked:** instant incremental builds, static output, `@path()`, clear errors.
- **Disliked:** tracked-page prerequisite invisible; hard-coded paths not flagged.
- **Surprised me:** the build loop felt like the best part of the stack.
- **Slowed me down:** guessing the wrong source dir; forgetting `nift track`.
- **Prevented mistakes:** clear build errors; validated `@path()` relationships.
- **Caused mistakes:** silent untracked pages; no @path enforcement.
- **Should change:** project-info/paths introspection; @path warnings; AGENTS.md
  convention; warn on untracked content.
- **Should not change:** instant incremental builds, static output, @path.

| Dimension | Grade |
| --- | --- |
| Discoverability | weak |
| Mental-model clarity | good (once learned) |
| Feedback-loop speed | strong |
| Error quality | strong |
| Debuggability | strong |
| Agent suitability | good (with the introspection gap above) |
| Production suitability | strong |

## Trestle — coordination truth

### What I actually did with it
Trestle carried ~30 collections: users, sessions, repos, work, attempts, runs,
PRs, PR checks, events, ref observations, workflow runs, workflow steps,
findings, integration queue, drafts, escalations, organisations, policies,
audit, credentials, executions. It was the durable backbone of CP2–CP13 and the
PX campaign.

### Friction log (the small repeated problems)
- **I kept forgetting records live under `.values`.** Every list/read returns
  `{ items: [ { id, version, values: {…} } ] }`. Reaching into `.values` became
  automatic only after dozens of iterations. It's consistent — but it's the
  single most-repeated small cost across the whole campaign.
- **`If-Match` on PATCH/DELETE was discoverable only by trial.** The first
  PATCH fails with `precondition_required`; you must first `FindRecord` to get
  the version. Good CAS, but the "you need a version first" relationship is not
  signalled.
- **Idempotency-Key had a genuinely surprising branch.** Same key + same body →
  `replayed`. Same key + different body → **`409 idempotency_conflict`**. This
  was a real discovery (CP5) that shaped the dual-path event design. It's a
  coherent semantic but looks like an error to a client. I want a response that
  structurally says "identity already exists; nothing applied" rather than a 409.
- **Schema strictness saved me and confused me.** Adding fields to an existing
  collection failed with `validation_failed: values.foo: unknown` — this caught
  real bugs (queue `new_base_sha`, draft `revision`/`base_sha`). But the error
  doesn't list the allowed fields, so the fix was guesswork.
- **Filter syntax is thin.** Single `=` predicates work; urlQueryEscape needs
  care; compound predicates are uncertain. Fine for the app, under-documented.

### What was excellent
- Strictness = a good safety property. It repeatedly caught schema drift that a
  permissive store would have silently accepted.
- Idempotency + CAS + durability covered exactly the coordination needs:
  at-least-once events, draft CAS, credential delete, workflow step records.
- HTTP-native: the control plane could be the only client and it felt natural.

### Would Postgres/SQLite/Redis have been better?
For this workload (CAS, idempotency, records, schema, HTTP control plane,
eventual clustering): no. A relational DB adds query power but removes the
HTTP-idempotent-CAS surface and re-introduces the plumbing Trestle already
solved. The improvements are AI-DX affordances, not replacement.

### Verdict (Trestle)
- **Liked:** strictness, idempotency, CAS, durability, HTTP-native.
- **Disliked:** `.values` shape; terse unknown-field errors; 409 semantics.
- **Surprised me:** 409 idempotency_conflict as a *feature* once understood.
- **Slowed me down:** finding versions for PATCH/DELETE; `.values` extraction.
- **Prevented mistakes:** schema strictness; If-Match; idempotency.
- **Caused mistakes:** terse errors; filter uncertainty.
- **Should change:** return versions in read shapes; structured idempotency
  response; list allowed fields on validation failure; filter docs; migration
  tooling; optional Go SDK ergonomics (flattening helper, upsert).
- **Should not change:** the strictness, idempotency, and CAS core.

| Dimension | Grade |
| --- | --- |
| Discoverability | mixed |
| API predictability | mixed |
| Error quality | mixed (terse but safe) |
| Feedback-loop speed | good |
| Agent suitability | good (ergonomics gaps above) |
| Production suitability | strong |

## Strut — bounded execution

### What I actually used Strut for
Exactly one thing: the deterministic worker binary that the Go control plane
execs in the `DeterministicRunner` adapter (`internal/agent/agent.go`). Task in
→ fixed edit out. That is the entire runtime surface.

### Friction log
- I wrote the deterministic worker once (a tiny Strut program) and it ran. It
  did the job with no syntax/diagnostic friction — but there was almost nothing
  to friction against: it reads a task, computes an edit, writes a result.
- I never used Strut's interesting surfaces (HTTP, processes, PTY, SQLite,
  cancellation) from Switchyard. Those are CP0-probed capabilities, not
  exercised runtime code.
- The exact complexity Strut added to Switchyard: one subprocess boundary, one
  binary to build and deploy, one `SWITCHYARD_STRUT_BIN` path, and one runner
  interface implementation. A Go subprocess (or an in-process function behind
  the same `agent.Runner` interface) would remove the binary and the boundary
  with no observable loss today.

### Is Strut earning its place?
**Today: no.** The `agent.Runner` interface is valuable and should stay; the
Strut runtime behind it is not carrying weight. The deterministic worker is 50
lines that any compiled language could host.

**Future: plausibly yes, if** the executor layer takes on real agent work —
sandboxed LLM coding agents, PTY handling, resource limits, tool proxies,
protocol bridges. That is exactly the CP6-bounded real-LLM leg and any
sophisticated runner. Until that lands, Strut is an incidental box in the
diagram.

### Improvement path
Either (a) commit to running real agent executors through Strut and expand its
responsibilities, or (b) collapse the deterministic adapter to a Go subprocess
worker and drop the dependency, keeping the `agent.Runner` interface. Do not
leave it as a load-bearing-looking box that only execs a toy worker.

### Verdict (Strut)
- **Liked:** the language was pleasant for the one worker I wrote.
- **Disliked:** no strong dislike; the issue is underuse, not quality.
- **Surprised me:** how little of the platform I actually exercised.
- **Slowed me down:** essentially nothing (the one worker was quick).
- **Prevented mistakes:** N/A at this usage level.
- **Caused mistakes:** N/A.
- **Should change:** either expand the executor role or retire it from the
  deterministic path.
- **Should not change:** the runner-adapter boundary (valuable abstraction).

| Dimension | Grade |
| --- | --- |
| Contribution to Switchyard | weak (today) |
| Development experience | good (limited sample) |
| Agent readability | good (concise syntax) |
| Executor fit | unproven (future) |
| Production suitability | unproven (as-used) |

## Cloudflare Artifacts — Git truth
Excellent. Real git, per-ref CAS via non-force push, scoped tokens. Friction:
event-subscription → Queue delivery currently not flowing upstream (CP0 proved
it; observed stopped). Mitigated by reconciliation.

## Cloudflare Queue — event transport
Fine for the pull-based consumer. Friction: double-encoded JSON message bodies
(discovered by a failed ingest); upstream delivery stopped.

## Go control plane
The right call: one binary, clear packages, `git`/Trestle/Artifacts/goja;
durable workflow interpreter is a highlight.

## Whole-stack coherence

```
Nift              presentation/build (static → served by Go)
Go control plane  orchestration
Trestle           coordination truth (HTTP records)
Strut             bounded execution (currently thin)
Artifacts         Git truth
Queue             event transport
```

- **What fits naturally:** Nift↔Go (static served directly); Go↔Artifacts (real
  git); Trestle as the single durable store; dual-path events.
- **What feels forced:** Go↔Strut (thin, artificial today).
- **What is redundant:** the Strut dependency for the deterministic adapter.
- **What is missing:** Trestle AI-DX affordances; a real executor strategy.
- **What should become tighter:** Trestle client ergonomics.
- **What should stay separate:** Git truth (Artifacts) vs coordination truth
  (Trestle) — the boundary is a feature, not a cost.

## Replacement matrix

| Component | Keep | Improve | Replace | Still unproven | Why |
| --- | --- | --- | --- | --- | --- |
| Nift | ✓ | ✓ | | | Strong build loop; needs introspection/@path warnings |
| Trestle | ✓ | ✓ | | | Correct core; needs API ergonomics |
| Strut | | | (today) | ✓ (future) | Not earning complexity for the deterministic worker; may earn it for real executors |
| Cloudflare Artifacts | ✓ | | | | Excellent Git truth; upstream event delivery to re-verify |
| Cloudflare Queue | ✓ | | | | Fine transport; double-encoding documented |
| Go control plane | ✓ | | | | Right choice |
| CodeMirror | ✓ | | | | Editor substrate |

## Recommended upstream campaigns (separate, not built here)

### Nift AI-DX campaign
- **Observed:** first-contact wrong-directory instinct; untracked pages silent;
  hard-coded paths not flagged; frontmatter vs tracked-title confusion.
- **Proposed:** `nift project-info --json` + `nift paths --json`; @path warnings
  for hard-coded tracked/asset paths; warn on untracked content files; an
  `AGENTS.md` convention.
- **Why generalizes:** any agent or new human entering any Nift repo hits the
  same discoverability cost.
- **Compatibility:** additive; no build-output change.
- **Test plan:** enter a fresh Nift repo with only the tooling; measure
  time-to-correct-first-edit.
- **Priority:** high.

### Trestle AI-DX/API campaign
- **Observed:** `.values` extraction; If-Match discoverability; 409 semantics;
  terse unknown-field errors; thin filters; no migration path.
- **Proposed:** version-in-read helpers; structured idempotency responses;
  unknown-field errors listing allowed fields; filter docs; migration tooling;
  optional Go SDK flattening/upsert helpers.
- **Why generalizes:** these are the top friction points for any agent or SDK
  building on Trestle.
- **Compatibility:** additive.
- **Test plan:** instrument a new consumer; count API-format fixes.
- **Priority:** high.

### Strut executor-fit campaign
- **Observed:** Switchyard uses Strut for exactly one process boundary.
- **Proposed:** decide the executor strategy: either real agent executors
  (sandboxing, PTY, resource limits, tool proxies) through Strut, or retire it
  from the deterministic path.
- **Why generalizes:** clarifies whether Strut is a platform or a language.
- **Priority:** medium — needs the real-LLM/executor decision first.

## Recorded in this repo
- This retrospective: `docs/plan/stack-dogfood-retrospective.md`.
- Product findings: `docs/plan/deepseek-post-px-review.md`.