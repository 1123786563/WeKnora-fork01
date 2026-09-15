# Knowledge-base parity agent report

日期：2026-09-15

## 范围

本次只新增/修改 `apps/web/src/knowledge-bases/` 及本报告；没有修改共享组件，也没有回滚工作树中其他人的改动。Vue 参考为：

- `frontend/src/views/knowledge/KnowledgeBaseList.vue`
- `frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue`
- `frontend/src/components/ShareKnowledgeBaseDialog.vue`
- `frontend/src/views/knowledge/settings/KnowledgeBaseActivitySettings.vue`

## 已对齐行为

- 列表：保留后端数据，不注入 fallback；可区分 loading、空列表、普通错误和 `TENANT_FORBIDDEN`/`FORBIDDEN` 无权限分支。
- 编辑：覆盖 document/FAQ section 顺序、Vue 默认值、snake_case hydrate、显式 `false/0/空数组`、create/update payload 分界和表单依赖校验。
- 权限：viewer 只读；editor 可编辑但不能分享；owner/admin 可编辑和分享；未知权限不允许访问。
- 分享：组织与 permission 显式组装；空/成功/失败加载分支；mutation 在 pending 时去重，并保留失败信息。
- 活动：从数组、`data`、`items` 读取真实行；空响应不造数据；错误保留消息。
- 提交：同一 mutation pending 期间第二次提交复用首个 Promise；成功/失败均有明确终态。

## TDD 证据

先添加测试并运行：

```text
states.test.ts -> ERR_MODULE_NOT_FOUND: states.ts
list.test.ts   -> forbidden 断言失败（实现未保留 code）
share.test.ts  -> ERR_MODULE_NOT_FOUND: share.ts
```

随后实现最小行为并运行：

```text
node --import tsx --test apps/web/src/knowledge-bases/*.test.ts
22 tests, 22 pass, 0 fail
```

## 验证结果

- 目标目录定向测试：通过（22/22）。
- `pnpm --dir apps/web exec tsc --noEmit -p tsconfig.json`：通过。
- 完整 `pnpm --dir apps/web test`：68 通过、1 失败；失败在目标外 `apps/web/src/platform/legacy-session.test.ts`，现有实现未按测试预期保留 legacy refresh token。本任务未越界修改。
- `pnpm --dir apps/web build` 首次检查曾同时暴露目标外 `auth-state.test.ts` 隐式 any 与 `react-dom` 类型解析问题；本次目标目录自身类型错误已修正，最终以定向 `tsc --noEmit` 作为当前可复核证据。

## 尚未宣称的证据

本报告是知识库状态/契约层的 focused parity 证据，不等同于 Vue/React 浏览器截图、真实后端权限、Wails、iOS 或 Android 验收。React 页面入口仍由上层应用集成任务负责；本次遵守“仅修改知识库目录及专属测试”的边界。
