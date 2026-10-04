# C24 operations evidence

`operator-reset-proof.json` records a full baseline/mutate/reset/repeat command
proof using five disposable local bare repositories and synthetic Trestle SQLite
layout. It ran under host process visibility in `/tmp` on the existing Linode;
production Git, database, configuration and services were untouched. The fixture
contains no real credentials. `operator-reset-fixture.py` is the exact fixture
driver: copy the repository's `scripts/operations/demo-{reset,state,git}.py` and
`scripts/state-backup.py` beneath its sibling `scripts/` directory before running
as an operator with host process visibility. The driver creates new temporary
repositories and state, never chooses live remotes.

Its first attempt refused an open inspection connection. Closing that connection
allowed the unmodified quiescence guard to pass. The successful proof covers exact
Git restoration, removal of extra branches, private full backups, selective state
restoration, credential/non-demo preservation, idempotency keys, exclusion-row
creation and repeat-reset behavior. Separate Go Actions tests verify that a demo
exclusion suppresses rediscovery and cannot suppress a non-demo run. Five Python
tests cover relation scope, shared Work rejection, active/unknown-effect rejection,
similarly named non-demo repos and a transactional SQLite round trip.

This is not an actual public-demo reset or an in-flight drain certification. C24
remains PARTIAL until queue publication, workflow effect and Actions coordination
shutdown/recovery are exercised against running services. The reset's external
CI/event drain flag is an explicit operator attestation, not an API-verification
claim. Rollback artifacts remain prepared rather than failure-injection certified.
