# C33 — repository-scoped Git capabilities

`POST /api/repositories/{owner}/{repo}/git-credential` requires an authenticated
user, even for public repositories. JSON accepts `scope` (`read`, default, or
`write`) and `ttl_seconds` (60–900, default 600). A successful response contains
`remote`, `token`, `expires_at`, `permissions`, and canonical `repository`.
Responses use `Cache-Control: no-store`. Secrets are returned once and never
persisted in audit records. Audit failure prevents returning the credential.

Read access follows repository authorization. Write requires write permission,
an unarchived repository and a writable upstream repository. Any protected-ref
policy blocks repository-wide write capability issuance: native Artifacts tokens
cannot enforce Switchyard's per-ref policy. This limitation is intentional and
does not bypass Integration Queue protection.

The Code menu offers a ten-minute read capability. The token stays in a closure,
not an input, URL, local storage or rendered text, and closing the menu clears it.
Copying is an explicit user action. `sy` adoption follows C39 and CLI review.

## Verified 2026-10-04 (Melbourne)

- Full Go tests and vet passed. Twelve credential authorization cases include
  guest, owner, public reader, private nonmember, read collaborator, protected
  read/write, archive, upstream read-only and invalid scope/TTL.
- Real endpoint returned a two-minute read capability for the disposable
  `alice/railway` repository, backed by `codex-actions-probe-20261003`.
- Real HTTPS clone and subsequent fetch succeeded at
  `e2a22198d24faa843975adef7fe7ffde1b997aa5`.
- Token was absent from command arguments and the cloned `.git/config`;
  transport authorization used ephemeral environment configuration.
- Actual provider tokens differed from the old fixture prefix. The handler now
  treats the secret as opaque while checking transport safety and bounded expiry.
- Chromium clone popover remained anchored within the viewport at
  1600/1280/1024/768/430/390, with keyboard tab changes, Escape/focus return,
  trigger toggle and outside dismissal. See `clone-layout.json`, `clone-menu.png`.

## Limits

Real write-token push, expiry rejection after elapsed TTL, and browser credential
issuance/copy contents are not yet certified. UI layout testing did not mint a
credential. No production deployment or `sy` change is part of this checkpoint.
The short-lived credential is still sensitive until expiry; it is not revocable
through this endpoint. The server's operator token helper is bounded to 20 seconds
and 64 KiB, and mint response reads to 64 KiB.
