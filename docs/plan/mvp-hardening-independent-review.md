# Post-fix independent review handover

Review the approved bounded CP1–CP7 hardening, not a platform redesign. No pushes,
tags or releases have been performed. Primary code/test target:

- Switchyard: `a7154d13de4f0397c12db8bb34254c1b8f12428f`.
- Brochure source stage: `47ef33f5f833595cb58b22302a37d45a64d1c32a`.
- Brochure output main: `5e1134b45ccf5371860f9b072a44a248fdf9f919`.
- Unchanged sy: `85f768347ec16c9226ae4e77445b7ea212785c6c`.
- Unchanged Trestle: `4ceac7a18d46e6355da235655077b5b891758e40`.

Final rollout/readiness documentation is a documentation-only descendant of that
Switchyard checkpoint. See `docs/operations/mvp-hardening-readiness.md` and
`docs/evidence/mvp-hardening` in the final checkout for deployment evidence.

Answer these questions with concrete file/line evidence and minimal reproductions:

1. Are F1–F5 resolved, particularly organisation fleet intersection, immutable
   queue/review source identity, provenance convergence, publication policy and
   honest worker-capacity claims?
2. Do Proposal/Pages browser mutations pass the origin boundary through the actual
   mux? Does production cookie transition reject legacy-name fixation/ambiguity?
3. Can source B publish under source A's queue/check/review identity? Can an Agent
   bypass current canonical/protected-ref policy through workflow/recovery calls?
4. Does durable intent -> Git -> remote observation -> stable fact -> completion
   survive the tested crash/ambiguous-result boundaries? Inspect conflicting
   receipt bodies, policy revocation, missing scratch and bounded queue retries.
5. Do Actions ref admission, release DTOs, reconciliation cursors, semantic bounds,
   Pages reserved names and throttle saturation introduce any new security bug?
6. Do corrected brochure/README claims match code and evidence? Are any genuine
   release blockers left, as distinct from documented MVP limitations?

Useful starting points: `internal/app/request_security.go`,
`internal/app/publication_authority.go`, `internal/refs/provenance.go`, queue/review
handlers and migrations; medium-hardening tests; Actions/Pages Worker entrypoints.

Do not assume source enqueue is independent review approval. Do not reinterpret
historical unpinned records, treat every HTTP 409 as success, or demand distributed
scratch recovery, production-scale Agent throughput, new semantics, PostgreSQL,
mail queues, full git-archive compatibility or response-signing infrastructure.
Report Critical/High blockers first; distinguish fixed findings, residual bounded
limitations and new regressions. No edits/deployments/pushes are requested by this
handover. Recommend release-readiness only within the working prototype/MVP scope.
