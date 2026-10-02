#!/usr/bin/env bash
# CP6 gate: Agent substrate + credentials.
# - credential lifecycle (create/list/rotate/delete) with no secret exposure
# - roles
# - substrate run: Work -> Attempt -> role implementer -> execution -> edit -> commit
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp6.jar; rm -f "$JAR"

TOK=$(/opt/cp0/switchyard/token.sh)
REPO=cp6-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"; echo base>base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base; git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

echo "### 1. roles"
curl -s -b "$JAR" "$APP/api/roles" | jq -c "[.items[] | {name, capabilities}]"

echo "### 2. create credential (secret must never echo)"
SECRET="sk-live-SUPERSECRET-abc123"
c=$(curl -s -b "$JAR" -X POST "$APP/api/credentials" -H "Content-Type: application/json" -d "{\"name\":\"deepseek-personal\",\"provider\":\"deepseek\",\"secret\":\"$SECRET\",\"scope\":\"personal\"}")
echo "$c" | jq -c .
credid=$(echo "$c" | jq -r .id)
echo "secret leaked in create response: $(echo "$c" | grep -c "$SECRET")"

echo "### 3. list credentials (metadata only, no ciphertext/secret)"
curl -s -b "$JAR" "$APP/api/credentials" | jq -c .
echo "secret leaked in list: $(curl -s -b "$JAR" "$APP/api/credentials" | grep -c "$SECRET")"
echo "ciphertext leaked in list: $(curl -s -b "$JAR" "$APP/api/credentials" | grep -c ciphertext)"

echo "### 4. rotate"
curl -s -b "$JAR" -X POST "$APP/api/credentials/$credid/rotate" -H "Content-Type: application/json" -d '{"secret":"sk-live-ROTATED-secret"}' | jq -c .
echo "old secret still visible anywhere: $(curl -s -b "$JAR" "$APP/api/credentials" | grep -c "$SECRET")"

echo "### 5. delete + revoked handling"
curl -s -b "$JAR" -X DELETE "$APP/api/credentials/$credid" | jq -c .
echo "list after delete: $(curl -s -b "$JAR" "$APP/api/credentials" | jq -c '[.items[].name]')"

echo "### 6. substrate run: Work -> Attempt -> implementer execution -> edit -> commit"
wid=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp6 agent run","kind":"feature"}' | jq -r .id)
att=$(curl -s -b "$JAR" -X POST "$APP/api/work/$wid/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\"}" | jq -r .id)
run=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$att/run")
echo "$run" | jq -c .
echo "--- executions recorded ---"
PASS=$(cat /opt/cp0/repro/.adminpass); J2=/tmp/adm11.jar
curl -s -c $J2 -X POST http://127.0.0.1:7350/admin/v1/session -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$PASS\"}" >/dev/null
curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/executions/records | jq -c "[.items[].values | {id, role, adapter, status, output}]"
echo "--- attempt branch content ---"
curl -s -b "$JAR" "$APP/api/repos/$REPO/content?ref=$(echo $att | sed 's/^wk_/attempt-/')&path=ATTEMPT.md" | tail -3

echo "### CP6 gate done"