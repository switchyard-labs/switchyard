# C45 immutable repository archives

Canonical GET `/{owner}/{repo}/archive/{ref}.zip` and `.tar.gz`, plus matching `/api/repositories/{owner}/{repo}/archive/...` routes. Refs resolve to a SHA before inventory/blob reads. The Code menu links Download ZIP to the exact displayed SHA.

Archives preserve executable modes and safe relative symlinks. Unsupported Git modes (including submodules), unsafe links, entries beneath symlinks, administrative `.git` paths and duplicate paths fail before response headers. Limits: 8 MiB per blob, 128 MiB total content, 136 MiB staged output, 10,000 files, 1,000 trees, depth 32 and a two-minute request deadline. A single blob is buffered; private 0600 staging is removed on completion/error. Completed files stream to the response. Large repositories exceeding these bounds receive an explicit error.

Full Go tests/vet pass. Tests cover public anonymous access, private/internal denial, authorized access, ZIP/tar contents/modes, pinned blob reads and oversized/unsafe inputs. Route registration is tested after a mux conflict was discovered and fixed. Actual existing disposable Cloudflare repository downloads in both formats match `git ls-tree` and every `git show SHA:path` byte; the browser ZIP matches the HTTP download. See archive-proof.json and code-menu.png.

The archive implementation is deployed on Linode. `sy repo archive` supports ZIP
and tar.gz with atomic no-overwrite downloads. The October 4 comparison in
`exact-browser-server-sy-tree.json` covers browser ZIP, server ZIP/tar.gz and `sy`
ZIP/tar.gz for immutable SHA `e2a22198d24faa843975adef7fe7ffde1b997aa5`.
Every decoded member byte and executable mode matches the six-file Git tree;
ZIP outputs are byte-identical and tar.gz outputs are byte-identical. MIME and
Content-Disposition were verified. The real repository had no symlinks;
symlink preservation/path rejection and public/private/internal authorization
are handler integration-test evidence, not invented live symlink evidence.
Streaming is bounded private-disk staging followed by response streaming, not
an unbounded in-memory archive or progressive output before validation.
C45 is DONE within these documented bounds; larger/submodule archives are
explicitly unsupported.
