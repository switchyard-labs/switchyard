# Switchyard Product Information Architecture — PX1

Status: PASS

## Principle

The simple case should read like a familiar Git host. Switchyard-specific machinery appears only where it adds real value.

A developer who knows GitHub/GitLab/Bitbucket should be able to answer these immediately:

- where am I?
- who owns this repository?
- what branch/ref am I looking at?
- where are the files/commits/PRs/settings?
- what work is active?
- what requires my attention?

## Global authenticated navigation

Primary destinations:

```text
Dashboard
Repositories
Organizations
Work
Pull Requests
Workflows
Needs Attention
```

Secondary/account destinations live in the account menu or full-screen navigation:

```text
Your profile
Your repositories
Your organizations
Settings
Credentials
Sign out
```

Do not promote Attempts, Review Findings, Provenance, raw queue items, or Agent executions to permanent global navigation. They are contextual details of Work, PRs, workflows and Needs Attention.

## Repository shell

Repository header identity:

```text
[owner avatar] owner / repository
visibility · description
branch/ref selector · clone
```

Repository-local tabs:

```text
Code
Work
Pull Requests
Workflows
Activity
Settings
```

Contextual data within those tabs may surface:

```text
Attempts
Agents
Findings
Checks
Provenance
Integration Queue state
Needs Attention
```

without forcing users to understand those concepts to browse ordinary Git.

## User profile

Canonical route:

```text
/{username}
```

Primary sections:

```text
Overview
Repositories
Activity
```

Future profile content should support a large avatar, display name, username, bio/location/site, profile README, pinned repositories and contribution/activity summaries.

## Organization profile

Canonical route:

```text
/{org}
```

Primary sections:

```text
Overview
Repositories
People
Teams
```

Organization settings remain contextual:

```text
/{org}/settings/...
```

## Repository URL model

Canonical repository identity:

```text
/{owner}/{repo}
```

Recommended routes:

```text
/{owner}/{repo}
/{owner}/{repo}/tree/{ref}/{path...}
/{owner}/{repo}/blob/{ref}/{path...}
/{owner}/{repo}/commits/{ref}
/{owner}/{repo}/commit/{sha}
/{owner}/{repo}/branches
/{owner}/{repo}/tags
/{owner}/{repo}/work
/{owner}/{repo}/work/{id}
/{owner}/{repo}/pulls
/{owner}/{repo}/pull/{id}
/{owner}/{repo}/workflows
/{owner}/{repo}/workflow/{id}
/{owner}/{repo}/activity
/{owner}/{repo}/settings
/{owner}/{repo}/settings/access
/{owner}/{repo}/settings/branches
/{owner}/{repo}/settings/integrations
```

The exact implementation can use static-shell fallbacks plus client routing initially, but public product URLs should not expose physical Artifacts repository identifiers.

## Owner-route disambiguation

`/{owner}` can identify either a user or organization. Usernames and organization slugs therefore share one top-level namespace and MUST be unique across both identity types.

`/{owner}/{repo}` resolves through Switchyard repository metadata, not by blindly concatenating the path and querying Artifacts.

## Compatibility routes

Existing prototype routes remain temporarily supported during migration:

```text
/repo.html?name=<artifact-name>&ref=<ref>&path=<path>
/work.html
/edit.html?name=<artifact-name>&ref=<ref>&path=<path>
```

They are compatibility routes, not the future canonical product IA.

## Full-screen global navigation

Both desktop and mobile may use the same full-screen menu interaction. Suggested groups:

```text
Explore
  Dashboard
  Repositories
  Organizations

Collaborate
  Work
  Pull Requests
  Workflows
  Needs Attention

Account
  Profile
  Settings
  Credentials
```

Long groups use collapsible headings. The current owner/repository context remains visible while the menu is open.

## State requirements

Every first-class destination requires:

- populated state;
- empty state;
- loading state;
- error state;
- permission-denied state;
- desktop behavior;
- narrow/mobile behavior;
- keyboard/focus behavior.

## Familiarity decisions

Keep established terms when semantics are genuinely Git/Git-host concepts:

```text
repository
branch
commit
Pull Request
review
check
settings
organization
team
```

Keep Switchyard terms only where the concept is actually new:

```text
Work
Attempt
Review Finding
Integration Queue
Needs Attention
Provenance
```

## PX1 gate result

The route map and navigation model support:

1. ordinary one-human/no-Agent Git hosting;
2. user and organization ownership;
3. repository-local collaboration;
4. progressive disclosure of agent-native state;
5. future settings/profile surfaces without leaking backend physical names into public URLs.
