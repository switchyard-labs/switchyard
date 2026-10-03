# C36 — automatic event-path certification, October 4

PASS for a real disposable repository, automatic Queue/ref observation, native
Cloudflare Workflow CI, authenticated SSE, duplicate delivery and reconciliation.
This is not the real external-Agent dogfood (C25).

Repository: `alice/railway`, physical Artifacts repository
`switchyard-cp0/codex-actions-probe-20261003`. Branch: `event-c36`.
Queue/subscription: `switchyard-event-probe-20261004`,
`a8e6747ff6414c4299ff8da3020c7634`, `dd5b9943eeec41678d3b82ebab71f17a`.
Only this disposable repository is subscribed; unrelated queues were not consumed.
No initial HTTP event-ingest request or manual Actions dispatch was used.

## Real pushes and timings

1. Scoped write capability → ordinary clone → new branch → configuration/marker
   commit → push `91c8376985668eb6e5bca932dfd0cb56160681eb`.
   Git returned success at Unix `1791050475.1531892`.
   Provider timestamp: `2026-10-03T18:01:15.434Z`.
   Switchyard persisted the Queue transition at `18:01:15.983479275Z`:
   **0.83 s** after Git returned. Native CI started at `18:01:26.114Z`
   (**10.96 s**) and succeeded at `18:01:47.410Z` (**32.26 s**).
   Worker manifest proves direct Artifacts `readCommit` and bounded configuration
   fingerprint for the same SHA. Run `ba56abf0-8f10-43aa-a0cc-a25425afd3d4`.
2. Replay the observed provider envelope, retaining its provider timestamp/account,
   through the disposable Queue. One provider receipt, one domain event, one Action
   remain. This replay is explicitly the duplicate-delivery test, not the initial proof.
3. Disable isolated Queue consumption; enable reconciliation. Ordinary second push
   `f7770dc0f0a6dcb2915df26e36401feb2e13b14e` converged through reconciliation at
   `18:15:12.102517464Z`, **415.82 s (6 min 55.82 s)** after Git returned.
   Re-enable Queue consumption: late delivery retains one domain transition,
   with the original reconciliation source. The serial 75-repository scan is slow;
   a `1s` ticker does not mean a one-second sweep. C23/operational scaling remains open.
4. Connect authenticated SSE before a third ordinary push:
   `74c746e6703a0094b0a9512ed948d2e58376e4de`.
   One live SSE frame arrived **1.11 s** after Git returned; one automatic Action
   completed. Maintained script: `scripts/certify-event-path.py`.

## Architecture and limits

The deployed paths are parallel fan-out:

```
Artifacts push → native CI trigger → Workflow → direct binding inspection → CI → R2
             → repository subscription → Queue → Switchyard → Trestle → SSE/activity
```

Go reconciles Worker run manifests into Actions/required-check state. This is not
a claim that the Worker forwards its event into this Queue. Individual Worker
receipt/inspection timestamps and the exact browser-render timestamp are not
instrumented, so do not invent per-arrow latency from the measured end points.

The browser showed the automatic exact-SHA succeeded run, with no captured
warning/error logs. `automatic-run.png` records it. The initial CI actor label
comes from commit author metadata; it is not authenticated push-actor identity.

Full Go tests/vet and Worker tests/typecheck passed. JSON files contain evidence,
not credentials. Capabilities were never written into Git config or argv.
No project Git pushes or Linode deployment occurred in this checkpoint.
