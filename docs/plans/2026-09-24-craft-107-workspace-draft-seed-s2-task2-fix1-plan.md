# Craft #107 T01 Workspace draft seed S2 Task 2 Fix 1 Plan

> **For Codex:** Execute by SDD RED → GREEN → REFACTOR; save exact uncommitted checkpoint and obtain independent Spec/quality review.

**Goal:** Close the High fresh Craft admission bypass in `2026-09-24-craft-107-workspace-draft-seed-s2-task2-review.md`. Every admitted Run for a registered Craft Session must carry a server-selected frozen Workspace seed; a missing `craft_input_manifest` cannot be used to bypass head selection.

**Sources:** Approved Craft Spec lines 64–66, S1 head and S2 Task1 reviews, S2 Task2 initial checkpoint and independent review.

## Global constraints

- Integration Worktree only, no commit/push. Preserve concurrent H2/T19 edits.
- The client may not provide `craft_workspace_seed`; fresh registered Craft Session admission requires the durable Craft marker and valid typed snapshot. Existing legacy persisted rows may fail closed at worker execution; do not create new legacy rows as a test convenience.
- Same-key replay remains bound to the original seed/request/actor without rereading the mutable head. Keep head selection inside the Session writer transaction.

## Task 1 — Reject unmarked fresh Craft admission

**Role:** backend_implementer. **Owned files:** `internal/application/repository/agent_run.go` and focused repository/worker fixture tests; no service, H2 or T19 files. **Consumes:** registered Craft Session identity and parsed durable snapshot. **Produces:** either a seeded admitted Craft Run or a denial before row/hold/claim.

1. RED: registered Craft Session plus missing, null, malformed or incomplete `craft_input_manifest` must fail admission without a new Run, even when caller supplies no seed. Demonstrate non-Craft admission unaffected. Move the intended legacy worker missing-seed test to a direct persisted legacy fixture; do not call fresh Admit to construct it.
2. GREEN: base Craft classification on authoritative registered Session identity. Require the marker for any fresh Craft admission, then resolve current head in the existing transaction and persist seed/hash. Reject caller-supplied seed. Keep same-key replay original snapshot and seed unchanged.
3. Add controlled tests for both head-advance/admission transaction orders if practical; at minimum retain a low-risk disclosure if one interleaving cannot be forced. Run focused SQLite, race, service StartRun and worker tests, compile checks, and disposable PostgreSQL behavior if available. Record exact commands, skips, hashes and task-local patch.

**Acceptance:** no fresh unseeded Run through any `Admit` caller for a registered Craft Session; legacy persisted missing seed still fails before model resolution. Independent review must pass. **Failure handling:** keep S2/R4 gated and production execution default-off.

## Review focus

Check authoritative Craft Session detection, absence of optional caller-controlled bypass, transaction atomicity, replay immutability, and non-Craft compatibility.
