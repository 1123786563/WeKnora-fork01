# N007 上传配置区段显示证据（2026-09-14）

## Vue 基线

`frontend/src/views/knowledge/components/UploadConfirmDialog.vue` 对 tags、parser、chunking、multimodal、asr、question 和 graph 使用 `v-show="activeSection === ..."`；切换区段时保留表单 DOM 与状态，只隐藏非当前区段。

## React 修复

`UploadConfirmSections` 新增可选 `activeSection`，对每个配置 fieldset 使用 `display:none` 进行 v-show 等价处理；父级 tags 区段也纳入同一规则。未传入该参数的独立组件夹具仍保持全量可见，便于复用和静态渲染。

## 验证

- 专项 `upload-confirm-dialog.test.tsx`：21/21。
- 新增测试验证非当前区段仍存在于 DOM、但带 `display:none`，当前区段不隐藏。
- `pnpm typecheck:web`：通过。
- `pnpm test:web`：758/758。

## 尚未验收

浏览器真实上传流程、Vue/React 同条件截图与 computed-style、后端解析任务及 Wails/native 仍未验证；本项只完成静态实现与交互模型回归。
