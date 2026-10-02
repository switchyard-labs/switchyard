#!/usr/bin/env bash
# CP11 gate: Needs Attention + escalation decision packets + human approval.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp11.jar; rm -f "$JAR"
PASS=0; TOTAL=7
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

REPO=cp11-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
printf 'line1\nline2\nline3\n' > shared.txt
echo base > base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
log "repo=$REPO seeded"

mkatt(){ curl -s -b "$JAR" -X POST "$APP/api/work/$1/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"$2\"}" | jq -r .id; }
pushcontend(){ # $1=branch $2=newline3
  local WC=$(mktemp -d); cd "$WC"
  git init -q -b tmp; git remote add origin "$REMOTE"
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin main
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin "$1"
  git checkout -q -b work FETCH_HEAD
  git checkout -q origin/main -- shared.txt base.txt 2>/dev/null || true
  sed -i "1s/.*/$2/" shared.txt
  git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "contend $1"
  git -c http.extraHeader="Authorization: Bearer $RT" push -q origin work:"$1"
  cd "$W"
}

echo; log "### 1. create a conflict: A integrates, B conflicts -> attempt parked in conflict"
WID=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp11 conflict","kind":"feature"}' | jq -r .id)
ATA=$(mkatt "$WID" "a1"); ATB=$(mkatt "$WID" "b1")
pushcontend a1 "AAAA"; pushcontend b1 "BBBB"
curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATA/run" >/dev/null
curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATB/run" >/dev/null
PA=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATA/pr" -H "Content-Type: application/json" -d '{"title":"PR A"}' | jq -r .id)
PB=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATB/pr" -H "Content-Type: application/json" -d '{"title":"PR B"}' | jq -r .id)
curl -s -b "$JAR" -X POST "$APP/api/prs/$PA/check" >/dev/null
curl -s -b "$JAR" -X POST "$APP/api/prs/$PB/check" >/dev/null
curl -s -b "$JAR" -X POST "$APP/api/prs/$PA/enqueue" >/dev/null
sleep 14
curl -s -b "$JAR" -X POST "$APP/api/prs/$PB/enqueue" >/dev/null
sleep 20
ST=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PB\") | .status][0]")
log "B queue status: $ST (expect blocked)"
[ "$ST" = "blocked" ] && PASS=$((PASS+1)) || log "FAIL: B not blocked"
# also run the review to produce an error finding for B (preview conflict findings already exist)
ATST=$(curl -s -b "$JAR" "$APP/api/attempts/$ATB/preview" -H "Content-Type: application/json" -d '{}' | jq -r .status)
log "B attempt status: $ATST (expect conflict)"
[ "$ATST" = "conflict" ] && PASS=$((PASS+1)) || log "FAIL: attempt not in conflict"

echo; log "### 2. Needs Attention aggregates the conflict + open findings"
ATT=$(curl -s -b "$JAR" "$APP/api/attention")
echo "$ATT" | jq -c .
N=$(echo "$ATT" | jq -c "[.items[] | select(.kind==\"attempt_conflict\" or .kind==\"queue_blocked\" or .kind==\"open_finding\")] | length")
log "attention items: $N (expect >=2)"
[ "$N" -ge 2 ] && PASS=$((PASS+1)) || log "FAIL: attention empty"

echo; log "### 3. escalate B -> self-contained decision packet"
ESC=$(curl -s -b "$JAR" -X POST "$APP/api/attention/attempt/$ATB/escalate" -H "Content-Type: application/json" -d '{}')
echo "$ESC" | jq -c ".packet | {target_kind, target_id, repo, branch, base, files: [.conflicting_files[].file], suggestions: .suggested_resolutions}"
EID=$(echo "$ESC" | jq -r .id)
PKT=$(echo "$ESC" | jq -r .packet)
echo "$PKT" | jq -r '.conflicting_files | length' | grep -q '^[1-9]' && PASS=$((PASS+1)) || log "FAIL: packet has no conflicting files"
# verify the packet is self-contained: contains base + source versions
echo "$PKT" | jq -r '.conflicting_files[0] | .base_version' | grep -q . && PASS=$((PASS+1)) || log "FAIL: packet missing base_version"

echo; log "### 4. human decides take_ours -> B resolved + requeued + integrated"
DEC=$(curl -s -b "$JAR" -X POST "$APP/api/escalations/$EID/decide" -H "Content-Type: application/json" -d '{"decision":"take_ours"}')
echo "$DEC" | jq -c .
echo "$DEC" | jq -r '.outcome.status' | grep -q resolved && PASS=$((PASS+1)) || log "FAIL: decision did not resolve"
QX=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PB\") | .id][0]")
curl -s -b "$JAR" -X POST "$APP/api/queue/$QX/requeue" >/dev/null
for i in $(seq 1 25); do
  QST=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PB\") | .status][0]")
  [ "$QST" = "done" ] && break; sleep 3
done
log "B after decision + requeue: $QST (expect done)"
[ "$QST" = "done" ] && PASS=$((PASS+1)) || log "FAIL: B did not integrate after decision"

echo; log "### 5. workflow waiting for approval appears in attention + approve flow"
W1=$(curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "{\"name\":\"cp11-wf\",\"script\":\"function run(ctx){ ctx.wait_approval({reason:\\\"approve me\\\"}); return {ok:true}; }\"}" | jq -r .id)
R1=$(curl -s -b "$JAR" -X POST "$APP/api/workflows/$W1/run" -H "Content-Type: application/json" -d "{\"params\":{}}" | jq -r .id)
for i in $(seq 1 15); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R1" | jq -r .run.status)
  [ "$ST" = "waiting_approval" ] && break; sleep 2
done
log "workflow run status: $ST (expect waiting_approval)"
[ "$ST" = "waiting_approval" ] && PASS=$((PASS+1)) || log "FAIL: workflow not waiting approval"
E2=$(curl -s -b "$JAR" -X POST "$APP/api/attention/workflow/$R1/escalate" -H "Content-Type: application/json" -d '{}' | jq -r .id)
curl -s -b "$JAR" -X POST "$APP/api/escalations/$E2/decide" -H "Content-Type: application/json" -d '{"decision":"approve"}' | jq -c .
for i in $(seq 1 10); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R1" | jq -r .run.status)
  [ "$ST" = "completed" ] && break; sleep 2
done
log "workflow run after approve decision: $ST (expect completed)"
[ "$ST" = "completed" ] && PASS=$((PASS+1)) || log "FAIL: workflow not completed after approval"

echo
log "#### CP11 GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"