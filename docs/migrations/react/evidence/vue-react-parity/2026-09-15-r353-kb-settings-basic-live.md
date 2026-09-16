# R353 — Knowledge-base settings basic section live parity

## Scope

Authenticated edit-state comparison for the in-place knowledge-base settings
dialog opened from the KB list card menu. The comparison uses the same local
fixture KB, locale, account, and Chrome viewport on Vue `:5180` and React
`:5181`.

## Vue baseline

Vue `KnowledgeBaseEditorModal.vue` shows, in the Basic Information section:

- the knowledge-base ID and its API integration hint;
- the disabled document/FAQ type control for an existing KB;
- the indexing-strategy controls and existing-content immutability state;
- the description textarea with a `34/200` counter for the fixture value;
- the footer action `保存并关闭` and `取消`.

Live Vue AX evidence was captured from
`http://localhost:5180/platform/knowledge-bases/22d38cb7-1fa5-48ed-8efc-be6f4f366640`
after opening the settings button.

## React finding and repair

React's in-place dialog had the main fields and indexing controls, but omitted
the edit-only ID block and character counter, and used the generic `保存修改`
label. `apps/web/src/App.tsx` now renders the Vue-derived ID/hint block for
edit mode, caps the description at 200 characters with the live counter, and
uses `knowledgeEditor.buttons.saveAndClose` for the edit footer.

## React live evidence

Live React AX evidence from
`http://localhost:5181/platform/knowledge-bases` after opening
`Parity KB Demo → 设置` showed:

- `知识库 ID`, the API hint, and fixture ID;
- disabled `知识库类型` with value `文档`;
- indexing controls and the existing-content state;
- `知识库描述` with `34/200`;
- `保存并关闭` and `取消`.

The dialog remained open after HMR. A no-op save of the unchanged local
fixture was then submitted through the real authenticated backend path; the
dialog closed and the list reloaded, which is the React success transition.
No provider or production data was involved.

## Validation

- Live authenticated browser/AX comparison: passed.
- The change is implemented with existing project-owned Tailwind and
  shadcn-compatible Input/Textarea/Button/Dialog primitives.
- This demonstrates only the local authenticated success transition. Save
  failure, full mutation-field coverage, Wails, native, and production
  acceptance remain outside this slice and are not claimed here.
