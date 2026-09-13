# N010 · FAQ 导入进度轮询对齐

- 日期：2026-09-14
- Vue 基线：`FAQEntryManager.vue:2091-2165` 将后端 `processing` 映射为 `running`、`completed` 映射为 `success`，按 1.5 秒轮询，完成后保留成功条再收起。
- React：`apps/web/src/faq/FAQPage.tsx` 与 `faq-import-poll.test.tsx`。

## 已验证

- `FAQImportProgress` 已从 `@weknora/api-client` 公共入口导出。
- 导入成功后进度条显示已处理数量与百分比；轮询状态按 Vue 语义转换；成功状态保持 3 秒后收起。
- 轮询异常会结束当前进度条，不会伪造成功状态。
- N010 相关测试与 Web 全量回归通过：FAQ 20/20，Web 672/672；`typecheck:web` 通过。

## 未覆盖

- 尚未在已认证浏览器中提交真实 FAQ 文件并连接真实后端任务队列；未将 jsdom/测试桩当作真实后端证据。
- Wails、iOS、Android 和双端同条件截图仍未完成。
