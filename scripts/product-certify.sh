#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "[1/6] JavaScript syntax"
for f in public/assets/js/*.js; do node --check "$f" >/dev/null; done

echo "[2/6] static accessibility"
python3 scripts/a11y-static-check.py

echo "[3/6] Go formatting"
if find internal cmd -name '*.go' -print0 | xargs -0 gofmt -l | grep -q .; then
  echo "gofmt drift detected" >&2; exit 1
fi

echo "[4/6] no GitHub-blue primary token regression"
if grep -RniE '#58a6ff|#0969da|#1f6feb|#3b82f6' public/assets/css templates content --exclude='*.map'; then
  echo "blue primary token regression" >&2; exit 1
fi

echo "[5/6] checkpoint handovers"
for n in $(seq -w 0 25); do test -f "docs/plan/handovers/PX${n}.md" || { echo "missing PX${n}.md" >&2; exit 1; }; done

echo "[6/6] git whitespace"
git diff --check

echo "STATIC PRODUCT CERTIFICATION PASS"
echo "NOTE: live Go 1.26/Trestle/Artifacts + Chromium/Firefox/WebKit gates are separate normal-environment requirements."
