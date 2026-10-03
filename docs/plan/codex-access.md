# Codex access handover — Switchyard live environment

For the Codex whole-project independent review. Start from the pushed state,
not the local workspace. This doc is the environment/access map.

## Start state (exact, pushed)

| Repo | Branch | HEAD (pushed) |
| --- | --- | --- |
| `switchyard-labs/switchyard` | `main` | see `git rev-parse origin/main` (DeepSeek's last push) |
| `switchyard-labs/switchyard-labs.github.io` | `stage` (source) + `public/` repo `main` (deployed) | see `origin/stage`, `public/origin/main` |

Run `git fetch --all --prune && git status && git branch -vv && git log --oneline -30` first.

## Live topology (already deployed)

```
Internet
   ↓
Caddy :80   (caddy.service, /etc/caddy/Caddyfile)
   ↓
Switchyard 127.0.0.1:8080   (switchyard.service, env /opt/cp0/switchyard/cp0.env)
   ↓
Trestle 127.0.0.1:7350   (localhost-only, never exposed)
```

- **Public URL:** http://45.79.189.46/
- **No DNS/TLS yet** (the Linode has no A/AAAA record for a domain; do not invent/purchase one — report it as a v1 blocker).

## Access credentials (usernames/passwords)

### Demo / application
- App user: **`alice` / `password123`** (public demo account; can register more via the UI).
- Admin of the app itself: there is no separate admin role; the Trestle admin is the backend owner.

### Trestle (backend coordination truth)
- Trestle is at `127.0.0.1:7350` on the Linode only.
- **Admin username:** `admin`.
- **Admin password:** read from the file **`/opt/cp0/repro/.adminpass`** on the Linode (do not paste into the repo).
- API: `POST /admin/v1/session`, CSRF from `GET /admin/v1/session`; records via `/api/v1/collections/{c}/records`.

### Linode host
- Host: `switchyard-cp0` (alias for `45.79.189.46`); SSH as root via your key.
- Deployment dir: `/opt/cp0/switchyard/` (binary `switchyard`, `public/`, `cp0.env`, `start.sh`, `data/`).
- Services: `systemctl status switchyard caddy`; logs: `journalctl -u switchyard`, `/opt/cp0/switchyard/control.log`.
- Trestle runs from `/opt/cp0/trestle-v015` (4 processes — the known clustered side-effect defect; single-node usage unaffected).

### Cloudflare / Linode API keys
Read the **end of `~/.bashrc`** on the workstation — the relevant exports are:
```
export LINODE_CLI_TOKEN=…
export CLOUDFLARE_API_TOKEN=…
export CLOUDFLARE_ACCESS_KEY_ID=…
export CLOUDFLARE_SECRET_ACCESS_KEY=…
export CLOUDFLARE_S3_API_ENDPOINT=…
```
(`grep -n "CLOUDFLARE\|LINODE" ~/.bashrc` — around the last lines.)

- **Cloudflare account id:** `b7f20353ee8a9e5d2003f52c74ba795e`
- **Artifacts namespace:** `switchyard-cp0`
- **Runtime account token:** the control plane gets it from the helper `/opt/cp0/switchyard/token.sh` (prints a fresh `cfoat_…` OAuth token). On the workstation you can run it via `ssh switchyard-cp0 '/opt/cp0/switchyard/token.sh'`.
- **Cloudflare Queue id (event fast path):** `2b68d6736b3a4d0ca4aea6b36d47fde1` (currently upstream delivery not flowing; reconciliation is the live safety net).

## Demo repositories (curated, in the namespace)
`demo-basic`, `demo-agents`, `demo-conflict`, `demo-semantic`, `demo-workflow`. The namespace also contains CP/PX fixture repos — treat those as evidence, not demo content.

## Current product state (DeepSeek's last deploy)
- Strut deterministic worker **replaced** with an in-process Go adapter (no Strut binary required at runtime; `SWITCHYARD_STRUT_BIN` inert).
- `/repositories` route collision fixed (owner-slug hijack) — the Repositories directory now serves correctly.
- Full-screen menu close button was 24px off the hamburger; a CSS inset fix was applied but **menu polish is explicitly handed to Codex** to verify/complete.
- Guest (anonymous) mode is read-only; every private API surface returns 401 for guests (verified through the proxy).

## Review pointers (docs to read first)
`docs/plan/` — `product-certification.md`, `post-cp13-review.md`, `stack-dogfood-retrospective.md`, `deepseek-post-px-review.md`, `codex-review.md`, `handovers/PRODUCT-UX-CAMPAIGN.md`, `PX*.md`, `CP*.md`. Treat prior PASS verdicts as fallible.