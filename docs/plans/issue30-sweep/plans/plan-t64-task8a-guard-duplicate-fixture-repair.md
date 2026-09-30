# T64 Task 8A Guard Duplicate Fixture Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the 8A duplicate-mapping fail-closed test meaningful after 8B installs a unique index that prevents normal fixtures from inserting duplicate mappings.

**Architecture:** Change only the SQLite guard test fixture. In its duplicate-mapping case, remove the 8B partial unique index from that test's isolated database after seeding the first mapping, then seed the second mapping and assert the guard still returns `ErrAgentSecurityReleaseUnresolvable`. Production guard code and both migration tracks remain unchanged.

**Tech Stack:** Go, GORM, SQLite migrations, testify.

**Spec:** `docs/specs/2026-09-20-agent-marketplace-domain-model.md` §§10–11; `docs/adr/0015-agent-security-transactional-admission.md`; `docs/plans/issue30-sweep/issues/issue-64.md`; Task 8A in `docs/plans/issue30-sweep/plans/plan-t64-task8-atomic-admission.md`.

## Global Constraints

- Preserve the 8B database invariant and the index named `uq_agent_adoption_variant_local_agent`; do not change production migrations or repository code.
- Preserve the guard's fail-closed result `ErrAgentSecurityReleaseUnresolvable` when duplicate eligible mappings are present.
- Confine the schema exception to the duplicate subtest's `openRunTestDB(t)` database, which is removed with that test's temporary directory.
- Keep the 8A and 8C security admission contracts from changing.

## Review Focus

- The index is removed only in `duplicate_eligible_mappings_fail_closed`; all other table-driven guard cases retain the migrated schema.
- The duplicate row has the same tenant and local Agent as the first row, so the guard observes two eligible mappings.
- The test still asserts `ErrAgentSecurityReleaseUnresolvable`, not a database insertion error.
- The migration test still proves the normal migrated schema creates `uq_agent_adoption_variant_local_agent`.
- No production file or migration is changed to make the fixture pass.

---

### Task 1: Simulate duplicate legacy mappings only in the guard test

**Files:**
- Modify: `internal/application/repository/agent_security_guard_test.go`

**Interfaces:**
- Consumes: `checkLocalAgentReleaseAdmissionTx(tx *gorm.DB, sourceTenantID uint64, localAgentID, localAgentVersionID string) (releaseID string, adopted bool, err error)`.
- Produces: `TestCheckLocalAgentReleaseAdmissionTx/duplicate_eligible_mappings_fail_closed` constructs two otherwise eligible mapping rows in its isolated SQLite DB and continues to require `ErrAgentSecurityReleaseUnresolvable`.

**Baseline evidence:** On `5839d6084900a052c05b46549ba58c051dcc5770`, the full repository package run fails at the second fixture insert with `UNIQUE constraint failed: agent_adoption_variants.tenant_id, agent_adoption_variants.local_agent_id`. Task 8B's SQLite and PostgreSQL migrations both create the partial unique index `uq_agent_adoption_variant_local_agent` for nonempty local Agent IDs. The first integration commit therefore cannot satisfy the duplicate fixture without explicitly modeling a legacy/corrupt database state.

- [ ] **Step 1: Remove the unique index only in the duplicate case.** After `seedAdmissionVariant` creates `variant-1` and immediately before creating `variant-2`, add a conditional `db.Exec("DROP INDEX uq_agent_adoption_variant_local_agent")` guarded by `tt.duplicate`; require the DDL call to succeed. Leave every other table-driven case on the migrated schema.
- [ ] **Step 2: Run the focused duplicate case.** Run `go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$/^duplicate_eligible_mappings_fail_closed$' -count=1`. Expected: PASS; fixture creates both rows and the guard returns the sentinel error.
- [ ] **Step 3: Run the complete guard test.** Run `go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1`. Expected: PASS for all table-driven guard cases.
- [ ] **Step 4: Confirm the normal migration still installs the invariant.** Run `go test ./internal/database -run '^TestSQLiteMigrationsCreateVersionedSchema$' -count=1`. Expected: PASS with `uq_agent_adoption_variant_local_agent` present in the migrated schema.
- [ ] **Step 5: Check the scoped diff and commit.** Run `git diff --check`; stage only `internal/application/repository/agent_security_guard_test.go` and commit as `test(repository): preserve duplicate mapping guard coverage`.

**Acceptance:** The targeted duplicate case and complete guard test pass; the migration schema test still sees the unique index; the commit changes only the test file; production code and migrations are unchanged. After integrating this repair, rerun `go test ./internal/application/repository` on the integrated 8C HEAD with a test timeout long enough for the complete serial package suite.
