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

**CHECKPOINT 0 CONDITIONAL PASS.** The fundamental assumptions held: Trestle
clustering has a real, now-reproduced defect (good — Switchyard should force
its fix), and Strut main is capable of the experimental step-executor role
(good — no architectural incompatibility). Two bounded items remain:

1. **Cloudflare live exercise** (E and G): requires a Workers Paid account +
   `CLOUDFLARE_API_TOKEN` with Artifacts permissions and a namespace. The
   round-trip code is staged under `cloudflare-artifacts/` and runs unchanged
   once the token is exported. This is the only hard blocker.
2. **Reproduction note:** the `backend_baseline_certification.py` script failed
   once under concurrent load on the 1-vCPU Linode, then passed consistently in
   isolation. Treat as suspected timing sensitivity on constrained hardware,
   not a confirmed Strut defect; re-run before relying on it.

Do not begin Checkpoint 1 until the Cloudflare credential gap is resolved and
the live round trip has been exercised.

## Secrets

No secrets are stored in this repository. Passwords, tokens, and credentials
live only on the disposable Linode (`/opt/cp0/repro/.adminpass`) or in the
operator's environment. Repro scripts reference paths on that host and expect
local secrets to be read from the host, never from the repo.