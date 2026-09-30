# Craft #107 T01 R5 Assembly Task 2 Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject a stale or inconsistent Craft Task before production RunView material resolution can create a container or OpenCode session.

**Architecture:** Bind the assembly to an authoritative durable Run reader. At ResolveMaterial entry, compare the Task's tenant, owner, session, Run, Workspace, writer epoch, actor and immutable snapshot against the current admitted Run; only then invoke the coordinator. Keep the runtime's later fence check as another guard.

**Tech Stack:** Go, AgentRunStore, RunView coordinator/provider and focused container tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r5-engine-task2-review.md`; approved Craft Spec #107/T01; R5 engine plan Task2.

## Global Constraints

- The current durable Run and writer fence are server-owned authority; Task/prompt/model fields cannot trigger material allocation alone.
- Denials must occur before `coordinator.Resolve` and before any engine/container/session side effect, including a generation allocation.
- Missing registry digest remains fail closed and production route remains disabled. No old workDir fallback, commit/push or unrelated edits.
- Preserve concurrent R4 `craft_runtime.go` and H2 fixture edits; exact task-local checkpoint and independent review required.

## Review Focus

- Tenant mismatch between Task scope and fence, empty owner/Run/Workspace or zero/stale epoch must invoke zero engine/session calls.
- Changed durable actor or immutable snapshot must deny even when tenant/owner/session/Run IDs match.
- Lost writer slot/Run status cannot be hidden by a previously valid RunView binding.
- A valid current admitted Run still resolves one exact provider-issued handle.
- A later fence loss remains caught by runtime's independent pre-dispatch recheck.

---

### Task 1: Durable admitted-Run guard before material resolution

**Depends on:** R5 Task2 reviewed FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `internal/container/container.go`, `internal/container/craft_runview_assembly_test.go` only. No RunView repository/provider or R4 runtime edits.

**Consumes / produces:** AgentRunStore.Get, current Task/Run scope and S2 typed snapshot, existing coordinator; produces a pre-side-effect authority check for ResolveMaterial.

- [ ] Capture preimage. RED stale epoch, Task/fence tenant mismatch, changed actor/snapshot, lost writer slot and malformed Task; assert zero RunView store allocation, Docker engine and OpenCode session calls. Keep valid current Run positive case.
- [ ] Inject/reuse durable Run reader and compare immutable admitted identity, snapshot, actor and current epoch/status before coordinator.Resolve; fail closed on missing/ambiguous fields. Preserve later runtime guard.
- [ ] Run focused assembly/provider/material tests, compile and full-container package when shared suite is stable; classify known unrelated failures, gofmt/diff check. Save exact checkpoint/report/patch and independent review.

**Acceptance / failure handling:** Medium finding closed without changing production feature toggle or registry pin requirement. Any missing canonical actor/snapshot API is reported as a concrete interface seam before widening ownership.
