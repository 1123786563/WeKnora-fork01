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
