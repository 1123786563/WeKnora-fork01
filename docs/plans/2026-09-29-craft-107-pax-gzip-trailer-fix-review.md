# Craft #107 F10 gzip trailer regression — independent review

## Scope and evidence

- Reviewed only the 15-line `TestArchiveRejectsCorruptGzipTrailerAfterTarEOF` delta in `internal/modules/craft/archive_test.go` on 2026-09-29. Preimage SHA-256: `346224d28b9cc51f0a144894d9bf97bef6465642da8925f253776afcf5241417`; current/postimage SHA-256: `aa721bd2a116da851221770ce61dbe90df8898ecbcc3d45b4dc8acef1e0276e5`; incremental patch SHA-256: `7fedb41f5092b1877d79b7b7c8af49836e6838a647d712e718f382f1ce0d1f10`. All three hashes matched the task package at review time.
- Read the approved Craft artifact spec (story 8, archive boundary and test decisions), scope design, `CONTEXT.md`, relevant ADR search (no archive-specific ADR), Task J/F10 review, assigned brief and repair plan. The brief and plan reside in the primary checkout; their paths are recorded in the implementer report. The unchanged `archive.go` SHA-256 was `56039016e4909259554c11c9cffcb8e244979ca22d51f44fa66ac47baf123522`.
- Independently ran only `go test ./internal/modules/craft -run '^TestArchiveRejectsCorruptGzipTrailerAfterTarEOF$' -count=1`: exit 0 (`ok .../internal/modules/craft 1.950s`). The implementer report records the broader archive selector, package tests, and `git diff --check`; I did not repeat those runs or invoke OCR.

## Findings

None in the reviewed delta.

## Assessment

- **Spec compliance: pass, scoped to this regression.** `buildTestTarGz` closes `tar.Writer` before compressing and closes `gzip.Writer` before returning; the new test first verifies that the original one-member fixture succeeds through `ExtractArchive`. It then copies the bytes and flips one bit at `len(data)-8`, the first byte of the gzip CRC32 trailer, leaving the tar body and its EOF blocks intact. `archive.go` drains the accounting stream after `tar.Reader.Next()` reaches EOF; the resulting gzip checksum error is wrapped with `ErrInvalidInput` and application-owned `after archive terminator` context. The test asserts both without matching Go's gzip error text.
- **Code quality: pass, scoped to this regression.** The fixture mutation is deterministic, avoids modifying the valid input, and distinguishes a malformed enclosing gzip stream from an invalid tar fixture through its precondition assertion. It exercises the observable `ExtractArchive` seam requested by the brief. No production code, shared state, or interfaces changed. The focused test passed; full branch/OCR coverage remains the controller's responsibility.
