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
