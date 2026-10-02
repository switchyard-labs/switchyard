#!/usr/bin/env bash
# Round-trip script for the Artifacts Git plane. Research-only, staged for
# Checkpoint 0. Requires: CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, a
# Workers Paid account, and the round-trip Worker deployed (or wrangler dev).
set -euo pipefail

ACCOUNT_ID="${CLOUDFLARE_ACCOUNT_ID:?set CLOUDFLARE_ACCOUNT_ID}"
NS="${ARTIFACTS_NAMESPACE:-switchyard-cp0}"
BASE="https://api.cloudflare.com/client/v4/accounts/$ACCOUNT_ID/artifacts/namespaces/$NS"

api() { # method path [data]
  local method=$1 path=$2 data=${3:-}
  if [ -n "$data" ]; then
    curl -fsS -X "$method" "$BASE$path" -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN" \
      -H "Content-Type: application/json" -d "$data"
  else
    curl -fsS -X "$method" "$BASE$path" -H "Authorization: Bearer $CLOUDFLARE_API_TOKEN"
  fi
}

echo "== 1. create baseline repo =="
BASE_REPO="cp0-baseline-$(date +%s)"
resp=$(api POST "/repos" "{\"name\":\"$BASE_REPO\",\"default_branch\":\"main\"}")
echo "$resp"
REMOTE=$(echo "$resp" | jq -r .result.remote)
TOKEN=$(echo "$resp" | jq -r .result.token)
echo "remote=$REMOTE"

echo "== 2. fork as task repo =="
TASK_REPO="cp0-task-$(date +%s)"
fresp=$(api POST "/repos/$BASE_REPO/fork" "{\"name\":\"$TASK_REPO\",\"default_branch_only\":true}")
TASK_REMOTE=$(echo "$fresp" | jq -r .result.remote)
echo "task_remote=$TASK_REMOTE"

echo "== 3. mint short-lived write token =="
tresp=$(api POST "/tokens" "{\"repo\":\"$TASK_REPO\",\"scope\":\"write\",\"ttl\":3600}")
WT=$(echo "$tresp" | jq -r .result.plaintext)
echo "write token acquired (expires=$(echo "$WT" | sed 's/.*expires=//'))"

echo "== 4. clone, commit, push =="
W=$(mktemp -d); cd "$W"
git -c http.extraHeader="Authorization: Bearer $WT" clone "$TASK_REMOTE" work >/dev/null 2>&1 || {
  git init -q work && cd work && git remote add origin "$TASK_REMOTE"
}
cd work
git -c user.name="cp0-agent" -c user.email="cp0@local" commit -q --allow-empty -m "cp0 round trip" >/dev/null 2>&1 || true
echo "hello from cp0" > roundtrip.txt
git add roundtrip.txt
git -c user.name="cp0-agent" -c user.email="cp0@local" commit -q -m "cp0 round trip push"
git -c http.extraHeader="Authorization: Bearer $WT" push origin HEAD:main
echo "== pushed. awaiting cf.artifacts.repo.pushed event =="

echo "== 5. (manual) observe event via Queues subscription, then POST to Worker /ingest =="
echo "done. baseline=$BASE_REPO task=$TASK_REPO"