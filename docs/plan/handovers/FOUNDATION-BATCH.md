# Product/UX Foundation Batch — PX0–PX4

## Status

| Checkpoint | Status | Commit |
| --- | --- | --- |
| PX0 — baseline audit/evidence | PASS | `b9d363a` |
| PX1 — IA + URL/domain model | PASS | `0071688` |
| PX2 — owner/repository namespace | CONDITIONAL PASS | `6e0610b` |
| PX3 — unified design system | PASS | `c771fc2` |
| PX4 — navigation systems | CONDITIONAL PASS | `ecc40b7` |

PX2's condition is deployment-only: register and browse one user-owned and one org-owned real Artifacts repository through the new canonical owner/repository layer.

PX4's condition is browser-certification-only: rerun desktop/mobile Chromium interaction screenshots on the normal development host. Static structure, JavaScript syntax and navigation smoke checks are green in this workspace.

## What now exists

### Product model

- canonical public repository identity is `owner/repo`;
- physical Artifacts repository identity is decoupled from public ownership/slug;
- users and organizations share a collision-safe top-level owner namespace;
- existing flat repository APIs/routes remain compatibility paths;
- no Artifacts repo was renamed/deleted or bulk-claimed.

### Visual system

- product GitHub-blue primary/accent debt removed from authored CSS;
- near-black graphite/iron surfaces;
- restrained rail amber + signal green/red functional accents;
- real Switchyard mark in product headers;
- exact `<title>Switchyard</title>` preserved for product/editor.

### Navigation

- product: hamburger/full-screen navigation at desktop + mobile widths;
- collapsible Explore / Collaborate / Account groups;
- Escape, focus trap, body scroll lock and active-page behavior;
- brochure: collapsible Product / Concepts / Documentation / Project groups;
- brochure docs: separate collapsible docs drawer/navigation;
- API-only concepts are not linked as if they were finished UX.

## Validation constraints in this artifact runner

The supplied workspace declares Go 1.26 and has uncached external modules; this isolated runtime cannot reach `proxy.golang.org`, so full `go test ./...` / `go vet ./...` cannot execute here. Go files were formatted/parsed with local `gofmt`; JS passes `node --check`; repository-specific unit tests were added for route/slug parsing and are intended to run in the normal Go 1.26 development environment.

Nift is not installed in this artifact runner. Source templates and generated public pages were kept aligned manually for this batch; the normal development host should run `nift build --all` before deployment and verify no output drift.

## Next batch boundary

Stop here. The next requested batch is **Code Surfaces (PX5–PX9)**:

1. shared language detection + syntax highlighting;
2. repository viewer 2.0;
3. Markdown/README/image renderer;
4. Warden-quality editor/workbench pass;
5. interactive Agent-to-draft UX.
