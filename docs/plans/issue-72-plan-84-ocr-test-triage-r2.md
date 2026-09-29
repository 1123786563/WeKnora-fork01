# OCR test supplement final R2 — finding triage

**Checkpoint:** commit `41ef32f0d38ca6ad79843c4a308b5b0127cc5933`; copied-file hashes: service test `cc23397dfa704b078dd2868108dc5468418a1ad8268a1b6899a3f3e374ef47d7`, handler test `a0ade870a9a8f5ad624a02da06bc8a8636de7abefd903999463fc0ac6dd58e3a`.

**Coverage status:** OCR session `3f1603a7-e254-41f1-95f4-00fd315731b8` selected 2 copies and completed 1; `commercial_order_review.go` completed with 5 findings. `commercial_purchase_review.go` failed with a provider HTTP 429 (`z-ai-coding` account rate limit). CLI exited 0 but `failed_files=1`, so this is partial OCR only and is not a pass. The repository currently has no other configured OCR provider. Session JSON and output report are authoritative.

## Finding decisions

| ID | OCR finding | Decision | Evidence / action |
|---|---|---|---|
| OCR-TEST-M1 | The one-order case in `TestListOrdersUsesOneAnomalyQueryForSmallAndLargeResults` checks query count but seeds no anomaly or attention flag. | Valid, fix planned. | The separate 40-order test covers batch attention, but no one-row `ListOrders` case exercises positive attention projection. R2 Task 1 will seed an anomaly for `count_single_order` and assert that row's flag while retaining one SELECT. |
| OCR-TEST-M2 | The service-view test checks attention true→false but not checkout URL preservation. | Valid as a service-level coverage improvement; broad “no single test” claim is false. | The handler replay test at `internal/handler/commercial_purchase_test.go:356-423` already covers unresolved attention and resolved omission/false with stable order ID and checkout URL in the same HTTP interaction. R2 Task 1 will also assert the `CurrentPayablePendingOrderView` keeps `order.CheckoutURL` before and after resolution, making the service projection contract direct. |
| OCR-TEST-M3 | Three fulfillment/overpayment `Count` calls ignore `.Error`, allowing false-green or confusing-zero paths. | Valid, fix planned. | `countOutbox(t, db, kind)` at `internal/modules/commercial/service/commercial/order_close_test.go:161` checks the GORM error and is package-local. R2 Task 1 will replace the three unchecked calls with this helper. |
| OCR-TEST-L1 | Three shared-cache SQLite fixtures leave `*sql.DB` open. | Valid low risk, fix planned. | The findings identify `newOrderTestEnv`, `TestOpenOrderPersistsChannelFailurePastCallerCancellation`, and `newRealWechatObservationEnv` in the same owned test file. R2 Task 1 will register cleanup for each handle. |
| OCR-TEST-L2 | The cancellation fixture's `AutoMigrate` omits `PaymentAnomalyRow`, unlike neighboring fixtures. | Valid low-risk fixture drift, fix planned. | The helper's current required schema includes the anomaly table, and adding it keeps the cancellation fixture aligned. |

**Repair plan:** `docs/plans/issue-72-plan-84-ocr-test-coverage-repair-r2.md`. This plan changes tests only; it does not claim or imply production code changes.
