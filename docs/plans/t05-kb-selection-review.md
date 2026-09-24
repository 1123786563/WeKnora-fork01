# T05 typed knowledge-base selection: independent scoped review

Date: 2026-09-23. Read-only review of `t05-kb-selection-plan.md`/report and the three-file service/test checkpoint in the integration worktree, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec #107 story 2, #124 acceptance mapping, prior T05 service/Publisher reviews, production `HybridSearch` and the shared-aware document ACL seam. No source/test edits, staging or OCR.

## Exact checkpoint and verdict

| File | Full-content SHA-256 |
| --- | --- |
| `internal/application/service/craft_knowledge.go` | `d2eeb2952a1b3904065c9487a0c47b053a5dafad48286368beabf8b6699653e6` |
| `internal/application/service/craft_knowledge_test.go` | `7c2683f9a4b8d17175d6ccd09fb9ec355efb217ec0dc25860c59064300144ba5` |
| `internal/application/service/craft_knowledge_t05_test.go` | `a7072bdb720fa3979cb916468d2248f0d297b0e9652897f5914220262b8f4cd5` |

All hashes match the report. The first two are tracked modifications, and the focused T05 test file is untracked. Their complete content includes inherited T05 work, so this review attributes only the typed KB selection increment and associated tests to the current task.

- **Scoped Spec compliance: FAIL (Medium finding below).** The typed path correctly selects KBs, checks current KB access, intersects returned documents with current document ACL and exact owning KB, separates request digests, and replays accepted sealed bytes. However, its `Truncated` result can falsely say no truncation when the production search has already cut off additional matches.
- **Scoped code quality: FAIL (same finding).** The truncation test fake returns more rows than its requested `MatchCount`, unlike production `HybridSearch`; the test cannot detect the production cap. No other scoped finding was identified.
- **Full T05/#124: NOT VERIFIED.** The durable Run request/snapshot still does not freeze typed KB selections or dispatch them to this method. Current-authenticated production wiring, per-Run read isolation, and final source-open/Publisher gates remain separate. The broad service package run was interrupted and is not completion evidence.

## Medium — production search cap hides truncation

**Evidence / affected symbols:** `BuildForKnowledgeBases` calls the search port separately for each selected KB with `MatchCount: craft.MaxKnowledgeSources` (20) (`craft_knowledge.go:843-852`). Production `knowledgeBaseService.HybridSearch` truncates its deduplicated result list to `params.MatchCount` *before returning* (`knowledgebase_search.go:293-300`). The typed builder then passes only returned candidates to `craft.BoundSources` (`craft_knowledge.go:892-925`), whose `Truncated` flag becomes true only when it sees an extra source or drops excerpt bytes (`internal/modules/craft/knowledge.go:102-119`). Thus a KB with 21 or more otherwise authorized matches can return exactly 20 and be recorded with `Truncated=false`. The focused truncation test seeds 21 chunks but uses a fake search that returns all 21 despite the requested cap (`craft_knowledge_t05_test.go:642-648` and fake adaptation in `craft_knowledge_test.go`), so it does not model the production contract.

**Impact:** The user-facing bundle and immutable actual-source record can say the selected KB's results were complete even though the retrieval layer discarded additional results. #124 requires empty and truncated results to be disclosed honestly.

**Smallest defensible correction:** Obtain an explicit `has_more`/total-count signal from the production search contract, or fetch and filter far enough to establish at least one additional *authorized* source beyond the bundle cap. If the backend cannot prove exhaustion after ACL filtering, record an explicit incomplete/possibly-truncated fact rather than `Truncated=false`. Add a test fake that enforces the same `MatchCount` cap as `HybridSearch`, with more authorized matches than the cap and with denied/noisy rows near the boundary.

## Confirmed behavior and limits

The new API canonicalizes bounded, unique KB IDs and hashes a typed `knowledge_base_selection` payload, distinct from the document-ID digest (`craft_knowledge.go:167-181, 932-961`). It requires current `TaskWrite` before record load or publication (`:187-204`). For each selected KB, it sends exactly one KB ID as both primary search ID and `KnowledgeBaseIDs` filter (`:843-858`); production `HybridSearch` loads that KB and checks its current grant before search, including empty results (`knowledgebase_search.go:145-187`). Returned document IDs are re-resolved through the shared-aware access port, and only rows whose `KnowledgeBaseID` equals the selected KB enter the package (`craft_knowledge.go:876-925`). Denied/cross-KB fake results are excluded from bundle and package tests (`craft_knowledge_t05_test.go:551-615`).

For an existing Run, digest mismatch returns conflict before package change; matching replay resumes the accepted digest-bound package, reauthorizes recorded documents and KBs, and discards new search bytes (`craft_knowledge.go:195-242, 376-456`). A selected KB with no recorded source rows is separately checked again through guarded search before replay (`:963-989`), with a revocation test (`craft_knowledge_t05_test.go:618-658`). These checks preserve the old document-ID service contract while making the new KB path explicit.

I independently ran `go test ./internal/application/service -run '^TestCraftT05BuildForKnowledgeBases' -count=1` (passed) and `git diff --check` on the two tracked files (passed). The implementer reports the wider focused T05 suite passed at these hashes; the full service package run was interrupted. The new typed method has no production call site yet, so service tests alone cannot establish actual member selection or per-Run isolation.
