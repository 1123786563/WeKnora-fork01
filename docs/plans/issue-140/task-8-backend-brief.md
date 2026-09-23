# T08/#146 JD paste backend Task Brief

**Source and status:** Issue snapshot `docs/plans/issue-140/issues/issue-146.md`, approved Career Spec, ADR-0015–0018, `docs/plans/issue-140/task-8-architecture.md`, parent implementation plan Task 8. Prepare only; dispatch after T07 is verified and integrated. **Owner:** backend_implementer in a new isolated worktree. **Validator:** backend_validator; independent reviewer assigned by root. Local commits authorized; no push/merge/deploy.

## Scope and owned interface

Add a Career-owned `import_jd` command/read seam, additive handler/routes, persistence and migrations, with focused tests. Own `internal/modules/career/` opportunity files and tests, narrow `handler.go`/`routes_career.go` edits, and new numbered SQLite/PostgreSQL migrations chosen after checking the then-current integrated head. Do not change the profile fact revision, source upload, resource catalog, chat host, TypeScript, or Web files in this backend task. Freeze wire examples in tests and report before the Web subtask begins.

Input carries a stable `requestId`, exact raw JD text, optional user-supplied source label/reference. Server derives owner/Tenant, IDs and acquisition time. Store the raw text separately from candidate extraction; represent missing conditions explicitly as unknown. Preserve exact raw text bytes in an immutable snapshot and keep source observation and acquisition timestamp. Same owner/request/canonical intent replays the identical receipt and IDs; changed intent conflicts. An `opportunity_imported` receipt includes opportunity, observation and snapshot IDs plus status. Scoped evidence read by opportunity and snapshot ID returns exact raw text, source/time, extracted known/unknown fields and extraction state. Reopening an old snapshot must not switch to a newer observation. A failed extractor retains evidence with a review-needed state, never fabricates hard requirements.

Untrusted JD content remains inert throughout import: no tool execution, grant, URL fetch, system instruction interpolation, or external side effect can derive authority from its text. Text is a candidate input only. The import does not advance `CareerView.revision` or create a profile change; do not send a dummy expected profile revision through `Office.Act`. Use a typed command seam or separate endpoint. Preserve T03/T07 receipt behavior; unknown result recovery uses request ID lookup.

## RED/GREEN and handoff

Write public Office/HTTP tests before implementation for exact raw text, explicit unknowns, stable replay/conflict, owner/Tenant denial, snapshot immutability, failed extraction preservation, and malicious instruction text doing no privileged work. Run focused RED, implement GREEN, refactor, then run `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...` and `git diff --check`. Record SQLite up/down/up and restart evidence; run PostgreSQL if available, otherwise disclose its absence. Report exact endpoint/wire payloads, commit SHAs and verification. Independent backend validation/review precede integration and Web dispatch.

## Shared-file preflight

At dispatch, re-read integration Git/DAG and verify T07 Web is complete, current migrations, router route count and architectureguard baseline. This task adds Career routes, so any architectureguard route-count lock change must be explicitly planned and independently reviewed at integration rather than silently changing the historical count. Shared `CareerReceipt` TS contract belongs to the following Web subtask. The backend task owns the Go contract and must not infer that a JD has authority to call tools.
