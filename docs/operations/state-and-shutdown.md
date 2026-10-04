# State, shutdown and metrics

Use SIGTERM for planned stops. Configure systemd `TimeoutStopSec=330` so Switchyard's five-minute drain can finish; do not use SIGKILL for routine upgrades. HTTP stops accepting traffic, pollers stop admitting effects and active durable work drains. If a drain times out, retain the database and scratch directories and use lease/reconciliation recovery before retrying publication. Never delete claims as an upgrade shortcut.

Back up after `systemctl stop switchyard` has completed, using a private fresh target:

```bash
python3 scripts/state-backup.py backup --trestle-db /path/to/trestle.db --data /path/to/switchyard-data --config /path/to/deployment.env --target /private/new-backup
python3 scripts/state-backup.py restore --source /private/new-backup --target /private/new-restore
```

Restore includes `trestle.db`, its companion `trestle-state/`, Switchyard `data/` (including credential key and durable scratch), and `deployment.env`. Configure the replacement Trestle/app to those restored paths before starting. Verify authentication, authorized repository identities, workflow/queue state and credential decryption. Artifacts remains remote Git truth; this snapshot is coordination/local state, not a backup of Cloudflare Git or R2 objects. Retain backups privately and apply your retention/encryption policy.

Set `SWITCHYARD_METRICS_LISTEN=127.0.0.1:9090` to enable the separate loopback `/metrics` listener; it is off by default. Do not proxy this listener publicly. It exposes aggregate operational state, not application records. HTTP metrics include full handler duration; upstream transport timings include response headers only. Counter resets follow process restarts. Poll at a moderate interval because state counts read Trestle collections.

Public deployment still needs a reverse proxy with HTTPS, domain/certificate ownership, suitable upload/time limits and a systemd dedicated service user with CPU/memory/task/file limits. TLS readiness is not certification that the current IP-address demo has TLS. Avoid claiming a domain is secured before testing its certificate and redirects.

## Curated demo reset

`scripts/demo-reset.sh` now captures and restores a private baseline for exactly
`demo-basic`, `demo-agents`, `demo-conflict`, `demo-semantic`, and `demo-workflow`.
Upgrade to schema 8 before capturing the baseline. It preserves credentials,
profiles, ownership, repository configuration, non-demo records, release metadata
and payloads, audit history, workflows and global event cursors. Shared Work with
any non-demo repository rejects the operation. Workflows must be terminal; demo
queue, Agent and Actions effects must be terminal. Do not retry retained historical
demo workflows against a reset baseline without reviewing their parameters.

Before capture or reset, finish/cancel external demo CI, pause demo event ingestion,
stop Switchyard gracefully and stop Trestle. Supply a private JSON PID manifest
containing the actual stopped `app` and `trestle` PIDs. The tool checks those PIDs
and all host processes for an open database/WAL/SHM; it requires host process
visibility and fails closed when that cannot be checked. Disable automatic service
restarts and keep other operators from restarting writers during the operation.

Supply a private JSON mapping of the five names to their token-free HTTPS Git
URLs. Pass short-lived authorized Git headers through environment configuration,
never a URL or command argument. Protected refs still enforce server permissions;
the reset does not bypass those rules. The external-effects flag is an operator
attestation, not an automatic Cloudflare drain check.

```bash
scripts/demo-reset.sh baseline --trestle-db /private/trestle/trestle.db --stopped-service-manifest /private/stopped-pids.json --remotes /private/demo-remotes.json --baseline /private/new-demo-baseline --external-demo-effects-drained
scripts/demo-reset.sh plan --trestle-db /private/trestle/trestle.db --stopped-service-manifest /private/stopped-pids.json --remotes /private/demo-remotes.json --baseline /private/new-demo-baseline --external-demo-effects-drained
scripts/demo-reset.sh reset --trestle-db /private/trestle/trestle.db --stopped-service-manifest /private/stopped-pids.json --remotes /private/demo-remotes.json --baseline /private/new-demo-baseline --backup /private/new-reset-backup --data /private/switchyard-data --config /private/deployment.env --external-demo-effects-drained
```

The reset makes a full integrity-checked state backup and five pre-reset Git
mirrors before publication. Each repository uses an atomic push with exact
force-with-lease checks for every changed/deleted ref. There is no cross-repository
Git transaction: a private journal allows resuming the same interrupted reset
with the same arguments and backup directory. A concurrent ref change rejects
overwrite. Coordination records restore in one exclusive SQLite transaction
only after every Git repository matches the baseline. Baseline and schema
identity checks reject accidental reuse against a different database or baseline.

Completed demo Actions removed by reset receive durable run exclusions so normal
discovery cannot recreate them. External manifests/logs remain retained. The
full-state restore snapshot is `BACKUP/snapshot`; reset journals and mirrors sit
outside that snapshot so its integrity manifest remains usable. Source and
backup/restore target trees must not contain one another. Keep backups private.
Check all five Git refs, SQLite integrity, credentials, public access and Actions
views before resuming Trestle, Switchyard and event ingestion.

The isolated operator-command proof restored five local bare repositories and
synthetic Trestle state, preserved credentials/non-demo activity and idempotency,
created an Actions exclusion, and repeated without changing the result. It did
not reset the public demo. C24 remains PARTIAL pending actual in-flight queue,
workflow and Actions shutdown/recovery proofs.
