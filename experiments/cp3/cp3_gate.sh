#!/usr/bin/env bash
# CP3 gate: safe ref mutation substrate — CAS semantics + provenance.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp3.jar; rm -f "$JAR"

# seed a repo
TOK=$(/opt/cp0/switchyard/token.sh)
REPO=cp3-race-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" \
  -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
echo "base" > base.txt; git add -A; git -c user.name=s -c user.email=s@l commit -q -m base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
BASE=$(git rev-parse HEAD)
echo "seeded $REPO base=$BASE"

# login
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

echo "### refs read (ls-remote via API)"
curl -s -b "$JAR" "$APP/api/repos/$REPO/refs" | jq -c ".refs"

echo "### writer A: CAS update from base -> should be ok"
A=$(curl -s -b "$JAR" -X POST "$APP/api/refs/update" -H "Content-Type: application/json" \
  -d "{\"repo\":\"$REPO\",\"branch\":\"main\",\"expected_sha\":\"$BASE\",\"message\":\"A adds a.txt\",\"changes\":[{\"path\":\"a.txt\",\"content\":\"A\"}]}")
echo "$A" | jq -c .
NEW=$(echo "$A" | jq -r .new_sha)

echo "### writer B: stale expected (old base) -> should be STALE"
B=$(curl -s -b "$JAR" -X POST "$APP/api/refs/update" -H "Content-Type: application/json" \
  -d "{\"repo\":\"$REPO\",\"branch\":\"main\",\"expected_sha\":\"$BASE\",\"message\":\"B adds b.txt\",\"changes\":[{\"path\":\"b.txt\",\"content\":\"B\"}]}")
echo "$B" | jq -c .

echo "### writer C: correct expected (new head) -> should be ok"
C=$(curl -s -b "$JAR" -X POST "$APP/api/refs/update" -H "Content-Type: application/json" \
  -d "{\"repo\":\"$REPO\",\"branch\":\"main\",\"expected_sha\":\"$NEW\",\"message\":\"C adds c.txt\",\"changes\":[{\"path\":\"c.txt\",\"content\":\"C\"}]}")
echo "$C" | jq -c .

echo "### verify tree reflects A+C (not B)"
curl -s -b "$JAR" "$APP/api/repos/$REPO/tree?ref=main" | jq -c "[.tree[].path]"

echo "### provenance records in Trestle (ref_updates)"
PASS=$(cat /opt/cp0/repro/.adminpass); J2=/tmp/adm7.jar
curl -s -c $J2 -X POST http://127.0.0.1:7350/admin/v1/session -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$PASS\"}" >/dev/null
curl -s -b $J2 http://127.0.0.1:7350/api/v1/collections/ref_updates/records | jq -c "[.items[].values | {repo, branch, old_sha: .old_sha[0:7], new_sha: .new_sha[0:7], provenance}]"

echo "### cleanup"
curl -s -X DELETE "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO" -H "Authorization: Bearer $TOK" | jq -c '{success}'
echo "### CP3 gate done"