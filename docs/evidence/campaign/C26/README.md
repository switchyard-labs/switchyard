# C26 — embedded release UI

The release embeds all public HTML and assets through Go embed. An empty
`SWITCHYARD_STATIC_DIR` / `--static` serves these assets; an explicit directory
remains available for development. Build with `scripts/build-release.sh` to
regenerate Nift output before compiling.

Validation on 2026-10-04: Go build, full Go tests, vet, Nift build and diff check
passed. Tests cover canonical embedded pages, asset byte ranges, HEAD, and an
explicit filesystem override. The standalone binary was started from `/tmp`
without a static-directory override and served the routes recorded in
`embedded-runtime.json`. The measured build is 25,521,581 bytes.

Runtime still requires Git, configured Trestle and Artifacts access, and the
configured token helper. External CLI Agent adapters can add their own runtime
dependencies. Node and Nift are build/development tools, not requirements for
serving the embedded UI. This verifies packaging, not the broader C43 clean
deployment certification or external coding-provider execution.
