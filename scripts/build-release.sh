#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
output="${1:-dist/switchyard}"
nift build --all
mkdir -p "$(dirname "$output")"
go build -trimpath -o "$output" ./cmd/switchyard
