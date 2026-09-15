# R226 数据源状态文案本地化（2026-09-15）

数据源列表和同步日志原先直接显示服务端英文状态以及 `Total/skipped/failed` 固定英文。React 现在复用现有 `dataSource.status.*` 与 `dataSource.logMetric.*` 五语言词条；未知服务端枚举仍原样保留，避免丢失诊断信息，请求和状态判断不变。

验证：

- 数据源表单/资源选择测试：4/4
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过

受保护数据源后端、响应式、多语言浏览器截图及桌面/native 运行时证据仍待补齐。
