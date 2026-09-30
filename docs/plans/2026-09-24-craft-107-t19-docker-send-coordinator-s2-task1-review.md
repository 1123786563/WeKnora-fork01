# T19 Docker send coordinator S2 Task 1 — independent review

Reviewed 2026-09-24 against the approved Craft spec's Task Budget and recovery acceptance, `CONTEXT.md`, ADR-0004, the Docker exec recovery design, S2 plan, S1 review, Task 1 report, and exact checkpoint. Scope is the five checkpointed Go files. This review made no source, test, requirement, or remote issue changes and did not invoke OCR.

## Findings

### S2-R1 — High — legacy intent adoption bypasses exclusive send ownership and unresolved fencing

- **Evidence:** `CraftDockerSendCoordinator.Prepare` (`internal/application/service/craft_docker_send_coordinator.go:88-126`) accepts any matching `intent` with the same grant and call binding and no receipt. It has no durable discriminator that the row was prepared by this coordinator. `CraftBudgetService.BeginBinding` and `StartBinding` (`internal/application/service/craft_budget.go:185-264,271-325`) create the same journal shape and can retain an outstanding external-start attempt. `resolveCraftChargeStart` (lines 219-235) updates any matching `intent` to `started`, `unknown`, or `definitely_unstarted` without checking `provider`, receipt, or `send_claimed_at`. Sequence: a legacy attempt commits intent/hold and begins its callback; the coordinator adopts that intent, binds and claims a Docker receipt; the legacy callback can send independently and then resolve the claimed row. The claim CAS only excludes another coordinator claim; it cannot exclude that earlier callback or keep its row unresolved.
- **Impact:** Two physical start paths can exist for one activity. A legacy `definitely_unstarted` or `started` resolution can remove the journal `intent` fence after the Docker claim even though Docker acceptance remains ambiguous. This violates one-send authority, no backwards-compatibility bypass, and hold/fence preservation.
- **Smallest defensible correction:** Give coordinator preparation a durable, exclusive protocol marker before returning an operation, and permit pre-receipt replay only for that marker. Ensure legacy resolution cannot transition a receipt-bound or send-claimed row; coordinate the marker/resolve guard in the same database transaction or an equivalent CAS. Add a regression test with an outstanding legacy attempt and coordinator replay/claim, asserting no second permission and no postclaim legacy resolution.

### S2-R2 — Medium — PostgreSQL claim and Run-lock behavior remains unverified

- **Evidence:** The report records `TRPC_TEST_POSTGRES_DSN` unset. Repository tests now iterate SQLite/PostgreSQL (`craft_docker_send_claim_test.go:53-57`), but `openRunTestDB` skips the PostgreSQL branch without that DSN. Coordinator tests use `openCraftBudgetTestDB` and only exercise SQLite. My focused rerun passed both packages, but did not provide a PostgreSQL DSN. S1-R1 identified the same missing engine evidence, and S2 added a transaction with a no-op Run-row update and status check (`craft_docker_send_claim.go:48-157`) whose lock behavior matters under concurrency.
- **Impact:** The exact PostgreSQL transaction, lock ordering, receipt constraints, stale Run fence, and one-owner race have no behavioral acceptance evidence. SQLite success cannot close this cross-engine gate.
- **Smallest defensible correction:** Run the repository test matrix against a disposable PostgreSQL schema with `TRPC_TEST_POSTGRES_DSN`, including concurrent claims and a concurrent Run status/revision transition. Record server version and results; add PostgreSQL coordinator integration coverage if its service-level behavior differs.

### S2-R3 — Medium — physical send and cancellation behavior is asserted only by a token counter

- **Evidence:** `TestCraftDockerSendCoordinatorClaimIsOneOwnerAndReplayCannotResend` (`craft_docker_send_coordinator_test.go:83-109`) increments `sendCalls` directly after `Permission.Consume()`. It never invokes a transport callback; its canceled context is created and checked after the claim but is not passed through a send path. No coordinator method accepts or bounds transport. The S2 plan Step 1 explicitly requests physical send callback counting across crash/replay simulations and Step 2 requests ambiguous postclaim errors to route to unknown.
- **Impact:** The test proves one durable permission and one token consumption, but does not verify that later caller code can issue at most one `ExecStart`, preserve the hold on cancellation, or bound a send. These remain S3 integration requirements; S2's stated behavioral acceptance is incomplete.
- **Smallest defensible correction:** At the S3 adapter seam, exercise the actual send callback or fake transport through the token consumption path under concurrent replay, crash, cancellation, and ambiguous response. Keep the claimed row unresolved and assert one physical invocation. Do not mark the S2 physical-send assertion verified from the current counter alone.

## Spec and quality verdict

**Spec compliance: not yet accepted.** For coordinator-owned rows, preparation commits the journal intent and dispatched hold before returning, receipt binding is immutable, and the repository CAS grants only one token while the Run row is locked in the same transaction. Claimed replay remains observation-only in this path. S2-R1 is a real cross-path violation of the one-send and unresolved-fence contract. S2-R3 leaves the physical-send acceptance unproven.

**Code quality: conditional, with a blocking safety defect.** The coordinator has no provider dependency, and no SQL transaction encloses transport. The new Run status/revision fence is enforced atomically with bind/claim in the repository. The five current files match the checkpoint's SHA-256 hashes exactly. The checkpoint has no pre-task hash for shared `craft_budget.go`, so it proves the reviewed postimage but cannot independently establish that its appended method was the sole S2 delta. PostgreSQL behavior is still a required acceptance gap (S2-R2). No production wiring or Docker call is present in this slice, and full T19 acceptance remains downstream.

## Evidence reproduced

- SHA-256 of all five current files matched `2026-09-24-craft-107-t19-docker-send-coordinator-s2-task1-checkpoint.json`.
- `go test ./internal/application/service -run '^TestCraftDockerSendCoordinator' -count=1` passed.
- `go test ./internal/application/repository -run '^TestCraftDockerSendClaim' -count=1` passed; PostgreSQL branch was unavailable without `TRPC_TEST_POSTGRES_DSN`.
