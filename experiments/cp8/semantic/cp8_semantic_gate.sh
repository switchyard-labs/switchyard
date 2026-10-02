#!/usr/bin/env bash
# Semantic-conflict experiment (post-cp13 review).
# Claim: "Git says the combined tree is fine, but Switchyard knows the two
# pieces of work are incompatible."
#   A edits api/version.json -> 2  AND  consumers/consumer.json -> 2 (together)
#   B adds consumers/gamma.json -> requires_version 3   (NEW file: clean merge)
#   contract: every consumer.requires_version must equal api.version
#   after A: consumer 2 == api 2 (ok), gamma absent (rule skipped) -> A passes
#   after B: gamma 3 != api 2 -> SEMANTIC conflict (git merge was clean)
#   => semantic finding + attempt semantic_conflict + Integration Queue blocked
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cpsem.jar; rm -f "$JAR"
PASS=0; TOTAL=5
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

REPO=cp8sem-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
mkdir -p api consumers
printf '{"version":1}\n' > api/version.json
printf '{"requires_version":1}\n' > consumers/consumer.json
printf '{"rules":[{"kind":"field_equals","a":"api/version.json:version","b":"consumers/consumer.json:requires_version"},{"kind":"field_equals","a":"api/version.json:version","b":"consumers/gamma.json:requires_version"}]}\n' > switchyard.contract.json
printf '# Attempt (seed)\n\n// baseline\n' > ATTEMPT.md
git add -A; git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
log "repo=$REPO seeded"

mkatt(){ curl -s -b "$JAR" -X POST "$APP/api/work/$1/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"$2\"}" | jq -r .id; }
pushfile(){ # $1=branch $2=path $3=content
  local WC=$(mktemp -d); cd "$WC"
  git init -q -b tmp; git remote add origin "$REMOTE"
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin main
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin "$1"
  git checkout -q -b work FETCH_HEAD
  git checkout -q origin/main -- ATTEMPT.md 2>/dev/null || true
  printf '%s\n' "$3" > "$2"
  git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "$1 edit $2"
  git -c http.extraHeader="Authorization: Bearer $RT" push -q origin work:"$1"
  cd "$W"
}

WID=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"semantic conflict experiment","kind":"feature"}' | jq -r .id)
ATA=$(mkatt "$WID" "a1"); ATB=$(mkatt "$WID" "b1")
pushfile a1 "api/version.json" '{"version":2}'
pushfile a1 "consumers/consumer.json" '{"requires_version":2}'
pushfile b1 "consumers/gamma.json" '{"requires_version":3}'
PA=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATA/pr" -H "Content-Type: application/json" -d '{"title":"PR A: bump api + consumer"}' | jq -r .id)
PB=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATB/pr" -H "Content-Type: application/json" -d '{"title":"PR B: add gamma consumer"}' | jq -r .id)
curl -s -b "$JAR" -X POST "$APP/api/prs/$PA/check" >/dev/null
curl -s -b "$JAR" -X POST "$APP/api/prs/$PB/check" >/dev/null
log "attempts A=$ATA B=$ATB"

echo; log "### 1. A integrates cleanly (api+consumer move together; contract holds)"
curl -s -b "$JAR" -X POST "$APP/api/prs/$PA/enqueue" >/dev/null
for i in $(seq 1 40); do
  QST=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PA\") | .status][0]")
  [ "$QST" = "done" ] && break; sleep 3
done
log "A queue: $QST"
[ "$QST" = "done" ] && PASS=$((PASS+1)) || log "FAIL: A not integrated"

echo; log "### 2. B preview: git merge CLEAN but semantic conflict detected"
PV=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATB/preview" -H "Content-Type: application/json" -d '{}')
echo "$PV" | jq -c .
CF=$(echo "$PV" | jq -r .conflict)
SF=$(echo "$PV" | jq -r .semantic_conflict)
ST=$(echo "$PV" | jq -r .status)
{ [ "$CF" = "false" ] && [ "$SF" = "true" ] && [ "$ST" = "semantic_conflict" ]; } && PASS=$((PASS+1)) || log "FAIL: expected clean git merge + semantic conflict (conflict=$CF semantic=$SF status=$ST)"

echo; log "### 3. semantic finding recorded"
FND=$(curl -s -b "$JAR" "$APP/api/findings" | jq -c "[.items[] | select(.target==\"$ATB\" and .severity==\"error\") | .message]")
echo "$FND"
echo "$FND" | grep -q "semantic conflict" && PASS=$((PASS+1)) || log "FAIL: no semantic finding"

echo; log "### 4. Integration Queue blocks B (semantic_conflict)"
curl -s -b "$JAR" -X POST "$APP/api/prs/$PB/enqueue" >/dev/null
for i in $(seq 1 30); do
  QST=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PB\") | .status][0]")
  [ "$QST" = "blocked" ] && break; sleep 3
done
QERR=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PB\") | .error][0]")
log "B queue: $QST error=$QERR"
{ [ "$QST" = "blocked" ] && echo "$QERR" | grep -q semantic_conflict; } && PASS=$((PASS+1)) || log "FAIL: B not blocked as semantic conflict"

echo; log "### 5. canonical holds only A's consistent change (B never leaked)"
V=$(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/api/version.json" -H "Authorization: Bearer $($TOK)")
C=$(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/consumers/consumer.json" -H "Authorization: Bearer $($TOK)")
G=$(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/consumers/gamma.json" -H "Authorization: Bearer $($TOK)")
log "canonical api=$V consumer=$C gamma-absent=$(echo "$G" | grep -qi "not found" && echo yes || echo no)"
{ echo "$V" | grep -q '"version":2' && echo "$C" | grep -q '"requires_version":2' && echo "$G" | grep -qi "not found"; } && PASS=$((PASS+1)) || log "FAIL: B leaked into canonical"

echo
log "#### SEMANTIC-CONFLICT EXPERIMENT: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"