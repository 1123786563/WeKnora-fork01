# R211 设置常规分区 React/Vue 配对 AX（2026-09-15）

## 范围

同一 Chrome 会话、zh-CN、1355x720：

- React：`http://localhost:5181/platform/settings?section=general`
- Vue：`http://localhost:5173/platform/settings?section=general`

## 结果

两端按相同顺序暴露设置抽屉、账户/空间/模型/发布集成/数据与扩展/平台导航分组，以及常规设置的语言、主题、界面字体、代码字体和字体大小控件。选中值均为简体中文、浅色、系统默认、系统默认、正常。

React 使用 button、popup button、menu、radio 的原生语义，Vue 使用自定义容器和 text field 语义；可见标签、顺序、选中值和交互锚点一致。`retrieval` 不再出现在 React 导航中，与 Vue 当前 navItems 一致。

## 结论

当前常规设置抽屉没有发现需要继续修改的可见结构差异。受保护设置写入、五种语言、响应式和 Wails/native 仍需独立证据。
