#!/usr/bin/env bash
set -u
cd /opt/cp0/strut-probes
COMPILER=/opt/cp0/strut/build/strut
mkdir -p bin
run_one() {
  local name=$1
  local src=$name.p
  local exe=bin/$name
  if ! "$COMPILER" "$src" -o "$exe" >"bin/$name.compile.log" 2>&1; then
    echo "COMPILE_FAIL $name"
    sed -n '1,8p' "bin/$name.compile.log"
    return
  fi
  if timeout 30 "$exe" >"bin/$name.run.log" 2>&1; then
    echo "PASS  $name :: $(cat bin/$name.run.log)"
  else
    echo "RUN_FAIL $name (rc=$?)"
    cat "bin/$name.run.log"
  fi
}

# SSE server for probe_sse
fuser -k 8090/tcp 2>/dev/null || true
sleep 1
nohup python3 sse_server.py >/dev/null 2>&1 &
SSE_PID=$!
sleep 1

run_one probe_https
run_one probe_json
run_one probe_process
run_one probe_git
run_one probe_concurrent
run_one probe_fs
run_one probe_hmac
run_one probe_cancel
run_one probe_child_failure
run_one probe_sse

# shutdown probe: spawn, SIGTERM, expect clean observed+exit0
if "$COMPILER" probe_shutdown.p -o bin/probe_shutdown >/dev/null 2>&1; then
  bin/probe_shutdown >bin/probe_shutdown.run.log 2>&1 &
  SPID=$!
  for i in $(seq 1 20); do grep -q shutdown-ready bin/probe_shutdown.run.log 2>/dev/null && break; sleep 0.2; done
  kill -TERM $SPID 2>/dev/null
  wait $SPID; rc=$?
  if grep -q shutdown-observed bin/probe_shutdown.run.log && [ $rc -eq 0 ]; then
    echo "PASS  probe_shutdown :: $(cat bin/probe_shutdown.run.log | tr '\n' ' ')"
  else
    echo "RUN_FAIL probe_shutdown (rc=$rc)"; cat bin/probe_shutdown.run.log
  fi
else
  echo "COMPILE_FAIL probe_shutdown"
fi

kill $SSE_PID 2>/dev/null
fuser -k 8090/tcp 2>/dev/null || true