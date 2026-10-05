# Self-service signup

Status: **IMPLEMENTED**, audited and completed 5 October 2026. Previously partial: the API created username/password accounts, but the form offered Display name and lacked email/confirmation.

## Routes and contract

- `/signup` is the registration page; `/register` is an alias.
- `/signin` and `/signin.html` provide “No account? Register”.
- `GET /api/auth/registration` returns registration policy and email-verification capability.
- `POST /api/auth/register`: `username`, `email`, `password`, `confirm_password`; returns 201 and a host-only session cookie after account/session persistence.
- `POST /api/auth/login`, `GET /api/auth/me`, `POST /api/auth/logout` verify the subsequent auth lifecycle.

Exactly four signup fields: Username, Email, Password, Confirm password. No Display name; profile editing can change the default username-based display name later. Username is normalized to lowercase, 3–39 characters, letters/numbers/hyphens, with infrastructure/application names reserved. Email is trimmed/lowercased, validated, private, and enforced unique by a Trestle database index in `account_emails`. Passwords are bcrypt-hashed, 8–72 bytes; confirmation is enforced in browser and API. Existing users retain their accounts and may have no email; no fabricated backfill.

Schema 12 adds nullable `users.email` and a new unique-email reservation collection. It does not modify existing username/password values or uniqueness constraints. Registration fails closed on metadata lookup/persistence failures; compensation only deletes version-1 IDs created by the failing request. A process crash between cross-collection writes may leave a conservative email reservation requiring operator reconciliation; the flow does not claim a multi-record transaction. A session-write failure leaves a persisted account; UI directs the user to sign in again.

## Policy and exposure

Current live policy: **open self-service signup**. `SWITCHYARD_REGISTRATION_POLICY=open` or unset enables it. `disabled` and every unknown value disable registration; UI checks policy before enabling account creation. Invite-only registration is not implemented or implied.

Safe to expose for username/password accounts with the tested repository authorization boundaries. Email is **not verified**, and there is no email recovery flow. Email must not establish identity, grant organization/repository access, or drive recovery until verification is implemented. New accounts do not inherit Alice's permissions. Public profiles omit email.

Same-origin write checks and host-only Secure/HttpOnly/SameSite=Lax cookies apply. Signup and login have separate bounded, in-memory throttles: 10 attempts per transport peer IP per 5 minutes, per app process. Forwarded headers are not trusted for this limiter; behind the current reverse proxy the signup budget is shared by that proxy's traffic. This is a conservative initial throttle, not distributed anti-abuse protection.

## Verification

- Full Go tests and vet; focused race tests for signup, validation, registration reservations, throttle, anonymous and demo boundaries.
- Backend validation tests: malformed email, short password, confirmation mismatch, reserved names; disabled/unknown policy; normalized persistence, bcrypt hash, duplicate username/email, auto-session, later login, email privacy.
- Real Linode desktop 1600px and mobile 390px browser flow: signed-out Register navigation, exactly four fields, client mismatch blocked without a request, signup 201, immediate authenticated state, logout 401, fresh login 200, duplicate username/email 409, public repo read 200, unrelated public repo administration denied 403, no JS errors/page overflow.
- Real Trestle read-only record/index verification and a disposable private metadata fixture: owner allowed, anonymous and another freshly registered account denied. No Git repository was created for this authorization fixture.
- Evidence: [signup captures and sanitized results](../evidence/signup/). Temporary test-account credentials stay outside the repository. Private fixture/test accounts are removed after certification.

The `sy` CLI currently provides login/status/logout, **not a signup command**. Self-registration is supported through browser/API; no CLI signup capability is claimed.

Linode schema migration backup: `/opt/cp0/switchyard/before-signup-20261005-085011/`, including binary/environment and a consistent Trestle database backup during app maintenance. Schema-11 binaries cannot be restarted against schema 12; rollback needs a compatible binary or an exclusive, write-aware metadata restore.
