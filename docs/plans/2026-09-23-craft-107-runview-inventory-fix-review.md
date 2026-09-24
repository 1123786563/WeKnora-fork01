# T01 RunView inventory completeness fix: independent scoped re-review

Date: 2026-09-23. Read-only review of `runview-inventory-review.md` findings INV-1/INV-2, `runview-inventory-fix-{plan,report}.md`, the checkpoint manifest/patch and two new OpenCode inventory files in the integration worktree. HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No source/test edits, staging, delegation, OCR or live binary claim.

## Exact checkpoint and verdict

| Evidence | SHA-256 |
| --- | --- |
| `internal/modules/agentruntime/agent/opencode/inventory.go` | `1d7d7242b7222096d37dfd54ad534ab35e388078d5c986fdef3c4a801730849d` |
| `internal/modules/agentruntime/agent/opencode/inventory_test.go` | `5a50e121461545e98292524bd8befe7fa45687f89f8600e196e0bf0b53ab8cb6` |
| `docs/plans/2026-09-23-craft-107-runview-inventory-fix-checkpoint.json` | `fe18ed194b4163ca6683dba022774a1f9979ac7818712b611688c88bc4cecd0a` |
| `docs/plans/2026-09-23-craft-107-runview-inventory-fix-checkpoint.patch` | `cd3a69166731d52db1e1286b67e1ecc996bcaf602006170e7ae0902d88aa061d` |

The live full-content source hashes match the manifest and implementation report. Both source files are untracked; shared `client.go` was not modified by this fix.

- **Scoped Spec compliance: PASS.** INV-1 and INV-2 are closed. Every list page needs a non-null cursor object; missing/null on first or later page fails. The inventory-specific redirect policy retains the existing same-origin/directory checks and additionally requires unchanged `project`, `limit`, and `cursor` values on each hop, including later pages.
- **Scoped code quality: PASS.** The client and `http.Client` are copied before wrapping `CheckRedirect`, so the extra policy is confined to inventory calls. Malformed, dropped, duplicated or changed scope values cause an error before the redirected request. Focused tests exercise the previous false-success cases and an exact-preserving later-page redirect. No new scoped finding was identified.
- **Full T01/RunView: NOT VERIFIED.** The pinned live OpenCode binary's `/api/session` list/GET behavior and compatibility with the existing legacy `/session` create route have not been exercised. Provider/runtime integration and per-Run isolation remain separate gates.

## Evidence and tests

`ListSessions` sends exact `project` and `limit=100`, plus `cursor` after the first page (`inventory.go:88-101`). It rejects a missing or null cursor envelope on **every** page (`:104-106`), while `{cursor:{}}` is a valid final envelope only if the page is not full (`:127-149`). It continues to reject missing data, oversized pages, malformed/duplicate/foreign session records and cursor loops (`:107-149,227-253`). This addresses INV-1 without treating a partial page lacking pagination metadata as complete.

`withInventoryRedirectPolicy` preserves the shared client policy and then compares redirect source/target queries (`inventory.go:152-179`). `sameInventoryQueryScope` parses both queries and compares presence, multiplicity and value for `project`, `limit`, and `cursor` (`:181-203`). Because each redirect hop is compared with its immediate source, a later-page cursor cannot be dropped through a chain. The earlier `directoryRedirectPolicy` still checks origin, directory header and forbidden directory query after any user redirect policy (`client.go:89-123`). Tests cover first-page project/limit drops and changes, later-page cursor drops and changes, and an exact-preserving same-origin redirect with directory/header and cursor intact (`inventory_test.go:112-209,323-347`).

I independently ran `go test ./internal/modules/agentruntime/agent/opencode -run '^TestInventory' -count=1` and `go test ./internal/modules/agentruntime/agent/opencode -count=1`; both passed. `git diff --check` on the two new files passed. These are local HTTP fixtures, not proof that the deployed pinned OpenCode binary exposes the proposed v2 inventory route or its exact response schema; 404 remains a fail-closed error (`inventory_test.go:296-321`).
