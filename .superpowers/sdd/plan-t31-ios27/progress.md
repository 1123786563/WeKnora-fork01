# SDD ledger — plan: docs/plans/issue30-sweep/plans/plan-t31-ios27.md

## Recovery facts
- Root Issue #30; current DAG node Issue #31 / T01 / B0. Snapshot: `docs/plans/issue30-sweep/issues/issue-31.md`; DAG: `docs/plans/issue30-sweep/dag.md`.
- Approved product spec: `docs/specs/2026-09-20-mobile-ai-office-design.md`; applicable ADR: `docs/adr/0005-weknora-native-mobile-client.md`; domain terms: `CONTEXT.md`.
- Execution worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-b0-t31`; branch `codex/issue30-b0-t31`; BASE `db234c5eb171f2dde7427d382b55b503a038f879`.
- Plan: `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`, copied from `docs/superpowers/plans/2026-09-21-t01-ios27-scene-lifecycle.md` and revised with current official Expo SDK 57 constraints. SDD workspace: `.superpowers/sdd/plan-t31-ios27/`.
- User explicitly authorized full Issue #30 implementation and selected SDD; no extra plan confirmation gate.
- Ruling: the approved product spec is silent on Expo version/minimum OS. The follow-up addresses Issue #31 iOS 27 startup. It supersedes the parent plan’s Expo 55 implementation pin for this repair; current Expo SDK 57 docs require iOS 16.4+, so set top-level `expo.ios.deploymentTarget=16.4` (not the deprecated plugin field). Cost if wrong: version/config change may reduce supported iOS range or create upgrade churn; native build verification is in scope.
- Ruling: execute deterministic local simulator/build/config subset. Do not inspect or fabricate credentials; staging login/OIDC and Android device acceptance remain explicitly pending per Issue #31 comments and DAG external blocker.

## Environment preflight (2026-09-29)
- Node `v26.7.0`, pnpm `10.28.2`, Xcode `27.0 (27A266a)`, CocoaPods `1.17.0`.
- iOS 27.0 iPhone 18 Pro simulator UUID `0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC` is booted.
- Worktree started at clean BASE; root checkout’s unrelated web edits were not carried into this worktree.

## Task status
- Task 1–3 combined vertical task: pending.
- External acceptance status: staging credentials, installed iOS auth/OIDC, and Android device evidence are blocked/pending; do not close Issue #31.
- Task scope refinement after read-only research: `apps/mobile/src/android-release-config.test.ts` hard-codes Expo 55 versions for expo-audio/network/file-system and must be updated to the selected SDK57 compatibility versions. This is a directly affected contract test, not an unrelated scope expansion. Research found Expo SDK57 does not require TypeScript6 specifically; root already pins TS6. No downgrade based solely on SDK migration.
