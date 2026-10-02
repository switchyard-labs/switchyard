# Switchyard product/UX campaign certification

## Executive result

The PX0–PX25 campaign has transformed the supplied checkpoint UI into a substantially broader Git-host product surface while preserving the core agent-native architecture. The source tree now contains coherent first-class surfaces for ordinary Git use **before** a user has to understand Attempts, Agents or the Integration Queue.

This artifact-runner certification is **not a public-release certificate**. It is a static/offline product certificate. The normal development environment must still close the live gates recorded by each CONDITIONAL PASS checkpoint (Go 1.26, Trestle, Cloudflare Artifacts and real browser matrix).

## Before → after

| Area | Before campaign | After campaign |
| --- | --- | --- |
| Repository identity | flat Artifacts name | durable `owner/repo` public identity separated from physical Git storage |
| Code browsing | plain `<pre>` | language detection, syntax tokens, lines/anchors, branch selector, README/image rendering, file actions |
| Editor | isolated CM6 gate page | Warden-inspired Files/Search/Source workbench, tabs, status, multi-file flows, Agent draft proposals |
| Users | auth record | avatar/profile/README/pins/activity/settings/security |
| Organizations | policy/fleet record | avatar/profile/README/repos/members/teams/invitations/access/settings |
| Repository landing | file tree | metadata, clone, latest commit, branch/tag counts, README, repo tabs |
| Git history | CLI/API only | commits, commit detail/provenance, branch selector, compare/diff |
| Pull Requests | API object | list/detail, conversation, files, checks/findings, queue state, actions |
| Work | thin task record | issue-like body/status/assignee/comments + optional Attempts/PRs |
| Repository settings | absent | general/access/protected refs/integration/danger-zone surface |
| Advanced operations | raw APIs | Needs Attention, Integration Queue, Workflows/Runs, Agent Roles/Executions |
| Public demo | SSH tunnel only | read-only demo mode/security contract ready for dedicated HTTPS deployment |
| Accessibility | opportunistic | skip links, focus/reduced-motion/overflow/mobile hardening + static gate |

## Route/product coverage

### Global
- `/` — dashboard
- `/signin.html` — authentication
- `/organizations` — organizations index/create
- `/{owner}` — user or organization profile
- `/settings` — user/account/security/provider settings
- `/organizations/{org}/settings` — org administration
- `/work.html` and `/work/{id}` — Work list/create/detail/conversation
- `/operations` — advanced operational views

### Repository
- `/{owner}/{repo}` — repository overview/code
- `/{owner}/{repo}/blob/{ref}/{path...}` — source/file view
- `/{owner}/{repo}/tree/{ref}/{path...}` — tree context
- `/{owner}/{repo}/commits/{ref}` — commit history
- `/{owner}/{repo}/pulls` — Pull Requests
- `/{owner}/{repo}/pull/{id}` — Pull Request detail
- `/{owner}/{repo}/settings` — repository settings
- `/edit.html?...` — browser workbench/editor

## GitHub / GitLab / Bitbucket parity classification

| Capability | Status | Switchyard interpretation |
| --- | --- | --- |
| user profiles + avatars | Implemented | large profile identity, Git-backed profile README |
| org profiles + avatars | Implemented | Git-backed org profile README, repos/members |
| owner/repo namespace | Implemented | public identity separated from Artifacts physical name |
| repo README | Implemented | sanitized renderer with relative images/links |
| syntax-highlighted browsing | Implemented | shared language registry/token palette |
| commits/branches/tags | Implemented core | history/compare; releases and blame deliberately later |
| Pull Requests | Implemented core | familiar outer model + progressive Switchyard state |
| Issues | Switchyard equivalent | Work is the generalized human-first issue/task object |
| collaborators/teams | Implemented core | user-repo collaborators + org teams/access records |
| protected refs | Implemented model/settings | full enforcement remains CP14 security-hardening work |
| repo settings | Implemented core | transfer/delete deliberately disabled pending hardening |
| workflows/CI | Switchyard equivalent | durable programmable JS workflows + runs/steps |
| code review findings | Implemented | Review Findings + textual/semantic conflict layers |
| web editor | Implemented | Warden-inspired workbench; Save ≠ Commit + draft CAS |
| notifications | Deferred | no fake notification product |
| followers/social graph | Not useful for v1 | deliberately not cargo-culted |
| stars/watch counters | Not useful for v1 | no product requirement established |
| releases/packages | Later | Git tags exist; full release UX not required by this campaign |
| enterprise SSO/compliance | Later | org/policy architecture exists; enterprise security not claimed |

## Warden editor-quality comparison

Switchyard now adopts the useful Warden workspace principles:
- integrated file explorer/search/source modes;
- tabs and multi-file context;
- deliberate dense desktop workbench;
- status information;
- resizable/contextual panes;
- integrated hideable Agent surface.

Switchyard deliberately does **not** adopt Warden's terminal/server-management scope. Its editor is repository collaboration software, not a server IDE.

## Remaining known UX/product gaps

1. Fresh real-browser visual review of the complete PX25 product has not run in this artifact runner.
2. Live Trestle/Artifacts gates remain for most identity/parity checkpoints.
3. Public demo deployment still needs HTTPS, rate/abuse controls and dedicated reset/credential operations.
4. Interactive Agent-to-draft is implemented against the deterministic/runner substrate; the real provider-backed coding-Agent condition from CP6 remains separately bounded.
5. Repository protected-ref/settings enforcement needs full CP14 adversarial/security coverage.
6. Repository transfer/delete are intentionally disabled.
7. Notifications, releases, blame and enterprise SSO are deliberately deferred rather than partially faked.
8. Final screenshots must be regenerated from the normal live environment because the supplied runner cannot reliably complete the required browser lifecycle.

## Bundle/performance implications

The campaign adds several small page-specific JS islands rather than turning the application into a monolithic SPA. The largest existing browser asset remains the vendored CodeMirror bundle. Shared code/Markdown renderers avoid duplicating source presentation logic. CP14 should benchmark the final browser bundle and retain lazy/contextual loading where useful.

## Security implications to carry into CP14

- Markdown remains sanitized; README/profile content does not become arbitrary HTML execution.
- avatars have server-side MIME/size validation from the identity phase.
- demo guest mode is centrally read-only and cannot use legacy flat repo APIs.
- user/org/repo settings use server-side authorization, not hidden-button security.
- protected refs and repository integration policy now have product models, but enforcement must be adversarially certified.
- provider secrets remain separate from Git/repository/account permissions.

## Recommended CP14 ordering

1. **Close all conditional live PX gates** on the normal development host: Go 1.26 build/test, Trestle/Artifacts fixtures, desktop/mobile Chromium, then Firefox/WebKit where available.
2. **Authorization/security matrix:** users/orgs/repos/settings, CSRF/session, path traversal, shell args, Markdown/image policy, Agent capability escalation.
3. **Protected-ref + Integration Queue enforcement:** external/direct push interactions, force movement, queue crash after merge/state update.
4. **Schema migrations/versioning:** the product campaign added durable coordination collections; migration must become explicit.
5. **Performance:** cached bare mirror/worktree vs scratch-clone baseline; browser bundle/page timings.
6. **Operations:** HTTPS deployment, service lifecycle, backup/restore, logs/metrics, upgrade/disaster recovery.
7. **Public demo certification:** dedicated reset credentials, limits, abuse controls and regenerated screenshots.
8. **Release candidate / clean-machine certification.**

## Certification questions

### Would we be comfortable sending a developer to the demo without explaining the interface first?

**Not yet as an externally hosted demo**, because the dedicated HTTPS deployment and live final browser/permission matrix have not been certified. The product information architecture itself is now designed to be understandable without agent-specific explanation: owner/repo, Code, Commits, Work, Pull Requests and Settings are the primary mental model.

### Does Switchyard look and behave like a serious Git collaboration product before its Agent-native features are even considered?

**At the source/product-design level, yes: this is now the intended shape.** It has ordinary Git identity, code browsing, history, PRs, Work, profiles/orgs and settings before advanced operations are required. Final visual confidence still depends on the fresh browser matrix/screenshots in the normal environment, so this artifact runner does not claim visual-release certification it could not observe.
