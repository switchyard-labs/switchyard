# Post-CP13 acceptance review

Consolidated checkpoint acceptance after the external review. This is a
correction pass, not a new feature campaign. Follow-up commits only; history is
preserved (no amend/rebuild/squash); nothing is pushed.

## Checkpoint acceptance after external review

| CP | Final reviewed status | Notes |
|---|---|---|
| CP1 — architecture synthesis + verification | PASS | T1–T4 still pending independent Codex review; E1–E4 evidence stands. |
| CP2 — control-plane foundation | PASS | auth/users/sessions, repo browser, Work, idempotent ingest, reconciliation, dark shell. |
| CP3 — safe Git/ref mutation substrate | PASS | per-ref CAS; stale handling; agent principals hold no canonical-write credential. |
| CP4 — deterministic end-to-end vertical slice | PASS | Work→Attempt→executor→edit→commit→PR→check→queue→merge; every step attributable. |
| CP5 — events/provenance/realtime | **CONDITIONAL PASS** | fast path (Queue→Switchyard→Trestle→SSE) + safety net + `(repo,ref,before,after)` identity proven; **upstream Artifacts event-subscription→Queue delivery currently unavailable** (CP0 proved it once); re-certify the live source leg when upstream resumes → then PASS. |
| CP6 — real Agent execution + credentials | **CONDITIONAL PASS** | agent substrate, roles, encrypted credentials, deterministic adapter proven; real provider-backed coding-Agent CLI adapter unexercised (no CLI/credential on the control-plane host; investigation done, no secret copied). |
| CP7 — durable workflows | PASS | **true process-death replay proven**: control plane killed during a running durable step; completed Agent step replayed (attempts=1), in-flight sleep retried, later steps executed once, no duplicated external effects. |
| CP8 — reviews/findings + conflict | PASS | structured findings; competing PRs; **textual/Git merge conflict** detection + deterministic repair loop (accurately worded). Semantic-conflict detection separately proven in `experiments/cp8/semantic` (clean git merge, contract violation, semantic finding, queue blocked). |
| CP9 — Integration Queue full semantics | PASS | strict FIFO; policy (check+preview-clean); canonical-head freshness (stale requeue, bounded); conflict blocking/repair; risk classes; observability. Measured ~18–22 s (3 PRs), ~42 s (10 PRs), ~4.2 s/item → scratch-clone overhead is a real CP14 optimization target. |
| CP10 — browser editor + Agent panel | PASS | CM6, durable draft, Save ≠ Commit, reload recovery, diff, commit via shared substrate, Agent panel (real Chromium, 13/13). **Draft revision/CAS added post-review** (no last-writer-wins; two-session / agent-vs-human / external-push races, 6/6). Agent panel classified as formal-Attempt mode; interactive draft-assistance is a CP14/v1 gap. |
| CP11 — Needs Attention + escalation | PASS | attention aggregation; self-contained escalation decision packets; human decisions via existing repair paths (take_ours/theirs/three_way, requeue, approve). |
| CP12 — organisation + policy/risk | PASS | organisation/policy architecture proven (org→repo, policy at run choke points, risk escalation, budgets, audit, fleet). Risk model is v1/prototype signals; enterprise authorization is NOT complete (later hardening). |
| CP13 — dogfooding maturity | PASS | **deterministic** Switchyard-on-Switchyard loop end-to-end (7/7). Dogfood 1/2/4/5 PASS, 3 BOUNDED (real-LLM leg), 6 component surfaces proven. |

Distinction maintained throughout: **checkpoint feature proven** vs **broader
future architecture implied by it** (see per-CP addenda and the CP14 backlog).

## CP14 proposed backlog (NOT executed)

### Correctness / hardening
- full recovery/restart campaign; queue worker crash mid-integration; workflow
  worker races; draft CAS/race hardening; schema migrations/versioning;
  malformed/untrusted repository content; API fuzzing/boundary tests.

### Security
- full authorization matrix; CSRF/session hardening; agent capability
  escalation tests; prompt/tool boundary review; path traversal; shell argument
  injection; credential lifecycle/leak scan; dependency audit; threat model;
  secret backend production options.

### Git / integration
- protected refs; force-push policy; tags; branch deletion; stale PRs; direct
  human canonical push while the queue is active; external force movement;
  queue crash after merge but before state update.

### Performance (benchmark, don't guess)
- current scratch-clone baseline vs bare-mirror + worktree/cache at 1 / 3 / 10 /
  25 concurrent or queued PR scenarios; preserve isolation and scoped
  credentials.

### UX
- end-user product review; desktop browser matrix; mobile layouts;
  keyboard/focus accessibility; editor stale-draft UX; **interactive
  Agent-to-draft flow**; error messages; empty/simple repo experience.

### Operations
- install/bootstrap; service lifecycle; backup/restore; logs/metrics; upgrades;
  schema migration; self-hosting documentation; disaster recovery.

### Certification
- clean machine; regression suite; sanitizer/static tooling where relevant;
  soak/concurrency; release candidate.

## Open items carried forward
- CP5: re-certify live `push → auto-delivered event → Queue → Switchyard →
  Trestle → SSE` when upstream Cloudflare event delivery resumes; upgrade to
  PASS.
- CP6: provide a provider-backed coding-Agent CLI + credential on the
  control-plane host; run the CP6 closing test; upgrade to PASS.
- T1–T4 independent Codex review (briefs updated with evidence; not faked).
- Interactive draft-assistance (Agent → draft → human diff → human commit).