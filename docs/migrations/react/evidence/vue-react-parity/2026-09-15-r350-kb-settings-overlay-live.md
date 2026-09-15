# R350 — Knowledge-base settings overlay live parity

- Runtime: Chrome, same 1160×~660 content viewport, zh-CN, Vue `http://localhost:5180` and React `http://localhost:5181`, authenticated with the local parity account in tenant `10001`.
- Fixture: `Parity KB Demo` and one manual document were provisioned in the local test backend only because the pre-existing parity account was absent from this backend instance. No Vue source or production data was changed.

## Baseline and discrepancy

- Vue list → card settings opens an in-place `KnowledgeBaseEditorModal.vue`: the knowledge-base page remains visible behind a dimmed/blurred backdrop, the editor has the Vue section rail, and the URL remains the knowledge-base list/detail context.
- React previously sent the settings action to `/knowledgeBase/<id>/settings`, rendering a full-page settings surface. This differed in overlay behavior, focus context, and navigation semantics even though the settings controls were present.

## Repair

- `apps/web/src/App.tsx` now routes the list card settings action and the uninitialized-card settings branch through the existing project-owned `Dialog` editor (`openEdit`) instead of navigating away.
- The dialog title uses the Vue settings label for edit mode; section-rail buttons explicitly reset native button borders/background so Tailwind styling is not replaced by browser defaults.
- The historical `/knowledgeBase/<id>/settings` route remains available for deep-link callers; the primary list interaction now matches Vue.

## Live evidence

- Vue AX tree: list → card → settings exposed `知识库设置`, `关闭设置`, the grouped section rail (`基本信息`, `模型配置`, `向量存储`, `解析引擎`, `分块设置`, `图像处理`, `音频处理`, `知识图谱`, `高级设置`, `存储引擎`, `数据源`, `共享管理`, `活动记录`) and the Vue basic form.
- React AX tree after repair: list URL remained `/platform/knowledge-bases`; settings action exposed a dialog named `知识库设置`, with the same grouped sections, `保存修改`, and `取消`.
- Same-window screenshots were captured in the live CUA transcript before and after repair. The post-repair React screenshot shows the list dimmed behind the centered editor overlay rather than a standalone settings page.
- Opening the editor was read-only; no save, delete, share, datasource, or document mutation was submitted.

## Verification boundary

- Focused KB anatomy tests: 18/18 passed.
- `git diff --check`: passed.
- This round proves the list settings-entry overlay/navigation behavior and AX structure only. It does not prove save success/failure, provider-backed datasource/share mutations, Wails desktop rendering, or native mobile behavior.
