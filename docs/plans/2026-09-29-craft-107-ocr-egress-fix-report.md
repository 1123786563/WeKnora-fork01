# OCR Round 3 Egress Fix — Task Report

## Scope

Task 1 from `docs/superpowers/plans/2026-09-29-craft-107-ocr-egress-fix.md`: make torn-response journal resolution errors observable, simplify the 502 status fallback, and preserve existing adapter behavior. No commit or staging performed.

## Changes

- In `internal/modules/craftegress/adapter.go`, the torn/incomplete response path now logs journal `Resolve` failures with `craft_attempt_id`, `definitive`, and `gateway_status`, matching the full-response path. It still returns HTTP 502 for the incomplete body.
- `responseBodyReadOutcomeIsDefinitive` now explicitly excludes 409 and every 5xx by checking the 5xx boundary; the redundant explicit 502 comparison was removed and the comment documents the policy.
- Added `TestAdapterLogsTornBodyResolutionFailure`, which closes the already-minted journal file at the mock gateway to force the subsequent durable resolve to fail. It asserts the existing HTTP 502 result and all required log fields. The test first failed because the log was absent, then passed after the fix.

## Verification

| Command | Result |
| --- | --- |
| `go test ./internal/modules/craftegress -run '^TestAdapterLogsTornBodyResolutionFailure$' -count=1` (before fix) | Expected RED: failed because the log did not contain the attempt ID. |
| `gofmt -w internal/modules/craftegress/adapter.go internal/modules/craftegress/adapter_test.go` | Completed. |
| `go test ./internal/modules/craftegress -run 'TestAdapterTornBody|TestGatewayOutcome' -count=1` | PASS. |
| `go test ./internal/modules/craftegress -count=1` | PASS. |
| `git diff --check -- internal/modules/craftegress/adapter.go internal/modules/craftegress/adapter_test.go` | PASS (no output). |

## Evidence hashes

SHA-256 of the scoped source and test files after formatting and verification:

- `internal/modules/craftegress/adapter.go`: `2ce25edb584a0c33f26b651a1feba3546cbf6737a560c5afa902b8a04cdcedc9`
- `internal/modules/craftegress/adapter_test.go`: `ff9ae222c3d1038848d1aeb7a7f05b13fca90248b68cab576e1d3c2710dd7de0`

The shared worktree contained pre-existing edits in these files and in `ocr_regression_test.go` before this task. Those edits were preserved; this task did not modify `ocr_regression_test.go`.

## Remaining risks

No known task-specific risk. Test-induced journal failure is deterministic and exercises the public HTTP handler. Broader OCR and integration review remain with the parent workflow.
