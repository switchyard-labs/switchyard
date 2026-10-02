#!/usr/bin/env bash
set -euo pipefail
grep -q 'profile-avatar' public/profile.html
grep -q '/api/users/.*repositories' public/assets/js/app-profile.js
grep -q "slug==='.github'\|r.slug===owner" public/assets/js/app-profile.js
grep -q 'profile-readme' public/assets/css/app.css
echo 'PX11 profile static gate PASS'
