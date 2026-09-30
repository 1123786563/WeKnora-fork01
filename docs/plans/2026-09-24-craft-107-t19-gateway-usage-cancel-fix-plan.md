# T19 gateway unknown usage on canceled inbound request

> **For Codex:** Repair only the remaining Medium finding in `t19-gateway-two-phase-review.md` with TDD, an exact uncommitted checkpoint, and independent re-review.

**Sources:** reviewed T19 two-phase plan/report/review, approved Spec #107/T19. Record current integration HEAD and pre-task owned-file hashes. No commits.

## Global Constraints

The provider request is one-shot. The journal remains `unknown` on post-send cancellation and remains `intent` if resolution fails. The usage record is a physical-attempt fact and must be attempted under a bounded context independent of inbound cancellation, retaining trace values. No fabricated token totals. Recording failure must be observable to operations/reconciliation; do not claim an HTTP header reaches a disconnected caller.

## Review Focus

Cancellation after provider acceptance, durable nil-usage append, bounded write lifetime, genuine production store context handling, journal/usage consistency on DB failure, no second upstream call.

## Task 1 — durable cancellation accounting

**Depends on:** `t19-gateway-two-phase-review.md` remaining Medium. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/handler/craft_model_gateway.go` and `_test.go` only. **Consumes:** `CraftUsageService.RecordPhysicalCall` and existing `unknown` journal state. **Produces:** independently bounded persistence context and service-backed cancellation test.

1. RED test with real SQLite-backed usage store/service (not context-ignoring fake): cancel inbound after provider headers/body start, assert journal `unknown`, one physical unknown usage row durable, no second provider request. Add a bounded persistence-context failure test showing journal stays unresolved and failure is logged or otherwise operationally visible.
2. Use `context.WithoutCancel(c.Request.Context())` plus a short explicit timeout for the usage append. Preserve trace values, record nil usage for ambiguous send, and emit structured server log on failure with safe Run/activity identity. Do not place credentials or model payload in log. Keep existing response header only as supplemental feedback.
3. Run focused handler/service tests and race where practical, diff/format checks; save task-local patch, pre/post hashes and report.

**Acceptance:** canceled inbound context cannot by itself suppress the unknown physical usage fact; journal fencing still prevents replay. **Failure handling:** if the persistence backend is unavailable, keep journal unresolved and emit a durable-identity server error for reconciliation, never reissue model traffic.
