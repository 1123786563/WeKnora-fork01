# Pass B 执行 DAG（execution-dag.md）

> 调度事实源是同目录 `execution-dag.json`（33 节点，字段与状态以 JSON 为准）；本文件是其人读视图：Mermaid 依赖图 + 任务表 + 修正记录。执行公约见主 checkout `.superpowers/sdd/passb/conventions.md`（git-ignored 区）。
>
> 基线：integration base = `b1a3d6dd8`（任务书称 `29c1e5635`，实际 HEAD 多 2 个纯前端 parity 提交，不触及 Pass B 文件，全部按 `b1a3d6dd8` 起算）；Pass A 验收 `78f18915f`。计数基线 633 路由 / 23+23 任务类型 / 58 生命周期挂点 / 537 迁移文件 + 396 legacy 文件 / 105 import 例外——语义为**三方一致**（guard 实测 == pass-a-acceptance.md 台账 == manifests 发现值，b0 审计裁定 F5），不是硬编码断言。
>
> 来源：`docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`、`docs/plans/passb/00-contract-and-ownership-freeze.md`、`docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §11–§17.2、16 份 `docs/architecture/moves/*.yaml`、13 份 `docs/architecture/passb/*.md` brief，叠加架构事实实测（跨模块 import、同包未导出互耦、*Handler 共享）校正。

## Mermaid DAG

```mermaid
graph TD
  subgraph B0
    b0["b0 契约与所有权冻结<br/>(B0.1–B0.6 + F1–F6 裁定)"]
  end
  subgraph B1["B1 四路并行"]
    b1id["b1-identity (27)"]
    b1ai["b1-airesource (33)"]
    b1cm["b1-commercial (8)"]
    b1ex["b1-execution (21)"]
  end
  subgraph IB1["IB1 barrier"]
    ib1["ib1 基础能力集成<br/>门面 current 化"]
  end
  subgraph B2["B2 并行程序"]
    k0["b2-k0 端口/所有权冻结"]
    k1["b2-k-ingest (9)"]
    k2["b2-k-retrieval (29)"]
    k3["b2-k-wikifaq (18)"]
    k4["b2-k-process (28+18 workers)"]
    k5["b2-k-integration (K5)"]
    aca["b2-ac-definition (25a)"]
    acb["b2-ac-skills (25b)"]
    acc["b2-ac-market (25c)"]
    ds["b2-datasource (26, 4)"]
    appc["b2-appconnector (27, 7)"]
  end
  subgraph IB2["IB2 barrier"]
    ib2["ib2 核心能力集成"]
  end
  subgraph B3["B3 并行程序"]
    r1["b3-r-memory (R1)"]
    r2["b3-r-tools (R2)"]
    r3["b3-r-engine (R3)"]
    r4["b3-r-protocol (R4)"]
    r5["b3-r-integration (R5)"]
    cqh["b3-conv-queryhistory (35a)"]
    cs["b3-conv-session (35b, 40)"]
    ch["b3-channels (36, 7)"]
    ins["b3-insights (37, 6)"]
  end
  subgraph IB3["IB3 barrier"]
    ib3["ib3 运行时与交互集成"]
  end
  subgraph B4["B4 三路并行"]
    wb["b4-workbench (40, 19)"]
    cr["b4-craft (41, 30)"]
    sp["b4-systempolicy (42, 5+1)"]
  end
  subgraph IB4["IB4 barrier"]
    ib4["ib4 用户工作与治理集成"]
  end
  subgraph B5
    b5["b5 兼容/横向宿主清理 + 终验<br/>(final)"]
  end

  b0 --> b1id & b1ai & b1cm & b1ex
  b1id & b1ai & b1cm & b1ex --> ib1
  ib1 --> k0 & aca & acb & ds & appc
  k0 --> k1 & k2 & k3
  k1 & k2 --> k4
  k3 --> k5
  k4 --> k5
  aca & acb --> acc
  k5 --> ds
  k5 & acc & ds & appc --> ib2
  ib2 --> r1 & r2 & cqh & ins
  r1 & r2 --> r3
  r3 --> r4 --> r5
  cqh --> cs --> ch
  r5 & cs & ch & ins --> ib3
  ib3 --> wb & cr & sp
  wb & cr & sp --> ib4 --> b5
```

## 任务表

| ID | 节点 | Phase | Role | 前置 | 计划 | 模式 | 关键耦合 / 裁定 |
|---|---|---|---|---|---|---|---|
| b0 | 契约与所有权冻结 | B0 | work | — | `00-contract-and-ownership-freeze.md`（已存在） | serial | task_ids B0.1–B0.6；notes 收录审计裁定 F1–F6（movemanifest 共享库 / façade planned-not-current / 事件三元组判重 / replay 语义 / 计数三方一致 / Makefile .PHONY 风格） |
| b1-identity | Identity 边界（27） | B1 | work | b0 | `10-identity.md` | parallel | system→identity 未导出（getJwtSecret 等，user.go）导出裁定 |
| b1-airesource | AI Resource 边界（33） | B1 | work | b0 | `11-airesource.md` | parallel | 下游最重：agentruntime 67 / conversation 7 / knowledge 7 处引用 |
| b1-commercial | Commercial 边界（8） | B1 | work | b0 | `12-commercial.md` | parallel | knowledge→commercial 3 条未导出（model_usage）须导出；payment/usage 高风险差分 |
| b1-execution | Execution 边界（21） | B1 | work | b0 | `13-execution.md` | parallel | execution→agentcatalog 4 条未导出断链 + browserskill/sandbox_terminal_ws 为 *Handler 方法文件 |
| ib1 | 基础能力集成 | IB1 | barrier | b1 四支 | `19-foundation-integration.md` | serial | 集成工程师独占 router/container/bootstrap/migration/go.mod；门面 current 化（F2） |
| b2-k0 | Knowledge 端口冻结 | B2 | work | ib1 | `20-knowledge-program.md` K0 | serial | 分配 Chunk/KnowledgeBase/Tag/semantic 共享类型，K1-K3 并行前提 |
| b2-k-ingest | K1 Ingest（9） | B2 | work | b2-k0 | `21-knowledge-ingest.md` | parallel | kb_activity.go（knowledge.yaml:155）搬迁须为 datasource/conversation 调用方导出端口 |
| b2-k-retrieval | K2 Retrieval（29） | B2 | work | b2-k0 | `22-knowledge-retrieval.md` | parallel | conversation→searchutil 10 处 + 5 条未导出调用方处理 |
| b2-k-wikifaq | K3 Wiki+FAQ（18） | B2 | work | b2-k0 | `23-knowledge-wikifaq.md` | parallel | wiki_fixer_scope.go 为 *Handler 方法文件（freeze:202） |
| b2-k-process | K4 Process（28） | B2 | work | b2-k-ingest, b2-k-retrieval | `24-knowledge-process.md` | serial | 18 worker handler 实现；注册行禁改 |
| b2-k-integration | K5 Knowledge 集成 | B2 | work | b2-k-process, **b2-k-wikifaq** | `20-knowledge-program.md` K5 | serial | **CORR-1**：骨架补 wikifaq 前置 |
| b2-ac-definition | 25a 定义/版本 | B2 | work | ib1 | `25a-agent-definition-version.md` | parallel | agentcatalog→conversation 7 条未导出（session_*.go 系）导出/shim 裁定 |
| b2-ac-skills | 25b Skill 目录 | B2 | work | ib1 | `25b-skill-catalog-install.md` | parallel | tenant_skill_reaper.go 导出化收口 execution→agentcatalog 4 条 |
| b2-ac-market | 25c Marketplace | B2 | work | b2-ac-definition, b2-ac-skills | `25c-marketplace.md` | serial | 消费 25a/25b release/install 门面 |
| b2-datasource | Data Source（4） | B2 | work | ib1, **b2-k-integration** | `26-datasource.md` | serial | **CORR-2**：datasource_service.go（datasource.yaml:59）调用 kb_activity.go（knowledge.yaml:155）未导出 5 条，先搬必断链 |
| b2-appconnector | App Connector（7） | B2 | work | ib1 | `27-appconnector.md` | parallel | →commercial/agentruntime 引用走门面；测试夹具随迁 |
| ib2 | 核心能力集成 | IB2 | barrier | b2-k-integration, b2-ac-market, b2-datasource, b2-appconnector | `29-core-capability-integration.md` | serial | K 序 + 25 序集成；R0 复核后放行 B3 |
| b3-r-memory | R1 memory/modelcontext | B3 | work | ib2 | `31-agentruntime-memory.md` | parallel | agentruntime→airesource 67 处引用改走门面 |
| b3-r-tools | R2 tools | B3 | work | ib2 | `32-agentruntime-tools.md` | parallel | repository/agent_run.go 的 runScope 等为 craft B4 消费导出 |
| b3-r-engine | R3 engine/run/approval | B3 | work | b3-r-memory, b3-r-tools | `33-agentruntime-engine.md` | serial | 三处跨 owner 断链：helpers.go searchResultFromMap、artifact_download.go artifactHandle、agent_run.go *Handler 方法；Agent Run recovery 高风险差分 |
| b3-r-protocol | R4 native/tRPC/OpenCode | B3 | work | b3-r-engine | `34-agentruntime-protocol.md` | serial | engine/protocol 所有权按 freeze:193-196 |
| b3-r-integration | R5 runtime 集成 | B3 | work | b3-r-protocol | `30-agentruntime-program.md` R5 | serial | 恢复竞态 -race；13 条 lint 已知债清偿 |
| b3-conv-queryhistory | 35a QueryHistory（4） | B3 | work | ib2 | `35a-conversation-queryhistory.md` | serial | notes 收录试点复用裁定全文（选择性采纳：文件级移植，禁 merge/cherry-pick） |
| b3-conv-session | 35b Session（40） | B3 | work | b3-conv-queryhistory | `35b-conversation-session.md` | serial | **CORR-3**：保守串行（framework:179 允许 application 并行）；*Handler（67 字段）拆分收口 |
| b3-channels | Channels（7） | B3 | work | b3-conv-session | `36-channels.md` | serial | 等 Conversation 公共 session API（framework:186）；10 别名包删除 |
| b3-insights | Insights（6） | B3 | work | ib2 | `37-insights.md` | parallel | 只读边界；analytics 方言差分 |
| ib3 | 运行时与交互集成 | IB3 | barrier | b3-r-integration, b3-conv-session, b3-channels, b3-insights | `39-runtime-interaction-integration.md` | serial | Task/Artifact 契约冻结供 B4（framework:204） |
| b4-workbench | Workbench（19） | B4 | work | ib3 | `40-workbench.md` | parallel | artifact_download.go 双面耦合点；handler/session 所有权精确不相交 |
| b4-craft | Craft（30） | B4 | work | ib3 | `41-craft.md` | parallel | runScope 等消费 B3 已导出端口；Craft Artifact 高风险差分 |
| b4-systempolicy | System+Policy（5+1） | B4 | work | ib3 | `42-system-policy.md` | parallel | KnowledgeHousekeeping 窄端口（freeze:209）；system→identity 收口 |
| ib4 | 用户工作与治理集成 | IB4 | barrier | b4 三支 | `49-user-work-governance-integration.md` | serial | 冻结 B5 清理台账（实际剩余路径） |
| b5 | 终清理 + 终验 | B5 | final | ib4 | `50-final-cleanup-and-acceptance.md` | serial | 零 alias/零例外/零遗留；`pass-b-acceptance.md` 对 Spec §17.2 |

## 骨架修正记录（相对派发骨架）

- **CORR-1** `b2-k-integration`：depends_on 由 `[b2-k-process]` 修正为 `[b2-k-process, b2-k-wikifaq]`——K5 是 K1–K4 全量汇聚（framework:119-120），漏 wikifaq 会丢 18 文件、`wiki_fixer_scope.go` 与对应别名。
- **CORR-2** `b2-datasource`：depends_on 由 `[ib1]` 修正为 `[ib1, b2-k-integration]`——datasource→knowledge 5 条同宿主包（`internal/application/service`）跨 owner 未导出调用（调用方 `datasource_service.go` 归属 datasource.yaml:59，定义方 `kb_activity.go` 归属 knowledge.yaml:155，本会话 grep 双向实证）。保守串行；IB1 若裁定共享 shim/提前导出端口，按基线变更流程回写 DAG 降级并行。
- **CORR-3** `b3-conv-session`：保持骨架串行（35b 依赖 35a）——framework:179 允许 application 层并行、装配串行（QueryHistory 先集成），但 handler/session 宿主包 agentcatalog↔conversation 7 条未导出互耦 + `conversation/module.go` 门面共享，DAG 层取串行为默认；35 计划内部可安排 application 层并行预写。
- **CORR-4** 30 计划的 R0（engine/protocol 所有权 SERIAL CHECK）不设独立节点——裁定已由 B0.3 Step 2 冻结（freeze:193-196），作为 ib2→b3-r-* 派发前置检查写入各节点 required_contracts。

已核查（本会话执行）：无环路、无悬挂前置、无重复节点 id、每节点 19 个必备字段齐全（`python3` 校验脚本，33 节点通过）；同一文件多写由 b0 产出的 ownership-matrix 单一属主约束 + 集成节点独占装配文件消除；「上游未冻结就开下游」由 façade planned/current 两区状态（F2）与 barrier 边消除。

## 字段与回填

字段语义、`status/base_sha/head_sha/review_status/task_ids` 回填规则、门禁 argv 展开方式（`["test",pkgs]`→`go test -count=1`、`["make",t]`、`["sh","-c",cmd]` 中 `$PASSB_BASE_SHA` 替换）见 `.superpowers/sdd/passb/conventions.md`。计划文件 `10-*.md`–`50-*.md` 尚未撰写，按 framework「Child Plan Authoring Order」逐阶段编写并在评审通过后回填各节点 `task_ids`。
