# Vue → React 逐页验收进度账本（vue-react-parity-progress）

## 2026-09-12 Round 1 基线

- 工作区：`.worktrees/react-multiclient`，分支 `codex/react-multiclient`，HEAD `b2d0cf6`。
- 已保留既有未提交修改（apps/mobile/expo-env.d.ts、reuse-manifest.csv、runtime-baseline.md、version-matrix.md 及 docs/superpowers/* 新文件）。
- 建立本矩阵与账本；evidence 目录 `docs/migrations/react/evidence/vue-react-parity/`。
- 初始状态：route-parity.csv 全部 53 条路由/入口均有 React 对应或明确 special-route 去向；无“React 尚无对应页面”项（待逐页核对确认非空壳）。
- 全部页面状态 review：已有测试/证据覆盖契约与路由分发，未覆盖逐页功能/布局/视觉/校验/文案/业务逻辑一致性与各端验收。
- 基线测试：见 evidence/vue-react-parity/2026-09-12-round1-baseline.md。

## 待办顺序（下一轮起逐页闭环）

1. /login + /register + /onboarding/workspace（auth 流，T03/T04）
2. /platform/knowledge-bases 列表页（T06）
3. /platform/knowledge-bases/:kbId 详情（T06–T09）
4. /platform/settings 各 section（T05/T17）
5. chat 全链路（T10–T14）
6. agents（T15）、organizations（T16）、integrations（T18）
7. embed 入口（T18）、移动端对应页（T20–T23）
