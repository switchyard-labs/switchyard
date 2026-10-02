# Realtime

Browser realtime uses a **Worker / Durable-Object SSE bridge to Trestle**.

```
browser (SSE) ──► Worker/DO (authenticated) ──► one Trestle SSE stream (service credential)
                    └── filtered fan-out to subscribers
```

Rationale (full detail in `checkpoint-0/realtime-decision.md`):

- Trestle's `/api/v1/realtime` accepts admin or service credentials, not
  application-user tokens, and browsers cannot set an `Authorization` header on
  `EventSource` — so a proxy/bridge is required regardless.
- The Worker authenticates the browser; the Worker holds a scoped service
  credential for Trestle; Trestle's durable event journal stays the single
  source of truth and the replay source.
- Reconnection/replay uses Trestle's event sequence ids (`Last-Event-ID`).
- WebSocket Hibernation/Durable Objects absorb many browser connections; Trestle
  polls its journal at ~500 ms — fine for a coordination dashboard.
- SSE is sufficient for a viewer dashboard; upgrade to WebSocket only if
  browser→server control messages become necessary.

**Candidate Trestle development item** (recorded, not done inside Switchyard):
making user-facing SSE a first-class Trestle capability (user tokens +
per-subscriber rule filtering). Not blocking — the Worker bridge works now.

Realtime surfaces: integration queue state, agent activity, concurrent-work
indicators, change checks/conflicts, Needs Attention count, workflow-run status.