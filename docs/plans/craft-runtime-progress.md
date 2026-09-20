# Craft Runtime 进度记录

日期：2026-09-10。架构方向已由用户确认。用户随后澄清当前需要先完成完整产品方案，实施流程已停止。

当前评审入口：[WeKnora Craft 产品与总体架构方案](../specs/2026-09-10-onyx-craft-product-proposal.md)。此前 P1 在 `.worktrees/craft-opencode` 独立工作树产生了局部实现与验证，现保留现场；本表是原计划的任务清单，不能代替工作树中的实现记录，也不代表产品已完成。

- [架构规格](../specs/2026-09-10-craft-runtime-design.md)
- [P1 计划](2026-09-10-craft-opencode-adapter.md)
- 当前源码基线：`e211610983e387316746b55e056d7208e83d46db`。
- wanwu 参考基线：`99df6e41d4141276553320b6f9a705d432c14bd7`。
- 工作区存在其他 React 迁移文档改动；本次仅新增 Craft 文件，未提交、未修改业务代码。

状态：pending / implementing / review / accepted / blocked。每项记录实现提交、验证命令/退出码、证据路径、未通过项与下一步。没有执行证据时不得填 accepted。

| 阶段/任务 | 状态 | 当前证据 | 下一步 |
|---|---|---|---|
| 架构与计划 | review | 已确认双 Runtime 方向；完整产品方案已补充 | 先评审完整产品方案，不继续实施 |
| P1 T01 wire profile | pending | wanwu 与官方文档参考；未固定实施版本 | 固定 OpenCode 版本并采集脱敏 fixture |
| P1 T02 内部契约 | pending | 计划内接口 | 身份与状态测试 |
| P1 T03 HTTP/SSE | pending | 计划内接口 | session 复用和握手测试 |
| P1 T04 事件/终态 | pending | 计划内接口 | 来源与消息关联测试 |
| P1 T05 Executor | pending | 计划内接口 | mock、race、真实协议验收 |
| P2 持久化/交互/恢复 | pending | 规格交付门槛 | 基于 P1 输出细化文件级计划 |
| P3 主 Runtime 接线 | pending | 已确认 tRPC-Agent-Go 编排 OpenCode | 基于 P2 冻结 Runtime 版本/API |
| P4 React 工作台 | pending | 沿用 React 迁移共享包职责 | 依赖 React 基础与 P3 |
| P5 隔离/恢复/发布 | pending | 规格验收矩阵 | 依赖 P4 真实闭环 |

## 本次验证范围

仅验证新增文档的本地链接、范围和接口一致性。没有执行 Go/前端测试、真实 OpenCode 调用或模型请求；没有构建或启动容器。文档检查通过不构成运行验收。
