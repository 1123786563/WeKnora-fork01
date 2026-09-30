# Craft #107 T01 R4 Task 2b Task 1 — private candidate report

## Scope and checkpoint

Implemented only Task 1 from `2026-09-24-craft-107-runview-r4-task2b-plan.md`, using the approved artifact Spec and `2026-09-24-craft-107-runview-r4-task2b-architecture.md`. The candidate is a separate private authority; this work does not wire RunView runtime execution, capture drafts, or promote a Version.

- Integration worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Task BASE: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- Checkpoint: `2026-09-24-craft-107-runview-r4-task2b-task1-checkpoint.json`
- Task-local patch: `2026-09-24-craft-107-runview-r4-task2b-task1-task-local.patch`
- Patch SHA-256: `3339c0a3ac1eb13483e232b6fbca421c6986cadaed8af1c15ccf804eae2cc873`
- No commit was created.

The four new SQL migration files are currently ignored by `.gitignore:96` (`migrations/`). They are present in the task-local patch and checkpoint, but were not force-added to the shared index to avoid changing other workers' staging state. The integrator must force-track them.

## Changes

- Added `craft.Candidate`, deterministic `CandidateID`, validation, and a narrow `CandidateStore` interface. Candidate identity binds tenant/owner/session, Workspace, Run, generation, manifest digest, files, checks and evidence.
- Added private candidate collection to `CraftArtifactService`. It requires an explicit per-call source exposing server-constructed Run and generation identity, checks both against the Task and verified generation before listing or uploading, then uses the same bounded list/read/path/manifest/upload helper as legacy `CollectForKind`.
- Preserved the legacy `CollectForKind` Version publication path. Candidate collection writes only through `CandidateStore`; it does not call `VersionStore.Publish`, and candidate rows are absent from VersionStore List/Get.
- Added SQLite `000122` and PostgreSQL `000201` private candidate tables, file rows, scoped unique identity, foreign keys and database update/delete guards. Repository writes the header and file manifest atomically, scopes reads to the workspace owner/session, rejects a different candidate for the same Run, and adopts identical content replay using the originally sealed object refs.
- Added service, module, repository, authorization, immutability, rollback and isolated migration tests.

## TDD and verification evidence

RED was observed before production declarations existed:

- `go test ./internal/modules/craft -run '^TestCandidateValidateBindsRunGenerationAndManifest$' -count=1` — compile failure on undefined `Candidate` / `CandidateID`.
- `go test ./internal/application/service -run '^TestCraftArtifactCollectCandidate' -count=1` — compile failure on missing candidate model and `NewCraftArtifactServiceWithCandidates`.
- `go test ./internal/application/repository -run '^TestCraftCandidate(MigrationSQLiteUpDownUp|StoreIsPrivateIdempotentAndScoped)$' -count=1` — compile failure on missing candidate model and repository constructor.

Final focused verification, after the implementation checkpoint:

- `go test -v ./internal/modules/craft -run '^TestCandidateValidateBindsRunGenerationAndManifest$' -count=1` — PASS.
- `go test -v ./internal/application/service -run '^TestCraftArtifact(CollectCandidate|CollectPinsImmutableVersions|CollectIsIdempotentForSameContent|CollectUploadFailureLeavesNoVersion|CollectRejectsHostileOutput|CollectChecksAreHonestFacts|CollectValidatesTask|Publish)' -count=1` — PASS. This covers per-call B source while service default source is A, wrong Run/generation rejected before upload, candidate upload and validation failure leaving no row, idempotent candidate retry, V1 Version List/Get preservation, and existing legacy collection/publication behavior.
- `go test -v ./internal/application/repository -run '^TestCraft(Candidate|Version)' -count=1` — PASS on SQLite. Candidate cases cover private lookup, owner/session scoping, same-Run changed-byte conflict, re-upload reference replay adopting the sealed row, VersionStore List/Get isolation, DB immutability, header/file transaction rollback, and isolated SQLite migration up/down/up. Existing VersionStore tests also pass.
- The repository candidate store and candidate PostgreSQL migration subtests skipped because `TRPC_TEST_POSTGRES_DSN` is unset. PostgreSQL SQL is therefore **not verified against a live PostgreSQL server**.
- `git diff --check -- internal/application/service/craft_artifacts.go` — PASS; `gofmt -d` on all changed Go source/test files — no output.

A broader package-wide command (`go test ./internal/modules/craft ./internal/application/service ./internal/application/repository -count=1`) was started but stopped after about 195 seconds to free shared test resources for another worker's targeted validation. Its partial output included unrelated failures such as `TestExcerptOfBoundsAtRuneBoundary`; it did not complete and is not counted as a gate. The final focused tests above completed independently.

## Integration constraints and remaining gates

- The RunView source identity interface is now explicit: `CraftArtifactRunID()` and `CraftArtifactGeneration()`. Task 3 must implement these methods on its server-constructed `runBoundCraftArtifactSource` and use the candidate-aware constructor. The service rejects source A paired with Task B / generation B before any source read or upload. No container/runtime file was changed here.
- Migration numbering is reserved at SQLite `000122` / PostgreSQL `000201`. The integration worktree at this checkpoint had not yet received SQLite `000120`/`000121` or PostgreSQL `000199`/`000200`; the owner confirmed those migrations must land and be reviewed first. Do not migrate a shared database to 122/201 until the reserved predecessors are integrated, or later lower-numbered migrations could be skipped by the migration runner. The recorded up/down/up proof runs only the candidate SQL in an isolated temporary database/schema.
- Independent Task 1 review remains pending with the parent. Task 2 draft capture, Task 3 runtime wiring, T15 promotion, full PostgreSQL verification and integrated A→B acceptance remain outside this task.
