# C42 — final canonical regression, DONE with tooling limits

October 4 canonical editor gap closed: repository Edit links and file navigation use
`/alice/railway/edit/editor-c20/src/routes.mjs`. Existing `edit.html?name=...`
bookmarks resolve through registered identity and the same public/private repository
boundary, validate file paths, and preserve Attempt context. API mutations continue
using the authorized physical repository identity internally; this grants no new access.

Real authenticated Chromium: direct routes.mjs link loaded, explorer network.json
selection changed the URL to `/alice/railway/edit/editor-c20/src/network.json`, and
refresh was exercised. `scripts/cua-canonical-navigation.mjs` includes a permanent
editor regression helper. Server tests cover shell selection, escaped paths, Attempt
query retention and invalid traversal/owner rejection. Final whole-surface/multiwidth
and public/org route recertification are still outstanding.

Final profile matrix at 1600/1280/1024/768/430/390 × 900 passed after visual QA
caught a mobile repository-card overlap missed by the earlier document-size test.
Removed the grid height cap without adding a nested repository scrollbar. Narrow
overview content uses one panel scrollbar, with activity rows sharing that panel;
desktop activity retains its own bounded card. A permanent overlap assertion now
checks every repository bottom against the contribution card top. Screenshots and
`profile-matrix.json` record matching 38×38 controls, named org links, 365 unique
dates and fixed document bounds. Firefox and WebKit are UNAVAILABLE through the
connected CUA surfaces (only Chromium IAB and MCP Apps were exposed); no binaries
were installed to manufacture coverage. Whole-surface certification remains open.

Latest screenshot follow-up: repository root content no longer scrolls across
files and README; README body owns its auto-scroll, with its heading fixed. A
very large file listing may independently scroll within its bounded region.
Outside popover dismissal passed all six widths; the click listener now uses
capture so another control stopping propagation cannot prevent closing it.
Avatar/menu both remain 38×38 with no account text. Single-select chevrons have
11px right inset and 34px text padding throughout the shared component system.
Loading/error paragraphs in commit/PR/Work lists now share a 16px inset.
`readme-card-scroll.png` records the actual updated repository. The local brochure
DOM confirmed distinct dashboard.jpg and repo.jpg homepage images after Nift build.

File-page breadcrumb now precedes both columns. Real Chromium file route measured
Files and viewer card tops both at 380.96875px; `aligned-file-cards.png` records it.

Latest continuation: the repository browser no longer mounts the shared duplicate
owner/repository context row. Its own avatar/owner link and linked repository
heading provide the identity once. Deep source breadcrumbs contain selected ref
and directory/file only; root pages omit the path breadcrumb. Actual Chromium
source proof: zero duplicate context rows, owner /alice, repo /alice/railway,
path main/src/routes.mjs and five real source lines. The maintained canonical
navigation helper now checks these invariants. See single-repository-heading.png.

The avatar-only global header passed 54 actual Chromium cases: dashboard, profile,
repository, source, editor, Actions, repository settings, PR list and Work at
1600/1280/1024/768/430/390. Each checks equal 38px controls, no account text,
zero inner image border, image filling the control, and no document overflow.
This is header geometry coverage, not certification of all loaded body states.
Evidence: header-54-cases.json; maintained helper scripts/cua-header-matrix.mjs.


Final deployed binary 84552b7 passed 114 Chromium cases: all 16 requested
surfaces plus release detail/new form and menu, at 1600/1280/1024/768/430/390.
`final-84552b7/matrix.json` and screenshots verify loaded content, actual viewport,
product shell, no horizontal/vertical document overflow and no dead placeholder
links. Canonical navigation records commit deep-link refresh, back/forward,
branches/tags and repository-scoped Work. Editor selection/refresh and single
repository identity are retained separately. Final Code popover passes bounds,
keyboard modes, Escape/focus return, toggle and outside dismissal at six widths.

One initial branch load returned a visible error; a reload loaded the real refs.
A source selector wait also timed out while content was still fetching, then
fully loaded without reload. Backend selector waits are shorter than requested;
readiness now allows bounded retries unless a visible alert appears. These
observations are retained as reliability limits, not hidden by the passing layout
captures. Screenshot checks are not a user-network latency benchmark.

Browser credential minting remains C33 and awaits action-time approval. Archive
exact-content proof is C45; final embedded runtime/download is C43. Firefox and
WebKit remain UNAVAILABLE through the connected browser tooling.
