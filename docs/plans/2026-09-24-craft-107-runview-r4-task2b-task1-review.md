# Craft #107 T01 R4 Task 2b Task 1 — independent review

Review target: the private candidate Task 1 checkpoint `craft-107-r4-task2b-task1-candidate` in the integration worktree. Reviewed the approved web-artifact Spec, `CONTEXT.md`, ADR-0004/0008/0009, Task 2b architecture and plan, report, exact patch, migrations, implementation and focused tests. Read-only review; no OCR run.

## Verdict

- **Spec compliance for Task 1: PASS, with an integration gate.** `CollectCandidate` takes a per-call source, compares its Run and generation with the admitted Task and verified generation before listing/upload, and writes only to `CandidateStore`. The candidate table is separate from published versions. `CollectForKind` still calls `VersionStore.Publish`. The focused tests preserve V1 in `VersionStore.List/Get`, reject A's source for B before upload, leave no candidate on upload/validation failure, and cover identical replay versus changed-content conflict.
- **Code quality: PASS for the exact checkpoint, with the migration and PostgreSQL limits below.** Scope is revalidated against durable Workspace and Run rows on write/read. Header and files insert in one transaction; a conflict adopts the previously sealed object refs only when identity, generation, content, checks and evidence agree. No source or test changes were made by this reviewer.

## Findings and integration gates

1. **Medium — migration delivery is not yet complete.** The four `000122`/`000201` SQL files are ignored by `.gitignore:96` and absent from Git's staged/tracked diff. They are present in the exact task patch and current worktree, but would be lost from a normal commit or branch handoff. SQLite `000121` and PostgreSQL `000200`, reserved as predecessors by the plan, are also absent from this worktree. If `000122`/`000201` is applied first, a later lower-numbered migration will be skipped by the versioned runner (`internal/database/migration.go`, `m.Up()`). **Smallest correction:** integrate and review the reserved predecessors first, then force-track all four candidate SQL files before running the full migration chain or committing. This is an integration gate, not a defect in the isolated candidate SQL.
2. **Medium — PostgreSQL behavior remains unverified.** `TestCraftCandidateMigrationPostgresUpDownUp` skips without `TRPC_TEST_POSTGRES_DSN`; this environment has no such DSN. The repository focused suite passes on SQLite, and the SQL text has the expected composite foreign keys, unique Run key, transaction-compatible tables, and update/delete guards. **Smallest correction:** run the candidate store and up/down/up PostgreSQL tests against a disposable PostgreSQL schema before treating the paired migration gate as complete.

## Evidence

- All ten source files match their checkpoint SHA-256 values; the task-local patch matches SHA-256 `3339c0a3ac1eb13483e232b6fbca421c6986cadaed8af1c15ccf804eae2cc873`. `git apply --reverse --check` accepts that patch against the exact current files, and the modified service's HEAD preimage matches the checkpoint preimage hash.
- SQLite `000122` and PostgreSQL `000201` both create private candidate header/file tables with `(tenant_id, workspace_id, run_id)` uniqueness and Run/Workspace foreign keys. Their down files drop guards, file table, then header; the isolated SQLite up/down/up test passed. The SQL files themselves, including ignored ones, were read in full and hash checked.
- `go test ./internal/modules/craft -run '^TestCandidateValidateBindsRunGenerationAndManifest$' -count=1` — PASS.
- `go test ./internal/application/service -run '^TestCraftArtifact(CollectCandidate|CollectPinsImmutableVersions|CollectIsIdempotentForSameContent|CollectUploadFailureLeavesNoVersion|CollectRejectsHostileOutput|CollectChecksAreHonestFacts|CollectValidatesTask|Publish)' -count=1` — PASS.
- `go test ./internal/application/repository -run '^TestCraft(Candidate|Version)' -count=1` — PASS (SQLite; PostgreSQL cases skipped without DSN).

Task 2 post-terminal draft capture, Task 3 production source wiring, T15 four-check promotion, real Linux source confinement, and integrated A→B/C behavior remain subsequent gates. This verdict covers Task 1 only.
