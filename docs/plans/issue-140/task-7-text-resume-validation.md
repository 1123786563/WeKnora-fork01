# T07 text resume compatibility validation

**Status: PASS**

## Scope and revision

Validated only the assigned T07 brief and acceptance criteria in `docs/plans/2026-09-24-issue-140-t07-text-resume-compatibility.md` at code SHA `120fda0052476c544088ed9a409c703dbab49940` (`fix(career): parse validated text resumes directly`). `git status --short` was empty before validation and remains empty; no source or test files were changed.

## Acceptance evidence

- Valid UTF-8 `.txt` input is used directly after storage readback and does not call `DocumentReader`. The focused test verifies exact text, seven extracted fields across six categories, missing `education.graduation_year`, multiple-experience review flag, and stored resource binding.
- Invalid UTF-8 and a mismatched declared MIME are rejected before storage (`files.saves == 0`). Unsupported extension is rejected in the adapter before content validation/storage.
- Non-`.txt` parsing remains on `DocumentReader`: the PDF adapter test returns the injected parser error, proving the reader call/error path remains active. The same test also verifies PDF signature rejection.
- Upload API, authorization, migrations, and storage/replay transaction logic are not changed by this commit. The shared source persistence and cleanup path remains downstream of the format branch. Cancellation has no new behavior in the changed code; context continues to be passed to storage and DocumentReader calls.

## Commands and results

- `go test -count=1 ./internal/modules/career -run '^(TestUploadAdapterParsesValidatedTextWithoutDocumentReader|TestUploadAdapterRejectsInvalidTextBeforeStorage|TestUploadAdapterValidatesStoresAndParsesDurableCareerSource|TestUploadAdapterRejectsMismatchedAndFailedParserInputs)$' -v` — PASS (all four tests).
- `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/handler/... ./internal/container/` — PASS, recorded by the implementation report at this same SHA; container linker emitted a duplicate `-lc++` warning.
- `git diff --check 120fda0052476c544088ed9a409c703dbab49940^ 120fda0052476c544088ed9a409c703dbab49940` — PASS, exit 0.
- `git rev-parse HEAD` — `120fda0052476c544088ed9a409c703dbab49940`.
- `git status --short` — empty.

## Gaps and risks

No acceptance gap found in the assigned code/test scope. Real DocReader container HTTP integration acceptance is explicitly outstanding in the implementation report and is outside this adapter-level validation. The implementation still checks that `reader != nil` in its initial dependency guard even for `.txt`; this does not affect the tested production wiring, but means direct `.txt` parsing is not available when the reader dependency is absent.
