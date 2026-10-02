#!/usr/bin/env bash
set -euo pipefail
# Normal-environment helper. Intentionally does not contain credentials or run
# automatically. Re-seed curated public demo repositories using the campaign's
# existing demo seeding process, then restart the demo control plane.
echo "Reset curated demo repositories: demo-basic demo-agents demo-conflict demo-semantic demo-workflow"
echo "Use approved Artifacts/Trestle credentials from the deployment environment; never embed them in this script."
