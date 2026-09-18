# CFT 续跑入口（RESUME）

最后更新：2026-09-18（T007 完成后；本轮已交付 8 项）

## 已完成（verified，自审）

| 任务 | commit | 核心交付 |
|---|---|---|
| CFT-S00-T001 | cd551bca | 基线 + e2e harness 两处回归修复（recovery worker、builtin 模型）；全入口实跑证据 |
| CFT-S00-T002 | e0ddd84b | contracts command-ports（冻结映射 + 7 测试；D003 裁决） |
| CFT-S00-T003 | d9490802 | .wk-craft 令牌作用域 + 对比度修正（4 测试） |
| CFT-S00-T006 | b4baf8f1 | @assistant-ui/react 0.15.20 真挂载（5 测试；恒等 convertMessage 必需） |
| CFT-S00-T005 | 9aa4783f | 能力投影全矩阵 + 服务端 capabilities 字段（domain 5 + Go 1） |
| CFT-S00-T004 | 8f347c7a | shell 组合层（4 测试；焦点委托共享 Sheet） |
| CFT-S01-T008 | efb0e369 | core command-bridge（顺序所有者 + 意图级幂等键；修复每次换键缺口；4 测试） |
| CFT-S01-T007 | 6f0bbb4b | 重放稳定组合契约 + thread.tsx 工具/交互卡（4 测试） |

## 进行中

- 无。

## S01 剩余（T009-T012）就绪态

- T009 首页/作品库/模板接线（deps T004✓ T005✓ T008✓）：home.tsx 类型选择器消费 capabilities（T005 已供 allowed_kinds）；作品库空/错态。
- T010 双栏工作台（deps T004✓ T007✓）：对话列切 CraftAssistantThread + thread.tsx 卡；响应式。
- T011 版本与来源视图（deps T008✓ T010）；T012 问题审批/取消/异常反馈（deps T008✓ T010）。
- 注：W05 旧实现已覆盖大部分 UI 语义（e2e 6 spec 全绿持证），本轮按高保真差距逐项补。

## 未提交改动

- 无（工作树干净）。

## 锁与活跃进程

- 台账锁：总控。无活跃 stack/进程（e2e 均已 teardown）。
- 主工作区（main）其他会话改动不要动；一切在 `.worktrees/craft-cft`。

## 下一条可执行命令

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/craft-cft
# T009 开始：packages/views/src/craft/home.tsx 的 kind 选择器接
# craftViewCapabilities + capabilities 响应字段（api-client 需从 create
# 响应透传或加 workspace capabilities 读取）；目标命令 test:craft:shared（现 90）
```

## 缺失环境

- 无。

## 待验证项

- real 模式 e2e（T024）；workbench 切 assistant-ui 后的视觉对比（T010/T034）；Mimosa 完整扫描（hook 报 scanner_enobufs，T035 完整审计）。
