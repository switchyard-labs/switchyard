#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
grep -q '<title>Switchyard</title>' "$ROOT/public/index.html"
[ "$(find "$ROOT/public" -maxdepth 1 -name '*.html' | wc -l)" -ge 5 ]
grep -Rq '#58a6ff' "$ROOT/public/assets/css"
grep -q 'class="file-view"' "$ROOT/public/repo.html"
echo 'PX0 baseline smoke: PASS'
