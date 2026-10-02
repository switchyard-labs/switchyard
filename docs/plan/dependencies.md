# Upstream dependencies and separate project work

Work that belongs to other projects, **not** to be fixed inside Switchyard.
Switchyard consumes released/certified capabilities; it does not patch its
dependencies ad hoc to make itself work.

## Trestle

Switchyard uses **single-node Trestle** for the coordination/application truth
in the first release.

- **Clustered Trestle is gated on a separate fix.** Checkpoint 0 reproduced that
  clustered/Raft record writes replicate records but produce **zero events,
  audit facts, or outbox jobs** (on any node, via leader or follower).
  [`checkpoint-0/trestle-clustering-repro/`](../../checkpoint-0/trestle-clustering-repro/README.md)
  Clustered Switchyard must not begin until this is fixed and certified upstream.
- **Cluster identity ergonomics:** forming a cluster requires the raft ServerID
  to equal the Gantry `tr_...` node_id, which is undocumented and fails
  confusingly. Recorded as a Trestle work item.
- **Candidate user-facing realtime** (user tokens on `/api/v1/realtime` +
  per-subscriber rule filtering) is a Trestle item. Not blocking — Switchyard
  uses the Worker/DO SSE bridge.
- Trestle rule language (7 expressions) is intentionally NOT used for Switchyard
  policy; policy lives in the application layer.

## Strut

Switchyard's step executors depend on the current-main Strut surface becoming a
released, stable version:

- **Release gate:** current-main HTTP streaming/SSE, cancellable processes/pipes,
  crypto, and cancellation must be reviewed, hardened, tested, documented, and
  included in an actual tagged release before Switchyard's engine depends on it.
  [`checkpoint-0/strut-surface-validation/`](../../checkpoint-0/strut-surface-validation/README.md)
- **Candidate Strut stdlib items (separate work):** string search/contains API;
  `embed_dir` returning `map<string,string>` without auto-including `<map>`
  (breaks the showcase examples); `:=` type-inference ergonomics for `+`
  expressions.
- No application-specific hacks added to Strut for Switchyard's convenience.

## Nift

- Switchyard consumes **released Nift** capabilities for the web product/docs
  build layer (static shell + islands for realtime panels).
- Do not modify Nift to make the Switchyard application work. If a genuine Nift
  gap is exposed, record it as a separate Nift consideration.

## Cloudflare (operational)

- Artifacts requires a **Workers Paid** account (account `b7f20353…` is on Paid
  as of 2026-10-02).
- Operational notes from CP0: the `cf` CLI's event-subscription source enum omits
  the artifacts source (use the REST API); HTTP-pull consumers need explicit
  enablement (`wrangler queues consumer http add` / dashboard); push is Git
  protocol v1 only; billing for Artifacts operations begins 2026-10-14.
- These are operational constraints, not Switchyard product work.

## Warden / gantry-core

- Consume `gantry-core/editor` (deterministic server-side edit primitive) and
  `gantry-core/agent` (run-state/outcome classification) as-is.
- **Do not transplant Warden's browser UI.** Conceptual reuse of the
  tree|editor|hideable-agent layout and the agent-CLI executor pattern; build a
  fresh Switchyard editor (see `../architecture/editor.md`).
- Extract shared browser primitives (tabs, resizers, dirty-state, diff component,
  findings gutters) only once a genuine stable boundary is visible after
  Switchyard's editor exists.

## Interaction rule

If a dependency change is required for Switchyard to proceed, it is filed as a
separate upstream work item with its own review, never patched inside the
Switchyard repository.