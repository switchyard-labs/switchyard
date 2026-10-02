# Public demo environment

Switchyard supports `SWITCHYARD_DEMO_MODE=true` for a dedicated staging/demo deployment.

## Security contract

- unauthenticated GET/HEAD API requests become a **demo guest**;
- demo guest repository listings contain only Switchyard metadata marked `public`;
- legacy flat Artifacts repository APIs are disabled for the demo guest, avoiding leakage of checkpoint/private repo names;
- unauthenticated mutation APIs return `403 demo_read_only` (auth endpoints remain available for deliberately provisioned demo accounts);
- browser receives no Artifacts/Trestle administrative credential;
- clone URLs contain no embedded token;
- the UI shows an explicit demo banner;
- demo infrastructure must expose the Switchyard control plane through HTTPS, never Trestle admin directly.

## Deployment gate

A clean browser must be able to browse curated public demos without logging in, while POST/PATCH/DELETE attempts fail. Rate limits, reset cadence, HTTPS and abuse controls belong to the deployment layer and must be certified before exposing a public URL.
