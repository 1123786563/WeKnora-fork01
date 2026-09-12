# settings.* 文案全量迁入共享 i18n（2026-09-12，Round 10）

- 扫描 frontend/src/views/settings/**（vue+ts，排除测试）全部 t()/$t() 引用，得 1038 个去重键。
- 程序化提取 5 locale 字节级值，生成 packages/i18n/src/settings.ts（settingsMessages，约 1022 键/locale；少量缺失为模板拼接键如 tenantInvitation.status.${status}、未在 locale 表出现的 mcpServiceDialog.oauth*，登记为待查）。
- index.ts 合并链追加 settingsMessages（优先级最高，settings 域内覆盖同名基础键为 Vue 精确值）。
- 新增 settingsMessages.test.ts：跨 locale 键集一致性 + 代表键存在性。
- i18n 12/12、shared 247/247、typecheck:shared 通过。
- 供 settings 页实施子代理直接使用 formatMessage(locale, key)。
