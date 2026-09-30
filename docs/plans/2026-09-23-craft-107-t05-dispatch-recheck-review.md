# T05 dispatch revalidation: independent scoped review

Date: 2026-09-23. Read-only review of `t05-dispatch-recheck-plan.md`/report, the prior KB selection and truncation reviews, approved Craft Spec #107/#124, and the two-file checkpoint in the integration worktree. HEAD was `a5e9195acd6500c085c85d60c852148e7bbbbf34`. No source or test edits, staging, delegation, or OCR.

## Checkpoint and verdict

| Reviewed full file | SHA-256 |
| --- | --- |
| `internal/application/service/craft_knowledge.go` | `bf93348d30bba628ee9947ea72e3fae610473c8080153f6de598f33f1cf04e60` |
| `internal/application/service/craft_knowledge_t05_test.go` | `b14068d35a7a43ba4651b2d5aeb04fbd448825380efe25051802b411d227cd2b` |

Both hashes match the implementation report. The files include earlier T05 work; this verdict covers the new `RevalidateForDispatch` method, its helper, and focused tests.

- **Scoped Spec compliance: PASS.** The method checks the authenticated actor against the supplied durable scope, current `TaskWrite`, exact scope/Run on the loaded record, published state, and present request/package digests before reauthorizing all recorded document and owning-KB coordinates. An empty published record still requires current `TaskWrite`.
- **Scoped code quality: PASS.** The check uses the existing shared-aware document ACL seam, returns denied/missing/changed coordinates as failures, and has no workspace, Publisher, search, or record mutation in the successful path. No scoped finding was identified.
- **Full T05/#124: NOT VERIFIED.** This method is not yet called by the delegate dispatch path. Durable snapshot selection, production actor principal wiring, Publisher assembly and per-Run sandbox material isolation remain integration gates.

## Evidence and limits

`RevalidateForDispatch` rejects missing records/access/search ports or an invalid Run ID (`craft_knowledge.go:363-365`). It compares the caller's tenant/user with the requested scope before `RequireTaskAccess(..., TaskWrite)` (`:367-373`). It loads the accepted record for the exact scope/Run and requires `SameScope`, matching Run ID, `Published`, nonempty request/package digests, and source/empty consistency (`:374-393`). The record store is a scope-bound durable interface, so this is a check of the accepted record, not an independent filesystem digest verification.

For every source, the helper extracts document and KB IDs from its immutable ref, requires nonzero source tenant, rejects conflicting repeated document coordinates, and loads the sorted document IDs through `CraftKnowledgeAccess` (`:401-435`). Every returned row must match the expected document ID, owning KB ID and source tenant (`:436-452`). Production `BindCraftKnowledgeAccess` calls `GetKnowledgeBatchWithSharedAccess` (`:51-56`), which rechecks the current owning-KB grant when returning each document (`knowledge.go:824-866`). Search is deliberately not invoked: live result bytes cannot rebuild or replace the accepted package during this authority check. The method also performs no Publisher call or record mutation.

Focused tests cover successful recheck with search and Publisher side effects forbidden, revoked TaskWrite/document/shared KB, forged caller, wrong record scope/tenant/Run, prepared/unknown publication, empty package, and missing ports (`craft_knowledge_t05_test.go:827-1000`). I independently ran `go test ./internal/application/service -run '^TestCraftT05RevalidateForDispatch|^TestCraftSourceGuard' -count=1` and `git diff --check -- internal/application/service/craft_knowledge.go`; both passed. Full service-package testing and live dispatch behavior were not established by this scoped run.

The accepted record stores source coordinates and digest strings, but this check does not reopen the published package or compare its bytes with `PackageDigest`. That is an intentional narrow authority check: package integrity depends on the separately reviewed Publisher and the later dispatch/per-Run read boundary. An empty package has no recorded KB coordinate to reauthorize; it is accepted only after current TaskWrite and published-record checks, as specified in this Task brief.
