# Real Agent runner investigation — 4 October 2026

C25 remains **PARTIAL / blocked on complete real runner certification**. This is a local Linux investigation, not a Linode capacity test. OpenCode 1.18.34, uid 1000, provider `opencode-go`, model `gpt-5.6-luna`; temporary personal credential is excluded from evidence.

## Original constraints

`internal/agent/runner.go` applies RLIMIT_AS 2 GiB by default (4 GiB maximum), RLIMIT_CPU 120 seconds by default (600 maximum), RLIMIT_NOFILE 128, RLIMIT_CORE 0, RLIMIT_FSIZE 16 MiB, and a nested RLIMIT_NPROC 256. Wall timeout defaults to two minutes (ten-minute maximum), captured output defaults to 1 MiB (8 MiB maximum). RLIMIT_DATA and RLIMIT_STACK are inherited; the local parent has unlimited data and an 8 MiB soft stack limit. RLIMIT_NPROC is a per-real-user limit, not a per-execution process-tree budget.

Bubblewrap uses private mount/PID/user namespaces, a fresh workspace, read-only runtime mounts, a minimal environment, fresh HOME and temporary directories. Network is allowed only for configured provider execution. Credentials are scoped private files. Parent cancellation kills the process group; bubblewrap dies with its parent. The OpenCode adapter adds a 100-second deadline and permits edits only to the selected task file, with shell and unrelated tools denied.

## Reproductions

See `runner-resource-study.json` for safe measurements.

| Local configuration | Outcome | Duration | Sampled peak RSS | Peak virtual address space |
|---|---|---:|---:|---:|
| Normal non-root, no address cap | Real response | 7.062 s | 711.6 MiB | 129.2 GiB |
| RLIMIT_AS 2 GiB alone | Runtime crash | 5.853 s | 336.0 MiB | approximately 2 GiB |
| RLIMIT_AS 4 GiB alone | JavaScriptCore MemoryExhaustion | 8.787 s | 585.5 MiB | approximately 4 GiB |
| RLIMIT_NPROC 256 alone | Runtime crash | 2.424 s | 100.5 MiB | 129.2 GiB |
| Filesystem isolation + cgroup memory | Real response | 4.750 s | parent-only sampling excluded | 531.2 MiB cgroup memory peak |

The address-space incompatibility is reproduced independently: OpenCode needs much larger virtual reservations than these caps permit, while observed resident memory is substantially lower. VmSize is not RAM consumption. The per-user process cap independently fails on this desktop, whose uid 1000 has 203 processes and 2,648 threads (see user-task-census.json). The original 4 GiB thread error could also involve address/stack allocation; its precise cause is not fully established.

## Five harmless sandbox certifications

Five actual provider responses succeeded with non-root filesystem isolation, fresh configuration, no host plugins/auth, no tool use or Git mutation. Cgroup controls: memory.max 1536 MiB, memory.high 1280 MiB, swap.max 0, pids.max 128, CPU quota two cores; wall service deadline 110 seconds and control-group cleanup. CPU/file/output/time boundaries remain active.

Durations: 4.873, 6.248, 4.298, 4.145, 4.576 seconds; median 4.576 seconds. Five samples do not establish a reliable population p95. Cgroup memory peaks 522.5–550.5 MiB; cgroup task peaks 40–42; maximum individual-process sampled RSS approximately 623–641 MiB. No memory OOM events. Shared file-backed mappings can make per-process RSS differ from memory charged to a cgroup. First provider-request time is not separately instrumented.

## Candidate Switchyard runner

An opt-in cgroup-backed CLIRunner uses a unique transient systemd user service with enforced memory.max, zero swap, pids.max 128, CPU quota two cores and a wall deadline. It removes RLIMIT_AS and the per-user RLIMIT_NPROC only for that path, retaining bubblewrap, non-root execution, minimal environment, private credentials, bounded output and CPU/file limits. Cancellation explicitly stops the service control group; unavailable user-service support fails closed. The OpenCode adapter selects this candidate with 1536 MiB memory. These are certification settings, not final production capacity defaults.

Full Go tests and vet passed. A race check found concurrent child stderr/event diagnostic writes; a shared serialized writer now protects them. Explicit local cgroup tests verify result parsing/environment containment and cancellation. Further tests must verify memory/PID enforcement, accounting, error classification and child cleanup before deployment.

## Real coding and concurrency

Provider authentication and five harmless sandbox requests passed. The first real coding task and five additional sequential coding tasks published new Git changes on disposable Attempt branches. Immutable archive checks confirmed the requested JavaScript behavior, preservation of existing code, and unchanged canonical main. Sequential task durations were 21.205–24.013 seconds, median 21.638 seconds; cgroup memory peaks were 630.6–675.9 MiB. These timings include publication and are not provider latency measurements.

Two concurrent tasks passed exact output and branch-isolation checks: 25.958 seconds combined wall time and 1.203 GiB sampled combined memory peak. Three concurrent tasks also passed: 32.203 seconds and 1.950 GiB. The first four-task run completed provider executions but FAILED publication correctness: two HTTP 200 responses returned earlier Attempt commits. Combined sampled memory was 2.286 GiB; this is a resource observation, not a coding success. Recovery incorrectly matched a shared Attempt commit message. The candidate now identifies the specific execution and matches the entire message; regression coverage rejects legacy and prior-execution commits. The first repeat returned three HTTP 200 responses and one explicit 502. Subsequent archive verification was blocked by upstream 502 responses; those three outputs are not yet certified. One further bounded repeat failed with upstream_timeout and runner_exit_nonzero errors. All failed/partial runs are retained as evidence. No four-task success is claimed.

Actual cgroup memory enforcement was tested separately: a 64 MiB allocation inside a 32 MiB, zero-swap service was killed by the memory controller. Cancellation/environment tests passed. Kernel resource accounting is sampled and can miss final teardown events; exhaustive failure classification remains incomplete.

## Linode baseline and remaining gates

Read-only Linode measurements: 961 MiB physical RAM, 605 MiB available, one CPU, 2,543 MiB swap (86 MiB used). Bubblewrap is absent and the production control plane runs as root. No Agent capacity test has run there. Do not deploy an unisolated/root fallback or infer a production capacity result from local virtual-address-space failures.

**Linode adequacy: INSUFFICIENT EVIDENCE.** The observed two-Agent workload exceeds this host's physical RAM; even one observed coding task requires careful reservation alongside the control plane. A lower-memory local test and a dedicated non-root execution boundary are needed before any bounded Linode certification. No resize or production concurrency change has been made. Separate control-plane and worker budgets.

C25 remains PARTIAL: complete real review/conflict/queue/Actions workflow, production isolation, safe Linode execution, first-provider-request instrumentation, and final capacity recommendations are outstanding. Local three-Agent success is not production certification.
