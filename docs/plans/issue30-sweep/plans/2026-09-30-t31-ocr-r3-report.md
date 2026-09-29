# T31 OCR Round 3 Repair Report

## Scope and findings

Implemented Task 1 from `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r3-repairs.md` at base `862f5a5d9bbd4d300772909876ca6ad676f3f2a6` (source commit `f3a9ca6ea`, report commit `c9c23cc90`). The exact-range OCR partial report is `docs/plans/issue30-sweep/ocr/ocr-t31-r3-final.md` (3 comments; one of 8 selected items failed), and the resumed complete report is `docs/plans/issue30-sweep/ocr/ocr-t31-r3-resumed.md` (6 comments across 8 selected items). OCR supplied no stable finding IDs; path/line anchors are used below.

## Six-comment disposition

1. `apps/mobile/scripts/verify-ios-framework-closure.py:88-96` — Medium; addressed in R4. Every unclassified non-system load now fails with `UNSUPPORTED_DYNAMIC_LIBRARY_LOAD`, including unknown dyld tokens and bare names. `/System/Library/` and `/usr/lib/` remain explicit allowed roots; the R4 fixtures cover unknown token, bare name, known token dylibs, and both allowed roots.
2. `apps/mobile/scripts/ios-release-build.sh:30-31` — Low; addressed in R3. The script comment explains that `--clean` deletes generated `ios/` output and makes each build a full rebuild.
3. `apps/mobile/scripts/verify-ios-scene-project.ts:26-29` — Low; addressed in R3. Provider conformance is checked against comment/string-masked Swift source. The R4 negative fixture removes real conformance and retains the token in both a comment and string; the valid fixture retains real conformance.
4. `apps/mobile/src/app/_layout.tsx:8-10` — Low; safe-area background/placement suggestion deferred. The codebase has no shared root theme background token. Choosing a literal light or dark color could produce a product regression; preserving the accepted SafeAreaProvider → SafeAreaView → Stack hierarchy is required by the task contract.
5. `apps/mobile/src/app/_layout.tsx:7-8` — Low; nested provider rationale documented in R4. The SDK57 shell intentionally keeps the explicit provider and its smoke-tested hierarchy.
6. `apps/mobile/src/app/_layout.tsx:7` — Low; addressed in R4. Root SafeAreaProvider receives `initialWindowMetrics`, preserving native initial inset values and nullable behavior where the native constant is unavailable.

## R4 changes

- Fail closed for every non-system load not resolved as an embedded framework dependency.
- Correct the Swift negative fixture so the comment and string markers remain after removal of actual conformance.
- Add a concrete `initialWindowMetrics` marker to the safe-area test stub and assert it is passed to RootLayout's provider.
- Pass SDK native initial metrics to the root provider and document the intentional shell contract.

## Verification

R4 command results and hashes are recorded in `.superpowers/sdd/plan-t31-ios27/ocr-r4-repair-report.md`.
