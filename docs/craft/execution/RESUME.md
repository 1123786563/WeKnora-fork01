# CFT 续跑入口（RESUME）

最后更新：2026-09-18（T001 完成后）

## 已完成

- CFT-S00-T001（verified，自审）：基线报告 + 全部验证入口实跑 + e2e harness 两处回归修复。

## 进行中

- 无。

## 未提交改动

- 见当前 git status（本文件与台账首次提交之前的改动均属 T001 交付：apps/web/e2e/craft-stack.sh + docs/craft/**）。

## 锁与活跃进程

- 台账锁：总控持有（单写）。
- 无活跃 stack/worker：e2e 已 teardown（run 目录 /tmp/craft-w06-mock.2YH3gP 保留检查，重启即失）。
- 主工作区（main b04722cf）有其他会话的未提交改动，**不要动**；本轮一切工作在 `.worktrees/craft-cft`。

## 下一条可执行命令

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/craft-cft
# T002 开始：冻结命令/能力/事件适配合同（读 04-API与状态契约.md 与 command-bridge.types.ts，
# 对照 packages/contracts/src/craft、api-client、internal/handler/session/craft.go）
```

## 缺失环境

- 无。real 模式 e2e（OpenCode 免费模型）可在 T024 直接跑（W06 历史通过）。

## 待验证项

- real 模式 e2e（T024）；typecheck:web（T002+ 每轮）；T006 assistant-ui 锁版兼容性实测。
