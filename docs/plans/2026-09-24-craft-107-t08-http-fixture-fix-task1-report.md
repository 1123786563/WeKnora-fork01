# Craft #107 T08 HTTP Fixture Fix Task 1 Report

## Scope and change

Changed only the fixture setup in `internal/handler/session/craft_test.go` inside `TestCraftHTTPListAndVersionDownload`: `workspaceID` now comes from `first["workspace_id"]` returned by `env.createSession`; removed the raw `UPDATE craft_workspaces SET id` that rewrote the immutable Workspace key. No production code or authorization assertions changed as part of this task.

## Evidence

- RED before edit: `go test ./internal/handler/session -run '^TestCraftHTTPListAndVersionDownload$' -count=1` failed at version publication with `craft not found: workspace ws-http-eabf84b8-96f8-4594-aa03-0467a9662774`.
- GREEN: same focused test passed (`ok .../internal/handler/session 1.580s`).
- Handler selector: `go test ./internal/handler/session -run '^TestCraftHTTP' -count=1` passed (`ok .../internal/handler/session 6.391s`).
- B5 route check: `go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction$' -count=1` passed (`ok .../internal/router 1.801s`).
- `gofmt -w internal/handler/session/craft_test.go` and `git diff --check -- internal/handler/session/craft_test.go` completed successfully.
- The worktree already contained unrelated changes to `craft_test.go` before this task. The task preimage was reconstructed by reversing only this one-line fixture change; its SHA-256 matches the pre-edit hash observed before editing: `ddcc0fe7fa024c468b8baf8a879178cecf5e90ca538c4ec1d732e98c8a532b48`.

## Checkpoint artifacts

- `2026-09-24-craft-107-t08-http-fixture-fix-task1.checkpoint.json`
- `2026-09-24-craft-107-t08-http-fixture-fix-task1.patch`
- `.preimage`, `.postimage`, `.pre.sha256`, and `.post.sha256` siblings preserve the exact owned-file before/after states.

No commit created.
