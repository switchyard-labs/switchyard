# Codex implementation campaign

Baseline: `ff22794`, main clean and equal to refreshed origin/main, 2026-10-03.
Only Switchyard is writable; all reference projects remain untouched. Local checkpoint commits only; no pushes.

The adopted outline requires C0–C4, then a handover before C5. Warden/Gantry Core editor adaptation without an interactive terminal is recorded for C20; collapsible directories and drag/drop moves are acceptance requirements.

## Reconciliation

| Finding | Baseline classification | Evidence / checkpoint |
| --- | --- | --- |
| Product route collision | FIXED BY DEEPSEEK | Explicit /repositories route; existing route tests |
| Strut runtime dependency | FIXED BY DEEPSEEK | Go deterministic adapter, f21261d |
| Menu geometry and design debt | PARTIALLY FIXED | inset changed; product pass still required |
| Filesystem traversal, Git metadata and symlink writes | STILL PRESENT | refs.buildCommit joins unchecked paths; C1 |
| Active same-origin repository HTML/SVG | STILL PRESENT | content handler selects active MIME; C1 |
| Repository authorization and global lists | STILL PRESENT | legacy APIs, grants, profiles, Work/PR; C2 |
| Credential ownership/rotation/session invalidation | STILL PRESENT | global credentials and cookie-only logout; C3 |
| Draft CAS and commit cleanup | STILL PRESENT | two reads, unchecked PATCH, no commit owner guard; C4 |
| Expected SHA and mutable validation | STILL PRESENT | independent clone after head check; C5 |
| Queue leases/recovery | STILL PRESENT | local busy flag and stranded running; C6 |
| Workflow effects/approval/budget | STILL PRESENT | replay completion persistence gaps; C7 |
| Real runner isolation | STILL PRESENT | no context/limits; C8 |
| Schema migrations | STILL PRESENT | existence-only provisioning; C9 |
| Cloudflare Actions | STILL PRESENT | separate CI domain absent; C10–16 |
| Repository/profile/editor/product quality | STILL PRESENT | supplied screenshots and current source; C17–21 |
| Certification gate false positive | STILL PRESENT → addressed in C0 | pipefail/gofmt grep pipeline replaced by captured output |

## C0 — baseline

Go test ./..., go vet ./..., go build ./..., git diff --check and nift build --all passed. Existing coverage is sparse; these are baseline checks, not release certification. Refreshed remote without altering history. Linode switchyard/caddy active, localhost Trestle responded 302. Live browser dashboard still contains the prototype card structure and demo links; product parity remains unfulfilled. Cloudflare-specific runtime certification remains pending; credentials were not printed. Formatting gate now fails reliably on existing drift instead of masking a SIGPIPE failure. No production deployment in this checkpoint.

## Checkpoint ledger

C0 complete: this checkpoint commit records baseline and repairs formatting detection. C1–C4 in progress. C5–C25 pending.

## C1 — contained repository filesystem

Repository paths now reject traversal, absolute/noncanonical paths, encoded aliases, Git metadata, Windows separators/drive syntax and symlink components. Applied at the ref mutation substrate, draft creation, conflict-resolution writes and semantic contract reads. Mutation scratch clones use unique temporary directories; failed commit builds clean up their clones. Raw repository responses are inert text with nosniff and sandbox CSP, including HTML/SVG. Adversarial traversal and symlink tests pass with the full Go suite. No live writes or deployment. Existing generated trailing whitespace from the C0 Nift build was normalized. C5 still owns Git expected-SHA publication semantics.
