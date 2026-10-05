# Auth and repository UI acceptance report — 2026-10-06

## A. Mailgun state

Active on Linode: authenticated Mailgun US SMTP, port 2525, required STARTTLS,
verified certificates and TLS 1.2 or newer. Sender: Switchyard
<no-reply@mg.switchyard.cx>. SPF, DKIM and monitoring DMARC are published;
existing apex mail routing is preserved. The user confirmed inbox delivery,
sender identity, expiry copy and mobile email layout. Recipient authentication
headers were not independently inspected. Credentials are confined to the
protected host environment and are absent from evidence.

## B–F. Link certification

- Verification resend: the user confirmed the old link was rejected and the new
  link verified the account. Verification persistence was checked through the API.
- Verification expiry: controlled-clock Trestle tests pass, including expired
  tokens and transactional rollback; no claim of waiting 24 hours for real mail.
- Verification reuse: the user confirmed the consumed link was rejected.
- Reset expiry: controlled-clock Trestle tests pass; no claim of waiting 30 minutes
  for a live email to expire.
- Reset reuse: the user confirmed rejection and successful login with the first
  replacement password. Live API checks reject the old password and session.

## G. Auth UI fixes

Signed-out headers now show Sign in and Sign up. Signup retains exactly Username,
Email, Password and Confirm password. Recovery links are checked on arrival,
before the form becomes visible. This read-only validation does not consume the
link, change the account version or produce an account audit mutation. Expired,
superseded and consumed links show the invalid-link message immediately. Submit
still validates atomically at consumption. Successful reset hides both the form
and its introductory instructions and offers sign-in with the new password.
Fragment tokens are immediately removed from the URL; responses retain no-store
and no-referrer. Known/unknown/disabled reset requests have equivalent responses
and a minimum response time; deterministic tests cover provider failure too.

## H. Screenshot-driven fixes

| Route | Problem / root cause | Fix / regression |
|---|---|---|
| Signed-out pages | Only one auth header action | Adjacent signup action; tested at six widths |
| Repository Code | Star occupied a separate heading row | Star next to Go to file in the toolbar |
| Work, Proposals, Pull requests, Actions, Commits, Releases, Pages, Settings | Page-specific headings omitted or diverged from repository navigation | Shared owner avatar, owner/repo breadcrumb, visibility badge, section label and all nine tabs; each section checked |
| Proposals | Loading class remained after an empty result | Settled empty state has no spinner; grouped header actions; spaced footer with zero-result pagination hidden |
| Actions | Partial tabs and plain visibility text | Shared full navigation and visibility badge |
| Pages | Loading class remained after empty deployments | Settled empty state has no spinner; irrelevant pagination hidden |
| Repository Settings | Generic shell sizing separated header and content vertically | Explicit header/body layout; content begins beneath navigation; desktop/mobile geometry assertions |

## I. Browser and code regression

60 repository/header checks passed at 1600, 1280, 1024, 768, 430 and 390 pixels,
using local UI assets, live public read APIs and isolated maintainer fixtures.
They check navigation, active tab, document overflow, signup discovery, settled
empty states and settings geometry. Desktop/mobile screenshots are in
`docs/evidence/ui-shell-c48/`.

Real disposable Trestle plus captured-mail desktop/mobile browser flows pass:
signup, resend supersession, verification, reset confirmation, reset, session
revocation, new-password login and link reuse rejected on arrival. Full Switchyard
Go tests and affected vet pass; Trestle account-token race tests and vet pass.
Real mail delivery is separately certified by the user; captured mail alone is
not delivery evidence.

## J–K. Checkpoints

Local Trestle: `4ceac7a` (non-consuming link validation).
Local Switchyard: `3c66d31` (Mailgun), `b29f45b` (arrival validation/reset copy),
`d462e69` (repository UI). Linode application checkpoint: `d462e69`; Trestle
checkpoint: `4ceac7a`. Documentation checkpoints may follow without changing
runtime code. No source was pushed, tagged or released.

## L. Remaining limitations

The token adapter supports standalone collection storage and fails closed for
replication. PostgreSQL runtime certification was unavailable. Mail throttles
are in memory and the reset-mail worker is nondurable: outage/crash requires a
new request. Existing missing-email accounts require private administrator
repair. Verification remains informational. DMARC is monitoring policy, not
an enforcement rollout. Snapshot restoration can revive post-snapshot tokens;
follow the documented incident invalidation procedure. Maintainer layout tests
use isolated fixtures and do not claim live settings-write certification.

## M. Readiness

The requested UI and real mail checks are complete. Final deployed browser checks
are recorded in `docs/evidence/auth-hardening/live-browser.json`. **READY TO PUSH** for this scoped campaign. Deployed desktop/mobile auth checks
passed. This report does not authorize or perform a push.
