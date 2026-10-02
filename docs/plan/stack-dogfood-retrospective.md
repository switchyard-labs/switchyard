# Stack dogfood retrospective

Written after building Switchyard through CP1–CP13 on the switchyard-labs
stack. This is an **independent, evidence-based** critique of Nift, Trestle, and
Strut based on actual use — not a defence because we built them. It records what
worked, what caused friction, and what upstream campaigns would generalise.

## Nift — presentation/build layer (brochure + app shell)

### What worked well
- **Build speed.** Instant incremental builds (~0.005–0.01 s for 4–7 pages;
  `nift build --all` full rebuilds in milliseconds). The feedback loop for
  content/template edits is effectively zero-cost, which is ideal for an agent
  iterating on a site.
- **Simplicity + static output.** No runtime, no bundler, no node pipeline. The
  output is plain HTML/CSS/JS served by the Go control plane directly.
- **`@path()` convention.** Once understood, project-aware links and asset
  references resolve correctly regardless of output location, and the build
  validates the relationship.
- **Clear, terse build errors.** "tracked name must stay inside the configured
  content/output directories", "tracked name resolves to a content/output path
  already managed by another tracked name", "dependency changed: …" — the
  diagnostics pointed at the actual mistake.
- **Incremental/targeted builds.** Editing one content file rebuilds only it.

### What was awkward (and why)
1. **Discoverability of the project model.** The first pass treated `public/`
   as hand-built HTML and produced hand-written pages alongside the Nift
   source. The actual model (content/ + templates/ → tracked pages → public/)
   and the `nift track` requirement (a new page is NOT built until tracked)
   were only fully understood after reading `HANDOVER.md`. The engine wasn't
   the problem; **the convention wasn't signalled until you read the handover.**
2. **Tracked-page registration is a hidden prerequisite.** `content/foo.html`
   existing is not enough; you must `nift track foo`. Nothing in a bare repo
   tells an agent (or human) that. The failure mode is silent-ish: the page
   just doesn't appear in the output.
3. **Metadata model ambiguity.** Nift tracks titles in `.nift/tracked.json`,
   not YAML frontmatter. A frontmatter `title:` in content is inert (harmless,
   but confusing when you expect it to drive `<title>`).
4. **Hard-coded paths were not flagged.** The first templates used
   `/assets/…` and `/repos.html`; Nift accepted them. `@path()` is documented
   but not enforced, so "works but wrong convention" is easy to ship.

### Classification
The friction points were **convention/discoverability gaps**, not engine
defects. Build errors were good; the missing signal was "this is a tracked-page
project with conventions; read the handover / use @path".

### AI-DX scorecard (evidence-based)
| Dimension | Grade | Evidence |
| --- | --- | --- |
| Discoverability | weak→mixed | Project model hidden until HANDOVER; `nift track` prerequisite unstated. |
| Agent readability | good | Output is plain HTML; templates are declarative. |
| API predictability | mixed | Frontmatter inert vs tracked.json titles; @path not enforced. |
| Error quality | strong | Terse, precise build diagnostics. |
| Feedback-loop speed | strong | Sub-10 ms rebuilds. |
| Documentation | mixed | HANDOVER exists and is complete, but not discoverable until told. |
| Debuggability | strong | Static output + clear dependency tracking. |
| Composability | good | Templates + @input + content. |
| Production suitability | strong | Static output, no runtime. |

**Verdict: Keep as-is, improve AI-DX.** General upstream ideas (not built here):
- a `nift doctor` / `nift inspect` / `nift project-info --json` that prints the
  tracked-page model and flags convention issues;
- warn on hard-coded paths that look like tracked pages or `public/` assets
  ("use @path(...)");
- emit a machine-readable project manifest an agent can read in one call;
- surface "content file exists but is not tracked" as a build-time warning.

## Trestle — coordination truth

### What it carried (actual load)
Work, attempts, PR coordination, events, ref observations, workflow runs,
workflow steps, findings, integration queue, drafts, escalations, organisations,
policies, audit, credential metadata. Everything coordination-shaped lived
here, and it held up across CP1–CP13 including restart recovery.

### What was excellent
- **Durability + idempotency.** At-least-once redelivery converges to one
  record via Idempotency-Key. The event fast path + reconciliation safety net
  share an identity and converge on one durable domain event.
- **Strictness caught real bugs.** Schema validation rejected unknown fields on
  write — this caught, at runtime, several Switchyard mistakes (the queue's
  `new_base_sha` patch, and later the draft `revision`/`base_sha` fields before
  they were added to the schema). "Validation failed: unknown" is terse, but it
  is a **good safety property** that prevented silent schema drift.
- **If-Match CAS.** Optimistic concurrency for PATCH/DELETE (credential delete,
  draft revision CAS, policy patches) maps directly to the editor's no
  last-writer-wins requirement.
- **Provisioning.** `EnsureCollection` made the control plane self-provisioning.

### What caused friction
1. **Record `.values` representation.** Every read returns `{id, version,
   values:{…}}`; clients must reach into `.values`. Consistent, but a repeated
   small cost across ~30 collections.
2. **If-Match discoverability.** PATCH/DELETE silently(ish) fail with
   `precondition_required` until you learn you must find the version first.
   Good CAS; the affordance (returning the version, a helper, clearer docs) was
   discovered by trial.
3. **Idempotency-Key semantics.** Same key + same body → `replayed`. Same key +
   different body → **`409 idempotency_conflict`**. This was a significant
   discovery (CP5) and is the crux of cross-path convergence. It's a coherent
   design, but it looks like an error to a client that expects either
   "replayed" or "created". A header or structured body that explicitly says
   "identity already exists; no change applied" would have saved real time.
4. **Filter syntax.** Single-`=` filters, `urlQueryEscape` needing care with
   quotes/spaces, and uncertainty about compound filters. Functional but
   under-documented for agents.
5. **The clustered side-effect defect (CP0).** A real upstream finding, gated;
   single-node use was unaffected.
6. **Schema changes.** Adding fields to a live collection required recreating
   the collection (ephemeral drafts were fine; production would want a
   migration path).

### Classification of each friction
| Friction | Classification |
| --- | --- |
| `.values` representation | SDK/client + documentation affordance |
| If-Match prerequisite | Good safety property, poor discoverability |
| 409 idempotency_conflict | API design subtlety; needs a clearer affordance |
| Filter syntax | Documentation + API affordance gap |
| Schema strictness | Good safety property, terse error |
| Clustered defect | Actual Trestle deficiency (upstream, gated) |

### Would Postgres / SQLite / Redis be better?
Compared against actual requirements (CAS, idempotency, records, durability,
schema, HTTP-native control plane, concurrency, eventual clustering,
outbox/event needs, agent ergonomics):
- A relational DB gives richer queries but loses the HTTP-native, idempotent
  record surface and the "coordination truth over HTTP" shape that fits a
  self-hosted control plane.
- Redis loses durability and schema enforcement.
- SQLite is a fine single-node fallback but re-introduces the query/CAS/
  idempotency plumbing Trestle already provides.
Trestle is still the right fit for the coordination-truth role. The
improvements are **AI-DX affordances**, not replacement.

### Verdict: Keep as-is, improve AI-DX (separate Trestle campaign, not built here)
- return the record version in every read shape the client can patch with;
- make `Idempotency-Key` responses structurally distinguish
  created / replayed / identity-conflict;
- a `?filter=` parser documented for compound predicates;
- friendlier "unknown field" errors that list allowed fields;
- a documented, safe schema-migration path.

## Strut — bounded execution

### What actually happened
- The **only** Switchyard code that exercised Strut is the deterministic runner
  adapter (`internal/agent/agent.go`), which execs the Strut worker binary with
  a task and reads its output. Everything else — workflows (goja interpreter),
  review, conflict resolution, queue, refs, policy — runs in the Go control
  plane using `git`, Trestle, and Artifacts.
- CP0's Strut surface validation probed the *capability model* (what a bounded
  executor can/can't do) and informed the executor boundary decision. That was
  design-stage value, not runtime value.
- The worker abstraction (the `agent.Runner` interface) is genuinely valuable:
  it is what makes the deterministic adapter and a future coding-agent-CLI
  adapter interchangeable. But Strut-the-runtime is not what carries that
  value — the interface does.

### Honest verdict: Strut is not currently earning its architectural complexity
The deterministic worker is effectively a Go binary the control plane execs. A
Go subprocess worker (or an in-process deterministic function behind the same
`agent.Runner` interface) would replace it with **no meaningful loss** today.
Strut would earn its box if real agent executors ran through it — sandboxed
LLM coding agents, capability-gated tools, disposable environments — which is
exactly the CP6-bounded (real-LLM) leg and any future executor sophistication.
Until then, the architecture diagram should honestly label Strut as
"bounded-execution substrate, currently exercised only by the deterministic
adapter."

### If Strut is to stay useful
General gaps to improve in a separate Strut campaign (not built here): a
clearer runner/task contract, capability introspection, and making the
executor boundary the natural place for sandboxing rather than an extra hop.

### AI-DX scorecard
| Dimension | Grade | Evidence |
| --- | --- | --- |
| Contribution to Switchyard | weak (today) | One binary exec in one adapter. |
| Executor boundary clarity | good | Runner interface is clean. |
| Strategic value | unproven | Real executors not yet run through it (CP6 bounded). |
| Replacement cost today | low | A Go subprocess worker would suffice. |

**Verdict: Improve or defer.** Either commit to running real agent executors
through Strut (making it earn the box), or collapse the deterministic adapter to
a Go subprocess worker and drop the dependency until then. Do not leave it as an
incidental box that looks load-bearing.

## Cloudflare Artifacts — Git truth
Excellent. Ordinary `git` works unchanged; refs/objects are exactly what git
wrote; per-ref CAS via non-force push is a genuine design win; scoped tokens fit
the authority model. Friction: the event-subscription → Queue delivery leg
currently not flowing upstream (CP0 proved it; observed stopped since) — an
upstream operational condition, mitigated by reconciliation.

## Cloudflare Queue — event transport
Fine for the pull-based consumer the control plane uses (edge can't reach the
control plane). Semantics (visibility timeout, lease, ack/retry) worked.
Friction: message bodies arrive as JSON-encoded strings (double-encoding) —
discovered by a failed ingest; and the delivery-stopped upstream condition.

## Go control plane
The right call. One binary, clear packages, `git` + Trestle + Artifacts +
goja; concurrency, CAS, and durability all expressed directly. No framework
lock-in; the durable-workflow interpreter (goja) is a highlight.

## Whole-stack coherence
```
Nift              presentation/build (static → served by Go)
Go control plane  everything orchestration
Trestle           coordination truth (HTTP records)
Strut             bounded execution (currently thin)
Artifacts         Git truth
Queue             event transport
```
- **Good seams:** Nift ↔ Go (static output served directly); Go ↔ Artifacts
  (`git` subprocess + REST; "Git stays Git"); Artifacts events ↔ Queue ↔
  reconciliation (dual-path, clean); Trestle as the single durable store.
- **Awkward seams:** Go ↔ Trestle (client ergonomics: `.values`, If-Match,
  idempotency semantics); Go ↔ Strut (thin, mostly incidental).
- The explicit boundaries were a net positive: they forced the fast-path /
  safety-net split and kept Git authoritative. The cost is the number of moving
  pieces; the two seams needing work are Trestle AI-DX and Strut's role.

## Replacement analysis
| Component | Replace with? | What improves | What gets worse |
| --- | --- | --- | --- |
| Nift | A generic static generator | Nothing fundamental | Loses instant incremental builds + @path validation |
| Trestle | Postgres/SQLite/Redis | Richer queries | Loses HTTP-native idempotent CAS surface; re-introduces plumbing |
| Strut | Go subprocess worker (today) | Fewer moving pieces | Nothing meaningful while only the deterministic adapter uses it |
| Artifacts | A self-hosted git host | — | Loses managed Git + scoped tokens + events |
| Queue | Long-polling the API | — | Loses at-least-once transport |

Do not replace a component merely because a mainstream alternative exists. The
evidence favours keeping Nift + Trestle + Artifacts + Queue, and either
redeploying Strut meaningfully or retiring it from the deterministic path.

## Recommended upstream campaigns (separate, not built here)
- **Nift:** `nift doctor` / `inspect` / `project-info --json`; @path warnings
  for hard-coded tracked paths/assets; warn on untracked content files.
- **Trestle:** idempotency response affordances, version-in-read helper, better
  "unknown field" errors, filter docs, safe migration path.
- **Strut:** decide the executor's strategic role; if kept, a clean runner
  contract + capability introspection + sandboxing as the default boundary.
- **Artifacts:** re-verify event-subscription delivery; document the
  double-encoded queue body shape.