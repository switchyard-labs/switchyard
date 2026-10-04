# C24 operational hardening — partial certification

Implemented and verified locally on 4 October 2026:

- HTTP header/read/idle deadlines (5/30/60 seconds), 32 KiB header ceiling, 16 MiB outer body ceiling and an explicit single-value 8 MiB JSON ceiling. Streaming has no blanket WriteTimeout.
- SIGTERM stops polling contexts, fences new background effects, drains HTTP and waits for tracked workflow/queue/Actions/reconciliation workers. The five-minute deadline leaves durable claims intact if exceeded; it never invents success or clears an outstanding lease.
- Race-enabled fixture proof holds a real persisted queue claim during shutdown, rejects newly admitted work, then verifies completion/release. Existing workflow/Actions recovery suites pass. A live in-flight cloud job/publication shutdown rehearsal is still required.
- Read-only merge previews now clean their scratch directories on failure and fail closed when Git fails without unmerged files. An actual unrelated-history Git fixture verifies both preview methods reject failure and leave scratch empty.
- Optional metrics listener accepts literal loopback addresses only. `/metrics` records HTTP count/error/duration, Trestle and Artifacts transport count/error/rate-limit/header latency, reconciliation/event-consumer outcomes and queue/workflow/Actions status/active-lease counts. No record IDs, bodies, URLs or credentials are retained. `metrics-local.json` is actual local proof; transport timing ends at response headers, distinct from whole request duration.
- `scripts/state-backup.py` uses the SQLite online backup API, closes/checkpoints before hashing, includes Trestle companion state, Switchyard data/scratch/encryption key and an opaque deployment config. Restore refuses an existing target, checks file hashes and database integrity. Run backup only after Switchyard is fully stopped; the script does not stop services for the operator.
- Fresh private restore target started a distinct real Trestle/app on separate loopback ports, authenticated Alice, recovered two visible repositories, profile and ten contributions, and verified key/config byte equality without outputting secrets. See `restore-rehearsal.json`. This proves isolated-demo restore, not a Linode production disaster rehearsal. Original preview was restarted; restored services were stopped.

Remaining C24 certification: Linode restore and in-flight cloud shutdown proof, deployment/systemd resource limits, domain/TLS installation and verification, demo reset execution, full dependency provenance/release upgrade rehearsal and operational scale limits. Do not mark C24 complete from these local results.

Git transport credentials now move from transient `-c http.extraHeader` construction to `GIT_CONFIG_COUNT/KEY/VALUE` in the child environment before process creation. They are absent from OS-visible argv and clone config options; real publication/preview Git tests and the full race suite pass. No stored repository credential is introduced.

### 2026-10-04 deployment/drain observation

Clean Nift/build release 6baddcb was deployed with SQLite integrity-checked private backup `/opt/cp0/switchyard-backup-20261004-6baddcb`; exact version and live HTTP 200 were verified. The first stop command exceeded its 45-second wrapper timeout before any migration/binary mutation; the old service subsequently drained in 133 seconds and restarted. The retry respected the existing five-minute application drain and completed. This is an operating-procedure timeout correction, not a claim of publication/drain certification.

Reconciliation now checks cancellation before entering each repository, while an admitted repository finishes its durable observation/event boundary. Regression coverage proves a pre-cancelled sweep never contacts dependencies; affected race tests pass. It does not cancel a Git command already running, and that operation's bounded timeout still applies. This improvement awaits the next deployment batch.

Authenticated fresh local restore was repeated with a private fixture included in the backup: login/new session, owned credential metadata without secret/ciphertext, owner private-repository read, anonymous private-repository denial, encryption-key and private deployment-config preservation all passed. Source fixture visibility was restored in cleanup. Evidence: restore-rehearsal-private-c25.json. This closes the local authenticated restore subgate; it does not claim a production authenticated restore or deterministic reset/drain certification.
