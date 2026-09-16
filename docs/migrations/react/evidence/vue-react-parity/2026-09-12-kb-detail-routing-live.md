# KB 卡片点击路由 live 验证（2026-09-12，Round 21）

- 真实登录后点击未初始化 KB 卡片标题 → 正确跳转 /platform/settings（模型未配置分支），KB 必修 #16（isInitialized 路由）live 验证通过。
- 文档列表/预览/wiki/FAQ/图谱的深入 E2E 需先为租户配置模型（涉及 provider 凭证，后续用测试凭证补齐）；已派发 KB 详情面只读审计（documents/document detail/processing timeline/wiki/FAQ/graph）。
- 截图：live-kb-detail.png（settings 落点）。
