# T05 / #124 fix round 1 independent re-review

Date: 2026-09-23. Read-only review of the six-file incremental checkpoint at `.superpowers/sdd/2026-09-23-craft-107-implementation/t05-fix1-checkpoint-01` in the T05 worktree, relative to HEAD `f11fb3e9831db6abd405f47e100e8c2a5f0cf94b`. The manifest SHA-256 is `8dbff4761bd1c650aee908b0d1194982a14a6ed2fb80f2ba26052f48d0146d88`; `tracked.diff` is `ad4a29173d579acce994cb8c14c4e3e2d32a6b06daadd1f66be74384b1791cd5`. All six current file hashes match the manifest. Sources: approved Craft web-artifact Spec, ADR-0004, `CONTEXT.md`, #124/T05 brief, first independent review, fix plan and implementation report. No OCR or tests were run in this review.

## Verdict

- **Spec compliance: FAIL.** The read-only Viewer and excerpt-truncation findings are corrected at the reviewed backend seam. The selected-only material boundary remains unproved and, with one persistent Workspace, the new directory still leaves prior Run material readable from the same root. T20 integration remains necessary for server Run binding and actual delegation.
- **Code quality: FAIL.** The retained material and mutable retry cases below affect confidentiality and consistency. The newly added sequential test inspects only the Run B prefix in a map of all workspace writes; it cannot establish that the delegate cannot read the sibling Run A path.

## Original findings revisited

1. **Viewer may build material — resolved at this seam.** `BuildForRun` now requests `craft.TaskWrite` before retrieval or writes (`internal/application/service/craft_knowledge.go:122-130`). `TestCraftT05BuildForRunRequiresTaskWriteBeforeSideEffects` rejects write while allowing read and asserts no staged bytes or record. A real admitted server Run association is still a T20 obligation; this service accepts any path-safe `runID` supplied by its caller.
2. **Earlier material remains available — unresolved.** `BuildForRun` writes to `knowledge/runs/<runID>` (`craft_knowledge.go:123,428,456`), but all paths are relative to the same persistent Workspace. It never removes or denies reads of `knowledge/runs/<priorRunID>`. The approved Spec requires one persistent Workspace and only selected, currently authorized knowledge to reach a Run (`craft-web-artifact-spec.md:57-62`). The fix report says the central worker *must* delegate exactly the new directory, but this checkpoint contains no capability, mount, root switch, or read restriction that makes that true. The test checks only paths beginning with Run B's prefix (`craft_knowledge_t05_test.go:145-163`) and does not attempt a read of Run A's existing file from Run B's execution context.
3. **Excerpt clipping undisclosed / cap exceeded — resolved for valid UTF-8 retrieval text.** `ExcerptOf` drops a partial trailing rune without adding bytes (`internal/modules/craft/knowledge.go:129-143`). The service marks a clipped source, `BoundSources` raises the bundle flag, and the bundle flag reaches the manifest and `KnowledgeRecord` (`craft_knowledge.go:403,422,134`; `knowledge.go:103-105`). The new ASCII and multibyte tests cover the hard byte cap and these facts. This conclusion does not claim that the test suite was independently run.

## Findings

### 1. High — a per-Run subdirectory is not a material isolation boundary

**Evidence / affected symbols:** `CraftKnowledgeService.build` writes old and new excerpts into sibling paths of the same `workspace` (`internal/application/service/craft_knowledge.go:304-314,426-460`); `KnowledgeRunDir` only computes a relative path (`internal/modules/craft/knowledge.go:38-49`). No cleanup or restricted delegate read root is established in the reviewed checkpoint. The new test sees Run A's staged file in `files` but verifies only the Run B prefix.

**Impact:** After Run A uses KB A and its grant is revoked or Run B selects only KB B, a delegate with normal read access to the persistent Workspace can open `knowledge/runs/run-a/<citation>.txt`. The prefix and B manifest do not prevent this. The first review's selected-only confidentiality finding therefore remains.

**Smallest defensible correction:** Establish and test an actual read boundary: stage the current selected bundle into a Run-specific read-only mount or separate sandbox input root that does not expose the persistent Workspace's prior `knowledge/runs` tree; alternatively remove the previous knowledge material tree before delegation with a failure-safe replacement protocol. Demonstrate a Run B delegate read of A's path is denied, including after revocation. T20 must bind the admitted server Run ID and enforce this before declaring #124 accepted.

### 2. Medium — reusing one Run ID can change staged bytes before insert-once rejects the record

**Evidence / affected symbols:** `BuildForRun` stages every excerpt and manifest in `s.build` before `records.Save` (`internal/application/service/craft_knowledge.go:130-139,426-463`). `KnowledgeRunDir` maps the same Run ID to the same path. A second call for the same Run with changed query or selected sources can overwrite an existing citation file or manifest, then fail the immutable record's different-payload check. No same-Run retry test covers material versus record consistency.

**Impact:** A failed/retried call can leave the original source record describing different material from what the delegate can read; a partial writer failure can similarly leave files with no matching record. This weakens the immutable actual-source evidence required by #124 and can expose material from a request that was rejected.

**Smallest defensible correction:** Before writing, resolve the Run's existing record and return or reject without touching files; for first build, stage into a fresh private location and publish only a complete package with a matching durable record. Test a same-Run call with different content and a writer/record failure, asserting unchanged visible files and record.

## Scope

The fix did not change central Run admission, delegation, source resolver, or frontend selection wiring. This re-review is a checkpoint judgment, not a conclusion about the final integrated branch. Independent backend validation and full-scope OCR remain controller gates.
