# T01 R3 Task 2 fix3 — atomic publication and truthful failure recovery

Date: 2026-09-24. Investigation only; source and tests were not changed or executed. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Observed HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Apply through a new writing-plans/SDD repair task, with exact uncommitted pre/post checkpoint and independent review; this document does not declare Task 2 verified.

## Evidence and root cause

Read the original `2026-09-24-craft-107-runview-r3-material-plan.md` and Task 2 review, fix1 and fix2 plans/reviews, fix2 report, and current `internal/container/craft_runtime.go` / `craft_runtime_test.go`. Used the systematic-debugging skill to trace the failed ownership assumption before proposing another change.

Original ordinary rename could overwrite a racing final target. Fix1 changed publication to `Linkat`, then attempted inode-check/path-unlink rollback. Fix2 correctly removed final-file and digest-directory rollback, but retained `Linkat(temp, final)` followed by `Unlinkat(temp)`. The same incorrect assumption moved to the temporary name: an open descriptor pins an inode; it does not reserve that inode's directory entry. `O_EXCL`, a random name, `Fstat(fd)`, and a preceding path identity comparison cannot reserve the name either.

Exact remaining interleaving in fix2 (`craft_runtime.go:782-794`): our link succeeds; target identity check succeeds; another writer renames the temporary entry away and creates its own file at that temporary name; our unlink deletes the other writer's file. A process mutex would only cover cooperating calls in that process and does not satisfy the stated mutation scenario. Adding another `Fstatat` would recreate the previous failure.

The directory failure finding is separate. `openOrCreateRunViewInputDigestDirWithAfterCreate` retains a created directory on open/chmod/sync/stat failure, but its test callback removes that directory itself. The test proves its own deletion. A combined `Fstat`/mode conditional also discards a syscall error in favor of `craft.ErrConflict`.

## Chosen narrow design

Use one atomic, descriptor-relative, same-directory **no-replace rename** to publish the fully written temporary file. The kernel removes the source entry as part of that operation, so there is no subsequent temporary-name unlink to race. Keep the existing policy of retaining failed material and returning before Prompt. Remove all staging cleanup calls that unlink temporary, final, or digest-directory names, on success and on error. Do not introduce general generation garbage collection in this task.

This revises the fix1 plan's suggestion that hard-link publication plus temporary unlink was sufficient. That architecture requires exclusive directory ownership, which current staging does not establish. Native no-replace rename supplies the needed indivisible publication step without such a cleanup phase.

Define one private helper:

```go
func renameRunViewInputNoReplace(dirFD int, tempName, targetName string) error
```

Platform files contain only this wrapper and its required import:

| File / constraint | Implementation |
| --- | --- |
| `internal/container/craft_input_publish_linux.go`, `//go:build linux` | `return unix.Renameat2(dirFD, tempName, dirFD, targetName, unix.RENAME_NOREPLACE)` |
| `internal/container/craft_input_publish_darwin.go`, `//go:build darwin` | `return unix.RenameatxNp(dirFD, tempName, dirFD, targetName, unix.RENAME_EXCL)` |
| `internal/container/craft_input_publish_unsupported.go`, `//go:build !linux && !darwin` | Return an explicit unsupported-platform error, with no filesystem mutation. This does not promise the rest of this Unix-dependent package supports every OS. |

Never fall back to ordinary rename, copy-over-target, or `Linkat` plus pathname unlink. Unsupported kernel/filesystem errors propagate with `%w` and fail preparation; they do not select a weaker publication implementation. `EEXIST` maps to `craft.ErrConflict`, preferably wrapping the underlying errno as well. Preserve other errno identities, including `ENOSYS`, `EINVAL`, and `ENOTSUP`/`EOPNOTSUPP` where returned.

### API verification performed locally

`go.mod:356` and `go list -m golang.org/x/sys` both identify **v0.46.0**. No dependency update is necessary. The earlier fix1 report's isolated v0.47.0 probe is not this repository's version and should not be reused as evidence.

Verified with `rg` in `/Users/wuyongjun/go/pkg/mod/golang.org/x/sys@v0.46.0/unix`:

- `zsyscall_linux.go:1424` exposes `Renameat2(..., flags uint)`; `zerrors_linux.go:3139` defines `RENAME_NOREPLACE = 0x1`.
- `syscall_darwin.go:413` exposes `RenameatxNp(..., flag uint32)`; both Darwin amd64 and arm64 generated syscall files bind `renameatx_np`; both constant files define `RENAME_EXCL = 0x4`.
- Running `GOOS=linux GOARCH=amd64 go doc ./unix Renameat2` and `GOOS=darwin GOARCH=arm64 go doc ./unix RenameatxNp` **from that cached x/sys module directory** succeeded and printed those signatures.
- Apple SDK primary documentation `/Library/Developer/CommandLineTools/SDKs/MacOSX27.0.sdk/usr/share/man/man2/rename.2:139` specifies destination-exists rejection for `RENAME_EXCL`; lines 334-336 specify `ENOTSUP` for filesystem-unsupported flags. The SDK `usr/include/sys/stdio.h:53` declares `renameatx_np` available since macOS 10.12.
- Linux's maintained [rename(2) manual](https://www.man7.org/linux/man-pages/man2/rename.2.html) documents `RENAME_NOREPLACE`, filesystem support requirements, and unsupported-flag failure. No kernel support is inferred from cross compilation.

An initial repository-root `go doc` attempt was blocked by unrelated missing module resolution with `GOPROXY=off`; rerunning docs from the pinned cached dependency succeeded. This was API inspection, not a package-build success. Current host reports `darwin/arm64`, `CGO_ENABLED=1`; do not reuse the prior report's statement that host tests exercised Linux.

### Publication sequence and bounds

1. Create the random temporary entry with current exclusive/no-follow flags. Write verified bytes, set `0644`, sync the open file. Keep the existing before-publication seam and its target-race test.
2. `Fstat` the open file and preserve its device/inode; check canonical regular/single-link/mode invariants if not already guaranteed. Call the platform wrapper once. There is no preliminary target check needed for safety.
3. On syscall failure, close our descriptor and return `published=false` plus the primary error. Retain any temporary entry; it is an extra object that makes exact-tree retry validation fail. Close failure must not mask the primary cause (join errors if reported).
4. On syscall success, publication occurred: subsequent errors return `published=true`. Check final target with `Fstatat(..., AT_SYMLINK_NOFOLLOW)` against the open-file identity and canonical type/mode/link count. Preserve syscall errors; identity mismatches are conflicts. Sync the directory, close the file, then retain the existing final full-tree manifest/digest verification in `stageWorkspaceInputs`.
5. Never undo a successful rename by unlink or rename-back. A later sync, close, cancellation, or full-tree validation error still blocks execution and retains material.

The no-replace guarantee covers the target at the atomic syscall and eliminates the reviewed postpublication unlink race. It is **not** a general exclusive-writer guarantee: a writer can replace the temporary *source* before rename; rename may move that different inode to the target. The retained target/open-descriptor identity check must detect this and fail closed without deleting it. Include that adversarial case in tests. A stronger requirement that foreign content never even moves from its source name would require exclusive mutation authority/private source namespace and is outside this narrow fix. Likewise, mutation after a completed inspection cannot be ruled out solely by this helper; live mount/lifecycle ownership remains the separate gate already recorded in the parent plan.

## Digest-directory failure policy and real tests

Keep directories in place on helper failure. Explicitly make the existing subset-retry behavior the policy: a retained empty, canonical `0755` directory whose digest is admitted may be completed only after current authorization and exact-tree validation. A directory of another digest, a noncanonical mode/type, or any extra temporary/foreign entry rejects reuse. Do not claim all failures leave a recoverable directory: for example, chmod failure under a restrictive umask may leave a noncanonical directory that must stay blocked.

Split the current `Fstat` combined conditional: if the syscall fails, close the descriptor and return that error; if it succeeds but shape/mode is wrong, return conflict. After successful `Mkdirat`, all errors preserve `created=true`, return `fd=-1`, and close any successfully opened descriptor exactly once. Parent staging already checks error before using the fd; retain that ordering. Keep `Mkdirat` errors distinct (`created=false` except successful mkdir). Retained entries are deliberate evidence, not proof that cleanup succeeded.

Replace the test callback that removes the directory with per-call syscall injection at the real operation boundary. A small private ops struct passed to an internal helper can wrap `Openat`, `Fchmod`, `Fsync`, and `Fstat`; the ordinary helper supplies real functions. Avoid package-global function replacement and parallel-test races. `Mkdirat` stays real in these tests. Each table case returns a sentinel from the selected operation without deleting anything. Later operations must not run after a failure.

For each open/chmod/sync/stat failure assert the primary error remains reachable by `errors.Is`, `created=true`, `fd=-1`, the directory still exists, and a captured successfully opened descriptor is closed (check before opening anything that could reuse the fd). Add successful `Fstat` with wrong mode as a distinct conflict case. In particular, the stat-error test is behaviorally RED on current code because it returns only conflict.

Use a real staging fixture to prove retained canonical-empty-directory retry succeeds only by filling the admitted manifest and doing final validation. Add/reuse revoked-association retry and noncanonical/extra-object rejection checks. The first failed preparation must not call the executor. These assertions replace the incorrect no-orphan expectation; no test callback should perform production cleanup for the code under test.

## Deterministic RED interleaving

Introduce a narrow **per-call** after-publication test seam carrying `(dirFD, tempName, targetName)`, immediately after successful publication/identity verification and before the old unlink location. Keep it nil on ordinary calls; no global hook. The seam can be factored through a private hooks/options helper while retaining the existing before-publication helper signature. For the initial RED step leave fix2's `Linkat` and `Unlinkat` behavior intact.

In the callback:

1. If the temporary name exists, rename it to a distinct witness path outside the input tree but on the same filesystem. This models the other actor taking the original link away. In the new atomic-rename implementation the temporary name is already absent, which is an expected branch.
2. Exclusively create a different file at that exact temporary name and write distinctive bytes. Capture its inode/bytes. Return normally.
3. After the writer returns, assert the replacement temporary entry still has that inode/bytes. Fix2 deterministically fails this assertion by unlinking it. No scheduler sleeps or goroutine timing is needed.
4. In a staging fixture using this hook, verify final exact-tree inspection rejects the retained extra temporary entry and Prompt calls remain zero. In the old code unlink may hide that extra entry, which is an additional observable failure. The witness outside the input tree prevents a different extra-file failure from obscuring this assertion.

After GREEN, also prove an ordinary successful publication leaves exactly the target, single-link `0644` bytes, and no temporary entry. Preserve target-created-before-publication tests for regular file, symlink, and directory: no existing destination may be replaced. Inject an unsupported/error return from the actual publication-operation seam and assert no fallback, original error, retained temp, and no Prompt. A prepublication source swap must result in an identity conflict and no rollback deletion. Existing failure-after-first-file and dirty-retry tests remain relevant.

The new seam itself is test infrastructure; a missing-symbol compile failure is insufficient RED evidence for this repeated behavioral defect. Record the actual replacement-preservation assertion failing on fix2 with the seam added, then passing with rename publication.

## Exact implementation ownership and verification handoff

One serial `backend_implementer` task owns only:

- `internal/container/craft_runtime.go` and `internal/container/craft_runtime_test.go`.
- The three new `craft_input_publish_{linux,darwin,unsupported}.go` files listed above.
- `internal/container/craft_input_publish_test.go`, an optional small shared primitive test file with `//go:build linux || darwin`, using only standard library plus x/sys, so platform wrappers can compile independently of database/CGO dependencies.
- Its specifically assigned fix3 plan/checkpoint/report files. No `go.mod` / `go.sum`, material-provider, other assembly, T05, preview, migration, or Docker edits.

The implementer is not alone in this worktree and must preserve others' changes. Capture full preimages and newly added-file absence, then all final contents/hashes and an exact task delta. Backend validator is read-only; independent reviewer gives Spec and quality verdicts and examines the source-swap boundary as well as the two current findings. This design is not authorization to clean or commit files.

Run native focused tests with CGO enabled using the repository's working environment (commands below are planned, not executed here):

```sh
go test ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInput|TestOpenOrCreateRunViewInputDigestDir|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1 -timeout=120s
go test -race ./internal/container -run '^(TestStageWorkspaceInputsUsesOnlySelectedTaskSnapshot|TestRunViewInput|TestOpenOrCreateRunViewInputDigestDir|TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity))' -count=1 -timeout=120s
git diff --check -- internal/container/craft_runtime.go internal/container/craft_runtime_test.go internal/container/craft_input_publish_linux.go internal/container/craft_input_publish_darwin.go internal/container/craft_input_publish_unsupported.go internal/container/craft_input_publish_test.go
```

With the dependency-minimal primitive test file present, compile the **actual wrapper and tests**, not a handwritten surrogate and not a different x/sys version:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/craft-r3-fix3-publish-linux-amd64.test internal/container/craft_input_publish_linux.go internal/container/craft_input_publish_test.go
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -o /tmp/craft-r3-fix3-publish-linux-arm64.test internal/container/craft_input_publish_linux.go internal/container/craft_input_publish_test.go
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/craft-r3-fix3-publish-darwin-amd64.test internal/container/craft_input_publish_darwin.go internal/container/craft_input_publish_test.go
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go test -c -o /tmp/craft-r3-fix3-publish-darwin-arm64.test internal/container/craft_input_publish_darwin.go internal/container/craft_input_publish_test.go
```

Run the native Darwin primitive tests directly and the Linux test binary on a real available Linux environment/filesystem; cross-compiled binaries alone establish no kernel/filesystem behavior. Do not run foreign binaries on this Darwin host or call a database-dependent package build with `CGO_ENABLED=0` a valid full gate. Full Linux package tests need the established Linux CGO toolchain/dependency environment. Record missing runtime/build evidence as a limitation rather than a PASS. Format changed Go files and inspect the entire staging section to confirm no `Unlinkat`, ordinary rename fallback, or error-path cleanup was reintroduced.

Acceptance for this repair: deterministic temporary-replacement preservation, atomic target no-replace on Linux and Darwin wrappers, no fallback on unsupported systems, preserved root-cause syscall errors, explicitly tested retained-directory recovery, no failed/dirty preparation reaching Prompt, and independent review of the exact checkpoint. Whole R3/T01 completion and live isolation remain governed by the parent ledger.
