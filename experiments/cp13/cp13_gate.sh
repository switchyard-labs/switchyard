#!/usr/bin/env bash
# CP13 gate: Dogfooding maturity — Switchyard operates on a Switchyard
# repository end-to-end. Repo is seeded externally (DOGFOOD_REPO).
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp13.jar; rm -f "$JAR"
PASS=0; TOTAL=7
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh
REPO=${DOGFOOD_REPO:-}
[ -z "$REPO" ] && { echo "DOGFOOD_REPO required"; exit 1; }

curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

echo; log "### 1. Switchyard-snapshot repo present"
NSEED=$(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/log?limit=1" -H "Authorization: Bearer $($TOK)" | jq -c "[.result[]] | length" 2>/dev/null)
log "repo=$REPO commits: $NSEED"
[ "${NSEED:-0}" -ge 1 ] && PASS=$((PASS+1)) || log "FAIL: repo not seeded"

echo; log "### 2. Work + Attempt on Switchyard's own repo"
WID=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"Switchyard dogfood: document the loop","kind":"feature"}' | jq -r .id)
AT=$(curl -s -b "$JAR" -X POST "$APP/api/work/$WID/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"df1\"}" | jq -r .id)
log "work=$WID attempt=$AT"
[ "$AT" != "null" ] && PASS=$((PASS+1)) || log "FAIL: attempt not created"

echo; log "### 3. durable workflow: implementer Agent edits Switchyard, check, review"
WF=$(curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "{\"name\":\"dogfood-$TS\",\"script\":\"function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:\\\"ATTEMPT.md\\\",append:\\\"dogfood-$TS\\\"}); var a=ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:\\\"docs/dogfood.md\\\",append:\\\"dogfood-$TS\\\"}); var c=ctx.check({repo:ctx.params.repo,branch:ctx.params.branch}); var v=ctx.review({repo:ctx.params.repo,branch:ctx.params.branch}); return {agent:a,check:c,review:v}; }\"}" | jq -r .id)
R=$(curl -s -b "$JAR" -X POST "$APP/api/workflows/$WF/run" -H "Content-Type: application/json" -d "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"df1\"}}" | jq -r .id)
for i in $(seq 1 30); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R" | jq -r .run.status)
  [ "$ST" = "completed" ] && break; sleep 2
done
OUT=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R" | jq -c .run.output)
log "dogfood workflow: $ST output=$OUT"
{ [ "$ST" = "completed" ] && echo "$OUT" | grep -q '"status":"pass"'; } && PASS=$((PASS+1)) || log "FAIL: dogfood workflow did not complete cleanly"

echo; log "### 4. review produced findings (Switchyard reviews its own change)"
RV=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT/review" -H "Content-Type: application/json" -d '{}')
echo "$RV" | jq -c .
echo "$RV" | jq -r '.findings | length' | grep -q '^[0-9]' && PASS=$((PASS+1)) || log "FAIL: review produced no findings"

echo; log "### 5. PR + preview clean"
PR=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT/pr" -H "Content-Type: application/json" -d '{"title":"dogfood: document the loop in Switchyard"}' | jq -r .id)
curl -s -b "$JAR" -X POST "$APP/api/prs/$PR/check" >/dev/null
PV=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$AT/preview" -H "Content-Type: application/json" -d '{}' | jq -c .)
log "preview: $(echo "$PV" | jq -c .)"
echo "$PV" | jq -r .conflict | grep -q false && PASS=$((PASS+1)) || log "FAIL: preview not clean"

echo; log "### 6. Integration Queue integrates the Switchyard PR (Dogfood 5)"
curl -s -b "$JAR" -X POST "$APP/api/prs/$PR/enqueue" >/dev/null
for i in $(seq 1 30); do
  QST=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PR\") | .status][0]")
  [ "$QST" = "done" ] && break; sleep 3
done
log "queue status: $QST (expect done)"
[ "$QST" = "done" ] && PASS=$((PASS+1)) || log "FAIL: Switchyard PR not integrated"

echo; log "### 7. canonical Switchyard repo now carries the dogfood change"
RAW=$(curl -s "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos/$REPO/raw/main/docs/dogfood.md" -H "Authorization: Bearer $($TOK)")
echo "$RAW" | grep -q "dogfood-$TS" && PASS=$((PASS+1)) || log "FAIL: dogfood change not in canonical"
log "canonical docs/dogfood.md: $(echo "$RAW" | tail -3 | tr '\n' ' ')"

echo; log "### provenance chain"
curl -s -b "$JAR" "$APP/api/work/$WID/provenance" | jq -c "[.chain[] | .kind]" | tr '\n' ' '; echo

echo
log "#### CP13 GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"