# r099 知识库页面 computed-style 对比

## 条件

- Vue `:5173/platform/knowledge-bases` 与 React `:5181/platform/knowledge-bases`
- 同一 Chrome 会话、同一 parity 用户与数据夹具、默认视口
- 通过浏览器页面内 `getComputedStyle` 与 `getBoundingClientRect` 采样

## 结果

| 元素 | React | Vue | 观察 |
|---|---|---|---|
| 页面标题 | 24px/32px、Inter、`x=336,y=48` | 24px/32px、系统字体、`x=344,y=20` | 字号和行高一致，字体族与纵向起点不同 |
| 页面根容器 | `MAIN`，`x=260,w=1095`，padding `48px 20px` | `.main`，`x=0,w=1355`，白色背景，14px | React 将内容放入独立主区；Vue 使用整页 flex 根容器 |
| 知识库卡片 | 圆角 8px、1px 边框、绿色渐变背景、1px/3px 阴影 | 结果节点为 `.card-title-text` 文本，未能从同一节点取得卡片盒模型 | DOM 结构与采样节点不等价，不能直接判定卡片样式 parity |

React 卡片的渐变、边框和阴影在浏览器 computed-style 中实际生效；React `main` 的 `max-w-[960px]` 类在 computed 中返回 `max-width: none`；追踪 `PlatformShell.tsx` 后确认这是 Shell 对 `.wk-page` 的全宽覆盖规则（`[&_.wk-page]:max-w-none!`），属于预期的受保护页面布局行为，不再作为待修复问题。

## 结论

本项提供了真实浏览器 computed-style 证据，并明确记录了不可比的节点结构与待确认的 React 宽度样式。完整视觉 parity 仍需按页面逐项采样和截图对照。
