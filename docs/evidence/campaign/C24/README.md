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

This is not an actual public-demo reset. The reset and all three requested drain/recovery gates are now certified in
isolated fixtures. The reset's external
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

`publication-drain-recovery.json` records native CI on a disposable Attempt,
semantic preview, PR and a real Integration Queue canonical Git push. A
PATH-scoped Git wrapper forwards to the real Git executable and holds only the
successful return of that repository's canonical-main push. Shutdown waits,
then restart reaches queue `done` and PR `integrated`; remote main equals the
recorded publication SHA and exactly one canonical push was observed. The
wrapper records public SHAs only, never command arguments or credentials. The
original launcher and revision-guarded Actions definition were restored.

C24 is DONE for the requested safe-reset fixture, authenticated restore evidence,
three actual graceful-drain/recovery gates and completed operator runbooks.
Public-demo reset remains an available operator procedure, not an executed
production operation. Live destructive rollback failure injection is not
claimed or required for the prepared rollback runbook. These proofs certify
graceful drain; abrupt-crash recovery is covered by the separate queue/workflow
durability tests rather than being inferred from SIGTERM.
