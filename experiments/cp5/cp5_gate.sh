#!/usr/bin/env bash
# CP5 gate: reconciliation, provenance, realtime.
# External git push -> control-plane reconciliation -> durable event -> SSE
# -> browser updates without manual refresh.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp5.jar; rm -f "$JAR"

# seed a repo
TOK=$(/opt/cp0/switchyard/token.sh)
REPO=cp5-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
echo "base" > base.txt; git add -A; git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
echo "seeded $REPO"

# login
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

echo "### 1. start an SSE client (browser equivalent)"
curl -s -N -b "$JAR" "$APP/api/events/stream" > /tmp/cp5_sse.out 2>&1 &
SSE_PID=$!
sleep 2

echo "### 2. wait for initial reconciliation (baseline ref_obs)"
sleep 12
PASS=$(cat /opt/cp0/repro/.adminpass); J2=/tmp/adm10.jar
curl -s -c $J2 -X POST http://127.0.0.1:7350/admin/v1/session -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$PASS\"}" >/dev/null
echo "ref_obs baseline: $(curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/ref_obs/records | jq -c "[.items[].values | {repo, branch, sha:.sha[0:7]}]")"

echo "### 3. external git push (human push outside the UI)"
cd "$W"; echo "external change" >> base.txt; git add -A
git -c user.name=external -c user.email=ext@local commit -qm "external push"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin main
NEW=$(git rev-parse HEAD)
echo "external pushed $NEW"

echo "### 4. wait for reconciliation to detect it"
sleep 14
echo "ref_obs now: $(curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/ref_obs/records | jq -c "[.items[].values | {repo, branch, sha:.sha[0:7]}]")"
echo "events now: $(curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/events/records | jq -c "[.items[].values | {type, repo_name}]")"

echo "### 5. SSE client received the event?"
sleep 2
kill $SSE_PID 2>/dev/null
echo "--- SSE stream captured ---"
grep -c "git.ref_changed" /tmp/cp5_sse.out || echo "no ref_changed event in SSE"
grep -oE 'data: .*' /tmp/cp5_sse.out | head -3

echo "### 6. restart resilience: kill + restart control plane, no duplicate events"
pkill -f "switchyard/switchyard" 2>/dev/null; sleep 1
PASS=$(cat /opt/cp0/repro/.adminpass)
setsid env SWITCHYARD_TRESTLE_URL=http://127.0.0.1:7350 SWITCHYARD_TRESTLE_USER=admin SWITCHYARD_TRESTLE_PASS=$PASS SWITCHYARD_ARTIFACTS_ACCOUNT=b7f20353ee8a9e5d2003f52c74ba795e SWITCHYARD_ARTIFACTS_NAMESPACE=switchyard-cp0 SWITCHYARD_ARTIFACTS_TOKEN_CMD=/opt/cp0/switchyard/token.sh SWITCHYARD_STATIC_DIR=/opt/cp0/switchyard/public SWITCHYARD_LISTEN=127.0.0.1:8080 SWITCHYARD_DATA_DIR=/opt/cp0/switchyard/data SWITCHYARD_STRUT_BIN=/opt/cp0/switchyard/deterministic-worker SWITCHYARD_RECONCILE_INTERVAL=10s /opt/cp0/switchyard/switchyard </dev/null >>/opt/cp0/switchyard/control.log 2>&1 &
sleep 12
echo "events after restart (should not duplicate the ref_changed): $(curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/events/records | jq -c "[.items[] | select(.values.repo_name==\"$REPO\")] | length")"

echo "### 7. duplicate ingest is idempotent"
EV="{\"type\":\"cf.artifacts.repo.pushed\",\"repo_name\":\"$REPO\",\"namespace\":\"$NS\",\"payload\":{\"ref\":\"refs/heads/main\",\"after\":\"$NEW\"},\"occurred_at\":\"2026-10-02T17:00:00Z\"}"
curl -s -X POST "$APP/api/events/ingest" -H "Content-Type: application/json" -d "$EV" | jq -c .
curl -s -X POST "$APP/api/events/ingest" -H "Content-Type: application/json" -d "$EV" | jq -c .

echo "### 8. provenance chain for a full Work->Attempt->PR->integrate run"
JAR2=/tmp/cp5b.jar; rm -f $JAR2
curl -s -c "$JAR2" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
wid=$(curl -s -b "$JAR2" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"provenance demo","kind":"feature"}' | jq -r .id)
att=$(curl -s -b "$JAR2" -X POST "$APP/api/work/$wid/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\"}" | jq -r .id)
curl -s -b "$JAR2" -X POST "$APP/api/attempts/$att/run" >/dev/null
pr=$(curl -s -b "$JAR2" -X POST "$APP/api/attempts/$att/pr" -H "Content-Type: application/json" -d '{"title":"provenance demo PR"}' | jq -r .id)
curl -s -b "$JAR2" -X POST "$APP/api/prs/$pr/check" >/dev/null
curl -s -b "$JAR2" -X POST "$APP/api/prs/$pr/integrate" >/dev/null
echo "--- provenance chain ---"
curl -s -b "$JAR2" "$APP/api/work/$wid/provenance" | jq -c "[.chain[] | {kind, id, status, branch, new_sha, provenance}]"

echo "### CP5 gate done"