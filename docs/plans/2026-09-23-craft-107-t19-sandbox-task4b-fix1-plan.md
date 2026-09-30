# T19 sandbox initiation contract review fix 1

> **For Codex:** Execute only these three independently reviewed findings with TDD and an exact uncommitted checkpoint. Request a read-only re-review before adapter work.

**Source:** `t19-sandbox-task4b-task1-review.md` High/Medium findings and `t19-sandbox-task4b-plan.md`. Original execution BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`; record current HEAD and pre-task owned-file hashes. No commits.

## Global Constraints

The old synchronous `RemoteSandboxClient.Create/Exec` are never a fallback. No provider adapter is enabled by this fix. After any `Start*` call, uncertain errors and invalid receipts remain `unknown` with all available diagnostic receipt data; only validation before invocation can be definitely-not-started. Preserve concurrent edits outside the two owned files.

## Review Focus

No second physical operation after a raw timeout/cancellation or invalid receipt; provider and sandbox provenance before exec start; raw versus typed error classification; correct error chaining for diagnostics.

## Task 1 — close three contract findings

**Depends on:** Task4b Task1 review FAIL. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/modules/execution/sandbox/remote_operation.go` and `_test.go` only. **Consumes:** optional initiation interfaces and existing receipt type. **Produces:** safe pre/post-dispatch classification.

1. RED tests for nil/empty/foreign-provider exec handle with zero `StartExec` calls; malformed/missing/mismatched provider, sandbox, key or ID receipts after `StartExec`/`StartCreate` as `unknown`; raw timeout/cancellation/error after dispatch as `unknown`; typed proven pre-send error only if the contract can verify it.
2. Validate target before invocation. After invocation, preserve received ref when possible and return typed unknown wrapping malformed receipt diagnostics or raw adapter errors. Do not downgrade an adapter-supplied unknown. Distinguish true pre-dispatch validation failures.
3. Run focused and full sandbox package tests, `gofmt -d`, `git diff --check`; save source hashes, task-local patch and report.

**Acceptance:** all three review findings demonstrably closed with call-count and classification tests. **Failure handling:** if the interface cannot honestly distinguish proven pre-send, conservatively classify every returned post-invocation error unknown and document that contract.
