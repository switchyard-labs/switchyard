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

This is not an actual public-demo reset. C24 remains PARTIAL until queue
publication shutdown/recovery is exercised against running services. The reset's external
CI/event drain flag is an explicit operator attestation, not an API-verification
claim. Rollback artifacts remain prepared rather than failure-injection certified.

## Actual coordinator drain and recovery

`actions-drain-recovery.json` records shutdown while a real disposable Cloudflare
CI step was running. The old isolated coordinator process group exited, the Worker
continued running, and the restarted coordinator reconciled success. Repeating
the original request returned the same run ID with exactly one run, job and check.
The original Actions definition was restored with its revision guard.

Worker source `6d6c655` was deployed as version
`f9bcb7b7-6b67-48bf-ad09-b0a366d4fa12` to the existing disposable Worker without
changing its capacity or configuration. This includes persisting running-step
admission before awaiting execution; earlier runs that never exposed the running
step were not counted as drain certification.

`workflow-drain-recovery.json` holds the successful Trestle response after a real
child workflow has been committed. Shutdown waits for the durable effect to
return, then exits. Restart completes parent and child with exactly one child
creation and one durable step each. This used one isolated coordinator and a
loopback forwarding proxy, not an Agent or production service. An earlier
clean-runtime preview sharing the test database was stopped before this proof;
its competing workflow runner invalidated the preceding interception attempts.
The restart helper now waits for the previous process group to exit.
