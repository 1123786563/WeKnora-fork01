# T19 durable unknown-usage cancellation fix report

**Status:** exact uncommitted checkpoint ready for independent re-review. No commit created.

## Change

`CraftModelGateway.recordCall` now appends the physical attempt with `context.WithoutCancel(c.Request.Context())` wrapped in an explicit three-second timeout (test-overridable through the gateway's private timeout field). This preserves request trace/context values while preventing inbound cancellation from suppressing a post-send usage fact indefinitely. Failed appends emit a structured server error event (`craft_model_gateway_usage_record_failed`) with only safe reconciliation identities (`run_id`, `call_id`, `attempt_id`) and the error. The response header remains supplemental. Credentials and request/model payload are not included in the event.

The activity journal still resolves body/transport ambiguity to `unknown`; append failure does not promote or clear that state, and the existing same-activity fence prevents another provider send.

## RED → GREEN evidence

With the old request context restored temporarily, the failure-context regression test failed because there was no append deadline. The service-backed cancellation test also failed as intended: after the provider flushed response headers and the inbound request was canceled during body read, SQLite had zero usage facts.

Final verification:

- `go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1 -timeout=90s` — PASS (`10.502s`). Includes real migrated SQLite `CraftUsageService`/`CraftUsageStore`, cancellation after provider headers, durable single unknown fact with zero fabricated totals, unknown journal outcome, and replay blocked before a second provider request.
- `go test -race ./internal/handler -run 'TestCraftGatewayPostSendAndBodyReadFailuresStayNonReplayable$/^inbound cancellation interrupts response body$|TestCraftGatewayUsageRecordFailureUsesBoundedDetachedContextAndLogsIdentity$' -count=1 -timeout=90s` — PASS (`8.632s`).
- `git diff --check -- internal/handler/craft_model_gateway.go internal/handler/craft_model_gateway_test.go` — PASS.

The bounded failure test verifies the recorder gets a deadline, context values survive detachment, the deadline terminates a failing append, the journal remains `unknown`, structured logs carry safe identities without credentials or payload, and the activity cannot be sent again.

## Exact checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`.
Starting HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`. These two handler files were already dirty from the reviewed two-phase correction; task-start hashes and pre-task staged/unstaged patches are recorded in the manifest/artifacts. The task-local patch is the diff from the reconstructed pre-task owned-file content to the final content. The checkpoint patch is the exact combined diff from HEAD and includes those documented earlier changes.

No files outside the owned handler source/tests and task report/checkpoint artifacts were edited by this task. Concurrent worktree changes remain untouched.
