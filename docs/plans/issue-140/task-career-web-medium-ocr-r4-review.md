# Independent Task 4 Career Web medium repair review

**Locked scope:** `50932d609f3a81666ec6efd5d6ae8ba8ebefae64` against parent `7f40e44e775cf180f9617a378e8d1cb236f63ad8`; changed Task 4 files only. Reviewed the approved Career spec, ADR-0017, `CONTEXT.md`, Task 4 plan, implementer report, and the round-4 high-risk/resume incremental analyses. This was a static review; no test or OCR command was run.

## Findings

### F1 — High — Unread material can be overwritten after URL restore fails

- **Evidence / affected symbol:** `MaterialPage.tsx:182-186` sets `materialId=stored` after any non-forbidden read error, without loading `savedBody` or `sections`. The editor remains enabled in `phase='error'` (`:526-527`, `:543-568`). A user can add a section, then `saveDraft` sends `{ materialId, body: newly entered sections }` (`:204-210`). The backend `EditMaterial` replaces `draft_body` with that body (`internal/modules/career/material.go:390-416`). The added URL test (`MaterialPage.test.tsx:608-622`) only checks retry and pointer preservation.
- **Impact:** A transient read failure followed by a plausible edit replaces the existing structured draft the user could not inspect. The request carries the latest *profile* revision, not a loaded material-body revision, so the server does not reject this as a stale edit. This violates the spec's trustworthy versioned material and no silent overwrite intent.
- **Smallest correction:** Keep the URL pointer separately from the editable `materialId`, or gate editor/save until a successful read has established the current body and matching pinned evidence. Add a regression test that tries to add and save before retry succeeds, and assert no `editMaterial` call; retain the pointer and allow retry.

### F2 — Medium — Forbidden export-list response is treated as a retryable network error

- **Evidence / affected symbol:** `SubmissionPage.tsx:141-145` catches all `materialExports` failures without `errorDetails` and exposes only a generic retry. `clearPrivate` at `:80-84` does not clear `exports`, revision, or export-error state. The source finding N14 in `round4-resume-increment-analysis.md` explicitly requires a forbidden branch that clears private state. The new test at `SubmissionPage.test.tsx:297-315` covers only HTTP 503.
- **Impact:** If access to the selected material is revoked while its submission pane is open, prior submission records and composed private content stay visible. The UI invites a retry as though access were temporary. It does block confirmation while `exports` is undefined, so this is an authorization-state and privacy defect rather than a false successful write.
- **Smallest correction:** Parse the export-list error; on `forbidden`, clear the pane's private state, including export/revision state, and show the existing forbidden view. Keep the retry path for transient failures. Test forbidden after a previously loaded record and composed form.

### F3 — Medium — A stale export choice can be submitted after the list changes

- **Evidence / affected symbol:** `SubmissionPage.tsx:134-140` refreshes `exports` but retains `versionChoice`. When a refreshed list no longer contains the selected export, `submitBlocked` at `:250` checks only that exports is defined and choice is nonempty. `runWrite` at `:165-167` sends that stale `exportId` with `materialId: undefined`. The new test exercises failure then an empty-list success with a newly chosen unknown value, not a previously selected export disappearing on refresh.
- **Impact:** The pane permits a confirmation request for an export that is no longer in the verified deliverable list. The backend should reject it, but the UI no longer enforces Task 4's fail-closed export-selection requirement and can put the user into an avoidable error/recovery path.
- **Smallest correction:** When an export read succeeds, clear a selected export ID absent from the new deliverable list; also require the selected ID to resolve to a current export before enabling or constructing the write. Add a refresh test where a previously selected export disappears.

## Verdict

**Spec compliance: not approved.** The changes correctly keep a verified material receipt successful when its follow-up read fails, preserve the URL pointer for transient restore failures, block the unknown-version option while export availability is unknown, and terminate the reviewed receipt mismatches and `not_found` imports. F1 violates the material-edit safety requirement; F2 and F3 leave the Task 4 export-selection and forbidden-state requirements incomplete. The plan's URL import receipt identity item is assigned to SearchPage/Task 2 and was outside this commit.

**Code quality: not approved.** The focused new tests cover the happy recovery transitions but omit the failing state transitions above. Existing implementer evidence reports 99 focused tests and web typecheck passing; this review did not rerun them. No source or requirement files were modified by this reviewer.
