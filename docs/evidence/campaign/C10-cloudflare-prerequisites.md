# C10 Cloudflare prerequisites — 2026-10-03

C9 is committed as `6d744c5`. C10 is not implemented or certified. This is account/API reconnaissance, not a successful CI run.

## Live read-only capability checks

Account: `b7f20353ee8a9e5d2003f52c74ba795e`; Artifacts namespace: `switchyard-cp0`.
Tokens were loaded in process memory from the existing configured API credential and remote OAuth helper, never printed or saved. Results:

| Credential | Resource | HTTP | Result |
| --- | --- | --- | --- |
| Configured API token | Workers scripts | 200 | Accessible; zero Workers |
| Configured API token | Workflows | 200 | Accessible; zero Workflows |
| Configured API token | Builds tokens | 401 | Code 12006, Invalid token |
| Configured API token | R2 buckets | 403 | Code 10042, enable R2 through dashboard |
| Existing Artifacts OAuth | Workers scripts | 200 | Accessible; zero Workers |
| Existing Artifacts OAuth | Workflows | 200 | Accessible; zero Workflows |
| Existing Artifacts OAuth | Builds tokens | 200 | Accessible; zero build tokens |
| Existing Artifacts OAuth | R2 buckets | 403 | Code 10042, enable R2 through dashboard |

No resource was created by these checks. A separate bounded repository-connection probe used `provider_type: artifacts`, the existing namespace and a uniquely named nonexistent test repository (`codex-actions-probe-20261003`). It returned HTTP 400 / 12002, Invalid request body. No connection was created. This does not prove Artifacts Builds is unavailable: that provider value is not in the published connection API contract, and the error does not identify which field failed. Do not infer a supported request shape or label this an upstream defect without further evidence.

## Verified contracts and proposed route

- [Artifacts custom CI guide](https://developers.cloudflare.com/artifacts/guides/build-and-deploy-on-push/), updated October 1, describes Workflows with `@cloudflare/ci`, Sandbox runners and an R2 snapshot bucket.
- Official npm `@cloudflare/ci` version **0.2.0** was inspected in temporary storage. Its `Bindings` requires `BACKUP_BUCKET`, bucket name and R2 access credentials. `CIWorkflow` documents a durable workspace snapshot for every runner, and the runner calls `sandbox.createBackup` even without dependency caching. Disabling cache does not remove the R2 prerequisite.
- [Official SDK source](https://github.com/cloudflare/ci) also states runner logs are raw and not secret-redacted. Switchyard must not expose those as safely redacted or inject deploy credentials into untrusted runner commands.
- [Workers Builds API guide](https://developers.cloudflare.com/workers/ci-cd/builds/api-reference/) requires user-scoped API authorization; an API token differs from the persistent build token registered for deployment. Existing OAuth can list Builds tokens, but no persistent build token is currently registered. A short-lived OAuth token must not be substituted as a durable build secret.
- [Artifacts Builds integration](https://developers.cloudflare.com/workers/ci-cd/builds/git-integration/artifacts-integration/) documents dashboard connection, `main` production and non-production Worker Previews. [Published connection API](https://developers.cloudflare.com/api/resources/workers_builds/subresources/repos/subresources/connections/methods/upsert/) currently enumerates GitHub/GitLab/Origin rather than Artifacts; use the actual supported integration contract instead of inventing another enum.
- [Build logs API](https://developers.cloudflare.com/api/resources/workers_builds/subresources/builds/subresources/logs/methods/get/) returns `cursor`, timestamp/message lines and `truncated`. C13 must preserve those boundaries and page upstream rather than putting unlimited logs in Trestle.

Preferred next step: enable R2 for this account through the dashboard, then verify buckets and scoped R2 credentials, deploy a uniquely named disposable Worker/Workflow/Sandbox pipeline, prove exact-SHA checkout and callbacks, and remove test resources afterward. Paid service enrollment/billing acceptance is an account-owner action; the campaign's bounded test-resource authorization is not treated as authority to accept billing terms.

Alternative: an account-owner-configured Artifacts Workers Builds connection with a persistent deploy-scoped build token can support the documented standard build route. It does not by itself establish the custom Actions execution architecture.

No shared Linode changes, demo Git pushes, Workers deployments, subscriptions, billing changes or reference repository edits were made. C10–C25, including all requested visual redesign and Warden/Gantry editor adaptation, remain pending. The R2 setup request is pending with Nick; elapsed time is not approval or completion.

## Prerequisite resolved and C10 execution evidence

The material above records the earlier blocked investigation. Nick enabled R2 and explicitly authorized provisioning the existing API/R2 credentials as encrypted secrets on `switchyard-actions-probe-20261003`. R2 is accessible. The disposable Worker, Workflow, one-instance Sandbox application and snapshot bucket were deployed; no shared Linode service or project Git remote was changed.

A signed manual dispatch passed a meaningful Node test and asserted checkout SHA `213c6fa8adf42b443730a555d3b645ae6fe69873`. A subsequent **native Artifacts push**, without a dispatch request, passed at SHA `b4e3fe7f7d3e76931b0f7a9dbdabc248c10cb402`, Workflow `11a042fc-df57-4acb-b9d4-4ffccff88fbc`, definition revision `7a140063092a5bbb0bc9f98fce4289611e87a96fec5a9cee6ebb7edd8cad6a72`. Its [status](C10/native-status.json) and [captured logs](C10/native-logs.json) are sanitized evidence.

Real execution caught two issues before certification: R2 write results cannot be returned from durable steps, and the SDK's workspace intentionally excludes `.git` (the source checkout is `/tmp/ci-source`). Both were corrected and the run repeated. An event receipt test caught first-delivery variable shadowing; the regression now verifies failed publication does not advance observation and retry preserves the exact envelope. Full `go test -race ./...`, Worker TypeScript check and all four Worker protocol tests passed. Credentials were neither logged nor committed. The test resources remain available for C11–C25 and will be cleaned up after the end-to-end gate.

C10 establishes the real custom CI path and its architecture. Trestle normalization/check gates are C11; UI/log viewer/security/deployment gates are C12–C16. Standard Workers Builds has no configured connection/deployment token and remains an explicit provider-setup limitation; no successful Builds preview/deployment is claimed.
