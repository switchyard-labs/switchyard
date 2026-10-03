Yes — both belong in the Git-host parity layer, and I’d add them as explicit late checkpoints rather than burying them inside repo polish.

For **repository downloads**, the GitHub-like baseline should be a `Download ZIP` action in the Code menu. I’d also make the backend capable of additional archive formats even if the UI initially exposes only ZIP:

```text
/{owner}/{repo}/archive/{ref}.zip
/{owner}/{repo}/archive/{ref}.tar.gz
/{owner}/{repo}/archive/{ref}.tar.xz   maybe later
```

The important behavior is that the archive is generated from the exact requested Git ref/SHA, not from a working directory snapshot. For branches/tags, resolve once to a commit SHA and archive that immutable tree. Archive root should be something predictable like:

```text
repo-name-ref/
```

or:

```text
repo-name-shortsha/
```

I’d support at least:

```text
zip
tar.gz
```

and defer rar/7z unless there is a strong reason. ZIP is the obvious browser download; tar.gz is useful for CLI/Linux users. It would also fit naturally into `sy` later:

```bash
sy repo archive alice/demo-basic --format zip
sy repo archive alice/demo-basic --ref v1.2.0 --format tar.gz
```

For **Releases**, I think Switchyard should have a proper first-class release model rather than just “tags exist.”

Something like:

```text
/{owner}/{repo}/releases
/{owner}/{repo}/releases/new
/{owner}/{repo}/releases/{tag}
```

with:

```text
tag
title
body/Markdown
draft
prerelease
created by
published at
target commit
assets
source archive links
```

The nice GitHub-compatible behavior is:

```text
Release v1.2.0
├─ release notes
├─ switchyard-linux-amd64.tar.gz
├─ switchyard-darwin-arm64.tar.gz
├─ switchyard-windows-amd64.zip
├─ Source code (zip)
└─ Source code (tar.gz)
```

I’d keep the source archives **derived automatically from the tag**, while uploaded/built release assets are separate objects.

There’s also a strong Actions integration opportunity:

```text
tag push
→ Artifacts event
→ Switchyard Action
→ build/test
→ produce assets
→ attach assets to draft release
→ human publishes
```

or allow:

```text
publish automatically when required checks pass
```

if policy permits.

For storage, I would not stuff large binaries into Trestle. Trestle should hold release metadata; release assets should live in something appropriate such as R2/object storage, with checksums and sizes recorded in Trestle. That fits the Cloudflare stack nicely:

```text
Artifacts
  tag / Git source truth

Trestle
  release metadata / state

R2
  release binaries/assets

Workers / Actions
  build + upload orchestration
```

I’d add two checkpoints to Codex’s current continuation:

---

## C45 — repository archives / Download ZIP

Implement first-class immutable repo archive downloads.

Requirements:

```text
Download ZIP in Code popover
archive exact branch/tag/SHA
ZIP
tar.gz
correct Content-Type
correct Content-Disposition
stream response
bounded memory usage
authorization enforced
public/private/internal respected
no .git internals
no filesystem traversal
```

Canonical routes:

```text
/{owner}/{repo}/archive/{ref}.zip
/{owner}/{repo}/archive/{ref}.tar.gz
```

or equivalent.

Resolve mutable ref once:

```text
ref → commit SHA → archive tree
```

and record that SHA in response metadata/filename where useful.

Browser tests:

```text
public repo anonymous download works
private repo anonymous download denied
authorized private download works
ZIP opens
tar.gz opens
contents match Git tree
symlinks handled safely
no .git directory
```

Add:

```text
Code ▾
  HTTPS
  sy CLI
  Download ZIP
```

If SSH is deferred, don’t show a fake SSH tab.

Also document API for later `sy repo archive`.

Commit C45.

---

## C46 — Releases

Implement first-class repository releases.

Data model:

```text
Release
  id
  repo
  tag
  target_sha
  title
  body
  draft
  prerelease
  author
  created_at
  published_at
```

Release assets:

```text
ReleaseAsset
  id
  release_id
  name
  size
  content_type
  checksum
  storage_key
  created_at
```

Use:

```text
Artifacts
  Git/tag/source truth

Trestle
  metadata/state

R2
  binary assets
```

Do not store release binaries in Trestle.

UI:

```text
/{owner}/{repo}/releases
/{owner}/{repo}/releases/new
/{owner}/{repo}/releases/{tag}
```

Repository navigation should expose Releases naturally, likely near:

```text
Code
Issues/Work
Pull requests
Actions
Releases
```

or in the repo sidebar/header where it fits best.

Release page:

```text
title
tag
commit
published by
date
Markdown notes
assets
checksums
source ZIP
source tar.gz
```

Support:

```text
draft
prerelease
publish
edit
delete draft
```

Be cautious with deleting published releases/assets; audit mutations.

Tag behavior:

```text
release tag must resolve to real Git object
published release stores immutable target SHA
moving/deleting tag later does not silently rewrite release identity
```

Actions integration:

```text
tag push
→ event subscription
→ Actions
→ builds
→ upload assets to R2
→ attach to release
```

Support checksums:

```text
SHA256
```

for every uploaded/generated asset.

Authorization:

```text
read public release
create/edit/publish only authorized repo maintainers
asset upload permission checked server-side
```

Browser and API tests required.

Commit C46.

---

Then I’d extend the final `sy` review with:

```text
sy release list
sy release view <tag>
sy release create <tag>
sy release upload <tag> file...
sy release download <tag>
sy repo archive --ref <ref> --format zip|tar.gz
```

Probably not necessary for the first `sy` update if time is tight, but the API should be designed so those commands are straightforward.

And releases give us another very good competition/demo story: an Agent or human tags a release, Cloudflare Actions builds cross-platform artifacts, R2 stores them, Switchyard shows checksums and release notes, and users can download both binaries and immutable source archives.