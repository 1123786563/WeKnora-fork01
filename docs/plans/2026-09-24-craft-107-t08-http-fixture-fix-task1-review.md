# Craft #107 T08 HTTP fixture Fix Task 1 — independent review

Scope: exact Task 1 checkpoint in `internal/handler/session/craft_test.go`, against the T08 fixture plan, current joined validation and approved web-artifact Spec. Read-only review; no source, test, requirement or remote issue changes.

## Verdict

- **Scoped Spec compliance: PASS.** `TestCraftHTTPListAndVersionDownload` now derives `workspaceID` from `first["workspace_id"]`, returned by the real `env.createSession` HTTP call. The raw `UPDATE craft_workspaces SET id` was removed. All list, version, download, missing-file and traversal assertions remain unchanged. No authorization or production policy code changed in this task increment.
- **Scoped code quality: PASS.** The fixture uses the authoritative created Workspace identity, so the published test Version refers to the same persisted Workspace that the handler later resolves. No finding in this one-file increment.

## Exact checkpoint and verification

- The preimage SHA-256 is `ddcc0fe7fa024c468b8baf8a879178cecf5e90ca538c4ec1d732e98c8a532b48`; the postimage and current owned-file SHA-256 are both `8bef3ad0fb7d0abb940109513a50d82a9aa4d573d231fe99b3963cdb3286727e`. The saved pre/post images differ only at the Workspace ID assignment and removed SQL line, exactly as in the task patch. The patch SHA-256 is `3bfc91feeb603dbef4db1f32f82bcec254614d131e2e248bc64074c08b35f6c3`.
- `go test ./internal/handler/session -run '^TestCraftHTTPListAndVersionDownload$' -count=1` — PASS (`1.889s`).
- `go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction$' -count=1` — PASS (`1.409s`).
- `git diff --check -- internal/handler/session/craft_test.go` — PASS.

This verdict covers only the Task 1 fixture increment. The broader T08 joined-validation report's preview-path limitation remains outside this fix.
