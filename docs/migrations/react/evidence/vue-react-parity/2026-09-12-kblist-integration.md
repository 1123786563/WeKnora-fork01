# /platform/knowledge-bases 集成复核（2026-09-12，Round 6）

子代理完成 8 项必修 + pin/duplicate/?scope= 后，主代理独立复核：

## 复核结论
- 全量验证（主代理本机重跑）：test:shared 236/236、test:web 131/131、typecheck:shared 0 错、typecheck:web 0 错、build:web 成功（仅既有 chunk>500kB 警告）。
- 边界检查：仅改 apps/web 与 packages/*（依赖方向 apps→packages）；apps/web/src/auth、frontend/、mobile/desktop/embed 未触碰。
- 端点真实性抽查：togglePin=PUT /api/v1/knowledge-bases/:id/pin（routes_knowledge.go:224, Viewer+KBAccessRead）、duplicate=POST /:id/duplicate（:237, Contributor）与后端一致，无虚构 API。
- 业务逻辑抽查：canManageKBCard/canDuplicateKBCard/isKnowledgeBaseInitialized/mergeAllScopeKnowledgeBases 均在 packages/domain 且带测试；58 个 knowledgeList.* 键 5 locale 字节级迁移并有跨 locale 键集一致性测试。
- 测试先行：子代理报告每项均 red→green（i18n 5、domain 6、api-client 2、web loader/guard 3）。

## 提交
- 39a9180 feat(web/kb): rebuild knowledge-base list page to Vue parity（15 文件，+1878/-78）

## 仍开放（/knowledge-bases 维持 review）
- 折叠分组节头（pinned/mine/tenantOthers/shared* 计数与折叠）未实现（i18n 标签已备好）。
- 需真实后端登录后的浏览器截图对比（卡片视觉/悬停/高亮闪烁动画）与 Wails 验证。
- 上传进度面板、共享 KB 详情抽屉、highlightKbId URL 态未实现。
