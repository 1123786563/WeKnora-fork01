# F17/F20 Fix1 Independent Validation

**Status:** DONE_WITH_CONCERNS. All four requested focused selectors and `git diff --check` passed on the assigned checkpoint. PostgreSQL runtime behavior remains unverified because `TRPC_TEST_POSTGRES_DSN` is unset.

## Revision and frozen input

- Validation worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-stop-fence/WeKnora-fork01`
- Git HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`
- Assigned Fix1 source report and validation brief: `docs/plans/2026-09-29-craft-107-f17-f20-fix1-report.md` and `docs/plans/2026-09-29-craft-107-f17-f20-fix1-validation.md`
- Frozen patch SHA-256: `204c26d2cd64f2ac4117f7663ebae5700715d3d7d2e8be35561317207fbc090f` — matched.
- Frozen source SHA-256 values, rechecked before and after test execution:
  - `internal/application/service/craft_control.go`: `f5a8526c1aaa23c3ee70f91f33ba1dbaa4220ffb6cc55b59d42b18bd520a72ef` — matched.
  - `internal/application/service/craft_control_f17_f20_test.go`: `de17056ac8cf60403b57bf437b4308f972150601bf96ec4f11fda85faca2ee3c` — matched.
  - `internal/handler/session/craft_interaction_test.go`: `f385f40f3948030bc0549f36b3733d115490082bdf683efc6df78683767fb5c5` — matched.
- Package `manifest.sha256` entries matched the patch and frozen source hashes above.
- Test runs were serial. SQLite test databases use in-memory databases or per-test `t.TempDir()` paths. A separate long-running `go run ./cmd/server` process was present; no concurrent `go test` process or shared database conflict was observed.

## Independently rerun commands

All commands below ran from the validation worktree above, at the stated HEAD and with the frozen hashes matching.

- `go test ./internal/application/service -run 'TestCraftControlRequiresTaskAccessChecker|TestCraftControlRejectsDelegationFromAnotherRun|TestCraftControlCollaboratorMayStopOnlyTheirOwnRun|TestCraftControlTaskRolesGateStopAndStatus' -count=1` — PASS, `ok`, 1.703s.
- `go test ./internal/handler/session -run TestStopCraftRunCollaboratorUsesPersistedOwnerStorageScope -count=1` — PASS, `ok`, 1.966s.
- `go test ./internal/application/repository -run 'TestCraftPrepareTaskStopIntent|TestCraftPrepareTaskSameSessionDifferentRunAfterPriorStop|TestCraftStopIntentAndPrepareTaskSerializeOnRun|TestCraftStopIntentStorePersistsAcrossReconstruction|TestCraftStopIntentStoreConcurrentConfirmNeverDowngrades|TestCraftStopIntentStoreRequiresExactRunIdentityBeforeAcceptingIntent' -count=1` — PASS, `ok`, 5.635s.
- `go test ./internal/container -run 'TestNewCraftInteractionAssemblyInjectsCurrentTaskAccess|TestWireCraftInteractionRegistrarRegistersPendingInteractions' -count=1` — PASS, `ok`, 7.844s; linker emitted the non-fatal warning `ignoring duplicate libraries: '-lc++'`.
- `git diff --check` — PASS, exit 0.
- `printf 'TRPC_TEST_POSTGRES_DSN=%s\\n' "${TRPC_TEST_POSTGRES_DSN:+set}"` — empty value; PostgreSQL runtime tests were not run.

## Acceptance evidence and gaps

The service selector covers missing-checker fail-closed behavior, cross-Run delegation rejection, collaborator self-stop restrictions, and Task-role gates. The session selector covers the collaborator HTTP stop journey and persisted owner scope. Repository selectors cover stop intent persistence, exact Run identity, same-session Run separation, concurrency/serialization and monotonic confirmation. Container selectors cover TaskAccessChecker injection and pending interaction registration. No migration files changed in the Fix1 review patch, so this checkpoint has no new migration behavior to validate.

No focused selector failed. This evidence does not establish PostgreSQL transaction/locking behavior, broader Spec #107 acceptance, or provider-side cancellation after delegation preparation; the source report explicitly limits its guarantee to preventing a fresh `PrepareTask` after a committed Stop intent. These remain acceptance limits rather than newly observed failures.

Validation modified no source or test files. This report was written to the assigned integration worktree; its existing unrelated worktree changes were left untouched.
