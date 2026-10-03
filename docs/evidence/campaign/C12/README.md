# C12 Actions UI evidence

The screenshots show the actual Go application at a canonical repository route, backed by a disposable local SQLite Trestle instance and the signed Cloudflare Actions provider. The populated run was imported by the real reconciler from the native Artifacts push proof. The empty fixture is a real disposable Artifacts repository; its run list is empty.

- `run-detail-1280.jpg`: status, source SHA, actor, event, timing, jobs and collapsed steps.
- `run-list-390.jpg`: usable narrow layout, native filters and exact commit identity; measured document width 375 <= viewport 390.
- `empty-1280.jpg`: repository with no runs and a meaningful configuration entry point.
- `browser-smoke.json`: permanent staged browser assertions from `scripts/cua-actions-smoke-stages.mjs`, executed with the documented cua_repl API.

The initial browser pass caught relative Nift asset URLs loading HTML fallbacks as scripts on nested canonical routes. Shared template asset/navigation paths now resolve from the application root; `TestCanonicalActionsPageAssets` tests actual route/asset responses. A local-service session expiry produced a temporary access error during fixture setup; the isolated services were restarted with owned process IDs recorded for cleanup. Raw provider error bodies are not displayed in the Actions UI.

C12 is the list/detail surface. Captured output viewer is C13, functional dispatch/rerun/cancel and deploy-provider availability are C14. Those controls are disabled with an explanation until implemented. No Worker Preview/deployment URL is fabricated, and downloadable build artifacts are not claimed available at this checkpoint. The full six-width product-wide browser gate is C22, still pending.
