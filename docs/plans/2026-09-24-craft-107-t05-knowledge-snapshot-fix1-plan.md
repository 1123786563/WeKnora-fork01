# T05 knowledge snapshot one-KB cardinality fix 1

> **For Codex:** Close only the scoped Medium finding in `2026-09-24-craft-107-t05-knowledge-snapshot-task1-review.md` with TDD, exact uncommitted checkpoint and independent re-review.

**Source:** approved Spec #107/T05, current `CraftRunRequest.KnowledgeScope` zero-or-one selection, T05 snapshot Task1 plan/review. Record current HEAD and pre-task owned-file content/hashes. No commits.

## Global Constraints

The immutable selection is empty or exactly one canonical KB ID at this Ticket seam. Do not silently broaden to multi-KB because `BuildForKnowledgeBases` accepts a slice; future multi-select requires a separate API/Spec decision. Preserve original query and T08 actor behavior. No Craft admission/Publisher edits in this fix.

## Review Focus

Builder/parser both reject two IDs, duplicate, null/oversized/noncanonical, old Craft explicit empty and generic compatibility. No accidental prompt text change.

## Task 1 — cardinality at durable boundary

**Depends on:** Task1 scoped review FAIL Medium. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/application/service/agent_run_graph.go`, `agent_run_graph_test.go` only. **Consumes:** typed `CraftKnowledgeSelectionSnapshot`. **Produces:** zero-or-one canonical selection invariant both building and parsing.

1. RED: replace two-ID success expectation with rejection in builder and parser; one ID and empty still round-trip.
2. GREEN: enforce `len(KnowledgeBaseIDs) <= 1` at both boundaries; keep all prior strict validation and query equality.
3. Run focused snapshot/T08 actor tests, `git diff --check`, exact task-local patch/hash/report.

**Acceptance:** no two-KB snapshot can be created or consumed under the current one-KB contract. **Failure handling:** do not coerce or truncate multiple IDs; return a typed invalid-input error.
