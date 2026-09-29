# T31 OCR Round 4 Repair Report

## Scope

Implemented Task 1 from `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r4-repairs.md` on base `fb28119e0e9511b5db412cf5346079b15b0520bd`. Updated the R3 repair record to account for all six comments from the complete R3 OCR run. The background suggestion remains deferred because the app has no shared root theme background token; the accepted provider/view/stack hierarchy is preserved.

## Changes

- `apps/mobile/scripts/verify-ios-framework-closure.py`: removed permissive fallthrough so every unclassified non-system dependency emits `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD`; explicit `/System/Library/` and `/usr/lib/` roots remain allowed.
- `apps/mobile/src/ios-framework-closure.test.ts`: added `@unknown_path/libMissing.dylib` and bare `libMissing.dylib` to fail-closed coverage. Existing fixtures retain known token failures and positive system-root checks.
- `apps/mobile/src/scripts/verify-ios-scene-project.test.ts`: remove only real provider conformance; preserve the provider name in a comment and a string and assert rejection. Positive real-conformance fixture remains.
- `apps/mobile/src/app/_layout.tsx`: pass `initialWindowMetrics` to the root SafeAreaProvider; document why the explicit SDK57 shell provider is retained. The library's nullable platform constant continues to be passed through.
- `apps/mobile/src/app-smoke.test.tsx`: expose concrete native initial metrics in the stub and assert RootLayout passes them to the provider.
- `.superpowers/sdd/plan-t31-ios27/ocr-r3-repair-report.md`: corrected its summary and documented all six R3 comment dispositions with path/line anchors and rationale.

## Verification

Used temporary ignored `node_modules` symlinks to `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`; removed both symlinks after checks.

- `pnpm --filter @weknora/mobile exec tsx --test src/ios-framework-closure.test.ts src/scripts/verify-ios-scene-project.test.ts src/app-smoke.test.tsx` — PASS, 104 tests, 0 failures.
- `pnpm --filter @weknora/mobile typecheck` — PASS (`tsc --noEmit`, exit 0).
- `bash -n apps/mobile/scripts/ios-release-build.sh` — PASS (exit 0).
- `git diff --check` — PASS (exit 0).
- `python3 -m py_compile apps/mobile/scripts/verify-ios-framework-closure.py` — PASS (exit 0).
- Generated-project verification not run: `git ls-files apps/mobile/ios | wc -l` returned `0`; no retained clean SDK57 generated output is available.

## Review package evidence

- Pre-commit tracked diff SHA-256: `9061f8ea3550a8e2600a87b355568bf6921e70d4a2c74ad2a9366d0e5bd72803` (source, tests, and updated R3 report; this report is added below).
- Source/test and initial report commit: `3dc26f767595a5d89c8d546047af62b170c1c201`; report SHA correction commit: `4b277699c352243c71cb71011d7681e46873bae6`.
- Final commit scope: only the six Task 1 owned files listed above plus this report.
