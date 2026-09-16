# N007 — graph extraction feedback toast

Date: 2026-09-14

## Vue baseline

`frontend/src/views/knowledge/settings/GraphSettings.vue` reports graph tag/text/relation extraction success and failure through TDesign `MessagePlugin` toasts. The feedback is outside the upload-confirm section and does not add an inline error block to the dialog layout.

## React implementation

`apps/web/src/documents/KnowledgeDocumentsPage.tsx` now routes graph extraction feedback, file-add feedback, and URL duplicate/add feedback through a page-level transient toast. The toast is top-centered, announced with `role="alert"`/`aria-live="polite"`, auto-dismisses after 3 seconds, and cleans up its timer on unmount. Graph success/example actions now use a distinct success tone, while file-added remains neutral and duplicate remains warning; this follows Vue `MessagePlugin.success`/`warning`/`error` semantics. Upload pipeline failures remain inline because Vue keeps those per-file/per-confirmation errors visible for correction.

The graph custom-instructions and sample-text fields also now use measured autosize bounds matching Vue `t-textarea`: 3–8 rows and 6–12 rows respectively, using the rendered line-height and vertical padding and enabling internal scrolling above the maximum.

The relation-type control now uses an accessible project field with multi-value chips, per-chip removal, clear-all, Enter/comma creation, and Backspace removal. This replaces the previous native multi-select and preserves the Vue `multiple + creatable + filterable + clearable` interaction shape.

The graph enable control now uses a project switch with Vue/TDesign-style checked track, sliding handle, focus-visible outline, and `role="switch"`/`aria-checked` semantics instead of the browser-default checkbox.

The sample-text field now exposes a live `current/5000` word-limit counter, matching Vue `show-word-limit` while retaining the Vue 5000-character cap and autosize behavior.

Relation rows now use project comboboxes for both entity endpoints and relation type. Entity options are filterable and keyboard-selectable; relation types additionally support creating a value, matching the Vue `t-select` configuration without retaining browser-native select styling.

The chunking strategy field now uses a Vue-shaped fixed-width single-select with selected/active states, ArrowUp/ArrowDown, Enter, Escape, and outside-click dismissal instead of a native `<select>`; its strategy values and update callback are unchanged.

The advanced chunking separator and language fields now use project multi-selects with Vue `multiple + creatable + filterable` behavior: chips, option filtering, Enter selection/creation, Backspace removal, Escape, and outside-click dismissal.

Parser engine rules now use the same project single-select surface as Vue `KBParserSettings.vue`: the default option clears the rule, unavailable engines remain disabled, and the existing per-file-type rule update contract is preserved.

Parser rows now follow Vue `KBParserSettings.vue` file-family grouping (Word/PPT/Excel/media and known text types) with extension chips and group-level rule updates. Excel + builtin also exposes the Vue first-row-as-header checkbox and round-trips `xlsx_first_row_as_header`.

Multimodal VLLM model, ASR model, and clearable image-description-language fields now use the project selector surface matching Vue `t-select`; empty required model states still use the existing form validation path.

Question generation now follows the Vue settings-row anatomy: description column, 88px count control (1–10), project switch, and an enabled-only instruction row with the Vue description/textarea structure.

The question count now uses a project numeric input matching Vue `t-input-number`: compact 88px field, decrement/increment buttons, min/max disabled states, clamped updates, native number keyboard editing, and explicit `aria-valuemin/max/now` values.

The same numeric input is now used for all five chunking numeric fields. The chunking fields keep their Vue-aligned 280px layout while preserving their individual ranges and steps: chunk size 100–4000/50, overlap 0–500/20, token limit 0–8192/64, parent chunk 512–8192/64, and child chunk 64–2048/32.

PDF scanned override, parent-child chunking, multimodal, and ASR enable controls now use a project switch matching Vue `t-switch`, including checked track/handle motion, focus-visible styling, and `role=switch`/`aria-checked` semantics.

Chunking controls now use a shared Vue-shaped `setting-row` layout with an information column and a 280px control column. The five chunk numeric controls use the Vue 200px field width, descriptions come from the existing Vue-aligned locale keys, and the row stacks at the mobile breakpoint.

Chunking, multimodal, ASR, and question sections now expose Vue-shaped visible title/description headers while retaining an accessible hidden fieldset legend.

Multimodal and ASR enabled fields, model selectors, language controls, and multimodal custom instructions now use the shared Vue-shaped setting-row information/control columns, including the existing localized descriptions and required/clearable behavior.

Parser file-family rows now match the embedded `KBParserSettings.vue` layout: bordered group container, 168px information column, 280px control column, compact 10px/14px row padding, monospace extension chips, and stacked controls below the mobile breakpoint.

Parser engine loading is now explicit: the React request tracks a loading flag and renders `settings.parser.loading`, while a successfully resolved empty engine list continues to render the empty-state message.

When a loaded file family has no available parser engine, React now renders the Vue warning state in the control column and exposes a parent-provided navigation action back to the parser settings route; no parser rule is written in that state.

Multimodal and ASR enabled fields, model selectors, language controls, and multimodal custom instructions now use the shared Vue-shaped setting-row information/control columns, including the existing localized descriptions and required/clearable behavior.

ASR language now uses a project clearable input with an explicit clear action, matching Vue `t-input clearable`; the existing language payload remains unchanged.

The relation-type combobox also exposes a clear action, while entity endpoint comboboxes remain non-clearable, matching the Vue per-field `clearable` configuration.

## Verification

- Focused upload-confirm and pipeline suites: 37/37 passed for the current graph/upload-confirm suite; prior upload-confirm and pipeline suites: 50/50 passed.
- Full Web suite: `pnpm run test:web` — 834/834 passed.
- Web typecheck: `pnpm run typecheck:web` — passed.
- `git diff --check` — passed.
- Browser extraction failure against a live graph-enabled backend remains unavailable; the current deployment has graph extraction disabled, so runtime endpoint acceptance is `blocked-env`.

Status: implementation and static/component regression verified; live graph extraction and cross-app screenshot evidence remain open.

## R027 model editor continuation

Vue `ModelEditorDialog.vue` uses TDesign inputs for dimension, context window, and max concurrency, plus TDesign switches for dimension override and vision support. React now uses project-level `ModelNumberInput` and `ModelSwitch` wrappers for those same fields. The wrappers preserve the existing draft update functions and numeric bounds while matching the Vue 32px input chrome, green focus state, disabled state, switch track/handle motion, inline descriptions, and keyboard-visible focus.

Verification: `pnpm exec tsx --test apps/web/src/settings/ModelSettingsPanel.test.tsx` 23/23; `pnpm run test:web` 835/835; `pnpm run typecheck:web` passed; `git diff --check` passed. This is static/component evidence only; authenticated browser computed-style, real backend mutation/connection evidence, and Wails/native evidence remain open under R027.

The React implementation now has a real Tailwind v4 + shadcn-style primitive layer in `packages/ui`: `cn`, `Input`, `Switch`, and `NumberInput`. The model settings and debug surfaces consume these shared primitives; the previous local number/switch chrome was removed. `typecheck:shared`, `typecheck:web`, and `build:web` pass, but this does not establish parity for pages that have not yet migrated to the shared layer.

`ModelOptionSelect` also now uses Tailwind utilities for the Vue-shaped trigger, popup, option states, selected indicator, and mobile width rule; its behavior remains covered by the model settings tests and the full Web suite (835/835).

The shared `@weknora/ui` Button, Card, and Status primitives now also merge Tailwind utility tokens through `cn` while keeping legacy semantic class names for unaffected pages. Shared and Web regression suites remain green; this is an incremental foundation, not whole-repository completion evidence.

R026 continuation evidence: `MemoryWorkspacePanel` now follows Vue `MemoryWorkspaceSettings.vue` auto-mode field visibility and setting-row anatomy, including model bindings, bounded delay/interval/threshold inputs, instructions maxlength, vector/conditioning switches, and max-items. Payload helper tests cover the extended fields and validation boundaries; Web regression remains 835/835. Runtime save/permission and browser/platform evidence remain open.

R026 continuation (N+79): workspace memory writes now use the Vue admin-role gate and a merged 500ms debounce, with timer cleanup on unmount. The shared model selector now exposes the Vue-equivalent clear action for embedding and add-model navigation entries for chat and embedding. Verification: Web 837/837, `typecheck:web`, `build:web`, and `git diff --check` pass. These are static/unit/build checks; authenticated browser computed-style, real backend success/failure/permission paths, and Wails/native evidence remain open.
