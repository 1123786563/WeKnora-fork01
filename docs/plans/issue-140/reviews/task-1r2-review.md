# Task 1R2 independent spec and quality re-review

Reviewed exact HEAD `7223fc076380eae11e03c3e038c5651a57df36ff` (implementation `e6a4ce588ddb393956c2bfe24eff6a5dd20c0cc0`) against BASE `dcb49b344bf051e94f558db2134cadbf45ccbab5`. Sources: approved Career product and main-port specs, ADR 0019, `CONTEXT.md`, Task 1 brief, Task 1 and 1R independent reviews, Task 1R2 plan, implementation report, source and tests. Review was read-only apart from this report.

## Verdict

- **Spec compliance: FAIL, one remaining medium finding.** The definite HTTP classification, original-ID preservation, monotonic `open` projection, and durable-write entry point now satisfy the reviewed invariants. The published `observe` operation still silently does nothing for the default production client; the injected observer is only a port, with no runtime revision source supplied by this implementation.
- **Code quality: FAIL for the same finding.** Focused tests, strict Career type check, and diff hygiene pass. The present observer test checks option forwarding, not revision delivery or the absence-of-source case.

## Finding

**T1R2-M1 — Medium — default production observation silently succeeds without a revision source.** `createCareerApi` defaults `observeScope` to `() => () => undefined` (`packages/api-client/src/career/index.ts:57`), while `careerObserver` is optional on `WeKnoraClientOptions` (`packages/api-client/src/client.ts:61`) and merely forwarded (`:320`). `CareerDesk.observe()` therefore returns normally and registers an inert subscription for the ordinary `createWeKnoraClient({ baseURL, transport })` path. The new test in `packages/api-client/src/client.test.ts` only proves forwarding when an observer is explicitly injected. ADR 0019 requires `observe` to treat revision hints as refetch triggers; the approved main-port plan freezes this operation for Web, Mini and Expo. The implementation report explicitly says the transport has no event endpoint or built-in stream and a deployment must provide one. Without that source, clients that rely on observation keep stale projections until a manual `open`. **Smallest correction:** make an absent observer an explicit unsupported state (or supply a real runtime source), then add an assembled test that emits a hint through the configured source and proves decoded refresh reaches the Desk; test the no-source behavior so it cannot silently regress. Name the actual runtime provider in the assembly/deployment contract before downstream clients depend on observation.

## Prior finding closure

| Task 1R finding | Re-review result |
| --- | --- |
| H1 production 403/409 | **Closed for action transport.** `ApiError` status is translated at the Career API boundary; 409 requires a decoded same-ID conflict receipt. The assembled client test covers 403, 409, timeout and malformed 200 response. |
| H2 rebase after unknown | **Closed.** `rebase` requires an authoritative same-ID conflict observed by the Desk; timeout followed by newer `open` retains the original pending key. |
| M3 late same-scope read | **Closed for `open` and hint-triggered refresh.** `readOpen` retains the highest revision; the reordered-read test rejects a stale command. |
| M4 public persistence bypass | **Closed.** `submit` was removed; public `act` saves before dispatch. |
| M5 observation failure/wiring | **Partially closed.** Refresh rejection is caught and can be reported through `onRefreshError`; configured observer forwarding works. The runtime-source gap is T1R2-M1. |

Original Task 1 strict DTO, receipt correlation, request validation, direct API/Desk assembly, and structured scope fixes remain present in source and passed focused suites. No new regression found in those paths.

## Checks and limits

- `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts packages/api-client/src/client.test.ts`: **pass, 38/38** at exact HEAD.
- `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts`: **pass**.
- `git diff --check dcb49b344bf051e94f558db2134cadbf45ccbab5..7223fc076380eae11e03c3e038c5651a57df36ff`: **pass**.
- Worktree was clean before writing this report. I did not rerun broad `test:shared` or `typecheck:shared`; the Task 1R2 implementation report records 1,143 shared tests passing and only the unchanged `packages/views/src/chat/mermaid.ts:127,158` typecheck baseline failure.
