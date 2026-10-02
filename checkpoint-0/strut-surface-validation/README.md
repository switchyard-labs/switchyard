# Strut surface validation for Switchyard

**Question:** can current Strut main implement a disposable Switchyard
step-executor cleanly, and what specifically prevents it if not?

**Verdict:** **Yes for the experimental phase.** Every Switchyard-required
surface passed on current main at the pinned commit. No hard blocker. The
step-executor architecture remains recommended. A small number of
ergonomic/stdlib gaps were found (none blocking) and are classified below.

## Pinned commit

- Repo: `github.com/strut-labs/strut`
- HEAD: `c4356beaf2ae5e87bb90ec9d0780fca00ef96a7e` (2026-10-02 19:04:35 +1000)
- Built from a clean checkout on the Linode with CMake/Ninja, Debug, per
  `BUILDING.md`.
- Compiler: `/opt/cp0/strut/build/strut` (reports `strut 0.0.3` string; this is
  main codegen).
- Not modified.

## Certification/tests rerun

| Suite | Result |
| --- | --- |
| `ctest --test-dir build` (16 unit binaries) | **16/16 passed** (63.7s) |
| Dogfood certifications (11 of the 26, Switchyard-relevant) | **11/11 passed** |

Certifications run: backend baseline*, HTTP client, HTTP client streaming, HTTP
lifecycle, HTTP request cancellation, cancellation, process lifecycle, process
cancellation, HTTP NDJSON, HTTP static file, HTTP WebSocket.

\* `backend_baseline_certification.py` failed **once** under concurrent load
(the 500-sequential-requests assertion), then passed consistently in
isolation. Suspected timing sensitivity on the 1-vCPU box; re-run before
relying on it. Not yet confirmed as a real defect.

The 16 ctest suites + certifications + probes were run directly on the Linode
and are reproducible from `strut-surface-validation/`.

## Switchyard-surface probes

Eleven throwaway probes (outside the Strut repo) exercised the exact surface a
step-executor needs. All pass at the pinned commit:

| Probe | Surface | Result |
| --- | --- | --- |
| `probe_https` | HTTPS GET + custom headers via `http_get`/`http_request` | PASS (200, 713 bytes) |
| `probe_json` | parse/stringify/round-trip, nested access | PASS |
| `probe_process` | `exec` stdout/stderr/exit, `process` streaming reads, `pipe_exec` | PASS (exit 7, stream "ready"+"done") |
| `probe_git` | full git CLI lifecycle: init/add/commit/log | PASS |
| `probe_concurrent` | 16 concurrent independent steps via threads+channels | PASS (sum 240, 16 files) |
| `probe_fs` | make_dir/write/read/exists/copy/move/ls/remove_all | PASS |
| `probe_hmac` | HMAC-SHA256 RFC 4231 vector, SHA-256, base64/base64url, secure random | PASS |
| `probe_cancel` | cancellation_source/token, cross-thread cancel | PASS |
| `probe_child_failure` | child exits 3; parent observes and continues | PASS |
| `probe_sse` | incremental SSE consumption via `http_request_stream` | PASS (5 chunks, 174 bytes) |
| `probe_shutdown` | SIGTERM → `wait_for_shutdown_signal()` → clean exit 0 | PASS |

Probe sources are committed under `strut-surface-validation/probes/`.

## Not validated here

- The independent regression suite (`strut-labs/strut-regression-suite`) was
  not run in this phase (separate repo, heavy, and its result is not needed to
  answer the step-executor question). ctest + certs + probes are the
  "relevant current certification/tests" for this surface.

## Concrete deficiencies and classification

Per the brief, each deficiency is classified:

1. Switchyard application concern
2. reusable Strut package concern
3. general Strut standard-library/runtime/language concern
4. unnecessary because the architecture can avoid requiring it

| Deficiency | Classification | Notes |
| --- | --- | --- |
| `examples/one-binary-todo`, `examples/static-site`, `examples/embed/basic` fail to compile: `embed_dir()` returns `map<string,string>` but the `<map>` standard module is not auto-included | 3 (minor) | Confirmed on the pinned commit; the error message tells you exactly how to fix (`include <map>;`). Examples are not in the certified manifest. |
| No string substring/search/contains API (`.find`/`.contains` unavailable; only `==`, `.at()`, `.size()`, concatenation) | 3 (genuine gap) | Forced workarounds in the SSE probe. A real text-processing gap for an orchestration engine that parses log/SSE/webhook text. Candidate general Strut stdlib work. |
| `x := a + b` cannot infer type for `string + string`; explicit `string x := ...` required | 3 (ergonomics) | Type checker does not infer the result of `+`. Minor friction, workaround is trivial. |
| `now_ms().to_string()` is invalid; must use free function `to_string(x)` (while `bytes.to_string()` exists as a method) | 3 (ergonomics) | Confusing asymmetry; minor. |
| `channel.receive()` returns an optional that must be dereferenced (`*got`) | 3 (ergonomics) | Fine, but undocumented for new users. |
| No outbound WebSocket client (deferred) | 4 (unnecessary) | Trestle realtime is SSE; probe proves SSE consumption works via streaming HTTP. No WS needed for v1. |
| HTTP server lacks PUT/DELETE/PATCH routing, middleware, subrouters; HTTP/2 deferred | 4 (unnecessary) | Step-executor workers are HTTP *clients*, not public servers. Keep them clients; front with Trestle/Workers. |
| SQLite rows are untyped (JSON) | 4 (acceptable) | Step-executor bookkeeping can use JSON rows; avoid a typed-domain-model dependency in Strut. |
| Async is a blocking thread-pool (`await` blocks a thread; no event loop) | 4 (avoided by design) | The step-executor model persists state in Trestle and executes short bounded steps, so long-lived in-process futures/actor loops are not needed. This is the key design decision that keeps Strut viable. |

## Recommendation

Strut **main** (not the v0.0.3 release) is the correct basis for the
experimental step-executor. Before building Switchyard's engine on it, the
release gate must be satisfied: the required surface (HTTP client streaming/
SSE, processes + cancellable pipes, crypto, cancellation) must be reviewed,
tested, hardened, documented, and included in an actual tagged release — per
the process agreed for Strut, not via Switchyard-specific patches.

The string search/contains gap (classification 3) is the most likely future
Switchyard pain point and is worth proposing as general Strut stdlib work if
it recurs.