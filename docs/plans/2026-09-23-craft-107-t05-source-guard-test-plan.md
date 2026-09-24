# T05 Source Guard Contract Reconciliation Plan

> **For Codex:** SDD RED → GREEN → REFACTOR, exact checkpoint and independent Review; no commit.

**Goal:** Resolve the persistent full-service failure in `TestCraftSourceGuardDocumentContentCannotWidenPermissions`: test expects bundle keys `Sources,Truncated` but current `craft.KnowledgeBundle` deliberately serializes `Empty` too. Preserve the security assertion that document content cannot introduce execution policy fields.

**Sources:** Approved Spec #107/#124 and current T05 Issue acceptance, `internal/modules/craft/knowledge.go` `KnowledgeBundle`/`BoundSources`, `internal/application/service/craft_source_guard_test.go`, T05 fix4 review, captured T19 full-service log. Confirm `Empty` is part of approved knowledge contract before changing expectation.

**Global Constraints:** backend_implementer owns only `internal/application/service/craft_source_guard_test.go` and a focused report; production source edits require a separate controller amendment. Other agents edit T08 `craft_session.go` and T19 Run repository files in integration. Preserve their changes, no commit/subagents. This is a test-contract reconciliation, not permission to weaken the prompt-injection/source ACL checks.

**Review Focus:** `Empty` is a bounded data-status boolean, cannot carry tool/network/sandbox policy or raw content; exact top-level and per-source key allowlist remains explicit. Test must fail if arbitrary policy fields are added. In a nonempty bundle `Empty=false`; empty case is covered elsewhere. Full service suite still may be temporarily RED from T08 concurrent source; capture exact test evidence.

## Task 1 — reconcile data-only allowlist

1. RED: run the exact failing test and record `[Sources Truncated Empty]` mismatch. Inspect approved Spec and current bundle contract; if `Empty` is not approved, stop and report conflict rather than changing the test.
2. GREEN: add only `Empty` to the top-level allowlist and assert its value is false for this nonempty bundle. Retain the per-source provenance allowlist and verbatim excerpt assertion.
3. Run the exact test, affected knowledge/service tests if package source stable, `git diff --check`, exact file hash/checkpoint/report. Independent reviewer checks that no security assertion was removed.
