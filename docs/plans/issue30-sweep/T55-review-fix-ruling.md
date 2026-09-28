# #55 Review Fix Ruling — 2026-09-28

## Accepted findings

1. **Task 2 medium — remote resolver evidence:** accepted. Existing `TestT25UnknownResolvesFromRemoteFactsOverHTTP` checks the resulting state but does not directly prove branch and PR remote reads determined it, nor that resolution emitted no remote write. Add GET/path/query and before/after side-effect evidence.
2. **Task 2 medium — route/auth scope claim:** accepted. `newRecoveryEnv` manually attaches handlers to a fresh Gin engine and supplies tenant/user identity directly to request context. It exercises real HTTP through real handlers and A03 approval, but does not prove production route registration or authentication middleware. The route researcher confirmed direct production setup is impractical in this repository test: router guard setup uses unexported helpers, full `NewRouter` construction requires many unrelated services, and authentication needs broader DB-backed fixtures. Ruling: retain the bounded handler/service HTTP fixture and injected principal context, ensure it mounts the resolve route, narrow comments/report and cite production route declarations. Production route registration/auth remains a stated risk.
3. **Task 2 low — credential scan coverage:** accepted. The test currently captures only dispatch/retry/read/resolve responses and checks delivery columns. It omits baseline/prepare/approve responses and action-row values. Add those response and persisted action checks.
4. **Task 3 finding — ineffective env-key whitelist:** accepted. `withWorkspaceEnvDefaults` mutates the provided map in place, so the current post-call iteration over `input` skips every inserted key. Snapshot configured keys before the call and compare against that snapshot.
5. **Task 7 medium — draft state:** accepted. Assert `Draft == true` on the real PR after initial dispatch and after rejected duplicate dispatch, alongside stable PR identity.
6. **Task 7 medium — rejected duplicate write effects:** accepted. Remote branch SHA stability is not proof that the service made no write request. Count outbound HTTP methods around duplicate dispatch and assert no POST/PATCH/PUT/DELETE.
7. **Task 7 medium — baseline deletion risk:** accepted. `DiffAgainstBaseline` marks omitted baseline files deleted; current real-provider fixture creates only README.md. Fail closed before delivery writes unless configured dedicated repository baseline contains exactly README.md, matching the workspace fixture.

## Execution partition

- Task 1 uses role `backend_implementer`, worktree `codex/issue30-t55`, and owns only `internal/application/repository/delivery_recovery_http_test.go`. Report: `.superpowers/sdd/plan-t55-review-fix/task-1-report.md`.
- Task 2 uses role `mechanical_worker`, a separate worktree `codex/issue30-t55-t3-review-fix` from the Task3 checkpoint, and owns only `internal/modules/execution/sandbox/workspace_env_test.go`. Report: `.superpowers/sdd/plan-t55-review-fix/task-2-report.md`.
- Task 3 uses role `backend_implementer`, the isolated existing worktree `codex/issue30-t55-t7`, and owns only `internal/modules/codedelivery/github_real_test.go`. Report: `.superpowers/sdd/plan-t55-review-fix/task-3-report.md`.
- Each task has a separate brief, worktree, verifier/reviewer checkpoint and local commit. There is no code-file overlap.

## Rationale and risks

The first four findings expose test coverage gaps and do not report a production defect, so the repairs are test-only. Production route/auth coverage cannot be added to Task 1's repository test without broad unrelated router and DB wiring; it must be described accurately as real handler/service HTTP with injected identity. Risk: a production route-registration or authentication-gate regression remains outside this T25 test and must be covered elsewhere. Task 3's additional opt-in baseline precondition may fail early against a non-empty dedicated test repo; this is intentional fail-closed protection against accidentally deleting LICENSE, `.gitignore` or other remote files. Attempting to mirror arbitrary trees instead would introduce path/symlink and tree-size handling into this narrow test. If any rejected dispatch emits a write, leave that evidence failing and open a separate production-fix task.
