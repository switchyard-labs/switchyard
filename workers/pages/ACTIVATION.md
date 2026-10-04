# Native Pages activation (conditional)

Local product tests do not require these steps. Nothing in this file authorizes
resource creation, DNS changes or deployment.

1. Purchase and configure `switchyard.cx` after owner approval. Confirm the zone
   and credentials on the intended Cloudflare account.
2. Confirm the existing Actions artifact bucket is the intended production
   bucket. `wrangler.jsonc` currently binds the existing campaign bucket;
   update the binding deliberately rather than creating another bucket.
3. Deploy the completed Actions Worker and app schema 11 together, with the
   existing signed control protocol and approved repository allowlist. Static
   build bundles and ready deployment receipts share its existing bucket.
4. Deploy the read-only serving Worker with its `PAGES_BUCKET` binding. After
   domain setup, add the wildcard route `*.switchyard.cx/*` in the approved zone.
   Verify wildcard DNS and edge TLS with owner and `dpl-<32 hex>` preview hosts.
5. Keep the application on a distinct host, such as `app.switchyard.cx`. App
   cookies must remain host-only (no parent-domain `Domain` attribute).
6. Smoke-test owner root, dotted/hyphenated/underscore repository prefixes,
   exact-SHA preview, atomic promotion, 404 scope and rollback over HTTPS.
   Then enable native public URLs in product messaging.

Project sites share the owner origin (scripts, cookies, localStorage and
IndexedDB). Owners must trust published projects. Project service workers have
normal script-directory scope; no `Service-Worker-Allowed: /` response is added.
The owner root is trusted same-owner content. CSP is authored site policy;
Pages does not promise per-project browser sandboxing.

## Build paths and limits

Choose any authorized backing repository explicitly, including for the owner
root. Plain branch names and qualified `refs/heads/...` or `refs/tags/...` are
supported; annotated tags resolve to the peeled commit. Raw SHA configuration
is not supported. Deployments always save the resolved SHA.

Builds receive `SWITCHYARD_PAGES_BASE_PATH` (`/` or `/<repo>/`) throughout the
build command. Configure the framework/Nift templates to use this prefix for
root-relative assets and client routing. Relative links naturally remain under
the current page path. No HTML rewriting occurs. Missing project content never
falls back to an owner-root page. SPA fallback is explicitly configured in Pages settings or with `sy pages
configure --spa-fallback index.html`; the approved Actions output carries it.
It is not inferred from a 404.

Each Actions static bundle is limited to 64 MiB encoded / 48 MiB total payload,
10,000 files, directory depth 32 and path length 1,024. Output directories must
be relative and remain beneath the build workspace. Symlinks and nonregular
files are rejected. The standalone exporter has a configurable maximum of
256 MiB, but Actions uses the tighter bounded bundle limit.

Disabling new builds does not unpublish current production. Mount reservations
are permanent; renames require an explicit future mapping migration. All
artifacts/previews are conservatively retained; there is no automated deletion
that could remove production or rollback candidates. Custom domains are later
scope.
