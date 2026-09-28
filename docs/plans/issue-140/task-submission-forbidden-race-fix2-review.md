# Independent review — submission forbidden race follow-up

**Scope:** `fd2ee0a20d126af584ce55413e74c4a643e26614` against its parent; `SubmissionPage.tsx`, its tests, and the assigned task report. Checked the prior review, the approved Career spec, ADR-0017, and `CONTEXT.md`. Read-only source review; no OCR was run.

## Finding

### F1 — Medium — Forbidden submission history hides but does not clear an earlier material review

- **Evidence / affected symbols:** `SubmissionPage.tsx:106-112` handles a forbidden `applicationSubmissions` response by setting only `records`, `readState`, and `readMessage`. Unlike the other forbidden paths (`:151`, `:194`, `:238`, `:255`), it does not call `clearPrivate` (`:81-86`). If the user first opens a bound version (`:243-258`), then refreshes history and receives forbidden, `versionDetail` remains in component state. The forbidden render branch (`:266`) hides it. When the same mounted pane changes to an accessible `applicationId`, the history effect (`:95-117`) can set `readState='ready'` without clearing `versionDetail`; the earlier application's material is shown under the new application's pane. The new test at `SubmissionPage.test.tsx:394-425` exercises a forbidden **export** result, not this history path.
- **Impact:** A forbidden history response does not establish the promised private-state clear. A subsequent prop change can reveal material from the inaccessible application, conflicting with the personal-space privacy boundary in the approved spec. This path predates `fd2ee0a20`; the follow-up commit does not introduce it.
- **Smallest correction:** Route forbidden submission-history results through `clearPrivate`, or perform the same complete private-state clear while preserving the forbidden notice. Add a test that loads a version review, returns forbidden on a later history read, then reuses the mounted pane for another accessible application and asserts that the old review and form values remain absent.

## Prior findings and additional checks

- **Prior F1 (refresh strands revision loading): fixed.** The revision effect now depends on `reload` (`SubmissionPage.tsx:121-134`), so refresh restarts `open` in the new private-read generation. The deferred test at `SubmissionPage.test.tsx:359-392` proves the late old revision cannot settle loading and the refreshed revision is used for confirmation.
- **Prior F2 (late material review restores cleared state): fixed.** `reviewVersion` captures and checks `privateReadGeneration` before both success and error updates (`SubmissionPage.tsx:243-256`). The test at `SubmissionPage.test.tsx:394-425` resolves the review after forbidden exports and reuses the pane; no material review appears.
- **Late history and revision responses:** Their success/error callbacks compare the captured generation with the current generation (`SubmissionPage.tsx:98-112,121-132`), so a forbidden export clear cannot be undone by these pending reads. The earlier late-history success test remains in place (`SubmissionPage.test.tsx:335-357`).
- **Scope changes:** `scopeController.isCurrent` fences pending private reads; the abort listener clears private state (`SubmissionPage.tsx:88-93`). A new scope generation restarts history and export reads, but does not automatically restart `readRevision` because its effect depends only on `readRevision` and `reload` (`:134`). The user can invoke refresh after history loads to restore the revision. This is a pre-existing recovery gap, not a regression in this commit; it has no direct evidence of cross-scope data disclosure because stale completions are fenced.

**Spec compliance: not approved.** The two assigned race findings are corrected, but F1 leaves the forbidden private-state boundary incomplete in the same component.

**Code quality: not approved.** The new tests meaningfully cover the two assigned response orderings, but the forbidden-history path lacks a behavioral regression that exercises pane reuse after clearing.

## Independent verification

- `cd apps/web && node --import tsx --test --test-concurrency=2 src/career/SubmissionPage.test.tsx`: **17 passed, 0 failed** (exit 0).
- `cd apps/web && ../../node_modules/.bin/tsc -b --pretty false`: **exit 0**, no diagnostics.
- `git diff --check fd2ee0a20^ fd2ee0a20`: **exit 0**.
- Tracked source files remained unchanged by this review. Other untracked reports in the shared worktree were preserved.
