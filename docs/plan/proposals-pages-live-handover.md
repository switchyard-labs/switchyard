# Proposal + Pages live activation handover

Certified 5 October 2026. Subsequent [signup audit](../operations/signup.md) completed the four-field registration contract and updated Linode to `c27c3fd` / schema 12; deployed-source/schema statements below are the earlier Pages certification snapshot. [Evidence](../evidence/proposals-pages/live/) contains sanitized results and desktop/mobile captures. No campaign source push, tag or GitHub release was performed. Disposable fixture Git pushes were explicitly authorized.

## A. Cloudflare zone state

`switchyard.cx` is active in account `b7f20353ee8a9e5d2003f52c74ba795e`, zone `abf2755072006add6ff1e59342ab5114`. Authoritative nameservers are `linda.ns.cloudflare.com` and `noel.ns.cloudflare.com`.

## B. Final DNS table

| Name | Type | Target | Proxy |
| --- | --- | --- | --- |
| @ | A | 45.79.189.46 | DNS-only |
| www | CNAME | switchyard.cx | DNS-only |
| docs | CNAME | switchyard-labs.github.io | DNS-only |
| * | AAAA | 100:: | Proxied |

Auto TTL; unrelated TXT record preserved. Apex remains on Linode/Caddy; www redirects to apex; docs remains GitHub Pages.

## C. Worker route

`*.switchyard.cx/*` routes to `switchyard-pages`. Apex is outside the pattern; explicit DNS-only www/docs records bypass it. Unknown owner and preview hosts fail closed. Claimed project prefixes do not fall through to owner content.

## D. Worker deployment/version

Serving Worker: `5a861b5e-8c8b-452e-91d7-7c129f9c1f7e`, 100%, deployed 04:46 UTC. Actions Worker `switchyard-actions-probe-20261003`: `1999cbae-0d3b-485a-8bd8-fff117c25ae0`, 100%, deployed 04:06 UTC. Serving Worker has R2/domain bindings only; workers.dev is disabled. Existing restricted Actions allowlist includes the disposable Pages fixture; no blanket rollout.

## E. Production R2 bucket

Existing `switchyard-actions-probe-20261003` retained. `build-assets/`, `release-assets/`, `pages/`, `pages-hosts/` remain separate namespaces. Direct authenticated R2 list/HEAD/payload checksum checks passed for owner/project A. No bucket rename, destructive migration or new paid bucket.

## F. TLS result

Universal TLS covers apex/wildcard. Real certificate-verifying HTTPS requests passed apex, www redirect, docs, owner/project/preview and unknown-owner 404. Native Pages status is explicitly active in app configuration.

## G. Reserved owner names

Infrastructure names `www`, `docs`, `app`, `api`, `admin`, `assets`, `static`, `pages`, `login`, `signup`, `sy`, and `dpl-` preview prefix are unavailable. Existing application route reservations remain. Live smoke found account registration initially had a narrower guard than Pages. Fixed shared owner validation; regression verifies rejection before persistence; subsequent live requests rejected. Removed only the exact www user, namespace and session created by the initial smoke at 06:19:21 UTC. Automatic review initially rejected repeating the unsafe request; after the guard fix, safer verification succeeded. No unresolved approval blocker remains.

## H. Live owner Pages result

`https://pages-probe-1005.switchyard.cx/` serves immutable A from explicitly chosen backing repo `pages-probe-1005/foo.js`, Artifact repo `codex-pages-live-20261005`. Owner A deployment `dpl_98b8566ae25fc83b6041760f587f7490`. Ordinary repo name, no magic naming requirement. Root/nested/assets/404, HEAD, ETag/304 and byte range/206 passed.

## I. Live repo Pages result

`https://pages-probe-1005.switchyard.cx/foo.js/` serves project A, deployment `dpl_d0910851ca1f63375a658da3fe46e003`. Hyphenated owner/dotted repo and immutable preview URLs work. Missing project content stays scoped. Local naming matrix additionally covers hyphen, underscore and dotted repository names.

## J. Nift base-path result

Real Nift v4.6.0 static binary ran in the cloud build. `SWITCHYARD_PAGES_BASE_PATH` controls template assets for `/` or `/foo.js/`. Live browser confirms prefixed CSS/JS and nested routing. Unprefixed `/assets/...` addresses the owner root by browser URL semantics; Pages does not rewrite HTML. Authors must configure their generator/router base path.

## K. Real Actions → Pages result

A manual approved build succeeded. Native Git push B triggered real Artifacts event → Actions run `179aba02-806f-491b-8405-90090742bfbd` → approved static output → immutable R2 manifest/payload → owner/project ready records. Exact B SHA `398a11a36274c5166b95f9f7e8087a5cf0f7a459` persisted. Historical definition approvals survive config edits, and terminal workflow observations survive restart. Export reads the SDK build workspace `/workspace`, not untouched source staging. Alternate branch C produced a ready project deployment pinned to `ff407b7f87bc697eaa96cac2f23b8457dceb1fd2`. Repeated terminal observation did not duplicate deployment records or advance production. Direct raw duplicate callback injection was not separately performed live; local replay tests cover it. Failed build produced a failed candidate while preserving production generation 6. No coding Agent ran on Linode.

## L. Promotion/rollback result

Owner/project A → B → rollback A passed through live CLI and Worker/R2 conditional mapping. Current generation 6, both A. Same-operation replay keeps generation 6; stale-generation mutation returns 409. Trestle audit replay collision exposed a real idempotency difference: recovery now verifies the existing complete operation identity and accepts its authoritative event rather than writing a mismatched retry. Old/failed output never implicitly changes production.

## M. Shared-origin security result

App cookie is host-only Secure/HttpOnly; browser requests to Pages do not carry app auth. Project service worker scope `/foo.js/` succeeds, `/` is denied, sibling path stays uncontrolled. Projects of one owner deliberately share origin/storage; local browser proof demonstrates localStorage/IndexedDB sharing. Owner-root scripts must be trusted. No per-project sandbox promise; authored CSP does not change this origin model.

## N. Live Proposal smoke

Proposal `prop_78a0ecfc4ae4`: create, discussion, retry-safe linked Work `wk_b03c39ccd91b968addc7d3c749b706be`, accepted state and four audit entries passed real Trestle. Public read boundaries follow repository authorization. No commercial proposer run was required for this static activation; local proposer execution/recovery certification remains the recorded evidence.

## O. sy live smoke

CLI source `85f768347ec16c9226ae4e77445b7ea212785c6c`: Proposal list/view, Pages status/deployments/view, promote and rollback passed live. Private CLI auth config stays mode 600 outside evidence. CLI source unchanged this activation.

## P. Existing-product regression

Full app Go tests/vet passed, affected race suite passed, Actions 30 tests/TypeScript and Pages 16 tests passed; Wrangler build checks passed. Live browser smoke: profile, public org directory, repo/source, Work/detail, PRs, actual Agents/Queue tabs, Actions, Releases, editor, Proposal and Pages, plus host-cookie isolation. This is read-only navigation/status/JS-error smoke, not a new execution of every mutation. Public repository archive separately returned ZIP magic. Proposal/Pages browser matrix passed six widths (1600,1280,1024,768,430,390). Signed-out delayed-auth tests verify six surfaces expose no Alice workspace flash. APIs are no-store; private org profile authorization was tightened. Intentionally public Alice demos remain publicly discoverable.

## Q. Final implementation-ledger state

[Ledger](proposals-pages-implementation.md) now leads with current live-certified state. Earlier PARTIAL/CONDITIONAL checkpoints are explicitly historical. [Activation operations](../../workers/pages/ACTIVATION.md) records current infrastructure and rollback constraints. Metadata schema 11 is additive but pre-11 binary rollback is incompatible; use compatible binary backup. Do not restore old metadata over post-backup writes.

## R. Final local commits

App implementation/runtime `2ce7c71`; test-only follow-up `c5cefb4`; this handover/evidence is the following documentation commit (resolve `git rev-parse HEAD`). Prior app activation fixes `9f37284`, `3bc4930`, `ba9f52a`, `47ba851`, `3db112e`, `a6076dc`. CLI `85f7683`. Before this documentation commit: cached origin comparison app 0 behind/123 ahead, sy 0 behind/9 ahead. Remote refs were not freshly fetched, so recheck before push. Older untracked C38/C42/C43 captures and Python caches were preserved outside this campaign staging; CLI tree clean. No unexplained new campaign files remain after staging.

## S. Deployed app commit

Linode binary source `2ce7c717b22bed1d6f33e1d448ca968fa2bc1fb3`, version activation-dev, Go1.26.0. SHA256 `c86cafa02809775d500301a1a9e1e2b3d30a0721ba8e880900a17c22bb604817`, identical locally/remotely; service active. Later commit changes tests only. PUBLIC_URL `https://switchyard.cx`, NATIVE_HOSTING active; embedded assets are used. Compatible pre-last-deploy backup `/opt/cp0/switchyard/before-signedout-20261005-065215/`; initial full backup `/opt/cp0/switchyard/before-pages-20261005-012909/` includes consistent Trestle DB. Secrets never entered evidence.

## T. Remaining blockers

- **must fix:** none identified in activation scope.
- **external prerequisite:** source push requires Nick's authorization; not an activation defect.
- **nice to have:** broaden provider quality/performance qualification and live mutation regression beyond this activation smoke.
- **defer:** custom domains, imports, confidential Security intake, rename migration, quota/billing/automated retention, archival beyond bounded histories and broad Actions allowlist rollout. Current artifacts/previews retained conservatively. Disposable fixture account/repo/build history retained as certification evidence; no automated cleanup removes rollback candidates.

## U. Push recommendation

**READY TO PUSH** after authorization and refreshed origin comparison. Order: Switchyard app (includes Workers/docs), then sy CLI. No separate website repo changed in this activation. Do not force push, tag or create release without instruction. Native DNS/TLS and real build/serving/promotion evidence are complete.
