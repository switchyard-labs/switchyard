#!/usr/bin/env bash
set -euo pipefail
grep -q 'Password & sessions' public/settings.html
grep -q 'Provider secrets do not grant repository authority' public/settings.html
grep -q 'handleChangePassword' internal/app/profiles.go
grep -q 'RevokeOtherSessions' internal/app/profiles.go
grep -q 'Account deletion is intentionally deferred' public/settings.html
echo 'PX12 settings static gate PASS'
