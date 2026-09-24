# T05 Selected-KB Truncation Disclosure Fix Report

## Checkpoint and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD stayed `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.
- At task entry `internal/application/service/craft_knowledge.go` was already modified from the preceding typed-KB task, and `craft_knowledge_t05_test.go` was already untracked from that task. Those edits were preserved. This fix changed only those two files and added this report.
- No snapshot/worker, container, RunView, T19, handler, UI, migration or other shared agent files were changed.

## Finding and fix

The production `knowledgeBaseService.HybridSearch` normalizes `MatchCount` and truncates returned primary chunks to that limit. The typed KB path previously asked for exactly `craft.MaxKnowledgeSources` (20), so it could receive 20 of 21 matches and conclude `Truncated=false`.

The typed path now requests a fixed, bounded sentinel limit of `MaxKnowledgeSources + 1` (21) for each independently selected KB. If a KB search returns all 21 results, `retrievalSaturated` is set before document ACL filtering. The signal is combined with the existing package source/byte clipping result on `craft.KnowledgeBundle.Truncated`, which flows into the source record and manifest. Thus a full capped result with denied or cross-KB noise is conservatively disclosed as partial, while the package still includes at most `MaxKnowledgeSources` authorized sources and exposes no rejected source metadata. Retrieval never becomes unbounded.

## RED → GREEN evidence

RED command against the old 20-result request, using a fake that clips exactly to `params.MatchCount`:

```text
go test ./internal/application/service -run '^TestCraftT05KnowledgeBaseSearchUsesBoundedSentinelForTruncation$' -count=1
FAIL: expected MatchCount [21], got [20]; expected regression reproduced before implementation.
```

GREEN after bounded sentinel and saturation propagation:

```text
go test ./internal/application/service -run '^TestCraftT05KnowledgeBaseSearch' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 1.583s
```

Focused full T05 regression after the change:

```text
go test ./internal/application/service -run '^TestCraftT05' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 1.214s
git diff --check
PASS (no output)
```

Regression cases cover: 21 authorized chunks proving the sentinel; multi-KB searches with exactly one selected KB filter per call; a full-cap result containing denied and cross-KB noise, which stays out of the package but conservatively sets `Truncated`; and a below-sentinel control that remains complete. Existing Empty, selected-KB revocation, replay, byte/source limit and UTF-8 truncation cases also passed in the `^TestCraftT05` run.

## File hashes at this checkpoint

```text
a43a0e2224f1e82cac8910956e8f66ec1d2d2f7894a209dec75e80e3491618f6  internal/application/service/craft_knowledge.go
808786c8d425ee8d546a8cd00e774769be5bd210f699c614c8e00a72434f66b7  internal/application/service/craft_knowledge_t05_test.go
```

These hashes identify the full current files, including the preceding typed-KB task edits already present at task entry; they are not hashes of only this fix's isolated increment.

## Remaining T05 gates

This fix only corrects selected-KB retrieval saturation disclosure at the application service boundary. The immutable KB selection still needs to be frozen in the admitted durable Run snapshot and carried through worker/capability/dispatch. Production assembly, current Task/source rechecks at final dispatch, and enforceable per-Run runtime read isolation remain required. This report does not claim full T05 completion.
