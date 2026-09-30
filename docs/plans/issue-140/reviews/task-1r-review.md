# Task 1R independent spec and quality review

Reviewed exact HEAD `ac2b7fe92734d2c8a5df72ff11798a9290864818` (implementation `a1be4d51b1dbf501d14de3c17c9eaa297b86a4a9`, followed by evidence commit) against BASE `ce7e2f09268271646c2e99b77e9c4d4c03929a71`. Scope: `BASE..HEAD`, with source and tests read directly. Sources: approved Career product and main-port specs, ADR 0019, `CONTEXT.md`, main implementation plan, Task 1 brief, Task 1 independent review, and Task 1R hardening plan. The implementation report was treated as a claim to verify, not review evidence by itself.

## Verdict

- **Spec compliance: FAIL.** Strict DTO/envelope parsing, the broad Career API, structured scope, and direct API/Desk assembly substantially close original findings H1–H3 and M6. H4 remains open: definite forbidden/conflict failures are not classified, and unknown requests can lose their original recovery key. Read projection can also regress within one scope. These violate the approved unknown-outcome, revision, and fail-closed invariants.
- **Code quality: FAIL.** Focused tests and strict type checking pass, but the paths below are untested and materially affect correctness and data consistency. Do not release Task 1R as the frozen Desk boundary yet.

## Findings

1. **High — definite HTTP 403/409 is reported as an unknown write.** `packages/career-core/src/index.ts:49-50,110-116` catches *every* `remote.act`/`lookup` rejection and creates an `unknown` receipt. The production API client throws `ApiError` for non-2xx responses (`packages/api-client/src/client.ts:215-220`); `CareerApi` passes that rejection through (`packages/api-client/src/career/index.ts:66-67,74-75`). Thus a definite 403 leaves an intent pending instead of returning terminal `forbidden`, and 409 cannot present the conflict envelope for explicit rebase. A `success:false` Career response is likewise rejected by the strict decoder and then converted to `unknown`. The test uses a mock that directly returns `{kind:'forbidden'}`, so it does not exercise production HTTP error behavior. **Smallest correction:** translate definitive authenticated transport/status responses into typed forbidden/conflict outcomes at the Career API boundary, preserving request-ID and decoded revision correlation; only indeterminate failures become unknown. Test the assembled `createWeKnoraClient`/CareerApi/Desk path for 403, 409, timeout, and malformed responses.

2. **High — rebase can discard an unresolved original request ID.** `packages/career-core/src/index.ts:126-149` permits `rebase` for any stored intent with a newer revision; it never checks that the original request has a confirmed conflict outcome. It saves a new key and removes the original at line 147. After an `act` timeout or `lookup` failure, a subsequent `open` at a newer revision permits this path, although the original write may already have applied. The new intent may then duplicate the operation and the original same-ID lookup is lost. This violates the approved unknown-create recovery rule. **Smallest correction:** retain durable outcome state or require an authoritative same-ID conflict receipt before rebase; do not remove an unresolved original. Add a timeout → newer open → attempted rebase test and ensure original lookup remains available.

3. **Medium — late same-scope reads can regress the authoritative projection.** `packages/career-core/src/index.ts:64-70` assigns every completed `open()` result to `projection`, with only a generation check and no revision ordering. Two same-scope reads can resolve revision 3 then revision 2; the later revision 2 overwrites 3. `act()` then accepts revision 2 as current (`:91-97`), contrary to revision monotonicity. The deferred tests cover scope changes, not out-of-order reads within one scope. **Smallest correction:** keep the highest decoded revision for a scope (and prevent older read completion from replacing it); test reordered open/observe responses and stale-write rejection.

4. **Medium — public `submit` bypasses durable intent persistence.** `packages/career-core/src/index.ts:99` calls `send(intent, false)`, while `send` only saves when `persist` is true (`:42-47`). A caller can dispatch a new intent through this exported operation without ever persisting its request ID; timeout recovery then has nothing to list. ADR 0019 requires persistence before sending every write. **Smallest correction:** remove this public bypass, or make it a recovery-only operation that verifies the same ID exists in the scoped store before sending; test timeout recovery through that entry point.

5. **Medium — observation refresh rejection is unhandled.** `packages/career-core/src/index.ts:152-157` calls `void readOpen()` from the revision-hint callback. On a network/decoder failure or a scope switch during refresh, `readOpen()` rejects without a handler. The production `createWeKnoraClient` also supplies no observer to `createCareerApi` (`packages/api-client/src/client.ts:313-318`), so its default observer is a no-op (`packages/api-client/src/career/index.ts:56`). **Smallest correction:** handle/report refresh failure without an unhandled rejection, and wire a real revision-hint adapter when observation is part of the frozen client contract; test failure and scope-switch hints through the assembled API/Desk.

## Original finding closure

| Original | Review result |
| --- | --- |
| H1 failed/malformed envelopes | **Closed for data acceptance.** Exact success envelope and receipt correlation are implemented; definitive failure outcome handling remains in finding 1. |
| H2 application snapshot validation | **Closed.** Canonical parser requires nonempty IDs, positive safe revisions, exact nested fields, and SHA-256 digest. |
| H3 API/Desk interface | **Substantially closed.** Planned workflow methods and a direct structural assembly test exist; observation wiring needs finding 5. |
| H4 unknown/conflict protocol | **Open.** Findings 1, 2, and 4 affect classification and durable recovery. |
| M5 write trust boundary | **Closed for tested inputs.** IDs, revisions, recursive authority keys, and JSON form are checked before transport. |
| M6 DTO variants | **Closed for the frozen DTO families.** Strict parsers and valid/hostile fixtures now cover search, materials, submission, timeline, reminders, export/delete and confirmed facts. |
| M7 structured scope | **Mostly closed.** Scope is carried through remote/store calls and late cross-scope responses are discarded; same-scope ordering remains in finding 3. |
| Low ADR traceability | **Closed.** ADR 0019 exists in this checkout. |

## Checks and limits

- `pnpm exec tsx --test packages/contracts/src/career/*.test.ts packages/api-client/src/career/*.test.ts packages/career-core/test/*.test.ts`: **pass**, 15/15.
- `pnpm exec tsc --noEmit --strict --skipLibCheck --target ES2022 --lib ES2022,DOM --module NodeNext --moduleResolution NodeNext --allowImportingTsExtensions packages/contracts/src/index.ts packages/api-client/src/client.ts packages/career-core/src/index.ts`: **pass**.
- `git diff --check ce7e2f09268271646c2e99b77e9c4d4c03929a71..HEAD`: **pass**.
- The implementation evidence records `pnpm test:shared` passing (1137 passed, 4 skipped) and `pnpm typecheck:shared` failing only in unchanged `packages/views/src/chat/mermaid.ts`; I did not rerun those broad commands. This review's verdict rests on directly inspected source and focused checks, not on that report.
