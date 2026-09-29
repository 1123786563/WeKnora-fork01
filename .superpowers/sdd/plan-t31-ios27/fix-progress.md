# SDD repair ledger — T31 review findings

- Root Issue: #30 descendants, task #31/T01 iOS27 scene lifecycle.
- Source review: `.superpowers/sdd/plan-t31-ios27/review.md`, findings R1 medium, R2 medium, R3 low.
- Plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-review-repairs.md`.
- Workspace: `/Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont`.
- BASE: `db234c5eb171f2dde7427d382b55b503a038f879`; starting integrated HEAD `62ee15032`.
- Commit policy: local commits allowed, no push/merge/deploy/Issue mutation.
- Status: all findings pending; implementation begins only after task brief/preflight.

## Task R1+R3 dispatch
- Brief: `.superpowers/sdd/plan-t31-ios27/fix-task-1-brief.md`.
- Owned files: release script, its source test, generated contract test and retired plugin test only.
- Interface: clean prebuild produces the ignored SDK57 `apps/mobile/ios` tree; contract helper consumes that directory before CocoaPods.
- Commit strategy: local commit only; no push.
- Status: ready → running; R1 and R3 implementation combined because script/test boundary overlaps.

## Task R1+R3 implementation checkpoint
- Implementation: clean Expo prebuild and generated project contract verifier added; retired SDK55 transform tests replaced by generated-output fixtures.
- Focused tests: 6/6 pass; full mobile tests: 293 total, 279 pass, 14 env/credential-gated skips; typecheck passes; `git diff --check` passes.
- Native validation limitation: attempted stale sentinel + `pnpm --filter @weknora/mobile exec expo prebuild -p ios --clean --no-install`; Expo config resolution stopped before generation because `expo-build-properties` could not resolve. Resolver stack uses Expo 55 config packages. Created sentinel/tree was removed afterward. No generated output or Pods/build run.
- Status: implementation complete; clean SDK57 output validation pending environment dependency repair; independent task review pending.
- Ruling: copied the read-only review into the integration workspace and corrected ADR pointer to `docs/adr/0005-weknora-native-mobile-client.md`; added `apps/mobile/scripts/verify-ios-scene-project.ts` as explicit owned helper to keep validation reusable/testable. Cost if wrong: small task-scope expansion, bounded to the exact generated native project contract requested by R3.
- Updated brief correction committed `3bf8d0082`; implementation agent resumed with frontend_implementer role.

## Task R1+R3 integration verification addendum
- Agent initial report noted `expo-build-properties` resolution failure under stale Expo 55 worktree modules. Ruling: run `pnpm install --frozen-lockfile` in the integration worktree before prebuild because its tracked SDK57 manifest/lock were integrated while ignored node_modules links remained from SDK55; risk is no source changes, only worktree dependency alignment.
- Re-ran clean SDK57 prebuild successfully; contract check passed. Added `apps/mobile/ios/STALE_SDK55_SENTINEL`, repeated `--clean` prebuild, confirmed sentinel removal, then contract passed. Xcode generated output is ignored. See Task 1 report addendum.
- Task R1+R3: running review; implementation commit `a74d4280588009fcb481afda6e8414548a9c63b6`.

## Review repair round 1: R3 F1
- Independent review `.superpowers/sdd/plan-t31-ios27/fix-task-1-review.md`: R1 resolved; R3 partial. F1 Medium says checker may false-pass mismapped application scene, URL callback missing while marker occurs elsewhere, and one drifted Xcode setting.
- Ruling: strengthen actual relationships and negative fixtures; reviewers supplied direct generated output and scenario evidence, so issue is valid. Cost if stricter parsing misreads a future legitimate Expo template: clean SDK bump requires updating contract/fixture.
- Brief `.superpowers/sdd/plan-t31-ios27/fix-task-1-r1-brief.md`; owned files bounded to checker/tests/report/ledger.
- Status: pending → running after implementation role dispatch.

## Review repair round 2: remaining R3 F1
- Re-review after round 1 found that only inline Debug/Release-named config IDs were included; a referenced Staging app configuration could drift unnoticed. Low test gaps remain for unrelated scene marker and universal-link-specific forwarding.
- Ruling: accept and fix; the gap is valid under the requirement “all app target configurations.” Cost if wrong: added parsing strictness might need amendment for new Xcode project format/config shape.
- Brief `.superpowers/sdd/plan-t31-ios27/fix-task-1-r2-brief.md`; exact owned code/test/report files recorded.
- Status: pending → running.


## Review repair round 2 implementation
- Scope: parse every WeKnora target configuration ID regardless of comment/name; fail closed for unresolved ID or missing/non-16.4 deployment setting. Fixture now covers Debug/Release/Staging and unrelated Pods value; negative cases cover Staging drift, wrong effective application scene mapping while marker remains elsewhere, and universal-link-specific forwarding while open-URL remains.
- TDD: RED reproduced with the old Debug/Release-only parser (focused test 3 pass / 1 fail; Staging 16.0 returned no issue); GREEN focused test 4/4.
- Verification: mobile suite 295 total, 281 pass, 14 opt-in skips, 0 fail; mobile typecheck pass; actual generated SDK57 tree contract pass; diff-check pass.
- Patch SHA-256: `874a6972170d5078632853d4335c98064efebb73d484aee44d998b174ea78530`; commit `1eb90c6e15a96420d26931aed18cfd88521a6ea5`; local worktree clean.
- Status: implementation complete; independent round-2 review pending.

## Review repair round 3: PBX comment false positive
- Review `.superpowers/sdd/plan-t31-ios27/fix-task-1-r2-review.md` found raw regex could accept a 16.4 string inside a PBX comment for a missing app setting. This is valid Medium because missing settings must fail closed.
- Ruling: strip block comments and read actual per-configuration buildSettings; add an absent-key-with-comment fixture. Cost if too strict: generated PBX syntax changes require a corresponding parser/fixture update.
- Brief: `.superpowers/sdd/plan-t31-ios27/fix-task-1-r3-brief.md`; implementation scope only checker/test/report/ledger.
- Status: pending → running.

## Review repair round 3 implementation
- Added Staging negative fixture with no actual deployment key and only `/* IPHONEOS_DEPLOYMENT_TARGET = 16.4; */`; RED reproduced against the previous verifier (contract incorrectly returned no issues).
- Verifier now strips PBX block comments before resolving each referenced XCBuildConfiguration and extracts `IPHONEOS_DEPLOYMENT_TARGET` only from that configuration's actual `buildSettings` dictionary. Test fixtures now model the generated PBX nesting.
- Verification: focused iOS project contract tests 5/5 pass; full mobile tests 296 total, 282 pass, 14 credential/environment-gated skips, 0 fail; mobile typecheck passes; generated SDK57 `ios` project contract passes; `git diff --check` passes.
- Source patch SHA-256: `138797e6d8104b18d851aab461e02641aa812af0e775c4c29f1072d9844d5251` (before report/ledger documentation edits).
- Status: implementation verified; local commit pending.

## Review repair round 4: Swift-comment URL forwarding false positive
- Review `.superpowers/sdd/plan-t31-ios27/fix-task-1-r3-review.md`: PBX comment finding resolved; new Medium F2 says callback body substring search can accept commented-out RCT forwarder.
- Ruling: strip Swift comments before per-method assertion and pin both callback cases. The finding is valid because release gate must prove executable URL handoff; cost if wrong is parser sensitivity to Swift comments/strings, mitigated by focused tests.
- Brief `.superpowers/sdd/plan-t31-ios27/fix-task-1-r4-brief.md`; status pending → running.
