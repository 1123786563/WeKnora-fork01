# Craft #107 first-frontier persistence migration Task

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development Task Brief discipline. This is a supporting implementation Task for T01/T05/T08, not a new business Ticket.

**Goal:** Make T01 input-recognition, T05 actual-source and T08 task-grant facts durable in both PostgreSQL and SQLite before those Tickets are verified.

**Authority:** Approved Spec `docs/specs/2026-09-23-craft-web-artifact-spec.md`, DAG/plan, T00 reviewed checkpoint. T01/T05/T08 implementers supplied the exact schema proposals in the active task. The controller allocates migration numbers because T20 depends on verified earlier Tickets; leaving all schema work to T20 would create a verification cycle.

**Ownership:** Only create `migrations/versioned/000189_craft_web_artifact_frontier.up.sql`, `.down.sql`, `migrations/sqlite/000110_craft_web_artifact_frontier.up.sql`, `.down.sql`, and focused migration verification source if the existing migration harness has a suitable test seam. Do not edit application code, models, T00 contracts, other migrations, or shared migration registry. No commits, push, merge, deploy, Issue closure or subagents. Other Agents are changing separate lane files; do not revert their edits.

## Required DDL

1. Add nullable `recognition_accepted BOOLEAN`, `recognition_understood BOOLEAN`, `recognition_reason TEXT` to `craft_workspace_inputs`. Legacy rows must remain all NULL (unknown), matching T00's absent recognition → `null` parser default. New T01 writes explicit facts. Do not use `DEFAULT TRUE`.
2. Create `craft_knowledge_records` with `tenant_id BIGINT NOT NULL`, `session_id VARCHAR(128) NOT NULL`, `run_id VARCHAR(128) NOT NULL`, `record_json TEXT NOT NULL`, `digest CHAR(64) NOT NULL`, `acquired_at TIMESTAMP NOT NULL`, and primary key `(tenant_id,session_id,run_id)`. No source FK or extra index. T05 stores deterministic JSON and lowercase SHA-256; same-key conflicting content is handled in service/repository code.
3. Create `craft_task_grants` with `tenant_id BIGINT NOT NULL`, `session_id VARCHAR(36) NOT NULL`, `user_id VARCHAR(512) NOT NULL`, `role VARCHAR(16) NOT NULL`, `granted_by VARCHAR(512) NOT NULL`, `created_at TIMESTAMP NOT NULL`, `updated_at TIMESTAMP NOT NULL`, primary key `(tenant_id,session_id,user_id)`, and secondary index `(tenant_id,user_id)`. The T08 service validates role and tenant; no automatic grant to tenant peers or admins.
4. Down migration reverses the two tables/index and the three columns. Follow existing project ordering and SQLite `DROP COLUMN` patterns.

## RED → GREEN and verification

- Inspect existing `migrations/versioned/000129_craft_sessions.up.sql`, `migrations/sqlite/000049_craft_sessions.up.sql` and the latest numbers (PG 000188, SQLite 000109) before writing.
- Add a focused migration test if one already exists, or run the SQL against a disposable SQLite database that first applies the Craft base schema. RED must demonstrate the missing table/columns before the new migration. GREEN must demonstrate all columns/tables/indexes, a legacy input row with NULL recognition, and rollback removing them. Verify PostgreSQL DDL syntax via the project's migration test path if available; do not claim a live PG run without one.
- Run `git diff --check` and record exact commands/exits. Full Go/TS Ticket tests belong to their lane implementers.

**Report:** Write `docs/plans/2026-09-23-craft-107-frontier-migration-report.md` with changed files, RED/GREEN evidence, schema objects, verification output, limitations, and exact migration file hashes. The controller will snapshot and review this uncommitted change, then copy the verified migration files to T01/T05/T08 worktrees.
