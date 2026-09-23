# T08 Central A Fix 1 Implementation Plan

> **For Codex:** Use superpowers:subagent-driven-development and TDD. This is a scoped correction of independent T08 central A findings A-1/A-2, before existing content-path support B.

**Goal:** Craft feature routes, including T08 access, are owned by one container/router assembly and cannot capture a prior assembly's service. A test must traverse the actual production composition seam and auth policy, not only a manually wired raw Gin group.

**Architecture:** Provide a fresh `session.CraftFeatureRoutes` per assembly and pass it through container registration and router construction. Keep unique-name/path restrictions and freeze-after-mount within that instance. Avoid a mutable process-global registry for production wiring; legacy test helpers may remain if safely isolated. Missing/duplicate provider or route registration must fail at boot.

**Tech Stack:** Go dig container, Gin router, Craft feature route registry, migrated SQLite access service.

## Global Constraints

- Integration Worktree `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`, HEAD `929fb287e5e0ed29c6637b118fbdf0d009232728` plus uncommitted T08 service/UI copy, migration and central A. Before checkpoint `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-central-a-checkpoint-01/manifest.json` SHA-256 `a16eb84d5881324b76b12949dc4bc248d8cd7f6babb5bc81572608bbc42bde97`.
- Read `docs/plans/2026-09-23-craft-107-t08-central-a-review.md`, prior central A plan/report, approved Spec/#126, T00 feature route contract/tests. No commit/push/merge.
- Exclusive central files for this fix: `internal/container/container.go`, `internal/container/craft_access_wiring.go`, `internal/container/craft_features.go`, `internal/handler/session/craft_features.go`, `internal/handler/session/craft.go`, `internal/router/routes_chat.go` and focused tests. No edits to T01 `craft_session.go` or `craft_runtime.go`, T08 access service/domain/handler, T14 preview, migration, frontend or other agent files. You are not alone; preserve other work; no subagents.
- Report to `docs/plans/2026-09-23-craft-107-t08-central-a-fix1-report.md` with RED/GREEN, exact tests/hashes and any API compatibility changes.

## Review Focus

1. Two sequential container/router assemblies in one process each mount exactly one access feature and use their own concrete `CraftAccessService`; first failing before mount does not poison second. Duplicate access name within one assembly fails.
2. Actual dig registration/invoke and production guarded session route seam are exercised. Unauthenticated and disallowed API-key requests cannot reach access; tenant admin without grant receives 403; owner grant/revoke produces audit.
3. T00 path-validation/escape protections and existing unrelated Craft routes remain intact. Avoid test-only resetting a global production registry as the implementation.

## Task 1 — RED composition tests

Add a two-assembly/retry test and a production composition/auth route test. Capture failures under current global registry. If full `BuildContainer` has external startup effects, exercise the exact extracted production constructor/registrar/router seam, not a manually wired service plus raw Gin group.

## Task 2 — GREEN instance-owned registry

Inject/forward a fresh registry through the container to router mounting; maintain constrained `CraftRouteGroup`. Keep existing public helper behavior only if it cannot influence production state. Make registration order before route build explicit, and return boot errors on duplicates/missing dependencies.

## Task 3 — Verification and report

Run focused container/router/session route tests, original T00 route safety suite, T08 service/migration tests and `git diff --check`. Report exact changes/hashes and central B limitations. Controller checkpoints the incremental fix and requests reviewer/backend validator before T08 can be verified.

## Controller ownership amendment — router constructor seam

Per-instance route registration requires the fresh `session.CraftFeatureRoutes` instance to reach `router.NewRouter`. The controller grants this fix exclusive narrow ownership of `internal/router/router.go` for adding the dig-injected feature registry field to `RouterParams` and forwarding it to `RegisterSessionRoutes`; no other active agent edits this file. Existing router behavior and unrelated params must remain unchanged. Add a focused production router constructor/auth test and include this file in the incremental checkpoint/review.
