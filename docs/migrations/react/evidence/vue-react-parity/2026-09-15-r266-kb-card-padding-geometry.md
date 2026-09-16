# R266 知识库卡片内边距与头部几何对齐（2026-09-15）

同浏览器、同视口、同一个 `Parity KB Demo` 知识库数据下，React 知识库卡片头部原先使用 `padding: 12px`，Vue 基线使用 `padding: 12px 14px`；更多操作按钮此前为 24px，造成标题和按钮相对 Vue 向左偏移、头部高度不同。

修正内容：
- React 卡片改为 `padding: 12px 14px`。
- React 更多操作按钮固定为 `28px × 28px`，与 Vue `.more-wrap` 一致。

浏览器复测（Chrome，同条件）：
- React/Vue 卡片：`x=344,y=202,w=319.664,h=136`
- React/Vue 卡片头部：`x=359,y=215,w=289.664,h=28`
- React/Vue 更多按钮：`x=620.664,y=215,w=28,h=28`
- React/Vue 卡片 computed padding：`12px 14px`

验证：
- 知识库列表聚焦测试：16/16
- `pnpm run typecheck:web`：通过
- `git diff --check`：通过
