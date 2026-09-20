# Mobile AI Office Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付以 Paseo 为移动底座、WeKnora 为唯一可信云执行后端的 iOS/Android 办公闭环。

**Architecture:** 26 个独立工作单元，M01 是必要的 expand 预重构，M02–M24 为功能/交付纵切，M25–M26 为平台验收。每个功能含自身移动入口、服务行为和测试；G01 为主控串行集成门禁，不是另一个 implementer Track。

**Tech Stack:** 复用 Go/Gin/Workbench/Session/Run；在独立 Expo 55 / React Native 0.83 容器逐模块适配 Paseo，沿用其独立 npm lock。根 pnpm 10.28.2 要求 Node >=26；当前 shell Node 22.22.3 不满足要求，执行前选择合规 runtime。Go 当前为 1.26.3。

**Spec:** [已发布规格 #3](https://github.com/1123786563/WeKnora-fork01/issues/3)；[本地规格](../specs/2026-09-20-mobile-ai-office-spec.md)。只读父 Issue，不修改或关闭。

状态：详细计划已完成定向审查与修订，拆分待用户确认。分支名是预定名称，不代表业务 worktree 已创建。当前三个 planning worktree 仅生产计划文档。

## Global Constraints

- Q1–Q20 全部继承；Task 使用 Session ID，WeKnora 为唯一运行与授权权威。Paseo/Happy 相邻源码仓库只读，不能直接改造后把结果当本仓交付。
- 主控记录固定集成基线；当前规划基线为 `9509df6b7f92bfffbf75bfef100c9917e2398800`。后续 Track 从其全部前置已集成的提交创建，不能全部从该旧基线提前创建。
- 最多 4 个实施 Track 同时运行；每个独立 branch + native managed worktree；一个 worktree 同时只有一个 implementer。审查只读。工作树 dirty state 不随意清除。
- 独占文件路径记入 DAG，调度前做路径/目录交集检查；同一文件不分派给两个并行 agent。跨轨共享路径只有固定整合 owner 可以修改，业务 implementer 不直接编辑。
- 根 manifests/lock、路由/DI、导出、root layout、迁移编号由固定整合 owner 在主控调度下逐轨串行更新；主控负责 cherry-pick、冲突协调与测试，不直接实现业务 Track。
- 每轨完成条件包括 scoped tests、移动/API 行为证据、独立需求与代码质量审查，以及共享装配后的复测。审查失败回原 owner 修复，禁止未经授权换模型或升档。
- 新增数据库变更需配 PostgreSQL/SQLite 方案；实际序号在每轨集成前重新扫描分配，同一固定迁移 owner 串行处理。不能以未编号 DDL/未挂载 API 充当可运行功能。
- 存储容量仅按字节检查和并发原子预占，不新增 Credits、套餐或计费业务。
- 保留原 Flutter 与现有 WeKnora Web；只有完整原生验收与回退条件满足才执行已授权切换，不随意删除历史客户端。
- 每个任务使用干净、自包含上下文；只给该票计划、规格、前置接口/集成 SHA、独占文件、测试命令与证据位置。发现超过单上下文容量须在开工前再拆，不能让一个 Track 吞下整个子系统。

## Review Focus

1. 空间/账号切换时旧 SSE 与请求迟到：不得混入新身份；由 M02/M05/M07 覆盖。
2. 停止超时或远端写结果未知：保留等待/对账，不自动重复；由 M05/M13/M21/M22 覆盖。
3. 用户已撤任务共享，但独立发布仍授权：仅发布副本可读；由 M15–M18 覆盖。
4. Run 启动和既有 PTY 并发写入：双向互斥且旧写租约失效；由 M13/M14 覆盖。
5. 无真实设备、无持久 provider 或配额不足：明确未验收/阻断，不能以 mock/跳过替代；由 M11/M12/M24–M26 覆盖。

## Track Map

| ID | 交付 | Blocked by | 独立计划 |
| --- | --- | --- | --- |
| M01 | Paseo 固定来源与可运行原生壳 | 无 | [计划](mobile-ai-office/m01-paseo-source-shell.md) |
| M02 | 登录、空间切换与可信云说明 | M01 | [计划](mobile-ai-office/m02-login-scope.md) |
| M03 | 任务列表、详情与工作区导航 | M02 | [计划](mobile-ai-office/m03-task-navigation.md) |
| M04 | Agent、知识资源与附件输入 | M02 | [计划](mobile-ai-office/m04-agent-resources.md) |
| M05 | 创建、继续运行与断线恢复 | M03, M04 | [计划](mobile-ai-office/m05-run-recovery.md) |
| M06 | 持久审批与版本冲突 | M05 | [计划](mobile-ai-office/m06-approvals.md) |
| M07 | 加密缓存、草稿与撤权清理 | M05 | [计划](mobile-ai-office/m07-offline-cache.md) |
| M08 | 最小锁屏通知与重验权深链 | M06 | [计划](mobile-ai-office/m08-notifications.md) |
| M09 | 任务独立持久工作区绑定 | M05 | [计划](mobile-ai-office/m09-workspace-binding.md) |
| M10 | 文件浏览与下载 | M09 | [计划](mobile-ai-office/m10-file-reader.md) |
| M11 | 工作区休眠与恢复 | M09 | [计划](mobile-ai-office/m11-workspace-resume.md) |
| M12 | 存储字节容量与超限只读 | M09 | [计划](mobile-ai-office/m12-storage-quota.md) |
| M13 | Run 与终端双向写入互斥 | M09 | [计划](mobile-ai-office/m13-terminal-exclusion.md) |
| M14 | 文件修改、删除与显式导入 | M10, M12, M13 | [计划](mobile-ai-office/m14-file-mutations.md) |
| M15 | 任务对话与运行只读共享 | M05 | [计划](mobile-ai-office/m15-task-sharing.md) |
| M16 | 共享文件、产物读取与撤权 | M10, M15 | [计划](mobile-ai-office/m16-shared-resources.md) |
| M17 | 固定版本空间产物发布 | M10, M15 | [计划](mobile-ai-office/m17-artifact-publication.md) |
| M18 | 固定产物发布到指定知识库 | M04, M17 | [计划](mobile-ai-office/m18-knowledge-publication.md) |
| M19 | 个人代码平台连接与仓库克隆 | M09, M13 | [计划](mobile-ai-office/m19-git-clone.md) |
| M20 | Git 状态、Diff 与云端本地提交 | M19 | [计划](mobile-ai-office/m20-git-local.md) |
| M21 | 精确分支推送审批与结果对账 | M06, M20 | [计划](mobile-ai-office/m21-git-push.md) |
| M22 | PR 创建审批与结果对账 | M21 | [计划](mobile-ai-office/m22-git-pr.md) |
| M23 | Home 真实数据聚合与入口 | M04, M06, M17 | [计划](mobile-ai-office/m23-home.md) |
| M24 | 构建入口、切换准备与来源复核 | M01, M02, M03, M04, M05, M06, M07, M08, M09, M10, M11, M12, M13, M14, M15, M16, M17, M18, M19, M20, M21, M22, M23 | [计划](mobile-ai-office/m24-release-entry.md) |
| M25 | iOS 真实云端验收 | M24 | [计划](mobile-ai-office/m25-ios-acceptance.md) |
| M26 | Android 真实云端验收 | M24 | [计划](mobile-ai-office/m26-android-acceptance.md) |

## Worktree 与调度

- [ ] 用户确认具体粒度/阻塞边后才发布子 Issue 与启动实施；沿用已指定的 Superpowers 并行方式，不再次询问执行方法。
- [ ] 用 native worktree 工具创建本次 integration worktree/branch，导入本任务必要规格计划文档；排除主 checkout 其他任务未提交修改。
- [ ] 对就绪 frontier 最多派 4 个 Track。每个 Track 创建 DAG 记录的独立分支；native worktree 返回路径记录到执行台账。运行环境在该目录检查。
- [ ] 读取 `sdd_implementer` / `sdd_task_reviewer` / `sdd_final_reviewer` 当前角色配置；请求模型和推理与角色一致、干净上下文、禁止子代理再派代理。记录 task/role/requested model/effort/agent ID，实际模型不可观察则写未验证。
- [ ] Track 提交仅含独占文件；固定整合 owner 串行完成已审核装配请求。主控逐轨 cherry-pick 至 integration，不将未审大批改动一次混入。
- [ ] 整合后复跑本轨外部 API 与移动演示，再解除其阻塞边。真实验收依赖缺失必须保留明确状态，不标 complete。

## G01 最终集成门禁

- [ ] 汇总 M01–M26 的测试与需求/代码质量审查记录，确认所有阻塞已解除、无共享文件并发写冲突。
- [ ] 在集成目录运行 `go test ./...`。
- [ ] 运行 `pnpm test:shared`、`pnpm typecheck:shared`、`pnpm test:web`、`pnpm typecheck:web`、`pnpm build:web`。
- [ ] 运行 `pnpm test:desktop`、`pnpm typecheck:desktop`、`pnpm build:desktop-renderer`、`pnpm test:embed`、`pnpm typecheck:embed`、`pnpm build:embed`，覆盖共享 API 改动消费者。
- [ ] 按 M24 冻结的移动容器脚本运行完整移动测试、typecheck、Paseo Web export 和 iOS/Android 构建；现 root mobile filter 不可用，不能假称已覆盖。
- [ ] M25 与 M26 分别提供真实设备 + WeKnora 云端 + 持久沙箱 + Git push/PR + 通知证据；当前 `adb` 未在 PATH，必须建立 Android 执行条件，不能静默跳过。
- [ ] 派 `sdd_final_reviewer` 复核集成分支、规格覆盖、回归和延期项；所有重大问题修复并复测后才报告完成。

## 发布规则

每个 Issue 草案只描述纵向行为、可检验验收与真实阻塞，不嵌入实现路径/代码；文件与代码细节位于各自独立计划。发布后用原生 GitHub blocked-by 建立依赖，失败则明确记录降级文本。父规格 #3 不修改。计划文档先放入可访问的固定提交，票中采用唯一计划链接，不用未推送的本地路径冒充远程证据。
