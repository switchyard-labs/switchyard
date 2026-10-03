Codex’s status report is much better than trying to infer completion from the repo. It’s explicit: **C0–C21 are done, C22 is only Chromium-certified, C23 is partial, and C24–C44 are not complete as checkpoints**. Pasted text

It also confirms several useful foundations are already in place: exact-SHA integration correctness, durable queue claims/recovery, durable workflow effects, a typed Go runner boundary, explicit Trestle migration handling, a real disposable Artifacts→Worker→Workflow→CI Sandbox→R2 path, Actions UI/logging, canonical repository routes, and the Warden-style editor with collapsible tree/drag-drop/file mutations. Pasted text

So the next prompt to Codex should **not redo C0–C21**. It should finish C22–C44, then review/update `sy`, while folding in the UI details from the screenshot and the Cloudflare competition items we discussed.

I’d send Codex this:

---

# Codex — Finish Switchyard C22–C44, close remaining UX defects, then review/update `sy`

Your reconciliation is accepted.

The current state is:

```text
C0–C21    DONE
C22       PARTIAL
C23       PARTIAL
C24–C44   NOT COMPLETE
```

with the qualifications in your handover. Pasted text

Do **not** spend time redoing completed C0–C21 work unless a later checkpoint exposes a regression.

Proceed through all remaining checkpoints:

```text
C22 → C44
```

and then perform the independent `sy` review/update described below.

Commit logically as you go.

Do not squash/amend prior campaign history.

Do not push until the final reconciliation report unless explicitly told otherwise.

---

# 1. Preserve current local work

Current project state from your handover:

```text
Switchyard changes committed locally
brochure changes committed locally
no project commits pushed
Linode has received authorized app/schema/assets updates
brochure local changes not externally published
sy/Warden/Gantry Core/Trestle source untouched
```

Preserve that. Pasted text

Start with:

```bash
git status
git branch -vv
git log --oneline --decorate -100
git remote -v
```

for:

```text
switchyard
switchyard-labs.github.io
switchyard-labs.github.io/public
sy
```

Do not reset or overwrite local campaign commits.

---

# 2. C22 — finish cross-browser certification

Chromium coverage is strong, but this checkpoint is not complete until the originally requested cross-browser coverage is addressed. Pasted text

Run the maintained browser suite in:

```text
Chromium
Firefox
WebKit where practical
```

At widths:

```text
1600
1280
1024
768
430
390
```

Cover:

```text
dashboard
repositories
profile
organization
repository
source
history
Work
PR
Actions
Actions logs
editor
settings
Needs Attention
workflow
signin/register
fullscreen navigation
```

Assertions must include:

```text
no JS page errors
no unexpected console errors
links navigate
menus open/close
outside-click dismissal works
keyboard navigation works
responsive layout holds
no hidden overlay intercepts clicks
```

Do not claim full browser parity if WebKit/Firefox cannot be run.

Record limitations explicitly.

Commit C22.

---

# 3. Profile/header UI — fix the remaining screenshot issues now

The latest profile screenshot is much better, but there are still visible defects.

## Header account control

Change authenticated global header from:

```text
[ avatar  alice ] [☰]
```

to:

```text
[avatar] [☰]
```

The profile link should be **avatar only**.

Make the account control exactly the same outer dimensions as the hamburger control.

For example, if the hamburger is:

```text
38 × 38
```

then the profile control is also:

```text
38 × 38
```

Requirements:

```text
same outer width
same outer height
same border
same radius
same vertical alignment
same spacing token
```

Use accessible naming:

```text
aria-label="View profile for alice"
title="alice"
```

Click goes to:

```text
/alice
```

No username text beside it.

---

# 4. Fix the stray square in the profile sidebar

The screenshot shows an isolated square beneath:

```text
Organizations
```

on the left of the contribution graph.

Investigate the DOM and fix the real cause.

Possible causes to inspect:

```text
empty organization avatar
empty org record
placeholder
failed image fallback
misplaced contribution cell
CSS grid artifact
```

Do not hide it with a blind:

```css
display:none
```

unless the element is genuinely a no-data placeholder.

Rules:

```text
no organizations → no orphan item
organization with no avatar → intentional fallback avatar
contribution cells → remain inside contribution grid
```

Add a browser regression assertion that the profile sidebar contains no unnamed/empty organization tile.

---

# 5. Profile page quality pass

Review `/alice` side-by-side with GitHub.

Current good pieces:

```text
large avatar
tabs
repositories
contribution calendar
activity
followers/following
organizations
```

Now refine:

```text
repository density
sidebar spacing
tab spacing
activity density
organization presentation
empty states
contribution graph sizing
```

## Remove the nested repository scrollbar if possible

The repository-card panel currently scrolls internally.

Prefer either:

```text
show recent/pinned subset + View all repositories
```

or:

```text
allow normal page flow
```

Nested scroll regions should be avoided unless they materially improve UX.

## Contribution graph

Verify:

```text
365 real dated cells
weekday alignment placeholders only where required
correct data mapping
accessible tooltip/title
no stray cells
responsive width
```

Profile contributions currently count attributable commits + new Work in visible repos, not imported Git history; preserve that limitation in docs/UI rather than implying GitHub-identical contribution semantics. Pasted text

---

# 6. C23 — finish real performance work

The current C23 work found and fixed a meaningful N+1-style lookup problem, with one sample dropping from roughly 320ms to 49ms, and benchmarked clones vs bare-mirror/worktree. Pasted text

Now complete the missing real measurements.

Measure:

```text
repository browse
source view
history
preview merge
semantic check
PR load
Actions load
queue integration
editor initial load
profile contributions
```

Use representative repeated runs.

Report:

```text
median
p95 if enough samples
min/max
cold vs warm where meaningful
```

Do not overstate tiny sample sizes.

## Remote Git reads

Investigate intermittent repository-load failures.

Specifically distinguish:

```text
Artifacts throttling
Git transport latency
token minting latency
Trestle latency
Switchyard application latency
network latency
```

## mirror/worktree

Do not adopt cached bare mirror/worktrees just because they sound faster.

Your current 25-way benchmark showed serialized worktree setup performing worse, so preserve the current approach unless better evidence justifies a change. Pasted text

Commit C23.

---

# 7. Fix Artifacts throttling/error mapping

This has now affected both Switchyard UX and `sy`.

DeepSeek’s `sy` dogfood repeatedly encountered:

```text
Artifacts upstream throttle
→ misleading 404/502 at Switchyard
```

Fix server error mapping.

Distinguish:

```text
repository_not_found
upstream_rate_limited
artifacts_unavailable
artifacts_auth_failed
upstream_timeout
```

Use sensible HTTP status codes.

Preserve `Retry-After` where upstream exposes it.

Do not make clients guess that:

```text
404 == maybe rate limit
```

Add tests.

This should help close the `sy repo list` 10/11 dogfood failure later.

---

# 8. C24 — operations/release hardening

Complete and certify:

```text
backup
restore
graceful shutdown
HTTP timeouts
request/body size limits
resource limits
metrics
worker leases
demo reset
TLS/domain readiness
dependency provenance
upgrade procedure
```

## Backup/restore

Actually rehearse:

```text
backup
fresh restore target
start
verify data
```

Include:

```text
Trestle data
Switchyard local data
credential encryption key
deployment config
```

Never print secrets.

## Graceful shutdown

Prove workers stop without abandoning:

```text
queue claims
workflow effects
Actions jobs
Git mutations
```

## Metrics

At minimum provide useful operational visibility:

```text
HTTP request counts/latency
Trestle errors
Artifacts errors/rate limits
queue depth/leases
workflow runs
Actions state
reconciliation outcomes
```

Commit C24.

---

# 9. C25 — real full dogfood

The current handover explicitly says **real external coding-agent provider is not certified**, and Workers Builds/Preview/deployment integration is still uncertified. Pasted text Pasted text

Therefore C25 is not done.

Build a reproducible real dogfood scenario with **genuine concurrent Agents**, not only deterministic adapters.

Target:

```text
Human creates Work
→ Agent A Attempt
→ Agent B Attempt
→ concurrent Git branches/state
→ both push to Artifacts
→ event subscriptions fire
→ Actions automatically run
→ CI checks exact SHAs
→ reviewer/findings
→ one candidate hits semantic conflict
→ repair or reject
→ selected PR enters Integration Queue
→ queue revalidates exact SHA
→ canonical branch updates
→ provenance records full chain
```

Prove:

```text
real provider
real credentials
real subprocess isolation
real Artifacts
real Actions
real queue
real Trestle
```

Commit C25.

---

# 10. C26 — single-binary Switchyard

This is now non-negotiable.

Strut is gone from the active runtime, so cash in on the simplification.

Goal:

```text
one switchyard binary
```

containing the Nift-built UI.

Build pipeline:

```text
content/templates/assets
→ nift build --all
→ public/
→ go:embed
→ switchyard
```

Nift remains the build/presentation layer.

Production runtime must NOT require:

```text
public/
templates/
Nift executable
Node
Strut
```

A development override like:

```text
SWITCHYARD_STATIC_DIR
```

is fine.

Default release behavior:

```text
serve embedded assets
```

Verify:

```text
dashboard
signin
profile
repo
Actions
editor
CSS
JS
images
CodeMirror
favicon
```

from embedded assets.

Commit C26.

---

# 11. C27 — release/deployment simplification

Final production deployment should become:

```bash
scp switchyard host:/opt/cp0/switchyard/switchyard
systemctl restart switchyard
```

not:

```text
copy binary
rsync public/
```

Provide build metadata:

```bash
switchyard --version
```

showing:

```text
version
commit
build date
Go version
```

Update:

```text
deployment docs
systemd docs
operations docs
brochure architecture
release docs
```

Commit C27.

---

# 12. C28 — canonical URLs

Complete the migration away from prototype routes.

Primary routes should be:

```text
/{owner}
/{owner}/{repo}
```

Examples:

```text
/alice
/alice/demo-semantic

/switchyard-labs
/switchyard-labs/switchyard
```

Then nested routes:

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

Exact path vocabulary can differ, but the model should be coherent.

Old:

```text
/repo.html?name=...
/profile.html?user=...
```

should redirect where mapping is possible.

Test:

```text
refresh
deep link
back/forward
copy URL
anonymous direct access
auth redirect
```

Commit C28.

---

# 13. C29 — breadcrumbs and owner identity

Every repository surface should communicate:

```text
[owner avatar] owner / repo      visibility
```

Owner links to:

```text
/{owner}
```

Repo links to:

```text
/{owner}/{repo}
```

From deeper views:

```text
owner / repo / current context
```

Examples:

```text
alice / demo-basic / src / main.go
alice / demo-basic / Actions / run 42
alice / demo-basic / Pull request 7
```

Apply across:

```text
repository
source
history
PR
Actions
settings
editor
```

Commit C29.

---

# 14. C30 — Code/clone popover

Fully certify the Code popover.

It should be anchored directly below/beside:

```text
Code ▾
```

Requirements:

```text
click button → open
click button again → close
outside click → close
Escape → close
focus returns to Code button
keyboard navigation works
mobile stays within viewport
scroll/reposition behaves
```

No centered/modal clone dialog.

HTTPS/SSH tabs only if actually supported.

Add regression tests.

Commit C30.

---

# 15. C31 — SSH support research

Research current Cloudflare Artifacts capability.

Answer with evidence:

```text
Does Artifacts support native SSH Git?
Does it support user public keys?
Does it support repo-scoped SSH authorization?
```

If yes:

```text
integrate native support
```

If no:

write a short decision comparing:

```text
HTTPS only
vs
Switchyard SSH gateway
```

Evaluate:

```text
security
Git protocol complexity
audit
key mapping
resource isolation
scaling
operational load
competition value
UX
```

Do not build an SSH gateway merely for GitHub parity if the cost is disproportionate.

Commit C31.

---

# 16. C32 — SSH public keys only if C31 justifies it

If SSH becomes a real supported path, add:

```text
Settings
→ SSH keys
```

Fields:

```text
title
public key
fingerprint
created
last used
delete
```

Rules:

```text
public key only
never private key
validate algorithm/encoding
server-side fingerprint
deduplicate
audit add/remove/use
```

Then prove:

```bash
git clone
git fetch
git push
```

over SSH.

If SSH is not justified, mark C32:

```text
NOT APPLICABLE / DEFERRED BY C31 DECISION
```

not fake-complete.

Commit C32 if applicable.

---

# 17. C33 — first-class clone credentials

Implement the server endpoint `sy` needs.

Flow:

```text
authenticated user
→ authorize owner/repo
→ request Git capability
→ mint short-lived repo-scoped Artifacts token
→ return clone metadata
```

Token properties:

```text
short lived
least privilege
repo scoped
read-only or read/write
never persisted
never logged
audited issuance metadata only
```

Then use this from web Code popover and later `sy`.

Commit C33.

---

# 18. C34 — meaningful Worker ↔ Artifacts binding

The Cloudflare competition specifically values direct Artifacts use from Workers.

The existing disposable CI path is useful foundation, but C34 requires a **real bounded production/demo responsibility**.

Use the Artifacts binding directly in a Worker for one or more of:

```text
inspect repo
read repo config
create/fork repo
mint scoped Git credentials
normalize push metadata
start CI/review flow
```

Document:

```text
why this belongs in Worker
why Go still owns the rest
```

Do not move the whole control plane to Workers.

Commit C34.

---

# 19. C35 — event subscriptions

Normalize the Artifacts event types actually exposed by the current platform.

At minimum consider:

```text
create
import
fork
delete
push
clone
fetch
```

Classify each:

```text
audit/activity
coordination
CI/review trigger
reconciliation hint
```

Normalized event should include where available:

```text
event ID
repo
ref
before SHA
after SHA
actor/source
timestamp
```

Persist event identity for deduplication.

Commit C35.

---

# 20. C36 — real automatic event-path certification

This must be a true live test.

Perform:

```bash
git clone ...
git checkout -b event-test
edit
git commit
git push
```

Then prove automatically:

```text
Artifacts event
→ subscription
→ Worker/Queue
→ Switchyard
→ Trestle
→ UI/SSE/Actions
```

No manual POST.

Measure latency.

Then test duplicate delivery.

Then temporarily bypass the fast path and prove reconciliation repairs eventual state.

Architecture remains:

```text
event subscription = fast path
reconciliation = safety net
```

Commit C36.

---

# 21. C37 — complete Cloudflare Actions alignment

Your handover says there is already a real disposable:

```text
Artifacts push
→ Worker
→ Workflow
→ CI Sandbox
→ R2
```

execution path. Pasted text

Now complete the competition-facing integration.

For Worker projects:

```text
Workers Builds
Worker Preview
deployment URL
```

For general CI:

```text
Workers Workflows
@cloudflare/ci
```

Bind all run/check state to exact Git SHA.

Integrate with:

```text
PR checks
Integration Queue required checks
Actions UI
Needs Attention when policy blocks
```

Do not move general CI onto the Linode.

Commit C37.

---

# 22. C38 — Actions final quality

The existing Actions work already includes list/detail pages, logs, follow/pause/search/copy/download/manual/rerun/cancel. Pasted text

Now certify final quality.

Run page should show:

```text
action/workflow
run number
branch
commit SHA
actor/event
jobs
steps
duration
artifacts
preview URL
deployment URL
rerun
cancel
```

Read-only log viewer should support:

```text
follow
pause
search
copy
download
ANSI safely
step boundaries
reconnect
```

Explicitly:

```text
NO stdin
NO shell
```

Compare directly with GitHub Actions and the Warden terminal presentation.

Commit C38.

---

# 23. C39 — `sy` API handoff first

Before touching `sy`, produce/update:

```text
docs/plan/sy-api-handoff.md
```

Document exact server contracts for:

```text
clone credentials
Actions list/view/run/cancel/logs
log streaming
Attempts
auth tokens/device auth if implemented
repo identity
rate-limit errors
```

Commit C39.

---

# 24. Review `sy` independently

DeepSeek has finished and frozen `sy`.

Read:

```text
sy/docs/codex-handover.md
sy/docs/switchyard-api-feedback.md
sy/docs/server-compatibility.md
```

Review:

```text
Cobra structure
command taxonomy
API client
human rendering
--json behavior
exit codes
config model
0600 permissions
atomic config writes
secret redaction
repo-context detection
remote parsing
clone credential hygiene
retry/idempotency
noninteractive behavior
cross-platform build
integration harness
release script
```

Central question:

> Does `sy` feel like a genuine Switchyard CLI peer to `gh`/`cf`, or like curl wrapped in Cobra?

Do not rubber-stamp DeepSeek.

---

# 25. Update `sy` after server APIs are ready

Once C33/C37 etc. exist, modify `sy`.

Add:

```text
sy actions list
sy actions view <run>
sy actions run <action>
sy actions cancel <run>
sy actions logs <run>
sy actions logs <run> --follow
```

Replace normal clone fallback with:

```text
Switchyard clone-token API
```

so normal users no longer need:

```text
SY_GIT_TOKEN
--token
```

If Switchyard now has top-level Attempts listing:

remove the bounded Work-detail scan.

If durable PAT/device auth is now available:

adopt it cleanly.

Preserve:

```text
token never in argv
token never in .git/config
clean --json
stable exit codes
```

Commit `sy` logically after the review.

---

# 26. Re-run `sy` dogfood

Target:

```text
11/11 PASS
```

including:

```text
auth
repo list/view/clone
work
attempt
PR
queue
attention
workflow
agent
org
Actions
```

If repo list still fails from genuine Cloudflare throttling:

report the upstream state.

But after fixing Switchyard error mapping, `sy` should at least receive a correct rate-limit error instead of misleading 404/502.

---

# 27. C40 — competition capability matrix

Create/update:

```text
docs/plan/cloudflare-competition-alignment.md
```

Mark:

```text
PASS
PARTIAL
NOT IMPLEMENTED
NOT REQUIRED
```

for:

```text
Artifacts = Git truth
multiple concurrent Agents
isolated Agent work
Worker uses Artifacts binding directly
repo-scoped Git credentials
event subscriptions
push-triggered automation
Workers materially used
Workers Workflows/CI
Workers Builds
Worker previews/deployments
human review
textual conflict
semantic conflict
Integration Queue
provenance
```

No inflated claims.

Commit C40.

---

# 28. C41 — competition demo

Build a polished reproducible demo:

```text
Human Work
→ 2 concurrent real Agent Attempts
→ Artifacts Git
→ push events
→ Worker
→ Actions
→ Cloudflare CI
→ streamed logs
→ reviews/findings
→ semantic conflict
→ repair/select candidate
→ Integration Queue
→ exact-SHA revalidation
→ canonical Git
→ Worker preview/deployment
→ provenance/activity
```

Capture:

```text
timeline
screenshots
Action logs
PR state
queue state
Git refs
preview URL
final provenance
```

If anything is manual, document it.

Commit C41.

---

# 29. C42 — canonical URL/breadcrumb/clone regression suite

Add browser tests for:

```text
/{user}
/{org}
/{owner}/{repo}
nested repo routes
```

Assert:

```text
owner breadcrumb
repo breadcrumb
deep link
refresh
old-route redirect
clone popover anchored
outside click closes
Escape closes
copy works
HTTPS works
SSH works only if implemented
```

Also include the profile header/avatar and orphan-square tests described above.

Commit C42.

---

# 30. C43 — clean binary-only environment certification

Create a clean disposable runtime containing only:

```text
switchyard binary
configuration
data directory
Trestle endpoint/config
Cloudflare credentials/config
```

No:

```text
public/
templates/
Nift
Node
Strut
frontend source
```

Start Switchyard.

Verify:

```text
all HTML routes
CSS
JS
images
favicon
CodeMirror
editor
Actions
profiles
repos
```

Inspect runtime file access/processes if practical.

Prove:

```text
production UI truly comes from the binary
```

Commit C43.

---

# 31. C44 — final architecture/docs cleanup

Final active architecture should reflect reality:

```text
Nift
  build-time UI layer

Go
  one Switchyard executable
  embedded product UI
  control plane
  runner abstractions

Trestle
  coordination truth

Cloudflare Artifacts
  Git truth

Cloudflare Worker
  event / Artifacts-native edge

Cloudflare Queue
  event transport

Workers Workflows / Builds / @cloudflare/ci
  Actions / CI
```

Strut may appear only historically.

Document:

```text
historical deterministic worker used Strut
current runtime does not
```

Update:

```text
brochure
architecture docs
operations docs
deployment docs
sy handoff
competition docs
```

Commit C44.

---

# 32. Refresh the brochure screenshots after all app work

Capture final deployed screenshots:

```text
profile
repository
source
PR
Actions list
Action run/logs
Work
editor
Needs Attention
workflow
```

Update brochure through Nift.

Use proper:

```text
@path(...)
```

everywhere.

Do not manually patch generated output without source changes.

---

# 33. Final design-quality pass

Before calling the campaign done, compare rendered output against:

```text
GitHub
GitLab
Warden
```

Especially:

```text
profile
repository
Actions
editor
PR
settings
```

The success criterion is not:

```text
all routes exist
```

It is:

> a developer would not be embarrassed to show Switchyard next to GitHub.

Keep Switchyard's own graphite/amber identity.

Do not turn it into a blue GitHub clone.

---

# 34. Final acceptance gates

Run:

```bash
go build ./...
go test ./...
go vet ./...
nift build --all
git diff --check
```

Browser:

```text
Chromium
Firefox
WebKit where practical
```

Cloudflare:

```text
real push event
real CI
real Actions
real Agent dogfood
```

`sy`:

```bash
go test ./...
go vet ./...
go build ./cmd/sy
scripts/integration.sh
```

Check every working tree.

---

# 35. Do not push until final report

Keep commits local until Nick reviews the final campaign.

Do not push:

```text
switchyard
brochure
sy
```

until explicitly told.

Cloudflare disposable/demo resources may be deployed as required for certification.

Do not modify:

```text
Warden
Gantry Core
Trestle
Nift
```

source.

---

# 36. Final report

Return:

## A. Final checkpoint ledger C0–C44

For every checkpoint:

```text
status
commit
evidence
limitations
```

## B. C22 browser matrix

## C. C23 performance measurements

## D. C24 operational certification

## E. C25 real-Agent dogfood

## F. Single-binary result

Include binary size and runtime dependencies.

## G. Canonical routing

Examples.

## H. Breadcrumb result

## I. Clone UX

## J. SSH decision/result

## K. Clone credential API

## L. Worker ↔ Artifacts direct binding

## M. Event subscription evidence

Include real push timing.

## N. Actions result

## O. Competition matrix

## P. Competition demo

## Q. Profile/UI fixes

Explicitly confirm:

```text
header profile link = avatar only
avatar control same dimensions as hamburger
orphan sidebar square removed
repository nested scrollbar addressed
contribution graph verified
```

## R. Warden editor comparison

## S. `sy` independent review

## T. `sy` changes made

## U. `sy` live dogfood result

## V. Brochure screenshots/content refresh

## W. Remaining blockers

Use only:

```text
must fix
should fix
nice to have
defer
```

## X. Git status/log

List all local commits awaiting approval.

Then stop.

---

The main thing I’d emphasize to Codex is that **it has already done the hard correctness work**. The remaining campaign is now very concrete: finish certification, single-binary packaging, Cloudflare competition integration, canonical Git-host polish, and make `sy` consume the APIs Switchyard was supposed to grow.

The handover is actually encouraging: the editor, Actions core, queue durability, workflow recovery, canonical repo routing, profiles, stars/follows/contributions and direct Cloudflare CI foundation are already there. Pasted text We’re no longer asking Codex to “make Switchyard into a product”; we’re asking it to **finish and certify the product it has already built**.