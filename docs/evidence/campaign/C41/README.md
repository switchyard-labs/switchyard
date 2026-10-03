# C41 competition preparation — partial

The maintained loopback-only preparation script created real Work and two isolated
Artifacts-backed Attempt branches under `alice/railway`. `prepared-attempts.json`
contains identities, no credentials. No mutation retries or synthetic Agent runs
are hidden in the script. Existing manifests prevent accidental duplicate setup.

Reproduce against a registered writable disposable codex-* repository:

```sh
python3 scripts/prepare-competition-demo.py --base http://127.0.0.1:18127 \
  --cookies /private/alice-cookies --repository alice/railway \
  --output /private/competition-manifest.json
```

Resume from that manifest. Do not rerun preparation with another filename merely
to bypass its duplicate guard. The cookie and manifest files must remain private.

## Honest remaining gates

The shipped entry point currently selects DeterministicRunner. CLIRunner is a
fail-closed isolated adapter library, not a configured production coding provider.
A genuine provider choice, owner-authorized credential resolution, adapter wiring
and bounded concurrent execution are prerequisites. Its present per-instance
single-flight gate does not certify concurrent external Agents. A provider key
alone does not close this gap; never call the deterministic adapter real Agents.

After selecting the provider, implement and certify that boundary before invoking
it for these two Attempts. Retain separate branches, exact source SHAs, bounded
outputs and credential-free provenance. Approve a CI definition for these exact
branches: the current C36 approved definition only covers main and event-c36.
Do not silently authorize arbitrary branch configuration in the privileged Worker.

Then capture ordinary pushes through scripts/certify-event-path.py, automatic
exact-SHA Actions, a structured human review/finding, a clean textual merge whose
API/consumer contract disagrees, explicit resolution and fenced queue integration.
Use the existing switchyard.contract.json equality rule; do not substitute a text
conflict for a semantic conflict. Verify the final main SHA and source provenance.
The complete combined story and recorded walkthrough are still outstanding.
C36 evidence already proves automatic push/CI, duplicate delivery and convergence
independently. Unit/real-Git gates prove integration and semantic behavior, but do
not count as this final combined live scenario.
