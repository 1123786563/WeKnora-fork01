# S2 PostgreSQL Replay Fix — Task Report

**Task:** Recursive semantic comparison for same-key Craft admission replay.  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Base HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34`  
**Owned changes:** `internal/application/repository/agent_run.go`, `internal/application/repository/agent_run_craft_seed_test.go`.

## Change

Replay comparison now parses JSON recursively with `UseNumber`, rejects duplicate keys at every object depth and trailing JSON, normalizes JSON numbers without converting through float64, removes only the top-level server-owned `craft_workspace_seed`, and marshals the full remaining value for semantic comparison. Nested object member order is ignored, while array order and all other values remain significant. `persistUsageBinding` uses the same exact, duplicate-rejecting decoder so a large integer cannot silently round during snapshot persistence.

The D1→D2 replay integration assertion uses JSON semantic equality for the returned snapshot because PostgreSQL JSONB normalizes whitespace and key order. It still asserts that replay retained the original typed D1 Workspace seed.

## TDD and verification evidence

**RED:** Added focused cases before production edits. The nested object reorder case incorrectly compared unequal; duplicate-key objects compared equal; and the `9007199254740993` value rounded to `9007199254740992` in the previous float64 path. The real PostgreSQL D1→D2 replay also returned `agent runtime conflict` before the fix. PostgreSQL 17.9 additionally showed JSONB renders equivalent numeric spellings such as `1e3` and `1000` identically, so canonical numeric normalization is covered.

**GREEN:** The following passed after the fix:

- `go test ./internal/application/repository -run 'SameCraftAdmissionIntentCanonicalizesNestedJSON|PersistUsageBindingPreservesExactIntegersAndRejectsDuplicateKeys' -count=1`
- `TRPC_TEST_POSTGRES_DSN='postgres://craft_validator:craft_validator_pw@localhost:55442/craft_validator?sslmode=disable' go test ./internal/application/repository -run '^TestAgentRunCraftSeedAdmissionFreezesAndReplaysOriginalHead/postgres$' -count=1 -v`
- `TRPC_TEST_POSTGRES_DSN='postgres://craft_validator:craft_validator_pw@localhost:55442/craft_validator?sslmode=disable' go test ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` — PASS (13.029s)
- `go test ./internal/application/service -run '^TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed$|^TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery$|Craft.*StartRun' -count=1` — PASS (3.803s)
- `TRPC_TEST_POSTGRES_DSN='postgres://craft_validator:craft_validator_pw@localhost:55442/craft_validator?sslmode=disable' go test -race ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` — PASS (12.914s)
- `go test ./internal/application/repository ./internal/application/service -run '^$' -count=1` — PASS (compile-only)
- `gofmt -d internal/application/repository/agent_run.go internal/application/repository/agent_run_craft_seed_test.go` — no output
- `git diff --check` — PASS

The disposable database used image `paradedb/paradedb:v0.22.2-pg17`; server reported PostgreSQL 17.9. Container `craft107-s2-pg17-replayfix` was removed after verification with `docker rm -f craft107-s2-pg17-replayfix` (exit 0).

## Checkpoint

Task-local preimage copies and hashes are recorded in `2026-09-24-craft-107-workspace-draft-seed-s2-pg-replay-fix-pre.sha256`. The owned-file patch, postimage archive and SHA-256 values are recorded in `2026-09-24-craft-107-workspace-draft-seed-s2-pg-replay-fix-checkpoint.json`; patch: `2026-09-24-craft-107-workspace-draft-seed-s2-pg-replay-fix.patch`; postimage: `2026-09-24-craft-107-workspace-draft-seed-s2-pg-replay-fix-postimage.tar.gz`.

## Limits

Behavior was exercised against SQLite and disposable PostgreSQL 17.9. Other PostgreSQL major versions were not separately run. Full repository/service package suites were not run; the focused acceptance, race, service and compile checks above passed.
