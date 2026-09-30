# T08 Central Support A Implementation Plan

> **For Codex:** Use superpowers:subagent-driven-development and TDD. This is a scoped central backend integration task needed for vertical T08/#126 verification, executed after reviewed T08 lane code and membership migration were integrated. It does not itself mark T08 complete.

**Goal:** Construct the live Craft Task access service once and mount the T08 grant/list/revoke routes in the authenticated, constrained Craft session route group before router construction.

**Architecture:** `service.NewCraftAccessService(db)` is the single policy instance and is exposed as `craft.TaskAccessChecker`. `container.RegisterCraftFeature("access", mount)` registers `session.RegisterCraftAccessRoutes` at boot. The route remains inside T00's `/:id` GET / `/:session_id` POST Craft path guard, with service permission checks as authority. Do not use a tenant-admin fallback.

**Tech Stack:** Go container/router, real SQLite grant/membership migration-backed test, Gin authenticated route test.

## Global Constraints

- Worktree `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, HEAD `929fb287e5e0ed29c6637b118fbdf0d009232728`. T08 seven reviewed service/UI files are already copied and hash-verified; PG/SQLite frontier migration checkpoint-02 is present but Git-ignored. No commit, push or merge.
- Source authority: approved Craft Spec/#126, `docs/plans/2026-09-23-craft-107-t08-review.md`, `...-t08-fix1-review.md`, `...-t08-fix1-backend-validation.md`, `...-frontier-wiring-map.md`, T08 lane report, T00 route contract.
- Exclusive files: `internal/container/container.go` and new focused container/router tests. May add a small `internal/container/craft_access_wiring.go` for constructor/invoke if needed. Do not edit `internal/application/service/craft_session.go` (T01 fix integration pending), T08 lane-owned files, T14 preview service, frontend, migration or other agents' files. You are not alone; preserve all other edits. No subagents. Do not commit.
- Write report `docs/plans/2026-09-23-craft-107-t08-central-a-report.md` with RED/GREEN, exact tests/hashes, boot route table and remaining direct/list/preview/download checks.

## Review Focus

1. One concrete service instance is supplied to access handler and narrow checker port; duplicate/provider cycles cause boot/test failure rather than silent nil.
2. `GET /:id/craft/access`, `POST /:session_id/craft/access` and revoke route mount through T00 constrained registry on authenticated sessions group, not broad unguarded root.
3. Real service + migrated SQLite HTTP path refuses tenant admin/ungranted user, permits owner grant, and records audit; ensure legacy Craft routes do not collide.

## Task 1 — RED boot/HTTP test

Write a test that constructs live access service and feature registration before a real guarded router. Assert route table, owner grant/list/revoke and denial in persisted DB. Record RED due missing container wiring. Prefer explicit registry instance or reset strategy to avoid process-global test order dependence.

## Task 2 — GREEN assembly

Register provider and feature at the existing container Invoke phase before router build. Keep `craft.TaskAccessChecker` injection available to later central support B without introducing a constructor cycle. Any missing dependency must fail construction. Preserve existing Craft gate default-off behavior for unrelated routes.

## Task 3 — Verification and report

Run focused Go container/router/session access tests, `go test ./internal/application/service -run 'TestCraft(T08|AccessMigration)' -count=1`, and `git diff --check`. Record full file list/hashes, exact exits and remaining central support B obligations: list/direct/session/version/preview/download enforcement and authenticated HTTP/audit journey. Controller snapshots and sends independent review/backend validator before T08 can be marked verified.
