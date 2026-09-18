# CFT 续跑入口（RESUME）

最后更新：2026-09-18（T015 完成后；累计 verified 15/36：S00+S01 全闭 + T013/T014/T015）

## 已完成（verified，自审；证据目录 docs/craft/evidence/<task-id>/）

| 任务 | commit | 核心交付 |
|---|---|---|
| CFT-S00-T001 | cd551bca | 基线 + e2e harness 两处回归修复；全入口实跑证据 |
| CFT-S00-T002 | e0ddd84b | contracts command-ports 冻结合同（D003 裁决） |
| CFT-S00-T003 | d9490802 | .wk-craft 令牌作用域 + 对比度修正 |
| CFT-S00-T006 | b4baf8f1 | @assistant-ui/react 0.15.20 真挂载 |
| CFT-S00-T005 | 9aa4783f | 能力投影全矩阵 + 服务端 capabilities |
| CFT-S00-T004 | 8f347c7a | shell 组合层（焦点委托共享 Sheet） |
| CFT-S01-T008 | efb0e369 | core command-bridge（顺序所有者 + 意图级幂等键） |
| CFT-S01-T007 | 6f0bbb4b | 重放稳定组合契约 + thread.tsx 工具/交互卡 |
| CFT-S01-T009 | d3198f20 | library/templates 新页 + kind 选择器接服务端能力 |
| CFT-S01-T010 | 1a47857d | 对话列切真实 assistant-ui Thread（composer=false 保留锚点）；spec05 定位器更新 |
| CFT-S01-T011 | 587dc369 | domain version-selection 四断言 + versions.tsx 恢复确认 + sources 撤权占位 |
| CFT-S01-T012 | d186b567 | CraftDecisionCard 四层送达状态 + 终态零控件 + StatusNotice 非失败横幅 |
| CFT-S02-T013 | 1646a846 | 协议 pin 3 测试 + CRAFT_LIVE=1 真实 serve 两轮 PASS |
| CFT-S02-T014 | b0cb7666 | TestDelegationIdentity*（幂等/冲突/重启读回） |
| CFT-S02-T015 | b2a3961a | **修复真实缺口**：预检读失败盲重发 prompt → 拒绝重提交转 unknown；3 注入测试 + live 50.8s 复验 |

测试基线：test:craft:shared **108 pass**；playwright mock **6/6**；go craft 103 + handler/service 全绿；typecheck:shared/web 通过。

## 进行中

- 无。

## S01 剩余

- **T010 双栏工作台**（deps T004✓ T007✓）：对话列从 W05 自绘切到 CraftAssistantThread + CraftToolFactList；CraftShell/CraftDrawer 接入顶栏与抽屉；响应式（<760 切换）；注意 workbench.tsx 有大量 W05 既有逻辑（turns 归档、preview/files/interaction 面板）——**渐进替换对话列渲染**而非重写整个组件；改后必须跑 e2e 6 spec（craft-goal/craft-main-status 等 data-testid 是断言锚点，不可破坏）。
- **T011 版本与来源视图**（deps T008✓ T010）；**T012 问题审批/取消/异常反馈**（deps T008✓ T010）——W05 交互卡（interaction.tsx）与 R06 decide 路由已有，主要核对高保真差距（DecisionCard 状态机 recorded→delivery_pending→delivered→unknown）。
- 新模块接线三链提醒：packages/*/package.json exports + apps/web/tsconfig.json paths + apps/web/vite.config.ts alias **都要登记**（T009 踩过：漏 vite alias 导致 e2e 红）。

## S02/S03 提示（按差距核实模式）

- 后端语义大量已有（internal/craft 20+ 模块 103 测试、W01-W06 报告）；T013-T023 按任务卡逐项核对现状（幂等/CAS/fence/预算/恢复/审批送达在 Go 测试与 docs/testing/craft/web-acceptance.md 有历史证据），缺什么补什么，不重建。
- **T024 里程碑验收**：跑 craft-stack.sh real 模式（W06 历史通过，需本轮复跑）+ mock 全量 + 反例清单。

## 未提交改动 / 锁 / 进程

- 工作树干净；无活跃 stack（均已 teardown）；台账锁总控。主工作区（main）其他会话改动不要动。

## 下一条可执行命令

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/craft-cft
# T010 开始：packages/views/src/craft/workbench.tsx 对话列接
# CraftAssistantThread（props: messageLog/controller 已在位）
# 验证: pnpm run test:craft:shared（94 基线）+ e2e mock 6 spec
```

## 缺失环境 / 待验证

- 无缺失。待验证：real e2e（T024）、视觉对比（T034）、Mimosa 完整扫描（T035；hook 一直报 scanner_enobufs 未出完整结论，本轮未宣称安全）。
