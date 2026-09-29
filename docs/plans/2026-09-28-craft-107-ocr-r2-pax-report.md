# Craft OCR R2 Task J: PAX stream budget report

## Scope and sources

- Task: Task J, “Bound decompressed pax metadata in archive extraction,” from `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md`. That plan was not present in this task Worktree; it was read from the shared integration Worktree at `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01/docs/plans/2026-09-28-craft-107-ocr-r2-plan.md`.
- Finding: F10 in `docs/plans/craft-107-ocr-final-1.md`.
- Spec: `docs/specs/2026-09-23-craft-web-artifact-spec.md`, Implementation Decisions: uploaded archives are bounded inputs and extraction enforces cumulative expanded size and resource limits.
- Owned source changes: `internal/modules/craft/archive.go` and `internal/modules/craft/archive_test.go` only. The report is the assigned report file. No shared integration files were changed.

## Implementation

`extractTarArchive` now wraps its decompressed input in a bounded accounting reader before constructing `tar.Reader`. This is required because Go's `tar.Reader.Next()` parses PAX metadata internally. The reader counts tar headers, PAX data, file data, padding and terminator blocks before `tar.Reader` can consume them. It stops reads at the lower of `MaxArchiveExpandedBytes` and the overflow-safe compression-ratio ceiling, probing one byte only to distinguish an exact-limit EOF from more stream data.

The shared archive budget now uses checked addition and overflow-safe ratio comparison. Tar member buffering retains its per-file cap but does not count bytes a second time; ZIP members continue to use the same shared expanded-byte and ratio checks. Tar/gzip support, path checks, member/entry limits, nested archive checks and typed invalid-input errors remain in place. The duration comment now accurately describes its service-layer role.

Regression coverage constructs PAX-only compressed streams incrementally, without holding the expanded 100 MiB test archive in memory. It covers a stream beyond the expanded-byte ceiling while remaining below the input cap and with the byte ceiling active, a PAX stream over the compression ratio, and an in-bound PAX entry followed by a regular member.

## RED → GREEN and verification evidence

RED command:

```text
go test ./internal/modules/craft -run 'TestArchive(RejectsPaxMetadataOverExpandedByteCap|AcceptsPaxMetadataWithinExpandedByteCap)$' -count=1
```

Before the implementation, the over-limit case failed with `craft invalid input: archive holds no usable members`, demonstrating that metadata was not charged before `tar.Reader` consumed it. The in-bound case was separately adjusted to use data below the compression-ratio threshold, because the complete-stream accounting intentionally includes PAX bytes and tar framing.

Final commands and results:

```text
gofmt -w internal/modules/craft/archive.go internal/modules/craft/archive_test.go
  exit 0

go test ./internal/modules/craft -run 'TestArchive' -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 2.646s

go test ./internal/modules/craft -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 1.071s

git diff --check
  exit 0; no whitespace errors
```

## Checkpoint

- Checkpoint ID: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159+ae720a8d37c4f3e01e99f121b1ea32b6044fa8cbd37d68c563a9fa9a933011f1` (HEAD plus SHA-256 of the tracked code diff for the two owned source files).
- HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`.
- SHA-256 `internal/modules/craft/archive.go`: `05e51e8fd4e831738607f60ac74847ffefaf193512df0ce71b30891e369b112a`.
- SHA-256 `internal/modules/craft/archive_test.go`: `3144bdfcc1c64f603d288c799143f263c36608857771bcd5b0d062075dbbc223`.
- No commit created. No other tracked or untracked source changes were present at checkpoint capture.

## Remaining risks

- The Go standard library caps each individual PAX special file at 1 MiB; the over-limit regression therefore uses repeated legal PAX records to exceed the cumulative stream ceiling.
- The focused and package tests passed; `-race` was not run because the task’s package test harness does not require it and this extraction path is synchronous.
- The report file is new and therefore not included in the source-diff hash component of the checkpoint ID.

## F10 Task J review repair: trailing gzip stream drain

The independent review found that `tar.Reader.Next()` stops at the two tar zero blocks, so bytes after the tar terminator were not charged and gzip CRC/trailer errors could go unchecked. `extractTarArchive` now drains the bounded `archiveStreamReader` to EOF before accepting members. This includes any later gzip members, charges trailing decompressed bytes with the same overflow-safe budget, and wraps limit/decompression/checksum errors as `ErrInvalidInput`.

Added `TestArchiveRejectsOverLimitGzipExpansionAfterTarTerminator`. It constructs one valid tar member with a valid terminator followed by noisy gzip-expanded trailing data that crosses the 100 MiB expanded cap while staying below the compressed input cap and keeping the compression-ratio ceiling above the expanded ceiling.

RED evidence (before the drain):

```text
go test ./internal/modules/craft -run '^TestArchiveRejectsOverLimitGzipExpansionAfterTarTerminator$' -count=1
  FAIL as expected: TestArchiveRejectsOverLimitGzipExpansionAfterTarTerminator got nil error instead of ErrInvalidInput
```

GREEN and package verification:

```text
gofmt -w internal/modules/craft/archive.go internal/modules/craft/archive_test.go
  exit 0

go test ./internal/modules/craft -run '^TestArchiveRejectsOverLimitGzipExpansionAfterTarTerminator$' -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 3.473s

go test ./internal/modules/craft -run 'TestArchive' -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 5.343s

go test ./internal/modules/craft -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 4.316s

git diff --check
  exit 0; no whitespace errors
```

Updated checkpoint:

- HEAD remains `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit created.
- Tracked owned-source diff SHA-256: `e63ede6321175fe0c4681f55f89661fe94c7dc5ec6309c15ac6ac773ce12d2b2`.
- Checkpoint ID: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159+e63ede6321175fe0c4681f55f89661fe94c7dc5ec6309c15ac6ac773ce12d2b2`.
- Current file SHA-256 values: `archive.go` `56039016e4909259554c11c9cffcb8e244979ca22d51f44fa66ac47baf123522`; `archive_test.go` `346224d28b9cc51f0a144894d9bf97bef6465642da8925f253776afcf5241417`.
- Other pre-existing task changes remain in this Worktree; this repair touched only the two owned source files and this report.
