---
status: final
verdict: PASS
reviewer: independent-reviewer (W01, web/connectors segment; did NOT write this code)
commit-under-review: de023ac (implementation, 8 files +367/-1) + 4c4ba57 (ledger row), worktree saas-billing-connectors
files-reviewed: packages/contracts/src/commercial.ts (54), packages/contracts/test/commercial.test.ts (110), packages/api-client/src/commercial.ts (68), packages/api-client/src/commercial.test.ts (126), packages/contracts/src/index.ts, packages/api-client/src/client.ts, packages/api-client/src/index.ts, package.json, ledger row 4c4ba57, task-W01-brief.md, siblings datasource.ts/datasource.test.ts/errors.ts, Go internal/commercial/amount.go + quote.go/quote_test.go
method: source-level evidence via git show de023ac + file reads + ONE independently re-run confirming command (brief Step-2/4 targeted test). Full gates NOT re-run here per instructions (coordinator already re-verified typecheck:shared, test:shared 55/55, diff-check).
confirming-command: pnpm exec tsx --test packages/contracts/test/commercial.test.ts packages/api-client/src/commercial.test.ts -> exit 0, tests 14, pass 14, fail 0, skipped 0, todo 0 (72.8ms)
---

# Task W01 Independent Review — 共享商业契约与 API client

## Y1 — Brief compliance: file set, Step-1 verbatim, Step-3 interfaces, paths/methods — PASS

- **File set exact.** git show de023ac --stat: precisely the 8 brief-named files — creates packages/contracts/src/commercial.ts (+54), packages/contracts/test/commercial.test.ts (+110), packages/api-client/src/commercial.ts (+68), packages/api-client/src/commercial.test.ts (+126); modifies packages/contracts/src/index.ts (+3), packages/api-client/src/client.ts (+3), packages/api-client/src/index.ts (+2), package.json (typecheck list). No other file touched; the ledger lives in the separate 4c4ba57 commit. Commit message is the brief Step-6 string verbatim (feat: expose typed space commerce client).
- **Step-1 test present and unmodified in substance.** contracts/test/commercial.test.ts:8-12 — test name 'paid is distinct from fulfilled', all three body lines (parse call with the exact literal object, assert.equal(value.fulfillment, 'pending'), assert.throws with amount_fen:100) byte-identical to brief Step 1. Only cosmetic adaptation: import order normalized and the parseOrderView import widened to include the two new parsers the same file needs (same standard the A05/A06 reviews accepted for gofmt-level normalization).
- **Step-3 interfaces byte-equivalent.** contracts/src/commercial.ts:1-13 — OrderView (id / payment 'pending'|'paid'|'closed' / fulfillment 'pending'|'processing'|'fulfilled'|'attention' / amount_fen: string / currency: 'CNY'), CommercialSummary, QuoteView, QuoteInput, CreateOrderInput (provider 'wechat'|'alipay', idempotency_key), RefundInput: field names, unions, and string amounts all match the brief sketch exactly, modulo line-wrapping only.
- **parseOrderView verbatim.** contracts/src/commercial.ts:14-21 — identical guard chain and identical regex /^\d+$/ (line 17), same two includes(String(...)) state checks, same throw new Error('invalid order'), same 'v as unknown as OrderView' return. Validation covers every OrderView field.
- **createCommercialApi signature + five methods.** api-client/src/commercial.ts:36 — createCommercialApi(request: (input: ClientRequest) => Promise<unknown>); getOrder GET /api/v1/commercial/orders/ + encodeURIComponent(id) (line 41; encoding is the datasource.ts:52 convention, strictly safer than raw :id interpolation), summary GET /api/v1/commercial/summary (46), quote POST /api/v1/commercial/quotes (49), createOrder POST /api/v1/commercial/orders (52), requestRefund POST /api/v1/commercial/refunds (55). Return types match the brief, including requestRefund -> Promise<{id: string; state: string}>.
- package.json adds exactly packages/api-client/src/commercial.ts to the typecheck:shared explicit list (contracts/src/index.ts already listed), per the coordinator deviation note; the test:shared glob already covers both new test locations.

## Y2 — Envelope discipline — PASS

- **Unwrap BEFORE parse, everywhere.** api-client/src/commercial.ts:39, 46, 49, 52, 55 — every method is parse*(unwrap(await request(...))); no method ever feeds a raw envelope into a parser.
- **success:false -> typed error via existing conventions.** unwrap (15-34) rejects non-object envelopes and success !== true with ApiError (errors.ts:10-24 class, same init shape as existing use). Server error.code/message/requestId pass through when present (24-28) — pinned by the test asserting error.code === 'QUOTE_EXPIRED' and error.message === 'quote expired'; missing-data envelope -> INVALID_RESPONSE (30-32); opaque failure -> non-empty fallback code 'COMMERCIAL_ERROR' (25-26), also asserted for the {success:false}-only case.
- **No auto-retry.** grep -i retry over api-client/src/commercial.ts: no match; the module is a pure one-shot request->parse mapper, so payment-creating keys are never re-sent by this layer.

## Y3 — Summary/quote field-by-field validation — PASS

- parseCommercialSummary (contracts/src/commercial.ts:33-44): plan_name non-empty string (36), paid_until null|string (37), available/held/refund_locked each /^\d+$/ digit-strings (38-40), as_of non-empty string (41), stale boolean (42); returns an explicitly constructed object, not a spread-cast. Positive and alternate-branch cases asserted (test:57-69: paid_until null / stale false, and '2026-12-31' / stale true).
- parseQuoteView (46-54): id non-empty (49), amount_fen digit-string (50), credit_delta /^-?\d+$/ string (51), expires_at non-empty (52). Negative-delta acceptance and numeric-rejection both asserted (test:96, 106).

## Y4 — Scope discipline — PASS

- No write input accepts tenant_id: QuoteInput/CreateOrderInput/RefundInput (contracts/src/commercial.ts:11-13) have no such field, and the dedicated test asserts 'tenant_id' in body === false across all three POST bodies (api-client test:80-90) — a genuine runtime counterexample, not a type-level claim.
- Amounts stay strings end-to-end: grep 'Number(' across both new source files -> no match; parsers reject numeric amount_fen outright (contracts test:35, 77, 104; api-client test 'rejects malformed payloads after unwrapping'); the client asserts typeof value.amount_fen === 'string' post-parse.

## Y5 — Test quality — PASS

- Counterexamples are real assertions: amount-as-number rejected at both layers; currency 'USD' rejected (contracts test:40); illegal states payment 'refunded' / fulfillment 'shipped' (41-42); envelope error propagation incl. the opaque-failure fallback; abort-signal passthrough checked by identity (seen[0] === controller.signal) on a read AND a write; the five-endpoint test deepEquals the full method/path/body sequence (api-client test:40-53) — path-by-path, not substring spot-checks.
- No weakening: the confirming run reports skipped 0, todo 0; every listed test passes on real assertions.
- Fake pattern follows existing convention: request-function injection mirrors datasource.test.ts:8-13 (createDataSourcesApi(async (request) => {...}) with a request log); the final test builds a typed HttpTransport object fake through createWeKnoraClient — the established transport-fake pattern — and additionally pins URL composition (https://example.test/api/api/v1/commercial/orders/o1) and transport-level cancellation (AbortSignal.abort() -> ApiError code 'CANCELLED').

## Y6 — Additivity of modified files — PASS

- contracts/src/index.ts: +2 export lines for commercial types/parsers only (diff hunk after line 61); every pre-existing export untouched.
- api-client/src/index.ts: +2 lines (createCommercialApi + type re-export from '@weknora/contracts', mirroring the existing KnowledgeDocument precedent at line 12).
- api-client/src/client.ts: +1 import, +1 local, +1 returned property (commercial); no existing behavior changed.
- package.json: single-line typecheck:shared list edit, insertion only; packages/api-client/src/commercial.ts added exactly as the coordinator note required.

## Y7 — Disclosed deviations judged — PASS (all four legitimate, none spec-violating)

1. **Types re-exported from '@weknora/contracts' instead of a local ./commercial.ts** — the types ARE defined in exactly one commercial.ts (packages/contracts/src/commercial.ts), the file the brief's Step-3 sketch and parseOrderView live in; the brief's "所有输入与结果类型在 commercial.ts 定义" is satisfied with contracts as the canonical definition. api-client/src/commercial.ts consuming them avoids type duplication, and the index re-export copies the existing line-12 pattern. No violation.
2. **Optional signal params on quote/createOrder/requestRefund** — additive optional parameters; the brief's minimal signatures remain exactly callable, consistent with getOrder/summary already taking signal, and it is tested. Harmless extension.
3. **Negative credit_delta /^-?\d+$/ ** — verified against the Go domain: internal/commercial/amount.go:11 'type Credits int64', and quote_test.go:21 exercises {-1, 10, 30} // negative delta as a legitimate proration case, with quote.go:79 typing the field CreditDelta Credits. A digit-only parser would reject valid negative deltas; the disclosed deviation is domain-correct, not a weakening (numeric and malformed values still rejected).
4. **Two commits (code + ledger) instead of one** — the same split the accepted A06 review treated as non-blocking; the union of both commits' files equals the brief Step-6 staging list exactly. Non-blocking.

## Y8 — Honesty / ledger accuracy — PASS

- Ledger row (4c4ba57) states RED (ERR_MODULE_NOT_FOUND both files), GREEN 14/14, typecheck:shared pass, test:shared 55/55, diff-check pass, and explicitly keeps real-endpoint verification as blocked-env/not-pass ("纯契约+fake transport 测试不等同 UI 验收…未对真实 HTTP 端点运行"). This reviewer's independent run reproduces 14/14 exactly.
- The RED claim is consistent-by-construction (both test files import source modules that did not exist pre-commit, so tsx fails with ERR_MODULE_NOT_FOUND); not re-runnable here without a destructive reset — noted, not counted against.
- One accuracy nit: the per-file split "contracts 6 用例 + api-client 8 用例" is inverted — actual is 7 + 7 (seven test blocks in each file). The aggregate 14/14 and every gate claim are correct. Non-blocking.
- The worktree pnpm install (untracked node_modules only, --frozen-lockfile) is disclosed with rationale; no tracked-file impact, verifiable from the commit's clean file set.

## Y9 — Code quality — PASS

- Strict-clean by inspection: zero any; casts confined to the documented parse pattern (value as Record<string, unknown>, and the brief-verbatim 'as unknown as OrderView'); coordinator-verified typecheck:shared clean. Error handling consistent: ApiError with explicit codes at the client layer, plain Error inside contracts parsers (the brief-mandated style for parseOrderView, kept uniform for the two new parsers). encodeURIComponent on the order id matches the datasource convention.

## Improvements (non-blocking)

1. Ledger per-file test counts (6/8 vs actual 7/7) — worth a one-word fix if the ledger is touched again; the aggregate is correct.
2. Forward-compat note for F05/C01/C05: Go Credits serializes via formatFixed(int64(c), 6) (amount.go:35). If the server ever emits fixed-decimal strings ("1200000.000000") on the wire, digitString/parseQuoteView will reject them. Pin the integer-string wire format explicitly when the server endpoints land.
3. unwrap re-derives envelope-error extraction that overlaps errorFromResult (errors.ts:26-49); reuse is awkward because errorFromResult needs an HTTP status this layer does not see — acceptable as-is, but a shared helper could serve both if a third envelope consumer appears.
4. contracts/src/commercial.ts intentionally mixes the brief's compact one-line style (verbatim Step 3) with expanded helper style for the added parsers — cosmetic tension, required by the verbatim mandate.

## Final verdict

**PASS.** No blocking finding across Y1-Y9. The brief's contract is implemented verbatim where the brief demanded verbatim (Step-1 test, Step-3 interfaces and parseOrderView, five endpoint mappings), the four disclosed deviations are each defensible on convention or domain grounds (negative credit_delta verified against Go 'Credits int64' and quote_test.go's negative-delta case), the envelope/scope/string-amount disciplines hold, and the targeted 14/14 was independently reproduced by this reviewer (0 skipped, 0 todo). Real-endpoint and UI acceptance remain correctly marked as pending server-side wiring, not claimed.
