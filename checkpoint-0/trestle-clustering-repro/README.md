# Trestle clustered side-effect reproduction

**Claim under test:** clustered/Raft record writes do not produce the same
events, audit records, webhook jobs, or function/outbox side effects that
standalone writes produce.

**Verdict: CONFIRMED.** In a real 3-process Raft cluster, record writes that
commit through consensus produce **zero** events, audit facts, and outbox jobs
on every node — including the node that received the write and the leader. The
docs' statement that these side effects "remain node-local" is inaccurate:
they do not occur anywhere.

## Exact version

- Trestle tag `v0.1.5` (the current "stable" public preview release).
- Commit: `3c58fd582d32d97aa360aa58b4572d2ce354b1f5`.
- Built from source with Go 1.27.1; binary `/opt/cp0/trestle-v015`.

## Topology

Three processes on one host (the disposable Linode), plaintext development
transport:

| Node | HTTP | Raft listen | Raft ID (Gantry node_id) | Role |
| --- | --- | --- | --- | --- |
| A | 127.0.0.1:7333 | 127.0.0.1:7440 | `tr_TgbTt8pyEbNr5cC9i09FsQ` | bootstrap, leader |
| B | 127.0.0.1:7334 | 127.0.0.1:7441 | `tr_iD-Q7miJoF8f5b4jjgmKaw` | voter / follower |
| C | 127.0.0.1:7335 | 127.0.0.1:7442 | `tr_X667x3JvWdt71flhrqHSTg` | voter / follower |

- Provider: SQLite (Trestle clustering is SQLite-only; PostgreSQL fails closed).
- Gantry peer clustering: full mesh, invitation-based pairing (A↔B, A↔C, B↔C),
  all members `active` with the `replication` capability.
- Raft formation: A bootstrapped (`TRESTLE_REPLICATION_BOOTSTRAP=true`); B and C
  added as voters via `trestle replicate join <node_id> <raft-address>`.
- Replication transport env: `TRESTLE_REPLICATION_ENABLED=true`,
  `TRESTLE_REPLICATION_INSECURE_PLAINTEXT=true` (explicit local-dev opt-in),
  per-node `NODE_ID`/`LISTEN`.

## Operation performed

Via the admin API on the leader (A):

1. Created collection `items` (field `title` text).
2. Created webhook `wh` → `https://example.com/hook`, topics
   `record.created|record.updated|record.deleted` (HTTPS is required by
   Trestle's SSRF guard; the local webhook receiver therefore observes
   enqueued jobs only, and deliveries to loopback are refused by design).
3. `POST /api/v1/collections/items/records` **via the leader (A)** → `201`, record
   `rec_4tST4YQqWP8R3_y01Bek`.
4. `POST /api/v1/collections/items/records` **via the follower (B)** → `201`,
   record `rec_NXXBXRz67u_7uJFjJiAE` (follower-forwarded mutation).

## Expected standalone behavior

The identical sequence on a standalone single-node instance produced, per
record write:

- `_trestle_events` = 1 (`record.created`)
- `_trestle_audit` = 1
- `_trestle_jobs` = 1 (kind `webhook`, enqueued transactionally)

## Actual clustered behavior

Both records committed through Raft and replicated to all three nodes:
2 rows in `_trestle_data_30a99b37638446fa` on A, B, and C, and all three
nodes' APIs return both records.

But on **all three nodes** after both writes:

- `_trestle_events` = **0**
- `_trestle_audit` = **0**
- `_trestle_jobs` = **0**
- webhook receiver: **no delivery** (no job was ever enqueued)

Leadership remained stable (`tr_TgbTt8pyEbNr5cC9i09FsQ`, `ready-leader`) through
the writes.

## Is the issue universal or path-dependent?

Both write paths reproduce it:

- leader write (A) → no side effects;
- follower-forwarded write (B) → no side effects.

Code inspection shows create/update/delete all share the same shape
(`internal/records/handler.go`): when `h.authority != nil` (Raft configured) the
handler calls `PutRecord`/`DeleteRecord` and returns **before**
`events.Emit` / `auditor.Emit` are reached. There is no write path that emits
side effects in replicated mode.

## Likely architectural cause

- `internal/records/handler.go`: the `authority != nil` branch (create ~288,
  update ~640, delete ~714) bypasses the transaction that in standalone mode
  appends the event, audit fact, and (via dispatchers) webhook/Lambda outbox
  job.
- `internal/replicated/fsm.go` `materialize()` only handles
  `KindCollectionPut/Delete` and `KindRecordPut/Delete` — it materializes
  collection and record tables and nothing else (no events, audit, or jobs).
- The event/audit/outbox machinery is wired to the standalone transaction path
  only. The docs' "side effects remain node-local" wording implies they happen
  locally; in fact they are absent entirely.

## Secondary finding: cluster formation is under-documented

Forming the cluster failed until the raft `ServerID` was set to the Gantry
`node_id` (`tr_...`) — the transport's membership pre-check looks up peers by
raft ServerID in `_trestle_cluster_members` (`internal/replicated/
authenticator.go` `Membership()`), and the Gantry identity is auto-generated
(`tr_...`) with no documented way to align it. An operator following the docs
with friendly node names gets "peer … is not an active replication member" and
an endless raft election. `trestle cluster init` does not accept or surface
the replication node id. This is a real documentation/UX gap worth a Trestle
issue.

## Evidence artifacts

Captured on the Linode:

- `/opt/cp0/evidence/cluster/summary.txt` — full per-node table dumps.
- `/opt/cp0/evidence/cluster/status_before.txt` — leadership before writes.
- `/opt/cp0/evidence/standalone/` and `/opt/cp0/repro/standalone.out` — the
  standalone contrast.
- Repro scripts: `repro/standalone.sh`, `repro/cluster_setup_v3.sh`,
  `repro/cluster_test.sh`, `repro/webhook_receiver.py` (see `repro/`).

## Recommended separate Trestle work

1. Decide the clustered side-effect contract explicitly. Either:
   - **replicate** events/audit/outbox as part of the Raft state machine
     (general and correct), or
   - **document** that clustered mode does not produce events/audit/automation
     and expose a first-class out-of-band mechanism (e.g., a "replicated
     change feed" the control plane consumes), rather than the current silent
     divergence.
2. Make cluster formation self-consistent: align raft ServerID with the Gantry
   identity automatically, or document the required equality and validate it at
   `trestle replicate join` time with a clear error.

**Note:** per the Checkpoint 0 brief, Trestle was NOT modified during this
phase. This is a reproduction and characterization for a later, separately
planned Trestle development campaign.