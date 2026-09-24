# T05 production Publisher adapter independent review

Date: 2026-09-23. Read-only scoped review in integration Worktree at HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Sources: approved Craft Spec/#124, T05 material-boundary design, Publisher plan/report, existing `CraftKnowledgePackagePublisher`/`Resumer` service contract. No code/test edits, staging, commit, or OCR.

## Checkpoint and verdict

The two untracked owned files match report SHA-256: `internal/container/craft_knowledge_publisher.go` `bd40ecd810aad81d84af45fe8319f75cbae546303c98feadbb76ae0764a1f48c`; `internal/container/craft_knowledge_publisher_test.go` `e72f629f5217d6ceed0bf51c4b57175c41483e241533ec501242c4be71a09514`. The implementation report names checkpoint `t05-publisher-uncommitted-01`; no committed diff represents these files.

- **Scoped Spec compliance: FAIL.** Normal-path private preparation, exact digest/manifest validation, same-filesystem atomic rename, replay and conflict behavior are present. A crash in the acknowledged post-rename/pre-chmod window leaves a visible 0700 package that every retry and Resume rejects, violating the plan's recoverable accepted package requirement. The directory also remains writable in that interval if a delegate can reach it with the publishing user's permissions.
- **Scoped code quality: FAIL on the High finding below.** Path token checks and `Lstat` reject straightforward traversal and symlinks, private candidates are 0700, files become 0444 before rename, and the code syncs files and directories. The failure window has no repair path or test. A smaller durability cleanup issue is recorded below.
- **Full #124 / Run A→B isolation: NOT VERIFIED.** The adapter is not wired into production; no per-Run sandbox root, read-only mount/copy enforcement, dispatch authorization recheck, actual delegate identity, or inside-sandbox access test is established here. Unix file modes alone do not prove delegate immutability.

## Findings

### High — crash after atomic rename permanently blocks exact-package recovery

**Evidence / affected symbols:** `Publish` renames the candidate payload into the visible final path while it is 0700 (`craft_knowledge_publisher.go:227-248`), then chmods it to 0555. If the process stops between lines 234 and 247, `readPublished` sees the final directory but `readCraftKnowledgePayload(..., requireReadOnly=true)` rejects its writable mode (`:422-444, 623-630`). `Resume` returns that error without trying the sealed candidate (`:301-326`); `Prepare` and `Publish` also return at `readPublished` (`:115-122, 206-213`). The candidate seal remains, but its payload has moved, so the private-candidate reader cannot recover it either (`:477-499`). The six tests verify post-return modes and an injected rename failure, not interruption after successful rename (`craft_knowledge_publisher_test.go:140-210`).

**Impact:** A fully accepted package can be visible yet unusable after restart. Same-Run retry cannot complete, and central service cannot retrieve the exact accepted bytes; manual filesystem repair would be needed. During the window the complete directory is writable by a process with the publisher's effective user permissions. No current production wiring proves a delegate can enter before `Publish` returns, but the adapter itself does not enforce that ordering.

**Smallest defensible correction:** Add a restartable finalize path: when a published directory is present but not sealed, validate its complete bytes against the exact accepted digest and retained candidate seal, set final modes, sync the directory and parents, then allow `Resume`/same-package `Publish` to complete. Reject mismatches. Test interruption immediately after successful rename, reconstruction with a new adapter, exact Resume/replay, and changed-digest conflict. Central integration must keep the delegate stopped or the RunView unmounted until Publish has returned successfully, or use a genuinely read-only mount/ACL that is effective before exposure.

### Low — candidate removal syncs the wrong parent directory

**Evidence / affected symbols:** After `os.RemoveAll(candidatePath)`, both `Publish` and `Discard` sync `filepath.Dir(filepath.Dir(candidatePath))` (`craft_knowledge_publisher.go:261-265, 292-296`), which is `candidateRoot`. The directory entry for `candidatePath` lives in `filepath.Dir(candidatePath)` (the per-Run candidate directory). `Prepare` correctly syncs that immediate parent after rename (`:182-190`).

**Impact:** Candidate deletion is not guaranteed durable after a power loss; a stale private candidate may reappear. This does not by itself expose delegate-visible files because the final tree is separately synced.

**Smallest defensible correction:** Sync the immediate per-Run candidate parent after removal, and include a recovery test for a stale or partially removed private candidate.

## Verified behavior and limits

`go test ./internal/container -run CraftKnowledgePublisher -count=1` passed independently (`ok`, 1.649 s; linker emitted a duplicate `-lc++` warning). The report also records six focused tests and `git diff --check` passing. I did not rerun broader integration tests because this adapter is not yet wired and the focused tests cover the owned seam.

The constructor requires an absolute canonical root and path-safe Run ID (`craft_knowledge_publisher.go:85-103`); package file names stay directly under the expected Run directory (`:329-345, 701-725`). Root/target and candidate paths are checked with `EvalSymlinks`/`Lstat` (`:538-620`), and payload readers reject symlinks, nonregular files, oversized content and writable published files (`:623-660`). These checks assume a server-owned host filesystem without an adversary concurrently replacing parent components between checks and opens. The root's sibling candidate tree is outside the intended delegate view and mode 0700 (`:74-83, 523-531`), but only central mount assembly can establish that the delegate cannot access that sibling.
