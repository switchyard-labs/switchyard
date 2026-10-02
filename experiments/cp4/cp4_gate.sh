#!/usr/bin/env bash
# CP4 gate: deterministic end-to-end vertical slice.
# Work -> Attempt -> Strut executor -> repo edit -> commit -> PR -> check
# -> Integration Queue -> canonical integration. No AI model.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp4.jar; rm -f "$JAR"

# seed a baseline repo
TOK=$(/opt/cp0/switchyard/token.sh)
REPO=cp4-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
echo "package main" > main.go; echo "base" > base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm "baseline"
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
BASE=$(git rev-parse HEAD); echo "baseline $REPO base=$BASE"

# login
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

echo "### 1. create Work"
wid=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"deterministic slice","kind":"feature"}' | jq -r .id)
echo "work=$wid"

echo "### 2. create Attempt (isolated branch)"
att=$(curl -s -b "$JAR" -X POST "$APP/api/work/$wid/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\"}")
echo "$att" | jq -c .
attid=$(echo "$att" | jq -r .id)
branch=$(echo "$att" | jq -r .branch)

echo "### 3. run Attempt via Strut executor"
run=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$attid/run")
echo "$run" | jq -c .

echo "### 4. open Pull Request"
pr=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$attid/pr" -H "Content-Type: application/json" -d '{"title":"deterministic change"}')
echo "$pr" | jq -c .
prid=$(echo "$pr" | jq -r .id)

echo "### 5. deterministic check"
curl -s -b "$JAR" -X POST "$APP/api/prs/$prid/check" | jq -c .

echo "### 6. integrate (Integration Queue) -> merge into canonical"
curl -s -b "$JAR" -X POST "$APP/api/prs/$prid/integrate" | jq -c .

echo "### 7. verify canonical main now contains the deterministic change"
curl -s -b "$JAR" "$APP/api/repos/$REPO/tree?ref=main" | jq -c "[.tree[].path]"
echo "--- ATTEMPT.md on main ---"
curl -s -b "$JAR" "$APP/api/repos/$REPO/content?ref=main&path=ATTEMPT.md"; echo

echo "### 8. provenance: ref_updates + prs"
PASS=$(cat /opt/cp0/repro/.adminpass); J2=/tmp/adm8.jar
curl -s -c $J2 -X POST http://127.0.0.1:7350/admin/v1/session -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$PASS\"}" >/dev/null
echo "-- ref_updates:"; curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/ref_updates/records | jq -c "[.items[].values | {branch, old_sha:.old_sha[0:7], new_sha:.new_sha[0:7], provenance}]"
echo "-- prs:"; curl -s -b "$JAR" "$APP/api/prs" | jq -c "[.items[] | {id, status, check_status, branch}]"
echo "-- runs:"; curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/runs/records | jq -c "[.items[].values | {attempt_id, new_sha:.new_sha[0:7]}]"

echo "### CP4 gate done"