# T07/#147 final task validation

Date: 2026-09-24. Integration code HEAD: `b09919c3f` on `codex/issue-140-integration`. Task source: Issue #147 and the approved Career Spec. T03/#141 predecessor is verified and integrated.

## Acceptance and evidence

| Acceptance | Evidence |
| --- | --- |
| Education, experience, project, skill, quantified achievement and certificate have source and confirmation state | Real local HTTP and browser fixture generated seven pending proposals across six categories, each with source ID and exact evidence line; missing graduation year and two experience claims were flagged. Browser confirmed education and dismissed one experience; reload preserved source, one confirmed fact, five pending proposals and revisions. |
| Pending/dismissed proposals excluded from evaluation/material inputs | Career Office confirmed-only purpose-input tests at reviewed backend SHA cover pending and dismissed exclusion and final serialized model payload; UI never represented the upload receipt as confirmed facts. |
| Upload failure preserves profile and versions | Real HTTP and browser invalid-PDF upload produced a failed source while the prior confirmed fact and source revision remained. Reupload/manual entry controls remained available. |
| Irrelevant identity numbers hidden from model input | Reviewed backend purpose-scoped model-input tests check omitted identity keys/metadata and redaction in final serialized JSON; the synthetic identity line produced no confirmed proposal. |
| Owner/Tenant, replay and recovery | Real HTTP exact request replay returned the same source/receipt, changed intent returned 409, cross-Tenant source read returned 403 with no private payload. Focused Web tests and independent reviews covered same-name ambiguity, exact replay, 403 late-response fencing, terminal 400/409 recovery and known POST success with failed follow-up GET. |

## Gates

- Backend initial and R1–R5 code underwent independent Spec/quality review; final verdict PASS/PASS with no valid medium/high. Independent backend validator ran real SQLite catalog cleanup and race tests plus repository/service suites. Integrated Career/repository/service/router/database/handler/container suite passed at `5d8c57fea`.
- Web initial implementation had two high findings; R1 closed both but found two medium findings; R2 closed both. Final independent Web reviewer verdict PASS/PASS and frontend validator PASS at `b80bf7ec3`. Integrated `pnpm typecheck:web && pnpm test:web && pnpm build:web` passed at `01f4143ef`: 2319/2319 Web tests, 0 failed/cancelled; build warnings are recorded in `task-7-web-integration-validation.md`.
- The real DocReader image exposed `.txt` incompatibility. A scoped backend fix `120fda005` was independently validated/reviewed PASS/PASS and integrated as `b09919c3f`. On this integration SHA, `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/handler/... ./internal/container/...` and `go test -count=1 ./tools/architectureguard/...` passed. Fresh real DocReader `.txt` and `.docx` HTTP probes both passed; final browser `.txt` upload showed seven pending proposals and zero confirmed facts. See `task-7-real-docreader-validation.md`.
- The whole-repository architectureguard task was separately reviewed and integrated: 17 manifests, 644 routes, zero violations; `go test -count=1 ./...` passed at its isolated code SHA. Final #140 OCR outer gate has **not** yet run; this task-level verification is not a full parent Issue completion claim.

## Disclosed limits

No live PostgreSQL concurrency/migration run for this task. The generic FileService physical-write-before-catalog-registration crash window remains outside returned Career refs; earlier reviewer ruled it an infrastructure limit. The text-format reviewer left two low-priority suggestions: reject a nil DocumentReader only on non-text branch (production creates a nonnil disconnected client) and add direct evidence/status assertions in the text adapter test (separate extractor/Office tests and live HTTP/browser evidence cover them). These are recorded, not represented as closed. The real browser used disposable local accounts and synthetic content; no real user resume or production service was involved.
