# Task 1R3 independent spec and quality review

Reviewed exact HEAD `108204431dbbf1d3c11b54a2306487a74cf2e827` (implementation `e2040f7703c8234724fc9ad78f0080d3c8430f64`) against BASE `7e209f5867f63c4b559e20e4b76e1773808252b3`. Sources: approved Career spec and main-port plan, ADR 0019, `CONTEXT.md`, Task 1 brief, all prior Task 1/1R/1R2 independent reviews, Task 1R3 hardening plan, implementation report, source and tests. Review was read-only except for this report; OCR was not invoked.

## Verdict

- **Spec compliance: FAIL, one medium finding.** R3 satisfies the specific missing-observer correction: ordinary client/Desk assembly throws the exported `CareerObservationUnavailableError` immediately. A configured hint causes CareerApi to fetch and strictly decode a Desk envelope before the Desk publishes its projection; disposal calls the provider's unsubscribe. ADR 0019 now documents the runtime-supplied provider and explicit `open()` refresh fallback. The remaining finding allows an observation subscription outside the authenticated client's deployment scope, contrary to the shared scope boundary required for cross-client state.
- **Code quality: FAIL for the same finding.** Focused suites, strict Career type checking, and diff hygiene pass, but the assembled observer test does not exercise a mismatched deployment scope. Prior strict DTO, receipt, recovery, monotonic revision, and scope-switch fixes remain present; no regression was found in those reviewed paths.

## Finding

**T1R3-M1 — Medium — configured observation bypasses the client's deployment-origin guard.** `packages/api-client/src/client.ts:317-322` wraps Career requests with a check that `input.scope.deploymentOrigin` equals `new URL(options.baseURL).origin`, then passes `options.careerObserver` directly to `createCareerApi`. `packages/api-client/src/career/index.ts:98-101` validates only the shape of the observation scope before invoking that provider. On exact HEAD, a client with `baseURL: 'https://a.example'` accepted `client.career.observe({ deploymentOrigin: 'https://b.example', tenantId: 't', actorId: 'a' }, ...)` and invoked the observer with `https://b.example`; its HTTP paths reject the same mismatch. A runtime provider that trusts the client-supplied scope could establish a cross-deployment revision subscription, exposing activity hints or triggering refreshes under an unintended identity. **Smallest correction:** apply the same authenticated base-origin check to the observer adapter before invoking the provider, with an assembled test asserting a mismatched scope never reaches it. The provider must still enforce its own authorization for tenant and actor.

## R3 acceptance and prior finding status

| Item | Result |
| --- | --- |
| Absent provider | **Pass.** `desk.observe()` synchronously throws `CareerObservationUnavailableError`; no inert subscription is returned. |
| Configured hint → authority | **Pass.** The assembled test drives a hint through the injected provider, fetches through `createWeKnoraClient`/CareerApi, and updates Desk revision 1 → 2 from the decoded response. Existing strict decoder tests reject malformed envelopes. |
| Unsubscribe/dispose | **Pass.** Desk disposal invokes the provider unsubscribe, invalidates the projection and generation, and late hints cannot publish. The test asserts unsubscribe was called. |
| Runtime/fallback contract | **Pass as an explicit integration contract.** ADR 0019 assigns `careerObserver` to the runtime and requires positive hints and unsubscribe; the current transport has no event source. An unconfigured consumer must call `open()` explicitly or disable observation-dependent freshness. Platform adapters remain downstream work, not evidence of a built-in stream. |
| Prior Task 1/1R/1R2 findings | **No regression observed.** Response/request parsing, typed 403/409 outcomes, same-ID unknown recovery, monotonic read projection, and durable act entry remain in source and focused tests. |

## Checks and limits

- `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts packages/api-client/src/client.test.ts`: **pass, 40/40** at exact HEAD.
- Strict `tsc --noEmit` over contracts, API client, and career-core entry points: **pass**.
- `git diff --check 7e209f5867f63c4b559e20e4b76e1773808252b3..108204431dbbf1d3c11b54a2306487a74cf2e827`: **pass**.
- Read-only `tsx -e` probe reproduced T1R3-M1: the observer received `https://b.example` from a client based at `https://a.example`.
- I did not rerun the broad shared suite or shared type check. The implementation report records 1,145 shared tests passed, 4 skipped, and only the unchanged `packages/views/src/chat/mermaid.ts` shared type-check baseline failure.
