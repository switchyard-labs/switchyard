# C41 reproducible competition walkthrough

DONE for the recorded real multi-role competition. The old preparation-only
status is superseded by the clean scripted C25 replay and this capture bundle.
The proof ran on a local isolated coordinator/Agent host and the existing
disposable Artifacts repository `codex-actions-probe-20261003`, presented as
`alice/railway`; no coding Agent ran on the Linode control-plane host.

## Reproduction

Configure genuine implementation, review and conflict-resolution providers, an
owner-authorized private credential profile, bounded isolated Agent execution,
and the native Actions Worker. Use a fresh registered writable codex-* repository
with `api-version.json` containing version/requires 1, `new-consumer.json`
containing requires 1, and a `switchyard.contract.json` field_equals rule from
api-version.json:version to api-version.json:requires. Its CI command must run
against the immutable checked-out source. Repository/provider provisioning is
a prerequisite, not something these scripts silently grant or create.

```sh
python3 scripts/prepare-competition-demo.py --base http://127.0.0.1:18127 \
  --cookies /private/alice-cookies --repository alice/railway \
  --output /private/competition-manifest.json
python3 scripts/run-semantic-competition.py \
  --manifest /private/competition-manifest.json \
  --cookies /private/alice-cookies --output /private/competition-proof.json
```

Resume the same manifest/output after interruption. Ambiguous mutations are
never blindly retried; inspect their durable state. Do not create another
manifest to bypass duplicate guards. The current disposable repository has
subsequently advanced and must not be treated as the fresh version-1 baseline.
Scripts and their recorded clean replay are maintained evidence, not remembered
terminal choreography. They do not reset an arbitrary live repository.

## Recorded sequence

1. Work `wk_de7b6ee579db014c` creates two isolated Attempt branches.
2. Real implementation Agents use OpenCode Go/gpt-5.6-luna in parallel. A
   publishes API version 2 while retaining its own consistent requires field.
   B publishes a consumer requiring version 1 and changes the semantic rule
   to compare API version with that new consumer. Each branch is initially
   semantically valid. Native push CI verifies both exact SHAs.
3. Real deepseek-v4-flash reviews complete without findings. Their actual empty
   findings arrays are retained; no fabricated human or model finding is added.
4. A's queue item publishes. B's textual merge is clean, but its real semantic
   preview reports API version 2 != consumer requires 1. This error finding
   and `semantic_conflict=true` are retained in `walkthrough.json`.
5. A real glm-5.3-flash resolver changes the consumer requirement to 2, producing
   `ec1338d5bb27cadae7999a4d8bec427f01b9993d`. Re-preview is clean and a real
   deepseek re-review completes without error findings. Native repair CI
   `5f79219d-87a0-4009-b97e-7af5a5b96b00` succeeds against exactly that SHA.
6. B's queue item publishes, both PRs are integrated, and the final main SHA
   at proof completion is `552beee1b6a57feae0f120c063f67360bff90ab8`.
   Work/Attempt/run/PR/ref-update provenance retains implementation and
   conflict-resolver attribution. Later disposable certifications advance
   main; the captured completion refs must remain historical.

## Evidence

- `walkthrough.json`: ordered stage results, real review findings, semantic
  conflict, final refs and provenance extracted from the C25 clean replay.
- `event-timeline.json`: five retained real normalized observations for the
  competition branches. Direct Artifacts binding and native event/queue
  delivery proof is C36; normalized observations are not relabelled as direct
  raw Cloudflare envelopes.
- `work-two-attempts.png`: actual Work and both branches. Its initial description
  mentions provider setup as a prerequisite; later real execution is established
  by the replay, not by rewriting that original historical text.
- `repair-ci-log.png`: captured real stdout and exact repaired source SHA.
  C38 independently verifies downloaded bytes and real clipboard paste.
- `integrated-pr.png`, `queue-done.png`, `exact-source-checks.png`: actual
  integrated PR, completed queue, exact-source native check and disabled
  terminal actions. Captures were taken after the run; the historical conflict
  is certified by its recorded API result rather than a fabricated screenshot.

Role providers, execution IDs, resource measurements and outputs are in
C25/semantic-clean-scripted-flow.json. New execution context-snapshot proof is
C25/live-context-snapshot-review.json; old runs are not retrospectively assigned
fields they did not record. The screenshot pass exposed and fixed PR Checks
omitting native CI results and integrated PRs offering enqueue (`84552b7`).
Generic Cloudflare CI is PASS; Worker Build/Preview remains CONDITIONAL on
genuine scoped deployment setup. Releases/R2 and archive proofs are C46/C45,
not implied by this PR integration scenario.
