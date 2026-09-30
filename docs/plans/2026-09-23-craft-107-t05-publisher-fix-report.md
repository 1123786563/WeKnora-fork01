# T05 Publisher Post-Rename Recovery Fix Report

**Plan:** `docs/plans/2026-09-23-craft-107-t05-publisher-fix-plan.md`  
**Review:** `docs/plans/2026-09-23-craft-107-t05-publisher-review.md`  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Checkpoint:** `t05-publisher-fix-uncommitted-01`  
**HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit)  
**Role metadata:** Runtime did not expose role/model/reasoning metadata.

## Change scope

Updated only the owned Publisher adapter and its focused tests. For a final Run directory left at mode 0700 after the atomic rename, the adapter now requires the matching private candidate seal, verifies the candidate is private and path-safe, verifies every final file and manifest against the sealed accepted digest and current workspace scope, and rejects corrupt bytes, symlinks, wrong scope, missing seals, unexpected candidate entries, or mismatched digests without deleting evidence. Exact valid bytes are finalized to read-only file/directory modes and synced before removing the private remainder. A retry after chmod but before candidate cleanup validates the published package, completes durability sync, removes the matching remainder, and is idempotent.

Candidate deletion in Publish, Discard, and retry recovery now syncs the immediate per-Run candidate parent containing the removed digest entry. If a retry finds that digest entry already absent but its per-Run parent still exists, it syncs that parent again to recover from a prior cleanup-sync failure.

## RED → GREEN evidence

- High-finding RED: `go test internal/container/craft_knowledge_publisher.go internal/container/craft_knowledge_publisher_test.go -run 'TestCraftKnowledgePublisher(ResumesCrashAfterRename|RefusesAndPreservesCorruptCrashRemainder)' -count=1 -v` failed the crash-after-rename test with `craft conflict` while the final directory was exactly 0700. Corrupt-byte and symlink refusal subcases passed before and after the fix, preserving their final/candidate evidence.
- Cleanup RED: `TestCraftKnowledgePublisherFinalizesChmodCrashAndSyncsCandidateParent` failed because the matching candidate directory remained after Resume.
- Parent-sync RED: `TestCraftKnowledgePublisherSyncsImmediateCandidateParentAfterRemoval` failed in both Publish and Discard because the injected sync seam did not observe the immediate per-Run candidate parent.
- Focused GREEN: `go test internal/container/craft_knowledge_publisher.go internal/container/craft_knowledge_publisher_test.go -run CraftKnowledgePublisher -count=1` — PASS, all ten Publisher tests including exact recovery, changed bytes, wrong scope, symlink preservation, post-chmod cleanup idempotence, and Publish/Discard parent sync.
- Package GREEN: `go test ./internal/container -run CraftKnowledgePublisher -count=1` — PASS (`ok`, 2.139s); linker emitted the existing duplicate `-lc++` warning.
- Formatting and whitespace: `gofmt -d internal/container/craft_knowledge_publisher.go internal/container/craft_knowledge_publisher_test.go` produced no diff; scoped trailing-whitespace check — PASS.

## Checkpoint

The adapter and test file existed as untracked files before this round; the prior checkpoint SHA-256 values were publisher `bd40ecd810aad81d84af45fe8319f75cbae546303c98feadbb76ae0764a1f48c` and tests `e72f629f5217d6ceed0bf51c4b57175c41483e241533ec501242c4be71a09514`. They remain untracked and include both earlier Publisher work and this scoped fix. Other agents' edits were preserved.

Final SHA-256:

- `internal/container/craft_knowledge_publisher.go`: `ecc1c5f8d85abf5ef495fdac3c9c9c4335b084c3d8a5e038a1d10f9514c5b453`
- `internal/container/craft_knowledge_publisher_test.go`: `962412211c6bf2bddcc7c4603d515c2ecf78edb4941eacaad70780e3bca2253f`

## Remaining gates and limits

No production wiring, per-Run read-only mount, dispatch ACL recheck, or actual delegate identity/access validation is included. Full T05 remains gated on those central integration checks. The macOS atomic rename path still makes the complete directory visible at mode 0700 for the brief interval before chmod; central integration must keep the delegate stopped or the RunView unmounted until Publish returns successfully. Unix mode checks in this adapter do not establish a production sandbox boundary.
