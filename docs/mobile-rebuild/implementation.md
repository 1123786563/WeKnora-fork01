# Mobile Rebuild · 从零实施记录（apps/mobile-next）

启动日期：2026-09-18。分支：`rebuild/mobile-next`。

## 0. 权威与边界（依据用户 2026-09-18 指令）

- **重写方式**：用户本次指令——从零新建 `apps/mobile-next`，不复用旧 `apps/mobile` / Happy / 旧前端业务包（api-client、domain、views、core、ui 等）的任何实现。设计包内"沿用 apps/mobile / 复用 Happy 组件"等段落按本指令覆盖。
- **页面、视觉、交互**：设计包 `weknora-expo-hifi-v2`（高保真 HTML 18 页 + tokens + 规格文档）。
- **接口事实**：本仓库 Go 后端实际路由/DTO（见 `docs/mobile-rebuild/api-facts.md`，来自只读源码核验）。
- **后端边界**：后端缺失能力时仅做有测试的最小补充；不建第二套权威任务库/审批引擎/钱包。

## 1. 路径与哈希

- REPO_ROOT：`/Users/wuyongjun/trea/WeKnora-fork01`（本地已有仓库，直接使用）
- DESIGN_ROOT（权威，仓库内，2026-09-18 用户迁入 commit d1253a67）：`docs/design/weknora-expo-hifi-v2`
- DESIGN_ROOT（原始来源，本机）：`/Users/wuyongjun/Downloads/weknora-expo-hifi-v2`；全文件递归 shasum 汇总 `54e02578a6ddbaae4366729285198e3d812df36a`
- 仓库内副本与原始包内容一致（tokens.json 经 json 规范化比对逐值一致）
- 令牌源归档：`docs/mobile-rebuild/design-ref/tokens.json`（与 DESIGN_ROOT/tokens/tokens.json 语义一致）
- 设计包实际文件名与任务提示有偏差（无 `tasks/task-index.json`，实为 `plans/task-index.json`；无 `tokens/native-theme.ts`，实为 `tokens/native-tokens.ts`；docs 为 01/02/03/04-api-contracts/05-development-handoff），按实际存在文件读取。

## 2. 工程决策（详见 docs/evidence/mobile-rebuild/decisions.md）

- 新工程 `apps/mobile-next` 为**独立 npm 包，不加入 pnpm workspace**（物理隔离，杜绝 workspace-link 旧业务包；对其他端零影响）。
- 工具链：Expo SDK 55（~55.0.x）/ React 19.3 / RN 0.83 / expo-router ~55.0（与本地缓存和设计包核验矩阵一致；版本组合选择，非旧代码复用）。测试：jest + @testing-library/react-native。
- 视觉权威：tokens.json 实际值（brand `#08766A`/`#94E0BA`、body 16/26 等）。任务提示中的"核对值"（#09694D、16/24 等）与设计包不一致，按指令"实际样式全部从 tokens.json 生成"以设计包为准，差异记录于 decisions.md D-01。
- 后端差异（api-facts.md）：`/workbench/overview`、`/workbench/inbox`、统一四类交互 kind、通用通知 inbox 后端暂缺。处理：客户端聚合降级 + 记差异，最小后端补充另行任务（不假装接口存在）。

## 3. 交付分组（DAG）

1. G1 工程：scaffold、TS、typecheck、jest、tsc 绿
2. G2 视觉：tokens→主题、双主题、基础组件、路由与页面结构
3. G3 数据：contracts 校验器、api client、auth、spaces、SQLite 持久化
4. G4 核心：M03/M04/M05/M06/M07、提交状态机、SSE 流与恢复
5. G5 扩展：M08–M18、四类决定、成果/知识/连接、通知、用量
6. G6 扩展能力：目标选择、听写、语音
7. G7 验收：视觉对照、E2E、故障、交付检查

## 4. 状态速览

见 `tasks.json`（RW-001…）与 `../evidence/mobile-rebuild/progress.md`。
