# T01 R4 Task 2a Fix1 — independent Spec and quality review

Reviewed 2026-09-24. Scope: `internal/container/craft_runtime.go` and `internal/container/craft_runtime_r4_run_source_test.go` at the declared Fix1 checkpoint, against the approved Craft web-artifact Spec, ADR-0004/0008/0009, `CONTEXT.md`, the Task2a/Fix1 plans, and the preceding Task2a review. No source, test, requirement, issue, or commit was changed.

## Checkpoint and verification

- Both current owned files match the checkpoint SHA-256 postimages: `f1acfa7b3b5fabbc0299ff2a9298040514e08ffe688997dd87195d881f47fee1` and `c8a05a8fb68a00d4cf1bdb0ba9cb5dc9a106a895a05c07b479a9993cf82146d3`. Report and patch hashes also match the JSON. Applying the Fix1 task-local patch to a temporary copy of current HEAD reconstructs both postimage hashes.
- The Fix1 patch is a full HEAD-to-postimage patch, not an isolated prior-Task2a-to-Fix1 delta. The earlier Task2a patch applied to current HEAD reconstructs test hash `73a4…4489` but runtime hash `5fa7…70f0`, not its declared runtime checkpoint hash `9c1d…f7a1`; the exact prior runtime postimage was not supplied as a file. Thus the two patch files alone do not independently reproduce the claimed Fix1 increment. The current postimage itself is authenticated by its hash; this is a checkpoint provenance limit, not a separate behavioral finding.
- Independently reran `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` and the same command with `-race`; both passed. Both emitted the duplicate `-lc++` linker warning. Full package and Docker acceptance are outside this narrow review.

## Finding F1 — Medium — mutations in an earlier or sibling nested directory can be omitted

**Evidence / affected symbols:** `walkRunBoundArtifactTree` validates each directory's names and metadata only at that directory's recursion return (`craft_runtime.go:1779-1791`). `ListSessionFiles` returns after the root call without revisiting every recorded nested directory (`craft_runtime.go:1645-1655`). With root children `a/` and `b/`, a writer can add `a/late.js` while `b/` is being scanned, after `a/` passed its final check. Root's immediate names and metadata do not change, so List can return a manifest missing `a/late.js`. `readRunBoundArtifactAt` verifies only the root and ancestors of the requested file (`craft_runtime.go:1911-1939,1973-1997`); after List, inserting/removing/renaming a child under `a/` and then reading `output/b/index.html` can return success while the recorded `a/` entry set is stale. The new tests exercise root insertion and mutation in the directory of the file being read, but not a sibling or previously visited nested directory (`craft_runtime_r4_run_source_test.go:218-266`).

**Impact:** The source may report a complete list or accept bytes from a mixed output tree while an omitted or changed nested entry exists. This violates the Fix1 plan's explicit all-directory list/read requirement and can silently change what Task2b collects if it consumes the source without a proven terminal writer fence.

**Smallest defensible correction:** Before List succeeds and before each Read returns bytes, re-open and compare every listed directory's identity and exact immediate name set, with deterministic tests that mutate an earlier nested directory while a later sibling is walked and mutate a sibling between List and Read. The terminal/quiescent writer fence must be established at Task2b's collection boundary and held through list and reads: repeated metadata and name checks cannot make a recursively writable POSIX tree an atomic snapshot. Without that fence, return unresolved rather than treating a successful scan as sealed.

## Other reviewed behavior

The new `ReadDir(limit+1)` path allocates at most 1,025 directory entries per enumeration before checking overflow (`craft_runtime.go:1667-1679,1794-1811`); the 4,096 total count includes directories and zero-byte files. Existing path, no-follow, device/inode, link-count, size and digest checks still protect the requested listed file. Direct root changes and changes in the read file's ancestor directory are checked. Same-size replacement of the requested file is detected by inode or digest; a sibling file changed after its own read still requires the terminal quiescence fence to prevent a mixed collection.

## Verdict

**Spec compliance: conditional / Fix1 not yet accepted.** The entry allocation cap closes the prior resource-exhaustion finding. The exact output-set guarantee remains unproven for sibling and previously visited nested directories (F1), and Task2b must provide the terminal writer fence before this source is used for a candidate or draft.

**Code quality: changes required.** Focused and race tests pass, and no critical/high issue was identified in the two-file increment. F1 is a medium data-consistency gap with missing behavioral tests. No conclusion is made about Task2b, version promotion, real Docker isolation, or the end-to-end Craft acceptance scenario.
