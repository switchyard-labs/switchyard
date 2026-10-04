#!/usr/bin/env bash
set -euo pipefail
# Explicit operator commands; no credentials or automatic service stops.
exec python3 "$(dirname "$0")/operations/demo-reset.py" "$@"
