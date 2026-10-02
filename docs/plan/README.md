# Switchyard plan

The implementation gameplan. This is a **living checklist**, not a sacred
waterfall: CP1 is detailed; later checkpoints are deliberately looser because we
will learn things before reaching them.

| Document | Purpose |
| --- | --- |
| `checkpoints.md` | The master checkpoint gameplan (CP0 → CPn) with gates, requirements map, and dogfooding progression |
| `verification.md` | Experimental verification checklist (must-prove items) |
| `dependencies.md` | Upstream project work (Trestle / Strut / Nift / Cloudflare / Warden) and the gates they impose |

## How checkpoints work

- Numbered checkpoints with: objective, prerequisites, concrete tasks, expected
  artifacts, tests/evidence, review gate, stop conditions, explicit non-goals.
- Every meaningful checkpoint ends with a gate verdict: **PASS / CONDITIONAL
  PASS / FAIL**, with evidence. No checkpoint silently bleeds into the next.
- Checkpoints favour **vertical slices** (prove working behaviour) over component
  silos (build all tables/APIs/workers/UI).
- **EXPERIMENT** checkpoints are explicitly marked: code lives under a research/
  experiment directory, a hypothesis + measurement + decision + cleanup rule
  exists, and no product architecture is silently changed.
- CP0 is historical evidence and is not renumbered or rewritten
  (`checkpoint-0/`).

## Requirements coverage

The requirement → checkpoint mapping lives in `checkpoints.md` so product
requirements are not silently lost to implementation ordering.