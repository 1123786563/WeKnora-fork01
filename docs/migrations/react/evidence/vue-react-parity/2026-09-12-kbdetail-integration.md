# KB 详情面必修集成复核（2026-09-12，Round 32）

- 门禁（主代理独立重跑）：test:shared 276/276、test:web 178/178、typecheck:shared/web 干净、build:web 成功。
- 提交 eb47a9c：14 文件，含 927 键 ×5 locale 的 knowledgeSurfaces i18n 模块（4655 行）+ 补充键 + 键集一致性测试；reparse/cancel + 处理时间线轮询；FAQ/Wiki 真分页；权限门控；FAQ 型路由。
- live 复测：Documents 页调试文案已消失、中文标题渲染（live-kb-documents-after.png）。
- 保留用户既有未提交修改未动（apps/mobile/expo-env.d.ts、reuse-manifest 等）。
