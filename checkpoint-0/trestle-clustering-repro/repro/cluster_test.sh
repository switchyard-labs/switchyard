#!/usr/bin/env bash
# Reproduction: do clustered/Raft record writes emit events, audit, and outbox jobs
# the same way standalone writes do?
set -uo pipefail
source /opt/cp0/repro/lib.sh
BIN=$TRESTLE
BASE=http://127.0.0.1:7333
EVID=/opt/cp0/evidence/cluster
mkdir -p "$EVID"
rm -f "$EVID"/webhook_cluster.log "$EVID"/summary.txt

# restart webhook receiver fresh
fuser -k 8099/tcp 2>/dev/null || true
sleep 1
nohup python3 /opt/cp0/repro/webhook_receiver.py "$EVID/webhook_cluster.log" 8099 >/dev/null 2>&1 &
sleep 1

echo "=== verify readiness ==="
PASS=$(cat /opt/cp0/repro/.adminpass)
$BIN replicate status --url http://127.0.0.1:7333 --pass "$PASS" > "$EVID/status_before.txt" 2>&1
grep -E '"leader"|"state"|"readiness"' "$EVID/status_before.txt"

JAR=/tmp/cl.jar
CSRF=$(login "$BASE" "$JAR")
echo "csrf=$CSRF"

echo "--- create collection on A (leader) ---"
admin_post "$BASE" "$JAR" "$CSRF" /admin/v1/collections "{\"name\":\"items\",\"fields\":[{\"name\":\"title\",\"type\":\"text\"}]}" | jq -c . | head -c 300; echo
echo "--- create webhook on A ---"
admin_post "$BASE" "$JAR" "$CSRF" /admin/v1/webhooks "{\"name\":\"wh\",\"url\":\"https://example.com/hook\",\"topics\":[\"record.created\",\"record.updated\",\"record.deleted\"]}" | jq -c . | head -c 300; echo

# ---- write 1: via leader (A) ----
echo "--- create record via LEADER (A:7333) ---"
admin_post "$BASE" "$JAR" "$CSRF" /api/v1/collections/items/records "{\"values\":{\"title\":\"via leader\"}}" | jq -c .; echo
sleep 5

# ---- write 2: via follower (B) ----
echo "--- create record via FOLLOWER (B:7334) ---"
JARB=/tmp/clb.jar
CSRFB=$(login http://127.0.0.1:7334 "$JARB")
echo "csrfB=$CSRFB"
app_post http://127.0.0.1:7334 "$JARB" "$CSRFB" /api/v1/collections/items/records "{\"values\":{\"title\":\"via follower\"}}" | jq -c .; echo
sleep 5

dump_node() { local name=$1 dir=$2
  echo "===== $name ($dir) ====="
  echo "-- records count:"
  sqlite3 "$dir/trestle.db" "SELECT count(*) FROM _trestle_data_$(sqlite3 "$dir/trestle.db" "SELECT substr(lower(hex(id)),1,8) FROM _trestle_collections WHERE name='items';");" 2>/dev/null || echo "ERR record table"
  for t in _trestle_events _trestle_audit _trestle_jobs; do
    echo "-- $t: $(sqlite3 "$dir/trestle.db" "SELECT count(*) FROM $t;")"
  done
  echo "-- events:"
  sqlite3 -header -column "$dir/trestle.db" "SELECT sequence,substr(occurred_at,12,12) t,topic,record_id FROM _trestle_events ORDER BY sequence;" 2>/dev/null
  echo "-- jobs (kind/status/attempts):"
  sqlite3 -header -column "$dir/trestle.db" "SELECT kind,status,attempts,last_error FROM _trestle_jobs;" 2>/dev/null
}

{
  echo "=== CLUSTER SIDE-EFFECT REPRODUCTION (v0.1.5 @ 3c58fd58) ==="
  echo
  dump_node "NODE-A (leader)" /opt/cp0/cluster/a
  echo
  dump_node "NODE-B (follower)" /opt/cp0/cluster/b
  echo
  dump_node "NODE-C (follower)" /opt/cp0/cluster/c
  echo
  echo "=== webhook log ==="
  cat "$EVID/webhook_cluster.log" 2>/dev/null || echo "(no webhook deliveries)"
} | tee "$EVID/summary.txt"