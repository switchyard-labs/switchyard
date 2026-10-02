# Switchyard Product, UX, Git-Hosting Parity & Website Campaign

## Purpose

This handover defines the next **product-quality campaign** for Switchyard after the CP1–CP13 implementation and post-CP13 correction work.

This is deliberately **not** CP14 hardening. It is a separate, checkpointed campaign focused on:

- making the authenticated Switchyard application feel like a credible GitHub/GitLab/Bitbucket-class Git collaboration product;
- preserving and progressively exposing Switchyard's genuinely new agent-native concepts;
- lifting the browser editor/repository viewer toward the quality bar set by Warden;
- rebuilding the public brochure/documentation site to the quality bar of Nift, Trestle, Strut and the stronger Gantry project sites;
- creating coherent user, organization, repository, settings and identity surfaces rather than API-only capabilities;
- making syntax highlighting, Markdown, READMEs, avatars, navigation and responsive behavior first-class;
- producing a public-demo-quality product rather than a checkpoint UI.

The user explicitly prefers a very dark palette with **no blue as a primary/accent color**. Both websites should use a near-black-but-not-black base (graphite / charcoal / iron), with restrained non-blue accents. The existing brochure's iron + amber/green direction is closer to the target than the current product application's GitHub-blue palette.

## Reference workspaces

Use these as **read-only references** unless a later instruction explicitly says otherwise:

- Warden application: editor/explorer/agent UX quality bar.
- Warden docs site: collapsible documentation navigation patterns.
- Trestle project/application and Trestle docs site: dense developer-product information architecture and documentation navigation.
- Cortex / Gantry projects: auth/account/product-shell consistency where useful.
- Nift / Strut sites: editorial hierarchy, landing-page quality, documentation structure, polish.

Do **not** modify Warden, Trestle, Cortex, Nift, Strut, gantry-core or other attached reference projects during this campaign.

## Competitive reference set

Study current GitHub, GitLab and Bitbucket behavior throughout the campaign. Borrow useful, familiar patterns rather than copying visual design literally.

Particularly inspect:

- user profiles and settings;
- organization/group/workspace profiles and settings;
- owner/repository URL conventions;
- repository Overview/Code/Issues/PRs/Actions-or-CI/Settings information architecture;
- profile and organization READMEs;
- repository README rendering;
- avatars;
- pinned/featured repositories;
- contribution/activity presentation;
- branch/tag/history browsing;
- rendered source and Markdown;
- syntax highlighting;
- web editor behavior;
- diff/review interfaces;
- permissions and collaboration settings;
- repository danger-zone actions;
- tokens/SSH keys/session/account security surfaces;
- responsive/mobile behavior;
- empty/loading/error states.

Familiarity is a feature. A user who knows GitHub/GitLab/Bitbucket should understand the simple Switchyard case immediately.

Switchyard-specific concepts should appear only where they add something real:

- Work;
- Attempts;
- Agents;
- Review Findings;
- semantic / intent / policy conflicts;
- Provenance;
- Integration Queue;
- Needs Attention;
- durable programmable Workflows.

---

# Campaign invariants

## Repository safety

The attached repository history is valuable.

Never:

- delete or replace `.git`;
- use destructive cleanup such as `rm -rf` on project repositories;
- use `git clean -fdx`;
- rewrite history;
- rebase/squash/amend checkpoint history;
- force-push.

Use normal follow-up commits. Commit at the end of every accepted checkpoint.

## Product invariants

1. Git stays Git.
2. Human-native Git workflows must remain excellent with **zero Agents**.
3. Advanced agent machinery appears progressively.
4. `<title>Switchyard</title>` remains site-wide for the authenticated product.
5. The real Switchyard mark/favicon is used; no generic gradient `S` badge.
6. Dark-first, near-black/graphite palette; no blue primary/accent palette.
7. Mobile is a first-class surface, not desktop squeezed smaller.
8. The desktop menu may also be hamburger-driven; when used it opens a proper full-page navigation surface.
9. Full-page navigation and docs navigation use **collapsible groups/headings** like the better Nift/Trestle/Warden documentation navigation patterns.
10. User-visible terminology must distinguish what is Git-standard from what is Switchyard-specific.
11. An API endpoint is not evidence of finished UX.
12. Every important state needs populated, empty, loading, error and permission-denied behavior.
13. Every visual checkpoint must be tested in a real browser, at desktop and mobile widths.
14. Preserve keyboard/focus accessibility throughout.

## Shared code-surface invariant

Do not separately invent four unrelated rendering systems for:

- repository source view;
- browser editor;
- diffs;
- review/findings snippets.

Create one coherent **language detection + syntax/token styling + code presentation layer** where practical, with context-specific rendering on top.

The same file should look recognizably consistent in repository view, editor, diff and review.

## README/Markdown security invariant

Rendered Markdown may include links and images, but must never become an arbitrary HTML/script execution path.

Define and test:

- Markdown dialect;
- sanitization;
- relative links;
- relative repository images;
- external images;
- image alt text;
- anchors/table-of-contents where useful;
- large-file limits;
- binary/non-text handling.

---

# Current-state findings to preserve in the campaign record

These are observations from the supplied workspace, not assumptions about future work.

## Identity/account state

Current user records are effectively:

```text
username
password_hash
display_name
```

There is authentication/session support, but no credible GitHub-style user profile product yet.

Missing or not presently surfaced as first-class product UX include:

- avatar;
- bio;
- location;
- website/social links;
- public profile page;
- profile README;
- pinned repositories;
- contribution/activity presentation;
- user settings center;
- account/security settings center;
- personal token / SSH-key UX;
- session management UI;
- notification preferences.

## Organization state

Organizations currently exist primarily as coordination/policy records and fleet grouping. That proves useful architecture but is not yet a GitHub-like organization product.

Missing or not presently surfaced as first-class product UX include:

- organization profile page;
- large organization avatar;
- description/location/website/contact metadata;
- organization README;
- pinned repositories;
- members page;
- teams/groups;
- invitations;
- owner/member/admin role UX;
- organization settings center;
- organization repositories view;
- member-visible vs public profile content;
- organization credential/security UX.

## Repository presentation state

The repository viewer still fundamentally renders source in a plain `<pre>` surface. It does not yet have the quality expected from modern Git hosting.

Missing or weak areas include:

- syntax-highlighted source browsing;
- Markdown/README rendering;
- image preview;
- binary-file treatment;
- branch/tag switcher;
- commit history;
- per-file history/blame;
- breadcrumb/folder navigation quality;
- file metadata/actions;
- download/raw/copy-path UX;
- repo metadata/about/sidebar;
- repository settings;
- collaborators/permissions;
- default branch/visibility/archive/delete/rename/transfer UX;
- webhooks/integrations settings where appropriate;
- protected branch/ref policy UX;
- clone surface/credentials UX;
- releases/tags surface (later if needed for v1).

## Editor state

The current editor has important correctness work already proven:

- CodeMirror 6 renders;
- line numbers/history/basic editor behavior;
- durable draft;
- Save != Commit;
- draft revision/CAS;
- stale-base handling;
- diff;
- findings;
- hideable Agent panel;
- formal Attempt Agent execution.

However, its current CM6 setup is minimal and does not yet provide the expected multi-language syntax experience or Warden-quality workbench ergonomics.

Warden demonstrates useful patterns worth extracting conceptually:

- integrated workspace/file tree;
- tabs;
- Files/Search/Source modes;
- resizable sidebar;
- status bar with language + cursor position;
- create/open/save actions;
- integrated hideable Agent panel;
- deliberate dense desktop workbench;
- responsive behavior.

Switchyard should take the useful editor/explorer core, **not** Warden's terminal/server-management scope.

## Palette state

The brochure has moved toward charcoal/iron with amber/green accents.

The product application still has a GitHub-derived blue accent (`#58a6ff`) and should be brought into the Switchyard visual system.

---

# Checkpoint execution rules

Each checkpoint must end with:

```text
Checkpoint
Status: PASS / CONDITIONAL PASS / FAIL
Objective
What changed
What deliberately did not change
Design decisions
Data/API decisions
Desktop evidence
Mobile evidence
Accessibility evidence
Security implications
Regression evidence
Known gaps
Impact on later checkpoints
Commit SHA
```

For browser-facing checkpoints, capture screenshots at minimum:

```text
1440x900 desktop
1280x800 desktop
390x844 mobile
```

Use real Chromium/Playwright or an equivalent browser gate.

Do not proceed to the next checkpoint while the current checkpoint has known blocking regressions.

---

# Phase 0 — Product model and design foundations

## PX0 — Baseline audit + screenshot corpus

**Objective:** establish a reproducible before-state and exact gap inventory before redesigning anything.

Tasks:

- run the current product and brochure locally;
- capture every currently reachable major product surface at desktop + mobile widths;
- capture Warden editor/explorer/agent reference surfaces;
- capture representative Trestle/Warden/Nift/Strut brochure/docs surfaces;
- produce `docs/plan/product-ux-baseline.md`;
- inventory current routes/API endpoints/data models;
- map API capability vs actual user-facing product surface;
- inventory all current demo repos and preserve the curated demo set separately from checkpoint debris;
- identify all current blue palette tokens and hard-coded colors;
- run an accessibility smoke baseline;
- record performance/load baseline for the main product pages.

**Gate:** a future reviewer can reproduce the current UX from the repo and knows exactly which capabilities exist only as APIs.

---

## PX1 — Information architecture + URL/domain model

**Objective:** lock the simple Git-host mental model before visual implementation.

Design the main authenticated IA around familiar concepts, likely along the lines of:

```text
Dashboard
Repositories
Organizations
Work
Pull Requests
Workflows
Needs Attention
```

Within a repository, likely:

```text
Code
Work / Issues relationship
Pull Requests
Workflows
Activity / Insights as justified
Settings
```

Keep Attempts, Findings, Provenance, Queue state and Agent execution contextual unless there is a proven reason for first-class global navigation.

Also design URL conventions such as:

```text
/{owner}
/{owner}/{repo}
/{owner}/{repo}/tree/{ref}/{path...}
/{owner}/{repo}/blob/{ref}/{path...}
/{owner}/{repo}/commits/{ref}
/{owner}/{repo}/pull/{number-or-id}
/{owner}/{repo}/settings/...
/settings/...
/organizations/{org}/settings/...   (or equivalent)
```

Do not blindly copy GitHub route syntax if a cleaner equivalent is preferable, but preserve obvious owner/repo semantics.

**Gate:** route map + navigation model reviewed against GitHub/GitLab/Bitbucket and Switchyard-specific flows.

---

## PX2 — Owner/repository namespace + migration design

**Objective:** move from flat repository naming to a durable ownership model suitable for user/org Git hosting.

Current Artifacts repos are effectively flat names. The product wants:

```text
user/repo
org/repo
```

Design and implement a Switchyard repository metadata layer containing at minimum:

```text
repository id
owner type (user/org)
owner id
slug/name
display name
description
visibility
default branch
underlying Artifacts repository identifier
created/updated timestamps
```

Requirements:

- do not rename or destroy existing Artifacts repos blindly;
- existing curated demo/checkpoint repositories remain reachable during migration;
- ownership changes/transfers can be represented later without corrupting Git truth;
- route identity is separate from physical backend repository naming where necessary;
- unique `{owner, slug}` enforced;
- collision/migration behavior documented;
- provenance continues to reference stable repository identity.

**Gate:** one user-owned and one org-owned demo repo work end-to-end through normal Git and web browsing using owner/repo URLs.

---

## PX3 — Unified Switchyard design system

**Objective:** establish one high-quality visual language for brochure + application without forcing both to have identical layouts.

Palette requirements:

- main backgrounds close to black but visibly charcoal/graphite, not `#000`;
- **no blue primary/accent palette**;
- use iron/graphite/stone neutrals;
- signal amber / restrained green / muted red are acceptable functional accents;
- avoid sherbet colors, glassmorphism and generic AI gradients;
- code surfaces may be slightly darker than the shell;
- status colors must remain accessible and not become the brand itself.

Define shared tokens for:

```text
backgrounds
panels/elevation
borders
type hierarchy
muted text
links/focus
success/warning/danger
code/diff syntax
spacing
radius
shadow (minimal)
```

Use the real Switchyard mark/favicon.

**Gate:** both sites render the new system consistently at desktop/mobile and no main interactive state depends on GitHub-blue remnants.

---

## PX4 — Navigation systems

**Objective:** implement deliberate navigation rather than link accumulation.

### Brochure/docs

Use sparse header + hamburger on desktop and mobile. Opening it reveals a full-page menu.

The full-page menu must have **collapsible group headings**, inspired by the Nift/Trestle/Warden documentation navigation quality bar.

Example groups:

```text
Product
Concepts
Documentation
Project
```

Within long documentation pages, provide a dedicated collapsible docs navigation/sidebar/drawer with active-page state.

### Product

Use a Git-host-like product shell with:

- global navigation;
- owner/repo context;
- profile/avatar menu;
- responsive hamburger/full-screen navigation where appropriate;
- repository-local tab navigation;
- contextual settings rather than one giant settings page.

**Gate:** keyboard-accessible open/close, Escape, focus management, active states, mobile body-scroll handling, nested/collapsible groups and no hidden overlay intercept bugs.

---

# Phase 1 — Shared code and content presentation

## PX5 — Language detection + shared syntax highlighting foundation

**Objective:** make code look like code everywhere.

Build one supported language registry/detection layer driven primarily by file extension/name with graceful plaintext fallback.

Initial useful language set should include at least:

```text
Go
C / C++
Rust
JavaScript / TypeScript
JSX / TSX
HTML
CSS
JSON
YAML
TOML
Markdown
Shell
Python
SQL
Nift templates / Nift script where feasible
Strut
```

Decide whether CodeMirror 6 language packages can supply both editor language support and tokens for read-only source surfaces, or whether a complementary renderer is cleaner.

Requirements:

- no runtime CDN dependency;
- lazy-load or split language support so bundle size stays sane;
- consistent token palette across viewer/editor/diff where practical;
- language shown in status/file metadata;
- unknown files fall back safely.

**Gate:** representative fixture repository renders correct language detection and highlighting across all major supported languages.

---

## PX6 — Repository source viewer 2.0

**Objective:** replace the plain `<pre>` viewer with a real Git-host source browser.

Add:

- syntax highlighting from PX5;
- line numbers;
- line anchors/permalinks;
- copy file / copy path / raw / download actions;
- breadcrumbs;
- file/folder icons kept restrained;
- branch/ref selector;
- file size/type metadata;
- binary/image handling;
- rendered Markdown toggle where appropriate;
- edit button routed into the editor;
- file history entry point;
- responsive source scrolling;
- keyboard usability.

Do not overdesign it into an IDE. This is the normal repository browsing surface.

**Gate:** demo-basic and Switchyard-self repositories are pleasant to browse as ordinary Git repos with zero agent context.

---

## PX7 — Markdown/README/image renderer

**Objective:** make README-driven repository/profile presentation first-class.

Implement sanitized Markdown with:

- headings;
- lists;
- tables;
- fenced code with highlighting;
- links;
- images;
- relative repository paths;
- relative images resolved against the repository/ref/path;
- external images with explicit safe policy;
- heading anchors / TOC where appropriate;
- blockquotes/task lists if the chosen dialect supports them;
- useful error states for missing image/file references.

Surface repository README automatically on repository overview/code pages using documented precedence (choose and document a convention such as root first, then `.github/`, then `docs/`, or another explicit policy).

This renderer will later power user/org profile READMEs.

**Gate:** a README containing multiple relative images, code blocks and internal links renders correctly and safely.

---

## PX8 — Editor/workbench quality pass

**Objective:** bring the editor toward Warden quality while staying Switchyard-specific.

Target desktop composition:

```text
Files / Search / Source
        |
        |  Tabs
        |  Editor
        |  status bar
        |
        + optional hideable Agent panel
```

No terminal.

Implement or assess:

- integrated repository file tree;
- multiple tabs;
- syntax language packs from PX5;
- dirty state per tab;
- Save Draft vs Commit semantics;
- draft revision/stale state surfaced non-intrusively;
- create/rename/delete file UX;
- repository search;
- optional source-control/diff view;
- status bar (language, Ln/Col, branch/ref);
- keyboard shortcuts;
- resizable sidebar + Agent panel on desktop;
- Files | Editor | Agent mode approach on narrow mobile layouts;
- findings markers/gutters;
- editor errors that do not rely on `alert()` as the final UX.

Preserve existing CAS correctness.

**Gate:** editing a multi-file demo repo feels materially comparable in care to Warden, while retaining Switchyard's Save != Commit model.

---

## PX9 — Interactive Agent-to-draft UX

**Objective:** complete the editor mode originally intended for human + Agent collaboration.

Current formal Attempt Agent mode may commit autonomously. Preserve that path.

Add distinct interactive assistance:

```text
human opens file/draft at revision R
→ asks Agent
→ Agent produces a proposed patch/draft update based on R
→ revision/CAS protects newer human edits
→ user sees changed files + diff
→ user accepts/rejects/edits
→ user chooses Commit
```

Requirements:

- Agent never silently overwrites a newer human revision;
- no implicit commit in interactive mode;
- multiple-file proposals represented coherently;
- provenance records Agent assistance without falsely attributing final human commit entirely to the Agent;
- user can promote/take over into a formal Attempt if work becomes autonomous.

**Gate:** race-tested human vs Agent draft updates with no last-writer-wins.

---

# Phase 2 — Users, organizations and identity

## PX10 — User profile data model + avatar pipeline

**Objective:** turn authenticated accounts into real Git-host identities.

Add user profile metadata, likely including:

```text
username (stable/rename policy explicitly designed)
display name
avatar
bio
location
website
optional social links
created_at
```

Avatar requirements:

- upload/change/remove;
- sensible file type/size limits;
- server-side validation;
- stable generated fallback/initial avatar when absent;
- thumbnail + full profile sizes;
- no remote-image SSRF path if URL avatars are considered;
- large profile avatar comparable to GitHub's profile emphasis, not a tiny nav icon only.

Use a storage abstraction suitable for later hosted deployment. Do not shove image blobs into arbitrary Git commits unless the design explicitly calls for that.

**Gate:** avatars render correctly in header menus, lists and large profile view.

---

## PX11 — User profile page + profile README + activity

**Objective:** provide a credible public/internal user profile.

Profile page should include:

- large avatar;
- display name + username;
- bio/location/site metadata;
- repositories;
- organizations;
- pinned/featured repositories;
- recent contribution/activity summary where data is actually available;
- profile README rendered through PX7;
- sensible private/public behavior.

Profile README design should use a documented convention. GitHub-style username repository is familiar; GitLab-style profile README also provides precedent. Choose one deliberately and make images/relative links work.

Do not fabricate a contribution heatmap until Switchyard has meaningful event data to support it. A simpler honest activity list is preferable to fake density.

**Gate:** two demo users with distinct avatars/READMEs/repositories render convincingly.

---

## PX12 — User settings center

**Objective:** build a real account/settings UX.

At minimum design sections for:

```text
Profile
Account
Appearance
Notifications (if supported / otherwise clearly deferred)
Emails (if supported / otherwise deferred)
Password / authentication
Sessions
Personal access / provider credentials
SSH keys or Git credentials (if supported by backend)
Agent providers / Agent roles
Danger zone
```

Settings must distinguish:

- external AI-provider credential;
- Switchyard repo permissions;
- Git credentials/tokens;
- account authentication.

Do not blur them into one secret-management page.

**Gate:** settings are routable, permission-safe, mobile-usable, and destructive actions have confirmation/audit behavior.

---

## PX13 — Organization model expansion + avatars/profile

**Objective:** turn policy/fleet org records into first-class Git hosting organizations.

Expand organization data model as needed:

```text
slug/name
display name
avatar
description
location
website/contact
owners/members
visibility/public-profile settings
created_at
```

Build org profile:

- large avatar;
- description/metadata;
- repositories;
- pinned/featured repos;
- members (respecting privacy/settings);
- organization README rendered through PX7;
- public/member view if justified;
- links to Work/PR/fleet/policy only when appropriate.

**Gate:** curated Switchyard Labs demo org owns multiple repos and has a polished profile.

---

## PX14 — Organization members, teams and invitations

**Objective:** add collaboration semantics closer to real Git hosting.

Design and implement the v1 subset of:

- invite user;
- accept/decline invitation;
- owner/member roles;
- remove member;
- team creation/membership if justified;
- repository access assignment;
- audit trail;
- role enforcement at API choke points.

Do not claim enterprise authorization until permission matrices are actually enforced.

**Gate:** unauthorized users cannot see/do what the UI claims they cannot; server enforcement, not CSS hiding.

---

## PX15 — Organization settings center

**Objective:** provide coherent org administration.

Sections to evaluate:

```text
General/profile
Members
Teams
Repositories
Policies
Agent/provider configuration references
Audit
Security
Danger zone
```

Switchyard policy/risk/fleet capabilities should live naturally here instead of as disconnected APIs.

**Gate:** org owner can manage profile/member/policy basics and a normal member cannot access owner-only settings.

---

# Phase 3 — Repository and collaboration parity

## PX16 — Repository overview + metadata + clone UX

**Objective:** make a repository landing page feel immediately familiar.

Add:

- owner / repo identity;
- visibility badge;
- description/topics if implemented;
- clone controls;
- default branch;
- branch/tag counts;
- commits/latest commit summary;
- file tree;
- README;
- about/sidebar or equivalent;
- Work/PR status summaries without dashboard overload;
- fork/source relationship if the data exists;
- star/watch/follow only if there is a meaningful product requirement — do not cargo-cult social counters.

**Gate:** a GitHub/GitLab user can land on `demo-basic` and navigate without instruction.

---

## PX17 — Commit / branch / tag history surfaces

**Objective:** expose ordinary Git history before agent-native metadata.

Build:

- commits list;
- commit detail;
- changed files/diff;
- branch selector/list;
- tags list if supported;
- compare refs;
- file history;
- optional blame deferred unless it can be done well.

Then layer provenance/Agent attribution onto these normal Git objects rather than replacing them.

**Gate:** history of the curated demos can be understood without going to CLI Git.

---

## PX18 — Pull Request product surface

**Objective:** make PRs a first-class familiar collaboration object.

Build/rework:

- PR list tabs/filter/status;
- PR header/state/base/head;
- conversation/timeline;
- commits;
- files changed;
- checks;
- reviews;
- Review Findings;
- conflict/semantic-conflict state;
- queue/integration state;
- merge/integrate controls based on policy;
- draft PR state if part of domain;
- links back to Work and Attempts without making them mandatory reading.

Agent-native extra data should be collapsible/contextual:

```text
Attempts
Evidence/artifacts
Provenance
Agent review/repair
Queue status
```

**Gate:** simple human PR looks simple; complex multi-Agent PR exposes advanced machinery progressively.

---

## PX19 — Work / Issues relationship

**Objective:** resolve the current ambiguity between conventional Issues and Switchyard Work.

Test product designs such as:

- `Work` as a generalized issue/task model with familiar issue-style views;
- conventional `Issues` as one subtype or facade over Work;
- separate Issues only if this materially improves human Git-host familiarity.

Requirements:

- title/body/status;
- author/assignees where supported;
- labels/milestones only if justified;
- links to Attempts/PRs;
- comments/activity if supported;
- simple bug/feature use without understanding Agents.

**Gate:** a human can use Switchyard as a normal issue/task tracker with zero agent involvement.

---

## PX20 — Repository settings

**Objective:** create the missing Git-host administration surface.

Design sections similar in spirit to established Git hosts:

```text
General
Collaborators / access
Branches / protected refs
Merge / Integration Queue policy
Work / Pull Request options
Webhooks / integrations (if implemented)
Agent / workflow configuration
Secrets / variables references (carefully scoped)
Audit/activity
Danger zone
```

General settings should evaluate:

- rename;
- description;
- visibility;
- default branch;
- archive;
- transfer owner;
- delete;
- fork policy where relevant.

Every destructive action requires server-side authorization and explicit confirmation.

**Gate:** repo owner/admin can perform v1-safe settings changes and unauthorized members cannot.

---

## PX21 — Workflow / Agent / Needs Attention product surfaces

**Objective:** turn proven backend capabilities into polished daily-use UI.

Build coherent product pages for:

- workflows + runs + steps + replay/retry/cancel;
- Agent roles + executions;
- provider credential metadata/rotation;
- Integration Queue;
- Needs Attention;
- escalation decision packets;
- org fleet/policy/audit where appropriate.

Do not make the global shell feel like an AI dashboard. These are advanced operational views.

**Gate:** demo-agent/demo-workflow/demo-conflict states are inspectable without raw API calls.

---

# Phase 4 — Public site, demo and product cohesion

## PX22 — Brochure/editorial redesign completion

**Objective:** make the public site a first-class project website, not formatted handover notes.

Requirements:

- near-black graphite palette shared in spirit with product;
- no blue primary accent;
- real Switchyard mark;
- sparse top bar;
- desktop/mobile hamburger;
- **collapsible full-page menu groups**;
- collapsible docs nav groups;
- strong typography/rhythm/whitespace;
- one clear purpose per page;
- more visual explanation, less wall-of-text;
- proper architecture diagrams;
- real product screenshots captured only after major product UI checkpoints;
- no checkpoint-campaign trivia in main marketing narrative;
- `@path()` for all Nift-tracked internal links/assets;
- site-wide link/image validation;
- `<title>Switchyard</title>` if retaining the current preference.

Do not merely polish the current screenshot cards. Replace screenshots as the product improves.

**Gate:** direct side-by-side review against Nift/Trestle/Strut/Warden sites no longer shows Switchyard as the weakest project visually/editorially.

---

## PX23 — Public demo environment

**Objective:** make the actual product safely viewable without SSH tunnelling.

Design a dedicated demo/staging deployment with:

- HTTPS;
- no exposed Trestle admin surface;
- no reusable private infrastructure credentials in browser/session;
- curated users/orgs/repos;
- rate/budget limits;
- reset strategy;
- safe public read vs authenticated write policy;
- abuse controls;
- demo Agent credentials isolated from personal credentials;
- obvious demo status banner if necessary.

Reuse the curated repos:

```text
demo-basic
demo-agents
demo-conflict
demo-semantic
demo-workflow
```

Rename/consolidate only if it improves the experience and does not destroy useful evidence.

Do not buy domains or provision new paid resources without approval.

**Gate:** user can open the demo URL from a clean browser and explore the product safely.

---

## PX24 — End-to-end responsive + accessibility campaign

**Objective:** product-wide polish across real flows.

Browser matrix as practical:

```text
Chromium
Firefox
WebKit if available
```

Check:

- keyboard navigation;
- visible focus;
- dialogs/drawers/menus;
- Esc close;
- focus trap/return;
- screen-reader labels/landmarks;
- contrast;
- reduced motion;
- table/code overflow;
- touch targets;
- mobile source viewer;
- mobile editor modes;
- long usernames/repo names;
- large README/images;
- empty repositories;
- huge directory/file names;
- auth expired states;
- network errors.

**Gate:** no critical accessibility/responsive defects across the curated demo journey.

---

## PX25 — Visual/product certification + handover

**Objective:** independently decide whether the product is ready to show publicly before returning to CP14/release hardening.

Produce:

- complete screenshot corpus;
- before/after comparison;
- route coverage matrix;
- GitHub/GitLab/Bitbucket parity matrix (Implemented / Switchyard equivalent / Deliberately deferred / Not applicable);
- Warden editor-quality comparison;
- current known UX bugs;
- technical debt from this campaign;
- performance/bundle changes;
- security implications;
- migration notes;
- final recommended CP14 ordering.

Explicitly answer:

> Would we be comfortable sending a developer to the demo without explaining the interface first?

and:

> Does Switchyard look and behave like a serious Git collaboration product before its Agent-native features are even considered?

**Gate:** campaign accepted or specific remaining blockers recorded. Do not silently call partial parity complete.

---

# Git-host parity backlog (evaluate; not every item is automatically v1)

The checkpoint campaign should classify each item as **v1**, **later**, or **not useful** rather than cargo-culting competitors.

## Account / profile

- avatar;
- profile README;
- pinned repos;
- activity;
- followers/following (probably not important unless justified);
- status;
- profile privacy;
- email/privacy;
- sessions;
- PATs;
- SSH keys;
- appearance;
- notification settings.

## Organization

- avatar/profile README;
- pinned repos;
- members;
- teams;
- invitations;
- owners/admins;
- repo roles;
- org policies;
- audit;
- secrets/provider profiles;
- verified domain/SSO later if enterprise direction warrants it.

## Repository

- owner/repo namespace;
- visibility;
- README;
- license/code-of-conduct/contributing recognition later;
- branches/tags;
- commit history;
- compare;
- PRs;
- Work/Issues;
- collaborators;
- protected refs;
- webhooks;
- workflows;
- release/tags UX;
- archive/transfer/delete;
- fork relationships;
- repository image/file previews;
- search.

## Code review

- syntax-highlighted diffs;
- side-by-side/unified where worthwhile;
- comments/findings;
- review states;
- checks;
- conflicts;
- semantic conflict;
- suggested fixes / Agent repair;
- provenance;
- queue state.

## Search / navigation

- global repo/user/org search;
- repository file search;
- command palette later if useful;
- keyboard shortcuts;
- recent repos/work;
- breadcrumbs.

---

# Suggested implementation order rationale

The order above is intentional:

1. **model + routing before paint** — owner/repo namespaces affect everything;
2. **design system/navigation before pages multiply**;
3. **shared syntax/Markdown before profiles/repos/editor each invent their own renderer**;
4. **identity before organization/repository settings**;
5. **ordinary Git-host parity before exposing agent-heavy operational surfaces**;
6. **brochure screenshots after the product actually looks good**;
7. **public demo after auth/permissions/settings are credible**;
8. **cross-browser/accessibility certification last, then feed findings into CP14**.

---

# First checkpoint to execute

Begin with **PX0 only**.

Do not start PX1 until PX0's baseline audit and screenshot corpus are complete and committed.

At the end of PX0, return:

```text
1. Product route/API/UX capability matrix
2. Brochure route/content matrix
3. Current user/org/repo model gaps
4. Current editor/viewer comparison against Warden
5. GitHub/GitLab/Bitbucket feature shortlist classified as v1/later/not-useful
6. Desktop/mobile screenshot paths
7. Accessibility baseline
8. Bundle/page performance baseline
9. Recommended adjustments to PX1–PX25 based on evidence
10. Commit SHA + clean git status
```

Do not begin the implementation-heavy checkpoints merely because the direction seems obvious. The point of PX0 is to make the rest of the campaign evidence-driven.
