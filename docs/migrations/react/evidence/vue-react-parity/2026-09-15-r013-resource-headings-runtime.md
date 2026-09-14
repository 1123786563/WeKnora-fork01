# R013 resource settings heading runtime evidence

Date: 2026-09-15

- Authenticated React Chrome reached `/platform/settings?section=vectorstore` and `/platform/settings?section=websearch`.
- Vector-store rendered `向量数据库引擎` with its localized description; web-search rendered `网络搜索配置` with its localized description.
- Both pages exposed localized editor labels and safe-config guidance while preserving add/refresh/list behavior.
- The available Vue tab had no authenticated session, so same-session pixel comparison remains open.
