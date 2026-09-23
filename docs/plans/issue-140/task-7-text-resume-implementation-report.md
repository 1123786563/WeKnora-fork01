# T07 text resume compatibility implementation report

## Outcome

Implemented local extraction for validated UTF-8 `.txt` resume bytes after storage readback. `.pdf`, `.docx`, and `.doc` continue through the existing `DocumentReader` path and retain its parse error behavior. The existing source binding, stored resource, cleanup, and replay paths remain shared.

The adapter test verifies exact text preservation, six distinct extracted categories (including two experience claims), the missing graduation year, the multiple-experience review flag, and that `DocumentReader` is not called. Invalid UTF-8 and mismatched declared MIME are rejected before storage.

## Changed files

- `internal/modules/career/upload.go`
- `internal/modules/career/upload_test.go`

## TDD and verification evidence

- RED: `go test ./internal/modules/career -run '^TestUploadAdapterParsesValidatedTextWithoutDocumentReader$' -count=1` failed before the implementation with `parse resume: unsupported file type: txt`.
- GREEN / targeted suites: `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/handler/... ./internal/container/` passed. Packages reported `ok`; container linker emitted a duplicate `-lc++` warning.
- Diff validation: `git diff --check` passed.
- Commit: `120fda0052476c544088ed9a409c703dbab49940` (`fix(career): parse validated text resumes directly`), based on `72c34d238f6e2a6f6f3402cbf51e332aade504cf`.

## Scope and remaining validation

No Web, router, migration, or catalog files were changed. Real DocReader container HTTP acceptance, independent backend validation, and review are owned by the parent task and remain outstanding here.
