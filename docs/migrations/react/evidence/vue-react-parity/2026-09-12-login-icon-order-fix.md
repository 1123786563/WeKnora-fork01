# /login 背景节点图标错位修复（2026-09-12，Round 6）

- 根因：Vue node-1 的书本图标是一个 SVG 含两条 path；React 移植时拆成两个图标，导致 node-2 起全部图标错位一位（crop 对比：node-3 位置 Vue=layers，React=folder-open）。
- 修复：合并 node-1 为单个 <g>，后续图标 key 依序前移，恢复 Vue 的 12 节点图标顺序。
- 证据：crop2-vue.png / crop2-react.png 同区域对比，node-3 均为 layers 图标；整页截图 login-round6-icons.png。
- web 测试通过。
