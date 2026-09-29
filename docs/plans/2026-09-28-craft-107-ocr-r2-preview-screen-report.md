# OCR Round 2 F09: Previewable Kind HTML Screening

## Scope and basis

- Finding: F09 in `docs/plans/craft-107-ocr-final-1.md` (HTML-family screening and `assets/craft-web.js` pin were conditional on `KindWeb`).
- Approved behavior reference: `docs/specs/2026-09-23-craft-web-artifact-spec.md`; its isolated preview boundary is shared by the previewable artifact kinds. Existing kind-specific staged manifest validation remains authoritative for document, spreadsheet, and slides.
- Worktree: `/Users/wuyongjun/.codex/worktrees/ocr-preview-kind-screen/WeKnora-fork01`.
- Base: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`.
- No commits created.

## Change

`stageAndUpload` now performs HTML-family denylist screening and verifies the pinned `assets/craft-web.js` digest for every `craft.PreviewableKind`. It still runs the existing per-kind manifest gate afterward; no web-only build, citation, or manifest behavior was added to the other kinds.

Focused service tests cover hostile external-navigation HTML rejection, pinned-asset mismatch rejection, and safe HTML acceptance alongside each applicable manifest contract for web, document, spreadsheet, and slides.

## TDD and verification evidence

- RED: `go test ./internal/application/service -run 'TestCraftArtifactStage(ScreensHTML|PinsRuntimeAsset)' -count=1` failed on the original implementation: document and slides accepted the hostile HTML or mismatched asset. The first fixture attempt exposed an incorrect slides preview path; corrected to `preview.json` before GREEN.
- GREEN/focused: `go test ./internal/application/service -run 'TestCraftArtifact(Stage(ScreensHTML|PinsRuntimeAsset)|Collect)' -count=1` — passed (`ok`, 1.627s).
- Static package check: `go vet ./internal/application/service` — passed (same command sequence completed successfully).
- Whitespace check: `git diff --check` — passed.
- Broader package attempt: `go test ./internal/application/service -count=1` did not complete cleanly. It emitted unrelated existing runtime/service failures, including `TestDurableRunClassificationLookupFailureAndManifestDisagreementFailClosed/manifest_without_registration`, `TestAgentRunWorkerTickSkipsPaseo`, `TestAgentRunWorkerRemoteDispatchUsesDurableIntent`, and `TestDecisionSurvivesKillAfterSavedDecision` (missing provider barrier after 30s). The test process was interrupted at 151s after continuing to run and emit failures. This does not affect the focused test pass; full package status remains unverified.

## Changed files and SHA-256

- `internal/application/service/craft_artifacts.go` — `ecc275a2d7013f2c5c82a6b131d54cbdffe62ecb870bdeae615d646cb029a890`
- `internal/application/service/craft_artifacts_test.go` — `8c9965ff143844dcb983545100d92d61c74187a341d47b18534ffd600046592b`

The report is the only added file. No migrations or other production files changed.
