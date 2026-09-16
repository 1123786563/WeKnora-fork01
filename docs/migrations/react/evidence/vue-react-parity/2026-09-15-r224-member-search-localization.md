# R224 成员搜索清除按钮本地化（2026-09-15）

成员管理面板的搜索清除按钮此前写死英文 `Clear search`。现新增 `tenantMembersPanel.clearSearch` 五语言 fallback，并通过当前面板 translator 生成 aria-label；搜索清空、分页重置和重新加载行为保持不变。

验证：`pnpm exec tsx --test apps/web/src/settings/TenantMembersPanel.test.tsx` 9/9 通过，包含 en-US 默认测试和 zh-CN 面板结构断言。
