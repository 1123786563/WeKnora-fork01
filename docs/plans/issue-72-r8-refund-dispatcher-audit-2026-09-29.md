# Issue #72 R8 Refund Payout Dispatcher Source Audit

**Observed:** 2026-09-29 00:33 UTC  
**Repository revision:** `9899372a727cdc8651c34873d9ea3a491d9f933f` (`codex/issue-72-lago`)  
**Method:** Read-only source, constructor, route, worker, and test-callsite inspection. No tests or services were run.

## Findings

- `RefundStore.ApproveRefund` in `internal/modules/commercial/repository/commercial/refund.go:270-337` validates/locks and changes refund state in a transaction, then inserts a pending outbox row with `kind="refund_payout"`, `event_key="refund_payout:" + refundID`, and refund/tenant/order/line/amount/credits facts. Generic `insertOutboxEvent` (`internal/modules/commercial/repository/commercial/outbox.go:57-77`) provides unique-key insertion but no producer fence or allowlist.
- `RefundService.ProcessPayouts` and `driveRefund` (`internal/modules/commercial/service/commercial/refund.go:157-220`) scan refund rows by refund state; they do not select `commercial_outbox_events`, claim a lease, increment event attempts, compare an event lease token, or update payout event state. `pending` invokes the provider with the stable refund identity; `reviewing` re-queries after prior attempts; `revocation_pending` retries precise-credit revocation.
- Repository-wide production caller/wiring inspection found no caller of `ProcessPayouts`, scheduler/ticker, CLI, queue consumer, container startup hook, or background worker. Search hits for `ProcessPayouts` are service tests (`internal/modules/commercial/service/commercial/refund_test.go:272,321,332,358`). `NewRefundService` is constructed in `internal/handler/commercial.go:80-86` with nil gateway/provider/eligibility in the blocked-environment wiring. Refund HTTP handlers expose request creation and review/approval (`internal/handler/commercial.go:750-820,877-914`) but do not dispatch payouts.
- The only commercial background loop found in container wiring is fulfillment (`internal/container/container.go:959,2492-2501`); its selector (`internal/modules/commercial/service/commercial/fulfillment.go:228-258`) is restricted to `kind="fulfill"` and does not consume refund payouts.
- `RefundStore.MarkRefundChannelResult` (`internal/modules/commercial/repository/commercial/refund.go:339-387`) performs guarded refund state/version transitions, stores provider refund identity, and marks the payout outbox event Sent after the refund leaves Pending.

## Operational conclusion and boundary

The inspected source/wiring supports the exact conclusion **“no autonomous refund payout dispatcher was found in inspected repository source and wiring.”** It does **not** prove that a deployed external process, scheduled job, operator command, or out-of-repository service never calls `ProcessPayouts` or an equivalent provider API.

The R8 upgrade gate must therefore require target-deployment inventory of processes, jobs, CLI/operator procedures, and external provider activity; identify and fence every payout path; and prove queued work plus in-flight provider calls have drained. If any caller, queue, process, or provider request cannot be identified or proven quiescent, the gate remains **BLOCKED**. A local outbox count cannot establish dispatcher absence or payout-call drain.

## Coverage limits

This is repository-source inspection only. It is not production configuration review, deployed process inventory, scheduler inspection, provider audit, drain evidence, or authorization to switch workers. No autonomous dispatcher absence is inferred from lack of a repository callsite.
