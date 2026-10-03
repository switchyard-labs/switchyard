# Switchyard schema upgrades

Switchyard owns its collection manifest and `switchyard_schema` version ledger. Each schema release increments the version and records a deterministic manifest fingerprint. Startup validates the full existing schema; existence alone is not compatibility. Extra optional legacy fields may remain, but extra required fields, changed types and changed uniqueness/required constraints fail closed.

Before an upgrade, stop Switchyard background workers and requests and prevent concurrent Trestle schema administration. Back up Trestle storage using its supported consistent backup procedure, Switchyard data/scratch and the encryption key separately. Keep the prior binary and tested restore path. Do not copy an actively written database as a substitute for a consistent backup.

Run the new binary with its normal configured environment and `-schema-plan`. This only reads collections and migration versions. Review the printed new collections and optional added fields. Any required/unique-field addition or type/constraint change stops planning: it needs an explicit data audit/backfill migration, not an automatic destructive acknowledgement.

While the maintenance window remains exclusive, run `-schema-migrate`. It regenerates the plan, rereads every source immediately before PATCH, preserves field identities/defaults, applies additive changes and verifies full schemas before recording the new ledger version. The upstream schema API has no conditional PATCH, so this reread does not protect against a concurrently acting administrator. Never run this command with concurrent schema writers.

A failed migration may have applied an earlier additive change; fix the cause and rerun the plan. The ledger is written only after all collections verify. Restart after successful verification. An older binary refuses a newer ledger. Restoring the prior consistent backup and prior key/binary is the rollback for an incompatible upgrade; automatic schema deletion/downgrade is deliberately unsupported.

Record backup identity, old/new schema version, plan, verification and restore rehearsal in deployment evidence. None of these CLI operations has been run against the shared demo by the implementation campaign.
