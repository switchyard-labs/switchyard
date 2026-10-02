# CP1 experiments (E1–E4)

All experiments were run against the live environment on the CP0 Linode
(real Cloudflare Artifacts account `b7f20353…`, real Git clients, real
`git merge-file`). Raw outputs are in this directory (`cp1_e1.out`,
`cp1_e1supp_e3.out`, `cp1_e2.out`, `cp1_e4.out`); spike scripts retained.

## E1 — Artifacts per-ref contention / CAS (RESOLVED: git gives us CAS for non-force pushes)

**Hypothesis:** a stale, non-force push to a moved ref is rejected cleanly by
Artifacts' Git protocol, giving the planned `UpdateRef(repo, ref, expected_sha,
new_sha)` semantics from Git itself; `git ls-remote` (with auth) reads the
current ref SHA.

**Setup:** fresh repo `cp1-race-…`; A pushes baseline X; A and B clone at X;
A pushes Y; B commits from X and pushes normally; then B force-pushes.

**Results:**
- B normal push: **rejected** — `! [rejected] main -> main (fetch first)`,
  rc=1, with the standard "remote contains work you do not have" hint.
- B `--force`: **succeeded** (`forced update`, rc=0) — force bypasses CAS.
- Read path: `git -c http.extraHeader=… ls-remote <remote>` returns
  `refs/heads/main <sha>` and tracks the head across commits; without auth it
  returns nothing (repos require auth to list).

**Decision:**
1. The ref-mutation substrate can rely on **non-force git push as the CAS
   primitive**: a stale writer is rejected by Git itself, cleanly and
   detectable. The control plane's `UpdateRef` = read current SHA via
   `ls-remote` (expected) + push new SHA non-force; rejection = STALE.
2. **Force-push is the escape hatch and must be policy-gated.** The integration
   path never force-pushes; Switchyard must ensure no agent/reviewer/workflow
   writer holds force authority on refs it does not own (canonical especially).
   This is a permission/policy concern, not a protocol one.
3. No global repository lock is needed — per-ref push already isolates writers;
   per-ref in-process serialization remains a small ordering convenience for
   control-plane writers.

**Cleanup:** experiment repos deleted via the API.

## E3 — three-way merge for stale human/Agent edits (RESOLVED: git merge-file is deterministic and conservative)

**Hypothesis:** `git merge-file` (base ↔ ours ↔ theirs) gives deterministic
per-file three-way merge: clean merge for non-overlapping edits, conflict
markers + non-zero rc for overlapping edits.

**Setup:** three cases against real `git merge-file`: same-file non-overlapping
lines; same-line overlap; (from E2) adjacent-line edits.

**Results:**
- Non-overlapping distinct regions → **clean merge**, rc=0, both edits present.
- True same-line overlap → **conflict markers**, rc=1.
- **Adjacent-line edits can also surface as a conflict** (conservative diff3
  behaviour). This is acceptable and desirable: surfacing contention beats
  silent overwrite.

**Decision:** the editor draft concurrency model applies Agent edits via
`git merge-file` per file (base@agent-rev ↔ human-latest ↔ agent-edits);
rc=0 → apply cleanly; rc≠0 → surface contention ("Review patch / Retry on
latest / Resolve"). No last-writer-wins; no CRDTs; no external merge lib.

## E2 — draft recovery/sync spike (CONFIRMED semantics; real latency deferred to CP10)

**Hypothesis:** a Save synchronously persists a draft revision (manifest +
content-addressed blobs); reload/multi-device reconstruct the same working set;
an Agent apply against an explicit base revision is STALE when the draft moved;
expiry is enforced; no version history.

**Setup:** Python spike with a filesystem-backed manifest+blob store as a
stand-in for the planned Trestle manifest + object-store blobs.

**Results:** save persisted (rev + content); a fresh instance (browser crash)
reconstructed the same working set; a second device saw the same revision;
stale agent save returned STALE; expiry enforced. File-backed save latency
~0.27 ms is informational only — the real backend (Trestle manifest + object
store) latency is measured at CP10, not promised here.

**Decision:** the draft model is sound. Save = synchronous durable draft
revision; autosave = debounced periodic sync; recovery point is the last sync.
No version history (a draft is one current snapshot + base).

## E4 — secret resolution → executor path (CONFIRMED pattern; full impl at CP6)

**Hypothesis:** a provider secret written to a 0600 file, delivered to a child
executor, and deleted after the run never appears in child output/environment;
expiry is enforced at resolution; children don't inherit.

**Setup:** minimal Python spike implementing `resolve-for-execution` → 0600 file
→ child reads/uses/deletes → expiry check.

**Results:** secret absent from child stdout/stderr and child env; file deleted
after run; expiry enforced at resolution.

**Decision:** the SecretStore architectural contract is viable. Full
implementation (encryption at rest, per-deployment backend, rotation, audit,
reduced child inheritance) is CP6 work; this spike validated the
resolution-path pattern only.

## Adversarial review of the four pending Codex threads

Preserved as review contracts in `docs/plan/codex-review.md`. Provisional
positions from this DeepSeek pass (subject to independent review):

- **T1 (ref primitive):** E1 supports the git-CAS approach; the open sub-questions
  are force-push policy enforcement and the exact stale-reconciliation contract
  (rebase draft diff vs surface). No contradiction found.
- **T2 (runner adapter):** no experimental contradiction; contract is as
  documented in `docs/architecture/agents-and-workflows.md`.
- **T3 (takeover):** E2 confirms drafts are checkpointable working state; the
  checkpoint-commit-before-Attempt design is consistent with "Artifacts alone
  reconstructs the repo" — an Attempt cannot rest on a draft. No contradiction.
- **T4 (role precedence / repo→org secret):** E4 confirms the secret stays out
  of repo/provenance; repo-config selecting a role whose credential resolves
  through org policy has no experimental blocker.

**Gate decision: CP1 PASS.** No experiment contradicted a foundational
assumption. The four Codex reviews remain explicitly pending external review
and are not marked complete.