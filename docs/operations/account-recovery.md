# Email verification and password recovery

Switchyard keeps its existing collection-backed bcrypt identities (cost 10,
8–72 byte passwords). Trestle's separate native Argon2id authentication is
unchanged. Signup remains Username, Email, Password, Confirm password and signs
in automatically. Verification is informational; legacy users stay usable and
are not silently marked verified. An account with no recorded email needs an
administrator to assign a private, uniquely reserved email before recovery can
work; there is no new email-change workflow in this campaign.

## Routes and ownership

Browser routes: `/verify-email`, `/forgot-password`, `/reset-password`.
Mutations: POST `/api/auth/verify-email`, `/api/auth/resend-verification`,
`/api/auth/password-reset/request`, `/api/auth/password-reset/complete`.
Private settings: GET `/api/settings/email`.

Trestle POST `/api/v1/collections/{collection}/records/{id}/account-auth`
accepts `action: issue|consume`, `purpose: email_verification|password_reset`,
issuance `lifetime_seconds`, or consumption `token` and reset `new_password`.
Only an authenticated administrator with the existing CSRF credential can call
it. Generic record grants, service tokens and anonymous clients cannot. The
administrator is a trusted database-wide application boundary, not a restricted
multi-application credential. Isolate applications in separate Trestle instances
if they must not share administrator trust. Cluster/replicated mode fails closed
with 503; this first adapter supports standalone collection storage only.

Trestle generates 256-bit URL-safe credentials and persists only SHA-256 hashes
bound to collection ID, record ID, purpose, email and current password state.
The account's bounded `account_auth` JSON holds one slot per purpose, including
creation, expiry and consumption timestamps. Latest issued link wins. This
uses existing collection backup infrastructure rather than an unbounded token
table. Consumption and account updates share one serialized transaction and
version check; PostgreSQL additionally locks the account row. New password hash
invalidates other outstanding links and every old Switchyard session through
its password-state fingerprint. Session rows may remain until cleanup but can
no longer authenticate. A concurrently validated old password cannot mint a
credential bound to the new hash. Reset creates no replacement session.

Links carry credentials in URL fragments, which are not sent in HTTP requests.
The head script clears them from browser history immediately and submits JSON
by POST. Compatibility query links are also cleared, but their initial request
can appear in upstream access logs; generate only fragment links. Recovery
responses use `no-store` and `no-referrer`. No token, password, private email or
full recovery URL belongs in logs or evidence. Mail delivery metrics use only a
static operation name, duration and status.

## Delivery configuration

One production adapter: authenticated SMTP with certificate verification and
TLS 1.2 or newer. `SWITCHYARD_SMTP_TLS_MODE` defaults to `implicit`; `starttls`
requires a successful TLS upgrade before authentication. Unknown modes fail
closed, and neither mode falls back to plaintext. Configure privately:

- `SWITCHYARD_SMTP_ADDRESS=provider-host:port`
- `SWITCHYARD_SMTP_TLS_MODE=implicit` or `starttls`
- `SWITCHYARD_SMTP_USERNAME`
- `SWITCHYARD_SMTP_PASSWORD`
- `SWITCHYARD_MAIL_FROM=Switchyard <no-reply@mg.switchyard.cx>`
- `SWITCHYARD_PUBLIC_URL=https://switchyard.cx`

On 6 October 2026, Linode authenticated to Mailgun at `smtp.mailgun.org:2525`
using required STARTTLS. Port 465 timed out. The supplied Mailgun SPF, DKIM and
monitoring DMARC records for `mg.switchyard.cx` are published and confirmed
through public DNS. Existing apex SPF and routing are preserved. Credentials
are stored only in the protected host environment. Real registration verification and reset emails reached the approved inbox;
the user completed both links and logged in with the new password. The live
API confirmed verification persistence and rejection of the old password and
session. Real resend, expiry and duplicate-link mail scenarios remain separate
certification checks; do not infer them from this successful flow.

Mail contains restrained plain-text and HTML alternatives, the one-time HTTPS
link, UTC expiry and an unsolicited-message notice. Signup survives delivery
failure with an unverified account and settings resend prompt. Resend reports a
safe failure. Reset request always returns the same 202 message for known,
unknown, disabled or throttled addresses. Its response has a 750 ms minimum;
delivery runs asynchronously to avoid provider-dependent account enumeration.
The background work is not a durable queue: a process crash/provider outage
requires a new request. SMTP socket work is bounded to 15 seconds; Trestle's
existing client timeout bounds its calls. No real delivery is claimed by fake
mail/browser tests.

## Limits and policy

Email issuance budgets per five minutes: transport IP 10, normalized email 3,
process-wide 100. Budgets are in memory and reset on restart. The current reverse
proxy presents a shared loopback peer, so the IP budget can be shared by clients;
do not trust arbitrary forwarded headers to circumvent it. Consumption uses
existing authentication throttles (10 attempts per five minutes/purpose/peer).
Registration retains its existing configurable open/closed policy and throttle.

Verification defaults to 86400 seconds, configurable with
`SWITCHYARD_VERIFY_TOKEN_SECONDS` (60–172800). Reset defaults to 1800 seconds,
`SWITCHYARD_RESET_TOKEN_SECONDS` (60–7200). Invalid configuration fails closed.
Single-use, expired and superseded links share a safe invalid/expired message.
Already-verified resend is idempotent. Disabled accounts cannot issue/consume
links or authenticate. CLI recovery is future work.

## Migration, backup and rollback

Switchyard schema 12 → 13 adds `verified_at`, `disabled_at` and `account_auth`
to `users`; existing email, hashes, verification state and sessions are preserved.
No fabricated addresses or verification timestamps. The current Trestle binary
also upgrades the deployed native storage schema 23 → 24; that separate additive
migration must be rehearsed and backed up with deployment.

Take a consistent database backup plus both binaries and private environment
before migration. Portable backups include hash-only token slots. Restoring an
unexpired, unconsumed token preserves its validity. Restoring an earlier snapshot
can revive a token consumed after that snapshot; invalidate account token slots
and revoke sessions/password state as part of an incident restore when needed.
Portable import intentionally revokes administrator sessions: log in again.
Older Switchyard binaries reject schema 13; roll back both compatible binaries
and the exclusive database snapshot while all writers are stopped. A rollback
loses changes after the snapshot and is not an ordinary online downgrade.

## Troubleshooting and certification

Check private settings first. Missing legacy email requires administrator repair.
Check static `auth_mail_delivery` metrics, provider sender approval, private
SMTP configuration, HTTPS public URL and TLS connectivity; never log credentials
while diagnosing. Request another link after a failure or expiry.

Focused race tests cover atomic verification/reset, replacement, email binding,
disable behavior, stale login state and portable restore. The opt-in real-Trestle
browser test runs signup/resend/signed-out verification/reset at desktop/mobile
widths with deterministic captured mail, checks URL clearing and rejects old
passwords/sessions/reused links. PostgreSQL execution and real provider delivery
must be certified separately if those deployment paths are enabled.
