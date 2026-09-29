# Issue 140 Integrated UI Behavior Review Repairs R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement these tasks. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore documented keyboard, modal, navigation and save behavior lost in UI migrations present in the Issue #140 authorized integration range.

**Architecture:** Preserve TDesign visual styling while retaining native interaction semantics and established user flows. Use controlled, serialized persistence for full-configuration updates; use genuine TDesign overlays where the approved spec requires direct component use.

**Tech Stack:** React 19, TypeScript, TDesign React, Node test runner.

**Spec:** `docs/specs/2026-09-21-tdesign-react-migration-design.md`, `docs/specs/2026-09-23-weknora-job-search-design.md`, `CONTEXT.md`, Issue #140 DAG, and final range reviewer evidence recorded in `docs/plans/issue-140/final-review-addendum-2026-09-29.md`.

## Global Constraints

- Base all tasks on integration checkpoint `e7edfa72728c5d44940d9f145a0b5489089f4692`, an ancestor of original issue30-sweep BASE.
- Preserve existing product actions, links, modified-click behavior, focus, keyboard shortcuts, localization and component ownership.
- Do not replace required TDesign components with hand-written facades or weaken a11y semantics to achieve visual parity.
- Keep each task in a separate worktree; commits are local only.

## Review Focus

- Every clickable navigation/action item remains keyboard focusable and activates with Enter/Space as appropriate.
- Modal overlays keep focus within the modal, close on Escape, restore focus to the opener, and expose `aria-modal`.
- An explicit load-older action remains possible when the first history page fits without overflow.
- Zero-valued valid settings survive read, edit, save and reload; an older full-config response cannot overwrite a newer draft.
- Action errors/cancellation do not close a confirmation menu as though the action succeeded.

## Task 1: Restore document breadcrumb links and keyboard-operable document actions

**Dependency:** None.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/documents/DocumentsPageChrome.tsx`, `apps/web/src/documents/KnowledgeDocumentsPage.tsx`, relevant tests/CSS.

**Consumes / produces:** Existing `href` breadcrumb model and document action callbacks. Breadcrumbs must remain native anchors; action triggers/menu items must be native buttons or equivalent keyboard-operable controls.

**Steps:** Add tests for Enter activation, modified-click/open-in-new-tab, `aria-current`, keyboard menu opening/actions, and document action selection. Run failing cases. Restore anchor/button semantics without changing TDesign classes. Run focused Web tests and `git diff --check`; commit.

**Acceptance:** Mouse and keyboard/native browser link behavior both work; document actions remain reachable without a pointer.

## Task 2: Restore keyboard selection and modal focus semantics for knowledge-base scopes and shared drawer

**Dependency:** None; separate files from Task 1.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/knowledge-bases/KnowledgeBasesPage.tsx`, `SharedKnowledgeBaseDrawer.tsx`, focused tests.

**Consumes / produces:** Existing scope callback and drawer open/close props. Use native buttons for scope selection with `aria-pressed`. The drawer uses a `tdesign-react` accessible overlay or complete focus management: initial focus, Tab loop, Escape close, focus restoration and `aria-modal`.

**Steps:** Add keyboard tests for all scope choices and drawer focus lifecycle. Run RED. Implement with existing TDesign primitives/styles. Run focused tests, Web typecheck, and `git diff --check`; commit.

**Acceptance:** Keyboard users can choose each scope and cannot tab behind the open drawer; closing returns focus to the trigger.

## Task 3: Restore Agent Editor section navigation semantics

**Dependency:** None; owns only Agent Editor component and tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/agents/AgentEditorModal.tsx` and focused tests.

**Consumes / produces:** Existing section-selection callback. Keep new appearance but use buttons with `aria-current`/pressed semantics.

**Steps:** Add keyboard focus and Enter/Space section-change tests; run RED; restore button semantics; run focused tests and `git diff --check`; commit.

**Acceptance:** Every editor section is keyboard focusable and selectable.

## Task 4: Make Web chat confirmation callbacks single-step and failure-aware

**Dependency:** None; owns ChatHeader and ChatRoutePage only.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/chat/chat-header.tsx`, `apps/web/src/chat/ChatRoutePage.tsx`, focused tests.

**Consumes / produces:** Confirmation menu performs one confirmation and calls an action promise; callback resolves only on success and rejects on cancellation/API failure. Menu closes only after successful completion.

**Steps:** Add tests for accept, cancel, API rejection and one-confirmation behavior; run RED; remove nested `window.confirm`, propagate failure/cancellation, and close only on success; run targeted tests and `git diff --check`; commit.

**Acceptance:** No double prompt; failed/cancelled clear/delete leaves state and menu available for retry.

## Task 5: Serialize settings saves and preserve valid zero values

**Dependency:** None; settings components have disjoint files, but integrate together after task review.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/settings/PersonalMemoryPanel.tsx`, `ConfigSettingsPanel.tsx`, relevant tests.

**Consumes / produces:** Existing full-configuration update methods. Preserve draft generations; serialize writes and after each completion send the latest unsaved generation. Ignore stale responses when updating local draft. `extract_min_interval_seconds=0` remains a valid saved value.

**Steps:** Add reload-at-zero and out-of-order-update tests for both autosave paths. Run RED. Implement generation-aware serialized save queue and field-specific zero validation. Run focused settings tests and `git diff --check`; commit.

**Acceptance:** Zero is not replaced by default; changes made during an in-flight request are eventually persisted and older responses cannot overwrite the latest draft.

## Task 6: Restore Craft TDesign component and overlay behavior

**Dependency:** None; `packages/views/src/craft/td.tsx` is owned only by this task.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/views/package.json`, `packages/views/src/craft/td.tsx`, Craft call sites/tests as required.

**Consumes / produces:** Direct `tdesign-react` Button/Drawer/Dialog imports required by the approved TDesign migration spec §6; preserve existing host APIs at call sites. Remove handwritten `TDButton`/drawer/dialog facades. Overlay must support focus trap, native close button, Escape and focus restoration.

**Steps:** Add contract tests for the direct component import and keyboard overlay behavior. Run RED. Add supported dependency/host wiring and migrate call sites, keeping behavior in TDesign-owned primitives. Run Craft tests/typecheck and `git diff --check`; commit.

**Acceptance:** No handwritten TDesign adapter remains; controls and overlays retain required keyboard semantics and existing Craft behavior.

## Task 7: Restore explicit chat history loading and localized invalid-image text

**Dependency:** None; owns only chat message list/face and their tests.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `packages/views/src/chat/message-list.tsx`, `message-face.tsx`, relevant tests.

**Consumes / produces:** Existing `hasMore`, `loadingOlder`, `onLoadOlder`, and localized `copy.invalidImageLink` contracts.

**Steps:** Add test where first page fits without overflow and older history remains reachable; add non-Chinese invalid-image rendering test. Run RED. Restore explicit accessible load-older control and pass localized copy into markdown renderer. Run focused package tests and `git diff --check`; commit.

**Acceptance:** History remains loadable for underfilled viewports and invalid image fallback follows active locale.

## Task 8: Make document move destinations and mode choices keyboard-operable

**Dependency:** Follow-up to Task 1 review finding `UI-R2-DOC-01`; continue in the Task 1 worktree after its initial commit.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/documents/KnowledgeDocumentsPage.tsx`, focused document action tests, and narrowly scoped CSS only if needed.

**Consumes / produces:** Existing move destination and move-mode callbacks. Replace click-only destination and mode `div`s with native buttons and radio/checkbox controls as appropriate; preserve styling and selected state. Tests must focus a destination, invoke it through keyboard semantics, and assert that the existing callback receives the selected destination and mode.

**Steps:**

- [ ] Add failing interaction tests for keyboard selection of a destination and each move mode, plus the resulting callback payload.
- [ ] Implement native controls and accessible selected/checked state, preserving the menu's existing pointer behavior.
- [ ] Run the focused KnowledgeDocumentsPage tests, document package typecheck, and `git diff --check`; perform a browser keyboard smoke if the jsdom environment cannot emulate native button activation.
- [ ] Commit a repair on the Task 1 branch and report RED/GREEN evidence plus any browser-only evidence.

**Acceptance:** Every choice needed to finish a document move can be opened, selected and confirmed without a pointer.

## Task 9: Preserve Agent Editor section button layout and verify real keyboard semantics

**Dependency:** Follow-up to Task 3 review finding `UI-R2-AGENT-01`; continue in the Task 3 worktree after its initial commit.

**Role:** `frontend_implementer`; validator `frontend_validator`; reviewer `reviewer`.

**Files:** `apps/web/src/agents/AgentEditorModal.tsx`, focused Agent Editor tests, and the modal's scoped CSS only.

**Consumes / produces:** Native button section navigation from Task 3. Reset button-specific border/background/font/padding/width properties to the prior `.nav-item` appearance without removing focus indication or disabled/selected state. Add an interaction check using a keyboard-capable test helper when available; otherwise document that jsdom cannot synthesize browser default activation and run a real browser keyboard smoke.

**Steps:**

- [ ] Add a test for section focusability, selected semantics, and Enter/Space activation; capture the current style regression where supported by the test harness.
- [ ] Restore the full-width navigation appearance with scoped button reset styles while preserving visible focus.
- [ ] Run focused Agent Editor tests, Web typecheck, and `git diff --check`; perform browser keyboard/visual smoke if the unit harness cannot exercise native activation.
- [ ] Commit a repair on the Task 3 branch and report evidence.

**Acceptance:** Keyboard activation switches sections, and native button styling no longer changes the modal's established navigation layout.

## Parallelism and integration

Tasks 1–7 have non-overlapping production file ownership and isolated test/artifact paths; they may be implemented concurrently from the same integration checkpoint. Task 5 owns two settings files with no overlap elsewhere. Integrate by task number only after task-specific validation and independent review; then run the complete relevant Web and views test/typecheck gates. No task may edit the original issue30-sweep worktree.
