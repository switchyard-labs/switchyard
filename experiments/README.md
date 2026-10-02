# Switchyard experiments

Experiments are disposable, evidence-driven work with a defined
hypothesis/setup/measurement/result/decision/cleanup discipline. They live here
so useful evidence is retained in the repo without polluting product code.

Rules (from the gameplan):

- every experiment has: hypothesis, setup, measurement, result, decision, cleanup.
- no product architecture is silently changed by an experiment.
- an experiment that contradicts an architecture assumption **stops and is
  reported**, never silently adapted.
- experiments may also use the CP0 Linode scratch area (`/opt/cp0`) where real
  infrastructure is required.

| Directory | Checkpoint | Topic |
| --- | --- | --- |
| `cp1/` | CP1 | Artifacts contention/CAS, draft three-way merge, draft recovery spike, secret-path spike |