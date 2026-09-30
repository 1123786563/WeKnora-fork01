# Task Brief — FR1 repair round 2: reject malformed loads and narrow otool headers

## Authority and findings

- Parent plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, FR1 repair round 2.
- Independent review: `.superpowers/sdd/plan-t31-ios27/fr1-review.md`, findings FR1-1 and FR1-2 (both Medium).
- Current implementation checkpoint: `97cf8dc86cd307ef3d23f8701086f13038e7c729` in the FR1 worktree. The new brief/ledger commit must be cherry-picked into that worktree before repair starts.
- FR1-1: `@rpath/Missing.framework` has no `/binary`; current fallback looks only for `.framework/`, so the load is skipped and checker returns closure success.
- FR1-2: `inspect()` discards every output line whose path starts with the checked binary path. A real absolute dependency beginning with the same prefix as the executable is discarded.

## Worktree, ownership, interfaces

- Continue in `/Users/wuyongjun/.paseo/worktrees/144ixsa6/t31-fr1-framework-closure`, branch `codex/t31-fr1-framework-closure`.
- Role: frontend_implementer. No subagents.
- Own only `apps/mobile/scripts/verify-ios-framework-closure.py`, `apps/mobile/src/ios-framework-closure.test.ts`, and `.superpowers/sdd/plan-t31-ios27/fr1-report.md`.
- Preserve CLI and mode properties contract. Do not change the Release script, generated iOS files, acceptance docs, plan/ledger, or evidence.

## Acceptance

1. Add a negative production-checker fixture for exact load `@rpath/Missing.framework`; checker must exit nonzero with a clear malformed/unsupported framework diagnostic. Also cover `@rpath/Missing.framework/` if parsed as a distinct load token.
2. Add an `otool -L` fixture with exact universal headers `<binary> (architecture arm64):` and `<binary> (architecture x86_64):`, plus a missing non-system dependency whose absolute path starts with the inspected executable path as a string prefix. The dependency row must not be treated as a header and must fail closure.
3. Skip only exact `otool` image-header lines. Match `<binary>:` or `<binary> (architecture <arch>):` with exact full-path identity. Never filter arbitrary dependency lines by broad string prefix.
4. Preserve all round-1 supported `@rpath`, `@loader_path`, and `@executable_path` cases; positive paths, unknown token and unsupported absolute path, system-framework ignore, bundle/symlink containment, versioned path, mode checks, and `otool` failure tests.
5. Keep dependency rows from every architecture; unresolved loads report owner and dependency and fail closed.
6. Run RED before edits, then focused from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts`.
7. Run `pnpm --filter @weknora/mobile test`, `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check`; report totals/skips/exit codes.
8. Run the production checker on retained Release app/properties and confirm unchanged executable SHA-256 `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`. No native build unless the retained app fails; never overwrite T39 artifacts.
9. Update FR1 report and commit only owned files locally. Provide exact base/HEAD, commit SHAs, changed files, tests, and checker output. Stop for independent review.

## Failure handling

- Skip only fully shaped headers reported by actual `otool`; add any newly observed exact valid header shape explicitly, never by prefix heuristic.
- Any framework name without an executable component is malformed and fails closed. Preserve documented system-framework ignores.
- If valid supported fixtures fail, correct resolution without weakening negative cases.
