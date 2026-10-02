#!/usr/bin/env bash
set -euo pipefail
grep -q 'agent-propose' internal/app/server.go
grep -q 'currentRev != startRev' internal/app/editor.go
grep -q 'committed.*false' internal/app/editor.go
grep -q 'btn-agent-draft' public/assets/js/app-editor.js
grep -q 'Applied to draft only' public/assets/js/app-editor.js
echo 'PX9 interactive Agent draft static gate PASS'
