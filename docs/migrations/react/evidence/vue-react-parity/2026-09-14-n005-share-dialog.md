# N005 · KB 共享弹窗局部对齐

- 日期：2026-09-14
- Vue 基线：`frontend/src/components/ShareKnowledgeBaseDialog.vue` 使用 520px 对话框；组织选项显示头像、角色和成员/知识库/智能体计数；权限提示位于表单下方；footer 分离取消/确认；已共享项显示头像、权限标签、设置入口和取消共享。
- React：`apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.tsx` 与 `apps/web/src/styles.css`。

## 已验证

- 组织权限过滤、已共享组织排除、共享/取消共享 API payload、确认和提交防重行为保留并通过回归。
- 新增 Vue 风格自定义组织选择器（头像、角色、成员/知识库/智能体计数、选中态、外部点击关闭和 Escape），并将权限从原生 `<select>` 改为 Vue `t-radio-group` 对应的可访问 radio-button group；组织信息预览、权限提示/footer、共享行头像/权限色签和组织设置导航入口保持不变；专测 13/13。
- 组织选择器补齐 Vue 选择器的 ArrowUp/ArrowDown active option 与 Enter 选择语义，仍支持 Escape/外部点击关闭；专测保持 13/13。
- Web 全量回归、浏览器与真实后端证据仍需补齐；本切片定向测试 13/13，`typecheck:web` 通过。

## 未覆盖

- 原生 `<select>` 的浏览器下拉弹层仍未达到 Vue TDesign 自定义选项的完整头像/徽标呈现；当前在已选项预览中补齐信息，需认证浏览器进一步复核。
- 同条件 Vue/React 截图、真实后端共享链路、Wails、iOS、Android 证据仍缺失；因此 N005 继续为 `implementing`。
