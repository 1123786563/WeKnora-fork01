# Task 11 Report: Local Runtime Authorization Failure Recovery

- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-fix-t11-unauth-recovery/WeKnora-fork01`
- Branch: `codex/issue-140-fix-t11-unauth-recovery`
- Base: `5b790d741d4bd546235b22544f96002b665b7812`
- Scope: Task 11 only; Career write classification and three OCR2-037 tests.

## Changes

- Classified the exact `RUNTIME_UNAUTHORIZED` message as a definite local failure. Existing wrapper behavior remains: throw `SCOPE_CHANGED` with the original runtime error as `cause`; do not persist a recovery intent.
- Updated material publish, search, and whole-space deletion tests to log out, assert no transport call occurred, assert the original unauthorized cause, and assert the relevant pending intent is empty.
- Ambiguous network recovery logic was not changed.

## Verification

- RED: `~/.nvm/versions/node/v22.22.3/bin/node --experimental-transform-types --test --test-name-pattern='OCR2-037' tests/application-material.test.mjs tests/career-discovery.test.mjs tests/export-deletion.test.mjs` — 3 failed as expected before classification, each showing an `outcome_unknown` wrapper instead of `SCOPE_CHANGED`.
- GREEN: same focused command after implementation — 3 passed, 0 failed.
- Related intent suites: `~/.nvm/versions/node/v22.22.3/bin/node --experimental-transform-types --test tests/application-material.test.mjs tests/career-discovery.test.mjs tests/export-deletion.test.mjs` — exited 0; all three files passed (test runner output was truncated by the command wrapper).
- `git diff --check` — passed.
- An initial attempt using default `node` failed because it was Node v26.7.0 and rejected `--experimental-transform-types`; reran successfully with installed Node v22.22.3.

## Risks / Limits

- The existing API presents definite local failures to callers as `SCOPE_CHANGED`; original authentication failure remains available through `cause` as requested. This preserves the established Career service contract.
- No browser/device run was applicable to this service-level regression.

## Follow-up Review Fix

- Finding: substring matching could classify a server/network message containing `RUNTIME_UNAUTHORIZED` as a definite local failure, losing recovery for a possibly completed write.
- Fix: the runtime error is now definite only when it is an `Error` with the exact message `RUNTIME_UNAUTHORIZED` and has neither a code nor HTTP status. Existing `SCOPE_CHANGED` and `AUTH_REQUIRED` handling is unchanged.
- Added a material publish regression test using a 503 `upstream_error` whose message contains the token; it must yield `outcome_unknown` and retain the intent.
- Focused Node 22 command `~/.nvm/versions/node/v22.22.3/bin/node --experimental-transform-types --test --test-name-pattern='RUNTIME_UNAUTHORIZED|mentions RUNTIME_UNAUTHORIZED' tests/application-material.test.mjs tests/career-discovery.test.mjs tests/export-deletion.test.mjs` — 4 passed, 0 failed.
- Related full files: `~/.nvm/versions/node/v22.22.3/bin/node --experimental-transform-types --test tests/application-material.test.mjs tests/career-discovery.test.mjs tests/export-deletion.test.mjs` — exited 0.
- `git diff --check` — passed.
