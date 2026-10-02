# Repository ownership and namespace migration — PX2

Status: PASS (implementation/build/unit gate in supplied workspace; live Artifacts/Trestle registration must be re-run when deployed).

## Decision

Switchyard public identity is independent of physical Git storage identity.

```text
public:      switchyard-labs/switchyard
stable id:   repo_<id>
backend:     <Artifacts repository identifier/name>
```

A rename or transfer changes owner/slug metadata and public routing; it does not inherently require rewriting Git history or renaming physical storage.

## New coordination records

### owner_namespaces

```text
slug (globally unique across users + orgs)
owner_type
owner_id
created_at
```

### repository_meta

```text
id
full_name (unique owner/slug)
owner_type
owner_id
owner_slug
slug
display_name
description
visibility
default_branch
artifact_name (unique physical backing repository)
created_at
updated_at
```

The pre-existing flat `repos` collection remains as compatibility/reconciliation history. No destructive migration is performed.

## Registration

`POST /api/repositories/register` explicitly binds an existing Artifacts repository to either:

- the authenticated user's owner namespace; or
- an organization owned by the authenticated user.

This deliberately does not bulk-assign the 60+ checkpoint fixture repositories to a user. Curated demos and real repositories can be registered intentionally.

## Canonical API/read paths

```text
GET /api/repositories
POST /api/repositories/register
GET /api/repositories/{owner}/{repo}
GET /api/repositories/{owner}/{repo}/tree?ref=...
GET /api/repositories/{owner}/{repo}/content?ref=...&path=...
```

Legacy `/api/repos/{artifact-name}` remains available during migration.

## Canonical browser paths

The server recognizes:

```text
/{owner}/{repo}
/{owner}/{repo}/blob/{ref}/{path...}
/{owner}/{repo}/tree/{ref}/{path...}
```

and serves the repository shell. The browser resolves the public identity to its physical `artifact_name` through metadata. Existing `repo.html?name=...` links remain compatibility paths.

## Collision rules

Usernames and organization slugs share the same top-level namespace. New user/org creation therefore claims an `owner_namespaces` record and fails on collision.

## Migration safety

- no Artifacts repository is renamed or deleted;
- no `.git` history is rewritten;
- checkpoint fixture repos remain available under legacy physical access until intentionally registered;
- provenance may continue to carry existing physical repo references while later work introduces stable repository ids more broadly;
- future transfers can update repository metadata without changing physical Git storage.
