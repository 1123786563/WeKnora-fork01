# T08 actor membership admission fix independent re-review

**Scope:** Exact three-file delta in `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-actor-membership-fix-checkpoint-01/`, compared with T08-A1 in `2026-09-23-craft-107-t08-actor-admission-review.md`, the membership fix plan/report, and the approved actor principal Task 1 plan. Read-only review; no production or test edits.

## Verdict

- **Spec compliance: PASS for Task 1 after this fix.** `actorBelongsToTenant` now requires an active, undeleted `tenant_members` row for the exact Run tenant and actor ID, joined to an active, undeleted user. It no longer uses `users.tenant_id` as proof of authority. Cross-home-tenant active membership is accepted; suspended, soft-deleted, missing membership and inactive/deleted users are rejected. T08-A1 is resolved.
- **Code quality: PASS for this delta.** The query runs in the existing admission transaction and uses the same active membership fields as Craft access. The delta changes only that predicate and focused test fixtures/cases. Owner storage, immutable actor persistence, cross-actor replay, generic actor fallback, and T01 owner-scoped claim/session predicates are unchanged from the Task 1 checkpoint.

## Findings

No critical, high, medium, or low finding in this scoped fix. The earlier T08-A1 finding is closed.

## Evidence and limits

- Exact checkpoint `after.sha256` verified for all three files. Live hashes match the fix report: `agent_run.go` `606f64e479772dd5520420c9a0b000f7e4caf404c9ddf15fe2b59aa023941d72`, `agent_run_actor_test.go` `1c10b70ba042e37e471ba24f60b56cdb2f1c9a0bbd341650f2db41ab4d4aba87`, and `agent_run_test.go` `1ecd83fba3c0c6bb32f62731ccd931429a5d0a439e7ebf844d7464b0efe07b12`. Task-delta patch SHA-256 `9282a6c33e957a009262672ec71eb9539800bcc84ae12e5fca4023ca6b50024b` also matches the report.
- Independently ran `go test ./internal/application/repository -run 'TestAgentRunAdmission|TestCraftRunAdmission|TestGenericRunAdmission|TestAdmissionUsesActiveMembership' -count=1` and `go test ./internal/modules/agentruntime/agent/runtime -count=1`; both passed. The new repository cases exercise cross-home Craft and generic admission plus suspended, removed, absent membership and inactive/deleted users. Existing replay and T01 admission tests remain in the focused selection.
- PostgreSQL repository subtests were unavailable without `TRPC_TEST_POSTGRES_DSN`; this review claims SQLite repository behavior and SQL predicate inspection, not PostgreSQL execution. Task 2 authenticated actor sourcing/TaskWrite and Task 3 worker capability authority remain separate acceptance gates.
