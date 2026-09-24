# T05 selected-KB truncation fix: independent scoped review

Date: 2026-09-23. Read-only review of `t05-kb-truncation-fix-plan.md`/report and its two-file service/test checkpoint in the integration worktree, HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec #107/#124, prior `t05-kb-selection-review.md`, production `HybridSearch` result cap and `craft.BoundSources`. No source/test edits, staging or OCR.

## Checkpoint and verdict

| Reviewed file | Full-content SHA-256 |
| --- | --- |
| `internal/application/service/craft_knowledge.go` | `a43a0e2224f1e82cac8910956e8f66ec1d2d2f7894a209dec75e80e3491618f6` |
| `internal/application/service/craft_knowledge_t05_test.go` | `808786c8d425ee8d546a8cd00e774769be5bd210f699c614c8e00a72434f66b7` |

Both hashes match the implementation report. The service file is a tracked modification, and the focused test file is untracked; both contain inherited T05 work. The current fix's delta is the bounded sentinel request, saturation propagation and production-cap-faithful tests.

- **Scoped Spec compliance: PASS.** The previous Medium false-complete result is closed. The typed selected-KB path requests one bounded sentinel beyond the 20-source material cap per KB. It reports truncation when a sentinel is dropped or when a full search result is inconclusive after document ACL filtering, without publishing rejected sources.
- **Scoped code quality: PASS.** The new fakes enforce the production `MatchCount` cap, test an authorized 21st result, saturated denied/cross-KB noise, independent selected-KB calls and a below-limit complete result. No new scoped finding was identified.
- **Full T05/#124: NOT VERIFIED.** Typed KB selection is still not frozen in the durable Run snapshot or wired through production worker/dispatch; final authority checks and per-Run material isolation remain separate gates. The broad service-package run was interrupted and is not a pass.

## Evidence

Production `knowledgeBaseService.HybridSearch` truncates returned chunks to `params.MatchCount` (`knowledgebase_search.go:293-300`). The reviewed path requests `craft.MaxKnowledgeSources + 1` for each selected KB, still with exactly that KB as the primary ID and the only `KnowledgeBaseIDs` filter (`craft_knowledge.go:843-859`). A full 21-result response sets `retrievalSaturated` *before* filtering document ACL/noise (`:861-881`); authorized candidates still pass the existing document-to-KB intersection (`:884-931`). `craft.BoundSources` limits the resulting material to 20 sources/byte cap, then the builder ORs in the saturation fact (`:932-938`). Therefore an authorized 21st candidate is omitted and disclosed; a full result with denied or cross-KB noise is conservatively disclosed as incomplete. A result below the sentinel limit remains complete when no source/byte clipping occurs.

The new sentinel fake explicitly slices search results to the requested `MatchCount` as production does (`craft_knowledge_t05_test.go:700-733`). Further tests cover saturated denied and cross-KB results across two selected KBs (`:735-794`) and an unsaturated control (`:796-825`). Rejected document content is absent from staged package bytes in the noise test. The source list, digest-bound package and replay mechanisms are inherited from the previously reviewed T05 checkpoint and were not weakened by this delta.

I independently ran `go test ./internal/application/service -run '^TestCraftT05KnowledgeBaseSearch' -count=1`, `go test ./internal/application/service -run '^TestCraftT05' -count=1`, and `git diff --check` on the reviewed tracked file; all passed. The report's complete hashes matched the live files after those checks. A full production search may have other internal limits or ranking behavior; this fix gives a truthful conservative saturation signal for the explicit `MatchCount` cap, not proof that a live search exhausts every authorized document.
