#!/usr/bin/env bash
set -euo pipefail
grep -q 'Organization settings' public/org-settings.html
grep -q 'org_owner_required\|org_permission_denied' internal/app/org_collaboration.go
grep -q 'requireOrgRole.*owner' internal/app/policy.go
grep -q 'orgs/{id}/audit' internal/app/server.go
grep -q 'Rename, transfer and delete are deliberately disabled' public/org-settings.html
echo 'PX15 org settings static gate PASS'
