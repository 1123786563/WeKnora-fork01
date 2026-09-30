# T08 B3: current preview authority at the integration constructor

> **For Codex:** Execute this scoped SDD task with RED→GREEN→REFACTOR, an exact uncommitted checkpoint, backend validation and independent Spec/quality review. It is one T08 acceptance slice; joined B5 and T14 live browser/network proof remain separate.

**Sources:** approved Craft Spec #107/#126; `2026-09-24-craft-107-t08-remaining-gap-map.md`; `2026-09-24-craft-107-t14-preview-integration-map.md`; `2026-09-24-craft-107-t14-preview-transfer-review.md`; T08 access B1 and actor Task3 reviews. Integration original BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`. Record current HEAD and owned-file preimage content/hashes. No commits.

## Global Constraints

The reviewed preview service already calls `craft.TaskAccessChecker` with `TaskPreview` at ticket issue, capability redemption and each file read. Wire the real T08 access service at the production constructor, using the authenticated actor/current membership, with no alternate preview authority. Keep `BrowserNavigationProtected=false` and the preview disabled until live browser and selected-bound-sandbox no-egress evidence is reviewed. Do not copy T14's older container/router files or weaken T08 wiring. Do not turn the access checker into an owner bypass or accept caller-selected sandbox IDs.

## Review Focus

Constructor dependency identity, scope/tenant/actor correctness, immediate revoke and membership removal, fail-closed disabled state, no preview bytes after denial, no authorization change to ordinary routes, and no accidental enablement of preview.

## Task 1 — production checker injection and focused revocation proof

**Depends on:** T14 transfer review PASS and T08 TaskAccessChecker interface. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/container.go` solely at `newCraftPreviewService` signature/config and a new focused `internal/container/craft_preview_access_wiring_test.go`; if necessary one new service/handler focused test file, with exact reason before editing. Existing `craft_preview.go`, `craft_access.go`, router files and other constructor areas are read-only. **Consumes:** `craft.TaskAccessChecker`, `CraftPreviewConfig.AccessChecker`, current T08 access service provider. **Produces:** production preview constructor bound to the real T08 checker while remaining disabled by browser/network gates.

1. RED test: use the real access service and persistent membership/grant store in the constructor test; show owner/granted Viewer allows `TaskPreview` check while ungranted Admin, cross-tenant, revoked Viewer and stale membership fail. A capability issued before revoke must fail on subsequent open/read with zero file bytes. If this requires a fake network/browser checker to exercise the service, construct that only in tests; production stays disabled.
2. Inject the checker into the existing constructor. Keep the preview origin/host restriction and all fail-closed gates; do not set `BrowserNavigationProtected=true` or install a pretend no-egress checker.
3. Run focused container/service/handler tests and `git diff --check`. Save pre/post HEAD, staged/unstaged and owned untracked complete content/hashes, task-local patch and report; bind test results to the checkpoint.

**Acceptance:** production assembly uses current T08 `TaskPreview` policy whenever preview can be enabled, and the current production state remains disabled pending T14 live proof. **Failure handling:** if the existing DI binding cannot supply the same access service, stop and report the specific interface mismatch; no permissive fallback.
