#!/usr/bin/env bash
# Draft concurrency gate (post-cp13 review): optimistic concurrency / revision
# CAS with no last-writer-wins, plus external-branch-movement detection.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cpdraft.jar; rm -f "$JAR"
PASS=0; TOTAL=6
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

REPO=cp10draft-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
printf 'hello\nworld\n' > base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
A=$(git rev-parse HEAD)
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
log "repo=$REPO seeded main@$A"

save(){ # $1=content $2=expected_rev
  curl -s -b "$JAR" -X POST "$APP/api/drafts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"main\",\"path\":\"base.txt\",\"content\":\"$1\",\"expected_revision\":\"$2\",\"base_sha\":\"$A\"}"
}

echo; log "### Race A: two sessions — both loaded rev 1; second save is stale"
R0=$(save "baseline" "")
log "baseline save: $(echo "$R0" | jq -c .)"
REV=$(echo "$R0" | jq -r .revision)   # both sessions now hold rev $REV
R1=$(save "session-a-content" "$REV")
log "S1 save (expected=$REV): $(echo "$R1" | jq -c .)"
[ "$(echo "$R1" | jq -r .revision)" = "$((REV+1))" ] && R1OK=1 || R1OK=0
R2=$(save "session-b-stale-content" "$REV")
log "S2 save (expected=$REV, now $((REV+1))): $(echo "$R2" | jq -c .)"
ST=$(echo "$R2" | jq -r .error)
{ [ "$ST" = "draft_stale" ] && [ "$(echo "$R2" | jq -r .current_revision)" = "$((REV+1))" ] && [ "$R1OK" = "1" ]; } && PASS=$((PASS+1)) || log "FAIL: stale save not rejected"
# newer content preserved
D=$(curl -s -b "$JAR" "$APP/api/drafts/$REPO/main/base.txt")
echo "$D" | jq -r .content | grep -q "session-a-content" && PASS=$((PASS+1)) || log "FAIL: newer content overwritten"

echo; log "### Race B: Agent-vs-human — agent result from an old revision is stale"
REV2=$(curl -s -b "$JAR" "$APP/api/drafts/$REPO/main/base.txt" | jq -r .revision)
RB=$(save "human-edit-v2" "$REV2")
log "human save (expected=$REV2): $(echo "$RB" | jq -c .)"
REV3=$(echo "$RB" | jq -r .revision)
RA=$(save "agent-result-based-on-old-rev" "$REV2")
log "agent result (based on rev $REV2, now $REV3): $(echo "$RA" | jq -c .)"
[ "$(echo "$RA" | jq -r .error)" = "draft_stale" ] && PASS=$((PASS+1)) || log "FAIL: agent stale result accepted"
D=$(curl -s -b "$JAR" "$APP/api/drafts/$REPO/main/base.txt")
echo "$D" | jq -r .content | grep -q "human-edit-v2" && PASS=$((PASS+1)) || log "FAIL: agent overwrote newer human edit"

echo; log "### Race C: external branch movement — commit rejects stale base"
# reset the draft to a known base (A), then push the branch externally to B
echo "externally pushed" > base.txt; git add -A
git -c user.name=ext -c user.email=ext@local commit -qm "external push"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin main
B=$(git rev-parse HEAD)
log "branch moved A=$A -> B=$B (external push)"
# the draft was based on A (base_sha=A recorded at first save); commit it now
DID=$(curl -s -b "$JAR" "$APP/api/drafts/$REPO/main/base.txt" | jq -r .id)
CC=$(curl -s -b "$JAR" -X POST "$APP/api/drafts/$DID/commit" -H "Content-Type: application/json" -d '{"message":"commit on stale base"}')
log "commit after external push: $(echo "$CC" | jq -c .)"
[ "$(echo "$CC" | jq -r .error)" = "stale_base" ] && PASS=$((PASS+1)) || log "FAIL: commit did not detect stale base"
# draft remains recoverable
D=$(curl -s -b "$JAR" "$APP/api/drafts/$REPO/main/base.txt")
echo "$D" | jq -r .content | grep -q "human-edit-v2" && PASS=$((PASS+1)) || log "FAIL: draft content lost"

echo
log "#### DRAFT CAS GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"