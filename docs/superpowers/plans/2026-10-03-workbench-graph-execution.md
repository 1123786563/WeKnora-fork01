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
