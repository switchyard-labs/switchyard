# Browser realtime decision

**Recommendation: a Worker / Durable-Object SSE bridge to Trestle.**

The browser-facing realtime path should be: browser → (authenticated) → Worker
→ single Trestle SSE connection (service credential) → filtered fan-out to
subscribers. Trestle's durable event journal remains the single source of
truth and the replay source.

## The two candidate paths

### Path 1 — Trestle application-user SSE (native)

Trestle's `/api/v1/realtime` currently accepts **only admin sessions or
service/personal credentials** (verified in `internal/events/handler.go`:
`admin.Authorize` else `credentials.Authenticate(records:read)`); application
user access tokens are refused. Even if Trestle accepted user tokens, browser
`EventSource` cannot set an `Authorization` header, so a browser would need
either a cookie-based app session or a proxy anyway. Making user-facing SSE a
first-class Trestle feature would require:

- accepting application-user bearer tokens (and/or an app-session cookie) on
  `/api/v1/realtime`;
- per-subscriber collection-rule filtering of delivered events (fail-closed);
- documented reconnection/replay semantics against the journal.

This is a legitimate, generally-correct Trestle development item — record it as
a candidate Trestle campaign, not as a Switchyard hack. It is not needed before
Checkpoint 1.

### Path 2 — Worker/Durable-Object realtime bridge (recommended now)

- Browser connects to a Worker endpoint (WebSocket or SSE-over-fetch) that
  authenticates the caller against the Switchyard platform.
- The Worker (or a Durable Object using WebSocket Hibernation for scale) holds
  one Trestle SSE connection with a scoped service credential and fans out
  filtered events to subscribed browsers.
- On reconnect, the client supplies the last Trestle event sequence id; the
  Worker resumes Trestle SSE with `Last-Event-ID` (Trestle already supports
  this — verified in the SSE handler).

## Why Path 2 for the near term

| Dimension | Assessment |
| --- | --- |
| Authentication model | Browser proves identity to the Worker; the Worker holds the Trestle service credential. Trestle's auth model is unchanged. |
| Trust boundary | Browsers never see Trestle credentials. The Worker is the trust boundary and enforces per-caller visibility. |
| Scaling | Hibernation/Durable Objects absorb many browser connections; one (or few) upstream Trestle SSE streams per topic. Trestle polls its journal at 500 ms — fine for a coordination dashboard, not a high-throughput fan-out. |
| Reconnection/replay | Use Trestle's journal sequence ids (`Last-Event-ID`); the Worker/DO is an ephemeral fan-out cache, never an authority. |
| Relation to Trestle history | Trestle's `_trestle_events` journal stays the durable, replayable history; Cloudflare adds no new durability. |
| WebSockets vs SSE | SSE is sufficient for a viewer dashboard (task transitions, provenance timelines). WebSocket only if browser→server control messages are needed later; the bridge can upgrade without changing Trestle. |

## Open items before Checkpoint 1

1. Confirm the Worker's auth hand-off to the platform (how the browser session
   token is validated — likely via the Switchyard platform API or a signed
   session token minted by the platform).
2. Confirm the browser realtime is fed by Trestle events only (task/run/
   provenance topics), and that clustered-mode events (see
   `trestle-clustering-repro/`) are not required for the single-node slice.