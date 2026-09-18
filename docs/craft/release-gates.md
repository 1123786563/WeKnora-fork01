# Craft 灰度发布与回滚（Release Gates）

任务：CFT-S05-T033 · 基线：`.worktrees/craft-cft`

## Gate 层级

| 层 | 开关 | 语义 |
|---|---|---|
| 功能总闸 | `WEKNORA_CRAFT_ENABLED` | 默认关（fail-closed）；关闭时路由不装配，503 |
| 类型闸 | `WEKNORA_CRAFT_KINDS` | 默认仅 `web`；每类型独立开放，未知 kind 拒绝 |
| 准入闸 | `agent.recovery.admission_enabled` | 可只关新准入、留 worker 排空存量（drain） |
| 商业门禁 | `scripts/check-craft-release.py` | 证据不全/HEAD 不符 → 拒绝放行（exit 1） |

## 关闭一个类型时发生什么

1. **新任务拒绝**：`CraftFeatureGate.Allows(kind)` 在 Create/StartRun 服务端强制（非 UI 隐藏）；closed kind → 503 Unsupported。
2. **已发布版本不受影响**：version store 读取无 kind 维度——GET 版本/下载照常（`TestCraftReleaseGateClosedKindRefusesNewKeepsPublished`）。
3. **活动 Run 排空**：关闭 admission 不杀在飞 Run——worker 继续执行至自身终态；需要立即停止走显式取消（StopStatus 权威终态语义，T023）。
4. **不删除任何数据**：Tombstone/回收属生命周期保守清扫（T031），与 Gate 无关。

## 回滚步骤

1. `WEKNORA_CRAFT_ENABLED=false`（或从 KINDS 移除某类型）→ 新任务立即拒绝。
2. 已发布版本与下载持续可用（只读不受闸影响）。
3. 需要停止在飞：显式 cancel（受理→停止→权威终态），不静默 kill。
4. 代码回滚：按任务独立 revert（36 个提交均为增量）；数据迁移可逆性由各 down.sql 保证。

## 商业门禁（本仓库现状）

`check-craft-release.py` 校验 release-evidence.json：HEAD 一致、26 项任务依赖 SHA 为真实祖先、REQUIRED 场景全部 passed 且证据文件非空含退出码、SQLite/Cube/E2B 独立能力、已知缺口必须列在 limitations。**当前证据 HEAD 与本 worktree HEAD 不一致 → 检查器正确拒绝（exit 1）**——生产放行前须在发布 commit 上重新生成证据；模拟环境（本轮全部）验收不构成生产 Gate 开放。

## 生产 Gate 状态（截至本轮）

- web：模拟+真实模型全链路证据齐（T024）；生产开放需发布 commit 上的证据重生成。
- document/spreadsheet/slides：模拟环境浏览器级验收通过（T026/T028/T030）；生产 KINDS 默认仍关。
