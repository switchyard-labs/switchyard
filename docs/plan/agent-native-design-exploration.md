# Agent-native collaboration: design exploration

3 October 2026. Design only; no runtime refactor or speculative feature implementation. Observations refer to the current working tree, including unfinished C22 work. Existing checkpoint certification is a separate claim.

## Recommendation

Keep Work as the durable intent object and Attempts as alternative implementations. Make a **Proposal** the immutable review package for one Attempt revision, rather than renaming Work to PR or deleting PR compatibility. Add a small Checkpoint manifest only when it binds intent, exact Git identities and evidence together. Keep the integration queue; introduce explicit dependency edges before considering a general integration graph.

The strongest prototype is a single Work item with two competing Attempts, evidence bound to their exact revisions, a contract conflict that textual Git merge misses, and a bounded repair that must pass independent validation before publication. Those four ideas tell a clearer story than a broad replacement of every Git abstraction.

Finish the existing correctness, performance and security gates. Do not implement this document automatically. In particular, changing labels is not a substitute for fixing the current deterministic Agent path or proving real concurrent execution.

## Current state and overlap

Facts observed in code:

| Concern | Current mechanism | Gap relevant to this exploration |
| --- | --- | --- |
| Intent | `work` plus `work_details`, title/body/owner/repository/assignee; Work detail joins Attempts and PRs (`internal/app/work.go`) | No immutable version of acceptance criteria or decision rationale |
| Alternatives | `attempts` links Work to an isolated branch (`attempts.go`) | Creation adds `ATTEMPT.md`; no first-class checkpoint fork from an arbitrary recorded SHA |
| Agent execution | Roles and execution records; bounded CLI adapter and encrypted credentials (`internal/agent/`) | Default is deterministic. Direct run handler operates on `ATTEMPT.md`; do not present this as general autonomous implementation |
| Isolation | Unique scratch clones for ref changes and prepared merges; bubblewrap adapter namespaces and resource limits | No durable workspace lease/lifecycle entity; a branch is not process isolation |
| Review | Durable findings, deterministic review execution, PR detail joins Work/Attempt/checks/diff (`reviews.go`, `prs.go`) | Review of stated intent is not a verified capability; findings are not a complete immutable review version |
| Integration | FIFO queue, exact source checks, immutable prepared candidate, semantic gates, policy, bounded stale retries (`queue_integration.go`, `integration_policy.go`) | No dependency DAG or bounded multi-proposal speculative scheduler |
| Publication | Non-force Git push with advertised expected SHA and receive-pack old-object checks (`internal/refs/publication.go`) | Preserve these invariants in any new conceptual model |
| Recovery | Durable workflow steps/effects, execution claims and queue recovery tests | Not a general guarantee for every new workspace/artifact lifecycle |
| Programmability | JS workflow runner, deterministic replay and persisted step results (`workflow_exec.go`) | Budgets and capabilities matter; a programmable script must not override immutable publication policy |
| Evidence | Actions checks/logs/captures and ref provenance, execution outputs, findings | Evidence is spread across objects; no universal content-addressed manifest tied to a Proposal revision |
| Git UI | Repository code, refs, commits, editor, PRs plus Work/Attention/queue | Familiar Git entry points are useful, but task outcomes require joining several surfaces |

Existing C5–C9 work already supplies candidate identity, publication fencing, durable replay and queue claims. C10–C15 supply real Cloudflare Actions execution and evidence. C20–C21 supply an editor and decision surfaces. C23 performance, C24 hardening and C25 actual multi-Agent dogfood should remain gates, not be displaced by speculative modeling. C26–C44 covers packaging, canonical routing, Git interoperability and competition proof. The term campaign checkpoint in these documents is a development milestone, not an existing product Checkpoint entity.

Inference: the current substrate can support a Proposal projection cheaply. A durable workspace scheduler and generic graph scheduler would be substantial new systems. This assessment is source-based, not a completed load or distributed-systems certification.

## Review the achieved intent, with a bounded Proposal

Work describes the goal; Attempt describes an approach; Proposal requests acceptance of an exact revision. A Proposal manifest should reference the version of the goal/criteria, Attempt ID, base/source/tree SHAs, checkpoint IDs, reviewer findings, test receipts, benchmark deltas, artifact digests, unresolved questions and decision. Detailed logs stay in referenced evidence, not a giant duplicated JSON object.

This makes review ask whether the work achieved the intended outcome while still inspecting the diff. A polished summary cannot prove correctness: acceptance criteria need explicit evidence, missing checks remain unknown, and an agent's claim is distinct from an independently executed result. Security findings should carry severity, scope, evidence and disposition, not just a pass badge.

Use **Proposal** in the task flow, **Pull request** for Git-host import/export and familiar repository navigation. “Dossier” is an awkward user action; “Change” collides with file edits; “Task” is too broad to represent a particular reviewable version. Do not remove PR terminology abruptly. A read-only Proposal projection of existing PR/Attempt data should come first; API/schema migration can be additive later. Export includes ordinary source/base branches and a manifest summary with links. GitHub/GitLab comments cannot be treated as complete Switchyard provenance on reimport; retain external IDs and explicit source attribution.

## Managed workspaces and checkpoint forks

Agents should request a workspace for a task, role and exact checkpoint, not manually select paths. Internally a clone, worktree or snapshot is an implementation choice. Review and benchmark workspaces should be read-only except explicitly designated output directories; implementation workspaces need separate writable trees. Sharing a worktree's Git administrative directory makes isolation and locking more subtle than copying a branch name.

A workspace lease needs owner, repository ID, Attempt ID, exact base/tree, role, allowed capabilities, expiration, resource budget and durable lifecycle. Suggested lifecycle: requested → preparing → ready → running → capturing → retained/released; failures are inspectable states. Reclaiming a process does not authorize deleting its files. Before expiration/cleanup, capture tracked and permitted untracked changes, logs and an immutable manifest; exclude credentials and ignored private state. A dirty workspace without a verified recoverable capture must be retained or escalated. GC may operate only inside verified Switchyard-owned roots, never arbitrary user checkouts, `.git` or canonical repository history. Test capture/restore before enabling deletion.

Forking from a checkpoint is technically straightforward when a retained commit/tree exists: create a new Attempt ref from that SHA and record the parent checkpoint. It is useful for a concrete uncertain design choice, less useful as an automatic response to every failed test. Limit concurrent alternatives and make resource cost visible. Abandoned alternatives remain in Attempt/evidence history with a reason; retention of their Git objects is a deliberate policy. Never merge every alternative or erase losing approaches silently.

## Checkpoints versus commits

A Checkpoint is an immutable evidence boundary, not a mutable branch pointer and not a replacement for a Git commit. It identifies Work intent revision, Attempt, parent checkpoint(s), base/head/tree SHAs, evidence manifest digest and status. One checkpoint may include multiple commits. A commit may be referenced by several checkpoints, but its contents must not be described as separable checkpoint changes unless explicit ranges/trees support that claim.

Rebase/squash creates new Git identities and a new checkpoint linked as superseding the old one. Never relabel old evidence as having tested the new SHA. Rollback creates a new Attempt/checkpoint or revert commit; it does not delete workflow history. Retry creates another execution receipt, resume uses captured state plus recorded constraints. Stable database IDs survive Git rewriting; old object retention must survive ref removal if a checkpoint promises reconstructability.

## Conflict classes and safe repair

| Class | Detection | False positives / limits | Response and escalation |
| --- | --- | --- | --- |
| Textual | Git merge of exact trees | Conflicts do not imply either approach is wrong | Bounded resolver in a new Attempt; review resolved intent |
| Build | Build exact combined candidate | Broken infrastructure/toolchains can mimic code failure | Compare baseline, retain environment receipt, repair only reproducible delta |
| Test | Tests on base, individual and combined candidates | Flaky tests and incomplete coverage | Retry with limits, diagnose; human direction for ambiguous requirements |
| Behavioral | Explicit scenario/property/regression tests | Cannot infer unexpressed behavior | Independent tests and review; no automatic claim of completeness |
| API contract | Versioned schema/type/consumer compatibility gates | Contract may be too conservative or stale | Update provider/consumer together or block; changing contract requires intent approval |
| Dependency | Lockfile/resolver/ABI checks | Platform differences, transient registries | Pin environment, propose explicit dependency change, rerun checks |
| Design/architecture | Declared constraints plus reviewer findings | Agent inference has high ambiguity | Advisory finding; human chooses when constraints conflict or intent must change |

Existing semantic contract checks overlap the fourth/fifth rows only within their declared rules. They do not establish general architectural understanding. Store evidence-backed conflicts with a class and failed predicate; derive “compatible” only for a particular candidate and gate set, never universally.

Repair flow: failure receipt → classify → fresh limited resolution Attempt → independent review → exact combined candidate checks → publication policy → success or bounded alternate strategy → human direction. Resolution must preserve both goal revisions, enumerate deviations, and cannot disable tests, weaken policy or alter secrets/permissions to obtain green checks. Independence means a separate execution and immutable tested revision, not merely prompting the same agent twice. Stop on repeated equivalent failure, exhausted budget, missing evidence or a requested intent change. Humans are the last resort for routine mechanical repairs, but the rightful authority for conflicting goals.

## Queue, dependencies and speculation

Start with a queue plus explicit `depends_on` and `incompatible_with` evidence. Validate cycles, permissions and revisions. “Blocked” is a state, not a graph edge. An inferred overlap in files can prioritize testing but must not automatically declare incompatibility. Dependencies should identify a Proposal revision or accepted checkpoint, with an explicit policy for supersession.

A temporary integration candidate already resembles `main + A`. Extend only after measuring benefit: test bounded prefixes of the ready queue and a few declared dependency combinations. Do not enumerate subsets: 25 proposals have over 33 million subsets before test permutations. Cap depth, concurrent candidates, wall time and spend; cancel descendants when ancestors fail. Use demand-driven speculation when review is complete or integration is near, not on every keystroke.

Cache by repository, ordered candidate SHAs/tree, gate definition digest, toolchain/environment, dependency lockfiles and relevant policy version. A cache hit on a similar branch name is unsafe. Non-hermetic/network-dependent tests need expiration or re-execution. A final publication must still compare the current canonical head and source identities, irrespective of speculative success. A general integration graph is high value only if observed dependency bottlenecks justify its complexity.

## Minimal programmable primitives

Reuse the workflow engine: request an Attempt, capture a checkpoint, request review, test an integration candidate, request integration, request direction. Each returns a durable receipt and explicit status. Integration is a request to the fenced queue, never unrestricted `git push` from JS. Add hooks only for checkpoint acceptance, review rejection and conflict detection once their receipts exist. Keep mandatory authorization, resource limits and final publication invariants outside script control. Avoid an open plugin framework or arbitrary capability escalation.

Artifacts Git repositories remain source authority. Generated evidence blobs have a different lifecycle: use content digest, media type, length, repository access scope and producer run/agent/workspace/checkpoint/candidate identities. Store immutable manifests and bytes in appropriate object storage, with Trestle indexing/access metadata. R2 captures already provide part of this execution path; a generalized evidence store is proposed, not implemented. Git notes may provide portable discovery, but must not be the sole database or authorization system. Secret redaction precedes upload; content-addressing does not make private data public or safe. Verify digests on restore/download. Record missing/expired evidence explicitly.

## Minimal model and Git mapping

Reuse Repository, Work, Attempt, WorkflowRun, execution records, Findings, PR and queue. Add only Checkpoint manifest and Workspace lease when implementing their promises. A Proposal can initially be a versioned manifest/projection; an IntegrationAttempt can extend existing candidate/effect receipts. Conflict and Decision can be typed immutable events, CommitRef a value containing SHAs, and compatibility a derived result. Do not create thirteen independent CRUD systems just because thirteen nouns are useful.

| Product object | Git mapping | Database responsibility |
| --- | --- | --- |
| Work | Optional conventional branch namespace | Intent revisions, acceptance criteria, ownership |
| Attempt | Normal branch/ref from exact base | Parent Work/checkpoint, approach, lifecycle |
| Checkpoint | Retained commit/tree, optional tag/note pointer | Immutable manifest, evidence, supersession |
| Workspace | Owned clone/worktree/snapshot | Lease, capabilities, capture/recovery receipt |
| Proposal | Source/base SHA pair with diff | Review version, findings and decision |
| Integration candidate | Temporary merge commit/tree/ref | Ordered inputs, gates, receipts, publication intent |
| Acceptance | Ordinary merge/rebase/cherry-pick under explicit strategy | Decision plus old/new SHAs and provenance |

Git must remain usable without Switchyard. Clone, normal branches, external edits and imported PRs remain valid. External changes invalidate stale decisions rather than being overwritten. Database loss must not destroy Git; Git alone cannot reconstruct every private workflow decision or artifact. Back up both authorities and document that distinction.

## User journey

Open Work: goal and acceptance criteria, state, agents and competing Attempts. Select an Attempt: checkpoint timeline, implementation summary, exact revision, tests/artifacts, costs and unresolved findings. Compare two Attempts: criteria-by-criteria evidence, not an opaque numeric agent score. Review a Proposal: goal, diff, independently verified receipts and exceptions. Integration view explains dependencies, failed predicates and next safe action. Attention asks for a specific decision only after bounded automated work has stopped, with consequences and rationale retained.

Keep source, commits and branch/ref identities one click away. Hide mechanical workspace creation, not the identities needed to debug/trust the system. Existing repository/PR navigation can coexist with this journey. Avoid putting every execution log on the dashboard or replacing a familiar code browser with a workflow diagram.

## Prior art and falsification

[Gerrit changes](https://gerrit-review.googlesource.com/Documentation/concept-changes.html) and its [patch-set workflow](https://gerrit-review.googlesource.com/Documentation/intro-user.html) separate stable review identity from commit revisions. Borrow immutable revision-specific review, not a forced one-commit-per-task rule.

[Zuul gating](https://zuul-ci.org/docs/zuul/latest/gating.html) already uses speculative execution; [pipeline dependencies](https://zuul-ci.org/docs/zuul/latest/config/pipeline.html) make combined testing explicit. This is proven prior art, not unique novelty. Borrow bounded ordered testing and invalidation.

[GitHub merge queues](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue) and [GitLab merge trains](https://docs.gitlab.com/ci/pipelines/merge_trains/) show that combined-state CI belongs in integration, not just individual review. Switchyard must distinguish itself with intent/evidence and agent repair, rather than claim a queue alone is new.

[Sapling stacks](https://sapling-scm.com/docs/overview/stacks/) and [Graphite review guidance](https://www.graphite.com/docs/best-practices-for-reviewing-stacks) favor manageable dependent changes. Borrow explicit dependencies and small review units. Large dossiers should not excuse unreviewable changes.

[Jujutsu's working-copy design](https://github.com/jj-vcs/jj/blob/main/docs/working-copy.md) treats working state differently from conventional Git staging. Stable logical identity is useful, but Switchyard should not quietly commit every editor change or require a new VCS to clone its repositories. An immutable artifact digest is similarly useful without claiming full Nix-style hermetic builds.

Counterarguments: Git already solves snapshots, refs and merging well. A task can be much larger than one review, so replacing PRs with tasks collapses useful boundaries. Checkpoints add value only when evidence and recovery need a stable boundary. Most semantic conflicts are detectable only through explicit tests/contracts. Managed workspaces are operationally managed worktrees, not a new storage invention. General speculation can burn budget without accelerating decisions. Hidden branches damage trust. A workflow engine is acceptable as coordination infrastructure, but should serve repository changes rather than expand into unrelated automation.

## Value, cost and production boundary

| Idea | Classification | Prototype recommendation |
| --- | --- | --- |
| Work/Attempt/Proposal intent-and-evidence view | High competition value / cheap as a projection | Show existing linked data and unknown evidence honestly |
| Exact checkpoint manifest and comparison | High value / moderate additive work | Prove two revisions and criterion evidence; no full history rewrite |
| Bounded contract-conflict repair with independent gates | High value / expensive to prove with real agents | Use one reproducible example; preserve safety gates |
| Managed workspace lifecycle | High value / expensive | Demonstrate safe isolated execution/capture; defer fleet scheduler |
| Queue dependency metadata | High value / moderate | Demonstrate one declared dependency if necessary |
| Arbitrary speculative graph/subset testing | Interesting but unnecessary for prototype | Defer; measure queue bottlenecks first |
| Agent-inferred architectural auto-merge | Probably bad idea | Advisory findings only |
| Delete PRs/branches from the product | Probably bad idea | Keep compatibility and inspectability |

Production needs lease fencing, durable captures before GC, orphan discovery, exact-object retention, per-tenant quotas, permission-filtered manifests, secret handling, migrations, idempotent effects, backup/restore, audit retention, observability and eventually HA. Those are extensions of current correctness work; do not market single-node crash tests as HA. Avoid mutable evidence URLs, branch-name-only checks, free-form relationship strings and workspace cleanup without receipts: these choices would paint the system into a corner.

Open questions: Who can revise acceptance criteria after work begins? Which review revisions survive source changes? How long do abandoned Attempts and private artifacts remain reconstructable? Which gates are mandatory and who may change them? Which genuine queue bottlenecks justify speculation? Can external Git clients discover a useful manifest without leaking private decisions? What is the cost budget for competing real Agents? Which Agent credentials/adapters are actually available for C25?

Additional suggestion: define an explicit **unknown** evidence state and a freshness reason everywhere. A failed test, missing test, stale test and agent claim are materially different. Present costs and unsuccessful alternatives as learning, not as contributions that inflate activity counts.

## Proposed checkpoint-plan adjustment

Do not reorder C23 performance, C24 security/durability or the core C25 dogfood gate. Add design acceptance criteria to C25: demonstrate one Work, alternative Attempts, exact-revision evidence, an explicit conflict predicate and a justified integration decision; identify deterministic versus real Agent execution. Treat Proposal and Checkpoint as manifest/projection recommendations until approved implementation work exists. C40–C41 should tell this task-centered story alongside the canonical Git-host UI. Defer workspace fleet management and a general integration graph. The current model already captures most of the proposed foundation; the valuable change is stronger immutable intent/evidence binding and a clearer task journey, not a broad replacement of Git or the current queue.
