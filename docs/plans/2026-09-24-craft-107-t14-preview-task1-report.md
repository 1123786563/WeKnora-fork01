# T14 preview seam transfer — Task 1 report

Status: **DONE_WITH_CONCERNS** (Task 1 complete; production preview remains closed pending B3 wiring and live browser/network attestation).

## Scope and baseline

- Integration worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- Integration HEAD before transfer: `a5e9195acd6500c085c85d60c852148e7bbbbf34`
- T14 source commit: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- T14 source tree: `d207c66a881a043d2cb2a93c6ded6e8ad9f89355`
- Immediately before transfer, all four owned integration paths were clean: no staged, unstaged, or untracked entries. The integration worktree had pre-existing changes elsewhere, including `internal/container/container.go`, `internal/router/routes_chat.go`, and `internal/router/routes_craft_features_test.go`; these were preserved.
- No container, router, render-boundary, or other source paths were edited. No commit was created.

## Transfer evidence

Applied the path-scoped diff from integration HEAD to the T14 source commit. No manual adaptation was needed: integration already provides `craft.PreviewOriginAllowed`, `craft.TaskAccessChecker` / `RequireTaskAccess`, and the sandbox binding/config interfaces used by the source seam.

| Owned path | Integration pre-transfer SHA-256 | Post-transfer SHA-256 (matches source) |
|---|---|---|
| `internal/application/service/craft_preview.go` | `dd3b6cdbd515331e794b4a50d26e69b1f276a50ea477fed6b235c3988771aec9` | `1d877c1193718f022c9f620c2d6bab607a4ac6dec4b55797c7ae41334b0ae4d7` |
| `internal/application/service/craft_preview_network_test.go` | absent | `0c974f83900804a1f303dc7f74d7a5b8b9eb82733e8e8a588332c6f64036ea38` |
| `internal/handler/session/craft_preview.go` | `f2381b7fe7a02a105c99124f1a69f2095557e18afbef5acde4abf484e1b3c2c5` | `4f4a5f61e9f9275c77cb936a51a3e7efde7a99bc179502725e18bfd7acc77c3b` |
| `internal/handler/session/craft_preview_test.go` | `476a73251387b363f9791a8ad590ee9a13cfb6db3ac3a5d66d56e3560d8bea33` | `1b0ce3b338c1fa5c26d73d49bede1f2027ba9a65ad94c594b6c6a5a7b52ec46f` |

Each post-transfer hash was compared to `git show <T14 commit>:<path>` and matched exactly. The full path-scoped transfer patch, including the new network test, is saved at `docs/plans/2026-09-24-craft-107-t14-preview-task1-task-local.patch` (SHA-256 `e8a710a192738deb6328ca7188234d609fb335f9aa04904fca430a563344b9c0`).

## Checks run

- `git apply --check docs/plans/2026-09-24-craft-107-t14-preview-task1-task-local.patch` — passed.
- `go test ./internal/application/service -run 'CraftPreview' -count=1` — passed (`ok`, 1.043s).
- `go test ./internal/handler/session -run 'CraftPreview' -count=1` — passed (`ok`, 0.875s).
- `go test ./internal/container ./internal/router -run '^$' -count=1` — both packages compiled; no tests selected. Linker emitted only a duplicate `-lc++` warning for container.
- `git diff --check -- <four owned paths>` — passed with no output.

## Result and limits

The service now enforces current TaskPreview access at issue/open/read, requires the preview network checker, checks the bound live Docker network mode, and remains disabled without browser navigation protection. The file handler requires the configured preview host. The focused tests pass.

The integration constructor was deliberately not wired by this task and currently does not supply these new security dependencies; preview therefore remains fail-closed. B3 must inject the current T08 access checker and selected-bound-sandbox network checker, and live browser/network attestation is still required before any production enablement. The optional live Docker inspector and browser probe were not run in this task.
