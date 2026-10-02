# Security and credentials

## Distinct concepts

- **Provider credential** — the secret used to authenticate to an external
  model/agent provider. BYOK: users configure their own.
- **Provider profile** — non-secret configuration (provider, model, role,
  instructions, capabilities, budget, timeout).
- **Agent role** — intent + capability contract; what workflows/panels reference.
- **Switchyard capability** — what an Agent execution may do inside Switchyard
  (write files, create branches, push, open PRs, review, spawn, queue, merge).
- **Execution** — one invocation.

**Permissions ≠ credentials.** A provider key says "Switchyard may call this
external service." It says nothing about the Agent's authority inside Switchyard,
which is enforced independently by role/capability policy.

## Credential scopes

- **v1: personal + organization credentials.** No repository-credential object
  unless live evidence proves a real need.
- Repository boundaries are expressed in **roles + policy**
  (`role: payments-security-reviewer · credential: org/deepseek-prod · allowed
  repositories: payments/*`).
- Individual-owned repo with its own provider account / OSS maintainers BYOK →
  modeled as a personal credential inside a repo-scoped role.
- Default provider/model per user; repository overrides; workflow-required role;
  fallback provider; quotas/budgets — configuration, not new object types.

## SecretStore architectural contract

```
put
resolve-for-execution
replace/rotate
delete
metadata/audit
```

- **Plaintext exists only in the narrow execution-resolution path.**
- Never expose stored plaintext through normal UI/API — the UI shows metadata
  only ("DeepSeek credential · •••••••••••• · Added 3 Oct · Last used 2m ago").
- Secrets never appear in Git, commits, branches, PR bodies, workflow
  definitions, logs, provenance, artifacts, or frontend HTML.
- Concrete storage backend (app-encrypted ciphertext in the control-plane DB vs a
  cloud secret facility vs a local store) is **deferred** — decide per deployment
  (self-hosted / hosted / enterprise) when needed. Do not design two production
  backends before one is needed.

## Secret lifecycle and delivery

- Secrets are encrypted at rest; keys owned by the deployment's control plane.
- Resolution happens only at execution time for a **permitted role call**; the
  resolved credential is delivered to the executor over a secure, scoped,
  expiring channel (in-memory or a 0600 file for the run), passed to the agent
  CLI, never persisted in run logs.
- **Child agents inherit reduced or no credentials** (capability-reduced
  principals).
- Audit events record metadata (who/which role/when/which run), never the secret.
- Rotation/replace/delete; no plaintext retrieval after creation.
- A single configured credential may back interactive, implementation, review,
  conflict, and workflow Agents **through separate roles** — the secret is never
  handed around; roles reference it by opaque id.

## Provider connections are not all API keys

A "provider connection / credential material" may be an API key, OAuth, a local
CLI login, a service account, an external agent runner, a local model socket, or
short-lived tokens. The simple API-key UX must be excellent, but the model must
not assume `credential = string API key`. Local/self-hosted Agents may need no
provider credential at all.

## Editor/Agent security

- The Agent panel receives explicit, permission-visible context (repo, branch,
  open file, selection, Work, PR, findings, diff).
- Concurrent-work awareness is lightweight warnings; no collaborative
  multi-agent editing.
- Editor Agent actions are constrained by the same role capabilities as
  automated hooks.