# C14 — authenticated run controls

Manual dispatch, rerun all, failed-job rerun, cancellation and administrator definition approval now use canonical repository grants and same-origin requests. Approved JavaScript is compiled with the bounded definition adapter; commands come from the approved definition rather than a dispatch body. Pull request dispatch snapshots its current source SHA and rejects an explicitly supplied stale SHA.

The immutable queued run exists before the Cloudflare effect. A client request key maps to one workflow ID and rejects reuse with a different source. Reconciliation recovers dispatch and cancellation after ambiguous responses. Queued intent fences earlier passing checks immediately. Terminal runs and obsolete definitions cannot be rerun as if current.

Real evidence on the disposable Worker/repository:

- `status.json`: authenticated manual run at `b4e3fe7f7d3e76931b0f7a9dbdabc248c10cb402`; verify passed and the failure probe failed.
- `rerunstatus.json`: browser-triggered failed-job rerun at the same SHA/definition. The successful verify job was explicitly reused from its parent, while the failed job executed again. The screenshot shows the parent capture served through the new run's authorized log endpoint.
- `cancelstatus.json`: cancellation before a provider manifest existed recovered to cancelled, with cancelled required checks. The live test exposed repeated termination returning an error; cancellation now reads terminal provider status before attempting termination again.

Tests cover immutable replay, identity conflicts, queued check fencing, failed-job selection, anonymous denial, parent SHA/command substitution and cancellation-before-manifest recovery. Actions/app race tests, vet, Worker type checking and six Worker protocol tests passed. ANSI safety's two tests were also rerun with real Node test isolation.

Preview/deployment integration is **not certified**. The configured API token cannot access Workers Builds and there is no verified connected build token/trigger. The UI states that prerequisite; no synthetic preview/deployment URL or successful deployment is claimed. C25 must report this limitation unless verified setup becomes available. Workflow termination stops scheduling; any already running sandbox command remains bounded by its approved timeout rather than a claimed immediate process kill.
