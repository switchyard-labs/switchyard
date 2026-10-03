# C16 — Cloudflare competition alignment

Checked against current primary sources on 3 October 2026. This is preparation and architecture review; no application, upload, terms acceptance or project push has occurred.

## Entry prerequisites

The [challenge page](https://www.cloudflare.com/git-competition/) asks for concurrent Agents, a 5–10 minute demonstration, open source code and runnable instructions by 14 October. The [announcement](https://blog.cloudflare.com/next-git-platform-on-cloudflare/) explicitly positions Workers and Artifacts as the foundation, with coordination, context, review and conflict handling as opportunities for a new product. Switchyard should demonstrate competing Attempts and a deliberate integration decision, rather than concentrate its video on a familiar repository browser.

The [official rules](https://www.cloudflare.com/documents/build-next-gen-git-platform-competition-terms.pdf), sections 2–6, limit entrants to legal US/Canadian residents aged at least 18, require Workers plus Artifacts and concurrent Agents, and require MIT, Apache 2.0 or BSD source with a LICENSE. The deadline is 14 October 2026 at 11:59 PM PDT. Judging weights are prototype originality/quality 50%, multi-Agent collaboration 25%, and usability 25%. Automated entry is prohibited. The maintainer must resolve eligibility before submission; an Australian timezone or address does not establish eligible residency. Rights in third-party code/artwork and the promotional grants in section 9 also require the entrant's review. No eligibility or acceptance is assumed.

The [submission form](https://www.cloudflare.com/git-competition/submit/) requests team/contact/location/attendee details, project vision, Cloudflare usage, source URL and run instructions. It accepts MP4/WebM/MOV videos up to 2 GiB and asks for confirmations about Workers/Artifacts and terms. Prepare those materials locally; the entrant submits them manually after reviewing the rules. The repository's existing MIT LICENSE satisfies the license format prerequisite; complete third-party notices and source availability still need final review.

## Architecture and evidence

| Responsibility | Current implementation | Evidence / remaining gate |
| --- | --- | --- |
| Git authority | Artifacts repositories, immutable commits, normal Git, scoped tokens | CP0 and C10 real Artifacts round trips; C25 full publication dogfood pending |
| Coordination | Trestle Work, Attempts, reviews, workflow/queue state, immutable receipts | C0–C9 tests and crash/replay evidence; single-node deployment only |
| Cloud execution | Native push → Worker → Workflows → official CI Sandbox → R2 captures | C10 native push, C13 actual logs, C14 manual/rerun/cancel and C15 credential probe |
| Integration policy | Exact-SHA/definition Actions checks fence PR and queue decisions | C11 required-check tests; C25 complete multi-Agent narrative pending |
| Human interaction | Repository browser, editor, Work, review and attention | Current Actions pages verified; C17–C22 comprehensive design/editor work pending |
| Worker previews/deployments | Verified Workers Builds setup required | No configured connection/build token; no successful preview/deployment claim |

The [Artifacts documentation](https://developers.cloudflare.com/artifacts/) establishes the Git substrate. The [custom CI guide](https://developers.cloudflare.com/artifacts/guides/build-and-deploy-on-push/) supports the implemented event/Workflow/SDK path and distinguishes it from standard Workers Builds. Cloudflare therefore owns execution, checkout and snapshot/capture storage, in addition to Git; it is not merely a logo or remote host. The [Builds API reference](https://developers.cloudflare.com/workers/ci-cd/builds/api-reference/) distinguishes user-scoped control API tokens from deployment build-token UUIDs and requires Worker tags/trigger IDs. The existing token's failed capability check is a concrete prerequisite, not grounds to invent a connection provider or deployment URL.

## Demonstration gate

C25 must show multiple Agents acting concurrently on one real Work item, distinct Attempts/commits, actual Cloudflare checks and logs, review evidence, semantic/text conflict handling, a human decision and serialized exact-candidate integration into Artifacts. Show a crash/retry recovering without duplicate effects and an external Git edit causing a stale decision to be rejected. Show the editor preserving a newer human draft when an Agent proposal is stale. Use the actual application and public source/run instructions; identify deterministic fixtures and unconfigured deployment paths honestly.

A recorded video, complete public source including dependency notices, deploy/restore instructions and any verified preview still remain deliverables. C16 passes architecture alignment; it does not certify submission readiness or eligibility.
