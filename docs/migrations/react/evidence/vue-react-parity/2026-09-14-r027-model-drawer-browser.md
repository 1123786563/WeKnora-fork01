# R027 · 模型编辑器右侧抽屉浏览器复核

- 日期：2026-09-14
- React：`http://localhost:5181/platform/settings?section=models`
- 条件：已登录管理员、zh-CN、默认浅色主题、浏览器视口 1440×900。

## 复核结果

- 模型设置页实际显示 Vue 形态的模型类型标签、内置模型卡片和网格新增模型入口。
- 点击“添加模型”后，编辑器实际显示为右侧 560px 抽屉，带遮罩、关闭按钮、模型类型/来源/接入配置分组；不再以内联卡片占据主内容流。
- 抽屉首屏实际可见 API/Ollama 来源分段、服务商选择、模型名称、显示名称和 Base URL 字段；当前 Ollama 因服务状态不可用而禁用，符合 Vue 的状态门控。
- 交互渲染测试 `ModelSettingsPanel.test.tsx` 23/23；本次截图通过 CUA 采集，未将截图单独作为最终验收。
- Provider 与 Thinking Control 选择器已改为项目级 Vue 风格双行选项控件：保留描述、选中态、禁用态、ArrowUp/ArrowDown、Enter、Escape 和外部点击关闭；表单内不再使用原生 `<select>`。

## 边界

- Vue 独立浏览器会话未登录，未输入凭证或提交模型；本次是 React 实际渲染与 Vue 源码结构的复核，不是完整双端同账号截图对照。
- R027 仍需设置子项切换、真实连接/保存流程、Wails/native 证据及完整 computed-style 对照。
