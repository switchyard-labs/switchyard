# C37 — Cloudflare execution decision and remaining preview prerequisite

The generic repository path is certified by C36: an ordinary Artifacts push launches
native CI, the deployed Worker verifies the immutable source through its Artifacts
binding, Workflows executes the official CI Sandbox, and R2 retains state/captures.
Go imports exact-source checks into Trestle; the Integration Queue consumes checks
at its existing enqueue/candidate/publication gates. C11 fault tests prove stale SHA,
changed definition and newer pending runs cannot reuse an earlier success. Final
full-story queue dogfood is separately outstanding; it is not inferred from CI success.

Use Workflows and `@cloudflare/ci` for generic CI. Workers Builds is a separate
option for deploying Worker projects, not a second mandatory executor for every repo.

Official documentation inspected on October 4:

- [Artifacts Builds connection](https://developers.cloudflare.com/workers/ci-cd/builds/git-integration/artifacts-integration/): configure the namespace/repository connection and Worker build settings.
- [Worker Previews](https://developers.cloudflare.com/workers/previews/get-started/): requires Wrangler 4.135.0 or later and preview configuration. The campaign pins 4.118.0.
- [Builds API](https://developers.cloudflare.com/workers/ci-cd/builds/api-reference/): deployment requires a configured repository connection, Worker tag, trigger and deployment build-token UUID.

A read-only request for the disposable Worker's Builds triggers returned HTTP 200
with zero triggers. No configured Builds connection or dedicated deployment-scoped
token exists in this campaign. The general account operator token must not be copied
into repository CI as a substitute. No preview URL has been manufactured, and the
application correctly reports deployments disabled.

Preview certification remains CONDITIONAL on genuine scoped deployment setup.
Once supplied: configure a disposable Worker project and supported connection,
upgrade/pin Wrangler deliberately, push an exact commit, wait for the provider's
build result, validate the returned URL and source SHA, import its provenance, and
verify the UI link. Until that succeeds, Worker previews/deployments are PARTIAL.
The generic Actions path is PASS; these distinct statuses must remain visible in
competition documentation.

Evidence: [C36](../C36/README.md), [C38](../C38/README.md),
[architecture](../../../architecture/cloudflare-actions.md).
