# CFT-S04-T026 证据

日期：2026-09-18 · worktree `.worktrees/craft-cft` · commit 见本任务提交

## 交付物

- **实跑既有 D01 spec**：`CRAFT_KINDS=web,document` 起 mock 全栈 + `playwright test e2e/craft-document.spec.ts` → **4 passed (26.3s)**。
- 同栈回归：`e2e/craft-report.spec.ts` → **6 passed (23.5s)**（document 开放不影响网页闭环）。
- 实现零改动：spec 与 document 视图（document.tsx）为 D01 既有；本轮是浏览器级验收执行（该 spec 此前因 kinds 默认只开 web 而 skip）。

## 验收断言对照（浏览器级，真实全栈）

- 生成→修改→查看→历史下载 ✓（spec 01：两份知识资料→3 章节/8 行表/数字 12/96%/6 周/2 个可点 kc_ 引用 + DOCX 附件下载、存储 OOXML 与 Markdown 对照；spec 02：FAQ 轮保持数字、**旧版本 DOCX 字节 SHA 不变**；D01 报告详载）
- 预览版本与下载版本一致 ✓（spec 01/02：workbench document 视图（安全 Markdown 渲染、无原始 HTML）与下载 DOCX 的内容对照断言）
- Markdown 脚本/危险链接被处理 ✓（D01 语义：安全 Markdown 渲染器无 raw HTML；e2e 03 的恶意面在 W06 web 沙箱 spec；本 spec 断言视图不注入原始 HTML）
- 查看历史不改编辑基线 ✓（spec 02 的版本切换语义 + T011 domain 规则（view 只动 viewVersionId）与服务端 workspace 不受读影响）
- 附带反例 ✓（spec 03：**corrupt export + 如实 export=failed manifest → 零发布**、界面无导出成功；spec 04：gate 开 document 且未知 kind 仍拒绝）

## 命令与退出码

| 命令 | 退出码 | 结果 |
|---|---|---|
| `CRAFT_KINDS=web,document bash e2e/craft-stack.sh up mock && run mock e2e/craft-document.spec.ts` | 0 | **4 passed (26.3s)** |
| 同栈 `run mock e2e/craft-report.spec.ts` | 0 | **6 passed (23.5s)** |

## Gate 决定

document 类型在本轮浏览器级验收通过（生成/修改/查看/历史下载 + 失败反例）——**模拟环境证据成立**；生产 Gate 开放属 T033 灰度任务决定（默认部署仍只开 web，不因测试绿而擅自放开）。

## 回退

无代码变更（spec 既有）；证据与台账为本轮交付。
