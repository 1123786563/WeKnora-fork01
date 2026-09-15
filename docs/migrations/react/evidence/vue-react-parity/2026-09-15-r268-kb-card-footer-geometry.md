# R268 知识库卡片底部间距对齐（2026-09-15）

同条件 Chrome 对照发现 React 卡片底部边框上内边距为 8px，Vue 基线为 6px，导致底部区域向上偏移 2px。将 React 底部区域改为 `padding-top: 6px` 后，底部区域与 Vue 完全重合。

浏览器复测：
- React/Vue 底部区域：`x=359,y=296.5,w=289.664,h=28.5`
- React/Vue `padding-top`：`6px`
- 卡片底部坐标：`y=325`

验证：
- 知识库列表聚焦测试：16/16
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
