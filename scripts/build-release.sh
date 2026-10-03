#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
output="${1:-dist/switchyard}"
nift build --all
mkdir -p "$(dirname "$output")"
release_version="${SWITCHYARD_BUILD_VERSION:-dev}"
release_commit="$(git rev-parse HEAD)"
if ! git diff --quiet || ! git diff --cached --quiet; then release_commit="$release_commit-dirty"; fi
release_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
for value in "$release_version" "$release_commit" "$release_date"; do
  if [[ ! "$value" =~ ^[a-zA-Z0-9._:+-]+$ ]]; then
    printf 'Invalid release metadata\n' >&2
    exit 1
  fi
done
go build -trimpath -ldflags "-X main.version=$release_version -X main.commit=$release_commit -X main.buildDate=$release_date" -o "$output" ./cmd/switchyard
