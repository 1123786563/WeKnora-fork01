# 2026-09-14 文档页 live 现状取证 + 新发现：知识库设置面板 i18n 泄漏（部分完成切片）

## 已取证

- React 文档页（/knowledgeBase/9727d104...?tab=documents，Parity KB Demo，zh-CN 1440x900）真实渲染：面包屑（知识库 > Parity KB Demo > 文档）、文件夹树（Root 0）、搜索/标签/类型/状态/来源过滤、批量工具条（取消选择/重新解析/移动/批量打标签/删除/取消解析）、解析引擎不可用警告（部分文档类型暂无可用解析引擎，前往配置 →）、空态（知识为空，拖放上传 + 格式说明）。截图 docs-live-20260914/d1-documents-react.png。
- 404 路由守卫 live 复现：/knowledgeBase/:id/documents 后缀形式不存在（React 路由的文档页用 ?tab=documents 查询参数），NotFoundPage 正确渲染「页面不存在」+ 返回知识库链接（negpath 第一批 D1/D6 修复的 live 行为）。

## 新发现（登记为后续切片）

1. **知识库设置面板 i18n 泄漏**：zh-CN 下 /knowledgeBase/:id/settings 渲染英文正文（General / Description / Parser engines / Server default …，h2 为中文「知识库设置」）。该面（apps/web/src/knowledge-settings + 相关面板）未接入共享 i18n——登记为独立 i18n 对齐切片（涉及面较大，不适合在本轮顺手修）。
2. **双端上传自动化工具待完善**：React 空态点击可触发文件选择器（filechooser 模式可用）；Vue t-upload 拖放区点击不触发选择器（需定位其内部上传按钮/事件）。文档真实上传 → 列表/详情/预览的 live 数据维度顺延至下一切片。

## 状态

- 本轮无代码变更；截图 docs-live-20260914/d1-documents-react.png 入库。
- 未写入任何共享数据（上传未完成，无文档创建）。
- 后续切片顺序建议：知识库设置面板 i18n → 双端文档上传 live 数据维度（R047-R050/N006/N008 的数据依赖）。
