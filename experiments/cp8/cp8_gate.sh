#!/usr/bin/env bash
# CP8 gate: reviews/findings + draft contention + preview conflict detection
# + conflict routing with a bounded repair loop.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp8.jar; rm -f "$JAR"
PASS=0; TOTAL=6
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

REPO=cp8-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
printf 'line1\nline2\nline3\nline4\n' > shared.txt
echo base > base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
log "repo=$REPO seeded (shared.txt 4 lines + base.txt)"
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

# create attempt via API (creates branch + ATTEMPT.md init)
mkattempt(){ curl -s -b "$JAR" -X POST "$APP/api/work/$1/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"$2\"}" | jq -r .id; }
runatt(){ curl -s -b "$JAR" -X POST "$APP/api/attempts/$1/run" >/dev/null; }
# push a change onto an existing attempt branch (bring shared.txt in from main, edit a line)
pushcontend(){ # $1=branch $2=newline3
  local B=$1; local VAL=$2
  local WC=$(mktemp -d); cd "$WC"
  git init -q -b tmp; git remote add origin "$REMOTE"
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin main
  git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin "$B"
  git checkout -q -b work FETCH_HEAD
  git checkout -q FETCH_HEAD~0 -- . 2>/dev/null || true
  # bring main's shared.txt into the attempt branch if absent
  git checkout -q origin/main -- shared.txt base.txt 2>/dev/null || true
  sed -i "s/^line3.*/$VAL/" shared.txt
  git add -A
  git -c user.name=agent -c user.email=agent@cp0 commit -qm "contend edit $B: line3 -> $VAL"
  git -c http.extraHeader="Authorization: Bearer $RT" push -q origin work:"$B"
  cd "$W"
}

echo; log "### 1. review produces structured findings"
WID1=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp8 review","kind":"feature"}' | jq -r .id)
AT1=$(mkattempt "$WID1" "a1"); runatt "$AT1"
F1=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT1/review" -H "Content-Type: application/json" -d '{}')
echo "$F1" | jq -c .
NF=$(echo "$F1" | jq -c "[.findings[]] | length")
log "findings produced: $NF (expect >=1)"
[ "$NF" -ge 1 ] && PASS=$((PASS+1)) || log "FAIL: no findings"
log "findings listing: $(curl -s -b "$JAR" "$APP/api/findings" | jq -c '[.items[] | {severity, status}] | unique_by(.severity)')"

echo; log "### 2. two competing attempts produce comparable PRs"
WID2=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp8 contention","kind":"feature"}' | jq -r .id)
AT2A=$(mkattempt "$WID2" "a2"); AT2B=$(mkattempt "$WID2" "b2")
pushcontend a2 "AAAA"
pushcontend b2 "BBBB"
runatt "$AT2A"; runatt "$AT2B"
PA=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT2A/pr" -H "Content-Type: application/json" -d '{"title":"PR A"}' | jq -r .id)
PB=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT2B/pr" -H "Content-Type: application/json" -d '{"title":"PR B"}' | jq -r .id)
log "PRs: A=$PA B=$PB"
PRN=$(curl -s -b "$JAR" "$APP/api/prs" | jq -c "[.items[] | select(.repo==\"$REPO\" and .status==\"open\")] | length")
[ "$PRN" = "2" ] && PASS=$((PASS+1)) || log "FAIL: expected 2 comparable PRs, got $PRN"

echo; log "### 3. A integrates first; B preview detects conflict"
curl -s -b "$JAR" -X POST "$APP/api/prs/$PA/check" >/dev/null
IA=$(curl -s -b "$JAR" -X POST "$APP/api/prs/$PA/integrate" | jq -c .)
log "A integrate: $IA"
PV=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT2B/preview" -H "Content-Type: application/json" -d '{}')
echo "$PV" | jq -c .
CONF=$(echo "$PV" | jq -r .conflict)
[ "$CONF" = "true" ] && PASS=$((PASS+1)) || log "FAIL: preview did not detect conflict"
log "conflict findings for B: $(curl -s -b "$JAR" "$APP/api/findings" | jq -c "[.items[] | select(.target==\"$AT2B\" and .severity==\"error\") | {file, status}]")"

echo; log "### 4. conflict-resolver bounded repair loop resolves B"
RES=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT2B/resolve" -H "Content-Type: application/json" -d '{}')
echo "$RES" | jq -c .
RSTATUS=$(echo "$RES" | jq -r .status)
[ "$RSTATUS" = "resolved" ] && PASS=$((PASS+1)) || log "FAIL: resolver did not resolve (got $RSTATUS)"
PV2=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT2B/preview" -H "Content-Type: application/json" -d '{}' | jq -c .)
log "re-preview after resolution: $(echo "$PV2" | jq -c .)"
echo "$PV2" | jq -r .conflict | grep -q false && PASS=$((PASS+1)) || log "FAIL: still conflicting after resolution"

echo; log "### 5. B integrates cleanly after resolution"
curl -s -b "$JAR" -X POST "$APP/api/prs/$PB/check" >/dev/null
IB=$(curl -s -b "$JAR" -X POST "$APP/api/prs/$PB/integrate" | jq -c .)
echo "$IB" | jq -c .
echo "$IB" | jq -r .status | grep -q integrated && PASS=$((PASS+1)) || log "FAIL: B did not integrate"
log "canonical shared.txt line3: $(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/shared.txt" -H "Authorization: Bearer $($TOK)" | sed -n 3p)"

echo
log "#### CP8 GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"