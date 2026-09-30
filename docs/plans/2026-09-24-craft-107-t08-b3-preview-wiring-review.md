# T08 B3 preview constructor wiring — independent review

Date: 2026-09-24. Scope: Task 1 in `2026-09-24-craft-107-t08-b3-preview-wiring-plan.md`, at checkpoint HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. I reviewed its report, checkpoint and exact task-local patch against the approved Craft web artifact Spec, `CONTEXT.md` private Task/Viewer/Admin model, ADR 0004 (Task is Session), ADR 0009 (private content boundary), the T08 gap map, and the reviewed T14 preview transfer. No OCR, browser probe, or source/test edit was performed.

## Verdict

- **Scoped Spec compliance: PASS by code inspection; execution evidence incomplete.** The production `newCraftPreviewService` now receives the same persistent `*service.CraftAccessService` registered in the container and passes it as `AccessChecker`. The transferred preview service calls `TaskPreview` at issue, ticket redemption, and each capability lookup before opening file bytes. Its stored scope carries tenant, user and Session/Task identity. The access service re-reads current session ownership, active tenant membership and the membership-bound Task grant. This matches the scoped B3 authorization intent.
- **Scoped code quality: PASS by inspection, with one verification limitation below.** The patch does not set `BrowserNavigationProtected`, install a production `NetworkChecker`, or change T14 service/handler or router files. Thus `Issue` and `Open` still fail closed in production, even if valid origins make `Enabled()` true. The test-only no-egress checker and browser gate are confined to a handler fixture. No actionable critical, high or medium defect was found in the task-local delta.
- **Full T08/T14 acceptance: NOT VERIFIED.** Production preview intentionally remains disabled pending live browser and selected-bound-sandbox no-egress proof. The joined authenticated route/assembly journey belongs to later B5/T14 verification.

## Evidence and test limitation

- Exact patch SHA-256 `6ec5a06367e38fa864720ba8bba7601aa3fa6842cfd46ec246a600d015ea8b69`; all three current owned-file hashes match the checkpoint post-image. The only production edit is the `AccessChecker: access` assignment and constructor parameter in `internal/container/container.go:2547-2557`.
- `internal/container/craft_preview_access_wiring_test.go` constructs the real persistent access service and checks Owner/Viewer allow, ungranted Admin/cross-tenant deny, revocation and stale membership deny. It also asserts that the production constructor refuses issuance while the browser gate is false.
- `internal/handler/session/craft_preview_persistent_access_test.go` uses a persistent access service at the Gin preview route: a Viewer reads the pinned bytes before revoke; the same previously redeemed capability returns 404 and an empty body after revoke. This proves the tested file-read revocation path; it does not exercise production constructor enablement or ticket revocation before redemption.
- The implementer reports focused service and handler tests plus `git diff --check` passed. **The container test did not pass or run to completion:** concurrent R3 changes in `internal/container/craft_runtime.go` and its tests block package compilation (`stageWorkspaceInputs` arity and missing `sort`). Therefore the constructor test is presently inspection evidence only. Re-run that exact container test after the R3 compile mismatch is resolved and bind the result to the unchanged checkpoint; if the checkpoint changes, review the new delta.

## Findings

No actionable finding in the scoped Task 1 patch. The unexecuted container test is a **verification blocker**, not a demonstrated B3 code defect. The broader T08 gap map also calls for ticket revoke-before-redemption, membership removal between stages and a joined production-route journey; those remain follow-on acceptance evidence, not claims established by this checkpoint.
