# Independent review — Career Web medium round-4 repair

**Scope:** commit `535c414db17d5812daa5907793b80b90836f8195` against its parent; `MaterialPage.tsx` and test, `SubmissionPage.tsx` and test. Reviewed the prior medium repair review, approved Career spec (`docs/specs/2026-09-23-weknora-job-search-design.md`), ADR-0017, `CONTEXT.md`, Issue 140 snapshot, and Task 4 repair plan. This is a read-only code review; no OCR was run.

## Finding

### F1 — High — A late submission read can undo a forbidden export clear

- **Evidence / affected symbols:** `SubmissionPage.tsx:93-114` starts `applicationSubmissions` on every `reload` and its success handler unconditionally calls `setRecords(next.submissions); setReadState('ready')` while the request remains active and the scope is current. The export read started separately at `:132-151` calls `clearPrivate(...)` on `forbidden` (`:146`), which sets `readState='forbidden'`, clears records and form, but does not invalidate the pending submission read. If the export request rejects first and the submission request then resolves, the latter sets `readState='ready'` and repopulates `records`. The render at `:261-266` then displays the returned private timeline and form. The added forbidden test (`SubmissionPage.test.tsx:318-335`) resolves the submission read before the forbidden export refresh, so it cannot catch this ordering. A concurrent late `open` response can also repopulate `revision` (`:118-129`), although it alone cannot reveal the pane.
- **Impact:** Revoked material access can be followed by redisplay of private submission content in the same pane. If the export remains unset, confirmation is blocked, but the privacy state required by the Task 4 forbidden handling is not stable. This conflicts with the spec's personal-space privacy boundary and the issue's scope-switch/late-response acceptance.
- **Smallest correction:** On a forbidden export result, invalidate or cancel the sibling submission and revision reads for that refresh (or make their completion handlers check a latched forbidden state). Clear the private state after invalidation. Add a deferred-promise test that lets `materialExports` reject with `forbidden` before `applicationSubmissions` resolves, then asserts the forbidden view and absence of the timeline/form after the late response.

## Prior findings and verdicts

- Prior F1 (unread material overwrite): **addressed in the rendered flow.** `MaterialPage.tsx:524-527,543-569` disables editor and save/confirm while `restoreFailed`; the new test attempts those actions before retry and observes no edit call. The stable URL pointer and retry remain available.
- Prior F2 (forbidden export handling): **partially addressed.** `SubmissionPage.tsx:80-85,144-147` clears private state and shows the forbidden view for the tested response ordering. F1 above leaves a concurrent ordering unsafe.
- Prior F3 (stale export selection): **addressed.** `SubmissionPage.tsx:140-143` clears a vanished choice; `:166-173,254-257` also checks membership before enabling or sending a bound submission. The new refresh test covers removal of a selected export.

**Spec compliance: not approved.** The material restore and stale export changes meet the reviewed requirements, but the forbidden-state race leaves the personal submission pane capable of redisplaying private content after access revocation.

**Code quality: not approved.** The new tests pass but omit the response ordering that breaks the new clear. No other blocking finding was identified in the locked four-file scope.

## Independent verification

- `node --import tsx --test --test-concurrency=2 src/career/MaterialPage.test.tsx src/career/SubmissionPage.test.tsx` from `apps/web`: **37 passed, 0 failed** (exit 0).
- `../../node_modules/.bin/tsc -b --pretty false` from `apps/web`: **exit 0**, no diagnostics.
- `git status --short` after checks showed only the pre-existing untracked planning/review files; no tracked source changes from this review.
