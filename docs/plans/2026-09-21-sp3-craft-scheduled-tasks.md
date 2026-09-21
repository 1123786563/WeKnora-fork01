# SP3 · Craft 定时任务实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** craft_scheduled_tasks 两表+七端点 API+30s dispatcher+asynq 执行器+stuck sweeper+前端四页（GAP-MATRIX C-23）。

**Architecture / Spec:** `docs/specs/2026-09-21-sp3-craft-scheduled-tasks-design.md`（**执行者必读**——数据模型 DDL/API 语义/调度执行/前端全部在 spec，本计划只列任务与步骤）。

**Tech Stack:** Go（gin/GORM/asynq/robfig-cron）、React（craft 自管路由）、node:test。

## Global Constraints

- 迁移双轨成对 up/down；**迁移号提交时重查**（ls 两轨下一可用——并行会话活跃）
- API 权限：Viewer+ apiKeyChat+repo 层属主过滤（错任务 404 不泄露）；cron 最小分钟粒度
- Dispatcher 认领=UPDATE CAS（WHERE next_run_at=旧值）；执行复用 StartRun 边界（**R4：属主身份内部 craftScope，不走 API key**）；SKIP_IF_RUNNING 写可见 SKIPPED 行
- 每 commit：porcelain+`git diff --cached --stat` 双核对，绝不 add -A
- 预存失败基线甄别（stash/worktree 对照）；R1 预授权/R2 预算聚合/R3 重试=范围外

## 任务

### Task 1: 迁移+模型+repository
两表迁移（spec §2 DDL）+ `internal/types/craft_scheduled.go`（两 struct+常量集+cron 校验 helper）+ `internal/application/repository/craft_scheduled.go`（CRUD/原子认领 ClaimDueTasks/插入 run/更新 run/查 runs keyset 分页/查 in-flight）+ interfaces + container Provide + 测试（sqlite：CRUD/认领 CAS 并发模拟【两 goroutine 同 claim 只一成功】/runs 分页/属主过滤）。

### Task 2: cron 编译器+service CRUD
`internal/application/service/craft_scheduled.go`：editor 三模式编译（interval 分钟/daily HH:MM/advanced 直传→5 字段 cron；robfig parser 校验）+ NextRuns(preview n) + Create/Update 重算 next_run_at + List/Get/Delete 软删幂等 + RunNow（插 manual QUEUED run 返回 runID——入队归 Task 4 接）。测试：三模式编译矩阵/非法 cron 400/next 计算/软删幂等。

### Task 3: handler+路由
`internal/handler/craft_scheduled.go` 七端点（spec §3 表；run_immediately 可选；runs 分页 before/limit）+ routes 挂 craft 组旁（照 craft sessions 组模式）+ apiKeyGroup 声明。测试：矩阵（创建/详情+preview/更新/软删 204 幂等/run-now 202/runs 分页/属主 404）。

### Task 4: dispatcher
`internal/application/service/craft_scheduled_dispatch.go` + container 启动（照 cleanup StartPeriodicSweep 模式，30s ticker，env 可禁）：ClaimDueTasks(50)→逐任务 skip 判定（in-flight→SKIPPED 行）→插 QUEUED run→asynq `TypeCraftScheduledRun`（types/task.go 常量+QueueSync+mux/Lite 双注册+MaxRetry 1+TaskID `craftsched:<runID>`）→CAS 已在 repo。测试：due 扫描/CAS 并发/skip 策略/入队断言（fake enqueuer）/next_run_at 推进。

### Task 5: executor+stuck sweeper
`ProcessScheduledRun(ctx, task)`：幂等护栏→queued→running→属主 craftScope 建会话（kind 标记——grep craft_sessions.kind 现值域后定标记方式）→StartRun（prompt turn 0）→**终态回写**：grep craft run 完成通知路径找挂点（run 行 succeeded+summary 120 字/failed+error_class/awaiting_interaction）。stuck sweeper（queued>15m/running>45m→failed(stuck)——并入 dispatcher ticker 的分钟级检查或独立）。测试：全链（sqlite fixture+fake craft StartRun 边界【若 service 依赖重则抽 seam】）/幂等重入/三终态/sweeper 两窗。

### Task 6: contracts+api-client
`packages/contracts/src/craft-scheduled.ts`（Task/Run 类型+parse）+ api-client `craftScheduled` 域（list/create/get/patch/remove/runNow/runs）。测试照 SP11-14 模式。

### Task 7: 前端四页+i18n
craft 自管路由 `parseCraftRoute` 扩展 `/craft/tasks`+`/new`+`/:id`（列表卡片/表单三模式/详情+run 历史表+run-now+暂停恢复+删除确认）；api.ts 走新 client 域；i18n 5 locale。测试：路由解析纯函数+表单编译纯函数+既有 craft 测试不破。

### Task 8: 回归冒烟+台账
回归（本线全绿+预存判归属）；冒烟（共享栈尽力：1 分钟任务两次 fire）；evidence+ROADMAP SP3 ✅+GAP-MATRIX C-23 ✅；遗留登记（R1-R3+30s 粒度+run 历史无清理）。

## Self-Review
spec §2→T1、§3→T2/T3、§4 前半→T4、后半→T5、§5→T7、§6→T8；执行期确认点：craft_sessions.kind 值域、craft run 终态通知挂点、StartRun 依赖注入 seam。
