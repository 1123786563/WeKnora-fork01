# Craft T03 Java source and read-audit repair — independent validation continuation

**Status: DONE_WITH_CONCERNS.** Revalidated the frozen Fix1 checkpoint at HEAD `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159` in `/Users/wuyongjun/.codex/worktrees/craft-107-t03-fix/WeKnora-fork01` on 2026-09-29. The four implementation/test files matched the previously recorded frozen hashes both before and after checks. No source or test files were changed.

## Exact checks

All commands ran from the worktree root:

- `go test ./internal/modules/craft/... -run 'Input|Code' -count=1` — PASS, exit 0 (`internal/modules/craft` 2.346s).
- `go test ./internal/application/service/... -run 'CraftInput|CraftDelegate|CraftMaterialPolicy' -count=1` — PASS, exit 0 (`service` 3.384s; `service/file` no tests to run).
- `go test ./internal/container/... -run 'CraftExecCommandFaces|CraftRuntime' -count=1` — PASS, exit 0 (`container` 2.254s); linker emitted duplicate `-lc++` warning.

## Frozen source hashes

| File | SHA-256 before and after |
|---|---|
| `internal/modules/craft/input_code.go` | `faa1c86cb6e467b39d8a866f5146147a8d902eccbda4837ad8ab1b169d9cb871` |
| `internal/modules/craft/input_code_test.go` | `bc5149458ab5cfbb41640577d25cc170efff6177e5bed25f9134ff08efcfb2d9` |
| `internal/application/service/craft_delegate.go` | `ef704ab689dd7c9d05bd6d813811c2a25ff3b58d22034e0862207db269abba92` |
| `internal/application/service/craft_execution_policy_wiring_test.go` | `49e2a6ca5b8c5d316f11cad89a74b0635e7e1f74627aa9471fe0304fb250fdbd` |

## Acceptance status and gaps

The targeted suites pass and cover the Java `--source` operand screening and successful-path input-read audit described in the existing independent validation report. They do not resolve the independent review findings `T03-FIX-R1` through `R3`: audit persistence failure can still allow dispatch or partial rows; reader argument handling can misidentify or collapse inputs; and preflight writes a success outcome before provider execution is known. These remain one High and two Medium acceptance/security gaps. Production normal/restricted command-face activation also remains explicitly blocked on the authoritative T19/T20 dispatch and identity contracts.

See `2026-09-29-craft-107-t03-java-source-and-read-audit-fix-review.md` and `2026-09-29-craft-107-t03-java-source-and-read-audit-fix-validation.md` for detailed evidence and rulings. No migration or authentication changes are in scope; persistence correctness is implicated by R1. The worktree contains other pre-existing modified/untracked material; this validation changed only this continuation report and did not stage or commit it.
