#!/usr/bin/env bash
# CP5 corrective gate (v2): dual-path event architecture.
#   FAST PATH:   Cloudflare queue -> Switchyard consumer -> observeTransition
#                (domain fact + SSE), acked.
#   SAFETY NET:  periodic reconciliation -> same observeTransition (dedup).
# Idempotency identity: (repo, ref, before, after) shared by both paths.
# NOTE: Cloudflare event-subscription -> queue delivery is currently NOT
# delivering (upstream condition; same config delivered in CP0). We therefore
# inject the *real Cloudflare schema* cf.artifacts.repo.pushed envelopes
# (identical bytes to CP0) into the queue via the Queues push API, proving the
# full Switchyard ingest path + convergence with genuinely observed SHAs.
set -uo pipefail
ACC=b7f20353ee8a9e5d2003f52c74ba795e
NS=switchyard-cp0
QID=2b68d6736b3a4d0ca4aea6b36d47fde1
APP=http://127.0.0.1:8080
TS=$(date +%s)
JAR=/tmp/cp5v2.jar; rm -f "$JAR"
PASS=0; TOTAL=6
log(){ echo "[$(date +%H:%M:%S)] $*"; }
TOK=/opt/cp0/switchyard/token.sh

qpush(){ # $1=body-object
  curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/queues/$QID/messages" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"body\":$1}" | jq -c .success
}
domain(){ # $1=repo $2=after -> count of git.ref_changed domain facts with that after
  curl -s -b "$JAR" "$APP/api/events" | jq -c "[.items[] | select(.type==\"git.ref_changed\" and .repo_name==\"$1\") | .payload | select(.after==\"$2\")] | length"
}
domain_sources(){ # $1=repo $2=after
  curl -s -b "$JAR" "$APP/api/events" | jq -c "[.items[] | select(.type==\"git.ref_changed\" and .repo_name==\"$1\") | .payload | select(.after==\"$2\") | .source]"
}
sse_capture(){
  (curl -s -N -b "$JAR" "$APP/api/events/stream" > /tmp/sse.out 2>/dev/null) & SSEPID=$!; sleep 1
}
sse_stop(){ kill "$SSEPID" 2>/dev/null; wait "$SSEPID" 2>/dev/null; }
envset(){ sed -i "s|^$1=.*|$1=$2|" /opt/cp0/switchyard/cp0.env; grep -q "^$1=" /opt/cp0/switchyard/cp0.env || echo "$1=$2" >> /opt/cp0/switchyard/cp0.env; }
envunset(){ sed -i "/^$1=/d" /opt/cp0/switchyard/cp0.env; }
restart(){ /opt/cp0/switchyard/start.sh >/dev/null 2>&1; sleep 2; }

# ---- setup: fresh repo, seed main@A ----
REPO=cp5evt2-$TS
r=$(curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/artifacts/namespaces/$NS/repos" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"$REPO\",\"default_branch\":\"main\"}")
REMOTE=$(echo "$r" | jq -r .result.remote); RT=$(echo "$r" | jq -r .result.token)
W=$(mktemp -d); cd "$W"; git init -q -b main; git remote add origin "$REMOTE"; echo base>base.txt; git add -A
git -c user.name=seed -c user.email=seed@local commit -qm base
git -c http.extraHeader="Authorization: Bearer $RT" push -q -u origin main
A=$(git rev-parse HEAD)
# event subscription (best effort; Cloudflare delivery currently not flowing)
curl -s -X POST "https://api.cloudflare.com/client/v4/accounts/$ACC/event_subscriptions/subscriptions" -H "Authorization: Bearer $($TOK)" -H "Content-Type: application/json" -d "{\"name\":\"cp5v2-$TS\",\"source\":{\"type\":\"artifacts.repo\",\"name\":\"Artifacts\",\"namespace\":\"$NS\",\"repo_name\":\"$REPO\"},\"events\":[\"pushed\"],\"destination\":{\"type\":\"queues.queue\",\"queue_id\":\"$QID\"}}" >/dev/null
# login
curl -s -c "$JAR" -X POST "$APP/api/auth/register" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null 2>&1
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
log "repo=$REPO seeded main@$A"

# ---- Phase A: FAST PATH only (reconciliation DISABLED) ----
log "### A: disable reconciliation, enable queue; prove fast path + SSE"
envunset SWITCHYARD_RECONCILE_INTERVAL; envset SWITCHYARD_QUEUE_ID "$QID"; restart
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
echo b>b.txt; git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "push B"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin main
B=$(git rev-parse HEAD)
log "pushed main $A -> $B (real push); injecting real-schema pushed event into queue"
sse_capture
qpush "{\"type\":\"cf.artifacts.repo.pushed\",\"source\":{\"namespace\":\"$NS\",\"repoName\":\"$REPO\",\"type\":\"artifacts.repo\"},\"payload\":{\"ref\":\"refs/heads/main\",\"before\":\"$A\",\"after\":\"$B\"}}"
ok=0
for i in $(seq 1 10); do
  n=$(domain "$REPO" "$B"); [ "$n" -ge 1 ] && ok=1 && break; sleep 2
done
sleep 1
log "domain facts for B: $(domain "$REPO" "$B") sources=$(domain_sources "$REPO" "$B") (expect [\"queue\"] after fast path)"
[ "$ok" = 1 ] && PASS=$((PASS+1)) || log "FAIL: fast path did not ingest"
sse_n=$(grep -c 'git.ref_changed' /tmp/sse.out)
log "SSE frames: $sse_n (expect >=1)"
[ "$sse_n" -ge 1 ] && PASS=$((PASS+1)) || log "FAIL: no SSE frame"
sse_stop
# reconcile is disabled; verify no duplicate from a manual reconcile pass
sleep 4
log "after idle: $(domain "$REPO" "$B") (expect still 1)"

# ---- Phase B: duplicate queue delivery ----
log "### B: duplicate queue delivery converges to one domain fact"
echo b2>b2.txt; git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "push B2"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin main
B2=$(git rev-parse HEAD)
qpush "{\"type\":\"cf.artifacts.repo.pushed\",\"source\":{\"namespace\":\"$NS\",\"repoName\":\"$REPO\",\"type\":\"artifacts.repo\"},\"payload\":{\"ref\":\"refs/heads/main\",\"before\":\"$B\",\"after\":\"$B2\"}}"
qpush "{\"type\":\"cf.artifacts.repo.pushed\",\"source\":{\"namespace\":\"$NS\",\"repoName\":\"$REPO\",\"type\":\"artifacts.repo\"},\"payload\":{\"ref\":\"refs/heads/main\",\"before\":\"$B\",\"after\":\"$B2\"}}"
for i in $(seq 1 8); do
  n=$(domain "$REPO" "$B2"); [ "$n" -ge 1 ] && break; sleep 2
done
sleep 3
n=$(domain "$REPO" "$B2")
log "domain facts for B2 after duplicate delivery: $n (expect 1)"
[ "$n" = 1 ] && PASS=$((PASS+1)) || log "FAIL: duplicate queue delivery produced $n facts"

# ---- Phase C: reconciliation-first, then late queue event ----
log "### C: reconciliation-only ingest (queue disabled), then late event dedups"
envset SWITCHYARD_RECONCILE_INTERVAL 8s; envunset SWITCHYARD_QUEUE_ID; restart
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
echo c>c.txt; git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "push C"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin main
C=$(git rev-parse HEAD)
log "pushed main $B2 -> $C with queue consumer disabled; waiting for reconciliation..."
ok=0
for i in $(seq 1 12); do
  n=$(domain "$REPO" "$C"); [ "$n" -ge 1 ] && ok=1 && break; sleep 2
done
log "reconciliation-only domain facts for C: $(domain "$REPO" "$C") sources=$(domain_sources "$REPO" "$C") (expect [\"reconciliation\"])"
[ "$ok" = 1 ] && PASS=$((PASS+1)) || log "FAIL: reconciliation did not ingest C"
# re-enable queue; inject the late C event -> must dedup
envset SWITCHYARD_QUEUE_ID "$QID"; restart
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
qpush "{\"type\":\"cf.artifacts.repo.pushed\",\"source\":{\"namespace\":\"$NS\",\"repoName\":\"$REPO\",\"type\":\"artifacts.repo\"},\"payload\":{\"ref\":\"refs/heads/main\",\"before\":\"$B2\",\"after\":\"$C\"}}"
for i in $(seq 1 8); do
  sleep 2
  s=$(domain_sources "$REPO" "$C"); echo "$s" | grep -q "queue" && break
done
n=$(domain "$REPO" "$C")
log "after late queue event for C: $(domain "$REPO" "$C") facts sources=$(domain_sources "$REPO" "$C") (expect 1 total)"
[ "$n" = 1 ] && PASS=$((PASS+1)) || log "FAIL: late event duplicated C"

# ---- Phase D: control plane unavailable; converges on restart ----
log "### D: control plane down during push; converges with no duplicates"
fuser -k 8080/tcp 2>/dev/null; sleep 1
echo d>d.txt; git add -A; git -c user.name=agent -c user.email=agent@cp0 commit -qm "push D while down"
git -c http.extraHeader="Authorization: Bearer $RT" push -q origin main
D=$(git rev-parse HEAD)
qpush "{\"type\":\"cf.artifacts.repo.pushed\",\"source\":{\"namespace\":\"$NS\",\"repoName\":\"$REPO\",\"type\":\"artifacts.repo\"},\"payload\":{\"ref\":\"refs/heads/main\",\"before\":\"$C\",\"after\":\"$D\"}}"
log "pushed main $C -> $D while control plane down; event queued"
restart
curl -s -c "$JAR" -X POST "$APP/api/auth/login" -H "Content-Type: application/json" -d '{"username":"alice","password":"password123"}' >/dev/null
ok=0
for i in $(seq 1 15); do
  n=$(domain "$REPO" "$D"); [ "$n" -ge 1 ] && ok=1 && break; sleep 2
done
sleep 10
n=$(domain "$REPO" "$D")
log "after restart: $(domain "$REPO" "$D") facts for D sources=$(domain_sources "$REPO" "$D") (expect 1)"
{ [ "$ok" = 1 ] && [ "$n" = 1 ]; } && PASS=$((PASS+1)) || log "FAIL: offline push not converged/duplicated"

echo
log "#### CP5 EVENT GATE v2: $PASS/$TOTAL checks passed"
echo "PASS=$PASS/$TOTAL"
echo "REPO=$REPO"