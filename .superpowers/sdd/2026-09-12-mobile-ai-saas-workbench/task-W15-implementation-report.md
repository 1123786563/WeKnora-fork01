# W15 implementation report — provider delivery, receipts, retry backoff

## Baseline

- Worktree: `.worktrees/react-multiclient/.sdd-worktrees/w15`
- Branch: `codex/mobile-w15`
- Base: `927faf3d` (W14 accepted implementation/fix chain)
- Scope: core notification delivery; provider/live and native evidence remain environment gated.

## Changes

- Added `internal/notification` provider-neutral contract (`PushPayload`, `PushReceipt`, `PushProvider`, `ProviderError`) and failure classification for permanent token failures, rate limits, transient transport failures, and unknown responses.
- Added Expo-compatible HTTP provider with bounded response parsing, per-ticket receipt validation, `Retry-After` handling for 429, and no token logging. Missing receipt IDs are never treated as success by the receipt path.
- Extended notification delivery persistence with `next_attempt_at`, `receipt_id`, and `last_error`; added migration `sqlite/000060` and versioned `000138`.
- Added fenced `RetryAt`, `AckReceipt`, and permanent `Expire` operations. Delivery worker now prefers receipt-aware providers, persists receipts, expires permanent provider failures, and uses capped exponential backoff with stable per-delivery jitter and provider retry hints.
- Added mobile notification configuration fields and environment overrides. Existing scoped gateway wiring remains fail-closed when no endpoint is configured.

## RED / GREEN

- RED: `go test ./internal/notification -run TestPushClassification -count=1` — failed with `undefined: ClassifyPushFailure`.
- GREEN: `go test ./internal/notification -count=1` — PASS (classification and Expo mock HTTP tests, including 429, device invalidation, missing receipt).
- GREEN: `go test ./internal/config -run '^$' -count=1` — PASS.
- Static: `gofmt` and `git diff --check` — PASS.
- Workbench focused test command is currently blocked by an unrelated pre-existing W20 compile collision in `internal/application/service/workbench/remote_dispatch.go` (`dispatch` field/method name conflict). This is recorded as `blocked-env`/dependency evidence and was not changed by W15.

## Known limits

- No real Expo/APNs/FCM credential or device was available; no live delivery is claimed.
- The existing production gateway resolves encrypted device tokens by scoped identity. The new direct Expo client is provider-neutral and tested against a local HTTP fixture; deployment must supply a configured gateway/provider before delivery is considered live.
- Independent reviewer must verify migration ordering, provider error semantics, fencing, and the receipt-aware worker integration before W15 is accepted.
