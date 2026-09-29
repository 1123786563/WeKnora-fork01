# T31 DAG Status Record Repair Plan

**Goal:** Correct stale T31 status descriptions in the Issue #30 DAG and issue index so they match the independently verified local iOS 27 Release launch while preserving the remaining external acceptance blockers.

**Spec:** `docs/specs/2026-09-20-mobile-ai-office-design.md`; ADR `docs/adr/0005-weknora-native-mobile-client.md`; Issue #31 snapshot `docs/plans/issue30-sweep/issues/issue-31.md`; integrated T31 review `.superpowers/sdd/plan-t31-ios27/final-review-after-repairs.md`.

**Global Constraints:** This is a documentation-only repair. Keep GitHub Issue #31 open and `blocked-external` / partial. State that local iOS 27 scene/startup is fixed and login display is verified. Keep staging password/OIDC authentication, real Deployment capability, and Android physical-device acceptance pending. Do not alter application code or claim full Issue #31 acceptance.

**Review Focus:** The DAG node and issue index must agree; status remains externally blocked only for the listed acceptance gaps; every cited evidence path exists.

## Task DOC3: Reconcile stale T31 status records

- **Dependency:** Integrated T31 implementation and evidence review at `057b46e0759e74f41f64e0aa1babd75900800206`.
- **Role / files:** `mechanical_worker`; only `docs/plans/issue30-sweep/dag.md` and `docs/plans/issue30-sweep/issues/index.md`.
- **Consumes:** T31 R4 local Release startup and safe-area evidence cited in `docs/testing/mobile-runtime-login-device-acceptance.md`; integrated review report.
- **Produces:** Updated #31 DAG status/notes and issue-index handling decision. Say the local iOS 27 scene/startup issue is fixed and login is visible below the status bar; retain `blocked-external (partial)` because staging password/OIDC, real Deployment capability, and Android device evidence are pending.
- **Steps:** Inspect exact rows and cited acceptance evidence; edit only stale #31 descriptions; confirm cited evidence artifacts exist; run `git diff --check`; commit only the two owned documentation files plus task report.
- **Acceptance:** No stale claim that local iOS 27 black screen/scene failure remains; #31 remains open and externally blocked for the actual outstanding evidence; no unrelated file changes.
- **Failure handling:** If evidence does not support a status statement, preserve it as unknown and report the discrepancy; do not infer external acceptance.

## Review Focus

An independent reviewer checks Spec Compliance and Code Quality for the exact DOC3 change, confirms the evidence and blocker wording, and verifies no unrelated files were changed. Then rerun the integrated T31 review on the new HEAD before releasing #32.
