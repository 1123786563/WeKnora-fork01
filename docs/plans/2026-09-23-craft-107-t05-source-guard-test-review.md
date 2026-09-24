# T05 source-guard test-contract reconciliation independent review

Date: 2026-09-23. Read-only scoped review at integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec/#124, the T05 source-guard test plan/report, `craft.KnowledgeBundle`/`Source` and `BoundSources` contracts, prior T05 reviews, and the captured full-service failure. Only `internal/application/service/craft_source_guard_test.go` was reviewed as the task delta. No production/test edits, staging, commit or OCR.

## Checkpoint and verdict

The live test file SHA-256 is `95dd674642e95771a45073396f92ec39934f518ce5ec22dd86a75922ddcbdcf6`, matching the implementation report; it is an uncommitted tracked modification. The diff changes only the two exact key allowlists and one `Empty=false` assertion (`craft_source_guard_test.go:78-83`).

- **Scoped Spec compliance: PASS.** `KnowledgeBundle` declares `Sources`, `Truncated`, and `Empty`; `BoundSources` sets `Empty` from the bounded source count (`internal/modules/craft/knowledge.go:94-118`). The test creates one retained source, so `Empty=false` is correct. `Source` declares `AcquiredAt` and `Truncated` in addition to the prior five provenance fields (`knowledge.go:58-63`). These fields are timestamp/status data, not tool, network, credential, sandbox or authorization policy. The approved source-material requirement is preserved.
- **Scoped code quality: PASS.** `require.ElementsMatch` still enforces an exact top-level and nested key set, so an added policy-bearing JSON field fails. The test still stages the prompt-injection text verbatim and asserts that it remains an `Excerpt` rather than changing any execution policy (`craft_source_guard_test.go:52-83`). It does not remove the cross-tenant or revoked-share access checks (`:16-50`). No scoped finding.
- **Full T05/#124 acceptance: NOT VERIFIED.** This is a test expectation correction. It does not establish production Publisher integration, dispatch grant recheck, per-Run read-only material projection or inside-sandbox Run A→B exclusion.

## Verification and limit

I independently ran `go test ./internal/application/service -run '^(TestCraftSourceGuard|TestCraftT05|TestCraftKnowledge)' -count=1`: PASS (`ok`, 0.774 s). The report records RED on the former top-level `Empty` mismatch, then on the nested `AcquiredAt`/`Truncated` mismatch, and GREEN for the exact target test. The altered assertion is a serialized DTO-shape guard; it does not by itself prove that a deployed delegate cannot execute instructions from an excerpt. That boundary remains with the sandbox/runtime checks documented in the T05 material design.
