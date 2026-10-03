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
