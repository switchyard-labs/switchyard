
### Direct provider HTTP timing — 2026-10-04

Real local reviewer `exe_263c2a64` succeeded against the unchanged repaired SHA
using OpenCode Go / deepseek-v4-flash. Three actual HTTPS exchanges measured
4.531 seconds cumulative request-to-stream-completion and 3.401 seconds
cumulative response-header latency; the model process took 5.448 seconds and
the complete review API took 19.354 seconds. Evidence: `provider-http-review.json`.
The HTTP totals include provider processing **and network transport**, and sum
requests rather than representing exclusive wall time. No prompts, headers,
URLs, response bodies or credentials are recorded by this instrumentation.
Timing currently covers OpenCode Go; other configured providers retain process
timing and are not claimed to have direct HTTP certification. A loopback-only
ephemeral proxy inside the isolated execution sandbox forwards only to the
installed OpenCode registry origin `https://opencode.ai/zen/go/v1`; normal
HTTPS verification remains enabled. Streamed-response and race tests pass.

Clean-flow repair Actions screenshot: `clean-repair-native-ci.jpg`. Actual
Chromium log download contains repaired SHA
`ec1338d5bb27cadae7999a4d8bec427f01b9993d`, 346 bytes, SHA-256
`cb77edefdcaef9dd6ec143d1f94ee12d565d9f320f52560c0f49646537baf678`.

### Execution context snapshot — 2026-10-04

New executions snapshot Attempt, Work and related PR identifiers before the
runner starts. Later Attempt reassociation does not rewrite this provenance;
historical executions are not backfilled. Context reads fail closed before
execution on storage errors. The regression test changes the Work association
during execution and verifies the original Work and both PR identifiers remain
in persisted metadata. Full Go tests, vet and the affected race test pass.
This source change awaits the next coherent deployment and real-run proof.

The immutable Work/Attempt/PR context snapshot is now verified by one real local
review on the Actions-release candidate. `live-context-snapshot-review.json`
records execution exe_414e1480, Work wk_de7b6ee579db014c, one linked PR, role,
provider/model, principal, credential profile, source SHA, sandbox limits and
resource/timing evidence. No findings and no Linode Agent. The complete earlier
multi-role flow remains the source of coding/conflict/repair/queue evidence;
this follow-up certifies the additional context recording without claiming the
older executions had fields that were added later.
