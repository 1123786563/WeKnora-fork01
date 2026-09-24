# T05 Dispatch-Time Knowledge Revalidation Report

## Checkpoint and scope

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD stayed `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit was created.
- At task entry `craft_knowledge.go` was already modified by earlier T05 rounds, and `craft_knowledge_t05_test.go` was already untracked from those rounds. Those edits were preserved. This task changed only these two files and added this report.
- No dispatch wiring, handler, container, runtime, RunView, T19, migration, UI, or other shared-agent files were changed.

## RED → GREEN

Before adding `RevalidateForDispatch`, the focused RED command:

```text
go test ./internal/application/service -run '^TestCraftT05RevalidateForDispatch' -count=1
FAIL: RevalidateForDispatch undefined at the new assertions, as expected.
```

After implementation and final test additions:

```text
go test ./internal/application/service -run '^TestCraftT05|^TestCraftSourceGuard' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 1.283s
git diff --check
PASS (no output)
```

The passing tests verify authenticated caller-to-scope equality; TaskWrite; exact Run and scope; published-only state; live document resolution and owning-KB permission revocation; empty published record still requiring TaskWrite; and fail-closed behavior when TaskAccess, records, document access or KB search port is missing. A guarded Search stub returns an error if called, and tests assert it is not called. Publisher counters/candidates remain untouched and the accepted record remains unchanged.

## Implementation evidence

`CraftKnowledgeService.RevalidateForDispatch(ctx, scope, runID)` is a distinct, read-only method. It requires record/access/search ports, valid Run identity, a caller matching tenant/user, current TaskWrite, an exact-scope exact-Run published record, and non-empty immutable request/package digests. It rejects empty/source consistency mismatches. It does not require or invoke Publisher.

For each recorded source, the method uses the immutable ref only to recover its document/KB coordinates, then resolves those document IDs through `CraftKnowledgeAccess` and requires exact current KB and tenant coordinates. Production binds this access port to `GetKnowledgeBatchWithSharedAccess`, which checks current document visibility and shared-KB grants. It fails closed on absent or changed coordinates. It does not call HybridSearch, rebuild, publish, mutate records or expose source bytes.

## File hashes at this checkpoint

```text
bf93348d30bba628ee9947ea72e3fae610473c8080153f6de598f33f1cf04e60  internal/application/service/craft_knowledge.go
b14068d35a7a43ba4651b2d5aeb04fbd448825380efe25051802b411d227cd2b  internal/application/service/craft_knowledge_t05_test.go
```

These identify the full current files, including previous T05 task edits present before this task; they are not hashes of only this task's isolated increment.

## Remaining integration gates

The later delegate wiring task must call this method for a genuinely new dispatch using the authenticated actor scope and authoritative durable `RunID`, after its retry/unknown-outcome checks and immediately before executor delegation. Worker snapshot selection and capability assembly remain unwired. Publisher production assembly and an enforceable per-Run sandbox read boundary are also still required. This task does not claim full T05 completion.
