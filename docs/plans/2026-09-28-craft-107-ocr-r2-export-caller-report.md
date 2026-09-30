# Task K Report — Export Consent Caller Binding

## Scope and evidence

- Task: `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md` Task K.
- Approved behavior: `docs/specs/2026-09-23-craft-web-artifact-spec.md`, requirement 24 (owner consent for derived data before source export).
- F23 evidence: `docs/plans/craft-107-ocr-final-1.md`, finding at `craft_export_consent.go:224-229`: `ExportConsentView` relied on `TaskRead` scope membership without binding the requested scope to the authenticated caller, unlike `DecideExport` and `ExportBundle`.
- Scope implemented: after `RequireTaskAccess(TaskRead)` succeeds, compare authenticated caller tenant/user with the requested scope; on mismatch emit the existing `craft.export_denied:caller_identity` denial audit attributed to the authenticated caller and return `craft.ErrForbidden` before projecting/reading the consent view. Requested Task/version remain the audit target. `DecideExport` uses the same caller-aware attribution for its identity mismatch denial.

## RED → GREEN evidence

- Added a direct service regression to `TestCraftT13Journey`: the task access checker authorizes the requested owner scope while context contains a different authenticated tenant/user; assert `ErrForbidden` and a caller-identity denial audit row attributed to caller tenant `7` / user `u-other`, with the requested task/version retained as target.
- RED command: `go test ./internal/application/service -run '^TestCraftT13Journey$' -count=1`
- RED result: failed at `craft_export_consent_t13_test.go:149`; expected `craft.ErrForbidden`, got `nil`.
- Caller-attribution re-review command: `go test ./internal/application/service ./internal/handler/session -run 'TestCraftT13(Journey|ExportConsentHTTPJourney)$' -count=1`
- Caller-attribution re-review result: passed; service and handler/session packages both reported `ok`.
- Formatting check after the attribution update: `git diff --check` — passed (exit 0).

## Checkpoint

- HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` (no commit created).
- Changed implementation: `internal/application/service/craft_export_consent.go`
  - SHA-256: `b8020dc8037e24d7fd0838c8732ff1537007a1edd6dbf515ff52e7bbb5bee304`
- Changed focused test: `internal/application/service/craft_export_consent_t13_test.go`
  - SHA-256: `6645777f9bef92fbafac53f8abe063e7ed6a6aada01fa8151315aa8ebc295b6a`
- Combined implementation/test diff SHA-256: `d6d397c39f7bced5bd811fc5d54ba03dc3dab4243530e83b8bc4e136cb772203`
- Diff summary: 2 files changed, 31 insertions(+), 6 deletions(-).
- At inspection, `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md` was already untracked in the Worktree; it is outside this task's ownership and was left untouched.

## Remaining review

- No known behavior gap in Task K. Independent review and full-scope OCR remain with the integration owner.
