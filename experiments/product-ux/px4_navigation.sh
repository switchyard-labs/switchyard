#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
grep -q '<details class="menu-group"' "$ROOT/templates/template.html"
grep -q 'body.menu-open' "$ROOT/public/assets/css/app.css"
grep -q 'querySelectorAll("a\[href\], button, summary")' "$ROOT/public/assets/js/app.js"
! grep -q '<nav class="site-nav"' "$ROOT/templates/template.html"
node --check "$ROOT/public/assets/js/app.js"
echo 'PX4 product navigation smoke: PASS'
