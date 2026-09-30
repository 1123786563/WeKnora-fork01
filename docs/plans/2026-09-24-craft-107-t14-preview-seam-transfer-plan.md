# T14 preview security seam: path-scoped transfer to integration

> **For Codex:** Transfer only the already reviewed T14 service/handler security seam, capture exact hashes/patch, run focused tests and obtain an independent integration review. This is a prerequisite to T08 B3 wiring and live T14 browser proof.

**Sources:** approved Spec #107/T14 and T08, T14 reviewed service/handler hardening on branch `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`, `2026-09-24-craft-107-t14-preview-integration-map.md`, integration original BASE `4bcad69baf033a1310b4dce1372c8153e66adc81`. Current integration HEAD must be recorded before transfer. No commits.

## Global Constraints

The four transferred integration paths are currently clean; verify that immediately before writing and preserve any later concurrent change. Do **not** cherry-pick T14 commits wholesale or copy T14 `container.go`, router files, render-boundary files or unrelated UI. Keep preview production gate closed: static `BrowserNavigationProtected=true` cannot replace live browser/network attestation. TaskPreview comes from current T08 `craft.TaskAccessChecker`; final production injection belongs to B3 after transfer. No main-checkout edits.

## Review Focus

Exact service/API parity with reviewed T14 commit; TaskPreview checks at issue/open/read, preview-host check, network checker fail-closed, no credential/main-site origin exposure, integration constructor compilation, no accidental loss of T08 route assembly.

## Task 1 — transfer reviewed service and handler delta

**Depends on:** T14 service seam reviewed; T08 TaskAccessChecker stable. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/application/service/craft_preview.go`, new `craft_preview_network_test.go`, `internal/handler/session/craft_preview.go`, `craft_preview_test.go`; transfer report/checkpoint. **Consumes:** exact T14 committed four-file contents and current integration base. **Produces:** same security seam in integration, still default-off until B3 wiring and browser proof.

1. Record HEAD, staged/unstaged/untracked state and SHA-256 for all four integration paths; record T14 commit/tree/file hashes. Apply only the four-file diff from integration HEAD to T14 commit, with manual adaptation only for any observed integration interface mismatch. No silent removal of integration behavior.
2. Run focused preview service/handler/network tests and compile container/router tests; if constructor fixtures fail because new dependencies are required, amend only the listed test files or request exact ownership of an additional fixture before editing. Keep nil/missing checkers fail closed.
3. Save complete uncommitted task-local patch including the new test file, before/after hashes, commands/results and report. Independent reviewer compares to T14 reviewed source and current integration behavior.

**Acceptance:** integration has the reviewed preview security seam without overwriting T08 assembly; no production enablement claim. **Failure handling:** if source contracts conflict, report the exact signature and leave preview closed; do not patch container/router opportunistically.
