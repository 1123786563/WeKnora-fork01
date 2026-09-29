# T31 iOS 27 Scene Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute this plan task by task after approval. Each implementation task needs a fresh implementer and independent review.

**Goal:** Restore Issue #31's native iOS 27 startup by generating Expo's supported UIKit scene lifecycle, building a coherent Release dependency graph, and rendering the Deployment Login surface below system safe areas.

**Architecture:** Preserve the WeKnora Mobile Runtime, auth/capability gates, app routes, and `weknora://` callback handling. Expo SDK 57 owns generated iOS scene configuration and native startup. `apps/mobile` remains the composition root; native project output is generated for verification and is not committed. The approved spec and ADR define product/runtime boundaries but do not prescribe Expo versions or an iOS minimum; those values follow the recorded SDK compatibility ruling below.

**Tech Stack:** Expo SDK 57 (resolved SDK-compatible patches), Expo Router, React 19.2.3, React Native 0.86.3, `expo-build-properties` 57, pnpm workspace, Xcode/iOS 27 simulator, generated CocoaPods project.

**Spec sources:** Approved [mobile AI Office product spec](../../../specs/2026-09-20-mobile-ai-office-design.md), [native mobile ADR 0005](../../../adr/0005-weknora-native-mobile-client.md), [Issue #31 snapshot](../issues/issue-31.md), [issue DAG](../dag.md), and [mobile product domain context](../../../../CONTEXT.md). The predecessor implementation record is [T01 mobile runtime login plan](../../../superpowers/plans/2026-09-20-t01-mobile-runtime-login.md); the iOS 27 follow-up predecessor is [2026-09-21 scene lifecycle plan](../../../superpowers/plans/2026-09-21-t01-ios27-scene-lifecycle.md).

## Global Constraints

- WeKnora remains the identity and Tenant authorization authority. Do not expose an authorized surface before login, `/auth/me`, and capability negotiation succeed.
- Preserve routes, URL forwarding, SecureStore integration, package boundaries, and Android compatibility.
- Use Expo's generated scene delegate and project configuration. Do not hand-maintain generated `apps/mobile/ios` files or commit generated native output.
- The approved SDK57 ruling is recorded in [T31 progress](../../../../.superpowers/sdd/plan-t31-ios27/progress.md): use SDK 57 scene support and top-level `expo.ios.deploymentTarget = "16.4"`. The older predecessor's 16.0 value is obsolete for current acceptance; the approved product spec is silent on these version values.
- Keep staging credentials and secrets out of code, tests, logs, and documentation. Simulator startup does not establish external authentication or device acceptance.

## Review Focus

- Generated output maps the application scene role to Expo's supported `EXExpoAppSceneDelegate`; generated AppDelegate conforms to `ExpoReactNativeFactoryProvider` and forwards both URL callback paths.
- Every app-target Xcode configuration and effective generated deployment setting uses 16.4.
- Clean prebuild removes stale ignored native output before contract checks and CocoaPods.
- Release dependency validation checks the actual app executable and embedded frameworks, fails closed on missing dependencies or inspection errors, and matches its integration tests.
- The iOS 27 Release app visibly renders Deployment Login with safe-area spacing; no simulator process-only or JS-export evidence is treated as visible startup.
- Issue #31 remains open until external HTTPS staging login/OIDC/capability and Android device checks have evidence.

## Task and Interface Coverage

Original acceptance is preserved from Issue #31 and the predecessor plans: SDK57 scene lifecycle; generated project contract; URL forwarding; visible iOS 27 startup; safe-area layout; Release dependency closure; Android compatibility checks; and external auth/capability/device checks. The exact original step sequence is not fully reconstructable from one durable task record; the established implementation and test slices below are grounded in `task-report.md` and the repair records rather than invented as an original sequence.

| Task | Consumes → Produces | Verification and status at DOC1 base `1134dda07` |
|---|---|---|
| SDK/config and app contract | Expo app/package config → SDK57-compatible dependencies, explicit scene opt-in and iOS 16.4 target | Focused native config and Android release config tests; frozen install; Expo dependency check/doctor; mobile typecheck/tests and iOS/Android exports. Original implementation is present and reported verified in `task-report.md`. |
| Generated native project and startup | SDK57 config → generated scene manifest/delegate, callback contract and visible simulator surface | Clean prebuild contract checker, Pods, Release build, iOS27 install/launch and screenshot. Initial startup evidence is recorded in `task-report.md`; later safe-area and generated-project repairs are in `fix-progress.md`. |
| Safe-area login surface | Root navigation → protected safe-area content layout | App smoke tests and iOS27 Release screenshot. R2 verification is recorded in `fix-progress.md`; external auth remains pending. |
| Release framework closure (R4) | Built `.app` and generated mode properties → fail-closed framework checker plus retained build/launch evidence | R4 rounds 1–3 and their scoped reviews are recorded in `fix-progress.md` and `fix-task-4-r*-review.md`; repair round 3 passed scoped review. |
| Generated scene contract (R1/R3) | Clean prebuild output → structural scene, callback and target-setting verification | R1/R3 repair rounds and final scoped review are recorded in `fix-progress.md`; final R3 review passed with documented low limits. |
| Integrated final review repairs | T31 integrated implementation → repaired framework validation and current durable records | At this plan's base, final review requests T31-F1 Medium (resolve `@loader_path`/`@executable_path` dependencies), T31-F2 Medium (this missing plan), and T31-F3 Low (stale device acceptance heading). The parent repair plan assigns these to FR1, DOC1, DOC2. F1 is not complete here. |
| External acceptance | Authorized HTTPS deployment and Android device → real auth/OIDC/capability and device evidence | Pending; no credential, device, or external staging acceptance is claimed. |

## Execution and Verification Records

- Original implementation evidence and limitations: [task brief](../../../../.superpowers/sdd/plan-t31-ios27/task-brief.md), [task report](../../../../.superpowers/sdd/plan-t31-ios27/task-report.md), and [progress ledger](../../../../.superpowers/sdd/plan-t31-ios27/progress.md).
- Repair task ownership, dependencies, rounds, and current status: [T31 review repair plan](2026-09-29-t31-review-repairs.md) and [repair progress ledger](../../../../.superpowers/sdd/plan-t31-ios27/fix-progress.md).
- Integrated findings: [T31 final review](../../../../.superpowers/sdd/plan-t31-ios27/final-review.md). Its T31-F1 and T31-F2 Medium findings require resolution before release; T31-F3 is a documentation correction.
- At the plan's recorded base, original code and scoped R4 repairs are present. The final integrated review has requested new repairs; do not mark the integrated T31 slice complete until those are fixed and reviewed. This plan restores the execution record and does not self-approve implementation.
- Later execution or resumption must use the current repair ledger and exact review packages. Run only the verification affected by each repair, bind results to the reviewed code snapshot, then perform the repository's final integrated review.

## Self-Review

1. **Spec coverage:** Issue #31 acceptance and original iOS27 follow-up scope are captured; local simulator evidence is distinguished from external login/OIDC/capability/device acceptance.
2. **Version ruling:** SDK57 and iOS 16.4 derive from the explicit T31 progress ruling and current generated-project checks, not from the obsolete 16.0 predecessor value.
3. **Repair state:** R1/R3 and R4 scoped repair status follows `fix-progress.md`; the new integrated T31-F1 is expressly pending.
4. **Provenance limitation:** The full original task step sequence is not present as a single durable record. This plan summarizes proven work and verification from execution reports and links the predecessor instead of asserting unrecoverable steps.
5. **Placeholders:** None.

## Status and Limitations

This is the restored current T31 implementation plan at the DOC1 starting revision. Implementation and scoped repair evidence exists, while the integrated final review is not yet clean. HTTPS staging authentication/OIDC/capability checks and Android device acceptance remain pending. This document does not claim external acceptance or approve its own completeness.
