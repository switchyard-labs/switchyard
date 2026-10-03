# Embedded releases and deployment

Build from the application checkout:

```sh
SWITCHYARD_BUILD_VERSION=0.1.0 scripts/build-release.sh dist/switchyard
dist/switchyard version
sha256sum dist/switchyard
```

The build regenerates Nift output and embeds it. Version, source commit, dirty
tracked-tree marker, UTC build time, and Go version are available through
`switchyard version` or `switchyard --version` without credentials or Git.
The version label is operator supplied; a label does not imply a published
release. Record the binary checksum alongside the deployment record.

Normal UI deployment copies only the binary. Keep the configuration, private
token helper, credentials, and data directory on the host. Unset
`SWITCHYARD_STATIC_DIR` and remove `--static` from the service to use embedded
assets. Preserve an explicit static override only for development.

For an existing installation, first follow the private backup procedure in
`state-and-shutdown.md`. Stop Switchyard, wait for drain completion, install
the verified binary alongside the previous binary, atomically select the new
binary, and restart. Check the version, service log, sign-in, a public repository,
and Actions before removing any old deployment files. Apply schema migrations
only in the documented exclusive maintenance window. Rollback must consider
schema compatibility; restoring an old binary alone is insufficient when a
migration is incompatible.

Suggested systemd settings for a dedicated service account:

```ini
[Service]
User=switchyard
Group=switchyard
WorkingDirectory=/var/lib/switchyard
EnvironmentFile=/etc/switchyard/environment
ExecStart=/opt/switchyard/switchyard
Restart=on-failure
TimeoutStopSec=330
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/switchyard
LimitNOFILE=4096
```

Adapt paths and permissions to the token helper and configured Agent adapter.
Do not copy these settings onto a live service without checking those paths.
Resource limits require measurement of Git and sandbox workloads. Put the
loopback HTTP service behind a configured TLS reverse proxy. Metrics remain
disabled by default; if enabled, bind only to loopback.

Packaging requires Go and Nift at build time. Serving the UI needs neither
Nift nor Node nor templates/public files. **Git is currently still required
for repository operations**, although version reporting does not use Git.
The adopted October 4 clarification permits Git as an external system dependency.
C26/C27 require one Switchyard application binary with embedded UI and version
metadata, deployed by copying that binary and restarting the service; they do
not require embedding or replacing Git. These packaging requirements are met.
Trestle, Artifacts access, and the configured token helper remain required.

This document describes the new deployment procedure. It does not certify a
production service-user migration, release publication, TLS deployment, or
brochure refresh; those have separate evidence gates.
