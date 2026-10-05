# Native Pages activation

Live infrastructure and end-to-end certification completed on 2026-10-05.
See [the live handover](../../docs/plan/proposals-pages-live-handover.md) and
[recorded evidence](../../docs/evidence/proposals-pages/live/).

- Cloudflare account: `b7f20353ee8a9e5d2003f52c74ba795e`.
- Active zone: `switchyard.cx` (`abf2755072006add6ff1e59342ab5114`).
- Nameservers: `linda.ns.cloudflare.com`, `noel.ns.cloudflare.com`.
- Apex A: `45.79.189.46`, DNS-only. Caddy terminates HTTPS on Linode.
- `www`: DNS-only CNAME to apex; Caddy redirects to `https://switchyard.cx`.
- `docs`: DNS-only CNAME to `switchyard-labs.github.io`; GitHub Pages HTTPS.
- Wildcard: proxied AAAA `100::`, Auto TTL; read-only Worker route
  `*.switchyard.cx/*`. Apex is outside this route; explicit DNS-only infrastructure
  records bypass the wildcard Worker.
- Serving Worker: `switchyard-pages`, version
  `5a861b5e-8c8b-452e-91d7-7c129f9c1f7e`, `workers_dev=false`.
- R2: existing `switchyard-actions-probe-20261003`, retained deliberately.
  Actions build assets, Release assets, immutable Pages content and production
  mappings occupy distinct namespaces. A bucket rename would require migration
  of those receipts and mappings; no new bucket or destructive migration was used.
- Cloudflare Universal TLS covers apex and wildcard. Apex, www, docs and an
  unconfigured owner host passed certificate verification (owner returns 404).
- Reserved owners: `www`, `docs`, `app`, `api`, `admin`, `assets`, `static`,
  `pages`, `login`, `signup`, `sy`, and every `dpl-` prefix.
- App schema 11 runs on Linode. Dedicated Actions control key stays in a
  mode-600 host file; the serving Worker has only R2 and domain bindings.
  Rotate by updating the Actions encrypted `CONTROL_SECRET` and the matching
  Linode key file in one maintenance window; never include values in evidence.
- Application cookies remain host-only and Secure with
  `SWITCHYARD_PUBLIC_URL=https://switchyard.cx`; application host remains apex.

Before activation the binary, environment, Caddy configuration and consistent
Trestle database were backed up on the host at
`/opt/cp0/switchyard/before-pages-20261005-012909/`. Schema 11 migration is
additive. The older binary is not compatible with schema 11: use a compatible
binary rollback, or restore metadata only during an exclusive maintenance window
with an explicit plan for writes made after that backup. Do not blindly restore
an old database over newer writes.

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
