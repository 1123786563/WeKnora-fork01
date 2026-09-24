# T19 gateway canceled-request usage fix — independent re-review

Date: 2026-09-24. Read-only scoped review against the approved #107/T19 snapshot, `CONTEXT.md`, ADR-0004, `t19-gateway-usage-cancel-fix-plan.md`, and the prior two-phase review. No OCR or source/test edits.

## Checkpoint and verdict

The live handler source SHA-256 is `4d11fa87f66d1f115495cd80671d70206b1a938546737b6f9ad4cc91e6e9476e`; the test SHA-256 is `955f927ea4b4e88b66ed0c12ebe21d206a34cdf722489df2d512602956f39822`. Both match the checkpoint manifest. The task-local patch SHA-256 is `3fab5650aea78c0d3e8bd9cf26c72e30b4be047c83a2fda186dede28429a7164`; the combined checkpoint patch is `b63be3b3972eca68ba3149e784ace5d6b12bd8fc1661a01bf062bf8ccd3cb00c`. Both reverse-check cleanly against the live files. The task-local patch changes only the two owned handler files, atop the documented prior baseline.

- **Scoped Spec compliance: PASS.** The prior Medium canceled-request accounting finding is closed. After headers and inbound cancellation, the gateway resolves the charge start to `unknown` and appends a physical unknown-usage fact through the production SQLite-backed `CraftUsageService`/`CraftUsageStore`. The same activity is rejected before another upstream request.
- **Scoped code quality: PASS.** `recordCall` uses `context.WithoutCancel` with an explicit three-second timeout, retaining context values while bounding the append. A failed append emits a structured server error with Run/call/attempt IDs and does not promote the ambiguous journal state. The failure test checks deadline expiry, trace-value preservation, safe log fields, and no resend.
- **Full T19: NOT VERIFIED.** Production activity-ID provenance remains the separate open High finding from the high-fix design; this scoped accounting fix does not address it.

No new blocking finding in the two-file task-local change. The error response header is only supplemental; the structured server log provides the operator-visible signal when the caller has disconnected.

## Verification

I independently ran `go test ./internal/handler -run 'TestCraftGateway|TestCraftModelActivityKey' -count=1 -timeout=90s` (PASS) and the targeted inbound-cancellation/failure test selection under `go test -race` (PASS). `git diff --check` on the two owned files and reverse `git apply --check` on both patch artifacts passed. The new SQLite test reads back one durable `UsageStatusUnknown` fact with zero token totals and checks that retry produces no second provider request. This review did not run PostgreSQL or a live pinned OpenCode gateway path.
