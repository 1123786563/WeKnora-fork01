# R251 设置页标题几何对齐（2026-09-15）

同认证用户、同租户、同语言和同视口下，设置页“空间信息”标题的实际计算样式出现差异：React 为 24px/700/36px 且带 4px 上下 margin，Vue 为 20px/600/normal/28px 且仅保留 8px 底部 margin。React 标题已改为显式复用 Vue 的字体、行高和 margin。

浏览器复测：
- React 修正后：`font-size: 20px`、`font-weight: 600`、`line-height: normal`、`margin: 0 0 8px`、`y=60`、`height=28px`
- Vue：同样为 `20px/600/normal/0 0 8px`、`y=60`、`height=28px`

验证：
- 设置页聚焦测试：36/36
- `pnpm test:web`：901/901 通过
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
