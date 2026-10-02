#!/usr/bin/env bash
# CP7 gate: durable workflows.
#   authoring model: JS run(ctx); Switchyard ops are durable steps.
#   durability: replay-from-top; wf_steps dedupe; crash/restart; idempotent
#   resume; retry; cancellation; parallel branches; bounded spawn/recursion;
#   spawn child + wait on approval; no duplicated external effects.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp7.jar; rm -f "$JAR"
PASS=0; TOTAL=9
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

# setup repo
REPO=cp7-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"; echo base>base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
A=$(git rev-parse HEAD)
log "repo=$REPO seeded main@$A"
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null

mkworkflow(){ # $1=name $2=script -> id
  curl -s -b "$JAR" -X POST "$APP/api/workflows" -H "Content-Type: application/json" -d "$(jq -nc --arg n "$1" --arg s "$2" '{name:$n, script:$s}')" | jq -r .id
}
runwf(){ # $1=workflowId $2=paramsJson -> run id
  curl -s -b "$JAR" -X POST "$APP/api/workflows/$1/run" -H "Content-Type: application/json" -d "$2" | jq -r .id
}
runstatus(){ curl -s -b "$JAR" "$APP/api/workflow_runs/$1" | jq -r .run.status; }
runerror(){ curl -s -b "$JAR" "$APP/api/workflow_runs/$1" | jq -r .run.error; }
wait_status(){ # $1=runid $2=status $3=timeout_s
  for i in $(seq 1 "$3"); do
    s=$(runstatus "$1"); [ "$s" = "$2" ] && return 0; sleep 1
  done
  log "timeout waiting $1 -> $2 (now $(runstatus "$1"))"
  return 1
}
exec_count(){ # $1=runid -> count of executions with attempt_id=wf:<runid>
  PASS=$(cat /opt/cp0/repro/.adminpass); J2=/tmp/cp7adm.jar
  curl -s -c $J2 -X POST http://127.0.0.1:7350/admin/v1/session -H "Content-Type: application/json" -d "{\"username\":\"admin\",\"password\":\"$PASS\"}" >/dev/null
  curl -s -b $J2 "http://127.0.0.1:7350/api/v1/collections/executions/records" | jq -c "[.items[] | select(.values.attempt_id==\"wf:$1\")] | length"
}

echo; log "### 1. basic durable run: agent -> check -> integrate"
W1=$(mkworkflow "cp7-basic" 'function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 basic step\n"}); var c=ctx.check({repo:ctx.params.repo,branch:ctx.params.branch}); var i=ctx.integrate({repo:ctx.params.repo,branch:ctx.params.branch,base:"main"}); return {check:c,integrate:i}; }')
R1=$(runwf "$W1" "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"b1\"}}")
wait_status "$R1" completed 40 && PASS=$((PASS+1)) || log "FAIL: basic run not completed"
log "basic run=$R1 status=$(runstatus "$R1") error=$(runerror "$R1")"
log "executions for basic run: $(exec_count "$R1") (expect 1)"

echo; log "### 2. crash/restart: executor death mid-run resumes idempotently"
W2=$(mkworkflow "cp7-crash" 'function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 crash step1\n"}); ctx.sleep({ms:20000}); ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 crash step2\n"}); var c=ctx.check({repo:ctx.params.repo,branch:ctx.params.branch}); return {check:c}; }')
R2=$(runwf "$W2" "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"b2\"}}")
# wait until the sleep step is running, then kill the control plane (executor death)
ok=0
for i in $(seq 1 40); do
  st=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R2" | jq -c "[.steps[] | select(.op==\"sleep\" and .status==\"running\")] | length")
  [ "$st" = "1" ] && ok=1 && break; sleep 1
done
if [ "$ok" = 1 ]; then
  log "sleep step running; killing control plane (executor death)"
  fuser -k 8080/tcp 2>/dev/null; sleep 2
  /opt/cp0/switchyard/start.sh >/dev/null 2>&1
  curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
fi
wait_status "$R2" completed 90 && PASS=$((PASS+1)) || log "FAIL: crashed run did not resume"
EC=$(exec_count "$R2")
log "crashed run=$R2 completed; executions=$EC (expect exactly 2: step1+step2, NOT re-executed)"
[ "$EC" = "2" ] && PASS=$((PASS+1)) || log "FAIL: duplicated external effects ($EC executions)"
# verify step attempts == 1
ATS=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R2" | jq -c "[.steps[] | {op, attempts, status}] | unique_by(.op)")
log "crash run steps: $(echo "$ATS" | jq -c '[.[] | select(.op=="agent") | .attempts]') (expect [1,1])"

echo; log "### 3. spawn child + wait on approval + integrate"
CHILD=$(mkworkflow "cp7-child" 'function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 child step\n"}); var c=ctx.check({repo:ctx.params.repo,branch:ctx.params.branch}); return {child:true,check:c}; }')
W3=$(mkworkflow "cp7-parent" 'function run(ctx){ ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 parent step1\n"}); var child=ctx.spawn({workflow:"cp7-child",params:{repo:ctx.params.repo,branch:ctx.params.childBranch}}); ctx.wait_approval({reason:"approve merge"}); var i=ctx.integrate({repo:ctx.params.repo,branch:ctx.params.branch,base:"main"}); return {child:child.child_run_id, integrate:i}; }')
R3=$(runwf "$W3" "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"b3\",\"childBranch\":\"cb3\"}}")
wait_status "$R3" waiting_approval 60 && log "parent waiting_approval (child spawned) -> PASS" && PASS=$((PASS+1)) || log "FAIL: parent did not wait for approval"
CHILDID=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R3" | jq -r '.run.output.child' 2>/dev/null)
CHILDID=$(curl -s -b "$JAR" "$APP/api/workflow_runs" | jq -r "[.items[] | select(.parent_run_id==\"$R3\") | .id][0]")
log "child run=$CHILDID status=$(runstatus "$CHILDID")"
wait_status "$CHILDID" completed 40 || log "FAIL: child did not complete"
curl -s -b "$JAR" -X POST "$APP/api/workflow_runs/$R3/approve" -H "Content-Type: application/json" -d '{}' | jq -c .
wait_status "$R3" completed 40 && log "parent completed after approval -> PASS" && PASS=$((PASS+1)) || log "FAIL: parent did not complete after approval"

echo; log "### 4. cancellation parks further steps"
W4=$(mkworkflow "cp7-cancel" 'function run(ctx){ ctx.note({text:"first"}); ctx.sleep({ms:60000}); ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 cancelled step\n"}); return {late:true}; }')
R4=$(runwf "$W4" "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"b4\"}}")
ok=0
for i in $(seq 1 30); do
  st=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R4" | jq -c "[.steps[] | select(.op==\"sleep\" and .status==\"running\")] | length")
  [ "$st" = "1" ] && ok=1 && break; sleep 1
done
log "cancelling run while sleep active"
curl -s -b "$JAR" -X POST "$APP/api/workflow_runs/$R4/cancel" -H "Content-Type: application/json" -d '{}' | jq -c .
wait_status "$R4" cancelled 30 && log "run cancelled -> PASS" && PASS=$((PASS+1)) || log "FAIL: run not cancelled (now $(runstatus "$R4"))"
LATE=$(curl -s -b "$JAR" "$APP/api/workflow_runs/$R4" | jq -c "[.steps[] | select(.op==\"agent\")] | length")
log "agent step executed after cancel: $LATE (expect 0)"
[ "$LATE" = "0" ] && PASS=$((PASS+1)) || log "FAIL: step ran after cancellation"

echo; log "### 5. parallel branches + bounded recursion"
W5=$(mkworkflow "cp7-par" 'function run(ctx){ var r=ctx.parallel([function(){ return ctx.agent({repo:ctx.params.repo,branch:ctx.params.branch,file:"ATTEMPT.md",append:"\n// cp7 par agent\n"}); }, function(){ return ctx.review({repo:ctx.params.repo,branch:ctx.params.branch}); }]); return {branches:r.length}; }')
R5=$(runwf "$W5" "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"b5\"}}")
wait_status "$R5" completed 40 && log "parallel run completed -> PASS" && PASS=$((PASS+1)) || log "FAIL: parallel run"
log "parallel steps ops: $(curl -s -b "$JAR" "$APP/api/workflow_runs/$R5" | jq -c '[.steps[].op]') (expect agent+review)"

W6=$(mkworkflow "cp7-rec" 'function run(ctx){ ctx.spawn({workflow:"cp7-rec",params:{repo:ctx.params.repo,branch:ctx.params.branch}}); return {recurse:true}; }')
R6=$(runwf "$W6" "{\"params\":{\"repo\":\"$REPO\",\"branch\":\"b6\"}}")
wait_status "$R6" failed 60 && log "recursive spawn depth-limited (failed) -> PASS" && PASS=$((PASS+1)) || log "FAIL: recursion not bounded (status=$(runstatus "$R6"))"
log "recursive run error: $(runerror "$R6")"

echo
log "#### CP7 GATE: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"