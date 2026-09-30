# Craft #107 T19 Normal Provider Empty Stdin Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the immutable staged distinction between stdin disabled and stdin enabled with zero bytes through inert Docker exec creation and one-send start.

**Architecture:** Add an explicit `StdinEnabled` boolean to the provider create request, independent of `len(Stdin)`. Receipt binds the flag, byte count and SHA-256. Docker `ExecCreate.AttachStdin` uses the explicit flag, and `StartAttachedExecOnce` compares the exact flag/bytes/hash while retaining the one-send permission and output sink behavior.

**Tech Stack:** Go pinned Moby normal exec provider and focused tests.

**Spec:** Craft #107 T19 normal output Task2b Task1/2 independently scoped PASS; Task3 service RED contract discovered 2026-09-24.

## Global Constraints

- Preserve existing nonempty and disabled stdin behavior, bounded input, exact one-send and callback prohibition. No coordinator, output store, migration, service or routing edits. No commit. Exact checkpoint.

## Review Focus

- Enabled-empty and disabled-empty create distinct immutable receipts despite both having zero bytes and SHA-256 of empty input; mismatched flag fails before Docker attach. Existing nonempty stdin and timeout identities remain exact.

### Task 1: Explicit stdin enablement in provider

**Depends on:** normal Task2b Task1/2 reviews PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/modules/execution/sandbox/docker_normal_exec.go` and focused test only.

- [ ] RED tests for enabled-empty create/receipt/start and disabled-empty; wrong flag/bytes on replay reject without Docker I/O.
- [ ] Add explicit request field and exact receipt/start comparison; preserve existing checks and provider API use.
- [ ] Run focused/race tests, package compile and exact checkpoint/report, then independent review. Physical Task3 starts only after review.

**Acceptance / failure handling:** Both empty cases remain distinguishable and safe; if Docker API cannot express enabled-empty, report exact blocker without narrowing staged contract.
