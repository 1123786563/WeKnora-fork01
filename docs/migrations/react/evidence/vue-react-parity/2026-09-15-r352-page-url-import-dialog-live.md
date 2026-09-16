# R352 — Page-level URL import dialog live parity

## Scope

Knowledge-base document detail, page-level `添加文档 → 导入网页` entry. This
round verifies the dialog opening contract and the empty-URL guard without
submitting a URL or changing backend data.

## Vue baseline

- Source: `frontend/src/views/knowledge/components/KbUploadSourceDropdown.vue`.
- The page-level source menu exposes `上传文档`, `上传文件夹`, `导入网页`, and
  `在线编辑`.
- In the authenticated fixture session, the Vue detail route was
  `http://localhost:5180/platform/knowledge-bases/22d38cb7-1fa5-48ed-8efc-be6f4f366640`.
- Chrome AX evidence showed the source menu text `导入网页`, followed by a
  dialog containing `URL地址`, placeholder
  `请输入网页URL，例如：https://example.com`, helper copy, and `取消`/`确认`.
- Clicking `确认` with an empty field kept the dialog mounted; no network
  mutation was attempted.

## React finding and repair

The page-level callback already set `sourceUrlDialogOpen`, but its dialog JSX
was nested under `uploadDialogOpen`. Therefore the page-level menu could set
the state without rendering a dialog. The dialog was moved to the page-level
render branch in `apps/web/src/documents/KnowledgeDocumentsPage.tsx`, while
the staged-file upload confirmation remains a separate dialog. The project
`Dialog` and `Input` wrappers retain the Vue-derived Tailwind tokens and
keyboard/outside-close behavior.

## React live evidence

- Authenticated route:
  `http://localhost:5181/knowledgeBase/22d38cb7-1fa5-48ed-8efc-be6f4f366640`.
- Chrome AX showed the same four source entries. Selecting `导入网页` produced
  a mounted `导入网页` dialog with `URL地址`, the same URL placeholder/helper,
  and `确认`/`取消` controls.
- Clicking `确认` with an empty URL left the dialog open, so the user remains
  in the validation context and no backend write occurred.

## Validation

- Focused upload/document contract tests: 37/37 passed.
- `pnpm run typecheck:web`: passed.
- `git diff --check`: passed.
- Evidence level: authenticated browser/AX interaction against the local
  backend fixture; no provider or production URL import was submitted.

