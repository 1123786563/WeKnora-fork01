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

## Task R4 clean Release framework closure
- Diagnosis: prior clean SDK57 output had `EXPO_USE_PRECOMPILED_MODULES=true`, `React-Core` source pod only, no `React-Core-prebuilt`, while precompiled `ExpoModulesWorklets.framework` binary load commands required `@rpath/React.framework/React`; embedded frameworks script copied Worklets but not React, producing the R2 dyld launch failure. `expo install --check` passed, so versions were aligned; generated framework mode was inconsistent.
- Fix: `apps/mobile/app.json` now selects supported `buildReactNativeFromSource: true` and `usePrecompiledModules: false`. In generated Pods this yields static/source React and Expo modules; app bundles no `ExpoModulesWorklets.framework` and no `React.framework`, and the main executable has only resolvable embedded dynamic framework loads (`ExpoModulesJSI`, `hermesvm`). Script contract inspects every embedded dynamic framework with `otool -L`, fails closed on inspection errors/missing @rpath frameworks, and rejects a precompiled Worklets framework under source mode. Evidence output supports `IOS_BUILD_EVIDENCE_DIR` so R4 runs preserve the T39 log.
- TDD: load-command fixture rejects Worklets → missing React; complete graph passes; source/precompiled mode assertions added. Existing app config contract updated for the supported settings.
- Native verification: canonical clean script exit 0, `** BUILD SUCCEEDED **`, `FRAMEWORK_MODE=source-expo-modules`, `FRAMEWORK_CLOSURE_OK`. Evidence log `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/xcodebuild-release.log`. Installed/launched on iPhone 18 Pro iOS 27.0 (UDID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC`); app evaluated JS bundle and rendered sign-in root without dyld loader error. Screenshot `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-launch.png`, SHA256 `2bcd87c0d9e6c757c59348fcad03717466b724b5fe97c2dbb08a0d240ce25485`.
- Checks: mobile suite 303 total / 289 pass / 14 environment skips / 0 fail; typecheck pass; `expo install --check` pass; `git diff --check` pass. Negative missing-React fixture emitted `MISSING_FRAMEWORK_DEPENDENCY: ExpoModulesWorklets.framework requires React.framework` and exited 1.
- T39 tracked Release log SHA256 verified at original `12a27ed4f698074356c5e1d7722b5d7af78b2d60b5cdf9e9856e44e37b7bb71e`.
- Initial implementation commit: `eb821ee9d9e6f34dba49beb2d336c7e8396ff8de`; Task R4 review package BASE `003f12cc1456b0eba6578c6c164474c31fa9fc70`, HEAD `eb821ee9d9e6f34dba49beb2d336c7e8396ff8de`, SHA256 `0e6c31b7f75fd2c61036356f0045317ef13f512b842c67502b8d785eae4a08e2`.
- Independent screenshot-only frontend validation: visible login title/form below status bar, screenshot SHA256 `2bcd87c0d9e6c757c59348fcad03717466b724b5fe97c2dbb08a0d240ce25485`; cannot alone bind pixels to app binary.
- Independent Task Review: initial verdict Spec FAIL / Quality FAIL; first re-review verdict Spec PASS / Quality FAIL with one Medium closure false positive and one Low mode-label issue. Full evidence is in `.superpowers/sdd/plan-t31-ios27/fix-task-4-review.md`.
- Ruling on brief criterion: approved Issue/spec/ADR do not prescribe dynamic React.framework packaging. Corrected the Task Brief to accept coherent supported Expo source mode when the app launches and all actual non-system framework dependencies resolve. Cost if wrong: undocumented release policy could require standalone React.framework; no evidence of such a policy was found.
- Status: round-1 repair addressed the original High and Medium findings; a new Medium false-positive path remains. Round-2 repair is running.

### R4 round-1 preflight consistency scan

| Pair / Task | Shared file or interface | Check | Ruling |
|---|---|---|---|
| Release script ↔ closure checker | Release script invokes `verify-ios-framework-closure.py`; tests execute the same entry point | Single production algorithm is exercised with fake `otool`; no disconnected duplicate logic | Keep checker as the owned seam |
| Checker ↔ test fixtures | `.app/Info.plist`, main executable, `.app/Frameworks/*.framework/Info.plist` binaries | XML and binary plist support comes from Python stdlib `plistlib`; fixtures pin executable resolution and fail-closed errors | No third-party dependency |
| Build/launch evidence ↔ report | compressed full build log, raw simulator output, executable/screenshot hashes | Use tracked artifact paths and manifest fields; keep original T39 hash unchanged | Evidence must survive ignored-worktree cleanup |
| R4 round 1 internal consistency | acceptance asks for current-app closure, not a specific dynamic React artifact | Current source-mode app launched without dyld error; production checker and test execute same code | Approved Issue/spec/ADR do not specify dynamic linking; corrected brief and recorded the cost if this ruling is wrong |

- Agent and model: resumed `/root/t31_fix_native_release_closure` as `frontend_implementer`; local commit strategy; no subdelegation; no native rebuild.
- Task base: `eb821ee9d9e6f34dba49beb2d336c7e8396ff8de`.
- Repair commits: `c4c16ad6cef751cec16e1c5bf22f694f3a83679e` (implementation), `d4e15789d80c0065bd3b0fd737967e01379a7f08` (report correction). Scoped re-review is now pending.
- Evidence: focused 8/8; full suite 306 total / 292 passed / 14 opt-in skips / 0 fail; typecheck, Expo install check and diff-check passed per the implementation report. The production checker passed on the retained Release artifact; no native rebuild occurred. Full log gzip and simulator command output are tracked in the repair commit.
- Round-1 re-review package: `.superpowers/sdd/2026-09-29-t31-review-repairs/review-eb821ee9d..1ebf2dfe5.diff`, SHA-256 `c7b0770f29dd69bf1f1e906539bdc857d239a7671a99cfe3b6811f2312c2c4d1`. Reviewer `/root/review_t31_r4_r1`: Spec PASS; Quality FAIL. Medium R4-R1-1 is bundle containment/path identity; Low R4-R1-2 is mode inference. Versioned path false rejection recorded as nonblocking edge case.
- Round-2 ruling: verify the canonical resolved dependency path so legitimate `Versions/A` layouts work while sibling bundle paths fail; mode output derives from effective generated Podfile properties. Cost if wrong: unsupported versioned layouts might be rejected or a future Expo property shape may require checker updates.
- Round-2 brief: `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-brief.md`; new implementation base `1ebf2dfe5`.
- Updated brief correction committed `3bf8d0082`; implementation agent resumed with frontend_implementer role.
- Round-2 independent review `.superpowers/sdd/plan-t31-ios27/fix-task-4-r2-review.md`: Spec FAIL only on fixture fidelity; Quality PASS with Low R4-R2-1. Production implementation fixes prior findings; sibling test wrote `../Beta/Alpha` although the existing executable lives at `Beta.framework/Alpha`, and no sibling-framework symlink case existed. Valid coverage finding; no production false pass identified.
- Round-3 brief `.superpowers/sdd/plan-t31-ios27/fix-task-4-r3-brief.md` limited changes to integration fixtures and the report. Implementation commits `1818cd5a5` / `196f85d02`; focused 13/13, full suite 311 total / 297 pass / 14 opt-in skips, typecheck, Expo check and diff-check pass per report. No native rebuild or evidence change.
- Round-3 review `.superpowers/sdd/plan-t31-ios27/fix-task-4-r3-review.md`: Spec PASS / Quality PASS; R4-R2-1 resolved by real sibling executable and sibling symlink fixtures. Scoped review covered `10c05c1e7..196f85d02`. R4 repair task is verified; T31 final integrated review remains pending.

## T31 final integrated review

- Exact review slice: base `1e9315773a971dd72fe621c94e308f45a0ca4692`, head `894d456cbf1c3506563204b4e018de02ce4e6700` (T31 only; earlier T55 work excluded). Report: `.superpowers/sdd/plan-t31-ios27/final-review.md`.
- Verdict: Spec Partial / Code Quality Changes Requested. T31-F1 Medium: production framework verifier silently ignores unresolved `@loader_path` and `@executable_path` framework loads; disposable production-checker fixture reproduced exit 0 + `FRAMEWORK_CLOSURE_OK`. Retained app itself currently passes and launches.
- T31-F2 Medium: cited current plan `docs/plans/issue30-sweep/plans/plan-t31-ios27.md` is absent. T31-F3 Low: device acceptance heading still calls JS startup unresolved despite later local Release login/safe-area evidence.
- Repair plan: `docs/plans/issue30-sweep/plans/2026-09-29-t31-final-review-repairs.md`, exact initial implementation HEAD `894d456cbf1c3506563204b4e018de02ce4e6700`; independent FR1/DOC1/DOC2 are now running in isolated worktrees from `1134dda07`.
- Final review verified local iOS27 Release startup and safe-area screenshot; staging password/OIDC, real Deployment capability and Android hardware acceptance remain external pending work and do not block these local checker/document repairs.
- Plan committed at `a2c695f18`; task briefs committed at `1134dda07`. Three independent tasks were dispatched in isolated worktrees from base `1134dda07`: FR1 to frontend_implementer (`t31-fr1-framework-closure`), DOC1 to mechanical_worker (`t31-doc1-plan-record`), and DOC2 to mechanical_worker (`t31-doc2-acceptance-record`). All statuses are running; no code changes are integrated yet.
- FR1 round 1 implementation `ed9754492` and report `97cf8dc86` passed implementation checks, but independent review `fr1-review.md` found two Medium false passes (malformed framework name without binary suffix; broad otool path-prefix header filter). Do not integrate round 1.
- FR1 repair round 2 brief `.superpowers/sdd/plan-t31-ios27/fr1-r2-brief.md` adds exact negative fixtures and exact-header matching. Same frontend_implementer and isolated FR1 worktree; status running. DOC1/DOC2 scoped reviews passed and await integration after FR1 in plan order.
- FR1 round 2 implementation `677f5e792` and report `4dfe529c7` now committed. RED reproduced both findings; focused 20/20, full suite 318 total / 304 pass / 14 gated skips, typecheck, Expo check, and diff-check pass per report. Retained app closure and SHA remain valid, no native build/evidence changes. Independent scoped review is pending; do not integrate until it passes.
- FR1 round-2 review `.superpowers/sdd/plan-t31-ios27/fr1-r2-review.md`: Spec PASS; Quality PASS with Low FR1-R2-1. A substring fallback rejects `@rpath/Foo.framework.dylib`, which is not a framework bundle. FR1 round-3 brief `.superpowers/sdd/plan-t31-ios27/fr1-r3-brief.md` requires complete component matching and a positive dylib fixture; same frontend implementer/worktree, status running.
- FR1 round-3 commits `362b474e8` / `d9482bb7c` were reviewed in `.superpowers/sdd/plan-t31-ios27/fr1-r3-review.md`: Spec PASS / Quality PASS, no findings. Focused 21/21, full 319 total / 305 pass / 14 skips, typecheck and Expo check pass; retained checker passes and SHA unchanged. FR1 commits through `d86bc885b` are integrated; focused tests rerun in integration 21/21 and retained checker/hash pass. No build/evidence changes.
- DOC1 `d85de58a00bdbbd64bb4ea7f96a9cf4fd8513eaa` and DOC2 `0792167e6a9fe97e59b40b74c7fcd9ab609b62b3` implementation commits are locally verified; independent DOC1/DOC2 reviews both PASS.
- DOC1 is now integrated as `d02b244fb`; independent review verdict PASS, no findings. DOC2 implementation/review is verified and next in integration order.
- DOC2 commit `70c8efd92` is now integrated after DOC1; its scoped review `.superpowers/sdd/plan-t31-ios27/doc2-review.md` is PASS with no findings. FR1/DOC1/DOC2 task commits are integrated; T31 remains pending a fresh integrated final review.

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

## Integrated final-review documentation repair DOC3
- Integrated review at `057b46e0759e74f41f64e0aa1babd75900800206` independently verified FR1/DOC1/DOC2 and found Low T31-R2-F1: DAG and Issue index still describe local iOS 27 black-screen/scene startup as unfixed despite retained Release login evidence.
- Ruling: valid required record correction. Keep #31 open and `blocked-external (partial)` because staging/OIDC, real Deployment capability, and Android device acceptance are still pending. Cost if wrong: stale DAG could misstate a completed local fix or incorrectly release external acceptance.
- DOC3 plan: `docs/plans/issue30-sweep/plans/2026-09-30-t31-dag-status-repair.md`; brief `.superpowers/sdd/plan-t31-ios27/doc3-brief.md`; implementation base `057b46e0759e74f41f64e0aa1babd75900800206`; status running.

## Integrated final-review finding T31-R2-F1 / DOC3 disposition
- DOC3 implementation commits `c31c721bc` and `1c1566c3e` were integrated as `09ea062e7` and `c7e18124b`; the standalone plan and both prior/focused review records are preserved in the integration worktree.
- DOC3 scoped independent review `.superpowers/sdd/plan-t31-ios27/doc3-review.md`: Spec Compliance PASS; Code Quality PASS; no findings. It confirms #31 remains open / blocked-external (partial), the local iOS 27 startup/login layout wording matches evidence, and HTTPS staging password/OIDC, real Deployment capability, and Android hardware acceptance remain pending.
- T31-R2-F1 is resolved. The reviewed integration snapshot is `2cceb281f84eb4c3ec00911f4ed96cf871460ae2`; request a fresh integrated review over `1e9315773a971dd72fe621c94e308f45a0ca4692..HEAD` before marking local T31 verified.

## OCR round 1 and expanded round 2
- OCR R1: exact T31 range `1e9315773a971dd72fe621c94e308f45a0ca4692..860b84016c8c6c94edd8c8adcd1ca672132efd17`, exit 0, report `.superpowers/sdd/plan-t31-ios27/ocr-t31-r1.md`; initially selected 7/77 paths. Independent adjudication `.superpowers/sdd/plan-t31-ios27/ocr-t31-r1-adjudication.md` found the two blocking claims false (Expo SDK57 deployment setting is valid; actual PBX configuration list uses comma-array references and both production prebuild verifiers exit 0), and found no applicable script testing gap. Low style/dependency suggestions were recorded for disposition.
- OCR R2 expanded via custom include rule to test paths: same range, exit 0, 14 code/test files selected, 10 comments, report `.superpowers/sdd/plan-t31-ios27/ocr-t31-r2.md`, session `691fd0b0-300c-48b7-9c82-2ff06b048502`. Session manifest: 14 selected, 14 completed, 0 failed; one LLM request cancellation is named for `_layout.tsx` + `app-smoke.test.tsx`, so final run must recover cleanly. Markdown and binary artifacts are unsupported by OCR; documentation was independently reviewed in SDD/DOC3. Preview without include selected 7 and excluded tests as `default_path`; the custom include raised it to 14.
- Independent R2 adjudication: `.superpowers/sdd/plan-t31-ios27/ocr-t31-r2-adjudication.md`. Valid Medium: smoke test does not assert SafeAreaProvider contains SafeAreaView. Valid Lows include Stack diagnostic, framework properties non-object error path, app-boundary load fixture, `.gitignore` stale comment, scene test filename, and nested ternary readability. Expo-asset plugin failure is false; old plugin deletion is not required by prior R3 plan; dependency directness is low and deferred.
- Repair plan `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r2-repairs.md`; task briefs `ocr-r2-1-brief.md`, `ocr-r2-2-brief.md`. Source-review HEAD before repair plan: `860b84016c8c6c94edd8c8adcd1ca672132efd17`; task worktree BASE: `1df6f5cb2cdfd4253941abc5b113e778248cd189`. Both pending; task files/interface preflight is recorded in the plan.
- Dispatch record: Task R2-1 was assigned to `frontend_implementer` (fixed role model gpt-6-luna / low); Task R2-2 to `mechanical_worker` (fixed role model gpt-6-luna / medium). Both are running in isolated worktrees from task BASE `1df6f5cb2cdfd4253941abc5b113e778248cd189`; source review HEAD is `860b84016c8c6c94edd8c8adcd1ca672132efd17`. Ownership is non-overlapping as listed in the repair plan.
- R2-1 initial implementation commit `ad68a23a83631e336400ae8167ee4bf2307fa225`; targeted smoke 74/74 and diff-check passed. Independent review `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-review.md` found valid Medium root assertion omission (SafeAreaProvider not asserted as rendered root) and Low report omission (commit SHA absent). Round-1 brief `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-r1-brief.md`; same implementer resumed; status running repair. The prior task worktree includes a coordinator-copied updated plan/brief; no shared source overlap with R2-2.
- R2-1 first review found Medium missing root assertion and Low report SHA; same frontend implementer fixed both in `187a0dfdda957ac6372b4d34e58fe103502ac502`. RED Fragment wrapping failed at root assertion; final targeted smoke 74/74 and diff-check passed; re-review pending.
- R2-2 implementation commit `a13263252533db91abf2baab2f00360e4c893543`; scoped review `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-review.md`: Spec Compliance PASS, no blocker. Valid Low: JSON invalid-root fixture exists but plist invalid-root path lacks matching case, contrary to plan acceptance. Round-1 brief `.superpowers/sdd/plan-t31-ios27/ocr-r2-2-r1-brief.md`; same mechanical implementer resumed, status running. Focused 28/28 reported by implementation; mobile typecheck unavailable in isolated tree, to rerun in integration.
- R2-1 round 1 re-review `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-r1-review.md`: Spec PASS and Code Quality PASS for hierarchy assertions; remaining Low report references intermediate `9ff7dab...` instead of final task commit `187a0df...`. Report-only correction brief `.superpowers/sdd/plan-t31-ios27/ocr-r2-1-r1-doc-fix-brief.md`; same implementer requested to correct.
- R2-1 implementation and repair commits integrated as `14fc187b0` / `5b1b5b4f4`, report correction `e9f7204b2`. Scoped review and repair reports are now preserved. Review repair R1 Spec PASS / Quality PASS; root Provider assertion and report SHA finding resolved.
- R2-2 implementation and plist repair commits integrated as `75b90a05c` / `2c31b8a8e`, checkpoint report `96ad09279`. Main review and repair review Spec PASS / Quality PASS; JSON and plist roots, outside-app loads, test rename, `.gitignore`, and lexer updates reviewed. Focused tests reported 28/28 and 19/19; isolated worktree typecheck was blocked for missing deps. Integration typecheck and all retained generated prebuild checks pending.
- Integrated OCR R2 code repair range now ends at `96ad092796ead8dbf44763e2693ed0a9ce5aee31`; final combined targeted verification and full OCR rerun remain pending.
- R2-1 and R2-2 integrated scoped reviews are preserved and pass Spec/Quality; report SHA low item and plist fixture low item are resolved.
- Integrated verification at `96ad092796ead8dbf44763e2693ed0a9ce5aee31`: `pnpm --filter @weknora/mobile exec tsx --test src/app-smoke.test.tsx src/ios-framework-closure.test.ts src/scripts/verify-ios-scene-project.test.ts` — **102/102 pass**; `pnpm --filter @weknora/mobile typecheck` — pass; retained Release app checker — `FRAMEWORK_MODE=source-expo-modules`, `FRAMEWORK_CLOSURE_OK`.
- Production scene verifier passed against current SDK57 prebuilds `/private/tmp/weknora-issue31-prebuild-p60v8_1x/ios` (created 2026-09-29 19:11) and `/private/tmp/weknora-issue31-final-prebuild-_x7w61dy/ios` (created 19:14). The earlier `/private/tmp/weknora-issue31-prebuild-ejlf6a8k/ios` (19:02) fails the newer scene contract because its generated Info.plist/AppDelegate predate the scene integration; it is a retained historical intermediate, not the supported final prebuild. This was recorded as provenance, not silently treated as current acceptance evidence.
- Final T31 supported-code OCR rerun pending after these integrated source/test repairs. Its custom include rule admits T31 mobile `src/**/*.test.ts*` (14 selected code/test items from the prior preview). Markdown/evidence paths remain unsupported by OCR and have independent SDD reviews, explicit acceptance records and hash/path verification. Do not claim an unqualified OCR pass until the final session manifest is complete and findings are adjudicated.
