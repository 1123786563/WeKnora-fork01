# KB 列表折叠分组节头落地（2026-09-12，Round 19）

- 主代理实施（KB 后续项 #11）：domain groupKnowledgeBaseSections（pinned 置顶最新优先/mine/tenantOthers/sharedEditable/sharedReadonly，空 section 省略；TDD 11/11）+ App.tsx 折叠 chips（计数 + aria-expanded）接入现有网格与分页。
- i18n：复用已迁移的 knowledgeList.sections.* 键，零新增键。
- live 验证：真实登录后「我创建的 · 2」chip 渲染、空 section（置顶/共享）正确省略（kblist-sections-live.png）。
- 门禁：web 157/157、domain list 11/11、typecheck:web 中仅存四页子代理 WIP 的 2 个 embed/index.ts 错误（其收尾自愈）。
- 提交：01bccd0。
