# Issue #72 / #84 OCR Test Coverage Repair R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close verified test-coverage and test-fixture findings from the final Issue #84 OCR supplement without changing production behavior.

**Architecture:** Keep this repair in the serial Issue #84 worktree and modify only its service test file. Extend the existing batch-query test to exercise attention in both small and large lists, assert checkout URL preservation through attention resolution, check every touched fulfillment outbox count, and close shared in-memory SQLite handles in all three fixtures identified by OCR.

**Tech Stack:** Go, GORM, SQLite in-memory tests, existing `countOutbox` test helper.

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`; Issue #84 implementation requirements and acceptance mapping in `docs/plans/issue-72-plan-84-full-review-fix-r1.md`; exact OCR input at commit `41ef32f0d38ca6ad79843c4a308b5b0127cc5933` in `.superpowers/sdd/issue-72-plan-84-full-review-fix-r1/ocr-test-final-r2.md`.

## Global Constraints

- A payment attention flag represents an unresolved anomaly; resolving the anomaly clears attention while preserving the existing payable order and checkout link.
- `ListOrders` must derive attention from the tenant's unresolved anomalies and perform one tenant-scoped anomaly query for the requested list.
- No production source, database schema, or runtime behavior changes are in scope for this repair.
- Independent Issue #84 validation and OCR evidence must refer to the exact final source hashes.

## Review Focus

- One-order and forty-order lists both project a seeded unresolved anomaly and each issue exactly one anomaly-table SELECT — `TestListOrdersUsesOneAnomalyQueryForSmallAndLargeResults`.
- Resolving an anomaly clears `PaymentAttention` in `CurrentPayablePendingOrderView` while its `CheckoutURL` remains byte-for-byte equal to the order's stored URL — `TestCurrentPayablePendingOrderViewProjectsAndClearsAttention`.
- Outbox count assertions fail with the underlying query error rather than passing or obscuring it — `TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly` and `TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment`.
- Every shared-cache SQLite test fixture closes its `*sql.DB` at test cleanup; the cancellation fixture migrates the anomaly table used by the service environment — `newOrderTestEnv`, `TestOpenOrderPersistsChannelFailurePastCallerCancellation`, and `newRealWechatObservationEnv`.

---

### Task 1: Close Issue #84 test-scan findings

**Task ID / source / dependency / status:** T1 / Issue #84 under root #72 / depends on verified Task2 commit `41ef32f0d38ca6ad79843c4a308b5b0127cc5933` in the same serial worktree / ready after preflight.

**Owner / validator / reviewer:** `backend_implementer` / `backend_validator` / `reviewer`. Runtime role routing fixes the backend implementer to `gpt-6-luna` low; record the dispatched model and do not represent it as the pasted model preference.

**Files:**
- Modify: `internal/modules/commercial/service/commercial/order_test.go`
- Read-only helper reference: `internal/modules/commercial/service/commercial/order_close_test.go:161` (`countOutbox(t, db, kind) int64`)

**Interfaces:**
- Consumes: `OrderService.ListOrders(ctx context.Context, tenantID uint64) ([]domain.OrderView, error)`; `OrderService.CurrentPayablePendingOrderView(ctx context.Context, tenantID uint64) (domain.OrderView, error)`; package-local `countOutbox(t *testing.T, db *gorm.DB, kind string) int64`.
- Produces: Tests proving one and forty order list projections each batch one anomaly query and mark exactly the seeded row; a service view that retains `CheckoutURL` before and after anomaly resolution; checked fulfillment outbox counts; cleanup for the three shared-cache SQLite handles; complete test schema for the cancellation fixture.

- **Test helper signature:** `closeSQLiteDBOnCleanup(t *testing.T, db *gorm.DB)` obtains `db.DB()`, fails the current test if retrieval fails, and registers `t.Cleanup(func() { _ = sqlDB.Close() })`.

- [ ] **Step 1: Add the missing projection assertions and prove RED.** In `TestListOrdersUsesOneAnomalyQueryForSmallAndLargeResults`, seed an unresolved amount anomaly for `count_single_order` (tenant 112) before registering the query callback. Extend the case table with the expected attention order ID. For both subtests, assert exactly the expected row has `PaymentAttention=true` while retaining exactly one anomaly-table SELECT. Also add exact `CheckoutURL == order.CheckoutURL` assertions to the unresolved and resolved checks in `TestCurrentPayablePendingOrderViewProjectsAndClearsAttention`. First apply each new assertion, then temporarily inject the corresponding regression (force the one-order list view's attention false; separately blank the returned service-view checkout URL), run only its named test and capture the expected assertion failure. Restore production `order.go` after each probe and verify its SHA-256 equals the pre-probe hash before proceeding.
- [ ] **Step 2: Check outbox query errors.** Replace the three unchecked `Count(&nFulfill)` / `Count(&nOver)` calls in `TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly` and `TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment` with `countOutbox(t, db, kind)` and preserve the exact expected zero/one counts.
- [ ] **Step 3: Close SQLite fixture resources.** Add one package-local helper that obtains `db.DB()` and registers `t.Cleanup` to close the `*sql.DB`. Use it in `newOrderTestEnv`, `TestOpenOrderPersistsChannelFailurePastCallerCancellation`, and `newRealWechatObservationEnv` after each successful `gorm.Open`.
- [ ] **Step 4: Align cancellation fixture schema.** Include `repocommercial.PaymentAnomalyRow{}` in the `AutoMigrate` call in `TestOpenOrderPersistsChannelFailurePastCallerCancellation`.
- [ ] **Step 5: Run focused tests.** Run `go test ./internal/modules/commercial/service/commercial -run '^(TestListOrdersUsesOneAnomalyQueryForSmallAndLargeResults|TestCurrentPayablePendingOrderViewProjectsAndClearsAttention|TestRecoverOrderStatusWrongCurrencyRetainedAsAnomaly|TestRecoverOrderStatusLateSuccessAfterFulfilledIsIdempotentOverPayment|TestOpenOrderPersistsChannelFailurePastCallerCancellation)$' -count=1`; expect PASS and no leaked database handles.
- [ ] **Step 6: Run package verification.** Run `go test ./internal/modules/commercial/service/commercial -count=1`, `gofmt -d internal/modules/commercial/service/commercial/order_test.go`, and `git diff --check`; expect PASS, no gofmt output, and no whitespace errors.
- [ ] **Step 7: Review the complete owned diff and commit.** Confirm only `order_test.go` changed in the implementation checkpoint, capture its SHA-256, and commit as `test(commercial): close payment attention review gaps`.

**Acceptance mapping:** OCR test supplement M1 → steps 1 and 6; OCR test supplement M2 service-view retention → steps 2 and 6 (the HTTP-level duplicate replay already independently asserts order ID and checkout URL before/after resolution); OCR test supplement M3 count-query failure safety → steps 3 and 6; OCR test supplement L1 DB lifecycle → step 4 and package test; OCR test supplement L2 migration drift → step 5 and package test.

**Failure handling:** If the package-local helper cannot be resolved in the same Go test package, define an equivalent error-checking helper in `order_test.go`; do not add production code. If a focused test exposes a real service defect, stop and record it rather than expanding this test-only task silently.
