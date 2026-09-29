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

## Review repair round 4 checkpoint
- R3 R4 implementation committed `9937fa095`; report commit `8619b5f70`. Focused 6/6, full mobile 297 total (283 pass, 14 opt-in skips), typecheck, generated SDK57 verifier and diff-check passed.
- R4 includes a dependency-free Swift comment scanner and explicit comment-only negatives for both callback bodies plus URL string control. Prior repair rounds corrected distinct false positives in app-target config extraction.
- R4 reviewer upgrade ruling: use default agent with explicit `gpt-6-sol` high for independent review. Fixed `reviewer` role exposes Sol medium and does not permit a reasoning override; the scanner/parser now spans Swift lexical edge cases and the R3 checker has had multiple rounds of newly found false passes, so high-effort adversarial review is justified. No implementation escalation; frontend_implementer handled the repair.

## Review repair round 5: executable Swift call validation
- High reasoning review report `.superpowers/sdd/plan-t31-ios27/fix-task-1-r4-review.md` found call text could come from a string and commented signature could be mistaken for real open-URL override. Both are valid Medium findings.
- Ruling: mask literals and comments before signature/call matching; add both counterexamples. This final plan round has one high-reasoning reviewer; if any residual finding remains, adjudicate accurately rather than exceed the five-round limit.
- Model/role upgrade: earlier fixed frontend implementer (gpt-6-luna) repeatedly fixed the specific false-pass classes but the latest adversarial review exposed lexical parsing limitations. Assign default + explicit gpt-6-sol/high with strict frontend-only file ownership to meet round 5 upgrade and model-task boundary.
- Brief `.superpowers/sdd/plan-t31-ios27/fix-task-1-r5-brief.md`; status pending → running.

## Review repair round 5 implementation checkpoint
- RED: two new fixtures exposed string-only forwarding and a commented open-URL signature (focused suite 6 pass / 2 fail before checker edit).
- The checker now uses a positional Swift lexical mask for signature, brace, and call matching, with balanced declaration parameters and required `-> Bool {` binding. String/comment markers cannot satisfy callback forwarding.
- GREEN: focused 9/9; full mobile 300 total, 286 pass, 14 opt-in skips, 0 fail; typecheck passed; actual generated SDK57 project checker passed; source diff-check passed.
- Source/test patch SHA256 `82fd31a20f95192bae15fc194ea8dcc41e086f11d64c7bde3098c1a2f3fd8d0f`; local implementation commit `3060a67a952959561b4e453e21dbe2d1feb18d55`.
- Report `.superpowers/sdd/plan-t31-ios27/fix-task-1-r5-report.md`; status implementation verified, independent final round review pending.

## R1/R3 final disposition and R2 release
- R1 clean prebuild fix: verified after actual SDK57 clean prebuild and old sentinel removal; focused release script test passes; full Pods/Xcode on the post-repair script was not repeated.
- R3 generated project contract: five bounded repair rounds completed; final high-review report `.superpowers/sdd/plan-t31-ios27/fix-task-1-r5-review.md` gives Spec PASS / Quality PASS, no actionable C/H/M. Two low limits documented: bounded Swift syntax matching may need future SDK template update, and static call token presence cannot prove runtime reachability (the generated delegates have direct calls).
- Task R2 safe-area brief `.superpowers/sdd/plan-t31-ios27/fix-task-2-brief.md`; now ready; release after R3 final review passed.

## Task R2 safe-area layout
- Status: implemented; mobile tests/typecheck and native Release build passed. Simulator runtime acceptance blocked by dyld packaging failure before UI launch.
- TDD: added root shell structural assertion; RED on original bare Stack (focused suite 73 pass / 1 fail: missing SafeAreaProvider). GREEN after centralizing `SafeAreaProvider > SafeAreaView(edges=['top','bottom']) > Stack(headerShown:false)`.
- Checks: focused smoke 74/74; full mobile suite 301 total, 287 pass, 14 credential/environment-gated skips, 0 fail; `pnpm --filter @weknora/mobile typecheck` pass; `git diff --check` pass.
- Native: `bash apps/mobile/scripts/ios-release-build.sh /Users/wuyongjun/.paseo/worktrees/144ixsa6/issue30-b6-t55-cont` clean-generated SDK57 scene contract passed and Xcode Release reported BUILD SUCCEEDED. Installed to iPhone 18 Pro iOS 27.0 UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`; `simctl launch --console` immediately exits: `dyld: Library not loaded: @rpath/React.framework/React`, referenced by `ExpoModulesWorklets.framework`; `.app/Frameworks` contains no React.framework. Captured screenshot is SpringBoard, not app acceptance, and is not valid safe-area evidence. Fresh login screenshot remains blocked pending native packaging fix outside R2 ownership.
- Build log: `docs/testing/evidence/mobile-runtime-login/2026-09-29-r2/xcodebuild-release.log`, SHA-256 `9f484fd2704cdf9a028dfa088f6f00a0791ff76330c6f3f296df256b0837db0b`. The canonical script overwrote pre-existing T39 build log during execution; restored from HEAD and verified original SHA-256 `12a27ed4f698074356c5e1d7722b5d7af78b2d60b5cdf9e9856e44e37b7bb71e` exactly. Invalid SpringBoard screenshot at `docs/testing/evidence/mobile-runtime-login/2026-09-29-r2/ios27-release-safe-area.png` is retained only to document the failed launch capture.
- Source SHA-256: `_layout.tsx` `1dcb178d008ad9418a1a394d0fe38dd49593232b458a4a6de5f7508c19e9744a`; `app-smoke.test.tsx` `48c15e53741c991a7eb0ddac719b95346a309cf41e12c8030dee439b5ef4a7b5`. No staging/OIDC or Android claims.
