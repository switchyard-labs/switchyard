#!/usr/bin/env bash
set -euo pipefail
grep -q 'SwitchyardCode.lines' public/assets/js/app.js
grep -q 'repo-ref-select' public/repo.html
grep -q 'copy-path' public/repo.html
grep -q 'image-preview' public/assets/css/app.css
grep -q 'handleCanonicalRepoRefs' internal/app/repository_identity.go
echo 'PX6 static gate PASS'
