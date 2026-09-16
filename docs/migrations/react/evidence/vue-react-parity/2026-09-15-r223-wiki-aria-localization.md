# R223 Wiki 区域标签本地化（2026-09-15）

Wiki 页面目录和版本历史导航此前写死英文 `Wiki pages`、`Wiki revisions`。现分别使用已有五语言 `wikiBrowser.pageActions` 与 `wikiBrowser.historyBtn`，只改变 aria-label，不改变页面结构、编辑权限和数据流程。

验证：`pnpm exec tsx --test apps/web/src/wiki/WikiPage.test.tsx` 5/5 通过。
