#!/usr/bin/env bash
# CP2 gate test (kept as reproducible evidence): sign in -> see repository ->
# browse files -> create/view Work, plus idempotent event ingest.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp2.jar; rm -f "$JAR"

echo "### 0. seed a demo repository (Artifacts + git)"
TOK=$(/opt/cp0/switchyard/token.sh)
REPO=switchyard-demo-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" \
  -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); mkdir -p "$W/src"; cd "$W"
git init -q -b main; git remote add origin "$REMOTE"
echo "package main" > src/main.go
echo 'func main() { println("hello switchyard") }' >> src/main.go
echo "# $REPO" > README.md
git add -A; git -c user.name=seed -c user.email=seed@local commit -q -m "seed content"
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
echo "seeded $REPO head=$(git rev-parse HEAD)"

echo "### 1. register alice (fallback: login if taken)"
resp=$(curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}')
if echo "$resp" | grep -q username_taken; then
  curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
  echo "logged in as existing alice"
else
  echo "registered: $resp"
fi

echo "### 2. me"
curl -s -b "$JAR" "$APP/api/auth/me"; echo

echo "### 3. list repos"
curl -s -b "$JAR" "$APP/api/repos" | jq -c '{count:(.items|length), sample:(.items[0] | {name, default_branch, registered})}'

echo "### 4. get repo + tree"
curl -s -b "$JAR" "$APP/api/repos/$REPO" | jq -c . | head -c 300; echo
curl -s -b "$JAR" "$APP/api/repos/$REPO/tree?ref=main" | jq -c . | head -c 400; echo

echo "### 5. file content"
echo "--- README.md ---"; curl -s -b "$JAR" "$APP/api/repos/$REPO/content?ref=main&path=README.md"; echo
echo "--- src/main.go ---"; curl -s -b "$JAR" "$APP/api/repos/$REPO/content?ref=main&path=src/main.go"; echo

echo "### 6. create work + list + get"
wid=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"Fix session cleanup race","kind":"bug"}' | jq -r .id)
echo "work id=$wid"
curl -s -b "$JAR" "$APP/api/work" | jq -c '{count:(.items|length), first:(.items[0] | {id,title,kind,status,owner})}'
curl -s -b "$JAR" "$APP/api/work/$wid" | jq -c '{id,title,status,owner}'

echo "### 7. idempotent event ingest"
EV='{"type":"cf.artifacts.repo.pushed","repo_name":"'$REPO'","namespace":"'$NS'","payload":{"ref":"refs/heads/main","after":"abc123"},"occurred_at":"2026-10-02T16:10:00Z"}'
curl -s -X POST "$APP/api/events/ingest" -H "Content-Type: application/json" -d "$EV" | jq -c .
curl -s -X POST "$APP/api/events/ingest" -H "Content-Type: application/json" -d "$EV" | jq -c .
curl -s -b "$JAR" "$APP/api/events" | jq -c '{count:(.items|length), first:(.items[0] | {type, repo_name})}'

echo "### CP2 gate done"