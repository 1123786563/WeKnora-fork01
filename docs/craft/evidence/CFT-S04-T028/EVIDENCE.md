# CFT-S04-T028 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- **实跑既有 D02 spec**：`CRAFT_KINDS=web,spreadsheet` 起 mock 全栈 + `playwright test e2e/craft-spreadsheet.spec.ts` → **4 passed (12.1s)**。
- 实现零改动（spec 与只读网格视图为 D02 既有；此前因 kinds 默认只开 web 而 skip）。

## 验收断言对照（浏览器级，真实全栈）

- 生成→修改→查看→历史下载 ✓（spec 01：创建/上传/生成/sheet 切换/XLSX 下载；spec 02：季度轮保持 300 + **旧 XLSX 字节 SHA 不变**；spec 03：加 50 行后重算总额 350——预览、缓存与下载一致）
- 只读网格 ✓（D02 网格视图：sheet 切换/分页；数值来自重算缓存——T027 pin 的预览=存储）
- 类型门禁 ✓（spec 04：部署未开放时 fail-closed；开放后未知 kind 仍拒）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `CRAFT_KINDS=web,spreadsheet bash e2e/craft-stack.sh up mock && run mock e2e/craft-spreadsheet.spec.ts` | 0 | **4 passed (12.1s)** |

## Gate 决定

spreadsheet 浏览器级验收通过（模拟环境证据）；生产 Gate 开放留 T033。

## 回退

无代码变更；证据与台账为本轮交付。
