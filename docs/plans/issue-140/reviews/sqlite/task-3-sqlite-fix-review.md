# T03 SQLite startup fix: independent Spec and quality review

**Reviewed:** `a4bb32ed3..21d45eb799f1d66e73504220835289f566aeb7e1` in the T03 fix worktree, 2026-09-24. Read only review of source and evidence; no OCR run. Sources: approved `docs/specs/2026-09-23-weknora-job-search-design.md`, ADR-0015/0017, `CONTEXT.md`, #141 snapshot, SQLite fix plan and implementer report, actual SQLite/PostgreSQL Career migrations.

## Findings

### F1 — High: table presence is mistaken for schema integrity

**Evidence / affected symbol:** `internal/modules/career/office.go:200-214`, `NewOffice`. The SQLite branch skips all schema work whenever `HasTable` returns true for seven table names. It does not verify columns, indexes, unique constraints, or a successful `000112` migration version. `migrations/sqlite/000112_career_profile.up.sql:3,8-9` defines the `(tenant_id,user_id,key)`, `(tenant_id,user_id,revision)`, and `(tenant_id,user_id,request_id)` uniqueness that T03 relies on. A seven-table database whose constraint or a later required column is absent therefore returns a usable `Office` without detecting the defect. The new regression test at `internal/database/career_migration_test.go:35-59` only uses a fresh, complete migration stream.

**Impact:** A partial, manually repaired, or previously AutoMigrated/upgraded SQLite database can admit duplicate confirmed facts, change revisions, or request receipts, breaking revision CAS and idempotency; missing columns fail later on user operations instead of at startup. The skip criterion is weaker than the plan's requirement to preserve upgraded databases and uniqueness. This does not show corruption in a database that has successfully applied the current versioned migrations; it leaves that condition unverified for other existing schemas.

**Smallest defensible correction:** Gate the skip on verified versioned migration state and required Career schema shape, including the three composite uniqueness constraints; fail initialization with an actionable schema error if existing tables are incomplete. Keep the no-table compatibility path only where intended. Add focused tests for a seven-table schema missing one required column or uniqueness constraint, and for an already applied `000112` database upgraded through `000113` before `NewOffice`.

### F2 — Medium: incomplete seven-table sets still take the known failing rebuild path

**Evidence / affected symbol:** `internal/modules/career/office.go:201-211`. If any one Career table is absent, the code calls `AutoMigrate` on *all* models. When `career_facts` already comes from `000112` with a table-level `UNIQUE` clause, the same GORM SQLite rebuild/parser failure cited in the fix report can recur. No test covers this partial state.

**Impact:** Recovery from a partially applied or repaired schema can fail again with `career_facts__temp has no column named UNIQUE`, obscuring which table is missing. The normal migration runner rejects dirty history before module startup, so this is principally a recovery/compatibility gap rather than a proven failure of the clean migration path.

**Smallest defensible correction:** Distinguish truly absent Career schema from mixed presence. For mixed presence, report an explicit incomplete-schema error or apply a versioned repair migration; do not run whole-model AutoMigrate against already versioned SQLite tables. Cover that branch with one test.

## Evidence and coverage

- The fix directly addresses the reproduced clean-path failure: versioned SQLite SQL creates all seven tables; `NewOffice` now leaves those tables intact. The new test applies the real repository migration stream, calls `NewOffice` twice, claims a space, and checks that duplicate fact, change, and receipt inserts fail. The RED and GREEN command results and package suite are reported by the implementer; this review did not repeat those commands.
- The duplicate-insert assertions exercise the three required uniqueness constraints in the current SQLite migration. The test's second initialization reuses the same GORM handle and does not rerun migrations or reopen the database, so it is weaker evidence for a process restart. It also does not prove a `000112`-to-`000113` upgrade path or any malformed-schema behavior.
- PostgreSQL retains the previous `AutoMigrate` branch because only the SQLite dialect may skip it. No PostgreSQL runtime or migration test was supplied for this fix; no PostgreSQL regression is visible in this two-file diff.
- The implementer's disposable server command used `APP_PORT`, whereas the server config expects `SERVER_PORT`; its exit before HTTP serving is not evidence that the Career SQLite change failed. The actual server start and authenticated `/api/v1/career/open` probe remain parent integration checks.

## Verdict

**Spec compliance: conditional / not yet approved.** The clean, current SQLite migration path appears repaired and preserves its database constraints, but the seven-table skip criterion cannot establish the schema and uniqueness invariants required for existing/upgraded databases. F1 must be resolved or bounded by demonstrated migration-history guarantees. **Code quality: changes requested.** F1 is a data-integrity risk; F2 leaves a known failing recovery path. PostgreSQL behavior is unchanged by inspection, with no new execution evidence. This verdict is limited to commit `21d45eb` and the cited report; it does not replace integration HTTP validation or OCR.
