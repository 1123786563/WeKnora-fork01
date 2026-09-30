# T31 R3 F1 repair report

## Result

Resolved the PBX comment false positive. Each app target configuration ID is resolved to its configuration block, PBX block comments are stripped, and the deployment value is read only from the configuration's actual `buildSettings` dictionary. Missing configuration blocks or missing deployment values continue to fail the existing all-configurations check.

Added a Staging fixture whose only occurrence of `IPHONEOS_DEPLOYMENT_TARGET = 16.4;` is inside a block comment. Before the production fix, the new test failed because the verifier returned an empty issue list. After the fix, it rejects the fixture. Existing wrong Staging value and scene/link relationship cases remain covered.

## Verification

- `pnpm --filter @weknora/mobile exec tsx --test src/plugins/ios-xcode27.test.ts` — pass, 5/5.
- `pnpm --filter @weknora/mobile test` — pass, 296 total; 282 pass, 14 opt-in credential/environment skips, 0 failures.
- `pnpm --filter @weknora/mobile typecheck` — pass (`tsc --noEmit`).
- `pnpm --filter @weknora/mobile exec tsx scripts/verify-ios-scene-project.ts ios` — pass against the generated SDK57 iOS project.
- `git diff --check` — pass.

Source-only patch SHA-256 before this report/ledger update: `138797e6d8104b18d851aab461e02641aa812af0e775c4c29f1072d9844d5251`. Full implementation commit patch SHA-256: `78d6f86ba62c30ae0f9d6af525337870bae38a7c2545499e546ba54ad7df2483`.

## Scope and limitations

Changed only the verifier, its generated-project contract test, and this repair's progress/report records. No dependency or generated output was added. Local commit: `c69c53c8e` (`fix(mobile): require explicit deployment settings`). No push, release, or Issue mutation was performed.
