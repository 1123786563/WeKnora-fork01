# Task Brief — FR1 repair round 3: match complete framework path components

## Authority and finding

- Parent plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, FR1 repair round 3.
- Independent review: `.superpowers/sdd/plan-t31-ios27/fr1-r2-review.md`, finding FR1-R2-1 Low.
- Implementation checkpoint: `4dfe529c78850a662281eb4e30b23711dd1ac103`; continue in FR1 worktree after incorporating the current parent plan/brief commit.
- The fallback `if ".framework" in load` incorrectly rejects non-framework dylib install name `@rpath/Foo.framework.dylib`. Reviewer reproduced exit 1, `UNSUPPORTED_FRAMEWORK_LOAD_PATH`.

## Worktree, ownership, interfaces

- Continue in `/Users/wuyongjun/.paseo/worktrees/144ixsa6/t31-fr1-framework-closure`, branch `codex/t31-fr1-framework-closure`.
- Role: frontend_implementer. No subagents.
- Own only `apps/mobile/scripts/verify-ios-framework-closure.py`, `apps/mobile/src/ios-framework-closure.test.ts`, and `.superpowers/sdd/plan-t31-ios27/fr1-report.md`.
- Preserve CLI, generated mode properties, R4 bundle identity, loader-relative support, and exact `otool` header handling. No build/evidence changes.

## Acceptance

1. Replace substring classification with recognition of a complete path component named `<name>.framework` (followed by `/` or end of load token). Do not classify `.framework.dylib` as a bundle.
2. Add a positive production-checker fixture for `@rpath/Foo.framework.dylib` (and a simple non-framework dylib path if practical) proving it is ignored by this framework-only gate and does not produce closure failure.
3. Preserve failing cases for malformed `@rpath/Missing.framework`, trailing-slash empty executable, unknown-token framework path, unsupported absolute framework path, missing supported framework paths, and path-prefix dependency under exact architecture headers.
4. If the prior scoped review's multi-arch test-layout limitation is addressed, emit a distinct dependency row after each exact architecture header and ensure each unresolved framework load is detected; otherwise document why one shared output row is sufficient for this parser. Do not invent architecture-specific state.
5. Run RED before implementation; focused tests from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts`.
6. Run `pnpm --filter @weknora/mobile test`, `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check`; report counts/skips/exit codes.
7. Retained Release checker must pass source mode; executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`. No native build unless retained checker fails.
8. Update FR1 report, commit only owned files locally, provide exact range/evidence, and stop for independent review.
