# Workbench→Graph 执行集成实施计划（#71 上游缝）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 workbench 准入的移动任务 Run 真正经 durable executor 执行并产出事件流/材料——在准入时冻结服务端解析的 model 身份与 AgentConfig（D8 判词预留的集成点：`agent_run_graph.go:561` 的 `ModelID==""` 显式失败分支），打通 app 端任务主链，为 #71 收口与材料/分享面供能。

**Architecture:** 事实源=graph 快照构建器 `BuildDurableRunSnapshot`（agent_run_graph.go:150）与 :438 调用方的能力解析先例；workbench 准入写快照于 `admission.go admitPending`（现含 admission-map + usage binding，D8 已类型化）。集成形态：准入时服务端解析 model/config（复用 :438 的共享能力解析缝），将 graph 核心（version/query/model/config/runtime）与 admission 身份组合进同一 durable 快照；executor 侧 D8 分支自然放行（ModelID 非空），事件/材料链复用既有 durable 执行面。模型解析失败=准入拒绝（显式原因），绝不延迟失败或静默换型。

**Tech Stack:** Go（workbench/service、application/service、agentruntime）；SQLite 测试；iOS/Android 模拟器活体验证（栈沿用）。

## Global Constraints

- 服务端权威：model/config 只来自服务端解析缝；不接客户端 JSON 字段。
- fail-closed：解析失败→准入拒绝带显式短码原因；不静默降级/换型（先例：`model %q unavailable` 终态语义）。
- 不可变快照契约：graph 核心冻结后不可变（D8 的严格解码 + rogue 拒绝保持）；不改 D1-D9 已修语义。
- TDD 每任务；提交带 `(WB-GRAPH)`；不 push（轮末统一）、不动 GitHub。
- 工作区 `.superpowers/sdd/2026-10-03-wb-graph/`。

---

### Task 1: 集成设计调查（只读）
- [ ] 测绘 :438 调用链的共享能力解析（query/images/modelID/rerank/config 的服务端来源与失败语义）；workbench `admitPending` 快照写点与可用依赖注入面；executor 对组合快照的消费路径（actor 上下文/craft 边界不适用判据）；产出设计纪要（字段映射表、解析缝选择、失败短码、测试清单）→ task-1-report.md。

### Task 2: 实现（条件于 Task 1 设计）
- [ ] RED：①组合快照过严格解码且 ModelID 非空 ②准入路径真实 Start→Get→Parse 含 graph 核心 ③解析失败→准入拒绝带短码（不落 Run）④executor 对组合快照进入正常执行（fake model 断言事件产出）。
- [ ] GREEN：实现冻结与桥接（owned=workbench admission + 必要的 service 缝扩展；不改 executor 既有语义仅使 D8 分支自然通过）。
- [ ] 全量门：workbench/service 两包 + `go build ./...` + `git diff --check`；提交。

### Task 3: 独立审查（opus）+ 修复循环
- [ ] 审查冻结契约/失败语义/安全（不越权解析、无客户端字段）；Critical/Important 修复循环。

### Task 4: 活体验证 + 判词
- [ ] 模拟器（iOS 或 Android 任一）真实提交任务→事件>0→SSE 流式→材料产出→分享面（如产品面已接线）；证据入 t39 目录新小节。
- [ ] 台账终局段（本计划文档 + t39-acceptance.md 追加）；#71 前置判定更新。

## Self-Review
覆盖：集成缝两端（准入冻结/executor 放行）→Task 2；安全与契约→Task 3；活体闭环（events>0，#69 残余项）→Task 4；条件任务契约明确无占位。

---

## 终判（2026-10-03，Task 4 活体验证后）

### 逐目标结果

| 目标 | 结果 | 证据 |
|---|---|---|
| 提交→Start 准入→run 执行（events>0） | **evidenced**：202 admitted；run2/3/5 均 succeeded、events=2（run_started+run_completed，D8 时代=0）；run5 助手消息 4852 字（glm-5.3 真实回合） | `android-evidence/t39/2026-10-03-wb-graph/`（README+logs/db-*） |
| SSE 流内容 | **evidenced**：`GET /executions/{id}/events?version=2` 200、**5484 bytes** 流到 Android app（okhttp）；detail「已接收事件：2·settled·已同步」 | logs/nginx-sse-hits.log、17-run5-detail-live.png |
| detail 出结果 | **evidenced**：时间线 2 活动+终态投影；内容在 messages/snapshot（run_completed 载荷 5259B） | 09/10/17 截图、logs/backend-execution.log |
| 材料产出+分享面 | **部分**（如实）：材料页渲染+`/artifacts` 200 空——quick-answer 文本回合不产工件行；分享动作需 materialId，面未呈现（同 #69 flow7 口径）；`/delivery` 500=`#31 已知残留`（workbench_notifications 迁移未跑） | 11/12 截图 |
| 解析失败短码+无 Run 行 | **evidenced**：无模型 agent→**409** `model_unresolved: ...`（活体另证 `rerank_unresolved`）；拒绝前后 agent_runs 计数 4/4 不变 | 13 截图、logs/failcase-evidence.txt |

### 提交（4，均未 push）

- `8558579a1` test：生产 freezer 失败短码表测试（review I-1；纯映射 5 例 + 真实 sessionService 链 4 失败+快乐路径）
- `e0d63bae1` fix：freezer Invoke 移至 provideAgentSecurity 之后（生产 boot panic，测试位未覆盖调用序）
- `5909d58da` fix：首次 graph 解析失败 HTTP 500→409，与重放口径对齐（+handler 集成测试）
- `1541b95c8` test-evidence：活体验证证据（25 文件 sha256+scrub）

### #71 前置判定（更新）

**本计划（#69/#70 遗留的「workbench→graph 执行集成」阻塞）已活体闭环**：准入冻结→executor 执行→事件流/SSE/失败短码全链 evidenced。对 #71（T41 跨平台发布证据矩阵与首版验收）：

- 移动任务主链维度：执行集成前置**解除**；任务详情/列表/失败语义可进矩阵。
- 仍欠：①`/delivery` 500（#31 迁移残留）阻断送达面证据；②材料/分享面需能产工件的 agent 路径（如 craft/导出类）才有实体可截；③FCM/APNs=blocked-env 不变；④B6 台账侧 #55/#65 等上游仍 pending，#71 排程以台账为准。
- 环境口径沉淀：workbench 执行要求 agent 自带 model_id（+需要时 rerank_model_id）——租户默认模型不进 freezer 解析链（watch-item 实测收窄）；t01live 栈 provisioning 见证据目录 logs/provisioning.sql。

### 跟进项（承 Task 3 review，均不阻塞）

| 级别 | 项 | 状态 |
|---|---|---|
| Minor | `"is unavailable"` 子串匹配过宽（admission_graph_freeze.go 短码映射） | 未修（仍 fail-closed，仅短码可能失真） |
| Minor | coreFields 并入静默覆盖同名 admission 键 | 未修（当前键集不相交） |
| Minor | adopted/marketplace agent 无条件过 freezer（与 Run 行 pin 的 Version/Release 可能配置漂移） | 未修**且本轮活体未覆盖**（只测了 builtin+本租户 custom）——下轮补 adopted 用例 |
| 新 | delivery 500=workbench_notifications 迁移（#31 残留） | 归 #31，非本轮面 |
| 新 | quick-answer 无工件行→材料/分享空 | 产品面缺口，归 #71 矩阵裁量 |
