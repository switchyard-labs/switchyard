# C22 browser certification

4 October 2026 (Melbourne). Actual application and native browser, real isolated Trestle and Artifacts fixtures, and public Linode demo. No injected responses or synthetic DOM states.

`matrix.json` indexes 168 product captures: 28 populated/public/empty surfaces at 1600, 1280, 1024, 768, 430 and 390px. Each checks loaded content, shell, document overflow, actual viewport and valid links. Product and brochure menus each have six geometry captures. The 19 brochure pages have 42 captures (homepage at six widths, other pages at desktop/mobile), with image/link/Nift expansion checks. The intentional wrong-viewport test fails as expected. Only indexed images are final evidence; superseded working images are not proof.

Native editor selection, repeated Ctrl+D edits and undo restored the original source. Explicitly authorized Ctrl+Shift+S saved that unchanged source as a draft on disposable editor-c20, without a Git commit; typing/undo then correctly cleared the dirty indicator. Persisted star/reload/profile, follower empty state, and own-profile Edit profile navigation were inspected through the product. Real Actions logs, PR diff, Work, workflow/Attention and genuine unborn repository/editor states are included.

Latest touch-ups: 250px square profile avatar; 365 dated contribution cells in 53 calendar weeks, hidden alignment placeholders, shared responsive month/cell columns, final month label aligned right, accessible counting info tooltip. Live desktop calendar scrollWidth equals clientWidth (833px); last populated column reaches the grid right edge. Header/menu right edge equals dashboard context-card edge. `profile-touchups-1280.png` shows the result. See `latest-verification.md` for latency measurements and their limits.

JavaScript/static accessibility/formatting gates pass. Full Go race suite and vet passed for these backend changes, followed by targeted profile/follow/star/contribution/legacy/operator tests. Chromium only: these results do not claim Firefox/WebKit certification or that later C23–C25 gates are complete.

### 4 October follow-up

The adopted continuation is recorded in `docs/plan/codex-finish-c22-c44.md`; C0–C21 remain accepted with prior limits, C22/C23 partial and C24–C44 pending. Header account links are now avatar-only, accessible and exactly 38×38 like the menu control. Profile overview cards show at most four repositories plus a View all link without nested scrolling. Organization fallbacks use the organization name rather than its internal ID; empty slug records are excluded and every tile has a name. The real isolated profile passes six actual-width checks in `profile-regression-20261004.json` using `scripts/cua-profile-regression.mjs`, including 365 unique dated calendar cells. Screenshot: `profile-regression-1280.png`. Only Chromium is connected; Firefox and WebKit coverage is unavailable and remains a gap.
