I’d add a **final extension block after C25**, so Codex finishes the core campaign first and then does a focused “Cloudflare competition + canonical Git-host UX + single-binary” pass.

I’d steer it with something like this:

---

# Additional Codex checkpoints — Cloudflare competition alignment, canonical routing, SSH, clone UX, and single-binary release

After completing the existing C5–C25 campaign, continue with the following additional checkpoints.

Do not reorder these ahead of the correctness/security/durability work.

Commit at the end of each checkpoint.

Do not squash.

---

# C26 — single-binary Switchyard

Now that Strut has been removed from the runtime path, simplify deployment properly.

Goal:

```text
one Switchyard executable
```

should contain everything required for the normal product UI.

Target deployment:

```text
/opt/cp0/switchyard/switchyard
/opt/cp0/switchyard/cp0.env
/opt/cp0/switchyard/data/
```

rather than requiring a separately synced `public/` tree for ordinary operation.

## Embed the web assets

Use Go embedding where appropriate:

```go
//go:embed ...
```

Embed:

- Nift-built HTML;
- CSS;
- JavaScript;
- favicon/images needed by the application;
- CodeMirror assets if practical;
- other static runtime assets.

Nift should **still be the source/build layer**.

Desired build pipeline:

```text
content/
templates/
assets/
      ↓
nift build --all
      ↓
generated public/
      ↓
go:embed
      ↓
single switchyard binary
```

Do not replace Nift.

Do not hand-maintain duplicate embedded templates.

The generated Nift output should remain the source for embedded UI assets.

## Preserve development ergonomics

Ideally support:

```text
development:
  optionally serve local public/ for fast UI iteration

release:
  serve embedded filesystem
```

If an override is useful, something like:

```text
SWITCHYARD_STATIC_DIR=/path
```

can take precedence over embedded assets in development.

But a default production binary should need no static directory.

## Gates

Prove:

```bash
nift build --all
go build ./cmd/switchyard
```

Then copy **only the binary** plus config/data to a clean disposable directory/host and prove:

```text
dashboard works
signin works
repo works
editor works
Actions works
all CSS/JS/images work
favicon works
```

Verify no accidental runtime dependency on:

```text
public/
templates/
Strut
Node
Nift
CodeMirror source tree
```

Nift is a build-time dependency, not a runtime dependency.

Commit C26.

---

# C27 — single-binary release and deployment simplification

Update the deployment process around the embedded binary.

Desired deployment:

```text
build
→ copy switchyard binary
→ restart service
```

No normal:

```text
rsync public/
```

step.

Update:

- systemd docs;
- deployment docs;
- backup/restore docs;
- release documentation;
- brochure architecture;
- operations docs.

Ensure:

```text
switchyard version
```

or equivalent reports useful build metadata:

```text
version
commit
build date
Go version
```

If useful, support:

```text
switchyard --version
```

Superseded by Nick’s October 4 continuation: Git is an allowed external runtime
dependency. Do not embed or replace Git; deploy one Switchyard application binary.

Commit C27.

---

# C28 — canonical Git-host URLs everywhere

Replace prototype-era public URLs such as:

```text
/repo.html?name=demo-semantic
/profile.html?user=alice
```

with canonical Git-host URLs.

Target:

```text
/{owner}
/{owner}/{repo}
```

where `owner` may be:

```text
user
organization
```

Examples:

```text
/alice
/alice/demo-basic

/switchyard-labs
/switchyard-labs/switchyard
```

Then use natural nested routes:

```text
/{owner}/{repo}/tree/{ref}
/{owner}/{repo}/blob/{ref}/{path}
/{owner}/{repo}/commits/{ref}
/{owner}/{repo}/commit/{sha}
/{owner}/{repo}/branches
/{owner}/{repo}/tags
/{owner}/{repo}/pulls
/{owner}/{repo}/pull/{id}
/{owner}/{repo}/work
/{owner}/{repo}/actions
/{owner}/{repo}/settings
```

Use whichever exact route vocabulary best fits the product, but keep it coherent and GitHub-familiar.

## Compatibility

Old URLs may redirect:

```text
/repo.html?name=...
→ /owner/repo
```

where identity can be resolved safely.

Do not maintain query-string URLs as the primary navigation model.

## Browser history

Verify:

- back/forward;
- refresh;
- deep linking;
- copied URLs;
- direct unauthenticated navigation;
- canonical links.

Commit C28.

---

# C29 — owner/profile breadcrumbs and repository identity

Improve repository identity/navigation.

Repository header should clearly show:

```text
[avatar] owner / repository       visibility
```

Both should link appropriately.

Example:

```text
[avatar] switchyard-labs / switchyard
         └──────────────┘
              ↓
      /switchyard-labs
```

Owner breadcrumb behavior:

```text
user owner
→ user profile

org owner
→ organization profile
```

Repo name from deeper pages should link to repo root.

On:

```text
source
history
PR
Actions
settings
editor
```

the user should always have a clear route back through:

```text
owner
repo
current section
```

Use familiar breadcrumbs rather than giant bespoke navigation widgets.

Commit C29.

---

# C30 — repository Code/clone popover UX

Rework the Code menu to feel like a serious Git host.

The popover must be anchored to the **Code** button itself.

Not:

```text
centered modal
random floating card
```

Target interaction:

```text
                          [ Code ▾ ]
                              ┌───────────────────────────┐
                              │ HTTPS   SSH               │
                              │ https://...      [copy]   │
                              │                           │
                              │ sy repo clone owner/repo  │
                              └───────────────────────────┘
```

## Behavior

Must support:

```text
click Code
→ open

click Code again
→ close

click outside
→ close

Escape
→ close

select clone mode
→ remains logically focused

close
→ focus returns to Code button
```

Ensure it:

- stays positioned near trigger;
- remains inside viewport;
- behaves on mobile;
- handles scroll;
- works keyboard-only;
- has proper ARIA semantics.

Add browser regression tests.

Commit C30.

---

# C31 — investigate SSH clone support

First, inspect the **current Cloudflare Artifacts documentation/API**.

Do not assume SSH exists.

Determine:

```text
Does Artifacts natively support:
  SSH Git transport?
  user SSH public keys?
  repo-scoped SSH auth?
```

If yes:

implement against the native facility.

If no:

do **not** immediately build an SSH server.

Write a short architectural decision comparing:

```text
A. HTTPS + scoped tokens only

B. Switchyard SSH gateway
   ssh://git@host/owner/repo.git
      ↓
   authenticate user's public key
      ↓
   authorize repo operation
      ↓
   mint/use scoped Artifacts credential
      ↓
   proxy Git operation
```

Evaluate:

- security;
- process isolation;
- Git protocol complexity;
- operational burden;
- auditing;
- scaling;
- user experience;
- competition relevance.

Only implement the gateway if it is clearly justified and bounded.

Commit C31.

---

# C32 — SSH public-key settings

If C31 concludes SSH is supported or an SSH gateway is justified, implement the user-facing key model.

Settings section:

```text
SSH and signing keys
```

or simply:

```text
SSH keys
```

Support:

```text
Add SSH key
Title
Public key
Fingerprint
Created
Last used
Remove
```

Requirements:

```text
store public key only
never accept/store private keys
validate supported formats
normalize safely
compute fingerprint server-side
deduplicate
associate with user
audit add/remove/use
```

Potential supported key types:

```text
ssh-ed25519
ecdsa-sha2-nistp256
rsa-sha2-* / ssh-rsa only if deliberately supported
```

Prefer modern algorithms.

Do not claim SSH support until actual clone/fetch/push works.

Commit C32.

---

# C33 — HTTPS clone-token API

Regardless of SSH, add the missing first-class clone credential API that `sy` currently needs.

`sy` should not depend on:

```text
SY_GIT_TOKEN
--token
```

for normal use.

Target server flow:

```text
authenticated Switchyard user
→ request credential for owner/repo
→ authorize operation
→ mint short-lived repo-scoped Artifacts credential
→ return clone metadata
```

Potential endpoint shape:

```text
POST /api/repositories/{owner}/{repo}/git-credential
```

or a better API if appropriate.

Response conceptually:

```json
{
  "remote": "https://...",
  "token": "...",
  "expires_at": "...",
  "permissions": ["read"]
}
```

For push:

```text
read/write credential
```

only if caller is authorized.

Security:

```text
short lived
repo scoped
least privilege
never persisted by Switchyard
never logged
audited issuance metadata
```

This should unblock:

```text
sy repo clone
```

cleanly.

Coordinate API contract documentation for the later `sy` review.

Commit C33.

---

# C34 — direct Artifacts binding from a Cloudflare Worker

The competition strongly emphasizes managing Artifacts directly from Workers.

Ensure Switchyard has a real production/demo path where a Cloudflare Worker uses the **Artifacts binding directly**, not merely the Go control plane calling REST.

Implement a bounded, meaningful Worker role.

Good candidates:

```text
repo create/fork
repo inspect
read AGENTS.md / config
mint scoped Git credential
normalize repo event
start CI/review flow
```

Do not move the whole Switchyard application to Workers merely for optics.

The Worker should have a clean responsibility.

Document:

```text
what runs in Worker
why it runs there
what remains in Go
```

Commit C34.

---

# C35 — Artifacts event subscriptions: react to every repository change

Cloudflare explicitly highlights reacting to repository events.

Implement/certify the real fast path.

Target:

```text
Artifacts
  repo event
      ↓
event subscription
      ↓
Cloudflare Worker / Queue
      ↓
normalized Switchyard event
      ↓
Trestle / workflow / review / Actions / UI
```

Support/normalize the relevant event types exposed by Artifacts, including where available:

```text
repository create
import
fork
delete
push
clone
fetch
```

Not every event needs to trigger heavy automation.

Classify them:

```text
audit/activity only
coordination update
CI/review trigger
reconciliation hint
```

## Push event

Push must be a first-class gate.

Normalize:

```text
repo
ref
before SHA
after SHA
actor/source if available
event ID
timestamp
```

Use durable event identity for deduplication.

Commit C35.

---

# C36 — prove event subscription fast path is automatic

This should be a real live certification, not a unit-test claim.

Perform:

```bash
git clone ...
git checkout -b event-test
edit file
git commit
git push
```

Then prove without manually invoking Switchyard:

```text
Artifacts event emitted
→ subscription delivered
→ Worker/Queue handled
→ Switchyard observed event
→ Trestle record updated
→ SSE/activity UI updated
```

Record timestamps for latency.

Also prove duplicate delivery is harmless.

Then disable/bypass the event fast path temporarily and prove:

```text
reconciliation
```

eventually repairs state.

The architecture should remain:

```text
event subscription = fast path
reconciliation = safety net
```

not the other way around.

Commit C36.

---

# C37 — competition-grade Cloudflare Actions integration

Finish the planned Actions work with direct Cloudflare alignment.

Target:

```text
Artifacts push
→ Worker
→ Workers Workflow / Workers Build / @cloudflare/ci
→ Action run
→ Checks
→ PR
→ Integration Queue
```

For Worker projects, support:

```text
Workers Builds
Worker Previews
deployment URL
```

For general CI:

```text
Workers Workflows
@cloudflare/ci runner
```

Do not execute general CI on the Linode merely because it is easier.

Commit C37.

---

# C38 — Actions UI + read-only terminal quality

Finish the GitHub-Actions-like UI.

Repository tab:

```text
Actions
```

Run list:

```text
name
status
branch
commit
trigger
actor
duration
```

Run detail:

```text
jobs
steps
timings
logs
artifacts
preview
deployment
rerun
cancel
```

Logs should use the Warden-quality terminal visual language, but:

```text
READ ONLY
```

No stdin.

No shell.

Clicking a job/step should focus the relevant logs.

Support:

```text
follow
pause follow
search
copy
download
```

Commit C38.

---

# C39 — update `sy` API handoff, but do not modify `sy`

DeepSeek currently owns/completed the `sy` CLI.

Do **not** modify the `sy` repository during this Switchyard campaign.

Instead document the now-available server APIs for later Codex review of `sy`.

Produce:

```text
docs/plan/sy-api-handoff.md
```

Include exact contracts for:

```text
clone credential
Actions list
Actions view
Actions run
Actions cancel
Actions logs
Actions log follow/stream
Attempts
auth token improvements if any
```

When Switchyard is complete, we'll separately let Codex review/update `sy`.

Commit C39.

---

# C40 — Cloudflare competition compliance matrix

Create a durable document:

```text
docs/plan/cloudflare-competition-alignment.md
```

Use the competition/blog requirements as evidence.

Include a matrix:

| Capability | Switchyard implementation | Evidence/demo |
|---|---|---|
| Cloudflare Artifacts as Git truth | | |
| Multiple concurrent Agents | | |
| Isolated Agent work | | |
| Direct Artifacts binding in Worker | | |
| Repo-scoped Git credentials | | |
| Event subscriptions | | |
| Push-triggered automation | | |
| Workers used materially | | |
| Workers Workflows / CI | | |
| Worker previews/deployments | | |
| Human review | | |
| Conflict handling | | |
| Deterministic integration | | |
| Provenance | | |

For each mark:

```text
PASS
PARTIAL
NOT IMPLEMENTED
NOT REQUIRED
```

No marketing inflation.

Commit C40.

---

# C41 — competition demo scenario

Build a polished end-to-end demo specifically suitable for showing Cloudflare.

Example:

```text
1. Human creates Work
2. Two Agents start concurrent Attempts
3. Artifacts forks/isolated branches are created
4. Agents push ordinary Git
5. Artifacts event subscription fires
6. Worker receives push event
7. Actions automatically starts
8. Cloudflare CI runs tests
9. logs stream into Switchyard
10. reviewer/Agent produces findings
11. semantic conflict is detected for one candidate
12. preferred candidate enters Integration Queue
13. exact-SHA gates revalidate
14. canonical Artifacts branch moves
15. Worker Preview/deployment becomes available
16. provenance/activity show the complete chain
```

Capture:

- UI screenshots;
- event timeline;
- Actions logs;
- PR;
- Integration Queue;
- final Git state.

This should be reproducible.

Commit C41.

---

# C42 — canonical URL + clone + SSH browser regression gate

Add permanent tests covering:

```text
/{user}
/{org}
/{user}/{repo}
/{org}/{repo}
```

Assert:

```text
owner breadcrumb links correctly
repo breadcrumb links correctly
refresh works
deep link works
old URL redirects where supported
clone popover opens at button
outside click closes
Escape closes
copy works
HTTPS tab works
SSH tab works if supported
```

If SSH is not implemented, do not render a fake active SSH tab.

Commit C42.

---

# C43 — single-binary clean-host certification

Final deployment gate.

Create a clean disposable environment with:

```text
switchyard binary
env file
empty data dir
Trestle endpoint
Cloudflare credentials/config
```

No:

```text
public/
templates/
Strut worker
Node
Nift runtime
frontend source tree
```

Start Switchyard.

Verify every important route and static asset.

Then inspect runtime file opens/processes if practical to prove there is no hidden dependency.

The final deployment story should be:

```bash
scp switchyard host:/usr/local/bin/switchyard
systemctl restart switchyard
```

plus config/data.

Commit C43.

---

# C44 — final architecture/documentation cleanup

Update all current documentation/diagrams to reflect the final architecture:

```text
Nift
  build-time presentation layer

Go
  single Switchyard binary
  control plane
  embedded application UI
  runner interfaces

Trestle
  coordination truth

Cloudflare Artifacts
  Git truth

Cloudflare Worker
  Artifacts-native programmable/event edge

Cloudflare Queue
  event transport

Workers Workflows / Builds / @cloudflare/ci
  Actions / CI
```

Strut should no longer appear in the active runtime architecture.

Historical docs may say:

```text
Earlier checkpoints used a Strut deterministic worker.
```

Do not rewrite old checkpoint evidence.

Commit C44.

---

# Final acceptance checklist

Before stopping, prove:

```text
[ ] Switchyard ships as one binary
[ ] no runtime public/ directory required
[ ] no runtime Strut dependency
[ ] canonical /owner/repo URLs
[ ] /owner profile/org URLs
[ ] repo owner breadcrumbs
[ ] Code popover anchored correctly
[ ] outside click closes popover
[ ] HTTPS clone works
[ ] short-lived clone token works
[ ] SSH investigated honestly
[ ] SSH keys work if implemented
[ ] Worker uses Artifacts binding directly
[ ] Artifacts event subscriptions live
[ ] push automatically triggers event path
[ ] reconciliation remains safety net
[ ] Actions runs on Cloudflare primitives
[ ] Actions logs stream read-only
[ ] competition matrix complete
[ ] end-to-end competition demo works
```

---

# Final handover

After C26–C44 return:

## A. Single-binary result

Binary size, embedded assets, runtime dependencies.

## B. Canonical URL result

Examples for user/org/repo.

## C. Breadcrumb/navigation result

## D. Clone UX

HTTPS and SSH.

## E. SSH decision

Native Artifacts support vs Switchyard gateway vs deferred.

## F. Direct Worker/Artifacts integration

Exactly what the Worker does.

## G. Event subscription result

Which events are subscribed and what each triggers.

## H. Live push-event certification

Include observed end-to-end latency.

## I. Actions result

Workers/Workflows/Builds/CI integration.

## J. Competition alignment matrix

## K. Competition demo

Exact reproducible sequence.

## L. Tests/browser gates

## M. Git commits

C26–C44 SHA ledger.

## N. Remaining gaps

Only real remaining gaps.

---

I’d especially make **C26 single-binary** non-negotiable now. Strut was the obvious separate runtime component, and now that it’s gone, Switchyard should cash in on that simplification instead of still behaving operationally like a multi-artifact deployment.

The clean story becomes:

```text
Nift builds the UI
        ↓
Go embeds it
        ↓
one switchyard binary ships
```

which is a much nicer deployment story for both humans and Agents—and a much cleaner thing to show Cloudflare.