# Final Switchyard reconciliation — 2026-10-06

The auth/UI scope is closed. This pass added documentation, test coverage and
certification evidence only; it did not deploy another runtime, push, tag or
create a release. Ready for a normal push of the reconciled commits, subject to
explicit user authorization. Remote references were freshly fetched for all
five repositories; all branches have zero commits behind their comparison ref.

## Repository inventory

| Repository | Branch / comparison | Local checkpoint | Ahead after final checkpoint |
|---|---|---|---:|
| Switchyard | `main` / `origin/main` | The commit containing this report; resolve with `git rev-parse HEAD` | 136 |
| sy | `main` / `origin/main` | `85f768347ec16c9226ae4e77445b7ea212785c6c` | 9 |
| Trestle | `main` / `origin/main` | `4ceac7a18d46e6355da235655077b5b891758e40` | 3 |
| Brochure source | `stage` / `origin/stage` | `2f36c0757a7d2063ead6b8b5fb1d18646a99f677` | 4 |
| Brochure output | `main` / `origin/main` | `f8584bdeddfcdfd3e1deef554301ada64455990c` | 5 |

The final response records the full Switchyard commit SHA after committing this
report. `repositories-before-final-commit.json` retains the preceding complete
inventory: status, upstream, HEAD, fresh origin SHA, unpublished commit list,
recent history, range statistics, whitespace findings and connectivity checks.
The brochure source has no upstream; the proposed push sets `origin/stage`.
The nested output is a separate repository and must be pushed separately.

The output's remote `Create CNAME` commit was merged normally, preserving
`docs.switchyard.cx`. The source documentation and generated output received
small final whitespace corrections. No histories were rewritten. Generated
Nift output still has one whitespace-only line in each of the cumulative app
and brochure-output diffs (`public/index.html:46`, `docs/internals.html:108`);
these do not block Git push or change behavior. Current final patch whitespace
checks pass. Git connectivity checks pass in all five repositories; existing
unreachable Trestle objects were preserved. No unpublished blob exceeds 100 MB.

All five working trees are clean after the final checkpoint. Historical
untracked campaign evidence has been retained and committed with a provenance
manifest; it is not presented as new certification. Python bytecode caches
remain on disk and are ignored. No `.git` deletion, destructive cleanup,
force push, unexplained file removal or secret publication was performed.

## Final regression results

| Check | Result / scope |
|---|---|
| Switchyard, sy, Trestle | Full `go test ./... -count=1` and `go vet ./...` pass |
| Affected race coverage | Switchyard app/agent/actions/maildelivery/refs, sy api/commands/config, and Trestle account-auth records pass |
| Product workflow tests | Proposal → Work/Attempts → Actions → Review → PR/Queue → canonical integration → frozen Pages output local dogfood passes; semantic contracts, resolver scope/symlink failures, queue recovery, releases/archives and public/private boundaries covered in full Go suite |
| Actions Worker | 30 tests, typecheck, dry-run build pass |
| Pages Worker | 16 tests and dry-run build pass |
| Editor / generated app | Editor bundle and 10-page Nift rebuild pass without changing runtime assets |
| Static product certification | JS syntax, 21-page accessibility checks, Go formatting, primary-color rules, handover and working-patch checks pass |
| Additional test scripts | ANSI, diff surface and repository-tree Node checks; five operations Python tests pass |
| Repository shell browser | 60 checks at 1600/1280/1024/768/430/390 px pass; local assets/live public reads/isolated maintainer fixtures |
| Proposal / Pages / Work browser fixtures | 12 Proposal, 12 Pages and six Work/Pages responsive checks pass |
| Anonymous no-flash regression | Permanent test: six workspace surfaces at desktop/mobile, 12 checks pass while auth is delayed and after HTTP 401; private markers never rendered |
| Live anonymous check | Six deployed workspace surfaces pass delayed-auth no-flash checks |
| Auth browser | Real disposable Trestle/captured mail, desktop/mobile signup/resend/verify/reset/new login/logout/old-session rejection/link arrival validation pass |
| Deployed product smoke | 15 checks pass, including authenticated session precondition, product navigation and host-only cookie isolation; read-only smoke, not new production mutations |
| Deployed Pages | 14 serving cases plus HEAD/ETag/range pass; owner/project/preview and scoped 404s |
| Deployed sy / archive | 11 authenticated/anonymous CLI checks pass; public ZIP downloads successfully |

The CLI raw `sy api` escape hatch rejects private account settings with HTTP 401
and process exit 1. It does not classify that raw error as typed-command auth
exit 3. This existing exit-code inconsistency is recorded explicitly; no CLI
feature correction was added in this documentation/test-only pass.

The browser checks use Chromium, not an unperformed cross-engine matrix. Live
product checks do not claim a fresh paid provider run or a new production
integration/deployment. Controlled-clock token expiry tests do not claim waiting
for real emails to expire. User-confirmed actual Mailgun delivery, resend/reuse
and reset/login results remain separately recorded in the auth report. The
production disposable account was already removed; this pass used a local
disposable Trestle instance for captured-mail flows.

## Live/local reconciliation

- Linode Switchyard is active at runtime commit `d462e69`; its binary SHA-256
  `b883d75ace0763ff939603a61c914e2c5043f672ab93686db7b6febf763e349b`
  matches the retained local binary. Later app commits change docs/tests/evidence.
- Linode Trestle is at `4ceac7a`; binary SHA-256
  `01bf418374d3637256220e318b44618f4d6add0fb382f3921f3c0d54e082489f`
  matches the retained local binary.
- All 64 tracked served HTML/assets match deployed HTTPS bytes. The live process
  uses embedded assets with no static-directory override. Historical host asset
  directories/backups remain preserved and are not the served source.
- Installed AppArmor profile matches local; restricted sandbox executable and
  protected environment permissions were inspected without disclosing secrets.
- Pages Worker version `5a861b5e-8c8b-452e-91d7-7c129f9c1f7e` and Actions Worker
  version `1999cbae-0d3b-485a-8bd8-fff117c25ae0` match recorded activations at 100%.
  Worker runtime sources/config/dependencies have not changed since activation;
  this is not a byte-for-byte remote Worker bundle verification claim.
- Mailgun SMTP requires STARTTLS; sender SPF/DKIM and monitoring DMARC are
  published. Recipient Authentication-Results headers were not inspected.
- No accidental live-only runtime fix or omitted local runtime change was found.
  The brochure remains pending its normal GitHub push; its newly generated docs
  are not claimed to be already published.

Evidence is in `docs/evidence/final-readiness/`, the refreshed auth and repository
shell evidence, and `docs/operations/auth-hardening-report.md`.

## Intentional limits and push decision

Standalone collection tokens remain the supported adapter; replication fails
closed and optional PostgreSQL runtime qualification was not available. Reset
mail delivery is nondurable and throttles are in memory. Verification remains
informational, legacy missing-email repair is administrative, and snapshot
restores require incident token invalidation. DMARC is monitoring-only. Pages
sites share an owner origin and generators must honor the configured base path.
Additional provider scaling, cross-engine, feature, queue/storage and mail
campaigns remain outside this pass.

No Git ancestry, cleanliness, asset parity or regression failure prevents a
normal push. The documented raw-API exit-code inconsistency and generated blank
line whitespace are nonblocking limitations, not unreported passing checks.
Remote state may change after this snapshot; review a fresh fetch before pushing.

## Commands only after authorization

```bash
git -C /home/nick/Repositories/go-projects/trestle-cv/trestle push origin main
git -C /home/nick/Repositories/switchyard-labs/switchyard push origin main
git -C /home/nick/Repositories/switchyard-labs/sy push origin main
git -C /home/nick/Repositories/switchyard-labs/switchyard-labs.github.io push --set-upstream origin stage
git -C /home/nick/Repositories/switchyard-labs/switchyard-labs.github.io/public push origin main
```

STOP here. These commands have not been executed. No tag or GitHub release is
part of this authorization boundary.
