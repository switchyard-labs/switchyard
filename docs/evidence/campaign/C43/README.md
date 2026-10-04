# C43 — final embedded-runtime certification

DONE within the declared runtime/browser bounds, 5 October 2026.

The exact deployed product binary from clean commit `84552b7` ran from a fresh
directory containing only the binary and data directory. No frontend source,
public directory, templates, Node, Nift or Strut was on its runtime tools path.
Git, external Trestle and Cloudflare were deliberately retained. This is a fresh
runtime directory, not a fresh database or a claim of zero external services.

`final-84552b7/runtime.json` records the binary SHA-256 and safe coordinator
handoff. `browser.json` records 16 fully loaded Chromium surfaces at 1600 pixels:
profile, repository, source, release detail, Actions, editor, dashboard, Work, PR,
organization, settings, Attention, workflow, history, branches and tags. All had
loaded content, embedded shell, bounded documents and images; no captured console
errors. Actual completed CI output was additionally captured.

`archive.json` verifies the actual browser ZIP against the exact immutable Git
tree: 18 paths with identical bytes and no .git. The browser download operation
blocked unexpectedly for approximately 40 minutes; no download-latency PASS is
claimed. This is recorded separately from ordinary API timing.

The final runtime was drained and the original isolated preview restored, so two
Actions coordinators were never intentionally active together. Firefox/WebKit
were unavailable. Authenticated live CLI and browser token issuance remain
separate conditional gates; they are not inferred from this packaging proof.
