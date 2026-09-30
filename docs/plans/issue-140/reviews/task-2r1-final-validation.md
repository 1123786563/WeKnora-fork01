# Task 2R1 final independent backend validation

## Result

**DONE_WITH_CONCERNS** — the assigned final corrections pass at source revision `b5c68450dfa32324f19dd4775fdac98e0d899f63` (current evidence HEAD `3fc8f6bbfbaa17a807b54c03310ce14d96501fe9`, whose direct parent is that source). Table-specific PostgreSQL fallback assertions, nanosecond version-grant expiry, and the unchanged legacy seconds contract are verified. Live PostgreSQL execution remains unverified.

## Scope and revision

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-task2-module/WeKnora-fork01`
- Requested source: `b5c68450dfa32324f19dd4775fdac98e0d899f63`
- Current HEAD: `3fc8f6bbfbaa17a807b54c03310ce14d96501fe9`
- `HEAD` is a direct child of the requested source; its only delta from source is the implementation evidence document `docs/plans/issue-140/reviews/task-2r1-caller-scope-gorm.md`.
- Assigned brief: `task-2r1-brief.md` in this directory.
- Validation was read-only against source and tests. This report is the only file created by this validator.

## Checks

All commands ran from the worktree above.

| Exact command | Result |
| --- | --- |
| `git rev-parse HEAD && git show -s --format='%H%n%P%n%s' HEAD` | HEAD `3fc8f6bbfbaa17a807b54c03310ce14d96501fe9`; direct parent `b5c68450dfa32324f19dd4775fdac98e0d899f63`. |
| `git diff --stat b5c68450dfa32324f19dd4775fdac98e0d899f63..HEAD` | One documentation file, 14 added lines; no source/test delta after requested source. |
| `go test ./internal/database -run 'Test(CareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape|CareerMigrationPairsMatchAcrossTracks|MigrationVersionsUniquePerTrack|SQLiteMigrationsCreateVersionedSchema)$' -count=1` | PASS (`internal/database`, 30.573s). |
| `go test ./internal/modules/workbench -run 'Test(NewVersionArtifactGrant|VersionArtifactGrant|ArtifactGrant)' -count=1` | PASS (`internal/modules/workbench`, 0.569s). |
| `go test ./internal/modules/career/... ./internal/modules/workbench ./internal/database` | PASS. Career repository, workbench, and database passed; career root, handler, and service have no test files. |
| `git diff --check b5c68450dfa32324f19dd4775fdac98e0d899f63..HEAD` | PASS (exit 0, no output). |
| `git status --short && (command -v docker || true) && (command -v psql || true) && (command -v pg_isready || true)` | No preexisting changes listed; Docker CLI exists at `/usr/local/bin/docker`; host `psql` and `pg_isready` are absent. |
| `docker ps --format '{{.Names}} {{.Image}}'` | Did not return within the 10-second tool wait and was interrupted (exit 130); no live PostgreSQL query or migration was run by this validator. The existing same-checkpoint final-review evidence records the container query failing because PostgreSQL was in recovery mode. |

## Acceptance evidence

- **PostgreSQL fallback — PASS:** `TestCareerPostgresMigrationDeclaresIsolationAndAppendOnlyShape` extracts each individual `CREATE TABLE` body and binds the required `response_json JSONB` plus composite tenant/owner FK to `career_idempotency_receipts`; `payload JSONB` plus the same FK to each of `career_profile_facts` and `career_evidence`. It checks the evidence update/delete trigger against `career_evidence` and checks trigger/function drops in the down migration. This closes the prior global-count assertion gap. These are structural assertions; they do not substitute for executing PostgreSQL migrations or PL/pgSQL.
- **Version artifact expiry precision — PASS:** `VersionArtifactGrant.ExpiresAt` is documented as Unix nanoseconds; creation adds the accepted duration then records `UnixNano`, verification rejects at `now.UnixNano()`. Tests use a non-millisecond-aligned timestamp (`987654321ns`), assert exact one-second and max-TTL lifetime, and accept one nanosecond before expiry while rejecting at exact expiry.
- **TTL bounds — PASS:** focused tests reject nonpositive and subsecond durations, accept exactly one second, cap an over-maximum duration, and assert exact maximum lifetime.
- **Legacy `ArtifactGrant` — PASS:** its separate wire contract still documents Unix seconds, canonicalizes the `ExpiresAt` integer as provided, verifies against `now.Unix()`, and existing signing/expiry tests pass. No change to its serialization or unit appears in this correction.
- **Repository consistency/migration scope — PASS for available evidence:** the full assigned Career/workbench/database package suite passes, including migrated SQLite persistence/isolation and migration coverage. No route/API wiring was part of Task 2R1's owned scope; no new HTTP cancellation or status mapping is claimed.

## Gaps and risks

- PostgreSQL migration execution, live FK enforcement, trigger execution, and GORM JSONB runtime roundtrip remain unverified. Static assertions and SQLite tests do not establish runtime PostgreSQL behavior. Earlier same-checkpoint review evidence reports the available PostgreSQL container was in recovery mode; this validation's Docker listing itself stalled.
- No acceptance gap was found in the requested correction set. Broader Task 2 and downstream route/lifecycle acceptance remain outside this validation.
