# SDD ledger — plan: docs/superpowers/plans/2026-09-11-saas-billing-connectors-implementation.md

## Setup

- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/saas-billing-connectors`
- Branch: `codex/saas-billing-connectors`
- Base before Task V01: `9211c20`
- Plan split: V=contract verification, F=foundation, C=payments, U=budget/metering, A=connectors, W=product flows, O=rollout/acceptance.
- The source plan files are present in this worktree as untracked user-authored documents. They will be committed in the first documentation-owned task only if required by the implementation workflow; implementation commits stage only their owned files.

## Preflight shared-file/interface scan

| Shared tasks | Shared file or interface | Finding and ruling |
| --- | --- | --- |
| V01/V02/V03 | `scripts/saas/*`, contract case JSON | Sequential verification tools; V02 consumes V01 path output, V03 consumes V02 evidence. No conflict. |
| F02/F05 | `internal/router/router.go`, `internal/container/container.go` | F02 creates commercial route dependencies; F05 later adds app routes and UI wiring. Ruling: serialize router/container edits in task order. |
| F03/F04/C01 | catalog, quote, subscription/order versions | F03 owns immutable versions; F04 owns lifecycle keys; C01 consumes quote and writes orders. No duplicate authority. |
| C01/C02/C03/C04/C05 | payment facts, Provider, CommercialGateway | C01 defines payment persistence; C02 defines Provider; C03 implements Alipay; C04 defines benefit gateway; C05 extends revoke. Ruling: each consumer updates the defining interface and fakes in the same task. |
| U01/U03/U05 | usage/settlement/execution interfaces | U01 owns physical UsageFact; U03 owns confirmation; U05 owns execution gate. No duplicate usage charge. |
| U02/U03/C05 | budget reservation and refund locking | C05 must use U02/U03 coordination, never a second balance read. Dependency is explicit; no conflict. |
| A01/A02/A03 | Installation, Connection, approval snapshots | A01 owns identity; A02 credentials; A03 immutable Action digest. No conflicting ownership. |
| A03/A04/A05/A06 | Adapter and Action execution | A04 consumes A03; A05/A06 implement provider adapters. Sequential by dependency. |
| A07/W04 | sync binding and app connector API | A07 owns server sync binding; W04 owns public projections and pages. W04 must not duplicate checkpoint semantics. |
| W01/W02/W03/W05 | `packages/contracts/src/commercial.ts`, API client, `apps/web/src/App.tsx` | W01 defines shared commerce types/client; W02/W03 consume it; W05 adds action UI. Ruling: serialize shared exports and App.tsx edits. |
| W04/W05 | `packages/contracts/src/appconnector.ts`, API client, App.tsx | W04 defines projections and routes; W05 consumes action types. Ruling: W04 must export all types before W05. |
| O01/O02/O03 | rollout switches, migration evidence, acceptance report | O01 protects recovery; O02 migration uses switches; O03 only collects evidence. No authority overlap. |

## Preflight self-consistency scan

| Task | Tests vs implementation/files | Result/ruling |
| --- | --- | --- |
| V01 | `operation_paths` test matches parser signature; files are created and staged by command. | Consistent. |
| V02 | `assert_subset` test matches recursive implementation; CLI case path is explicit. | Consistent. |
| V03 | Gate test rejects schema-only evidence; selected-model output is validated. | Consistent. |
| F01 | MonthBoundary and ParseCredits tests cover stated fixed units and month-end implementation. | Consistent. |
| F02 | Access test matches owner/grant/inactive rules; migrations and repository are owned. | Consistent. |
| F03 | Prorate tests match integer arithmetic and catalog immutability files. | Consistent. |
| F04 | Monthly key replay test matches UTC-stable key; lifecycle migration files are owned. | Consistent. |
| F05 | Quota tests match `CanIncrease`; router/container are explicitly modified. | Consistent; `math` import is required by plan. |
| C01 | Payment mismatch test matches `ValidatePayment`; order/outbox files and migration are owned. | Consistent. |
| C02 | RSA body-tamper test matches VerifyWechatSignature; callback/provider files are owned. | Consistent. |
| C03 | CNY parser test matches Alipay implementation; Provider interface is consumed. | Consistent. |
| C04 | Fulfillment key test matches gateway idempotency implementation. | Consistent. |
| C05 | Refund state test matches lock release rule and refund migration/API files. | Consistent. |
| U01 | BYOK test matches funding validation and UsageFact files. | Consistent. |
| U02 | Available test matches reservation CAS; PG integration test is tagged and explicit. | Consistent; no PG evidence without DSN. |
| U03 | Protection test matches settlement acknowledgement interface. | Consistent. |
| U04 | Lease state test matches release/reconcile service. | Consistent. |
| U05 | Funding test matches execution gate adapter and runtime touchpoints. | Consistent; current Craft child runtime remains gated until its own binding exists. |
| A01 | Personal connection test matches installation/connection models and migrations. | Consistent. |
| A02 | OAuth replay test matches binding validation and credential resolver. | Consistent. |
| A03 | Risk approval test matches Action digest/state persistence. | Consistent. |
| A04 | Internal-IP test matches HTTP policy; MCP adapter consumes Action/ExecutionGate. | Consistent. |
| A05 | Feishu retry test matches provider idempotency policy. | Consistent; real provider remains a separate gate. |
| A06 | Notion partial-success test matches page recovery state. | Consistent; real provider remains a separate gate. |
| A07 | Checkpoint/fence test matches sync service and existing Connector interfaces. | Consistent. |
| W01 | Strict amount parser test matches shared contract/client signatures. | Consistent. |
| W02 | paid vs fulfilled UI test matches OrderView and scoped page props. | Consistent. |
| W03 | refund status test matches RefundView/client review signatures. | Consistent. |
| W04 | connection label test matches projections/routes and scope usage. | Consistent. |
| W05 | unknown/approval message test matches ActionView and budget UI. | Consistent. |
| O01 | rollback test matches recovery switch function and runbook files. | Consistent. |
| O02 | migration-state test matches dry-run CLI and no invented credits. | Consistent. |
| O03 | release gate tests reject mock provider evidence and missing cases. | Consistent. |

## Rulings

- `Ruling: Implementation starts with V01 in the dedicated worktree; the main checkout remains untouched — the user selected Subagent-Driven execution and the plan requires isolation — cost if wrong: branch setup overhead, reversible.`
- `Ruling: V03 may record OM-10 as a coordination strategy while U02/U03 provide production concurrency proof — otherwise the verification task and budget implementation form a dependency cycle — cost if wrong: the selected-model gate may need one additional update before release.`

## Task status

V01 implementation committed as `8a7bff2`; review fixes are committed in the same isolated worktree. Static regression evidence is available; live provider probing remains blocked until the fixed schema and service environment are available.

- Task V01: fix round 1/5 (0 addressed, 2 open — missing short-circuit assertion; empty paths fail-closed; commits `8a7bff2..e343086`).
- Task V01: fix round 2/5 (2 addressed, 0 open — re-review verified; commit `0bc7272`).
- Task V01: complete (commits `8a7bff2..0bc7272`, review clean; live provider/schema remains blocked-env by design).
- Remaining tasks pending; next dispatch V02.
- Task V02: implementation commit `a17bf4d`; first review FAIL (6 blocking findings: schema default, HTTP classification, cleanup, idempotency, failure artifacts, embedded namespace enforcement).
- Task V02: fix round 1/5 committed as `69e5248`; focused tests 5/5, py_compile and diff-check pass; re-review pending. Live OM-02–OM-10 experiments remain blocked-env; cleanup requires declared captured IDs and settled transactions.
- Task V02: fix round 1 re-review FAIL; remaining blockers are cleanup write gating/path safety, namespace capture handling, idempotency replay proof, and HTTP failure artifacts. Fix round 2 dispatched.
- Task V02: fix round 2 committed as `88f4129`; focused tests 7/7, py_compile and diff-check pass; re-review pending.
- Task V02: fix round 2 re-review FAIL; remaining blockers are cleanup namespace/path validation, missing cleanup capture handling, and explicit same-resource replay identity assertion. Fix round 3 dispatched.
- Task V02: fix round 3 committed as `8c9b477`; focused tests 8/8, py_compile and diff-check pass; final re-review pending. Cleanup requires declared captures and replay requires an explicit identity JSON Pointer.
- Task V02: fix round 3 re-review FAIL; remaining blocker is namespace enforcement for string paths plus replay_identity pointer validation. Fix round 4 dispatched.
- Task V02: fix round 4 committed as `0734cd0`; focused tests 9/9, py_compile and diff-check pass; final re-review pending. Live experiments remain blocked-env.
- Task V02: fix round 4 re-review FAIL; capture references in paths require post-bind namespace validation. Fix round 5/5 dispatched (maximum).
- Task V02: fix round 5 committed as `ee4f7ad`; focused tests 10/10 pass; definitive re-review pending. Captured request/cleanup paths are rebound and namespace-validated at runtime; live experiments remain blocked-env.
- Task V02: fix round 5 re-review BLOCKED/FAIL. Remaining blocker: cleanup does not prove the target ID was created by the current run; a read-derived capture could be deleted. Max 5 fix rounds exhausted; V02 is blocked and downstream V03+ tasks must not dispatch until this ownership check is implemented and reviewed.
- Task V02: ownership fix implemented after user-directed continuation. Successful write-step captures are tracked in `created_captures`; cleanup rejects read-derived/untracked captures. Added unit and run-level regressions; 12/12 focused tests pass.
- Task V02: fresh ownership re-review FAIL; capture names were tracked but values could be overwritten by a later GET. Follow-up fix dispatched to track immutable created ID values and reject changed values.
- Task V02: ownership fix commit `5e34e9e` passed independent final review; 13/13 focused tests, py_compile, and diff-check pass. External OpenMeter execution remains blocked-env.
- Task V03: brief generated; implementation dispatch follows V02 PASS.
- Task V03: implementation commits `f84b98b`, `108e924`, gate fix `6b8dcf5`; independent final review PASS. Gate tests 26/26, py_compile and diff-check pass; selected model remains blocked-env and exits 2.
- Task F01: brief generated; implementation dispatch follows V03 PASS.
- Task F01: implementation commit `5a1111e` + ledger commit `df1e575`; independent review PASS (task-F01-review.md); targeted Go tests and diff-check pass.
- Task F02: brief generated; implementation dispatch follows F01 PASS.
- Task F02: implementation commit `bb7e8c6`; coordinator verified 4/4 targeted tests, go vet, go build, and diff-check; independent review PASS (task-F02-review.md, B1–B7). PostgreSQL concurrency evidence remains blocked-env.
- Task F03: brief generated; implementation dispatch follows F02 PASS.
- Task F03: implementation commit `2f4462a` (RED-first: undefined symbols; GREEN: 12/12 targeted tests; vet/build/diff-check/gofmt pass); coordinator re-verified tests ok, vet 0, diff-check clean, scope = 9 owned files; independent review PASS (task-F03-review.md, C1–C9). Repository-layer runtime and PostgreSQL dialect remain blocked-env/not-run and are not marked pass.
- Task F04: brief generation and dispatch follow F03 PASS.
- Task F04: implementer timed out at wall clock after creating all files; finisher subagent verified GREEN 12/12 (5 domain + 7 SQLite integration), vet/build/diff-check pass, and committed `af899ce` (10 files). Coordinator re-verified tests 12/12, vet 0, diff-check clean, scope exact.
- Task F04: independent review PASS (task-F04-review.md, D1–D8); failed-issuance auto-retry noted as ops follow-up, non-blocking. PostgreSQL migration execution and external-issuer e2e remain blocked-env.
- Task F05: brief generation and dispatch follow F04 PASS. F05 owns router.go/container.go edits per preflight ruling.
- Task F05: implementer timed out after writing test-first files (tests 17:38, impl 17:39-17:40); finisher verified GREEN 11/11 (Test(Quota|Commercial)), brief regex 6/6, vet/build/gofmt/diff-check pass, full existing packages ok, made 3 minimal fixes (UserIDFromContext, types.APIKeyCapabilityCommercial additive constant, gofmt) and committed `195cf6e` (9 files incl. +7-line additive types change required for grantable commercial capability). Coordinator re-verified tests/vet/diff-check and scope.
- Task F05: independent review PASS (task-F05-review.md, E1–E7); non-blocking: POST /orders 501 stub, admitted-key assertion strength, single-connection SQLite test pool; process note: RED transcript unpreserved but disclosed, timestamps support test-first. PostgreSQL dialect + external provider e2e remain blocked-env.
- Foundation segment F01–F05 complete. Next: C01 per payments plan (docs/superpowers/plans/2026-09-11-saas-02-payments.md).
- Task C01: brief generated; implementation dispatch follows F05 PASS.
- Task C01: implementation commit `19e64ef` (10 files incl. brief-mandated repo order_test.go); RED valid (undefined Order/PaymentFact/ValidatePayment); GREEN 12/12 (5 domain + 7 SQLite real-transaction tests: rollback atomicity, replay idempotency, concurrent two-channel single fulfillment, quote single-consumption, paid/fulfilled distinction); vet/build/gofmt/diff-check + full two-package regression pass. Coordinator re-verified.
- Task C01: independent review PASS (task-C01-review.md, F1–F7); non-blocking: single-connection SQLite pool serializes concurrency at driver level (guarded UPDATE decides winner). PostgreSQL dialect remains blocked-env.
- Task C02: brief generation and dispatch follow C01 PASS.
- Task C02: implementation commit `e269415` (6 files; implementer finished despite wall-clock kill of its report); RED valid (undefined WechatProvider/VerifyWechatSignature); GREEN 9/9 (tampered body/key/serial/ts+nonce/base64/stale/duplicate/merchant mismatch + valid signature with AES-256-GCM and mchid/appid match); router suite unbroken; vet/build/diff-check pass; routes_commercial.go additive with fail-closed unwired default. Coordinator re-verified.
- Task C02: independent review PASS (task-C02-review.md, G1–G9); non-blocking: in-memory nonce store, algorithm-string not explicitly checked, C01-delegated coverage disclosed. WX-01～05 real-channel acceptance remains blocked-env.
- Task C03: brief generation and dispatch follow C02 PASS.
- Task C03: implementer timed out after writing alipay.go/test; finisher wired the callback dispatch (WeChat path byte-identical), fixed one fixture compile error, verified GREEN 14/14 Alipay + 9 WeChat (full package ok), vet/build/gofmt/diff-check pass, and committed `187570b` (4 files). Coordinator re-verified.
- Task C03: independent review PASS (task-C03-review.md, H1–H8); non-blocking: QueryRefund single-identifier extension point documented, C05 providers-map wiring deferred. ALI-01～05 real-channel acceptance remains blocked-env with method/version pinned.
- Task C04: brief generation and dispatch follow C03 PASS.
- Task C04: assembled across three agents (implementer timed out mid-task twice): domain+gateway, then worker+gateway tests, then worker DB tests+container wiring+commit `1e7de9a` (8 files, +1606). Targeted 12/12 (domain 5, openmeter httptest 4, worker SQLite 3: saved-then-dropped 3-pass idempotency, 8-worker single fulfillment, refusal-recovery); vet/build 4 packages, gofmt, diff-check, existing suites all pass. Coordinator re-verified.
- Task C04: independent review PASS (task-C04-review.md, I1–I9); non-blocking: orderer-leaves-space has no dedicated test (code derives tenant from order row), subscription settlement op deferred to V03-validated credit-grant schema. OM-04/09 real-model rerun remains blocked-env (httptest stub evidence labeled separately).
- Task C05: brief generation and dispatch follow C04 PASS.
- Task C05: implementer interrupted pre-verification; finisher made 3 minimal fixes (review-record transactionality, stub revoke counter, relative seed dates), verified targeted 11/11 (domain 6 + service SQLite 5 incl. -race race test), all gates green, committed `d22aad3` (15 files, +1579; disclosed extras: fulfillment.go interface +RevokeBenefit, openmeter gateway implementation, fulfillment_test.go stub counter). Coordinator re-verified and removed the implementer's untracked scratch debug_test.go (package green before/after).
- Task C05: independent review PASS (task-C05-review.md, J1–J8); non-blocking: lock reads/isolation when P03 lands on PostgreSQL, ProcessPayouts head-of-line blocking, admin-review authority realized as Admin tenant role. Real-channel refunds/revocations remain blocked-env; P03 approvals stay requested/reviewing by design until U-segment lands.
- Payments segment C01–C05 complete. Next: U01 per budget/metering plan (docs/superpowers/plans/2026-09-11-saas-03-budget-metering.md).
- Task U01: brief generated; implementation dispatch follows C05 PASS.
- Task U01: implementation commit `5449a1c` (9 files incl. disclosed repo usage_test.go); RED valid (undefined BillableModel/ValidateFunding/UsageFact/UsageStore); GREEN 14/14 (domain 6: BYOK/platform/untrusted, funding validation, forged funding, model-only BYOK waiver, single rounding via big.Rat; repo SQLite 8: duplicate idempotent, partials→one final delta, display_only parent no settlement, retry new attempt, same-revision conflict, late correction new revision, unknown no zero settlement, missing rates error). Coordinator re-verified.
- Task U01: independent review PASS (task-U01-review.md, K1–K9); non-blocking: current-pointer moves on any revision (U03 netting heads-up), concurrency falls back to DB UNIQUE. PostgreSQL dialect remains blocked-env.
- Task U02: brief generation and dispatch follow U01 PASS.
- Task U02: implementation commit `a89920d` (10 files, +1294); RED valid (undefined Available); SQLite -race 10/10 (20-way exactly-one-winner with held==allocations, LockRefunds vs Reserve race, parent/child no-copy, external balance never refreshes unreflected); PG integration file with brief template + 2 additional scenarios compiled-not-run (blocked-env, no SAAS_TEST_PG_DSN).
- Task U02: first independent review FAIL (L4: lot-guard miss committed partial state via retry=true+return nil; reservation-insert error same; errBudgetCASRetry sentinel leaked to callers). Fix commit `0cfa73e`: guarded misses now return the sentinel (GORM rollback), errors.Is consumed in retry loops, callers never see it; 5 new test-first regressions (RED on old implementation); 15/15 -race; disclosed behavior-equivalent ApplyExternalBalance sentinel uniformity.
- Task U02: independent re-review PASS (task-U02-rereview.md, R1–R6). PG integration execution remains blocked-env.
- Task U03: brief generation and dispatch follow U02 PASS.
- Task U03: assembled by three agents (implementer + finisher-1 interrupted; finisher-2 verified and committed `3ef3742`, 8 files +1674). Targeted TestSettlement -race 10/10 (domain 5 incl. correlation-evidence rule; service 5: aggregation delay, accepted-then-crash replay, confirmation-lost watermark-once, watermark regression refused, refund-simultaneous coordination). Full -race regressions on three packages ok; vet/build/gofmt/diff-check pass. U02 rollback discipline preserved (+263 additive budget.go). RED evidence unpreserved (interruptions), disclosed.
- Task U03: independent review PASS (task-U03-review.md, M1–M9); non-blocking: equal-watermark liveness edge and delta>upper over-protection edge, both fail-safe. Real-channel settlement-ack remains blocked-env behind the V03 gate.
- Task U04: brief generation and dispatch follow U03 PASS.
- Task U04: implementation commit `a137c79` (6 files, +957; repo diff additive-only +370). RED valid (undefined MayReleaseWithoutQuery); targeted -race 8/8 (held-release CAS incl. dispatch complement, cancel-but-remote-succeeded needs confirmation with protection preserved, duplicate Extend idempotent, downgrade pauses new/permits reconcile, lease fence takeover stale-rejection, expiry-release vs refund-lock race single-release); full -race regressions green. Coordinator re-verified.
- Task U04: independent review PASS (task-U04-review.md, N1–N8); non-blocking: ErrTaskBudgetExpired lacks a dedicated test (follow-up), ledger service-test count undercounted. PostgreSQL dialect + real-provider query-confirmation remain blocked-env.
- Task U05: brief generation and dispatch follow U04 PASS.
- Task U05: implementation commit `0a38092` (8 files, +603; engine +15 minimal injection with nil-gate no-op, usage.go +50 additive adapter, container +6). RED valid (undefined ExecutionGate family; ValidateFunding pin via U01 single source). Targeted 3/3 packages ok; 8 new domain cases; vet/build/gofmt/diff-check pass; ./internal/agent full suite green. Pre-existing environmental failure disclosed: TestBuildChatCompletionRequest_GPT5MaxCompletionTokens (azure endpoint DNS-blocked by SSRF guard; proven pre-existing via stash-rerun; usage.go untouched functionally).
- Task U05: independent review PASS (task-U05-review.md, P1–P9); non-blocking: err== instead of errors.Is for ErrAbnormalCost, zero-usage reservations left to U04 TTL recovery. Craft/OpenCode runtime_binding remains undelivered — only the current AgentEngine entry claims pass; real-channel evidence blocked-env; unavailableRates rejects rather than zero-charging.
- Budget/metering segment U01–U05 complete. Next: A01 per connectors plan (docs/superpowers/plans/2026-09-11-saas-04-connectors.md).
- Task A01: brief generated; implementation dispatch follows U05 PASS.
- Task A01: implementation commit `a798e77` (10 files, +746 incl. disclosed install_test.go); RED valid (undefined Connection/CanUseConnection); both packages 13/13 (access 6: owner-only personal, explicit-grant space, disabled/cross-tenant/unknown-kind fail-closed; repository 7: two-space independence, multi-connection, reauthorization_required on scope upgrade with zero side effects, stale CAS conflict). CanManageBilling appears only in comments — installation never consults billing. Coordinator re-verified incl. boundary grep.
- Task A01: independent review PASS (task-A01-review.md, Q1–Q7); non-blocking: AutoMigrate vs migration-SQL execution gap disclosed. PostgreSQL dialect remains blocked-env.
- Task A02: brief generation and dispatch follow A01 PASS.
