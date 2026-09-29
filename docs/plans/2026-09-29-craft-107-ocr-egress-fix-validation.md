# CRAFT-107 OCR Egress Fix1 Validation

## Result

**DONE_WITH_CONCERNS** — the assigned egress acceptance behavior is covered by source inspection and passing focused and full package tests. The checkout was already broadly dirty before validation; this validation made no source or test edits. Only this assigned report was added.

## Scope and revision

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Revision: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`
- Scope: `internal/modules/craftegress/adapter.go` and `adapter_test.go`
- No authentication, authorization, or schema migration surface is involved in this local HTTP forwarding adapter change.

## Evidence

At validation start, the two scoped files had the same SHA-256 values recorded by the Fix1 task report and re-captured after testing:

| File | SHA-256 before | SHA-256 after |
| --- | --- | --- |
| `internal/modules/craftegress/adapter.go` | `2ce25edb584a0c33f26b651a1feba3546cbf6737a560c5afa902b8a04cdcedc9` | `2ce25edb584a0c33f26b651a1feba3546cbf6737a560c5afa902b8a04cdcedc9` |
| `internal/modules/craftegress/adapter_test.go` | `ff9ae222c3d1038848d1aeb7a7f05b13fca90248b68cab576e1d3c2710dd7de0` | `ff9ae222c3d1038848d1aeb7a7f05b13fca90248b68cab576e1d3c2710dd7de0` |

Validation commands and results:

| Command | Result |
| --- | --- |
| `go test ./internal/modules/craftegress -run '^(TestAdapter(TornBody|LogsTornBodyResolutionFailure|Proxy5xxJSONKeepsAttemptParked|Complete502EnvelopeKeepsAttemptParked)|TestGatewayOutcomeIsDefinitive)$' -count=1` | PASS |
| `go test ./internal/modules/craftegress -count=1` | PASS |
| `git diff --check -- internal/modules/craftegress/adapter.go internal/modules/craftegress/adapter_test.go` | PASS, no output |

Source and tests establish:

- A truncated or oversized response body is surfaced as HTTP 502 (`egress response incomplete`).
- A failed journal `Resolve` in this path is logged with attempt ID, definitive flag, and gateway status; the regression test induces the failure and asserts those fields while checking HTTP 502.
- Complete response policy keeps all 5xx outcomes parked, including 502/503/504 JSON lookalikes. An explicit unresolved activity remains parked. A bare 409 is definitive for a complete response.
- Torn-body fallback is status-sensitive: 409 and every 5xx remain parked; tested 200 is definitive and a retry receives a new attempt ID. This reflects the intended fallback when the body cannot be trusted.
- `git status` after validation still showed the pre-existing dirty `ocr_regression_test.go`; no validation command wrote it. The worktree also had extensive unrelated pre-existing modifications and untracked plan artifacts at start. No other tracked source/test path was edited by validation.

## Acceptance gaps and risks

No scoped acceptance gap found. Authentication, authorization, migration, and cancellation behavior are not applicable to these changes. Repository-wide state was already dirty, so the conclusion that unrelated files were unchanged is limited to observed pre/post scoped status and the read-only test commands; this report is the only intentional filesystem addition.
