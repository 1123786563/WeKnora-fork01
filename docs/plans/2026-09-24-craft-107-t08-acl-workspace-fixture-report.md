# T08 ACL workspace fixture Task 1 report

Status: blocked at the plan's explicit failure boundary; no production or test-file change retained.

## Evidence

- Baseline focused selector failed both requested tests in `publishCraftVersion` (`craft_session_test.go:588`): `craft not found: workspace ws-of-<session-id>`.
- Removing only the two unchecked `UPDATE craft_workspaces SET id = ...` statements reproduced the same errors. This confirms the statements were not a valid fixture correction.
- `publishCraftVersion` independently fabricates `workspaceID := "ws-of-" + scope.SessionID` and publishes using it (`craft_session_test.go:582-588`). That helper is outside Task 1 file ownership. The unchecked setup rewrites were likely intended to make that fabricated ID resolvable, but the draft-head FK rejects the rewrite.
- `createCraftSession` returns the persisted Workspace ID, but `publishCraftVersion` accepts only scope and does not use that ID. The plan explicitly says to stop if the version helper itself assumes a fabricated ID.

## Result

No ACL assertions were changed. Restored the two original fixture statements after the failed experiment, leaving `internal/application/service/craft_session_acl_test.go` unchanged. Escalate the version helper seam for an authorized fix; do not weaken or bypass the draft-head FK.
