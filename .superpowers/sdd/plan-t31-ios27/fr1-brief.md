# Task Brief — FR1: Resolve loader-relative framework dependencies

## Authority and finding

- Parent plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, task FR1.
- Independent finding: `.superpowers/sdd/plan-t31-ios27/final-review.md`, T31-F1 Medium.
- Exact initial HEAD: `a2c695f18` (after the review-finding repair plan/ledger checkpoint).
- Reviewer reproduced a false pass by invoking the production checker on a disposable app whose `otool -L` output contained unresolved `@loader_path/Frameworks/Missing.framework/Missing`; checker exited 0 with `FRAMEWORK_CLOSURE_OK`. The retained production app itself currently passes.

## Worktree, ownership, and interfaces

- Execute in `/Users/wuyongjun/.paseo/worktrees/144ixsa6/t31-fr1-framework-closure`, branch `codex/t31-fr1-framework-closure`; base will be fast-forwarded to include this brief commit before implementation starts.
- Role: frontend_implementer. No subagents.
- Own only `apps/mobile/scripts/verify-ios-framework-closure.py`, `apps/mobile/src/ios-framework-closure.test.ts`, and `.superpowers/sdd/plan-t31-ios27/fr1-report.md`.
- Preserve CLI: `verify-ios-framework-closure.py <app.bundle> <generated-Podfile.properties.json>`. Keep source/precompiled mode validation and versioned framework behavior.
- The test harness runs this checked-in Python checker with fake `otool`; production Release script continues invoking the same checker.
- No changes to generated `apps/mobile/ios/`, release script, evidence, user data, or other plan/ledger files. Do not run native build unless the retained artifact fails after the checker repair.

## Acceptance and exact cases

1. Keep current `@rpath` behavior, system-library ignoring, exact declared-executable identity, own-bundle containment, and source-mode Worklets rejection.
2. Add failing production-checker integration cases for unresolved `@loader_path/Frameworks/Missing.framework/Missing` and `@executable_path/Frameworks/Missing.framework/Missing`. Each must exit nonzero and name both owning image and dependency.
3. Add passing `@loader_path` and `@executable_path` cases resolving to real embedded framework executables; add a framework-owner `@loader_path` case where the framework references a sibling embedded framework if supported by the generated install name.
4. Add a non-system unsupported token form such as `@unknown_path/Foo.framework/Foo`; it must fail closed with a clear classification diagnostic. Keep system paths such as `/System/Library/Frameworks/...` out of the non-system closure failure path.
5. All accepted path forms must resolve the canonical requested file to the declared executable of the named embedded framework, and all canonical targets must remain inside the app bundle. Missing paths, directories, symlinks outside the app, wrong framework executable, malformed install name, and unsupported non-system forms cannot produce closure success.
6. Run RED before implementation; focused command from `apps/mobile`: `node --import tsx --test src/ios-framework-closure.test.ts src/scripts/ios-acceptance-scripts.test.ts`.
7. Run `pnpm --filter @weknora/mobile test`, `pnpm --filter @weknora/mobile typecheck`, `pnpm --filter @weknora/mobile exec expo install --check`, and `git diff --check`; report exact totals, skips, exit codes.
8. Run the production checker against the retained Release app and generated properties. Expect `FRAMEWORK_MODE=source-expo-modules` and `FRAMEWORK_CLOSURE_OK`; verify app executable SHA-256 remains `8d2141d06f3b6f0d185be79dff122e7fd0d102e901d72862bed7dec200e073e4`.
9. Commit only owned files locally. Send commit SHA, changed-file list, exact base/HEAD and validation evidence. Stop for independent reviewer; do not self-approve.

## Failure handling

- If any new positive fixture fails, correct classification/resolution while preserving exact file identity; do not weaken negatives to pass.
- If the retained app fails with an actual unresolved supported path, record it and do not claim local runtime acceptance; determine whether a clean Release rebuild is required and preserve all evidence without overwriting T39 logs.
- If a non-system path form cannot be safely resolved, fail closed and describe the unsupported form rather than silently skipping it.
