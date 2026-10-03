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
