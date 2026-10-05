# MVP hardening readiness — 2026-10-06

The approved CP1–CP7 campaign is implemented, certified and reconciled. No new
release blocker was found. One final independent review is recommended before
normal pushes/public posting. No normal push, tag or release has occurred.
Corrected brochure publication is pending that push; the live brochure is healthy
but still contains the preceding copy.

## Checkpoints

| Checkpoint | Commit | Result |
| --- | --- | --- |
| CP1 isolation/origins/cookies | c152f70 | Complete |
| CP2 immutable queue/review identity | fd48186 | Complete |
| CP3 application publication authority | d67e1d9 | Complete |
| CP4 publication journal/provenance/retry | b3007fd | Complete |
| CP5 bounded medium fixes/schema 16 | 1eb2154 | Complete |
| CP6 claims/enqueue asset version | 0d4448d | Complete; brochure commits below |
| CP7 regression fixtures/evidence | a7154d1 | Complete; final rollout documentation follows |

## Finding disposition

| Finding | Disposition |
| --- | --- |
| F1 fleet cross-tenant disclosure | Fixed: requested organisation intersects current repository-read permission before aggregates. Legacy JSON associations still require a global read/filter; timing/workload isolation is not claimed. |
| F2 mutable queue/review identity | Fixed: source SHA required/persisted; source movement cannot retarget an intent; base movement revalidates the same source. Deterministic and model review evidence identifies source. Legacy unpinned intents fail closed. Enqueue approval is not an independent review approval. |
| F3 missing publication provenance | Fixed for new application publications: durable intent precedes Git, stable fact precedes completion, ambiguous results converge by remote observation. No historical provenance invented. |
| F4 publication authority/role gaps | Fixed at shared application boundary: current permission/policy, typed operation context, isolated Agent writes and validated queue authority. Supported workflow roles preserved; unsupported roles rejected. Independent external Git credentials remain outside this boundary. |
| F5 worker-capacity claims | Corrected: genuine local model-backed evidence retained; current Linode control plane is not sized for routine Agent execution. No production-throughput/scaling proof claimed. |
| F6 archive claims | Corrected: moving branch/tag snapshots, immutable full-SHA release source links, unsupported submodule contents and gitattributes export rules explicit. Full git-archive compatibility deferred. |
| F7 release storage-key exposure | Fixed: public asset DTO omits storage key; internal download storage retained. |
| F8 Pages/cookie boundary | Switchyard authentication hardened with HTTPS host-prefixed cookie and legacy-name rejection. Shared owner origin and arbitrary parent-domain-cookie limitations documented. |
| F9 queue starvation | Fixed: backoff, bounded retry budget, terminal blocked state and explicit requeue; repeated stale/base movement also bounded. |
| F10 reconciliation | Fixed current cursor, deletion observation and timestamp ordering. Polling cannot reconstruct intermediate pushes; identical transition tuples retain deduplicated receipt semantics. |
| F11 Actions | Approved refs enforced by app and actual Worker entrypoint; approval checked before admission/execution. HMAC control-request wording corrected; no response-signing infrastructure claimed. |
| F12 CLI taxonomy | Actual raw sy api HTTP 401 -> exit 1 documented; CLI redesign deferred. |
| F13 missing scratch | Explicit bounded recovery failure; exact current remote match may acknowledge a publication fact without scratch. Descendant inspection/candidate recovery still requires retained local scratch. Distributed recovery deferred. |
| F14 selected lower-severity findings | Origin gap promoted/fixed in CP1; semantic input/rule bounds, Needs Attention, throttle saturation and Pages inherited names fixed; source identity fixed in CP2; semantic claims narrowed. If-Range date handling and brochure Gantry navigation remain lower-priority scope exclusions. |

## Regression and evidence

`docs/evidence/mvp-hardening/regression.json` records the full certification:

- Full Switchyard Go suite, go vet and internal/app + internal/refs race tests pass.
- Full sy suite and relevant Trestle collections/records/store suites pass.
- Actions: 32 tests, typecheck and dry build pass; bundled real entrypoint includes
  fetch/HMAC/receipt behavior, with Cloudflare runtime classes mocked.
- Pages: 17 tests pass, including real Nift owner/project builds and promotion
  concurrency. Brochure rebuilt eight changed pages; app Nift check reports ten
  generated pages up to date. Static product checks include 21-page accessibility
  plus browser-module/ANSI/tree/diff checks.
- Chromium: 60 repository responsive checks; 12 delayed-auth signed-out checks;
  12 Proposal, 12 Pages and six Work/Pages checks; pinned-source enqueue payload.
- Desktop/mobile auth against current disposable Trestle with captured mail:
  signup, login/logout, resend invalidation, verify/reuse, reset mismatch/reuse,
  expired links and session revocation pass. Real email was not resent here;
  earlier user-certified delivery remains the evidence for inbox delivery.
- Full app tests cover Proposal, Work/Attempts, review, semantic/resolver, queue,
  Actions/Pages, releases/archives, permissions and organisation boundaries.
- Real process-death and fault injection cover before push, after push, after
  provenance persistence and before completion; ambiguous push, duplicate and
  conflicting receipt, stale/no-op, base movement, revoked-policy recovery and
  persistent-error retry behavior pass.

No cross-engine browser certification, new paid model-backed campaign, PostgreSQL
runtime certification or production-load certification is claimed.

## Migration and deployments

Reviewed additive schema migration: optional queue source/approval/base fields and
next_retry_at, plus publication_intents and ref_current collections. Production
ledger advanced from 13 to 16. The exclusive SQLite/scratch/config/key/binary
snapshot is `/opt/cp0/switchyard/before-mvp-hardening-20261005-200527` (10,814
manifest files). SQLite integrity passed and pre-existing collection counts were
unchanged during migration. Two legacy queue items remain blocked; claimed and
validating integration effects have no prepared candidate and were retained.
Historical reviews were not reinterpreted/backfilled. Post-start cursor baseline
contains 267 records. No production Git mutation was introduced for certification.
Rollback requires compatible code/state or verified exclusive snapshot restoration;
an old binary alone cannot run against the newer ledger.

Switchyard runtime was built at CP6 0d4448d with version mvp-hardening and SHA256
`d6b39b93c0507efae6c91124209171f0c15a4d0b763fa16652802f1516a6421b`.
CP7 only added test/evidence support; later final documentation does not change
runtime. Production service is active; Trestle binary unchanged/not redeployed.
Worker source is the exact a7154d1 review checkpoint:

- Actions active version: `09ed49a7-4b6a-45ec-8d87-4de4a4d80ae3`, 100%.
- Pages active version: `9cf8ad7f-1d20-47e0-aef6-b0e106db1983`, 100%.

Narrow live verification passes 16 public/private/origin/site checks and matches
all 61 served app assets byte-for-byte. Unsigned Actions dispatch returns 401.
Cloudflare browser-integrity policy rejects default Python User-Agent with 1010;
browser-like probes to Pages and brochure return 200. This edge-policy
qualification is recorded, not hidden as an application failure.

Brochure source/output are committed and clean; CNAME remains docs.switchyard.cx.
GitHub Pages is not republished without the deferred normal output push. Known
host snapshot/staging/inventory helpers are operational artifacts, not unexplained
live-only runtime edits. Existing old binaries/snapshots were retained.

## Exact source checkpoints and repository reconciliation

| Repository/branch | Review HEAD | Ahead/behind at certification |
| --- | --- | --- |
| Switchyard main | a7154d13de4f0397c12db8bb34254c1b8f12428f | 7/0 before final documentation commit |
| sy main | 85f768347ec16c9226ae4e77445b7ea212785c6c | 0/0 |
| Trestle main | 4ceac7a18d46e6355da235655077b5b891758e40 | 0/0 |
| Brochure stage | 47ef33f5f833595cb58b22302a37d45a64d1c32a | 1/0 |
| Brochure output main | 5e1134b45ccf5371860f9b072a44a248fdf9f919 | 1/0 |

All five repositories were fetched and inspected for status, branch, upstream,
HEAD, recent history and untracked files: clean, none behind. Switchyard's final
HEAD is the documentation-only descendant reported in the final chat response;
review code at the exact checkpoint above. No code change followed certification.
Two generated brochure whitespace-only lines are cosmetic and retained.

After independent review and explicit push authorization, normal commands are:

```sh
git -C /home/nick/Repositories/switchyard-labs/switchyard push origin main
git -C /home/nick/Repositories/switchyard-labs/switchyard-labs.github.io push origin stage
git -C /home/nick/Repositories/switchyard-labs/switchyard-labs.github.io/public push origin main
```

sy/Trestle have no new commits to push. No known technical blocker to normal
pushes; independent review and authorization remain the deliberate gates.

## Retained limits and final review

New publication facts converge when the effect remains observable (exact remote
SHA, or ancestry with retained scratch). An external overwrite/revert before
observation can require operator reconciliation; no invented success/attribution.
Same-host scratch, narrow JSON field_equals semantics, periodic-ref fidelity,
external Git authority, shared Pages owner origins, in-memory throttling, archive
export limitations and raw CLI exit behavior remain explicit MVP limits.
DMARC monitoring-only and no independent recipient-header inspection remain prior
auth-report qualifications; this campaign does not broaden mail work.

Send the compact handover in `docs/plan/mvp-hardening-independent-review.md` for one
final independent review. No independent review has yet certified these fixes.
