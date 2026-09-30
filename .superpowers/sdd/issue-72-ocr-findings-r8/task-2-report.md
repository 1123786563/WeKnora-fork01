# R8 Task 2 Report — paid top-up fulfillment attention

**Workspace / branch:** `/Users/wuyongjun/.codex/worktrees/issue72-r8-late-success/WeKnora-fork01`, `codex/issue-72-r8-task2`

**Base:** `d907ceb04604f5a98dd1c1e71036ad8bf9fdc700`

## RED → GREEN

- Added unquoted contradictory-winner and missing-winner assertions for one open `invalid_winning_payment` exception, Pending outbox state, unchanged paid state, and zero external grant calls for the invalid event. The contradictory-winner test repairs the event to the registered immutable transaction and verifies one benefit grant, Sent event, and resolved exception.
- Added an exception-storage failure case; failure leaves the event Pending, writes no exception, and calls no benefit gateway.
- Baseline failure was established by source path inspection, not by running the new assertions against a reverted baseline: prior `fulfillEvent` routed an invalid unquoted winner to `quarantineFulfillEvent(..., "invalid_winning_payment")`, which sets the outbox event Dead and stores no exception. This is the precise behavior the new assertions reject.
- GREEN focused tests and full package results are recorded below.

## Implementation evidence

- `FulfillmentExceptionRow` is migrated at `NewFulfillmentService` startup and has SQLite and versioned up/down migrations. It uses the internal outbox event key only as its primary key; the opaque random ID is the API identity.
- Invalid unquoted winners idempotently create/reuse one `top_up` / `invalid_winning_payment` / `open` exception and release the event as Pending in one transaction. Existing `FulfillmentRecord` rows are not modified and no `PaymentAnomalyRow` is created.
- Verified completion changes the paid order to fulfilled, resolves an open exception, and marks the event Sent atomically. Applied fulfillment receipts are not downgraded.
- `GET /admin/fulfillment-attentions` is mounted behind `RequirePlatformRefundReviewer`; no manual resolve route exists. The response contains only ID, order/tenant IDs, kind, fixed reason, state, and timestamps. Tests verify the event key and provider/merchant/transaction/payload/credential fields are absent. Unauthenticated and space-admin requests receive 403.
- Fulfillment event identities are omitted from fulfillment error wrappers and quarantine logs.
- Startup migrations: service `AutoMigrate` includes `FulfillmentExceptionRow`; explicit migration pairs are `migrations/sqlite/000110_*` and `migrations/versioned/000189_*`.

## Verification

Commands and results on the final source version:

- `go test ./internal/modules/commercial/service/commercial -run 'TestFulfillmentTopUp(RejectsContradictoryWinningAttempt|MissingWinningAttemptIsRetained|ExceptionStorageFailureDoesNotAcknowledge)$' -count=1` — PASS.
- `go test ./internal/modules/commercial/service/commercial -count=1` — PASS.
- `go test ./internal/handler -count=1` — PASS.
- `go test ./internal/router -count=1` — PASS.
- `git diff --check` — PASS.

Behavior exercised: invalid paid top-up does not change `paid`, never reaches `FindBenefit`/`ApplyBenefit`, retains one exception across repeated drains, storage failure does not acknowledge, and a corrected exact winner grants once and resolves atomically. The handler test checks the closed/sanitized response and both unauthenticated and tenant-admin denial. Existing commercial service and handler suites also pass.

## File hashes (SHA-256)

```
e58f3be505ec04a8f270d3716aa514a36d8d84a59fcb1e1fb372c088cd2503aa  internal/modules/commercial/service/commercial/fulfillment.go
90091192adba3d443cbf38cc9f2e3245b4ba9d011daad0a5a200ee10bb7c3ea4  internal/modules/commercial/service/commercial/fulfillment_test.go
a10d2af1fd253bf6be26ed915f3b0511b0f8c0919c0c2f27ba97ddb1ae2615db  internal/handler/commercial.go
028e6965247a4ab6c06c6771ca4c37749ae7c6c1b0144b29608c0c00f15f9497  internal/handler/commercial_anomaly_test.go
43f2d064087898ef5e6a3a59769c78b05d09f237011f8bcd85c70f48d994e1bc  internal/router/routes_commercial.go
ccf238910b2915215b26271a28f0065cef12547f7e54a778b076e984f73a4ee2  migrations/sqlite/000110_commercial_fulfillment_exceptions.up.sql
6a67323850baac981824e1c9c2f14ad9fcfcb40986fb48c8d940adc612095512  migrations/sqlite/000110_commercial_fulfillment_exceptions.down.sql
2a0be00b0e7109ac8622c034e79b0f265fa8bcc541c6e6121771f9c2a005bd74  migrations/versioned/000189_commercial_fulfillment_exceptions.up.sql
6a67323850baac981824e1c9c2f14ad9fcfcb40986fb48c8d940adc612095512  migrations/versioned/000189_commercial_fulfillment_exceptions.down.sql
```

## Limitations

- The implementer initially verified old Dead disposition from source rather than running RED. This evidence gap was resolved by the controller’s isolated-baseline RED test documented below.
- PostgreSQL runtime migration/integration was not available in this local verification. The versioned SQL is additive and startup model migration is covered by the SQLite service setup.

## Supplemental RED evidence (controller, isolated baseline)

The implementation report originally noted that the pre-change behavior was checked by source inspection only. To complete the plan's RED evidence, I reused the now-free managed Task 6 worktree as an isolated baseline checkout at exact Task 2 BASE `d907ceb04604f5a98dd1c1e71036ad8bf9fdc700` and added a temporary test file `internal/modules/commercial/service/commercial/fulfillment_task2_red_test.go` (SHA-256 `4dcc75590731e9adbd1dfffe345267050733869e41edb1600329d4148b41787e`). The test seeded a paid top-up, removed its registered payment attempt, drained fulfillment, then asserted that the unverified event remained Pending. It compiled and failed on the intended behavior:

```text
go test ./internal/modules/commercial/service/commercial -run '^TestR8Task2RedUnverifiedPaidTopUpRemainsPending$' -count=1 -v
fulfillment_task2_red_test.go:25: unverified paid top-up must remain pending for operator attention, got state=dead
--- FAIL: TestR8Task2RedUnverifiedPaidTopUpRemainsPending
```

The temporary test was removed after the baseline run; no baseline source file or commit was changed. The four Task 6 implementation commits were preserved under branch `codex/issue-72-r8-task6-reviewed`, and the baseline worktree is clean at the recorded BASE.
