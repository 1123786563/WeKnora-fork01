# CFT-S04-T030 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- **实跑既有 D03 spec**：`CRAFT_KINDS=web,slides` 起 mock 全栈 + `playwright test e2e/craft-slides.spec.ts` → **4 passed (11.6s)**。
- 实现零改动（spec 与分页视图为 D03 既有；此前因 kinds 默认只开 web 而 skip）。

## 验收断言对照（浏览器级，真实全栈）

- 生成→修改→逐页查看→历史下载 ✓（spec 01：5 页 deck 生成、**逐页截图 + 分页导航** + 修改通道；spec 02：仅第 3 页修改——**其余页字节相等、页数稳定、旧版本 SHA 完好**；D03 报告详载）
- 逐页渲染与引用 ✓（spec 03：第 3 页引用解析到 deck 源注册表与来源页；逐页图像来自渲染链——T029 fixture 链）
- 布局无溢出 ✓（渲染页检查 + T029 的 OverflowPages 拒绝语义；逐页截图在 spec 01 断言）
- 类型门禁 ✓（spec 04：fail-closed；开放后未知 kind 拒）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `CRAFT_KINDS=web,slides bash e2e/craft-stack.sh up mock && run mock e2e/craft-slides.spec.ts` | 0 | **4 passed (11.6s)** |

## Gate 决定

slides 浏览器级验收通过（模拟环境）；生产 Gate 开放留 T033。**S04 关闭：Office 三类型（document/spreadsheet/slides）浏览器级生成→修改→查看→历史下载全部实跑通过。**

## 回退

无代码变更；证据与台账为本轮交付。
