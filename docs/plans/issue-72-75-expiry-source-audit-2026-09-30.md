# #75 expiry prerequisite audit for #86

Date: 2026-09-30 (Asia/Shanghai)  
Audited source checkpoint: `8329b85d4dfff301d03f94406dfc829d87cb5b26`  
Method: read-only `git show <sha>:<path>`; no checkout, tests, services, credentials, or edits to the repository.

## Scope and dependency facts

- Approved inventory says #75 is the T03 Lago wallet-semantics experiment. Its acceptance is: (a1) monthly included credits expire without carry-over; (a2) paid/top-up batches expire 12 months after grant; (b) consumption is earliest expiry then earliest grant; (c) concurrent expiry/consumption/retry does not double-deduct or create negative refundable balance; (d) void/refund removes only the unconsumed remainder. The T03 verdict records a1/b/c/d `PASS-WITH-COORDINATION` and a2 `BLOCKED` by Lago's six-active-wallet cap.
- The approved DAG records `75 → 86`: #86 consumes the T03 measured wallet semantics and must encode expiry rank into Lago `priority`; it cites the T03 verdict as the prerequisite. The inventory's #86 acceptance is: monthly non-carryover and 12-month top-up expiry; unified earliest-expiry consumption; no duplicate concurrent consumption/expiry; and reconciliation with Lago traceable state.
- The current readiness records explicitly say the approved #75-a2 Option B design moves top-up expiry authority into the WeKnora coordinator, but runtime proof of the 12-month behavior remains outstanding and gates #85/#86 expiry evidence. This is a dependency/evidence gap, not proof that the source is absent.

## Static source evidence at the exact checkpoint

The checkpoint contains a coordinator implementation for monthly and top-up batch expiry/order:

- `internal/modules/commercial/repository/commercial/benefits.go:54-72,177-195`: `CreditBatchRow` persists `ExpiresAt`, `GrantedMicro`, `WalletRef`; `ActiveBatches` uses the strict predicate `expires_at > now` (equality is unavailable); `ListBatches` retains expired rows for the breakdown.
- `internal/modules/commercial/service/commercial/benefits.go:392-461`: `EnsureMonthlyCredits` derives the UTC period end as the monthly wallet expiry, persists the registry row, computes the monthly wallet's initial priority from live top-up expiries, then issues the deterministic grant. `:551-630` overlays registry expiry on authority balances: `ExpiresAt <= now` produces zero even if Lago still reports an active lazy-terminated wallet; monthly rows aggregate by period and top-up rows remain individual.
- `internal/modules/commercial/subscription_command.go:94-160`: priority classes and `WalletRank` implement `(ExpiresAt ASC, GrantedAt ASC, WalletRef ASC)` and fail closed above `MaxWalletPriority=50`; the adapter is intended to make Lago's `priority ASC, created_at ASC` order equivalent to the business order.
- `internal/modules/commercial/commercialplatform/lago.go:891-1056,1063-1208`: real adapter creates short-TTL monthly wallets, waits for asynchronous settlement, lists all wallet statuses, and maps wallet metadata/expiry into benefit snapshots. `:1064-1103` explicitly preserves the lazy-termination case as raw authority truth, leaving the registry overlay to the coordinator.

These static facts cover the intended implementation shape for #75 a1 coordination duties and #86's order/overlay model. They do not prove the external Lago runtime or the paid top-up fulfillment path.

## Test evidence present at the checkpoint

### Offline/fake tests (no external services)

`internal/modules/commercial/service/commercial/benefits_test.go` uses `FakeAdapter`, injected clock, and SQLite helper. Relevant tests include:

- `TestMonthlyGrantNewPeriod` (`:446-484`): advances September→October, expects two wallets, September balance zero, October balance one month, and both registry rows retained.
- `TestMonthlyGrantEncodesYieldPriority` (`:638-682`): aging top-up makes monthly initial priority 3; after expiry, next monthly wallet returns to priority 1.
- `TestRefreshRebalancesMixedFamilies` (`:684-735`): aging top-up/monthly/fresh top-up converge to ranks A=1, M=2, B=3.
- `TestExpiredBatchSurfacesZero` (`:737-770`): fake authority remains active with balance after expiry, while the coordinator reports the expired batch as zero.
- `TestBreakdownCrossMonthBatchesCarryGrantedAt` (`:577-636`): after authority termination, the retained expired registry row still has nonzero grant time and zero balance.
- `TestRefreshSyncsLotsFromSnapshot` (`:492-540`): monthly and top-up snapshot rows synchronize into local budget lots.

`internal/modules/commercial/subscription_command_test.go:328-451` covers pure ordering and validation: aging priority, mixed-family rank, same-expiry earliest-grant tie-break, rank-domain overflow/duplicate rejection, and grant priority range. These are deterministic unit tests and require no external service.

### Real-stack tagged test (external service required)

`internal/modules/commercial/commercialplatform/lago_credits_order_integration_test.go:1-35,114-250` is build-tagged `lago_integration`; it skips when `LAGO_INTEGRATION_BASE_URL` or `LAGO_INTEGRATION_API_KEY` is absent. It calls a pinned Lago v1.53 runtime and tests priority creation, top-up snapshot shape, aging-wallet yielding, and real `PUT` rebalance to A=1/M=2/B=3. It does **not** trigger an actual billable consumption event and does not prove expiry after 12 months.

The integration helper (`lago_benefits_integration_test.go:35-120`) uses in-memory SQLite for local service state but a real Lago HTTP API for authority operations. Thus it needs an external Lago stack/API key, but not WeKnora PostgreSQL for this order test.

Historical `docs/testing/lago/credits-order-acceptance.md` records a 2026-09-28 real-stack run, including consumption and reconciliation, but the same document labels it historical and says current exact-hash replay/duplicate contract and stable identity remain unverified. It also explicitly says true-time month crossing was not performed; expiry was covered by object semantics, termination-equivalent simulation, and offline overlay tests.

## Missing proof / validator gap

1. **#75 a2 runtime evidence remains absent:** no exact-checkpoint evidence demonstrates a real paid/top-up wallet expiring at grant+12 months. The source only models/records `ExpiresAt`; the real-stack #86 test seeds a top-up object with a future expiry and checks priorities, not time passage or consumption rejection after expiry.
2. **No source test proves real pre-dispatch consumption rejection at the expiry boundary:** offline `TestExpiredBatchSurfacesZero` proves the local projection overlay, not the real dispatch gate; `TestLagoCreditsOrder` never posts a usage/event invoice. Lago's T03 evidence says an expired-but-not-yet-terminated wallet remains consumable, so this distinction is material.
3. **No current exact-hash proof for #86's full runtime acceptance:** historical flow artifacts are from a different integration HEAD and are explicitly non-current. They cannot by themselves clear the `75 → 86` validator edge.
4. **The source's paid top-up path is preparatory:** the integration test directly POSTs the #85-shaped wallet, so it does not prove payment-confirmed exactly-once fulfillment, source/expiry identity, or response-loss recovery. Those belong to #85 and remain upstream evidence gaps.

## Minimal isolated verification path (recommendation)

The smallest runtime probe that would close the specific #75/#86 expiry evidence gap is an isolated pinned Lago project/customer with synthetic IDs and an operator key:

1. Create one monthly wallet and one top-up wallet through the same adapter/object shape, recording `expiration_at`, `created_at`, metadata, and balances.
2. Use a short-TTL top-up (minutes-scale, as the T03 lab does) to test the **boundary semantics**, while separately preserving the production 12-month payload shape (`expiration_at = grant + 12 months`) for static/date assertion.
3. After the short expiry but before the lazy termination tick, attempt the real consumption trigger (billable metric/plan/subscription/event path used by T03). Assert the coordinator dispatch path rejects the expired batch and the authority balance remains unchanged; then assert an unexpired batch is consumed in earliest-expiry order.
4. Read back Lago wallets/transactions and the WeKnora benefits snapshot, reconcile active authority balances against the projected non-expired batches, and record whether the expired wallet remained active (the expected reason the overlay is required).

This can reuse the tagged integration harness and T03 trigger, but it requires an external Lago stack/API key and must be run at the exact reviewed source hash with fresh synthetic identities. A short-TTL run is evidence of the coordinator's boundary behavior; it is not, by itself, proof that Lago's native 12-month clock can be waited out. The production 12-month date derivation must remain separately asserted from the payload and coordinator registry.

## Conclusion

**Verified static fact:** checkpoint `8329b85d` contains substantial #86 expiry/order implementation and offline tests, plus an external-service-gated priority/rebalance integration test.  
**Verified test fact:** offline tests cover registry expiry overlay, monthly rollover, local lot projection, and deterministic rank calculation; the tagged test covers real Lago priority/rebalance only.  
**Missing proof:** exact-checkpoint real runtime evidence for paid/top-up 12-month expiry and consumption-side rejection/ordering at the expiry boundary. This is the stated `#75 expiry evidence` prerequisite gap and remains a valid validator blocker for full #86 acceptance.
