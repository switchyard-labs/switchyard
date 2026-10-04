# C46 — Releases, DONE within documented bounds

The deployed schema-8 binary `84552b7` includes manual draft/prerelease/published
releases, editable notes, draft deletion, safe Markdown, canonical list/new/detail
pages, immutable Git target SHAs, source ZIP/tar.gz, R2 assets and `sy` integration.
Manual release creation does not require Actions. Publication rechecks the exact
tag and rejects a moved tag rather than rewriting release identity.

Trestle stores metadata; R2 stores payloads. Assets record name, size, type,
SHA-256, storage key and timestamps. Uploads are bounded to 64 MiB, with 50 assets
per release. Published assets cannot be replaced/deleted. Draft asset deletion
uses a durable deleting reservation. Failed/ambiguous uploads retain recoverable
reservations; recovery reads and verifies the full object before marking it ready.

Actual manual upload/download, same-byte retry, conflicting content, anonymous
published access and `sy` lifecycle/download evidence are retained here. Native
tag event `154b609e-266a-4226-b7bc-74a9008d28e0` succeeded on its first attempt,
attached a 34-byte artifact and published a prerelease at the exact tag SHA.
`actions-native-tag-release.json` records the artifact checksum. An earlier failed
SDK run remained draft and subsequently recovered; this is separate evidence,
not a first-attempt success claim. No preview URL was fabricated.

Concurrency coverage uses real local Git tags and a versioned HTTP store with
barriers forcing concurrent requests to read the same release version:
`TestReleaseConcurrentEditAndPublish`. Exactly one concurrent edit/publish or
publish/publish request wins; the other receives 409. Retrying against fresh state
publishes safely. Repeated publication retains one release, the target SHA and
publication date. `TestReleaseUploadFencesPublicationAndConflictingName` holds an
upload while another upload and publication compete; both are rejected until the
reservation completes. Full Go tests/vet and affected release race tests passed.
These deterministic concurrency tests complement live R2 proof; they are not
labelled as simultaneous production uploads.

Generic CI receives only short-lived upload grants scoped to repository/run/job/
asset/SHA. Release objects are copied only after successful jobs with matching
receipts. The managed `build-assets/` retention rule is 30 days; existing multipart
cleanup is preserved and `release-assets/` is untouched. Readback passed; the
campaign did not wait 30 days to claim physical expiration.

Final C42 covers loaded release list, detail and unsaved new-release form at
1600/1280/1024/768/430/390. Live Linode has the feature but its public demos contain
no release assets; their empty Releases page is the live smoke evidence. Full
authenticated live `sy` dogfood still requires a live credential.
