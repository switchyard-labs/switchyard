#!/usr/bin/env bash
set -euo pipefail
source /opt/cp0/repro/lib.sh
BASE=http://127.0.0.1:7333
DIR=/opt/cp0/standalone
EVID=/opt/cp0/evidence/standalone
mkdir -p "$EVID"
pkill -f "trestle-v015 serve" 2>/dev/null || true
sleep 1
setup_node "$DIR" admin@example.com
: > /opt/cp0/evidence/webhook_standalone.log
nohup python3 /opt/cp0/repro/webhook_receiver.py /opt/cp0/evidence/webhook_standalone.log 8099 >/dev/null 2>&1 &
echo $! > /opt/cp0/repro/wh.pid
TRESTLE_DATA_DIR="$DIR" TRESTLE_PORT=7333 nohup $TRESTLE serve > "$EVID/server.log" 2>&1 &
echo $! > /opt/cp0/repro/server.pid
sleep 3
JAR=/tmp/sa.jar
CSRF=$(login "$BASE" "$JAR")
echo "csrf=$CSRF"
echo "--- create collection ---"
admin_post "$BASE" "$JAR" "$CSRF" /admin/v1/collections "{\"name\":\"items\",\"fields\":[{\"name\":\"title\",\"type\":\"text\"}]}"
echo
echo "--- create webhook ---"
admin_post "$BASE" "$JAR" "$CSRF" /admin/v1/webhooks "{\"name\":\"wh\",\"url\":\"https://example.com/hook\",\"topics\":[\"record.created\",\"record.updated\",\"record.deleted\"]}"
echo
echo "--- create record ---"
app_post "$BASE" "$JAR" "$CSRF" /api/v1/collections/items/records "{\"values\":{\"title\":\"hello standalone\"}}"
echo
sleep 4
echo "=== DB COUNTS (standalone) ==="
for t in _trestle_events _trestle_audit _trestle_jobs _trestle_webhooks; do
  c=$(sqlite3 "$DIR/trestle.db" "SELECT count(*) FROM $t;" 2>/dev/null || echo ERR)
  echo "$t = $c"
done
echo "=== events rows ==="
sqlite3 -header -column "$DIR/trestle.db" "SELECT sequence,occurred_at,topic,collection_name,record_id FROM _trestle_events ORDER BY sequence;"
echo "=== jobs rows ==="
sqlite3 -header -column "$DIR/trestle.db" "SELECT id,kind,status,attempts FROM _trestle_jobs ORDER BY created_at;"
echo "=== webhook log ==="
cat /opt/cp0/evidence/webhook_standalone.log
