# tRPC 恢复实施进度

- 设计：[已批准规格](../specs/2026-09-10-dual-agent-trpc-recovery-design.md)
- 计划：[实施计划](2026-09-10-dual-agent-trpc-recovery.md)
- 当前阶段：规划完成；代码实施未开始。
- 范围：两引擎分会话，复用现有能力，仅 tRPC 持久化恢复，未知结果等待用户。
- 规划基线：`e91f8af`。执行工作树与实际起点在启动实施时登记。

状态：pending / in_progress / implemented / reviewed / verified。只有目标测试及必要实网/进程验收通过才能 verified。

| ID | 任务 | 状态 | 实现提交 | 验证 / 审查证据 |
|---|---|---|---|---|
| 01 | SDK 恢复实证 | pending | 未实施 | 未运行 |
| 02 | 共享装配与引擎类型 | pending | 未实施 | 未运行 |
| 03 | Run 与租约存储 | pending | 未实施 | 未运行 |
| 04 | 模型与 checkpoint | pending | 未实施 | 未运行 |
| 05 | 工具日志与策略 | pending | 未实施 | 未运行 |
| 06 | tRPC 图纵向链路 | pending | 未实施 | 未运行 |
| 07 | 受理与后台接管 | pending | 未实施 | 未运行 |
| 08 | 持久化等待与决策 | pending | 未实施 | 未运行 |
| 09 | Sandbox 恢复 | pending | 未实施 | 未运行 |
| 10 | 能力完整复用 | pending | 未实施 | 未运行 |
| 11 | 事件、投影、steering | pending | 未实施 | 未运行 |
| 12 | HTTP 与生命周期 | pending | 未实施 | 未运行 |
| 13 | 客户端恢复交互 | pending | 未实施 | 未运行 |
| 14 | 崩溃矩阵与启用门禁 | pending | 未实施 | 未运行 |

## 执行记录要求

每次任务追加开始/结束时间、实现者、固定 HEAD、失败测试原因、通过命令、审查问题与修复提交。保留历史记录，不用最终 PASS 覆盖中途失败。

SDK 候选版本尚未通过本地兼容验证；Task 01 完成后记录根/子模块精确版本及实际 saver 接口。PostgreSQL/SQLite/Sandbox 各组合独立登记，测试未运行不能推断为通过。
