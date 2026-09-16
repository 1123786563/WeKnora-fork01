# W14 fix report round 4

## Commit

- `d1fbcecb` — `fix(notifications): fence interaction delivery outcomes`

## Fixed findings

1. `NotificationStore.RevalidateDelivery` now fails closed for `interaction_requested` deliveries when the immutable event payload cannot identify the pending interaction. It extracts `pending_id`/`interaction_id` from the durable event payload and adds an atomic `workbench_interactions` predicate requiring the same tenant, run, owner, pending status, non-revoked state, and unexpired interaction. A resolution racing after claim therefore prevents the provider call.
2. `NotificationDeliveryWorker.RunOnce` now observes every `Retry` and `Ack` boolean. A lost fence is returned as an explicit `notification_delivery_retry_fence_lost` or `notification_delivery_ack_fence_lost` error after the batch, while stale workers cannot mutate the newer state.

## Tests

With the unrelated W20 `remote_dispatch.go` compile collision temporarily excluded from the package command:

```text
go test ./internal/application/service/workbench -run 'TestNotificationDelivery(RejectsResolvedInteractionAfterClaim|ReportsLostAckFence|ReportsLostRetryFence)' -count=1 -v
PASS
```

The full workbench package remains blocked by the pre-existing W20 field/method collision in `remote_dispatch.go`. The existing `TestNotificationDeliveryConcurrentWorkersSendExactlyOnce` also remains failing in this branch with fence 1 instead of the test's expected fence 2; this is retained as baseline evidence and was not changed by this fix.

`git diff --check` passes and the worktree is clean after the scoped commit.
