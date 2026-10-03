# C28 canonical navigation follow-up

Added `/{owner}/{repo}/branches`, `/tags`, `/work`, and `/commit/{sha}` to
the existing repository, tree/blob, history, PR, Actions and settings routes.
Repository Work links use the nested route and preserve the repository filter.
History rows are copyable commit links, while ordinary clicks update details
and browser history. Refresh resolves the commit from the URL. Legacy
`/profile.html?user={owner}` bookmarks redirect to the owner route; existing
repository bookmarks retain authorization-aware identity resolution.

`canonical-navigation.json` records the maintained Chromium test against the
disposable fixture: branches, empty tags, filtered Work, commit refresh and
back/forward, and no captured warning/error console entries. Go tests, vet and
diff checks passed. This is a focused addition to C22, not Firefox/WebKit
certification. Broader unauthenticated, owner/organization and all-route
regression coverage remains C42; editor URLs still use legacy query parameters.
