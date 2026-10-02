# Switchyard — Checkpoint 0: Platform reconnaissance

Status: **CONDITIONAL PASS** (see Gate decision at the end).

This directory records the Checkpoint 0 / platform reconnaissance phase of
Switchyard, an agent-native Git collaboration platform. It is **research and
evidence only** — no Switchyard product code, no deployment, no changes to
Nift, Trestle, or Strut.

## Scope

Per the Checkpoint 0 brief:

- A. Provision an isolated disposable Linode for experiments.
- B. Establish this research workspace in the Switchyard repository.
- C. Reproduce the Trestle clustered (Raft) side-effect claim.
- D. Validate the Strut surface Switchyard actually needs.
- E. Validate the Cloudflare Git-plane round trip (repo → fork → token →
  clone → commit → push → event → Worker → Trestle ingest).
- F. Decide the browser realtime architecture.
- G. Dogfood the Cloudflare Artifacts docs/API.

## Directory

| Path | Content |
| --- | --- |
| `environment.md` | Linode provisioning, SSH, toolchain, costs, reproduction |
| `trestle-clustering-repro/` | Clustered side-effect reproduction: evidence + scripts |
| `strut-surface-validation/` | Strut pinned commit, certification runs, Switchyard-surface probes |
| `cloudflare-artifacts/` | Artifacts contract (docs-level), round-trip scaffolding, credentials needed |
| `realtime-decision.md` | Browser realtime recommendation and rationale |
| `architecture-update.md` | Phase-1 architecture revised from evidence |

## Headline findings

1. **Trestle clustered side-effect gap is CONFIRMED at runtime** (reproduced on
   a real 3-process Raft cluster at tag `v0.1.5` / commit `3c58fd58`). Record
   writes that commit through Raft produce **zero** events, audit facts, or
   outbox/webhook jobs on *any* node — including the node that received the
   write. This contradicts the docs' "side effects remain node-local" claim.
   See `trestle-clustering-repro/`.
2. **Strut current main cleanly implements a disposable step-executor surface.**
   16/16 CTest suites, 11/11 relevant certification scripts, and 11/11 probes
   (HTTPS, SSE streaming, JSON, processes/pipes, git CLI, 16-way concurrency,
   filesystem, HMAC/crypto, cancellation, child-failure recovery, graceful
   shutdown) all pass at pinned commit `c4356bea`. No hard blocker for the
   step-executor design was found. See `strut-surface-validation/`.
3. **Cloudflare Artifacts round trip could not be executed live**: no Cloudflare
   API credentials are available in the environment or on the Linode. The full
   REST/Workers-binding/events/Git-protocol contract was extracted from the
   official docs, and the round-trip Worker + scripts are staged and ready to
   run the moment a `CLOUDFLARE_API_TOKEN` (Workers Paid account with an
   Artifacts namespace) is provided. See `cloudflare-artifacts/`.
4. **Browser realtime recommendation:** a Worker/Durable-Object SSE bridge that
   authenticates the browser and proxies Trestle's SSE with a service token;
   plus a candidate Trestle development item to make user-facing SSE a
   first-class platform feature. See `realtime-decision.md`.

## Gate decision

**CHECKPOINT 0 PASS — ready to design Checkpoint 1** (earned from live evidence,
not assumption). The remaining blocker from the earlier conditional pass is
closed:

- The **Cloudflare Git-plane round trip was executed live** after the account
  was upgraded to Workers Paid: repo create → push → REST reads → fork →
  scoped tokens → git clone/commit/push → `pushed` event via a Queues event
  subscription → normalized → ingested into single-node Trestle with verified
  idempotent re-delivery. See `cloudflare-artifacts/` for observed-vs-
  documented findings and `evidence-live/` for captured outputs.

Still recorded (not blockers):

- **Trestle clustered side-effect gap** is confirmed and is a separately
  planned Trestle development campaign (do not rely on clustered side effects
  until it is fixed); phase 1 uses single-node Trestle by design.
- **Strut** remains the planned step-executor; it depends on a future release
  that includes the current-main surface (release gate per the agreed process).
- `backend_baseline_certification.py` flaked once under load on the 1-vCPU
  Linode; passes in isolation — reproduce before trusting it on constrained
  hardware.

**Checkpoint 1 remains blocked until this handover is reviewed.** Do not begin
Checkpoint 1 on instruction, even though the gate reads PASS.

## Secrets

No secrets are stored in this repository. Passwords, tokens, and credentials
live only on the disposable Linode (`/opt/cp0/repro/.adminpass`) or in the
operator's environment. Repro scripts reference paths on that host and expect
local secrets to be read from the host, never from the repo.