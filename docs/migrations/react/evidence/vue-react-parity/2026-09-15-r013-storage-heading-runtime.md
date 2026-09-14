# R013 storage settings heading runtime evidence

Date: 2026-09-15

- Authenticated React Chrome reached `/platform/settings?section=storage`.
- The section heading rendered localized Chinese copy: `存储引擎` and `配置文档与图片的存储方式。此处设置各引擎参数，知识库中仅选择使用哪个引擎。`
- Resource editor labels and row fallback copy were also localized; existing storage row actions remained reachable.
- The available Vue tab had no authenticated session, so same-session pixel comparison remains open.
