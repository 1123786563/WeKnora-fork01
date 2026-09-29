# Craft PAX fix independent validation

## Scope

Validate only the assigned Task J PAX archive budget checkpoint. No business or test source was edited by this validator.

- Validation workspace: `/Users/wuyongjun/.codex/worktrees/ocr-pax-budget/WeKnora-fork01`
- Checkpoint HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`
- Source/test paths: `internal/modules/craft/archive.go`, `internal/modules/craft/archive_test.go`
- Existing implementation report: `docs/plans/2026-09-28-craft-107-ocr-r2-pax-report.md`
- Integration report workspace HEAD: `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`

## Checkpoint and report evidence

Current SHA-256 values:

- `internal/modules/craft/archive.go`: `56039016e4909259554c11c9cffcb8e244979ca22d51f44fa66ac47baf123522`
- `internal/modules/craft/archive_test.go`: `346224d28b9cc51f0a144894d9bf97bef6465642da8925f253776afcf5241417`
- `docs/plans/2026-09-28-craft-107-ocr-r2-pax-report.md`: `408bccce8c7f50fa3490506d8225200c42c444c51945047f817bcc5921fb2915`
- Tracked diff of the two owned source/test files (`git diff ... | shasum -a 256`): `e63ede6321175fe0c4681f55f89661fe94c7dc5ec6309c15ac6ac773ce12d2b2`

These source, test, and diff hashes match the report's updated checkpoint block, including checkpoint ID `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159+e63ede6321175fe0c4681f55f89661fe94c7dc5ec6309c15ac6ac773ce12d2b2`. The report's earlier checkpoint block records the pre trailing-stream-drain revision and older hashes; its later section explicitly updates them. The report states previous `TestArchive` and full package test passes, plus the trailing gzip regression's RED/GREEN evidence. This validation independently reran the assigned focused command below.

## Resource check

Before testing, `ps -axo pid,etime,command | rg 'go test|go build|golangci|craft'` found no pre-existing Go test/build/lint process (only the inspection shell and `rg`). `df -h .` reported 222 GiB available.

## Commands and results

Executed from `/Users/wuyongjun/.codex/worktrees/ocr-pax-budget/WeKnora-fork01`:

```text
go test ./internal/modules/craft -run 'TestArchive' -count=1
exit 0
ok github.com/Tencent/WeKnora/internal/modules/craft 7.266s

git diff --check
exit 0; no output (no whitespace errors)
```

## Acceptance assessment and status

- PAX metadata is charged within bounded decompressed archive accounting: supported by the reported implementation and regression evidence; focused archive tests pass at the assigned HEAD and source diff.
- Over-cap and over-ratio PAX streams are rejected as invalid input, while within-budget PAX archive extraction succeeds: reported targeted regressions; covered by the independently passing `TestArchive` run.
- Trailing expanded gzip bytes after the tar terminator are included in budget enforcement: reported RED/GREEN regression and the independent `TestArchive` run passes.
- No assigned acceptance gap observed.

**Status: DONE.** No source changes were made. Validation is limited to the assigned PAX checkpoint and specified checks; it does not establish unrelated backend API, authentication, migration, or cancellation behavior.
