# Email verification and recovery campaign

Status: implementation and local certification complete; backed-up live rollout in progress. Real mail certification awaits a provider.

## Architecture audit (2026-10-05)

Switchyard's identities are records in its `users` collection, not Trestle's
native `_trestle_app_users` table. Registration normalizes username and email,
reserves email in `account_emails`, persists a bcrypt hash (cost 10), creates an
owner namespace and signs in. Password policy is 8–72 bytes. Email is optional
for legacy accounts and remains private. Before this campaign there was no verification timestamp,
disabled-account field, recovery token or mail transport in this model.

Switchyard sessions are persisted collection records with a seven-day expiry.
Their opaque credential includes a fingerprint of the current password hash;
session authentication rejects credentials bound to an earlier hash. Logout
deletes the exact session. There is no separate refresh credential. Cookies are
HttpOnly, SameSite=Lax and Secure for the configured HTTPS public URL. Mutations
use the existing same-origin check. Native Trestle auth instead uses Argon2id,
`verified_at`, `disabled_at`, hashed rotating refresh credentials and access
credentials. These are separate account systems; changing native Trestle auth
alone would not implement recovery for Switchyard users.

The Trestle repository is
`/home/nick/Repositories/go-projects/trestle-cv/trestle`. Its collection service
already supports transactions and versioned writes on SQLite and PostgreSQL.
Its serialized-transaction helper provides bounded retries. Cluster mutations
have a separate replication authority and must never be bypassed by a new
security operation.

## Intended boundary

Preserve Switchyard's collection identities and bcrypt hashes. Trestle must
generate and hash tokens and transactionally consume them together with the
account update. Mail transport, templates, public links and browser policy stay
in Switchyard. Do not emulate atomic consumption with separate HTTP record
patches or move existing users into native auth as an incidental migration.

Legacy accounts remain usable and unverified. Verification is informational
for current actions. Reset returns to sign-in and invalidates every existing
session; it never creates a fresh session. Both purposes use latest-token-only
semantics and bind tokens to the issued email and password state.

## External delivery prerequisite

Read-only inspection of deployed environment variable names found only the
Trestle connection credentials and no SMTP or transactional-mail configuration.
Provider selection/access has been requested. No email credentials or raw
account tokens belong in this document or evidence. Existing DNS sender records
must be inspected and preserved before any changes.

## Completed implementation and local acceptance

Trestle owns a reusable administrator-authorized collection account-token
operation with hash-only bounded slots, expiry and latest-only semantics,
transactional verification/password change and minimal secret-free audit.
Standalone SQLite is certified; replication fails closed, PostgreSQL needs its
own configured provider test run. Account/collection/purpose/password/email
binding prevents credentials being copied between accounts or used after change.

Switchyard now has verification, resend, forgot/reset APIs and three browser
routes; private settings show status. It preserves bcrypt identities, automatic
signup login and unverified account usability. Reset invalidates old sessions
and the stale-password login race is closed. One implicit-TLS SMTP adapter and
deterministic mail tests cover MIME, authentication and safe transport failures.

Full tests/build/vet, affected race, portable backup/restore and generated/static
checks pass. Desktop/mobile tests against real disposable Trestle pass signup,
resend/replacement, signed-out verification, reuse rejection, generic reset,
confirmation, reset, old session/password rejection and new-password login.
Migration rehearsal on a private live snapshot passed Trestle 23→24 and
Switchyard 12→13, preserving users/session/email counts. Evidence is under
`docs/evidence/auth-hardening/`; see `account-recovery.md` for exact operations,
limits and restore implications.

## Remaining production acceptance

Backed-up rollout and live disposable-account regressions are next. Real
verification/reset email delivery and sender DNS cannot be certified until a
mail provider is configured. Existing registrar SPF is preserved; no DKIM/DMARC
or mail credentials were found. No push, tag or release is authorized.
