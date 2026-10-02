# Experimental verification checklist

Must-prove items before depending on them. Each experiment has: hypothesis,
measurement, decision, cleanup/retention rule. Code for experiments lives under a
research/experiment directory; **no product architecture is silently changed**;
every experiment is explicitly marked `EXPERIMENT`.

## E1 — Artifacts per-ref contention / CAS

- **Why:** the ref-mutation substrate assumes `UpdateRef(repo, ref,
  expected_sha, new_sha)` semantics are obtainable through Artifacts' Git
  protocol (Git's receive-pack expected-old behaviour).
- **Hypothesis:** a stale, non-force push to a moved ref is rejected cleanly, and
  the control plane can detect the expected-old mismatch to drive reconciliation.
- **Measurement:** push two commits against the same branch from two writers
  where the second's base is stale; record accept/reject behaviour and error
  shape; test force-push handling; test per-ref independence (two refs updating
  concurrently).
- **Decision gate:** if stale pushes do not reject cleanly, the substrate needs a
  control-plane-enforced serialization layer instead of relying on Git — this
  changes CP3 scope.
- **Cleanup:** delete experiment repos; no residual refs.

## E2 — Draft recovery / sync

- **Why:** editor drafts must be recoverable transient working state with
  bounded recent-edit loss under reload/crash and multi-device continuation.
- **Hypothesis:** explicit Save persists a draft revision durably; autosave
  yields bounded loss under normal operation.
- **Measurement:** measure real sync latency (no promised numbers in docs);
  simulate reload, browser kill, and device switch; measure lost-edit window.
- **Decision gate:** acceptable if Save is durable and autosave loss is small and
  clearly bounded under normal operation; otherwise revisit storage/backoff.
- **Cleanup:** experiment drafts TTL'd/deleted.

## E3 — Three-way merge for human/Agent stale edits

- **Why:** Agent edits apply against an explicit draft base with no
  last-writer-wins.
- **Hypothesis:** per-file three-way merge (base ↔ human-latest ↔ agent-edits)
  applies cleanly for non-overlapping edits and surfaces contention for
  overlapping same-file edits, deterministically.
- **Measurement:** the race matrix A–G from design (`../architecture/editor.md`
  and the design history): human/agent same file, two agents different files, two
  agents same file, commit during agent run, external push, PR-branch movement,
  canonical movement.
- **Decision gate:** deterministic outcomes in all cases; overlapping edits
  always surfaced, never silently overwritten.
- **Cleanup:** none beyond test fixtures.

## E4 — Secret resolution → executor path

- **Why:** plaintext must exist only in the narrow execution-resolution path.
- **Hypothesis:** a role's credential is resolved at execution time, delivered to
  the executor scoped and expiring, and never appears in logs/provenance/
  artifacts; child agents inherit reduced or no credentials.
- **Measurement:** resolve → run → capture logs/provenance; verify no secret;
  verify expiry enforcement; verify child-agent reduction; measure resolution
  latency.
- **Decision gate:** no leakage anywhere; expiry enforced; latency acceptable.
- **Cleanup:** delete experimental secrets on completion.

## E5 — Event bridge at-least-once under outage

- **Why:** the Artifacts → Worker → Trestle bridge is at-least-once + idempotent
  + reconcilable.
- **Hypothesis:** dropping the bridge worker or killing Trestle mid-ingest
  converges via reconciliation within a bounded window, with no missed or
  duplicated durable effects (idempotency keys).
- **Measurement:** forced outages; converge time; duplicate-effect count.
- **Decision gate:** convergence bounded; idempotency holds (partially validated
  in CP0 — extend to outage injection).
- **Cleanup:** none.

## E6 — Durable interpreter resume/idempotency

- **Why:** workflows are a durable interpreter (state in Trestle, steps in
  Strut).
- **Hypothesis:** killing an executor mid-step resumes from the last checkpoint
  and re-executes idempotently (deterministic step ids + idempotency keys).
- **Measurement:** kill at several points; verify exactly-once durable effects.
- **Decision gate:** resumable + idempotent at all kill points.
- **Cleanup:** none.

## Handling of experiments

- Experiments may live under `experiments/` (to be created at CP1) or the Linode
  `/opt/cp0` scratch area.
- Each has a one-page README: hypothesis, setup, measurement, decision, cleanup.
- An experiment that contradicts an architecture assumption **stops and is
  reported** — never silently adapted.