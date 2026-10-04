# Runner/provider/publication stages

Candidate implementation records bounded structured provider status codes, avoiding provider messages in the protocol. Auth (401/403), rate limits (429), request timeout (408/504), invalid structured output and other requests have separate codes. Kernel OOM-kill and PID-denial events classify resource failures; absent accounting stays unknown/nonzero. Wall deadline, cancellation, startup and queue saturation remain distinct.

Attempt API responses return a safe code, concise summary and execution ID where available. Provider rate limits/queue saturation use 429; provider auth 424; timeout 504; runner startup/resource limits 503; cancellation/stale publication 409. Deep diagnostics remain redacted execution records. Git fetch/commit/push errors carry typed stage codes. Failed recovery reads have publication_recovery_failed. A stale ref is no longer a successful HTTP 200.

Measured adapter timings: bootstrap before runtime start, complete model/runtime process duration, result validation/read and total adapter time. `model_execution_ns` includes runtime startup, provider communication, tool events and model time: **it is not direct HTTP provider latency**. Exact provider request-start/response timestamps remain unavailable from this event interface and are not fabricated.

Measured publication timings: preflight (repo/token/ref checks), Git fetch, commit preparation/commit, push (including authoritative ref guard), state recording and publication total. These have explicit nested boundaries; summing them with adapter total does not include all HTTP route/auth/source-read overhead. Execution metadata retains publication results/timings and returned SHA. Accounting is sampled and may precede final exit.

Policy environment variables expose concurrency (1–4), bounded queue depth (default 16, 1–256), memory MiB, PIDs, CPU quota percent, wall timeout seconds and output KiB. Loopback `/metrics` exposes limits as agent_policy plus running/queued gauges and failure/duration counters. Defaults are provisional pending resource comparison; no production policy change or Linode capacity verdict follows from local results.
