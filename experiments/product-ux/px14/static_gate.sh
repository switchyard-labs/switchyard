#!/usr/bin/env bash
set -euo pipefail
grep -q 'org_invitations' internal/app/org_collaboration.go
grep -q 'cannot_remove_primary_owner' internal/app/org_collaboration.go
grep -q 'org_repo_access' internal/app/org_collaboration.go
grep -q 'canAccessRepository' internal/app/repository_identity.go
grep -q 'org_permission_denied' internal/app/org_collaboration.go
echo 'PX14 collaboration enforcement static gate PASS'
