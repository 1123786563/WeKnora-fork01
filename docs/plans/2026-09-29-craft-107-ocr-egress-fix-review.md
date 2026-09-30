# OCR Round 3 Egress Fix1 — Independent Review

## Scope and evidence

Read-only review of the Fix1 portion of `internal/modules/craftegress/adapter.go` and `TestAdapterLogsTornBodyResolutionFailure` in `adapter_test.go`. The shared worktree also contains earlier egress changes and unrelated edits; those are outside this task delta. Sources: approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`, `CONTEXT.md`, `docs/plans/2026-09-24-craft-107-t19-activity-id-protocol.md`, `docs/plans/craft-107-ocr-source-round3-partial.md`, and `docs/superpowers/plans/2026-09-29-craft-107-ocr-egress-fix.md`. No applicable ADR changes the scoped egress retry rule.

Reviewed file SHA-256: `adapter.go` `2ce25edb584a0c33f26b651a1feba3546cbf6737a560c5afa902b8a04cdcedc9`; `adapter_test.go` `ff9ae222c3d1038848d1aeb7a7f05b13fca90248b68cab576e1d3c2710dd7de0`.

## Findings

No blocking findings in the scoped Fix1 delta.

The focused test proves the missing error log and externally visible 502 for a torn HTTP 200 response, but does not itself assert the subsequent same-fingerprint retry after the induced journal failure. This is a **low-severity test coverage limitation**, not a demonstrated defect: `Resolve` retains the parked in-memory state when append fails (`journal.go`, `Resolve`/`appendLocked`), and the existing status-sensitive test covers successful resolve retry identities for torn 200, 409, 502, 503, and 504. Smallest optional strengthening: use a failing append seam that can be restored, then assert a retry reuses the parked ID after failed definitive `Resolve`. The current test deliberately closes the journal file, so a direct retry through that instance would first need a restorable failure seam.

## Spec compliance verdict: PASS for Fix1 scope

- `adapter.go:227-235` uses the status-only classification for a torn response, logs a `Resolve` error with `craft_attempt_id`, `definitive`, and `gateway_status`, and still returns HTTP 502. Logging does not change the journal decision or send path.
- `adapter.go:343-349` excludes 409 and every 5xx from definitive outcomes; the redundant explicit 502 exclusion is gone. A torn 200 remains definitive, consistent with the pre-Fix1 policy and the assigned brief.
- The protocol's unknown-outcome identity rule is preserved: a failed durable resolve leaves the prior parked identity in the journal, while successful 409/5xx torn resolves remain unresolved. The focused test checks the requested error observability, and the existing status-sensitive test checks retry identity for these status classes.

## Code quality verdict: PASS for Fix1 scope

- The new error handling mirrors the complete-response branch and uses the request context logger. It records opaque identity and decision metadata without logging a credential or request body.
- The test triggers an actual journal write failure after mint-before-send and drives the public HTTP handler. Its asserted log fields and response status would fail if the new branch were removed. It uses one request and does not introduce a production concurrency path.
- Independent verification: `go test -race ./internal/modules/craftegress -run 'TestAdapterLogsTornBodyResolutionFailure|TestAdapterTornBodyResolutionIsStatusSensitive' -count=1` passed. The implementer report records passing package tests and diff check for the same file hashes; this review did not rerun the entire package.

This verdict covers the named Fix1 delta only. It does not certify the larger shared-worktree delivery or the outer OCR gate.
