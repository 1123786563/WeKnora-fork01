# Task 11 Final Integrated Validation

- Revision: `daddfcd8579e4235c46aa4593ed26f866146a9db`
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-sweep-integration/WeKnora-fork01`
- Brief/evidence read: `docs/superpowers/plans/2026-09-29-issue140-miniprogram-residuals.md` (Task 11, acceptance at lines 87–97) and `docs/superpowers/reports/2026-09-29-issue140-task11-report.md`.
- Scope: integrated Task 11 authentication failure recovery behavior, the three OCR2-037 cases, and the 503 server-error message ambiguity regression.

## Validation

Command (run from `apps/miniprogram`):

```sh
/Users/wuyongjun/.nvm/versions/node/v22.22.3/bin/node --experimental-transform-types --test --test-name-pattern='OCR2-037|5xx response mentioning RUNTIME_UNAUTHORIZED' tests/application-material.test.mjs tests/career-discovery.test.mjs tests/export-deletion.test.mjs
```

Result: exit code 0; 4 tests passed, 0 failed, 0 skipped. The passing cases were:

- `OCR2-037 E2: RUNTIME_UNAUTHORIZED on the first write never persists a recovery intent`
- `OCR2-037: RUNTIME_UNAUTHORIZED on the first search never persists a recovery intent`
- `OCR2-037: RUNTIME_UNAUTHORIZED on the first deletion never persists a recovery intent`
- `a 5xx response mentioning RUNTIME_UNAUTHORIZED remains an ambiguous publish outcome`

The three unauthorized cases passed with their test assertions for failure before transport and no persisted recovery intent. The 5xx regression passed with assertions that the 503 remains the underlying status and the potentially completed publish stays recoverable.

## Acceptance and limits

- Covered the three specified local `RUNTIME_UNAUTHORIZED` operations and confirmed their no-intent behavior.
- Covered the specified server-side 503 message containing `RUNTIME_UNAUTHORIZED` and confirmed ambiguous outcome recovery remains available.
- No acceptance gap found in the requested Task 11 cases.
- Browser/device accessibility and responsive behavior are not applicable to these service-level tests; this run does not claim broader Mini Program UI or full-suite coverage.
- The worktree was clean at initial inspection. A concurrent change appeared in `apps/miniprogram/tests/export-deletion.test.mjs` during validation; its diff adds a bounded request wait to two N9 tests, outside this Task 11 validator's scope. I did not edit or revert it. The focused OCR2-037 deletion test passed on the integrated HEAD before this subsequent worktree-only edit appeared.
- No production or test source files were modified by this validator.
