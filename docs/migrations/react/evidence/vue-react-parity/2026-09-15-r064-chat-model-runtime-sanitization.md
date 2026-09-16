# React/Vue parity evidence — runtime model sanitization

Date: 2026-09-15

## Browser evidence

Authenticated React chat at `http://localhost:5181/platform/creatChat` was checked after the model-option sanitization update. The tenant's malformed model row no longer produces a blank `<select>` option (`select[aria-label="对话模型"]` count: 0), and the visible model chip resolves to `mock-stream-model200K`.

## Validation

- Browser DOM inspection confirmed the blank option is absent and the localized model chip remains visible.
- Web tests: 895/895 passed.
- Web typecheck and production build passed in the same change window.

## Limitation

This tenant still does not expose two valid selectable KnowledgeQA models, so multi-model selection and persistence remain fixture-gated.
