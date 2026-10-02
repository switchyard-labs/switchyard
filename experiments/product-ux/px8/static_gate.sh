#!/usr/bin/env bash
set -euo pipefail
grep -q 'workspace-tree' public/edit.html
grep -q 'editor-tabs' public/edit.html
grep -q 'status-language' public/edit.html
grep -q 'rename-file' public/assets/js/app-editor.js
grep -q 'api/refs/update' public/assets/js/app-editor.js
grep -q 'mobile-workbench-switcher' public/assets/css/editor.css
echo 'PX8 static workbench gate PASS'
