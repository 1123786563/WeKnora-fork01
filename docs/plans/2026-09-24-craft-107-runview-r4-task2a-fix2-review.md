# T01 R4 Task 2a Fix2 — independent Spec and quality review

Reviewed 2026-09-24. Scope: the exact two-file Fix1-to-Fix2 increment in `internal/container/craft_runtime.go` and `internal/container/craft_runtime_r4_run_source_test.go`. Fact sources: the approved Craft web-artifact Spec, ADR-0004/0008/0009, `CONTEXT.md`, the Fix2 plan and brief, Fix1 review, and the saved Fix2 checkpoint. No source, test, requirement, issue, or commit was changed.

## Checkpoint and verification

- Saved preimages match the declared Fix1 SHA-256 values (`f1ac…fee1`, `c8a0…2146`); saved postimages and current owned files match the Fix2 values (`42f2…84ca`, `4a05…8098`). Report and patch hashes match the checkpoint JSON. Applying the task-local patch to a temporary copy of the saved preimages reproduces both postimage hashes exactly. HEAD remained `a5e9195acd6500c085c85d60c852148e7bbbbf34` during the review.
- Independently ran `go test ./internal/container -run '^TestCraftRunViewR4' -count=1` and `go test -race ./internal/container -run '^TestCraftRunViewR4' -count=1`; both passed. Both emitted only the duplicate `-lc++` linker warning. The implementer's reported RED run, compile-only command, formatting, and diff check were not independently rerun.
- `ListSessionFiles` validates the complete recorded directory tree after enumeration and Run/material binding, before publishing `s.listed` (`craft_runtime.go:1647-1658`). `readRunBoundArtifactAt` validates the same set before reading and again before returning bytes (`craft_runtime.go:1917-1921,2000-2005`). The recursive verifier begins at root, visits every recorded nested directory, and compares each immediate child set and identity (`craft_runtime.go:2067-2159`), so a previously visited nested directory or unrelated sibling is included. The new deterministic tests cover insertion during later sibling traversal and sibling insertion/removal/rename between List and Read (`craft_runtime_r4_run_source_test.go:269-315`).
- Enumeration still uses `ReadDir(limit+1)`, with at most 1,025 entries allocated per directory read and a cumulative 4,096-entry limit. Descents use descriptor-relative `Openat` with `O_NOFOLLOW` and `Fstatat` with `AT_SYMLINK_NOFOLLOW`; the existing listed-file canonicality, link-count, size, and digest checks remain. The final directory name/stat recheck detects observed drift during the verifier itself.

## Finding F1 — Low — sibling same-name replacement has no direct regression test

**Evidence / affected symbol:** The Fix2 plan's Review Focus includes same-name replacement. `verifyRunBoundArtifactDirectory` compares each child's recorded device/inode/type/size at `craft_runtime.go:2111-2138`, which should reject a replaced sibling even when its name and bytes match. The new sibling test table at `craft_runtime_r4_run_source_test.go:290-314` covers insert, remove, and rename only. The older replacement test at lines 82-145 replaces the requested file or its ancestor, not an unrelated sibling.

**Impact:** No demonstrated production defect; the sibling replacement branch lacks a direct behavioral regression guard, making a future weakening of per-child identity validation less likely to be caught by focused tests.

**Smallest defensible correction:** Add one sibling case that atomically replaces `other/sibling.txt` at the same name after List, then asserts that reading unchanged `assets/app.js` returns `craft.ErrConflict`. This is a low-risk test follow-up and does not block acceptance of this increment.

## Boundary and verdict

The verifier detects observed changes; repeated POSIX walks do not provide an atomic recursive snapshot. A writer could mutate an already rechecked directory before the overall List/Read returns. Task2b must hold the approved authoritative terminal/quiescent writer fence throughout candidate/draft collection. This is an explicit downstream requirement, not a Fix2 defect or a claim that this source alone seals a live tree.

**Spec compliance: PASS for the assigned Fix2 increment, conditional for end-to-end collection.** The Fix1 Medium gap is closed for root, nested, and sibling directory entry-set and identity changes at the stated List/Read boundaries. Candidate/draft acceptance still depends on Task2b's terminal writer fence.

**Code quality: PASS with one Low test follow-up.** No critical, high, or medium finding in the two-file increment. Focused and race tests pass; no full package, Docker, or end-to-end Craft acceptance conclusion is made here.
