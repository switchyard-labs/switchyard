# Auth hardening acceptance report — 2026-10-05

Implemented and deployed; real provider delivery is the remaining infrastructure
prerequisite. Source has not been pushed, tagged or released.

| Item | Result |
|---|---|
| A. Trestle | Administrator/CSRF-protected generic collection account-token operation; atomic update/consume and minimal audit. Native Argon2id auth unchanged; actual Switchyard bcrypt identities preserved. |
| B. Tokens | 256-bit CSPRNG, hash-only bounded account JSON slots, account/collection/purpose/email/password-state binding, expiry, latest-only, single-use. |
| C. Verification | Signup issues link, automatic sign-in retained, mail failure preserves unverified account; private settings/resend; browser GET and explicit POST consumption. |
| D. Reset | Email-only request with equivalent 202 response; new/confirm password; atomic Trestle password change; returns to sign-in. |
| E. Sessions | Password-state change rejects every old session; login race cannot bind a validated old password to the new hash. No fresh reset session. Old rows may remain but cannot authenticate. |
| F. Transport | One authenticated implicit-TLS SMTP adapter, TLS>=1.2/certificate verification, plain/HTML templates, deterministic fake. Production credentials absent. |
| G. DNS | Existing registrar-forwarder SPF preserved. No DKIM/DMARC found; provider-specific sender records not configured. |
| H. Limits | Mail IP10/email3/global100 per five minutes; existing consume/signup throttles. Verification24h (60s–48h), reset30m (60s–2h), configurable. |
| I. Migration | Users stay usable/unverified; no fabricated email/timestamp. Trestle23→24 and Switchyard12→13 rehearsed and backed up. Missing-email legacy accounts need private administrator repair. |
| J. UX | Four-field signup, settings status/resend, forgot discovery, safe invalid/reused links, immediate URL clearing. Desktop1600/mobile390 tested locally and deployed. |
| K. Tests | Full build/tests/vet; affected race; atomic reset/verification, resend-vs-verify, expiry, rollback fault, latest/email/disable binding; portable restore; SMTP/MIME tests; real-Trestle captured-mail browser flows pass. |
| L. Live verification | Trusted-backend issuance plus public signed-out consumption, state and reuse rejection pass. Real verification email delivery not certified. |
| M. Live reset | Public consumption, old password/session rejection, new login/reuse rejection pass; disposable accounts cleaned. Real reset email delivery not certified. |
| N. Limits | Standalone storage only for this adapter; replication fails closed. PostgreSQL test environment unavailable. Shared proxy peer budget, in-memory throttles, nondurable reset mail worker. Legacy email assignment and CLI recovery have no new self-service flow. Snapshot restores can revive post-snapshot-consumed tokens; documented incident invalidation is required. |
| O. Deployment | https://switchyard.cx active on `b2bb0d9`; Trestle implementation `aebb5a4`. Consistent backup `/opt/cp0/switchyard/before-auth-20261005-105212`. Pages, Proposal, cookie isolation and signed-out regressions pass. |
| P. Local commits | Trestle `aebb5a4` implementation, `6edad83` acceptance tests; Switchyard `b2bb0d9` implementation/UI/tests and subsequent certification documentation commit. |
| Q. Readiness | Feasible implementation/certification complete. Configure authorized SMTP sender plus SPF/DKIM/DMARC, then certify real verification/reset emails before claiming email recovery production-ready. No push/tag/release. |

Evidence: `docs/evidence/auth-hardening/`. Operational setup, routes, privacy,
limits, failure handling and rollback: `docs/operations/account-recovery.md`.
