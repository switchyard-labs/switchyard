# C23 performance work in progress

The contribution endpoint made 160 metadata reads for 40 attributed and 80 unrelated updates in the isolated HTTP fixture. Filtering actor/date first and memoizing repository metadata/permission decisions for one request reduces that to one. A second request after changing the repository to private returns zero contributions: permission decisions never survive requests. Exact actor, private-data and race checks pass. This is measured query reduction, not a claim about Trestle database speed.

`local-git-workspaces.json` compares scratch clones with one refreshed bare mirror and isolated detached worktrees at 1/3/10/25 concurrent tasks. Exact base and resulting trees/text agree, and the source branch is unchanged. The four probes cover Git browse, merge preview, diff comparison and integration candidate creation. They do not measure the semantic engine, remote provider, publication fences or complete queue latency. Those end-to-end gates remain outstanding.

For this small local fixture, mirrors improve single-task setup but serialized worktree creation/removal makes 25-way batches slower (browse 233ms versus 64ms; preview 240ms versus 79ms). Do not replace production scratch clones from these results. A remote fixture and workload history are needed before deciding whether network savings justify a mirror design.

The script creates only its own temporary repositories, pins detached workspace bases, compares candidate results and makes no remote push. It removes its owned temporary directory on exit. C23 is not complete.

Deployment sample: the real Linode contribution endpoint returned HTTP 200 in 0.048565s on loopback after the change, versus the earlier 0.320171s sample. These are individual observations, not latency percentiles or a controlled network benchmark. The update has an integrity-checked private SQLite/binary/environment/key backup at the existing host; no schema change was needed.

4 October: upstream REST/raw/bounded-read/token failures now carry typed safe codes. Repository and Git-host HTTP handlers preserve classification: 404 repository_not_found, 429 upstream_rate_limited (with Retry-After), 502 artifacts_auth_failed/artifacts_unavailable, 503 service unavailable, 504 upstream_timeout. Response bodies and token-helper output are not exposed. Protocol regression tests exercise all four upstream paths and verify no sensitive upstream body disclosure; HTTP regression verifies wrapped rate-limit propagation. This does not yet classify an opaque Git subprocess transport failure as an HTTP rate limit, nor certify the remaining full C23 performance matrix.

Repeated real isolated-demo HTTP samples (7 per endpoint, all 200) are recorded in local-http-20261004.json. Median ms: overview 2613.55, source 655.07, history 661.85, Actions 5.95, contributions 7.57, queue listing 1.82. This is from the benchmark host, not Nick's browser/network. No p95 is claimed for seven samples. Queue listing is not queue integration.

Overview stage samples in overview-stages-20261004.json: warm coordination 0.37–0.83ms, REST metadata 340–406ms, cached token 0ms, Git references 1372–1601ms, REST history 574–651ms. First sample had metadata 6748ms and token 988ms. Response Server-Timing exposes only these named durations. No evidence here establishes Trestle as the source of the remote delay. Preview/semantic/integration and whole-page load coverage remain outstanding.

Twelve-sample follow-up (`http-12-samples-20261004.json`), all HTTP 200:

| Surface | Median ms | Nearest-rank p95 ms |
| --- | ---: | ---: |
| Repository overview | 2845.98 | 4025.78 |
| Source | 696.29 | 845.62 |
| History | 601.01 | 5668.48 |
| Actions list | 11.69 | 13.73 |
| Action detail | 10.40 | 12.07 |
| Profile | 4.18 | 5.35 |
| Contributions | 12.38 | 19.54 |
| Queue listing | 2.47 | 3.17 |

For twelve samples nearest-rank p95 is the maximum; it is an honest small-sample
upper observation, not a reliable population-tail estimate. All raw values and
Server-Timing stages are retained. The benchmark includes response bodies but
excludes browser rendering and Nick's network. C36 separately measured remote
push→Go event 0.83s, push→captured SSE frame 1.11s, push→CI success 32.26s.
The 75-repository reconciliation safety-net observation was 415.82s and is a
known serial-scan scaling limitation. None of these results supports blaming
Trestle for the remote repository browse delay or adopting a mirror prematurely.

Twelve real disposable Attempt previews also passed: median 13160.72 ms, nearest-rank p95 18714.59 ms, range 11828.96–18714.59 ms. Each request performs textual merge preview and semantic validation of the merged tree; this measures their combined end-to-end cost, not independent semantic compute. No canonical publication occurred. Raw data: `preview-12-samples-20261004.json`.

The preview handler previously performed two complete remote checkout paths, one
for textual merge and another for semantic validation. Both now share one merged
checkout, with caller-owned cleanup and no mirror/cache or publication change.
Real Git regressions verify merged contents, conflict filenames containing spaces,
scratch cleanup and unchanged remote refs. Twelve follow-up requests all passed:
median 6696.15 ms, nearest-rank p95 10446.97 ms, range 5979.85–10446.97 ms.
That is a 49.1% lower observed median and 44.2% lower upper observation than the
preceding run, not a controlled population estimate. Remote Git transport still
dominates this path. Raw results: `preview-single-checkout-12-samples-20261004.json`.

Continuation measurements: twelve authorized PR-list API requests all returned
200 (median 2.92ms, upper observation 4.44ms). Embedded editor-shell requests
returned 200 (median 0.32ms, upper observation 1.98ms). Twelve real Chromium
reload-to-visible CodeMirror observations took median 7341.5ms, upper observation
11832ms, minimum 7117ms. These include CUA observer/protocol overhead and remote
Git work; they are not browser Performance API measurements or Nick's network.
Raw data: pr-editor-http-12-samples-20261004.json and editor-browser-12-samples.json.

Preview Server-Timing now separates remote Git merged-checkout and local semantic
validation. Twelve prepared C41 Attempt previews all returned ok (median 6384.67ms,
upper observation 13147.95ms). This fixture has no semantic contract: its tiny
semantic-stage time measures the missing-contract fast path, not representative
contract validation. Do not use that number to claim semantic performance complete.
Queue publication timing and representative contract validation remain open.

The genuine two-rule semantic fixture is now measured, not the missing-contract
fast path. Twelve repaired-tree previews passed: end-to-end median6881.50ms,
nearest-rank p95/max11554.50ms, min6108.78ms. Server-Timing Git-preview median
6336.24ms (min5743.09/max8753.06); actual local semantic validation median0.133ms
(min0.084/max0.421). The remote checkout dominates this small JSON contract.
This is not evidence for large contracts or Nick's internet. Raw samples:
semantic-contract-preview-12.json. Durable queue stage timing is now implemented;
repeated actual publication samples remain open.

### Queue measurements — 2026-10-04 (partial)

Eight exact-SHA integrations completed with the genuine two-rule semantic
contract. Median Git revalidation was 8.159 s; publication 4.018 s; semantic
validation 0.139 ms. Raw samples and nearest-rank p95/min/max are recorded in
`queue-semantic-timing-partial.json`. At n=8, nearest-rank p95 is the maximum.
The ninth source passed push but native CI failed twice before enqueue with
`RPCTransportError: RPC session was shut down by disposing the main stub`.
The first interruption was a 600-second disposable Git credential expiry;
renewal resumed existing state without duplicate Work/Attempts. The CI retry
kept the exact source SHA. The requested twelve-sample queue matrix remains
partial; the provider failures are not represented as queue samples.
