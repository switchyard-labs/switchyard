# C38 Actions usability certification

Local isolated preview, Chromium, 2026-10-04. No production mutation.

The successful automatic C36 run `ba56abf0-8f10-43aa-a0cc-a25425afd3d4`
shows trigger, ref, exact SHA, actor, jobs, step duration, executor and the
Worker's verified immutable source/configuration fingerprint. Go retains the
signed source inspection and rejects mismatched repo/SHA/inspector, invalid
configuration path/digest and configurations exceeding 64 KiB before checks
are persisted. Historical manifests without this newer optional field remain
readable; their UI does not invent verification.

Real failed run `manual-41fa9958f8cf735f4f888c0ce77fee734874e1844dde9e43ec0e836a55a83a0e`
automatically selected the failed job after a successful first job and opened
the failed step with its captured stderr diagnostic. The log region has zero
editable/input elements. Search and pause/follow controls were exercised on
the real C36 capture. Download, reconnect and copy controls are present;
clipboard contents and downloaded file contents are not newly certified here.
Earlier C13/C14 evidence covers SSE cursor recovery and execution controls.

Screenshots: `provenance-logs.png`, `failed-step.png`.

Validation: full `go test ./...`, `go vet ./...`, JavaScript syntax check and
`git diff --check`. Source-boundary tests ensure rejected provenance cannot
publish a passing check. Nift rebuilt Actions from `content/actions.html`.
The Actions script now has a version suffix to invalidate an observed stale
browser asset during upgrades.

Worker Preview/Builds links remain explicitly unavailable until verified setup;
this checkpoint does not certify deployment URLs or real external Agents.

Continuation: real successful capture downloaded through Chromium to action.log.
Read-back verified 114 bytes, exact source SHA 91c8376985668eb6e5bca932dfd0cb56160681eb
and NO_PRIVILEGED_CREDENTIALS stdout. SHA256:
74ac063e29c0c7df7d76c0d0b85ef299863b8605b051b4a3947936c74c232a56.
Copy reported Output copied, but CUA clipboard read-back returned an empty string;
clipboard contents remain UNVERIFIED, not PASS. Actions now also uses one owner/repo
heading instead of adding the shared duplicate context row.
