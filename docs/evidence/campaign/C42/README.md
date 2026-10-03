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
