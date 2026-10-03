# DeepSeek post-PX review

Independent review + live certification + deployment notes recorded after the
PX0–PX25 campaign. Written to be useful without this chat.

## Review verdict on PX0–PX25

The PX0–PX25 campaign is a strong product pass: it turned the checkpoint UI
into a coherent Git-host product (owner/repo identity, code browsing with
syntax/lines/README, a Warden-inspired editor, profiles, organisations, PRs,
Work, queue/attention/workflow surfaces, a read-only public demo contract, and
an accessibility pass) while preserving the agent-native core.

Weaknesses found in review:
- The app shell had never loaded `theme.css` (see bug 1) — the "product UI"
  was rendering unstyled until fixed.
- The brochure initially used the generic blue/gradient identity and a
  seven-link top nav; the redesign (rail-signal palette, full-screen hamburger
  nav, webp mark) fixed it.
- Strut carries the deterministic worker only; the executor strategy is
  unresolved (see the stack retrospective).

## Bugs found (fixed)

1. **App shell unstyled (white pages).** `templates/head.html` and
   `public/edit.html` linked only `app.css`, which uses `var(--bg)` etc. without
   defining them — the `:root` variables live in `theme.css`, which was never
   linked. The app rendered white since CP2 (the CP10 gate passed because it
   asserted functionality, and CM6's own oneDark made the editor dark).
   **Fix:** add `theme.css` before `app.css`; verify `body` background =
   `#0d1117` in a real browser.
2. **Favicon referenced `.svg`** in brochure templates and the app
   (`favicon.svg`) while the real mark is `favicon.webp`. **Fix:** all
   references now use `favicon.webp` with `type="image/webp"`.
3. **Brochure content images/links hard-coded.** Templates used `@path` but
   content pages used `/assets/...` and `/page.html`, which broke images from
   subpages. **Fix:** converted all to `@path('public/assets/...')` and
   `@path('<tracked-page>')`; verified screenshots load from both `/` and
   `/product/demo.html`.
4. **Guest-mode security** verified through the real proxy: anonymous requests
   to private API surfaces (repos, drafts, work, credentials, audit, queue,
   attention, SSE) all return 401; the public shell renders read-only.

## Deployment (live)

- **Service:** Switchyard runs as a systemd unit (`switchyard.service`),
  environment from `/opt/cp0/switchyard/cp0.env` (no secrets in the repo).
- **Reverse proxy:** Caddy (`caddy.service`, `/etc/caddy/Caddyfile`) on `:80` →
  `127.0.0.1:8080`; Trestle stays on `127.0.0.1:7350` (never exposed).
- **URL:** http://45.79.189.46/ (public IP; no DNS yet — TLS + a real domain is
  a follow-up). SSH tunnel no longer required.
- **Demo repos:** `demo-basic`, `demo-agents`, `demo-conflict`,
  `demo-semantic`, `demo-workflow` (curated; the many CP fixtures are hidden
  from prominent surfaces).

## Screenshots

Captured from the deployed PX-era app (dashboard, repo, pulls, work, editor,
profile) and pushed into the brochure.

## Remaining blockers to CP14/v1 (prioritised)

1. Real-LLM coding-agent leg (CP6 condition: provider credential + CLI on the
   control-plane host).
2. TLS + a real domain for the demo URL (DNS change required).
3. Trestle AI-DX campaign (API ergonomics) — see stack retrospective.
4. Strut executor strategy decision.
5. Interactive Agent-to-draft editing (Agent proposes → human diff → human
   commit) — the draft CAS substrate is ready.
6. Scratch-clone → bare-mirror/worktree performance work.
7. Independent Codex review of T1–T4.