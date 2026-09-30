# Craft #107 T01 R4 Task 2a Run Bound Source Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a read-only artifact source bound to one verified RunView generation while preserving the separate post-terminal draft and T15 candidate gates.

**Architecture:** Resolve the admitted Run and material handle once, then enumerate/read only the verified generation's `output/` via descriptor-relative no-follow operations with identity checks. This task does not call the shared `CraftArtifactService` or advance a draft: those require new reviewed seams because current collection publishes immediately and `DraftHeadStore.Advance` demands terminal/quiescent Run state.

**Tech Stack:** Go, RunView material handle, filesystem descriptor APIs, focused container tests.

**Spec:** Approved Craft Spec #107/T01; `docs/plans/2026-09-24-craft-107-runview-r4-version-plan.md` Task2; R4 Task1 Fix2 review.

## Global Constraints

- Tenant/owner/session/Run/Workspace/fence/generation identity comes from admitted durable state, never model or client input.
- The source reads only the exact verified generation `output/`. No shared workDir symlink, previous Run, HOME, input or knowledge directory fallback.
- Reject symlink, hard link, device, path traversal, list/read replacement, changed material identity and stale/foreign Run before returning bytes.
- Do not call `DraftHeadStore.Advance` while the Run/session is nonterminal or publish a Version/default pointer before T15 checks. Keep runtime default-off; no commit/push.

## Review Focus

- Same Task but different Run/generation cannot read the source.
- A changed container/layout/mount identity between list/read rejects.
- Symlink, hard link, special file and descriptor replacement reject.
- Extra sibling output is not silently included or exposed.
- A source built before fence change cannot read after authority is lost.

---

### Task 1: Exact generation output source

**Depends on:** R4 Task1 Fix2 reviewed PASS. **Owner:** `backend_implementer`; **validator:** `backend_validator`, then independent `reviewer`.

**Owned files:** `internal/container/craft_runtime.go` and focused new/existing container source tests. No `craft_artifacts.go`, `container.go` or DraftHead repository/service edits.

**Consumes / produces:** Admitted Run/fence and verified material handle; produces a narrow list/read source for later per-Run collector seam.

- [ ] Capture preimage. RED foreign Run/generation, changed handle, traversal, symlink/hard-link/device, list/read replacement, extra file, stale fence and no shared output pointer.
- [ ] Implement exact descriptor-relative source with regular-file, size, link-count, device/inode and no-follow checks at list and read. Revalidate material/Run identity before both operations. Do not plug it into legacy singleton collector yet.
- [ ] Run focused/race tests and scoped compile, save exact checkpoint/report/patch and independent review. Report inability to bind identity without widening rather than silently broadening.

**Acceptance / failure handling:** Source is independently safe and reviewable. R4 Task2 remains OPEN until a post-terminal draft/candidate seam is designed, implemented and reviewed; no end-to-end A→B or default promotion claim follows.
