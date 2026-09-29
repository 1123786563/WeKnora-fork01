# DOC3 Report — Reconcile stale T31 status records

Base: `057b46e0759e74f41f64e0aa1babd75900800206`

## Changes

- `docs/plans/issue30-sweep/dag.md`: updated the #31 external-blocker summary, the B0 actionable-subset note, and the status-table comment. They now record that local iOS 27 UIScene/startup is fixed and the R4 Release login screen is below the status bar, while HTTPS staging password/OIDC, real Deployment capability acceptance, and Android physical-device evidence remain pending. #31 remains `open` and `blocked-external (partial)`.
- `docs/plans/issue30-sweep/issues/index.md`: updated #31's handling decision with the same local resolution and outstanding external acceptance gaps; retained `open` and `blocked-external (partial)`.

## Evidence inspected

- `docs/testing/mobile-runtime-login-device-acceptance.md`, including its current iOS simulator status and explicit pending staging/OIDC, capability, and Android checks.
- `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-launch-r1.png`
- `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/ios27-release-deployment-login-safe-area.png`
- `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/simctl-launch-r1.txt`
- `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/simctl-install-launch.txt`
- `docs/testing/evidence/mobile-runtime-login/2026-09-29-r4/xcodebuild-release-final.log.gz`
- `.superpowers/sdd/plan-t31-ios27/final-review-after-repairs.md`, finding `T31-R2-F1` and its required correction.

## Checks

- `test -f` for the acceptance record and each listed R4 evidence artifact: passed.
- `git diff --check`: passed.

No application code was changed. The local simulator evidence establishes startup and visible layout at capture time only; it does not establish long-duration stability, interactive login, staging/OIDC behavior, actual Deployment capability negotiation, or Android device acceptance. Those checks remain pending.

## Commit

Status-document changes and this report were first committed as `c31c721bc` (`docs: reconcile T31 iOS 27 acceptance status`).
