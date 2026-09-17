# W15 empty-token independent review

- Reviewed commit: `fed0fd1f` (`fix(notifications): isolate empty batch tokens`)
- Baseline: `ad3ff2d4`
- Reviewer: independent reviewer; no product code was modified.
- Scope: resolver empty/blank-token classification, batch isolation, single-send rejection, Expo payload filtering, focused tests, race, vet, and diff checks.

## Verification evidence

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go test -race ./internal/notification -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools go vet ./internal/notification` — PASS.
- `git diff --check ad3ff2d4..fed0fd1f` — PASS.
- The focused workbench tests for the new empty-token path were attempted, but package compilation is blocked before test execution by the pre-existing W20 collision in `internal/application/service/workbench/remote_dispatch.go` (the `dispatch` field and method share a name). This review does not claim workbench package test/race/vet success.
- No live Expo/APNs/FCM credentials or native devices were available; live delivery remains `blocked-env`.

## Spec checks

1. **Empty/blank resolver output is fail-closed.** `PushNotificationProvider.SendReceipt` trims the resolver result and returns `ProviderError{Code:"InvalidProviderConfig", Retry:false, Revoke:false}` without invoking the provider. `SendBatch` applies the same check to each resolved token.
2. **Resolver failures preserve configuration semantics in batch.** A resolver error is retained as a per-delivery `InvalidProviderConfig` result with `Retry:false` and `Revoke:false`; it is omitted from the provider batch.
3. **Valid siblings are unaffected.** `SendBatch` submits only valid items, retains the empty-token delivery result by durable delivery ID, and preserves the valid sibling receipt. The added workbench test covers a whitespace token plus a valid token.
4. **Single-send empty token is rejected.** The adapter rejects an empty/blank resolved token before calling `PushProvider.Send`. The direct Expo provider also rejects an empty token before HTTP submission.
5. **Expo never serializes empty tokens.** `ExpoProvider.SendBatch` filters blank tokens before constructing `expoMessage` values and returns an item-level non-retryable, non-revoking `InvalidProviderConfig` result. The added HTTP fixture asserts that the request contains only the valid token and that result ordering is restored.
6. **No unrelated product changes.** The commit is scoped to the notification adapter/provider and their tests; `git diff --check` is clean.

## Decision

- **Spec: PASS**
- **Quality: APPROVED**
- **Blocker:** the W20 `remote_dispatch.go` compile collision prevents runtime execution of the workbench package tests in this worktree. This is an external dependency/blocking-environment condition, not a finding against `fed0fd1f`.
- **Live provider evidence:** `blocked-env`; no live delivery claim.

W15 empty-token behavior is ready to advance once the existing W20 compile blocker is resolved or an equivalent package-level test environment is available.

