# Git authentication decision — C31/C32

Reviewed official Cloudflare documentation on 2026-10-04:

- [Git protocol](https://developers.cloudflare.com/artifacts/api/git-protocol/)
- [Authentication](https://developers.cloudflare.com/artifacts/guides/authentication/)
- [REST API](https://developers.cloudflare.com/artifacts/api/rest-api/)

The documented transport is Git smart HTTP over HTTPS. Repository-scoped
tokens grant read or write access; write includes read. The reviewed API and
authentication documents do not provide an SSH endpoint, user SSH-key registry
or repository-scoped SSH authentication facility. This is a conclusion about
the published interfaces inspected, not a claim about unannounced features.

Decision: use HTTPS with short-lived repository-scoped credentials. Do not
advertise SSH or store SSH public keys while no usable SSH transport exists.
C32 is conditionally not applicable under this decision. Revisit when Cloudflare
documents native SSH, or a separately justified gateway has a bounded design
and actual clone/fetch/push evidence.

| Concern | HTTPS scoped tokens | Switchyard SSH gateway |
| --- | --- | --- |
| Security | Authorize before minting; narrow scope and expiry | Adds public-key identity, host keys, forced commands and another network service |
| Isolation | Existing bounded Git subprocess boundary | Must isolate each SSH session and every Git subprocess |
| Protocol | Use the documented native smart HTTP service | Translate/proxy upload-pack and receive-pack, streaming, failures and backpressure |
| Operations | Existing app and provider credentials | Host-key rotation, key revocation, service patching and abuse controls |
| Audit | Record issuance metadata without secrets | Also record key identity, operation/session outcome and authorization |
| Scaling | Provider handles Git transport | Switchyard handles long-lived connections and repository transfer load |
| Experience | CLI can obtain credentials automatically | Familiar SSH remotes, but requires key setup and operational reliability |
| Competition relevance | Demonstrates native Artifacts integration | Additional infrastructure without evidence it improves the core workflow |

The gateway's operational and security scope outweighs its current benefit.
An SSH placeholder tab would imply a capability the product does not have.
The clone menu therefore offers HTTPS and the Switchyard CLI. C33 supplies the
first-class credential endpoint so ordinary CLI use can avoid manual tokens.

Switchyard never returns its Cloudflare account credential to a caller. Token
issuance must enforce repository permissions, avoid persistence/logging of the
issued secret, use no-store responses, and audit only non-secret metadata.
Git operations must keep tokens out of remote URLs, process arguments and
repository configuration. This is separate from SSH signing-key features,
which are not implemented or claimed here.
