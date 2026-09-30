# T05 Source Guard Contract Reconciliation Report

**Plan:** `docs/plans/2026-09-23-craft-107-t05-source-guard-test-plan.md`  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Checkpoint:** `t05-source-guard-contract-uncommitted-01`  
**HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)  
**Role metadata:** Runtime did not expose role/model/reasoning metadata.

## Contract check and change

The approved Craft Spec requires bounded selected source material and traceable source evidence, and does not prohibit a data-status field. The current `internal/modules/craft/knowledge.go` contract defines `KnowledgeBundle.Empty`, and `BoundSources` sets it to true exactly when the bounded bundle contains no sources. The prompt-injection test constructs a nonempty bundle, so `Empty=false` is its correct status. No production source was changed.

Updated only `internal/application/service/craft_source_guard_test.go`. The top-level allowlist now includes the status-only `Empty` field and asserts it is false. The exact allowlist still rejects any undeclared bundle key. The nested source allowlist explicitly covers only declared provenance: `ID`, `Ref`, `Excerpt`, `Digest`, `TenantID`, `AcquiredAt`, and `Truncated`. The excerpt remains asserted verbatim, including its attempted policy escalation text.

## RED → GREEN evidence

- RED, exact plan target: `go test ./internal/application/service -run '^TestCraftSourceGuardDocumentContentCannotWidenPermissions$' -count=1 -v` failed because actual top-level JSON contained the additional `Empty` key.
- After adding `Empty`, the same test exposed two existing serialized source fields omitted by the old nested allowlist: `AcquiredAt` and `Truncated`. Both are fields in the current `craft.Source` provenance contract, so the exact nested allowlist was reconciled to those declared data fields; no policy fields were admitted.
- GREEN, exact plan target rerun: `go test ./internal/application/service -run '^TestCraftSourceGuardDocumentContentCannotWidenPermissions$' -count=1 -v` — PASS.
- Affected source/knowledge/service tests: `go test ./internal/application/service -run '^(TestCraftSourceGuard|TestCraftT05|TestCraftKnowledge)' -count=1` — PASS (`ok`, 0.778s).
- Formatting: `gofmt -d internal/application/service/craft_source_guard_test.go` produced no diff. Scoped `git diff --check` — PASS.

## Checkpoint

Only `internal/application/service/craft_source_guard_test.go` is modified for this task; the report is the only added task artifact. Other agents' shared changes were preserved.

SHA-256:

- `internal/application/service/craft_source_guard_test.go`: `95dd674642e95771a45073396f92ec39934f518ce5ec22dd86a75922ddcbdcf6`

## Limits

This reconciles the test with the declared data-only bundle contract. It does not claim full T05 production Publisher, RunView isolation, or dispatch authorization acceptance.
