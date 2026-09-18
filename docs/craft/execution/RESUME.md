# CFT 续跑入口（RESUME）

最后更新：2026-09-18（S00 全部完成后）

## 已完成（全部 verified，自审；台账见 STATUS.md / task-status.json）

| 任务 | commit | 核心交付 |
|---|---|---|
| CFT-S00-T001 | cd551bca | 基线报告 + e2e harness 两处回归修复（recovery worker env、builtin 模型降级）；test:craft:shared 61 / shared 573 / go craft 103 / playwright mock 6/6 |
| CFT-S00-T002 | e0ddd84b | contracts command-ports.ts 冻结命令合同（7 测试）；D003 裁决：expectedWorkspaceRevision 留意图层、wire 暂不传、T016 复评 |
| CFT-S00-T003 | d9490802 | .wk-craft 27 别名入 theme.css；craft.css 17 种色值→别名；绿按钮 ink 前景（≥4.5:1）；4 令牌测试 |
| CFT-S00-T006 | b4baf8f1 | @assistant-ui/react 0.15.20 锁版真挂载（assistant-runtime.tsx：稳定ID/isRunning 外部驱动/命令桥 onNew；恒等 convertMessage 是 0.15.20 必需）；5 JSDOM 测试 |
| CFT-S00-T005 | 9aa4783f | domain craftViewCapabilities 全矩阵投影 + 服务端 Capabilities() 进 create/workspace 响应；closed kind 503 |
| CFT-S00-T004 | 8f347c7a | shell.tsx 组合层（Shell/PanelHeader/Notice/Drawer 委托 Sheet 焦点契约）；4 测试 |

## 进行中

- 无（S00 关闭，S01 未开始）。

## S01 就绪态（依赖已满足）

- T007 稳定消息与工具卡投影（deps T002✓ T006✓）：把 workbench 对话区切到 CraftAssistantThread + 工具卡槽。
- T008 统一附件与提交命令桥（deps T002✓）：api-client 消费 command-ports（craftDraftWireBody 等）+ routes.tsx 提交链接入（附件引用、幂等键复用、创建失败不重建会话已有语义核对）。
- T009/T010/T011/T012 依赖上述。

## 未提交改动

- 无（工作树干净，全部已提交到 craft/cft-execution）。

## 锁与活跃进程

- 台账锁：总控（单写）。e2e stack 已 teardown，无活跃进程。
- 主工作区（main b04722cf）有其他会话未提交改动，**不要动**；本轮一切在 `.worktrees/craft-cft`。

## 下一条可执行命令

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/craft-cft
# S01 从 T008 开始（命令桥，解锁 T009/T011/T012）：
# 1) packages/api-client/src/craft/index.ts 的 submit/create 接 command-ports 校验+序列化
# 2) apps/web/src/features/craft/routes.tsx 提交链接用冻结合同（附件 ref、request_id 复用）
# 目标命令: pnpm run test:craft:shared（现 82）+ typecheck:web + e2e mock
```

## 缺失环境

- 无。

## 待验证项

- real 模式 e2e（T024）；Shell/assistant-ui 组件进 workbench 后的视觉对比（T010/T034）；Mimosa 完整安全扫描（hook 持续报 scanner_enobufs，已按兼容策略提交但未宣称安全——T035 跑完整审计）。
