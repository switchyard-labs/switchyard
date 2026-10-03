# C42 — final canonical regression, in progress

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
