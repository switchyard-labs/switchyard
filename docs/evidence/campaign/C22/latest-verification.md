# Latest C22 verification

3 October 2026 UTC, continuation into 4 October Melbourne time. C22 remains in progress; this is evidence, not a completion certificate.

The live public Alice profile uses a 300 × 300 pixel desktop avatar. Six actual viewport widths (1600, 1280, 1024, 768, 430, 390) passed document overflow checks for the refreshed profile, dashboard, editor and public demo-conflict repository. Dashboard hamburger and Ready for review right edges both measured 1241 pixels at a 1280-pixel viewport. The public repository loaded actual Artifacts content while signed out.

Native Ctrl+D selected multiple occurrences and typing changed both; Ctrl+Z restored the original source. The explicitly authorized Ctrl+Shift+S test saved the unchanged `src/routes.mjs` draft on the disposable local `editor-c20` branch without a Git commit. A subsequent native edit/undo cleared the dirty indicator on returning to the saved baseline.

Social/profile, stars, follow, contribution, activity, legacy routing and operator-import Go regression tests passed. Static product certification passed (17 pages, JavaScript syntax, formatting, whitespace and static accessibility). This does not establish Firefox/WebKit parity.

## Latency observations

These are diagnostic samples, not percentiles or a load test:

| Request | Location | Result |
| --- | --- | --- |
| Alice profile | Linode loopback | HTTP 200, 0.002115 seconds |
| Alice contributions | Linode loopback | 0.320171 seconds |
| demo-conflict overview | Linode loopback | HTTP 200, 0.410062 seconds |
| Alice profile | Public network from agent host | HTTP 200, connect 0.284976 seconds, first byte 0.566783 seconds, total 0.566857 seconds |

The Trestle `/health` probe returned 404 in 0.000474 seconds; that is **not** a valid database benchmark. Profile processing is fast in the measured sample; contribution processing needs further work. Authorization and contribution assembly currently perform repeated per-record repository/Work metadata queries. Git overview also makes remote Artifacts/ref/history calls. C23 should instrument and benchmark these paths separately, preserve access filtering, and avoid shared permission-insensitive response caches. No measurement here establishes the user's internet speed or GitHub-relative latency.

The separate architecture exploration is committed as `b3ae046`. It recommends preserving Work/Attempts and the fenced queue, with immutable intent/evidence manifests and a task-centered journey; it does not authorize implementation of speculative proposals.
