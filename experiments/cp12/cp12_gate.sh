#!/usr/bin/env bash
# CP12 gate: org/policy/risk/audit/fleet. Policy constrains an org's agents
# without breaking the simple (policy-less) repo case.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp12.jar; rm -f "$JAR"
PASS=0; TOTAL=8
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

REPO=cp12-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"
mkdir -p config; echo "secret" > config/secrets.txt; echo "ok" > public.txt
printf '# Attempt\n\n// base\n' > ATTEMPT.md
git add -A; git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
log "repo=$REPO seeded (config/secrets.txt + public.txt)"

echo; log "### 1. create org owning the repo + policy denying implementer writes to config/secrets/*"
OID=$(curl -s -b "$JAR" -X POST "$APP/api/orgs" -H "Content-Type: application/json" -d "{\"name\":\"acme-$TS\",\"repos\":[\"$REPO\"],\"members\":[\"alice\"]}" | jq -r .id)
log "org=$OID"
POL=$(curl -s -b "$JAR" -X POST "$APP/api/orgs/$OID/policies" -H "Content-Type: application/json" -d '{"name":"protect-secrets","rules":{"agent":{"deny_paths":["config/*"]},"integrate":{"require_approval_risk":"high"},"workflow":{"budget_steps":100}}}')
echo "$POL" | jq -c .
[ "$(echo "$POL" | jq -r .id | cut -c1-4)" = "pol_" ] && PASS=$((PASS+1)) || log "FAIL: policy not created"

echo; log "### 2. agent attempt on a protected path is denied (policy), public path allowed"
WID=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp12 denied","kind":"feature"}' | jq -r .id)
ATD=$(curl -s -b "$JAR" -X POST "$APP/api/work/$WID/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"bd\"}" | jq -r .id)
# a workflow whose agent targets config/secrets.txt
WFP=$(curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "{\"name\":\"cp12-denied-wf\",\"script\":\"function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:\\\"config/secrets.txt\\\",append:\\\"tamper\\\"}); return {ok:true}; }\"}" | jq -r .id)
RDN=$(curl -s -b "$JAR" -X POST "$APP/api/workflows/$WFP/run" -H "Content-Type: application/json" -d "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"bd\"}}" | jq -r .id)
for i in $(seq 1 15); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$RDN" | jq -r .run.status)
  [ "$ST" = "failed" ] && break; sleep 2
done
ERR=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$RDN" | jq -r .run.error)
log "denied workflow run: status=$ST error=$ERR"
[ "$ST" = "failed" ] && echo "$ERR" | grep -qi "policy" && PASS=$((PASS+1)) || log "FAIL: protected path not denied"
# public path allowed
WFA=$(curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "{\"name\":\"cp12-ok-wf\",\"script\":\"function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:\\\"public.txt\\\",append:\\\"ok change\\\"}); return {ok:true}; }\"}" | jq -r .id)
RDA=$(curl -s -b "$JAR" -X POST "$APP/api/workflows/$WFA/run" -H "Content-Type: application/json" -d "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"ba\"}}" | jq -r .id)
for i in $(seq 1 15); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$RDA" | jq -r .run.status)
  [ "$ST" = "completed" ] && break; sleep 2
done
log "allowed workflow run: $ST (expect completed) -> simple repo case not broken"
[ "$ST" = "completed" ] && PASS=$((PASS+1)) || log "FAIL: allowed path broken"

echo; log "### 3. audit trail records both policy decisions"
AUD=$(curl -s -b "$JAR" "$APP/api/audit" | jq -c "[.items[] | select(.org==\"$OID\") | {action, decision}]")
echo "$AUD"
DEN=$(echo "$AUD" | jq -c "[.[] | select(.decision==\"deny\")] | length")
[ "$DEN" -ge 1 ] && PASS=$((PASS+1)) || log "FAIL: no deny audit entry"

echo; log "### 4. simple repo case (no org/policy) is unconstrained"
REPO2=cp12nopol-$TS
r2=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO2\",\"default_branch\":\"main\"}")
REMOTE2=$(echo "$r2" | jq -r .result.remote); RT2=$(echo "$r2" | jq -r .result.token)
W2=$(mktemp -d); cd "$W2"; git init -q -b main; git remote add origin "$REMOTE2"
printf '# Attempt\n\n// base\n' > ATTEMPT.md; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT2" push -q -u origin main
WFS=$(curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "{\"name\":\"cp12-nopol\",\"script\":\"function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:\\\"config/secrets.txt\\\",append:\\\"x\\\"}); return {ok:true}; }\"}" | jq -r .id)
RS=$(curl -s -b "$JAR" -X POST "$APP/api/workflows/$WFS/run" -H "Content-Type: application/json" -d "{\"params\":{\"repo\":\"$REPO2\",\"branch\":\"bs\"}}" | jq -r .id)
for i in $(seq 1 15); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$RS" | jq -r .run.status)
  [ "$ST" = "completed" ] && break; sleep 2
done
log "no-policy repo run: $ST (expect completed - simple case not broken)"
[ "$ST" = "completed" ] && PASS=$((PASS+1)) || log "FAIL: simple repo case broken by policy"

echo; log "### 5. high-risk integration escalates under policy (require_approval_risk=high)"
# build a high-risk PR (10 files) on the org repo
WID2=$(curl -s -b "$JAR" -X POST "$APP/api/work" -H "Content-Type: application/json" -d '{"title":"cp12 highrisk","kind":"feature"}' | jq -r .id)
ATH=$(curl -s -b "$JAR" -X POST "$APP/api/work/$WID2/attempts" -H "Content-Type: application/json" -d "{\"repo\":\"$REPO\",\"branch\":\"bh\"}" | jq -r .id)
WC=$(mktemp -d); cd "$WC"
git init -q -b tmp; git remote add origin "$REMOTE"
git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin main
git -c http.extraHeader="Authorization: Bearer $RT" fetch -q origin bh
git checkout -q -b work FETCH_HEAD
git checkout -q origin/main -- ATTEMPT.md config/secrets.txt public.txt 2>/dev/null || true
for i in $(seq 1 10); do echo "f$i" > "extra$i.txt"; done
git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "high risk 10 files"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin work:bh
cd "$W"
PRH=$(curl -s -b "$JAR" -X POST "$APP/api/attempts/$ATH/pr" -H "Content-Type: application/json" -d '{"title":"PR high"}' | jq -r .id)
curl -s -b "$JAR" -X POST "$APP/api/prs/$PRH/check" >/dev/null
EQ=$(curl -s -b "$JAR" -X POST "$APP/api/prs/$PRH/enqueue" -H "Content-Type: application/json" -d '{}')
echo "enqueue high-risk: $(echo "$EQ" | jq -c .)"
RKH=$(echo "$EQ" | jq -r .risk)
for i in $(seq 1 20); do
  QST=$(curl -s -b "$JAR" "$APP/api/queue" | jq -r "[.items[] | select(.pr_id==\"$PRH\") | .status][0]")
  [ "$QST" = "blocked" ] && break; sleep 3
done
log "high-risk queue status: $QST (expect blocked via policy), risk=$RKH"
{ [ "$QST" = "blocked" ] && [ "$RKH" = "high" ]; } && PASS=$((PASS+1)) || log "FAIL: high-risk not escalated/blocked"

echo; log "### 6. fleet view + workflow budget policy"
FLEET=$(curl -s -b "$JAR" "$APP/api/orgs/$OID/fleet")
echo "$FLEET" | jq -c '{org: .org.name, repos: .repos, queue_items: .queue_items, workflow_runs: .workflow_runs, escalations: .escalations}'
[ "$(echo "$FLEET" | jq -r .repos)" -ge 1 ] && PASS=$((PASS+1)) || log "FAIL: fleet view empty"
# workflow budget policy: org policy budget_steps=100; a run with budget 200 should be denied
WFB=$(curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "{\"name\":\"cp12-budget\",\"script\":\"function run(ctx){ ctx.note({text:\\\"x\\\"}); return {ok:true}; }\"}" | jq -r .id)
RB=$(curl -s -b "$JAR" -X POST "$APP/api/workflows/$WFB/run" -H "Content-Type: application/json" -d "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"bq\"},\"budget_steps\":200}" | jq -r .id)
for i in $(seq 1 10); do
  ST=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$RB" | jq -r .run.status)
  [ "$ST" = "failed" ] && break; sleep 2
done
ERRB=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$RB" | jq -r .run.error)
log "budget-denied run: $ST error=$ERRB"
{ [ "$ST" = "failed" ] && echo "$ERRB" | grep -qi "policy"; } && PASS=$((PASS+1)) || log "FAIL: workflow budget not enforced"

echo; log "### 7. risk model (blast x uncertainty) observable"
log "riskScore for high-risk branch was high ($RKH)"

echo
log "#### CP12 GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"