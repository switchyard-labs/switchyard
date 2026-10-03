# C34 — immutable CI source inspection through Artifacts Worker binding

The Actions Worker now calls the native binding's `readCommit(exactSHA)` before
CI, and reads optional `switchyard.actions.js` at that same immutable SHA. It
records only existence, byte count and SHA-256 of the configuration (64 KiB
maximum); it does not evaluate repository code or replace owner-approved jobs.
The RPC repository capability is disposed on success and error.

This belongs beside push-triggered cloud CI: the Worker can directly verify Git
truth before allocating a Container. Go retains identity, repository policies,
definition approval, Work/Attempts, reviews, Integration Queue and coordination.
The signed `/source/{artifactRepo}/{sha}` endpoint exposes the same bounded
inspection to operators; it requires the existing control-channel signature and
repository allowlist, and offers no arbitrary file-read or token-minting route.

The disposable `switchyard-actions-probe-20261003` deployment returned HTTP 200:
`commit_present=true`, SHA `e2a22198d24faa843975adef7fe7ffde1b997aa5`,
`inspection=artifacts-worker-binding`, `config=null` (the fixture has no committed
configuration file). This proves direct native commit inspection in the deployed
Worker; CI execution with this new preflight and a nonempty config fingerprint
remain to be exercised in C36/C38.

Local Worker tests and TypeScript checks pass, including missing commit,
oversized config, repository allowlist, immutable SHA and capability disposal.
Only the existing disposable Worker was deployed; no Git project was pushed.

Provider contract reviewed 2026-10-04:
[Artifacts Workers binding](https://developers.cloudflare.com/artifacts/api/workers-binding/).
