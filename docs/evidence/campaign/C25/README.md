
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
