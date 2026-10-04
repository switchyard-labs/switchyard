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

### Release assets (C46)

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
When R2 upload succeeded but metadata completion was interrupted, a maintainer
can use `POST /api/repositories/{owner}/{repo}/release-assets/{asset-id}/recover?tag={tag}`
or **Recover attachment** in the draft UI. Recovery reads the actual immutable
payload and verifies its size and full SHA-256 before completing metadata. It
does not require the original local file. A missing object requires retrying the
original upload; corrupt bytes remain pending. Automatic abandoned-upload
cleanup remains an outstanding operational gate.

The foundation API/schema and signed R2 Worker transport are deployed. The UI
has local browser publication/download proof, and `sy release` has real API/R2
proof. Recovery requires the next application deployment. Actions tag automation
and final live browser/CLI certification remain unfinished gates; consult the
campaign ledger for the deployed commit and evidence.

## Actions release automation candidate

The local candidate accepts an explicitly approved tag-only definition:

```js
export default {
  refs: ['refs/tags/v1.0.0'],
  release: {publish: true, prerelease: false},
  jobs: [{
    id: 'build',
    steps: [{id: 'package', command: 'make package', timeout_ms: 120000}],
    assets: [{name: 'program.zip', path: 'dist/program.zip'}]
  }]
};
```

Approval records the policy and output paths with the definition revision.
Names are unique across jobs, at most 180 ASCII characters; paths must be safe
relative paths outside `.git`. There are at most 50 outputs of 64 MiB each.
The trusted uploader runs after the last command of each build job succeeds.
Node.js with fetch is required in the build image. It streams opened regular
files whose descriptor remains within the checkout. The build receives only
short-lived grants scoped to one run/job/output/SHA, never the control secret
or R2 credentials. Upload grants are redacted from captured logs.

Payloads first land under the existing R2 bucket's `build-assets/` prefix.
Successful receipts are read from R2 metadata and matched to the approved
configuration and exact source SHA. Reconciliation owns a deterministic
automation release ID; it refuses to adopt a manual draft. It reserves all
output names with release CAS, streams verified outputs into immutable release
objects, and publishes only after every configured asset is ready and the tag
still points at the original commit. Copies interrupted after reservation remain
pending and can be retried. Failed or cancelled builds retain a draft without
attaching failed output. Current maintainer authority is required during
reconciliation. Manual releases remain independently available.

This candidate passes Go/Worker tests, affected races, vet, typecheck and Worker
dry-run bundling. Native tag-to-release proof, deployment, and the staging R2
object cleanup policy remain outstanding. No automated-release deployment is
claimed by this documentation. Definitions currently require tag-only refs;
main-branch CI should retain its existing definition until a coherent release
automation rollout is selected.

### Native CI sandbox recovery (4 October 2026)

The Actions-output candidate `25361e0` is deployed to the existing disposable
Actions Worker and isolated local app, but not Linode. Its real tag build failed
before output upload with `RPC session was shut down by disposing the main stub`.
The associated release stayed a draft with no assets, as required. The container
API showed one late-started runner still occupying the existing one-instance cap.

A version- and source-hash-pinned compatibility patch guards the Sandbox 0.12.1
`onStop` disconnect when RPC has no WebSocket connection. This matches the symptom
reported in https://github.com/cloudflare/sandbox-sdk/issues/928; the report is a
hypothesis, not proof of our root cause. All other SDK cleanup remains intact.
Installation/build fails closed on dependency drift; remove the patch after a
reviewed upstream fix. Deployments must run the postinstall/prebuild patch.

Signed POST `/sandbox-recovery` accepts an operator-selected instance name/id,
failed or cancelled run and `inspect`/`destroy`. It checks the allowlisted repo,
terminal Workflow, matching step label and exact namespace-derived instance ID
before any destroy call. Inspect never starts or destroys a runner. This is an
operator recovery action: step-label matching is not a persisted ownership receipt,
so independently confirm the sole orphan in the container API before destruction.
No application deletion, instance-limit increase or new paid allocation is needed.
The real recovery and successful native release proof remain unverified.
