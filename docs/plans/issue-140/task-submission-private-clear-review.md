# Independent review — forbidden submission-history private clear

**Scope:** commit `c189ea5f4e641df43f87d719a27f4122f12846f6` against its parent, focusing on `SubmissionPage.tsx` and `SubmissionPage.test.tsx`. Reviewed the assigned prior finding in `task-submission-forbidden-race-fix2-review.md`, the approved job-search spec, ADR-0017, and `CONTEXT.md`. No OCR was run.

## Findings

No blocking findings in this commit.

The prior medium finding is fixed: the forbidden `applicationSubmissions` catch now calls `clearPrivate` (`SubmissionPage.tsx:106-110`). That function clears records, revision, exports, form fields, the loaded version detail and version message, and advances `privateReadGeneration` (`:81-86`). The history, revision, export, and version-review read completions compare their captured generation before updating state (`:104-107`, `:127-130`, `:144-150`, `:249-253`), so responses started before the forbidden result cannot restore private content. The mounted-pane regression loads a version and form, receives forbidden history, then renders another application in the same component and confirms the old version and form values are absent (`SubmissionPage.test.tsx:359-395`). A second test resolves a pending version review after forbidden history (`:397-420`).

**Nonblocking test limit:** the pane-reuse stub returns `record()` for both application IDs (`SubmissionPage.test.tsx:365-369`), so that test establishes private-state clearing and a ready timeline, but does not prove that the second timeline belongs to `app-2`. The component passes the current `applicationId` to the API (`SubmissionPage.tsx:103`); no production defect is indicated by this stub.

## Verdict

- **Spec compliance: approved for this fix.** The forbidden history path now enforces the private-content boundary required by the approved spec's personal-space access rule, including loaded material and composed form data across pane reuse. ADR-0017's immutable version/evidence binding is unaffected.
- **Code quality: approved for this fix.** The change reuses the established full-clear path and generation fence. Focused tests cover the reported state leak and a late version response. No critical, high, or medium issue was found in the reviewed delta.

## Independent verification

- `cd apps/web && node --import tsx --test --test-concurrency=2 src/career/SubmissionPage.test.tsx`: **19 passed, 0 failed** (exit 0).
- `cd apps/web && ../../node_modules/.bin/tsc -b --pretty false`: **exit 0**, no diagnostics.
- `git diff --check c189ea5f4^ c189ea5f4`: **exit 0**.
- Review did not modify application source or requirements; concurrent untracked reports were preserved.
