#!/usr/bin/env bash
# Corrected cluster formation: raft ServerID must equal the Gantry node_id (tr_...).
# Node ids are read directly from each node's identity table.
set -uo pipefail
source /opt/cp0/repro/lib.sh
export TRESTLE_REPLICATION_ENABLED=true TRESTLE_REPLICATION_INSECURE_PLAINTEXT=true
BIN=$TRESTLE

pkill -f "trestle-v015 serve" 2>/dev/null || true
sleep 1
rm -rf /opt/cp0/cluster; mkdir -p /opt/cp0/cluster/{a,b,c}
setup_node /opt/cp0/cluster/a admin@example.com
setup_node /opt/cp0/cluster/b admin@example.com
setup_node /opt/cp0/cluster/c admin@example.com

# Create Gantry identities (no replication env needed for init).
env -u TRESTLE_REPLICATION_ENABLED TRESTLE_DATA_DIR=/opt/cp0/cluster/a $BIN cluster init --name node-a --endpoint http://127.0.0.1:7333 --json >/dev/null 2>&1
env -u TRESTLE_REPLICATION_ENABLED TRESTLE_DATA_DIR=/opt/cp0/cluster/b $BIN cluster init --name node-b --endpoint http://127.0.0.1:7334 --json >/dev/null 2>&1
env -u TRESTLE_REPLICATION_ENABLED TRESTLE_DATA_DIR=/opt/cp0/cluster/c $BIN cluster init --name node-c --endpoint http://127.0.0.1:7335 --json >/dev/null 2>&1

NID_A=$(sqlite3 /opt/cp0/cluster/a/trestle.db "SELECT node_id FROM _trestle_cluster_identity WHERE singleton=1")
NID_B=$(sqlite3 /opt/cp0/cluster/b/trestle.db "SELECT node_id FROM _trestle_cluster_identity WHERE singleton=1")
NID_C=$(sqlite3 /opt/cp0/cluster/c/trestle.db "SELECT node_id FROM _trestle_cluster_identity WHERE singleton=1")
echo "node ids: A=$NID_A B=$NID_B C=$NID_C"

start_one() { local dir=$1 port=$2 nodeid=$3 raft=$4 bootstrap=$5
  env TRESTLE_DATA_DIR="$dir" TRESTLE_PORT=$port TRESTLE_REPLICATION_NODE_ID=$nodeid \
      TRESTLE_REPLICATION_LISTEN=$raft TRESTLE_REPLICATION_INSECURE_PLAINTEXT=true \
      TRESTLE_REPLICATION_ENABLED=true TRESTLE_REPLICATION_BOOTSTRAP=$bootstrap \
      setsid $BIN serve </dev/null >>"$dir/server.log" 2>&1 &
}
start_one /opt/cp0/cluster/a 7333 "$NID_A" 127.0.0.1:7440 true
start_one /opt/cp0/cluster/b 7334 "$NID_B" 127.0.0.1:7441 false
start_one /opt/cp0/cluster/c 7335 "$NID_C" 127.0.0.1:7442 false
sleep 6
for p in 7333 7334 7335; do echo -n "port $p health: "; curl -s -m 3 http://127.0.0.1:$p/system/health; echo; done

RAFT_A=127.0.0.1:7440
RAFT_B=127.0.0.1:7441
RAFT_C=127.0.0.1:7442

ci() { local dir=$1 listen=$2; shift 2; TRESTLE_DATA_DIR=$dir TRESTLE_REPLICATION_LISTEN=$listen $BIN cluster "$@"; }

pair() { local fd=$1 fdraft=$2 td=$3 tdraft=$4 tp=$5 tag=$6
  ci "$td" "$tdraft" invite --secret-file /tmp/inv_$tag >/dev/null 2>&1
  local outid=$(ci "$fd" "$fdraft" join --url http://127.0.0.1:$tp --token-file /tmp/inv_$tag --json 2>/dev/null | jq -r .id)
  local pend=$(ci "$td" "$tdraft" joins --json 2>/dev/null | jq -r ".[0].id")
  ci "$td" "$tdraft" approve --id "$pend" >/dev/null 2>&1
  local res=$(ci "$fd" "$fdraft" collect --id "$outid" 2>&1)
  echo "pair $tag out=$outid pend=$pend -> ${res:0:60}"
}
pair /opt/cp0/cluster/b "$RAFT_B" /opt/cp0/cluster/a "$RAFT_A" 7333 ab
pair /opt/cp0/cluster/c "$RAFT_C" /opt/cp0/cluster/a "$RAFT_A" 7333 ac
pair /opt/cp0/cluster/c "$RAFT_C" /opt/cp0/cluster/b "$RAFT_B" 7334 bc

echo "=== members A ==="; ci /opt/cp0/cluster/a "$RAFT_A" members --json | jq -r ".[] | .display_name + \" \" + .public_endpoint"
echo "=== members B ==="; ci /opt/cp0/cluster/b "$RAFT_B" members --json | jq -r ".[] | .display_name + \" \" + .public_endpoint"

PASS=$(cat /opt/cp0/repro/.adminpass)
echo "=== replicate status A (expect leader) ==="
$BIN replicate status --url http://127.0.0.1:7333 --pass "$PASS" 2>&1 | head -25
echo "=== join voters (use Gantry node ids) ==="
$BIN replicate join --url http://127.0.0.1:7333 --pass "$PASS" "$NID_B" 127.0.0.1:7441 2>&1 | head -3
sleep 3
$BIN replicate join --url http://127.0.0.1:7333 --pass "$PASS" "$NID_C" 127.0.0.1:7442 2>&1 | head -3
sleep 6
echo "=== replicate status after joins ==="
$BIN replicate status --url http://127.0.0.1:7333 --pass "$PASS" 2>&1 | head -40