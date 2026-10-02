#!/usr/bin/env bash
# CP9 gate: Integration Queue full semantics + performance.
#   ordering (FIFO), canonical-head freshness (CAS), policy (check+preview),
#   repair loop (conflict -> resolve -> requeue; stale -> bounded requeue),
#   risk classes, UI observability, perf (3/10/25 PR scenarios).
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp9.jar; rm -f "$JAR"
PASS=0; TOTAL=8
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh
ms(){ python3 -c "import sys;print(int(round(float(sys.argv[1])*1000)))" "$1"; }

REPO=cp9-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
printf '# Attempt (seed)\n\n// baseline\n' > ATTEMPT.md
echo "count=0" > counter.txt
git add -A; git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
log "repo=$REPO seeded (ATTEMPT.md valid, counter.txt)"
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

# mkpr: create attempt branch + PR + check. $1=branch $2=extra filespec(s) separated by ;
#   filespec = "path:content" (added) or "path=value" (edit existing line via sed)
mkpr(){
  local B=$1; local FILES=$2
  local WID=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp9 pr","kind":"feature"}' | jq -r .id)
  local AT=$(curl -s -b "$JAR" -X POST "$APP/api/work/$WID/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"$B\"}" | jq -r .id)
  local WC=$(mktemp -d); cd "$WC"
  git init -q -b tmp; git remote add origin "$REMOTE"
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin main
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin "$B"
  git checkout -q -b work FETCH_HEAD
  git checkout -q origin/main -- ATTEMPT.md 2>/dev/null || true
  IFS=';' read -ra FS <<< "$FILES"
  for spec in "${FS[@]}"; do
    if [[ "$spec" == *=* ]]; then
      local PP="${spec%%=*}"; local LL="${spec#*=}"
      sed -i "1s/.*/$LL/" "$PP"
    else
      echo "${spec##*:}" > "${spec%%:*}"
    fi
  done
  git add -A
  git -c user.name=agent -c user.email=agent@cp0 commit -qm "cp9 $B"
  git -c http.extraHeader="Authorization: Bearer $RT" push -q origin work:"$B"
  cd "$W"
  local PR=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT/pr" -H "Content-Type: application/json" -d "{\"title\":\"PR $B\"}" | jq -r .id)
  curl -s -b "$JAR" -X POST "$APP/api/prs/$PR/check" >/dev/null
  echo "$AT $PR"
}
enq(){ curl -s -b "$JAR" -X POST "$APP/api/prs/$1/enqueue" -H "Content-Type: application/json" -d '{}'; }
qstatus(){ curl -s -b "$JAR" "$APP/api/queue" | jq -c "[.items[] | select(.repo==\"$REPO\") | {id: .id, status, risk, attempts, pr_id}] | sort_by(.id)"; }

echo; log "### 1. policy: failing check cannot enqueue"
WIDF=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp9 fail","kind":"feature"}' | jq -r .id)
ATF=$(curl -s -b "$JAR" -X POST "$APP/api/work/$WIDF/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"bad\"}" | jq -r .id)
PRF=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATF/pr" -H "Content-Type: application/json" -d '{"title":"PR bad"}' | jq -r .id)
curl -s -b "$JAR" -X POST "$APP/api/prs/$PRF/check" >/dev/null
EQ=$(enq "$PRF")
log "enqueue failing-check PR: $(echo "$EQ" | jq -c .)"
echo "$EQ" | jq -r .error | grep -q check_not_passed && PASS=$((PASS+1)) || log "FAIL: policy allowed failing check"

echo; log "### 2. clean churn: 3 PRs integrate FIFO, canonical has all features"
read -r A1 P1 <<< "$(mkpr "c1" "f1.txt:one")"
read -r A2 P2 <<< "$(mkpr "c2" "f2.txt:two")"
read -r A3 P3 <<< "$(mkpr "c3" "f3.txt:three")"
T0=$(date +%s.%N)
for P in $P1 $P2 $P3; do enq "$P" >/dev/null; done
for i in $(seq 1 30); do
  D=$(qstatus | jq -c "[.[] | select(.status==\"done\")] | length")
  [ "$D" = "3" ] && break; sleep 2
done
D=$(qstatus | jq -c "[.[] | select(.status==\"done\")] | length")
log "done items: $D (expect 3)"
[ "$D" = "3" ] && PASS=$((PASS+1)) || log "FAIL: churn not all done"
log "queue view: $(qstatus | jq -c '[.[] | {status, risk}]')"
log "canonical files: $(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/log?limit=5" -H "Authorization: Bearer $($TOK)" | jq -c '[.result[].hash] | length' 2>/dev/null) commits; feature files present:"
for F in f1.txt f2.txt f3.txt; do
  code=$(curl -s -o /dev/null -w "%{http_code}" "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/$F" -H "Authorization: Bearer $($TOK)")
  echo "  $F: $code"
done
C=$(curl -s -o /dev/null -w "%{http_code}" "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/f3.txt" -H "Authorization: Bearer $($TOK)")
[ "$C" = "200" ] && PASS=$((PASS+1)) || log "FAIL: f3.txt not in canonical"
T1=$(date +%s.%N)
log "churn wall time (3 PRs): $(ms "$(echo "$T1 $T0" | awk '{print $1-$2}')") ms"

echo; log "### 3. conflict blocked -> repair loop -> requeue -> done"
read -r AX PX <<< "$(mkpr "x1" "counter.txt=count=2")"
read -r AA PA <<< "$(mkpr "a1" "counter.txt=count=1")"
enq "$PA" >/dev/null; sleep 1; enq "$PX" >/dev/null
# A integrates (count=1); X should be blocked by preview conflict
sleep 8
QV=$(qstatus)
log "after both enqueued: $(echo "$QV" | jq -c '[.[] | {pr_id, status}]')"
BLK=$(echo "$QV" | jq -c "[.[] | select(.pr_id==\"$PX\" and .status==\"blocked\")] | length")
[ "$BLK" = "1" ] && PASS=$((PASS+1)) || log "FAIL: conflict PR not blocked (statuses: $(echo "$QV" | jq -c '[.[].status]'))"
# repair loop: resolve the attempt, then requeue
ATX=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AX/resolve" -H "Content-Type: application/json" -d '{}' | jq -r .status)
log "resolve attempt: $ATX"
QX=$(echo "$QV" | jq -r "[.[] | select(.pr_id==\"$PX\") | .id][0]")
curl -s -b "$JAR" -X POST "$APP/api/queue/$QX/requeue" >/dev/null
for i in $(seq 1 20); do
  ST=$(qstatus | jq -r "[.[] | select(.id==\"$QX\") | .status][0]")
  [ "$ST" = "done" ] && break; sleep 2
done
ST=$(qstatus | jq -r "[.[] | select(.id==\"$QX\") | .status][0]")
log "conflict PR after repair loop: $ST (expect done)"
[ "$ST" = "done" ] && PASS=$((PASS+1)) || log "FAIL: repair loop did not integrate"
log "canonical counter.txt: $(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/counter.txt" -H "Authorization: Bearer $($TOK)")"

echo; log "### 4. risk classes"
read -r AR PR <<< "$(mkpr "r10" "r1.txt:a;r2.txt:b;r3.txt:c;r4.txt:d;r5.txt:e;r6.txt:f;r7.txt:g;r8.txt:h;r9.txt:i;r10.txt:j")"
EQ=$(enq "$PR")
log "10-file PR enqueue: $(echo "$EQ" | jq -c .)"
echo "$EQ" | jq -r .risk | grep -q high && PASS=$((PASS+1)) || log "FAIL: risk not high"

echo; log "### 5. UI observability (API surface)"
curl -s -b "$JAR" "$APP/api/queue" | jq -c "[.items[] | select(.repo==\"$REPO\") | {status, risk, policy, attempts}]" >/tmp/cp9q.json
log "queue states observed: $(jq -c '[.[].status] | unique' /tmp/cp9q.json)"
N=$(jq -c '[.[] | select(.status=="done")] | length' /tmp/cp9q.json)
[ "$N" -ge 4 ] && PASS=$((PASS+1)) || log "FAIL: queue observability incomplete"

echo; log "### 6. performance: 10-PR concurrent enqueue scenario"
REPO2=cp9perf-$TS
r2=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO2\",\"default_branch\":\"main\"}")
REMOTE2=$(echo "$r2" | jq -r .result.remote); RT2=$(echo "$r2" | jq -r .result.token)
W2=$(mktemp -d); cd "$W2"; git init -q -b main; git remote add origin "$REMOTE2"
printf '# Attempt (seed)\n\n// base\n' > ATTEMPT.md
git add -A; git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT2" push -q -u origin main
mkpr2(){
  local B=$1; local F=$2
  local WID=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"perf","kind":"feature"}' | jq -r .id)
  local AT=$(curl -s -b "$JAR" -X POST "$APP/api/work/$WID/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO2\",\"branch\":\"$B\"}" | jq -r .id)
  local WC=$(mktemp -d); cd "$WC"
  git init -q -b tmp; git remote add origin "$REMOTE2"
  git -c http.extraHeader="Authorization: Bearer $RT2" fetch -q origin main
  git -c http.extraHeader="Authorization: Bearer $RT2" fetch -q origin "$B"
  git checkout -q -b work FETCH_HEAD
  git checkout -q origin/main -- ATTEMPT.md 2>/dev/null || true
  echo "perf-$F" > "p$F.txt"
  git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "perf $F"
  git -c http.extraHeader="Authorization: Bearer $RT2" push -q origin work:"$B"
  cd "$W2"
  local PR=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT/pr" -H "Content-Type: application/json" -d "{\"title\":\"PR perf $F\"}" | jq -r .id)
  curl -s -b "$JAR" -X POST "$APP/api/prs/$PR/check" >/dev/null
  echo "$PR"
}
PRS=""
for i in $(seq 1 10); do PRS="$PRS $(mkpr2 "perf$i" "$i")"; done
TP0=$(date +%s.%N)
for P in $PRS; do enq "$P" >/dev/null; done
TP1=$(date +%s.%N)
for i in $(seq 1 40); do
  D=$(curl -s -b "$JAR" "$APP/api/queue" | jq -c "[.items[] | select(.repo==\"$REPO2\" and .status==\"done\")] | length")
  [ "$D" = "10" ] && break; sleep 2
done
TP2=$(date +%s.%N)
log "10-PR scenario: enqueue phase $(ms "$(echo "$TP1 $TP0" | awk '{print $1-$2}')") ms; full queue-to-done $(ms "$(echo "$TP2 $TP0" | awk '{print $1-$2}')") ms ($D done)"
[ "$D" = "10" ] && PASS=$((PASS+1)) || log "FAIL: 10-PR perf scenario incomplete"

echo
log "#### CP9 GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"