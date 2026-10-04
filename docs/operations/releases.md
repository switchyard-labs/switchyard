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

### Release assets (C46 local implementation)

Assets use the signed Actions Worker R2 transport, independently of a CI job.
The canonical upload API is `POST /api/repositories/{owner}/{repo}/release-assets?tag={tag}&name={filename}` with a raw body and explicit Content-Length.
The maximum payload is 64 MiB. The server stages the bytes in a private temporary
file, computes SHA-256, and reserves name/content metadata in the release record
before R2 upload. Binaries never enter Trestle. At most 50 assets attach to a release.

Publication and upload reservation share the release record CAS. Pending/deleting
assets prevent publication. Same-name different-content uploads are rejected;
retrying identical content resumes an interrupted upload. Published assets cannot
be uploaded or deleted. Draft deletion requires removing assets first.

Download/delete use `/api/repositories/{owner}/{repo}/release-assets/{asset-id}?tag={tag}`.
Downloads enforce repository/draft visibility and check remote size/checksum headers.
Draft asset deletion persists a deletion reservation before the R2 effect, then
removes metadata. Interrupted deletion can be retried. Pending uploads must be
retried to completion before deletion, to avoid racing a still-running writer.
Automatic abandoned-upload cleanup remains an outstanding operational gate.

These endpoints are local implementation only until a clean verified deployment
of the app/schema and Worker. The release browser UI, Actions tag automation and
CLI remain separate unfinished gates.
