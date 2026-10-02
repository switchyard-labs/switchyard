# Switchyard Product/UX Baseline — PX0

Status: PASS

## Scope
This baseline records the pre-foundation state of the supplied workspace. It distinguishes implemented API/backend capability from visible product UX and preserves representative screenshots before design/model changes.

## Authenticated product routes currently present
- `edit.html`
- `index.html`
- `repo.html`
- `signin.html`
- `work.html`

## Public brochure routes currently present
- `/concepts/architecture.html`
- `/concepts/integration-queue.html`
- `/concepts/provenance-needs-attention.html`
- `/concepts/repositories.html`
- `/concepts/reviews-conflicts.html`
- `/concepts/work-attempts.html`
- `/concepts/workflows.html`
- `/docs/api.html`
- `/docs/configuration.html`
- `/docs/getting-started.html`
- `/docs/internals.html`
- `/docs/operations.html`
- `/docs/performance.html`
- `/docs/status.html`
- `/index.html`
- `/product/demo.html`
- `/product/how-it-works.html`
- `/product/human-ai.html`
- `/product/overview.html`

## HTTP API surface currently registered
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/repos`
- `GET /api/repos/{name}`
- `GET /api/repos/{name}/tree`
- `GET /api/repos/{name}/content`
- `GET /api/work`
- `POST /api/work`
- `GET /api/work/{id}`
- `POST /api/events/ingest`
- `GET /api/events`
- `POST /api/refs/update`
- `GET /api/repos/{name}/refs`
- `POST /api/work/{id}/attempts`
- `POST /api/attempts/{id}/run`
- `POST /api/attempts/{id}/pr`
- `POST /api/prs/{id}/check`
- `POST /api/prs/{id}/integrate`
- `GET /api/prs`
- `GET /api/events/stream`
- `GET /api/work/{id}/provenance`
- `POST /api/credentials`
- `GET /api/credentials`
- `POST /api/credentials/{id}/rotate`
- `DELETE /api/credentials/{id}`
- `GET /api/roles`
- `POST /api/workflows`
- `GET /api/workflows`
- `POST /api/workflows/{id}/run`
- `GET /api/workflow_runs`
- `GET /api/workflow_runs/{id}`
- `POST /api/workflow_runs/{id}/cancel`
- `POST /api/workflow_runs/{id}/approve`
- `POST /api/workflow_runs/{id}/retry`
- `POST /api/attempts/{id}/review`
- `POST /api/attempts/{id}/preview`
- `POST /api/attempts/{id}/resolve`
- `GET /api/findings`
- `POST /api/orgs`
- `POST /api/orgs/{id}/policies`
- `GET /api/orgs/{id}/fleet`
- `GET /api/audit`
- `GET /api/attention`
- `POST /api/attention/{kind}/{id}/escalate`
- `GET /api/escalations`
- `POST /api/escalations/{id}/decide`
- `POST /api/drafts`
- `GET /api/drafts/{repo}/{branch}/{path...}`
- `POST /api/drafts/{id}/commit`
- `POST /api/diff`
- `POST /api/prs/{id}/enqueue`
- `GET /api/queue`
- `POST /api/queue/{id}/requeue`

## Current coordination/product models
- users: `username`, `password_hash`, `display_name`
- sessions
- repositories: flat Artifacts names plus a small coordination registry
- Work / Attempts / Pull Requests / checks
- normalized events / ref observations / ref updates
- credentials / Agent executions / roles
- durable workflows and workflow steps
- findings / Integration Queue
- drafts with revision CAS and base-SHA protection
- escalations / organizations / policies / audit

The model is substantially richer than the visible UX. There are not yet first-class Git-host product surfaces for profiles, avatars, owner/repository namespaces, repository settings, user settings, organization profiles/teams/settings, commit/branch/tag browsing, or README-driven profile/repository presentation.

## Product UI capability vs product surface
| Capability | Backend/API | Current first-class UX |
| --- | --- | --- |
| Authentication | yes | sign-in/register only |
| Users | minimal record | no public profile/settings/avatar |
| Organizations | policy/fleet records | no GitHub-like organization product |
| Repositories | list/tree/content/refs | basic repository browser |
| Source rendering | raw bytes | plain `<pre>`; no syntax highlighting |
| Work | yes | basic create/list page |
| Attempts | yes | contextual/API only |
| Pull Requests | yes | API/flow exists; no mature PR page |
| Reviews/findings | yes | editor/contextual, not full review UX |
| Integration Queue | yes | API, limited visible surface |
| Workflows | yes | no mature product surface |
| Needs Attention | yes | no mature product surface |
| Editor | draft CAS/diff/Agent | functional CM6 page, minimal language/workbench UX |
| Credentials/roles | yes | no mature settings surface |
| Org policy/audit/fleet | yes | API-level proof, not Git-host settings UX |

## Current palette findings
The authenticated product still contains GitHub-derived blue tokens and blue interaction states. The brochure is already closer to the desired iron/graphite + amber/green direction.

### Product hard-coded / token colors
- `#0b0e14`
- `#0d1117`
- `#161b22`
- `#171a21`
- `#1c2029`
- `#22262f`
- `#2b2f3a`
- `#30363d`
- `#3b6ef6`
- `#58a6ff`
- `#8b949e`
- `#d9a441`
- `#e5484d`
- `#e6e8ee`
- `#e6edf3`
- `#f85149`
- `#fff`

### Brochure hard-coded / token colors
- `#08090d`
- `#0c0d11`
- `#13151b`
- `#1a1408`
- `#1a1d25`
- `#21252f`
- `#262b37`
- `#3fd0a8`
- `#9aa1b2`
- `#d96c5a`
- `#e9e7e2`
- `#f0a43a`

## Baseline usability/design findings
1. **Navigation is under-modeled.** The application exposes Dashboard / Repositories / Work even though the backend already supports PRs, workflows, queue, attention, organizations and policies.
2. **Repository identity is flat.** URLs and API calls identify repositories only by Artifacts name, which blocks a convincing `owner/repo` Git-host mental model.
3. **Identity is nearly absent.** Users and orgs are coordination records rather than rich Git-host identities.
4. **Repository browsing is a prototype.** The source view is a raw `<pre>` with no syntax, line anchors, README rendering, history or normal Git-host actions.
5. **Editor correctness exceeds editor presentation.** Save≠Commit and CAS are strong, but CM6 language/workbench ergonomics are intentionally minimal.
6. **Product visual system is generic GitHub-dark.** The blue accent conflicts with the requested Switchyard palette and with the newer brochure identity.
7. **Brochure information architecture is much stronger than the product shell**, but long navigation groups still need collapsible behavior rather than permanently expanded lists.

## Reference-quality observations
Warden demonstrates a stronger dense developer-tool workspace: integrated explorer/editor/Agent layout, deliberate panes, better workspace hierarchy and more mature interaction density. The Trestle/Nift/Strut sites demonstrate stronger documentation grouping and editorial hierarchy. These are references for care and structure, not templates to copy literally.

## Baseline constraints
The supplied workspace is offline from the live CP0 Linode/Trestle/Artifacts environment, so this baseline does not claim fresh live backend response timings. It records static-shell/browser timing and source-backed API/model inventory. Existing real-browser CP10 and curated demo screenshots/evidence remain preserved in repository history.

## Visual evidence
`docs/plan/evidence/px0/` preserves the real-product screenshots already captured from the live CP10/demo deployment (dashboard, repository browser, Work and editor where present). Fresh Chromium capture was attempted in this offline runner but did not terminate reliably, so the campaign records that as a tooling limitation rather than replacing real live evidence with fabricated screenshots.
