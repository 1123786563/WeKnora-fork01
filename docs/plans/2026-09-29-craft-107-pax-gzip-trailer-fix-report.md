# Craft F10 gzip trailer regression report

## Scope and preflight

- Task: Add the independent review regression for a corrupt gzip CRC trailer after valid tar EOF, following `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-brief.md` and `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-plan.md` in the integration Worktree.
- Spec: `docs/specs/2026-09-23-craft-web-artifact-spec.md`; Task J in `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/ocr-pax-budget/WeKnora-fork01`.
- HEAD before and after: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commit or staging performed.
- At preflight, `archive_test.go` was `346224d28b9cc51f0a144894d9bf97bef6465642da8925f253776afcf5241417`. `archive.go` was already modified in the lane and had SHA-256 `56039016e4909259554c11c9cffcb8e244979ca22d51f44fa66ac47baf123522`; it contains the bounded drain after tar EOF. I left it unchanged.
- No test process was running at preflight. Existing unrelated lane changes and report/plan artifacts were preserved.

## Change and evidence

Added `TestArchiveRejectsCorruptGzipTrailerAfterTarEOF` to `internal/modules/craft/archive_test.go`. It first confirms the original tar.gz with one member extracts successfully, then flips one bit in the first CRC byte of the closed gzip stream's final eight-byte trailer. The corrupt input must wrap `ErrInvalidInput` and report the stable application context `after archive terminator`.

The regression passed immediately because the lane already drains the bounded decompressed stream after `tar.Reader.Next()` returns EOF. That drain also validates the gzip trailer and preserves the byte/ratio budget for trailing data. This task made no production-code edit.

Exact verification commands and results:

```text
gofmt -w internal/modules/craft/archive_test.go
  exit 0

go test ./internal/modules/craft -run '^TestArchiveRejectsCorruptGzipTrailerAfterTarEOF$' -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 1.939s

go test ./internal/modules/craft -run 'TestArchive' -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 6.346s

go test ./internal/modules/craft -count=1
  exit 0; ok github.com/Tencent/WeKnora/internal/modules/craft 7.576s

git diff --check
  exit 0; no whitespace errors
```

## Incremental review package

- Exact test preimage: `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-task1-preimage.archive_test.go`, SHA-256 `346224d28b9cc51f0a144894d9bf97bef6465642da8925f253776afcf5241417`.
- Exact test postimage: `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-task1-postimage.archive_test.go`, SHA-256 `aa721bd2a116da851221770ce61dbe90df8898ecbcc3d45b4dc8acef1e0276e5`.
- Incremental patch: `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-task1.patch` (zero-context diff), SHA-256 `7fedb41f5092b1877d79b7b7c8af49836e6838a647d712e718f382f1ce0d1f10`.
- Final `internal/modules/craft/archive_test.go` SHA-256: `aa721bd2a116da851221770ce61dbe90df8898ecbcc3d45b4dc8acef1e0276e5`.
- Final `internal/modules/craft/archive.go` SHA-256 remains `56039016e4909259554c11c9cffcb8e244979ca22d51f44fa66ac47baf123522`.

## Limitations

The test asserts the application-owned error context and `ErrInvalidInput`; it does not depend on exact Go standard-library gzip wording. No race run was requested by this focused test plan.
