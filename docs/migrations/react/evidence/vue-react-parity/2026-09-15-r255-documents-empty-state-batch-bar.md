# R255 — Empty documents batch toolbar parity

日期：2026-09-15

## 实测差异

在同一有效知识库 `Parity KB Demo`、同一账号 `parity-test@local.dev`、zh-CN
和相同 Chrome 视口下，Vue 空文档页面仅显示上传空态；React 还显示禁用的
`全选`、`本页已选`及批量操作按钮。

## 修复与验证

React 的批量工具条现在仅在加载中或存在文档时渲染，成功空态和错误态不再
显示该工具条。浏览器复测确认 React 空态仍显示“知识为空，拖放上传”，且
不再出现 `全选`/`本页已选`。文档标签/批量聚焦测试 9/9，`pnpm
typecheck:web` 通过。

## 边界

当前有效 fixture 为空，非空文档行选择、批量模式进入/退出和真实 mutation
仍未由本项覆盖；文件选择器自动化接口在本轮未能提供可用 filechooser，未
上传或改变后端数据。
