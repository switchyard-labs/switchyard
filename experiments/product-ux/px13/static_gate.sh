#!/usr/bin/env bash
set -euo pipefail
grep -q 'org_profiles' internal/app/profiles.go
grep -q 'handleUploadOrgAvatar' internal/app/profiles.go
grep -q "slug==='.github'" public/assets/js/app-profile.js
grep -q 'owner_type.*org' public/assets/js/app-profile.js
echo 'PX13 org profile static gate PASS'
