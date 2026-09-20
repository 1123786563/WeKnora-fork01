# Semantica Q1–Q17 Rebaseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不实施生产能力的前提下，校准 2026-09-11 的 24 个 Semantica 实施任务，使其遵从 2026-09-20 已确认的 Q1–Q17 和后续能力验证边界。

**Architecture:** [ADR-0002](../../adr/0002-semantica-independent-service.md) 与本 rebaseline 优先于 2026-09-11 旧计划及其未验证建议。旧计划仍是 24 个细化任务的唯一执行拆分；本文件只映射必须校准的边界、测试和证据门禁，不重写实现步骤，也不授权生产实施。

**Tech Stack:** Go、Python、gRPC、Semantica、现有 PostgreSQL/SQLite/图与对象存储基础设施、React；具体上游版本、协议和持久化方案均待验证。

**Spec:** [ADR-0002](../../adr/0002-semantica-independent-service.md)、[原规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)、[总计划](2026-09-11-semantica-implementation.md)。当前可复现先导证据见 [README](semantica/evidence/2026-09-20/README.md)、[REPORT](semantica/evidence/2026-09-20/REPORT.md) 和 [runtime-312.json](semantica/evidence/2026-09-20/runtime-312.json)；它们是有限 probe，不构成 V01/V02/V03 或生产验收。

## Global Constraints

- 基线 SHA：`f7802c47c82835514e4a5d30457222b6c0e891d9`；本文件只校准计划，不以已留证的有限 probe 替代 V01–V03 或生产验收，实验结果须按证据层进入任务证据产物。
- [实施台账](semantica/progress.md) 中 V01–O03 共 24 项均为 `pending`；本轮 probe 不会使 V01 或 V02 变为 `verified`。
- 官方文档、公开包元数据和静态源码属于文档/静态证据，不能替代 actual runtime、controlled provider、live model 或 real storage 证据。
- 旧计划中未验证的协议、持久化、拓扑、版本、阈值和签名均为候选；不能因为计划存在而当作已实现或已验收。
- 本文件不授权自动迁移、默认启用、真实 KB 切换、生产模型调用或外部部署。

## Review Focus

- 存储隔离：复用基础设施时每种持久层必须有专用隔离边界；shared-isolated 与 dedicated 是能力比较候选，后续只对选定生产拓扑做完整验收。
- 规则推导：仅显式注册的 `depends_on` 传递规则可生成 `indirectly_depends_on`，任意关系不得自动传递。
- 试点启用：无 KB 管理权限、未展示存量规模和预算、未验收的 KB 均不得 promotion。
- 查询模式：普通 GraphRAG 的 requested/actual mode 不能被隐式升级为 Reason；Agent Reason 需要授权的工具、scope 和预算。
- 证据分层：mock 或静态资料必须保持较低层级，不能填入 verified 或上线验收。

---

## Q1–Q17 到既有任务的覆盖

| 已确认范围 | 既有任务 | 校准后的验收重点 |
|---|---|---|
| Q1–Q3 独立服务、解析/语义边界、Go 业务权威 | C01–C03、I02、A01 | 专用 gRPC DTO；DocReader→Go 标准分块→Semantica；tenant/KB/文档/权限由 Go 决定 |
| Q4 模型统一授权入口和预算任务 | A03、Q03、O01 | 消费者自有 gateway 先适配模型解析与商业原语；无 Python 长期密钥 |
| Q5 单 KB 跨文档 | I03、Q01、V03 | scope 只在单 KB 内；中文样本验证跨文档证据 |
| Q6 每 KB 后端切换 | W03、I03、O03 | native 保留；Semantica 验收后 promotion；每 KB 正式查询只有一个后端 |
| Q7 注册规则和模型辅助推断 | Q02、Q03、W02 | 开发者注册版本化规则；规则编辑器不在首期 |
| Q8 回填、异步更新与状态分离 | I05、W01、W02 | 启用后分页回填可恢复；parse/semantic 状态分别展示 |
| Q9 普通检索降级与推理失败 | Q04、W01、W02 | actual mode 可见；Reason 不可用不伪装成功 |
| Q10 旧 generation 可读 | I03、I05、Q04 | 新建期间返回 version/stale；仅成功发布后切换 |
| Q11 删除与撤权 | I04、A01、A02、Q04、O02 | deny 立即生效；交付前重校验；有效的独立来源可继续支持事实 |
| Q12 基础设施隔离 | V02、I03、O01 | 比较 shared-isolated 与 dedicated 的隔离能力；具体生产拓扑由能力证据选择 |
| Q13 双入口和现有页面 | W01、W02 | 问答与 Agent 共用 Go 授权/适配；页面显示状态、推理结果和证据 |
| Q14 技术文档依赖规则 | C03、Q02、V03 | `depends_on` 两跳生成 `indirectly_depends_on`；冲突、缺前提、撤销、隐藏来源不误推导 |
| Q15 显式推理触发 | Q04、W01、W02 | 用户 requested mode 或已授权 Agent 工具调用才 Reason |
| Q16 默认关闭试点 | W01–W03、O03 | 管理员 preview 后 enable/backfill；验收前不 promotion；其他 KB native |
| Q17 两阶段验收 | V03、O03 | 可复现中文受控样本后，再以授权代表性业务样本测量门槛 |

## 八项校准

| 差异 | 修改位置 | 必须保留的断言 |
|---|---|---|
| 1. 优先级与 pending | 总计划、七个子计划、台账 | ADR/rebaseline 优先；24 项 pending；probe 不是 V01/V02 verified |
| 2. 存储 | V02、I03、O01 | 优先专用 account/db/schema/collection/prefix 的 shared-isolated；不能可靠隔离则 dedicated；比较后只对选定生产拓扑完整验收 |
| 3. 规则 fixture | 总计划、V03、Q02 | `A depends_on B` 与 `B depends_on C` 才生成 `A indirectly_depends_on C`；不泛化 |
| 4. 试点开关 | W01、W02、W03 | 管理员 preview（存量规模、预算）→ enable/backfill → 验收 → promotion；默认关闭 |
| 5. 回填和版本 | I03、I05、Q04 | 分页可恢复；parse/semantic 分离；active old generation 的 stale/version 返回到成功发布 |
| 6. 请求模式 | Q04、W01 | requested/actual mode；GraphRAG 不隐式 Reason；Reason 校验用户模式或 Agent tool/scope/budget |
| 7. 证据层 | V01–V03、O03、台账 | `static`、`actual-runtime`、`controlled-provider`、`live-model`、`real-storage` 分层；mock 不得 verified |
| 8. 模型接缝 | A03 | 先定义消费者自有 `SemanticModelGateway`；`ModelService.GetChatModel` 只解析租户模型；组合 `ExecutionGate.Begin/Finish`、预算和用量原语；Craft gateway 私有 |

## 后续能力门槛

- [ ] **V01 static 与 actual-runtime：** 冻结候选上游版本，并以实际安装的 import/signature 输出证明；当前公开文档称 `GraphReasoner.reason(graph, query)` 返回 plain string，但这只指导接口风险，不能作为运行结果。
- [ ] **V02 real-storage：** 比较 shared-isolated 和 dedicated 的持久化/隔离能力，证据记录账号与实际 database/schema/collection/prefix 或实例边界；选定生产拓扑后才进入该拓扑的完整恢复和上线验收。
- [ ] **V03 controlled-provider 与 live-model：** 中文受控样本先覆盖 Q14/Q17；模型能力单独保留 live-model 证据，未配置时保持 blocked/pending。
- [ ] **O03 promotion：** 仅在授权代表性业务样本与 native 的比较有可复现结果后，由产品确认质量、延迟、成本门槛；没有测量和确认不得 promotion。

## A03 真实接缝与 RED/GREEN

`internal/types/interfaces/model.go:14-47` 和 `internal/application/service/model.go:589-625` 的 `ModelService.GetChatModel(ctx, modelID)` 只负责租户模型解析，不包含预算。`internal/commercial/execution.go` 的 `ExecutionGate` 提供 `Begin(ctx, BudgetRequest) (Reservation, error)` 与 `Finish(ctx, reservationID string, fact UsageFact) error`；`budget.go` 和 `usage.go` 是可组合原语。`internal/container/craft_model_gateway.go` 组装的是 Craft 私有 handler，不能假定其 HTTP handler、reserve/finalize 名称或调用签名可复用。

后续 A03 先定义消费者自有、完整的候选接口：

```go
type SemanticModelGateway interface {
    Invoke(ctx context.Context, req SemanticModelRequest) (SemanticModelResult, error)
}

type SemanticModelMessage struct {
    Role string
    Content string
}

type SemanticModelParameters struct {
    Temperature *float64
    MaxOutputTokens int
}

type SemanticModelWireRequest struct {
    CapabilityToken string
    Messages []SemanticModelMessage
    Parameters SemanticModelParameters
}

type SemanticModelCapability struct {
    TenantID uint64
    RunID string
    CallID string
    ModelID string
    Upper commercial.Credits
    Deadline time.Time
}

type SemanticModelRequest struct {
    Capability SemanticModelCapability
    Messages []SemanticModelMessage
    Parameters SemanticModelParameters
}

type SemanticModelResult struct {
    Text string
    InputTokens int64
    OutputTokens int64
    ProviderRequestID string
    ModelVersion string
}

func buildTrustedSemanticModelRequest(ctx context.Context, wire SemanticModelWireRequest) (SemanticModelRequest, error)
func trustedUsageFact(req SemanticModelRequest, result SemanticModelResult) commercial.UsageFact
```

`SemanticModelWireRequest` 是 Python 可发送的唯一执行 wire 类型；tenant、model、run、call、预算上限和 deadline 只存在于 Go 用认证短期 capability、持久模型配置和价格估算派生的 `SemanticModelCapability`，wire 中的同名字段或篡改 token 必须拒绝。KB 管理端可在授权、额度校验和持久化后配置预算设置，但该设置不构成模型 wire 的 `Upper`。RED 断言必须证明：校验失败或篡改 tenant/model/upper 时，`GetChatModel` 和 `Begin` 都不发生；`Begin` 拒绝时不调用 `GetChatModel`；成功调用以同一 reservation ID 恰好一次 `Finish`；未知上游结果保留可对账 usage fact；Python 只能得到短期请求能力而非长期密钥。GREEN 只在这些消费者自有测试通过后记录，不能据此声明上游或 live model 已验证。

## 自评

- Q1–Q17 已映射到现有任务，未新建第 25 项实施工作。
- 八项 review 修正均映射到任务和验收，不把静态资料升级为能力通过。
- 本文件只校准文档；实施和证据写入须以真实输出为准，产品范围与策略 Q1–Q17 已确认。
