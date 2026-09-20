# Mobile Office 固定整合 Owner 操作计划

> **For agentic workers:** Use superpowers:subagent-driven-development. Controller coordinates; one designated sdd_implementer owns shared assembly writes, reviewers are read-only.

**Goal:** 让每张纵切在正式路由、依赖装配和实际迁移下可运行；根文件不由并行功能 agent 竞相修改。

**Architecture:** 主控在独立 integration worktree 串行集成。一个固定整合 owner 承担共享路径实际修改，功能 owner 只提供已审的行为模块和本计划定义的装配请求。G01 是最终检查，不能代替逐轨装配。

**Spec:** [规格 #3](https://github.com/1123786563/WeKnora-fork01/issues/3)。

## 独占路径与实际入口

| 共享路径 | 整合责任 |
| --- | --- |
| `internal/router/routes_workbench.go`、`internal/router/router.go` | 挂载每轨独立 Register 函数，读取继续使用服务端 scope，写操作保持原 owner/授权检查。 |
| `internal/container/container.go` 及依赖装配文件 | 把真实 repository/service/provider 注入 handler；没有可用 provider 时保留明确不可用，不使用假成功实现。 |
| `packages/contracts/src/index.ts`、`packages/api-client/src/index.ts` | 只导出已经实现和测过的 facade；不创造另一套 Run wire status。 |
| `apps/mobile-next/app/_layout.tsx` 及共享 provider 入口 | 将每轨路由/视图挂入实际导航，统一认证 scope 与 CloudWorkspaceClient 生命周期。 |
| 根与移动容器 `package.json`、pnpm workspace/lock、移动 npm lock | 安装已选依赖、维护唯一对应锁文件；根与容器版本边界按 M01/M24 明确，不盲目混装。 |
| `migrations/versioned/`、`migrations/sqlite/`、共享 migration schema 测试 | 从每轨 DDL 合同生成实际 PG/SQLite 迁移对，按当前最高号串行分配并验证升级，不占用并发任务序号。 |
| `internal/handler/session/agent_run.go`、共享 Workbench read/commands/start 入口、`internal/application/service/craft_session.go`、已有终端 ticket/WS bridge 入口 | 当多轨需要修改同一个文件时，由整合 owner 唯一写入。M05/M13/M15 计划必须说明目标函数与行为，不能只写“由 controller 补线”。每轨集成后验真实路由，防绕过 admission、read ACL 或 PTY 排它。 |
| `apps/mobile-next/src/contracts/workbench.ts`、`apps/mobile-next/src/features/auth/AuthController.ts` | 固定 owner 按 M02/M05/M06/M07 的类型与缓存回调 handoff 串行装配，功能实现者不直接编辑。 |
| `apps/mobile-next/src/cloud-workspace/CloudWorkspaceClient.ts` | 唯一组合根；后续功能分别实现专属端口，由整合 owner 组合，不由两个 Track 修改此文件。 |

其他任何文件若在两个 Track 写集合出现，主控先把它显式纳入固定 owner 表，再核对相关功能票的装配请求；若不能拆出安全可测端口，则改成串行依赖，且同一文件仍由固定 owner 落笔。

## 每轨装配请求（必须在该票计划中填写）

每个请求列出：已审功能 commit；真实共享文件和函数；新增模块的确切构造器与依赖签名；接入前后行为；新增 schema 的表、键、唯一约束与回滚；所需 exports/native dependencies；真实 API 路径与最小输入/期望输出；受影响的旧入口回归用例。没有这些信息则返回功能 owner 补计划，不能让整合 owner猜测业务。

## 逐轨串行步骤

- [ ] 读取 Track issue、独立计划、前置集成 SHA、角色配置和文件 lease，确认该 Track scoped tests 与独立审查已通过。
- [ ] 在 integration worktree 检查 `git status --short`，确认没有另一 implementer 持有写 lease。主控从台账读取该轨已审 commit，执行 `git cherry-pick "$TRACK_COMMIT"`；不得整包带入主 checkout dirty 差异。
- [ ] 固定整合 owner 按该票装配请求修改共享入口。新增 DB schema 在本轨此时分配版本号，并在对应迁移测试库执行升级/读取/重启测试。不得等所有轨结束才首次创建实际表。
- [ ] 运行本轨独立计划的 targeted Go/移动测试，再用真实挂载 router 验证允许与拒绝请求。新文件模块单测通过但真实入口未调用时，本轨仍未完成。
- [ ] 只读 sdd_task_reviewer 复核功能加装配后的 diff，重点检查 owner/read-share 区分、scope、request/decision 幂等、版本冲突与未知结果。
- [ ] 审查失败回相应唯一文件 owner 修复；主控重跑受影响测试。通过后才把完整集成 SHA 写入台账并解锁消费者。
- [ ] 下游 Track 的 worktree 从该集成 SHA 创建；已有独立 worktree 只有经检查无 dirty 后才以明确 rebase/新基线继续，不自动覆盖用户内容。

## 接口与迁移的稳定顺序

- M02 冻结认证 scope 与只负责组合的 CloudWorkspaceClient；M03/M05 在专属 task/run 端口实现，不复制 wire authority。
- M09 定义 workspace binding 的持久 schema 与能力读取；M11/M12/M13 各增加自己的持久数据/约束。迁移号由整合时的实际仓库状态决定，业务 DAG 与迁移命名不混为一套编号。
- M15 定义成员只读访问解析器，并逐读入口接入；start/continue/decision/terminal/Git 写路径仍走更严格授权，不能把共享读授权复用于写。
- M17 固定 publication 版本与独立 ACL/object reference；M18 复用 publication 到知识资源授权，不复制任务共享权限。
- M19 定义 Git provider/broker/sandbox 操作端口；M21/M22 绑定已有 Run 与审批，新增的外部操作状态只跟踪副作用与对账，不能成为第二 Run 状态机。

## G01 门禁

主控按主计划执行 Go 全集、共享 TS、Web/Desktop/Embed 回归、移动完整测试和构建，读取 M25/M26 的物理设备证据，最后派 sdd_final_reviewer。任何真实依赖缺失必须记录为未验收；单测、fixture、模拟器和物理设备证据分别列出。

本文件属于调度/装配计划，不是新增产品票，也不创建独立的 G01 implementer worktree。
