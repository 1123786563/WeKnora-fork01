# Independent review — submission forbidden race repair

**Scope:** commit `c9ef0147ccbb716dd7bca5b75fc1bd500eaba027` against its parent, limited to `SubmissionPage.tsx`, its test, and the task report. Reviewed the approved Career spec, ADR-0017, `CONTEXT.md`, the assigned brief, and the preceding independent finding F1. Read-only source review; no OCR was run.

## Findings

### F1 — Medium — Refresh can strand the profile revision read in loading

- **Evidence / affected symbols:** `SubmissionPage.tsx:121-134` starts `readRevision` only when its callback identity changes. `refresh` at `:158` advances `privateReadGeneration` but only changes `reload`, which does not trigger `readRevision`. If the user presses “刷新投递记录” (`:289`) while the initial `open` call is pending, its eventual success or error is ignored by the new generation guard (`:127,130`). `revisionState` remains `loading` and `revision` remains undefined, so confirmation remains disabled (`:262`) with no revision retry action (`:296` only appears after a revision conflict). The refresh button is available as soon as submission history has resolved, independently of `open`.
- **Impact:** A normal refresh can permanently prevent the user from confirming a submission in that mounted pane. This is a liveness regression introduced by invalidating a read that refresh does not restart.
- **Smallest correction:** Restart `readRevision` on `reload`, or avoid invalidating its pending result on a refresh that does not restart it. Add a deferred `open` test: resolve history, refresh while `open` is pending, then resolve `open` and verify the revision and confirmation path recover.

### F2 — Medium — In-flight version review bypasses the private read fence

- **Evidence / affected symbols:** `reviewVersion` at `SubmissionPage.tsx:243-256` starts a `materialVersion` private read, but its success and error callbacks check only `scopeController.isCurrent`. A forbidden export in the same scope calls `clearPrivate` (`:81-86,151`), advancing the generation and clearing `versionDetail` and `versionMessage`. A later `materialVersion` response can repopulate either field. The forbidden render branch (`:266-309`) currently hides the repopulated state, but the state clear is not stable and a subsequent pane reuse or prop change can expose it.
- **Impact:** The requested guarantee that stale private reads cannot undo a forbidden clear is incomplete. The current render gate prevents immediate display, but private material content can return to component state after access revocation.
- **Smallest correction:** Capture `privateReadGeneration.current` when `reviewVersion` starts and check it before applying both success and error state. Add a deferred version-review response test that rejects exports as forbidden first, then resolves the version read and confirms the private review stays cleared.

## Prior finding and verdict

The preceding F1 ordering for `applicationSubmissions` is **fixed**: its success and error callbacks both compare the captured generation (`SubmissionPage.tsx:98-112`); forbidden exports advance that generation before clearing state (`:81-86,149-152`). The same check covers `open` and export callbacks. The new deferred regression exercises a late history success (`SubmissionPage.test.tsx:335-357`). Late history errors and late revision results are not directly tested, although their callbacks use the same guard.

**Spec compliance: not approved.** The targeted timeline redisplay is prevented, but F2 leaves a private material read able to repopulate cleared state, contrary to the personal-space privacy boundary in the approved spec and the task's full private-read fencing objective.

**Code quality: not approved.** F1 is an observable refresh regression; the focused tests do not cover refresh while revision loading or an in-flight version review during forbidden clearing.

## Independent verification

- `cd apps/web && node --import tsx --test --test-concurrency=2 src/career/SubmissionPage.test.tsx`: **15 passed, 0 failed** (exit 0).
- `cd apps/web && ../../node_modules/.bin/tsc -b --pretty false`: **exit 0**, no diagnostics.
- `git diff --check c9ef0147ccbb716dd7bca5b75fc1bd500eaba027^ c9ef0147ccbb716dd7bca5b75fc1bd500eaba027`: **exit 0**.
- The worktree had pre-existing unrelated modified and untracked files; this review changed only this report.
