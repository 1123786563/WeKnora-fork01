# T15：有证据的知识问答闭环（Issue #45）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 知识问答 Task 的答案带可打开的版本与时间证据，并在生产代码中明确区分「原文事实 / 规则推导 / 模型推断」三类结论；撤权、无证据与多知识源三个场景以真实集成路径验证，端到端行为通过最高稳定 Interface（Go 侧 knowledge-chat SSE wire + 移动端 Task Office 深模块 Interface + 真实部署 opt-in 集成冒烟）核实，不以 mock 冒充集成证据。

**Architecture:** Go 侧在既有 knowledge-chat SSE 流上新增一个 `evidence` 帧（`internal/types/evidence.go` 全新 wire 类型：`AnswerEvidence` 信封 + 三类 `EvidenceKind` + 逐引用 `revision`/`retrieved_at` + `ValidateAnswerEvidence` 交付不变量），由 QA 管线在答案流开始前交付：交付前对每条检索结果的 KB 读权限做实时重校验（ADR-0002「执行中请求在交付前重校验，受影响结果作废或重算」——org share 行删除/KB 删除即翻转），全部证据失权时按固定文案作废本回答；检索为空走既有 fallback 时显式发 `no_evidence` 态；显式请求推理（新请求字段 `reasoning_mode`）而部署未接入语义推理服务时按 ADR-0002 以 `reasoning.state='incomplete'` + 重试入口收尾，零结论零引用，不用普通检索冒充推理成功。客户端沿 #44 已建立的 knowledge-chat SSE 消费 seam：`@weknora/contracts` 新增 `parseAnswerEvidence` 严格解析器；`mobile-core` Task Office 新增 `askKnowledge` 入口（`KnowledgeQABackendPort` 注入、lease fail closed、迟到拒绝）；`api-client` 新增 `createMobileKnowledgeQARemote`（无 evidence 帧即 fail closed）；`apps/mobile` 新增 `/ask` 路由的 KnowledgeQAScreen（证据面板逐引用展示 版本·时间·类别徽标），并保留 opt-in 真实部署集成冒烟。语义推理（Semantica gRPC）的执行接入不在本计划（ADR-0002 试点门控、默认关闭），其三类结论分类函数按真实 proto wire 形态实现并被测试锁定，供该 rollout 消费。

**Tech Stack:** Go 1.26（gin + gorm，sqlite 内存真表测试，`migrations/` 不动——本计划无新迁移）、TypeScript（`packages/contracts`、`packages/mobile-core`、`packages/api-client`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器）、`testify`（Go）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置：已执行过 `pnpm install`（本计划作者已实跑基线：`go test ./internal/application/service/ -run 'TestEmitKnowledgeReferencesEventIgnoresCitationOutputSetting' -count=1` ok、`pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 全绿 pass 8 fail 0）。

**Spec:**
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-45.md`（验收标准原文见「Global Constraints」末尾）
- 批准 Spec：`docs/specs/2026-09-20-mobile-ai-office-design.md`（User Stories 24–27；Implementation Decisions 中「Lead Agent 检索只检索成员获准且 Tenant 允许发现的知识」「Task Office owns …」；Testing Decisions 中「Tests target observable behavior at the highest stable Interface」「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters」「End-to-end tests cover the five vertical workflows rather than isolated frontend/backend layers」）
- 批准 Spec：`docs/specs/2026-09-20-mobile-module-seams.md`（§5 Task Office——「一个初始目标创建 Task，追问继续原 Task」与 §5.2「Screen 不调用多个 wire 方法」/§5.3「Scope Lease 失效后丢弃迟到结果」「capability 缺失和未知 schema 一律 fail closed」；§10 App Shell 禁止 Screen 直连 contracts/api-client）
- ADR：`docs/adr/0002-semantica-independent-service.md`（三类区分、普通问答显式注明未使用语义图谱、明确请求推理不可用时「显示推理未完成和重试入口，不能用普通检索结果冒充推理成功」、「删除或撤权立即阻止失效证据：新请求禁止使用；执行中请求在交付前重校验，受影响结果作废或重算」、「本轮设计确认不等于实现或能力验收」）
- ADR：`docs/adr/0004-task-is-session.md`（taskId = sessionId，快速知识问题创建会话即创建 Task）
- ADR：`docs/adr/0009-cloud-data-trust-boundary.md`（Go 后端是知识与权限的唯一权威，撤权判定在服务端）
- 领域术语：`CONTEXT.md`（「证据引用（Evidence Citation）」「知识推理」「可发现知识范围」「任务（Task）」）
- Parent：Issue #30；Blocked by：#33/#35/#36——均已合入当前 HEAD（亲眼核实：`packages/mobile-core/src/shelf/`、`task-detail.ts`、`task-office.ts` 的 `start`/`legacyTasks`/`followUp` 均在）
- 前序批次产出（本计划 Consumes，全部在当前 HEAD 亲眼核实）：#36 的 `TaskBackendPort.createSession(input: { title: string }): Promise<{ sessionId: string }>`（`packages/mobile-core/src/task-office/task-office.ts:160`）、`TaskOfficeGoal.knowledgeIds` 与 `goalKeyOf`（task-office.ts:137-144）；#44 的 knowledge-chat SSE 消费 seam——`createServerSentEventParser`/`parseChatEvent`/`responseType`（`packages/api-client/src/chat/stream.ts:14,94`、`packages/contracts/src/chat/events.ts:23`）、`createMobileLegacyTaskRemote` 的 fail-closed 终止帧判定（`packages/api-client/src/mobile/legacy-tasks.ts:106-121`）；#32 的 `MobileRuntime.authorizedRequest`/`authorizedEventStream`（composition.ts:93-101 装配）；#33 的 Resource Shelf `browse().knowledge` 投影（`apps/mobile/src/app/new.tsx:26-30` 消费示例）；#35 的 `streamAuthorizedSse` body 透传；既有 Go QA wire：`POST /api/v1/knowledge-chat/:session_id`（`internal/router/routes_chat.go:242`）、`CreateKnowledgeQARequest`（`internal/handler/session/types.go:47`）、`emitKnowledgeReferencesEvent`（`internal/application/service/session_knowledge_qa.go:1200`）、`buildStreamResponse`（`internal/handler/session/helpers.go:190`）、`AgentStreamHandler.Subscribe`（`internal/handler/session/agent_stream_handler.go:132`）。

## Global Constraints

以下为批准 Spec / ADR / Issue 的项目级约束，逐字引用，所有任务隐含遵守：

- 「事实、规则推导和模型推断明确区分。」（Issue #45 验收标准 1）
- 「撤权、无证据和多知识源场景均有端到端验证。」（Issue #45 验收标准 2）
- 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」（Issue #45 验收标准 3）
- 「**证据引用（Evidence Citation）**：把 Agent 结论追溯到知识版本、外部资源版本或操作回执的可打开记录，包含来源和获取时间，并标明结论属于原文事实、规则推导还是模型推断。_避免_：仅显示资源名称、在用户请求后临时生成的出处、把模型推断标成来源事实。」（CONTEXT.md · 证据引用）
- 「**知识推理**：基于获准使用的知识证据，通过明确规则或模型推断得到结论的能力；结论需区分规则推导与模型推断。_避免_：把模型生成的解释等同于规则证明或原文事实。」（CONTEXT.md · 知识推理）
- 「**可发现知识范围**：成员有权访问且空间策略允许 Lead Agent 为当前任务自动检索的知识范围；Agent 必须保留并展示实际使用的来源。」（CONTEXT.md · 可发现知识范围）
- 「普通知识问答可退回现有检索，并显式注明未使用语义图谱；明确请求推理但服务不可用、索引未就绪或推理执行失败时，显示推理未完成和重试入口，不能用普通检索结果冒充推理成功。」（ADR-0002）
- 「删除或撤权立即阻止失效证据：新请求禁止使用；执行中请求在交付前重校验，受影响结果作废或重算。物理清理可以异步，其他有效来源仍可支持同一事实。」（ADR-0002）
- 「最终能力范围是构图、GraphRAG 和知识推理，回答必须区分原文事实和推理结论。」（ADR-0002）
- 「2026-09-20 整体 Q1–Q17 已确认；本轮设计确认不等于实现或能力验收。」（ADR-0002——语义推理执行接入按此门控，不在本计划内）
- 「Task is the product name for the existing Session identity. A Task contains multiple Runs; initial goals create Tasks and same-goal follow-ups stay within them.」（mobile-ai-office-design.md · Implementation Decisions）
- 「Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.」（同上）
- 「Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.」（同上）
- 「Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.」（同上 · Testing Decisions）
- 「Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.」（同上）
- 「End-to-end tests cover the five vertical workflows rather than isolated frontend/backend layers.」（同上）
- 「Screen 不调用 start、lookup、snapshot、events、interaction、command 等多个 wire 方法。Module 内部决定顺序、幂等、重连、revision 和错误呈现。」（mobile-module-seams.md §5.2）
- 「禁止：Screen 直接导入 packages/contracts 或 packages/api-client；Screen 自己维护 request_id、cursor、revision、scope generation；每个 Screen 建独立 query cache 或 token refresh」（mobile-module-seams.md §10）
- 「capability 缺失和未知 schema 一律 fail closed；Scope Lease 失效后丢弃迟到结果」（mobile-module-seams.md §5.3）
- 安全约束（会话注入）：服务端 SQL 一律参数绑定（本计划零新 SQL——重校验走既有 `KBPermissions.Check` 与 gorm 仓库）；服务端请求仅 http/https 且真实集成沿用 `disallowedDeploymentHost` 同源防线（Task 8 沿用既有 HTTPS origin 校验）；凭据只从环境变量读取，源码与测试不写入可用凭据字面量；真实 HTTP 集成证据沿用 `WEKNORA_MOBILE_TEST_*` opt-in 环境变量（无回退凭据）。
- 工作流约束：严格 RED→GREEN→REFACTOR（每个任务先写失败测试、实跑确认失败、最小实现、通过、提交）；实现不与已批准 Spec 冲突，冲突时升级而非静默重设计。

**Issue #45 验收标准原文（docs/plans/issue30-sweep/issues/issue-45.md）：**

1. 「事实、规则推导和模型推断明确区分。」
2. 「撤权、无证据和多知识源场景均有端到端验证。」
3. 「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」

验收标准 2/3 的本地可验证性说明（blocked-env 声明）：知识问答的完整生产检索链（Elasticsearch/Milvus + embedding 模型 + LLM 提供方）是 remote-owned 依赖，本地单测环境无此基础设施——按 Spec Testing Decisions「Remote-owned dependencies use in-memory scenario Adapters for Module tests」，本地以**真实 sqlite 行 + 真实 KBShareService/仓库 + 真实事件总线 + 真实 SSE 序列化路径**驱动，仅检索排序以确定性夹具代入（Task 3/2）；跨进程全链路（真实部署 + 真实知识库 + 真实模型）沿用 opt-in 真实 HTTP 模式（Task 8，需 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`），本地无此环境时以 `t.skip` 跳过（**不得伪造通过**）。语义推理执行（Semantica gRPC 服务启用且可达）本质上不可本地验证：ADR-0002 明示试点门控且默认关闭，本计划交付其诚实不可用语义（Task 3）与真实 proto wire 形态的分类锁定（Task 1），启用态执行接入列为后续 rollout。

**与调查结论的差异记录（以代码现状为准）：**

1. 调查缺口第 5 条称「Task Grant/可发现知识范围在代码中缺位（rg 'TaskGrant' 无生产命中）」。该缺口在调查时点为真，但已被第三批 #42 解决：当前 HEAD 存在 `internal/types/task_grant.go`、`repository.NewTaskGrantStore`、`service.NewTaskGrantService`（`ResolveTaskAccess`）及 `POST/GET/DELETE /workbench/tasks/:task_id/grants`（亲眼核实）。本计划不重建 Task Grant；「只检索成员获准的知识」由 QA 链路既有的请求时授权（`buildSearchTargets` → `KBPermissions.Check`，session_knowledge_qa.go:441-491）+ 本计划新增的**交付前重校验**共同承担。
2. 调查缺口第 3 条称「现有 QA 引用模型不携带版本与时间证据」。核实：`types.SearchResult.ContentRevision` 存在但 `json:"-"` 不序列化（`internal/types/search.go:220`），且无检索时间字段——本计划不改 `SearchResult`（冻结的存储/引用形状，DB Scan 兼容），而是新增独立的 `AnswerEvidence` 信封（新文件），引用行携带 `revision`（取自 ContentRevision）与 `retrieved_at`。
3. 调查缺口第 1 条称「三类区分无生产实现，仅原型 app.js」。属实（全仓 rg 亲证仅 `docs/design/craft-hifi/prototype/app.js` 一处）。本计划 Task 1 在 `internal/types` 建立生产级三类区分（含真实 semantic wire 的分类函数），Task 3 接入 QA wire。
4. 调查缺口第 2 条（语义 Search/Reason 客户端无业务消费方）在当前 HEAD 仍属实（`internal/modules/knowledge/semantic/client.go:201-231` 的 Search/Reason 仅有接口与 wire 映射）。本计划的边界：**不**将 Semantica gRPC 执行接入 QA 链路（ADR-0002 试点门控 + 「本轮设计确认不等于实现或能力验收」）；交付其不可用时的诚实语义与分类函数。`ConclusionFromSemanticReason` 的启用态消费方属 Semantica rollout，本计划以测试锁定其行为。

## Review Focus

Spec 隐含但无任务测试覆盖、最可能咬到真实用户的五类输入/失效模式（每行后在所属任务落地测试）：

1. **无证据冒充 / 证据帧缺失**：服务端未发 evidence 帧（旧版本部署）或流被代理截断后只剩 answer 帧——客户端若照常渲染答案即把无证据回答冒充有据回答；无证据态（`no_evidence`）若仍携带引用即伪造证据。——Task 6 测试 `KNOWLEDGE_QA_MISSING_EVIDENCE`/`KNOWLEDGE_QA_TRUNCATED` fail closed + Task 3 测试 `no_evidence` 零引用零结论 + Task 1 `ValidateAnswerEvidence` 不变量。
2. **交付前撤权旁路**：org share 在检索后、答案交付前被撤销（或 KB 被删除），旧证据行仍进入引用与 prompt——必须实时重校验并丢弃，全部失权时作废回答而非降级为无证据继续作答。——Task 3 测试 `TestKnowledgeQAEvidenceRevocationBeforeDeliveryDropsSource`（真实 `RemoveShare` 行翻转）与 `TestKnowledgeQAEvidenceAllSourcesRevokedVoidsAnswer`（作废文案 + 不进入 completion）。
3. **推理冒充**：明确请求 `reasoning_mode` 但部署未接入语义推理，服务端仍以普通检索总结回答并当作推理结果交付；或规则推导结论不带规则 ID、模型推断结论不带模型版本（无法与原文事实区分）。——Task 3 测试 `TestKnowledgeQAReasoningRequestShortCircuitsIncomplete`（零结论零引用 + retryable）+ Task 1 测试 rules 无 RuleIDs / model 无 ModelVersion 一律拒绝。
4. **越权/不可归属知识源**：无权限 KB 的检索行、或 `KnowledgeBaseID` 为空（无法证明归属）的行进入证据——必须按撤权同形丢弃（fail closed），不得因「检索已经发生」而放行。——Task 3 测试异租户无 share 的 `kb-denied` 行被丢弃、空 `KnowledgeBaseID` 行被丢弃。
5. **畸形 wire**：evidence 帧 JSON 畸形、`revision` 为负、`retrieved_at` 非 RFC3339、`kind`/`state` 白名单外取值、结论引用不存在的 citation——解析器必须整体拒绝而非部分渲染。——Task 4 `parseAnswerEvidence` 拒绝矩阵测试 + Task 1 `ValidateAnswerEvidence` 对应拒绝 + Task 2 SSE 序列化键集测试。

## 任务结构与文件地图

| # | 任务 | 主要交付 |
|---|---|---|
| 1 | Go：证据 wire 类型与三类结论分类 | `internal/types/evidence.go`（`EvidenceKind`/`AnswerEvidence`/`CitationsFromSearchResults`/`ConclusionFromNativeAnswer`/`ConclusionFromSemanticReason`/`ValidateAnswerEvidence`） |
| 2 | Go：evidence SSE 帧 | `event.EventAgentEvidence` + `ResponseTypeEvidence` + `AgentStreamHandler.handleEvidence`；真实 EventBus→StreamManager→`buildStreamResponse` 键集测试 |
| 3 | Go：QA 证据交付与撤权重校验 | `deliverKnowledgeEvidence`/`applyEvidenceRevocation`/`emitKnowledgeEvidenceEvent`/`emitReasoningIncomplete` + `reasoning_mode` 请求字段；三场景真实 sqlite 集成测试（AC2 Go 面） |
| 4 | contracts：证据信封解析器 | `packages/contracts/src/mobile/knowledge-evidence.ts` 的 `parseAnswerEvidence` + 拒绝矩阵测试 |
| 5 | mobile-core：知识问答域与 Task Office 入口 | `knowledge-qa.ts`（DTO/`KnowledgeQABackendPort`/scenario Adapter/类别文案）+ `TaskOffice.askKnowledge` + 错误码 |
| 6 | api-client：Knowledge QA Remote | `createMobileKnowledgeQARemore`（knowledge-chat SSE + evidence/answer/终止帧解析 + fail closed）+ `./mobile/knowledge-qa` 导出 |
| 7 | apps/mobile：知识问答界面 | `knowledge-qa-view.ts` 控制器、`KnowledgeQAScreen`、`/ask` 路由、composition `knowledgeQA` 装配、Home 入口、app-smoke 源级断言 |
| 8 | apps/mobile：真实 HTTP 集成证据 | `knowledge-qa-integration-smoke.ts`（opt-in `WEKNORA_MOBILE_TEST_*`：真实部署提问→evidence 帧存在→引用携带版本与时间→探针任务归档清理） |

执行门控：无——前置 #33/#35/#36 已全部合入当前 HEAD。Task 2 依赖 Task 1 的类型；Task 3 依赖 Task 1/2；Task 5 依赖 Task 4 的 wire 形状（类型逐字一致）；Task 6 依赖 Task 4/5；Task 7 依赖 Task 5/6；Task 8 依赖 Task 7；其余按序执行。

并行合并注意（本计划与同批次其余计划独立 worktree 后合并）：共享文件改动收敛为——`internal/event/event.go`（EventType 常量块追加一行）、`internal/event/event_data.go`（文件末追加一个 struct）、`internal/types/chat.go`（ResponseType 常量块追加一行）、`internal/handler/session/agent_stream_handler.go`（Subscribe 追加一行 + 文件内新增 `handleEvidence` 方法，不动既有方法）、`internal/application/service/session_knowledge_qa.go`（`CHAT_COMPLETION_STREAM` 发射块与 `ErrSearchNothing` 分支两处局部修改 + `KnowledgeQA` 顶部一个短路口 + 文件内新增三个方法与两个常量）、`internal/types/qa_request.go`（追加一个字段）、`internal/handler/session/types.go`（追加一个字段）、`internal/handler/session/qa.go`（`qaRequestContext` 追加一个字段、`buildQARequest` 追加一行映射、`parseQARequest` 追加一段校验）、`packages/mobile-core/src/task-office/task-office.ts`（`ports.knowledgeQA` 可选字段 + `askKnowledge` 方法，标记「T15 追加区」）、`packages/mobile-core/src/task-office/task-office-errors.ts`（错误码联合追加一员）、`packages/mobile-core/src/index.ts`（追加导出块）、`packages/contracts/src/index.ts`（追加一行导出）、`packages/api-client/package.json`（exports 追加一条）、`apps/mobile/src/composition.ts`（`taskOfficeFor` 内追加 `knowledgeQA` 装配 + import 一行）、`apps/mobile/src/screens/HomeScreen.tsx`（一个按钮）、`apps/mobile/src/app-smoke.test.tsx`（一个源级断言测试）。新增文件全部为本计划独有。

---

### Task 1: Go——证据 wire 类型与三类结论分类

**Files:**
- Create: `internal/types/evidence.go`
- Test: `internal/types/evidence_test.go`

**Interfaces:**
- Consumes: 既有 `SearchResult`（`internal/types/search.go:151`，`ContentRevision`/`ContentRewritten` 内部字段）、`SemanticReasoningMode`/`SemanticReasonStatus`/`SemanticReasonResponse` 与 `SemanticReasonResponseFromWire`（`internal/types/semantic.go:394-461,648`）、`semanticpb`（`github.com/Tencent/WeKnora/semantic/proto`）。
- Produces: `EvidenceKind`（`fact`/`rule_derived`/`model_inferred`）、`AnswerEvidenceState`（`cited`/`no_evidence`/`revoked`）、`EvidenceReasoningState`（`not_requested`/`incomplete`）、`EvidenceCitation`/`EvidenceConclusion`/`EvidenceReasoning`/`AnswerEvidence`（JSON 键集见文件）、`CitationsFromSearchResults(results []*SearchResult, retrievedAt time.Time) []EvidenceCitation`、`ConclusionFromNativeAnswer(modelID string, citationIDs []string) EvidenceConclusion`、`ConclusionFromSemanticReason(mode SemanticReasoningMode, reason SemanticReasonResponse) (EvidenceConclusion, error)`、`ValidateAnswerEvidence(e AnswerEvidence) error`。Task 2/3 消费。

- [ ] **Step 1: 写失败测试**

`internal/types/evidence_test.go`（新文件，完整内容）：

```go
package types

import (
	"strings"
	"testing"
	"time"

	semanticpb "github.com/Tencent/WeKnora/semantic/proto"
	"github.com/stretchr/testify/require"
)

func wireReasonRules(t *testing.T, ruleIDs []string) SemanticReasonResponse {
	t.Helper()
	conclusion := "A 间接依赖 C"
	decoded, err := SemanticReasonResponseFromWire(&semanticpb.ReasonResponse{
		Status:     semanticpb.ReasonStatus_REASON_STATUS_DERIVED,
		Conclusion: &conclusion,
		RuleIds:    ruleIDs,
	})
	require.NoError(t, err)
	return decoded
}

func TestConclusionFromSemanticReasonRulesModeRequiresRuleIDs(t *testing.T) {
	got, err := ConclusionFromSemanticReason(SemanticReasoningModeRules, wireReasonRules(t, []string{"dep-transitive"}))
	require.NoError(t, err)
	require.Equal(t, EvidenceKindRuleDerived, got.Kind)
	require.Equal(t, []string{"dep-transitive"}, got.RuleIDs)
	require.Empty(t, got.ModelID, "规则推导结论不得携带模型版本（三类不可混标）")

	_, err = ConclusionFromSemanticReason(SemanticReasoningModeRules, wireReasonRules(t, nil))
	require.Error(t, err, "规则推导结论缺规则 ID 即无法与原文事实区分，必须拒绝")

	_, err = ConclusionFromSemanticReason(SemanticReasoningModeRules, wireReasonRules(t, []string{"dep-transitive"}))
	require.NoError(t, err)
	insufficient := wireReasonRules(t, []string{"dep-transitive"})
	insufficient.Status = SemanticReasonStatusInsufficientEvidence
	_, err = ConclusionFromSemanticReason(SemanticReasoningModeRules, insufficient)
	require.Error(t, err, "证据不足态没有结论可标类")
}

func TestConclusionFromSemanticReasonModelModeRequiresModelVersion(t *testing.T) {
	conclusion := "服务 A 的故障可能影响 B"
	modelVersion := "qwen3-32b"
	decoded, err := SemanticReasonResponseFromWire(&semanticpb.ReasonResponse{
		Status:       semanticpb.ReasonStatus_REASON_STATUS_DERIVED,
		Conclusion:   &conclusion,
		ModelVersion: &modelVersion,
	})
	require.NoError(t, err)

	got, err := ConclusionFromSemanticReason(SemanticReasoningModeModel, decoded)
	require.NoError(t, err)
	require.Equal(t, EvidenceKindModelInferred, got.Kind)
	require.Equal(t, "qwen3-32b", got.ModelID)
	require.Empty(t, got.RuleIDs, "模型推断结论不得携带规则 ID（三类不可混标）")

	_, err = ConclusionFromSemanticReason(SemanticReasoningModeModel, wireReasonRules(t, nil))
	require.Error(t, err, "模型推断结论缺模型版本即无法审计，必须拒绝")
}

func TestConclusionFromNativeAnswerIsModelInferred(t *testing.T) {
	got := ConclusionFromNativeAnswer("chat-model-1", []string{"c1", "c2"})
	require.Equal(t, EvidenceKindModelInferred, got.Kind)
	require.Equal(t, "chat-model-1", got.ModelID)
	require.Equal(t, []string{"c1", "c2"}, got.CitationIDs)
	require.Empty(t, got.RuleIDs)
}

func TestCitationsFromSearchResultsDeduplicatesCapsQuoteAndStampsTime(t *testing.T) {
	retrievedAt := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	results := []*SearchResult{
		{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own", KnowledgeTitle: "手册", ContentRevision: 3, StartAt: 10, EndAt: 40, Content: "第一段命中内容"},
		{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own", KnowledgeTitle: "手册", ContentRevision: 3, StartAt: 10, EndAt: 40, Content: "第一段命中内容"},
		{ID: "chunk-2", KnowledgeID: "doc-2", KnowledgeBaseID: "kb-shared", KnowledgeTitle: "指南", ContentRevision: 5, Content: strings.Repeat("长", 500)},
		nil,
	}
	citations := CitationsFromSearchResults(results, retrievedAt)
	require.Len(t, citations, 2, "重复 chunk 与 nil 行不得产生重复/空引用")
	first := citations[0]
	require.Equal(t, "chunk-1", first.CitationID)
	require.Equal(t, "doc-1", first.KnowledgeID)
	require.Equal(t, "kb-own", first.KnowledgeBaseID)
	require.Equal(t, 3, first.Revision)
	require.Equal(t, EvidenceKindFact, first.Kind, "检索命中行本身是原文事实")
	require.Equal(t, retrievedAt.Format(time.RFC3339), first.RetrievedAt)
	second := citations[1]
	require.Equal(t, 5, second.Revision)
	require.LessOrEqual(t, len([]rune(second.Quote)), evidenceQuoteMaxRunes, "引文超长必须截断")
	require.Equal(t, 0, second.StartAt+second.EndAt, "无原文坐标的行不得伪造位置")
}

func validEvidence() AnswerEvidence {
	return AnswerEvidence{
		State:             EvidenceStateCited,
		SemanticGraphUsed: false,
		RetrievedAt:       "2026-09-24T08:00:00Z",
		Citations: []EvidenceCitation{{
			CitationID: "c1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own",
			Revision: 3, Kind: EvidenceKindFact, RetrievedAt: "2026-09-24T08:00:00Z",
		}, {
			CitationID: "c2", KnowledgeID: "doc-2", KnowledgeBaseID: "kb-shared",
			Revision: 5, Kind: EvidenceKindFact, RetrievedAt: "2026-09-24T08:00:00Z",
		}},
		Conclusions: []EvidenceConclusion{{Kind: EvidenceKindModelInferred, ModelID: "chat-model-1", CitationIDs: []string{"c1", "c2"}}},
		Reasoning:   EvidenceReasoning{State: EvidenceReasoningNotRequested},
	}
}

func TestValidateAnswerEvidenceAcceptsMultiKnowledgeSourceEnvelope(t *testing.T) {
	require.NoError(t, ValidateAnswerEvidence(validEvidence()))
}

func TestValidateAnswerEvidenceRejectsMasqueradeAndInconsistency(t *testing.T) {
	noEvidence := validEvidence()
	noEvidence.State = EvidenceStateNoEvidence
	require.Error(t, ValidateAnswerEvidence(noEvidence), "无证据态携带引用即伪造证据")

	revoked := validEvidence()
	revoked.State = EvidenceStateRevoked
	require.Error(t, ValidateAnswerEvidence(revoked), "撤权作废态携带引用即伪造证据")

	cited := validEvidence()
	cited.State = EvidenceStateCited
	cited.Conclusions = nil
	require.Error(t, ValidateAnswerEvidence(cited), "cited 态必须至少一条结论")

	unknownKind := validEvidence()
	unknownKind.Conclusions[0].Kind = EvidenceKind("oracle")
	require.Error(t, ValidateAnswerEvidence(unknownKind), "白名单外类别必须拒绝")

	citationKind := validEvidence()
	citationKind.Citations[0].Kind = EvidenceKindModelInferred
	require.Error(t, ValidateAnswerEvidence(citationKind), "引用行只能是原文事实")

	dangling := validEvidence()
	dangling.Conclusions[0].CitationIDs = []string{"c1", "missing"}
	require.Error(t, ValidateAnswerEvidence(dangling), "结论不得引用不存在的 citation")

	negativeRevision := validEvidence()
	negativeRevision.Citations[0].Revision = -1
	require.Error(t, ValidateAnswerEvidence(negativeRevision))

	badTime := validEvidence()
	badTime.RetrievedAt = "2026-09-24 08:00:00"
	require.Error(t, ValidateAnswerEvidence(badTime), "检索时间必须是 RFC3339")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/types/ -run 'TestConclusionFrom|TestCitationsFromSearchResults|TestValidateAnswerEvidence' -count=1`
Expected: FAIL（`undefined: ConclusionFromSemanticReason` / `EvidenceKindRuleDerived` 等编译错误——先写实现骨架前测试不可编译即 RED）

- [ ] **Step 3: 写最小实现**

`internal/types/evidence.go`（新文件，完整内容）：

```go
package types

import (
	"fmt"
	"time"
)

// T15（Issue #45）：有证据的知识问答。三类结论区分与逐引用版本/时间证据的生产
// wire 类型。CONTEXT.md「证据引用」：可打开的记录，包含来源和获取时间，并标明
// 结论属于原文事实、规则推导还是模型推断；避免把模型推断标成来源事实。

// EvidenceKind 标注一条结论/引用的知识来源类别（三类互斥，不得混标）。
type EvidenceKind string

const (
	// EvidenceKindFact 原文事实：直接来自获准知识源的检索命中行。
	EvidenceKindFact EvidenceKind = "fact"
	// EvidenceKindRuleDerived 规则推导：开发者注册的版本化规则链推得的结论（ADR-0002）。
	EvidenceKindRuleDerived EvidenceKind = "rule_derived"
	// EvidenceKindModelInferred 模型推断：LLM 生成或语义推理 model 模式的结论。
	EvidenceKindModelInferred EvidenceKind = "model_inferred"
)

func (k EvidenceKind) IsValid() bool {
	return k == EvidenceKindFact || k == EvidenceKindRuleDerived || k == EvidenceKindModelInferred
}

// AnswerEvidenceState 一次问答交付的证据状态。
type AnswerEvidenceState string

const (
	// EvidenceStateCited 有获准证据支撑（≥1 引用 + ≥1 结论）。
	EvidenceStateCited AnswerEvidenceState = "cited"
	// EvidenceStateNoEvidence 检索无命中：回答是 fallback，零引用零结论。
	EvidenceStateNoEvidence AnswerEvidenceState = "no_evidence"
	// EvidenceStateRevoked 交付前证据全部失权：本回答作废（ADR-0002）。
	EvidenceStateRevoked AnswerEvidenceState = "revoked"
)

func (s AnswerEvidenceState) IsValid() bool {
	return s == EvidenceStateCited || s == EvidenceStateNoEvidence || s == EvidenceStateRevoked
}

// EvidenceReasoningState 显式推理请求的执行状态。
type EvidenceReasoningState string

const (
	EvidenceReasoningNotRequested EvidenceReasoningState = "not_requested"
	EvidenceReasoningIncomplete   EvidenceReasoningState = "incomplete"
)

// EvidenceCitation 一条可打开的原文事实引用：知识文档、其所在知识库、
// chunk 编辑版本（revision）、检索时间与可选的原文坐标/引文。
type EvidenceCitation struct {
	CitationID      string       `json:"citation_id"`
	KnowledgeID     string       `json:"knowledge_id"`
	KnowledgeBaseID string       `json:"knowledge_base_id"`
	Title           string       `json:"title,omitempty"`
	Revision        int          `json:"revision"`
	StartAt         int          `json:"start_at,omitempty"`
	EndAt           int          `json:"end_at,omitempty"`
	Quote           string       `json:"quote,omitempty"`
	Kind            EvidenceKind `json:"kind"`
	RetrievedAt     string       `json:"retrieved_at"`
}

// EvidenceConclusion 一条结论的类别与审计锚点：规则推导必须带规则 ID，
// 模型推断必须带模型标识——否则与原文事实不可区分（AC1）。
type EvidenceConclusion struct {
	Kind       EvidenceKind `json:"kind"`
	ModelID    string       `json:"model_id,omitempty"`
	RuleIDs    []string     `json:"rule_ids,omitempty"`
	CitationIDs []string     `json:"citation_ids,omitempty"`
}

// EvidenceReasoning 显式推理请求的诚实状态：unavailable 时 state=incomplete
// 且 retryable=true，绝不用普通检索结果冒充推理成功（ADR-0002）。
type EvidenceReasoning struct {
	Requested bool                   `json:"requested"`
	Mode      string                 `json:"mode,omitempty"`
	State     EvidenceReasoningState `json:"state"`
	Reason    string                 `json:"reason,omitempty"`
	Retryable bool                   `json:"retryable"`
}

// AnswerEvidence 是 knowledge-chat SSE evidence 帧的载荷（信封）。
type AnswerEvidence struct {
	State             AnswerEvidenceState  `json:"state"`
	SemanticGraphUsed bool                 `json:"semantic_graph_used"`
	RetrievedAt       string               `json:"retrieved_at"`
	Citations         []EvidenceCitation   `json:"citations"`
	Conclusions       []EvidenceConclusion `json:"conclusions"`
	Reasoning         EvidenceReasoning    `json:"reasoning"`
}

// evidenceQuoteMaxRunes 限制单条引用的原文引文长度（信封不搬运整段正文）。
const evidenceQuoteMaxRunes = 200

// CitationsFromSearchResults 把检索命中行映射为原文事实引用：按 chunk ID 去重、
// 引文截断、统一盖检索时间戳；无原文坐标（或被 merge 重写）的行不伪造位置。
func CitationsFromSearchResults(results []*SearchResult, retrievedAt time.Time) []EvidenceCitation {
	stamp := retrievedAt.UTC().Format(time.RFC3339)
	seen := make(map[string]bool, len(results))
	citations := make([]EvidenceCitation, 0, len(results))
	for _, result := range results {
		if result == nil || result.ID == "" || result.KnowledgeID == "" || result.KnowledgeBaseID == "" {
			continue
		}
		if seen[result.ID] {
			continue
		}
		seen[result.ID] = true
		quote := []rune(result.Content)
		if len(quote) > evidenceQuoteMaxRunes {
			quote = quote[:evidenceQuoteMaxRunes]
		}
		citations = append(citations, EvidenceCitation{
			CitationID:      result.ID,
			KnowledgeID:     result.KnowledgeID,
			KnowledgeBaseID: result.KnowledgeBaseID,
			Title:           result.KnowledgeTitle,
			Revision:        result.ContentRevision,
			StartAt:         result.StartAt,
			EndAt:           result.EndAt,
			Quote:           string(quote),
			Kind:            EvidenceKindFact,
			RetrievedAt:     stamp,
		})
	}
	return citations
}

// ConclusionFromNativeAnswer 标注本地检索问答的答案结论：LLM 总结属模型推断。
func ConclusionFromNativeAnswer(modelID string, citationIDs []string) EvidenceConclusion {
	return EvidenceConclusion{Kind: EvidenceKindModelInferred, ModelID: modelID, CitationIDs: append([]string(nil), citationIDs...)}
}

// ConclusionFromSemanticReason 标注语义推理（Semantica）结论：rules 模式必须是
// 规则推导且携带非空规则 ID；model 模式必须是模型推断且携带模型版本。证据不足、
// 冲突或不可用态没有可标类的结论，一律报错（调用方按推理未完成处理）。
func ConclusionFromSemanticReason(mode SemanticReasoningMode, reason SemanticReasonResponse) (EvidenceConclusion, error) {
	if reason.Status != SemanticReasonStatusDerived || reason.Conclusion == nil || *reason.Conclusion == "" {
		return EvidenceConclusion{}, fmt.Errorf("semantic reason did not derive a conclusion (status=%s)", reason.Status)
	}
	switch mode {
	case SemanticReasoningModeRules:
		if len(reason.RuleIDs) == 0 {
			return EvidenceConclusion{}, fmt.Errorf("rule-derived conclusion requires non-empty rule ids")
		}
		return EvidenceConclusion{Kind: EvidenceKindRuleDerived, RuleIDs: append([]string(nil), reason.RuleIDs...)}, nil
	case SemanticReasoningModeModel:
		if reason.ModelVersion == nil || *reason.ModelVersion == "" {
			return EvidenceConclusion{}, fmt.Errorf("model-inferred conclusion requires a model version")
		}
		return EvidenceConclusion{Kind: EvidenceKindModelInferred, ModelID: *reason.ModelVersion}, nil
	default:
		return EvidenceConclusion{}, fmt.Errorf("unsupported reasoning mode %q", mode)
	}
}

// ValidateAnswerEvidence 是交付不变量：信封在发出（服务端）与渲染（客户端解析后）
// 两侧都必须自洽——cited 必须有引用与结论；no_evidence/revoked 必须零引用零结论；
// 引用行只能是原文事实；规则推导结论必须带规则 ID、模型推断结论必须带模型标识；
// 结论不得引用不存在的 citation；所有时间是 RFC3339。
func ValidateAnswerEvidence(e AnswerEvidence) error {
	if !e.State.IsValid() {
		return fmt.Errorf("evidence state %q is invalid", e.State)
	}
	if _, err := time.Parse(time.RFC3339, e.RetrievedAt); err != nil {
		return fmt.Errorf("evidence retrieved_at must be RFC3339")
	}
	if e.State == EvidenceStateCited && len(e.Citations) == 0 {
		return fmt.Errorf("cited evidence requires at least one citation")
	}
	if e.State != EvidenceStateCited && (len(e.Citations) > 0 || len(e.Conclusions) > 0) {
		return fmt.Errorf("state %q must not carry citations or conclusions", e.State)
	}
	if e.State == EvidenceStateCited && len(e.Conclusions) == 0 {
		return fmt.Errorf("cited evidence requires at least one conclusion")
	}
	citationIDs := make(map[string]bool, len(e.Citations))
	for i, citation := range e.Citations {
		if citation.CitationID == "" || citation.KnowledgeID == "" || citation.KnowledgeBaseID == "" {
			return fmt.Errorf("citations[%d] is not attributable to a knowledge source", i)
		}
		if citation.Kind != EvidenceKindFact {
			return fmt.Errorf("citations[%d].kind must be fact", i)
		}
		if citation.Revision < 0 {
			return fmt.Errorf("citations[%d].revision must not be negative", i)
		}
		if _, err := time.Parse(time.RFC3339, citation.RetrievedAt); err != nil {
			return fmt.Errorf("citations[%d].retrieved_at must be RFC3339", i)
		}
		if citationIDs[citation.CitationID] {
			return fmt.Errorf("citations[%d].citation_id is duplicated", i)
		}
		citationIDs[citation.CitationID] = true
	}
	for i, conclusion := range e.Conclusions {
		if !conclusion.Kind.IsValid() {
			return fmt.Errorf("conclusions[%d].kind %q is invalid", i, conclusion.Kind)
		}
		if conclusion.Kind == EvidenceKindRuleDerived && len(conclusion.RuleIDs) == 0 {
			return fmt.Errorf("conclusions[%d] rule-derived requires rule ids", i)
		}
		if conclusion.Kind == EvidenceKindModelInferred && conclusion.ModelID == "" {
			return fmt.Errorf("conclusions[%d] model-inferred requires a model id", i)
		}
		for _, id := range conclusion.CitationIDs {
			if !citationIDs[id] {
				return fmt.Errorf("conclusions[%d] references unknown citation %q", i, id)
			}
		}
	}
	if e.Reasoning.State == "" {
		return fmt.Errorf("reasoning state is required")
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/types/ -run 'TestConclusionFrom|TestCitationsFromSearchResults|TestValidateAnswerEvidence' -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/types/evidence.go internal/types/evidence_test.go
git commit -m "feat(evidence): answer evidence envelope with fact/rule-derived/model-inferred classification"
```

---

### Task 2: Go——evidence SSE 帧（事件类型、响应类型与流序列化）

**Files:**
- Modify: `internal/event/event.go`（EventType 常量块，`EventAgentReferences` 行后追加一行）
- Modify: `internal/event/event_data.go`（文件末追加 `AgentEvidenceData`）
- Modify: `internal/types/chat.go`（`ResponseType` 常量块，`ResponseTypeReferences` 附近追加一项）
- Modify: `internal/handler/session/agent_stream_handler.go`（`Subscribe` 追加一行订阅 + 新增 `handleEvidence` 方法，置于 `handleReferences` 之后）
- Test: `internal/handler/session/agent_evidence_stream_test.go`

**Interfaces:**
- Consumes: Task 1 的 `types.AnswerEvidence`；既有 `event.EventBus`（`internal/event/event.go:108`，`On`/`Emit` 同步分发）、`stream.NewMemoryStreamManager()`（`internal/stream/memory_manager.go:37`）、`NewAgentStreamHandler`（`agent_stream_handler.go:100`）、`buildStreamResponse`（`helpers.go:190`，包内可见）。
- Produces: `event.EventAgentEvidence EventType = "evidence"`、`event.AgentEvidenceData{ Evidence interface{} }`（`types.AnswerEvidence` 以接口承载，event 包不导入 types——与 `AgentReferencesData` 同例）、`types.ResponseTypeEvidence ResponseType = "evidence"`、`(*AgentStreamHandler).handleEvidence`。Task 3/6 消费（SSE 帧形状 `{"response_type":"evidence","data":{"evidence":{...}}}`）。

- [ ] **Step 1: 写失败测试**

`internal/handler/session/agent_evidence_stream_test.go`（新文件，完整内容）：

```go
package session

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/stream"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// AC3（Go 面）：evidence 事件经真实 EventBus → 真实 AgentStreamHandler 订阅 →
// 真实 MemoryStreamManager → 真实 buildStreamResponse 序列化为 SSE 帧，不经任何 mock
// 断言中间结构——消费者看到的就是 wire 字节形状。
func TestEvidenceEventStreamsAsEvidenceResponseFrame(t *testing.T) {
	bus := event.NewEventBus()
	streams := stream.NewMemoryStreamManager()
	assistant := &types.Message{ID: "am-1", SessionID: "sess-1"}
	handler := NewAgentStreamHandler(context.Background(), "sess-1", "am-1", "req-1", 1, time.Now(), assistant, streams, bus, nil, nil, nil)
	handler.Subscribe()

	evidence := types.AnswerEvidence{
		State:             types.EvidenceStateCited,
		SemanticGraphUsed: false,
		RetrievedAt:       "2026-09-24T08:00:00Z",
		Citations: []types.EvidenceCitation{{
			CitationID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-own",
			Revision: 3, Kind: types.EvidenceKindFact, RetrievedAt: "2026-09-24T08:00:00Z",
		}},
		Conclusions: []types.EvidenceConclusion{{
			Kind: types.EvidenceKindModelInferred, ModelID: "chat-model-1", CitationIDs: []string{"chunk-1"},
		}},
		Reasoning: types.EvidenceReasoning{State: types.EvidenceReasoningNotRequested},
	}
	require.NoError(t, types.ValidateAnswerEvidence(evidence))

	require.NoError(t, bus.Emit(context.Background(), event.Event{
		ID:        "ev-1",
		Type:      event.EventAgentEvidence,
		SessionID: "sess-1",
		Data:      event.AgentEvidenceData{Evidence: evidence},
	}))

	events, _, err := streams.GetEvents(context.Background(), "sess-1", "am-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 1, "evidence 事件必须恰好流化为一帧")
	require.Equal(t, types.ResponseTypeEvidence, events[0].Type)

	response := buildStreamResponse(events[0], "req-1")
	require.Equal(t, types.ResponseTypeEvidence, response.ResponseType)
	payload, err := json.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"response_type":"evidence"`)
	require.Contains(t, string(payload), `"state":"cited"`)
	require.Contains(t, string(payload), `"kind":"fact"`)
	require.Contains(t, string(payload), `"revision":3`)
	require.Contains(t, string(payload), `"kind":"model_inferred"`)
	require.Contains(t, string(payload), `"semantic_graph_used":false`)
	require.NotContains(t, string(payload), `"knowledge_references"`, "evidence 帧不得混入旧引用形状")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/handler/session/ -run 'TestEvidenceEventStreamsAsEvidenceResponseFrame' -count=1`
Expected: FAIL（编译错误：`event.EventAgentEvidence` / `event.AgentEvidenceData` / `types.ResponseTypeEvidence` 未定义）

- [ ] **Step 3: 写最小实现**

3a. `internal/event/event.go`：在 `EventAgentReferences    EventType = "references"` 行（event.go:57）后追加：

```go
	EventAgentEvidence     EventType = "evidence"      // 答案证据信封（T15：版本/时间证据 + 三类结论区分）
```

（对齐既有常量块的注释与对齐风格；若 gofmt 对齐要求不同，以 `gofmt -w internal/event/event.go` 结果为准。）

3b. `internal/event/event_data.go`：文件末追加：

```go
// AgentEvidenceData carries the answer evidence envelope (T15). Evidence is
// types.AnswerEvidence, kept as interface{} for the same reason
// AgentReferencesData does: the event package stays free of a types import.
type AgentEvidenceData struct {
	Evidence interface{} `json:"evidence"`
}
```

3c. `internal/types/chat.go`：在 `ResponseType` 常量块内（`ResponseTypeSteer` 之前的引用类区域，`ResponseTypeReferences` 对应的常量行附近）追加：

```go
	// ResponseTypeEvidence is the T15 evidence envelope frame: per-citation
	// version and retrieval-time provenance plus the fact / rule-derived /
	// model-inferred classification of conclusions. Emitted before the answer
	// streams so clients can bind citations to the answer they ground.
	ResponseTypeEvidence ResponseType = "evidence"
```

3d. `internal/handler/session/agent_stream_handler.go`：`Subscribe()` 中 `h.eventBus.On(event.EventAgentReferences, h.handleReferences)` 行后追加：

```go
	h.eventBus.On(event.EventAgentEvidence, h.handleEvidence)
```

并在 `handleReferences` 方法后新增（同一文件、只新增不改动既有方法）：

```go
// handleEvidence streams the answer evidence envelope (T15). The envelope is
// authoritative provenance: version + retrieval time per citation and the
// fact/rule-derived/model-inferred classification of conclusions. It is
// emitted before the answer streams; no client-visible mutation happens here.
func (h *AgentStreamHandler) handleEvidence(ctx context.Context, evt event.Event) error {
	data, ok := evt.Data.(event.AgentEvidenceData)
	if !ok {
		return nil
	}
	if err := h.streamManager.AppendEvent(h.ctx, h.sessionID, h.assistantMessageID, interfaces.StreamEvent{
		ID:        evt.ID,
		Type:      types.ResponseTypeEvidence,
		Content:   "",
		Done:      false,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"evidence": data.Evidence},
	}); err != nil {
		logger.GetLogger(h.ctx).Error("Append evidence event to stream failed", "error", err)
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/handler/session/ -run 'TestEvidenceEventStreamsAsEvidenceResponseFrame' -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/event/event.go internal/event/event_data.go internal/types/chat.go internal/handler/session/agent_stream_handler.go internal/handler/session/agent_evidence_stream_test.go
git commit -m "feat(qa): stream answer evidence envelope as an SSE frame"
```

---

### Task 3: Go——QA 证据交付：交付前撤权重校验、无证据态与推理不可用短路

**Files:**
- Modify: `internal/application/service/session_knowledge_qa.go`（`KnowledgeQA` 顶部短路口；`KnowledgeQAByEvent` 的 `CHAT_COMPLETION_STREAM` 发射块与 `ErrSearchNothing` 分支两处局部修改；文件内新增 `deliverKnowledgeEvidence`/`applyEvidenceRevocation`/`emitKnowledgeEvidenceEvent`/`emitReasoningIncomplete` 与两个常量）
- Modify: `internal/types/qa_request.go`（`QARequest` 追加 `ReasoningMode` 字段）
- Modify: `internal/handler/session/types.go`（`CreateKnowledgeQARequest` 追加 `reasoning_mode`）
- Modify: `internal/handler/session/qa.go`（`qaRequestContext` 追加 `reasoningMode`；`buildQARequest` 追加映射；`parseQARequest` 追加枚举校验）
- Test: `internal/application/service/knowledge_evidence_test.go`
- Test: `internal/handler/session/qa_reasoning_mode_test.go`

**Interfaces:**
- Consumes: Task 1 的 `CitationsFromSearchResults`/`ConclusionFromNativeAnswer`/`ValidateAnswerEvidence`/`AnswerEvidence` 等；Task 2 的 `event.EventAgentEvidence`/`event.AgentEvidenceData`；既有 `emitKnowledgeReferencesEvent`（session_knowledge_qa.go:1200）、`emitFallbackAnswer`（:1218）、`handleFallbackResponse`、`kbReadPermissions(ctx, access.KBShareLookup) *access.KBPermissions`（knowledgebase_access.go:14，`Check(kbID, ownerTenantID, OrgRoleViewer)` 语义：同租户 Viewer 放行、跨租户走 org share/grant 实时行）、`uniqueNonEmptyStrings`（session_knowledge_qa.go:464 同文件已有）、`generateEventID`、真实 `service.NewKBShareService(shareRepo, orgRepo, kbRepo, kgRepo, chunkRepo, audit)`（kbshare.go:48；`RemoveShare` 在 `share.SharedByUserID == caller` 时放行，audit 为 nil 安全——kb_activity.go:110）、真实 `&knowledgeBaseService{repo: repository.NewKnowledgeBaseRepository(db)}`（`GetKnowledgeBasesByIDsOnly` 只用 `s.repo`，knowledgebase.go:334-338）、`ChatManage{PipelineRequest{ChatModelID}, PipelineState{MergeResult}, PipelineContext{EventBus: bus.AsEventBusInterface()}}` 组装形态（session_knowledge_qa_test.go:43-54 先例）。
- Produces: `QARequest.ReasoningMode string`（`"rules"|"model"|""`）、`CreateKnowledgeQARequest.ReasoningMode string json:"reasoning_mode,omitempty"`、`(s *sessionService) deliverKnowledgeEvidence(ctx, chatManage) types.AnswerEvidenceState`、`(s *sessionService) applyEvidenceRevocation(ctx, chatManage) types.AnswerEvidenceState`（副作用：过滤 `chatManage.MergeResult`——被撤销/不可归属的行同时从 prompt 上下文移除）、`(s *sessionService) emitKnowledgeEvidenceEvent(ctx, chatManage, state, reasoning)`、`(s *sessionService) emitReasoningIncomplete(ctx, eventBus, req)`。Task 4/6 消费其 wire 行为；knowledge-chat SSE 的 evidence 帧语义由此定型。

- [ ] **Step 1: 写失败测试**

`internal/application/service/knowledge_evidence_test.go`（新文件，完整内容；三场景 + 推理短路的真实 sqlite 集成测试——真实 KBShareService/仓库/事件总线，仅检索排序以夹具代入）：

```go
package service

// T15（Issue #45）AC2 三个场景的服务端集成证据：多知识源、撤权、无证据。
// 全部跑在真实 sqlite 行 + 真实 KBShareService + 真实事件总线上；只有「检索
// 已返回什么」以夹具 MergeResult 代入（检索引擎是 remote-owned 依赖）。

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type evidenceCapture struct {
	evidence []types.AnswerEvidence
	fallback []string
	other    []string
}

func newKnowledgeEvidenceEnv(t *testing.T) (*sessionService, *gorm.DB, *event.EventBus, *evidenceCapture) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.KnowledgeBase{}, &types.Knowledge{}, &types.Chunk{},
		&types.Organization{}, &types.OrganizationTenantMember{}, &types.KnowledgeBaseShare{},
	))
	shares := NewKBShareService(
		repository.NewKBShareRepository(db),
		repository.NewOrganizationRepository(db),
		repository.NewKnowledgeBaseRepository(db),
		repository.NewKnowledgeRepository(db),
		repository.NewChunkRepository(db),
		nil,
	)
	svc := &sessionService{
		cfg:                  &config.Config{},
		knowledgeBaseService: &knowledgeBaseService{repo: repository.NewKnowledgeBaseRepository(db)},
		kbShareService:       shares,
	}
	bus := event.NewEventBus()
	capture := &evidenceCapture{}
	bus.On(event.EventAgentEvidence, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentEvidenceData)
		require.True(t, ok)
		capture.evidence = append(capture.evidence, data.Evidence.(types.AnswerEvidence))
		return nil
	})
	bus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		if data, ok := evt.Data.(event.AgentFinalAnswerData); ok && data.IsFallback {
			capture.fallback = append(capture.fallback, data.Content)
			return nil
		}
		capture.other = append(capture.other, "final_answer")
		return nil
	})
	bus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		capture.other = append(capture.other, "references")
		return nil
	})
	return svc, db, bus, capture
}

// caller 是租户 2 的 Viewer：kb-own 同租户；kb-shared 属租户 3、经 org-1 共享给租户 2；
// kb-denied 属租户 3、无共享。
func seedKnowledgeEvidenceRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, kb := range []*types.KnowledgeBase{
		{ID: "kb-own", TenantID: 2},
		{ID: "kb-shared", TenantID: 3},
		{ID: "kb-denied", TenantID: 3},
	} {
		require.NoError(t, db.Create(kb).Error)
	}
	for _, knowledge := range []*types.Knowledge{
		{ID: "doc-own", TenantID: 2, KnowledgeBaseID: "kb-own", Type: "file"},
		{ID: "doc-shared", TenantID: 3, KnowledgeBaseID: "kb-shared", Type: "file"},
	} {
		require.NoError(t, db.Create(knowledge).Error)
	}
	require.NoError(t, db.Create(&types.Chunk{ID: "chunk-own", TenantID: 2, KnowledgeBaseID: "kb-own", KnowledgeID: "doc-own", Content: "同租户手册命中段", ContentRevision: 3, StartAt: 10, EndAt: 40}).Error)
	require.NoError(t, db.Create(&types.Chunk{ID: "chunk-shared", TenantID: 3, KnowledgeBaseID: "kb-shared", KnowledgeID: "doc-shared", Content: "跨租户共享指南命中段", ContentRevision: 5, StartAt: 0, EndAt: 30}).Error)
	require.NoError(t, db.Create(&types.Organization{ID: "org-1", Name: "org", OwnerID: "user-3", OwnerTenantID: 3}).Error)
	require.NoError(t, db.Create(&types.OrganizationTenantMember{ID: "member-2", OrganizationID: "org-1", TenantID: 2, Role: types.OrgRoleViewer}).Error)
	require.NoError(t, db.Create(&types.KnowledgeBaseShare{ID: "share-1", KnowledgeBaseID: "kb-shared", OrganizationID: "org-1", SharedByUserID: "user-3", SourceTenantID: 3, Permission: types.OrgRoleViewer}).Error)
}

func knowledgeEvidenceCallerContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(2))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-2")
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
}

func evidenceChatManage(bus *event.EventBus, results []*types.SearchResult) *types.ChatManage {
	return &types.ChatManage{
		PipelineRequest: types.PipelineRequest{Query: "依赖关系是什么", SessionID: "sess-1", ChatModelID: "chat-model-1"},
		PipelineState:   types.PipelineState{MergeResult: results},
		PipelineContext: types.PipelineContext{EventBus: bus.AsEventBusInterface()},
	}
}

func searchRow(chunkID, knowledgeID, kbID, title string, revision int) *types.SearchResult {
	return &types.SearchResult{ID: chunkID, KnowledgeID: knowledgeID, KnowledgeBaseID: kbID, KnowledgeTitle: title, ContentRevision: revision, Content: "命中内容", StartAt: 10, EndAt: 40}
}

// 场景 3（多知识源）+ AC1（三类区分）：同租户与跨租户共享两个知识源的引用并存，
// 逐引用携带 revision 与 retrieved_at；结论是模型推断（带模型标识），引用是原文事实。
func TestKnowledgeQAEvidenceMultiKnowledgeSourceAndKinds(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	cm := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-own", "doc-own", "kb-own", "手册", 3),
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
	})

	state := svc.deliverKnowledgeEvidence(knowledgeEvidenceCallerContext(), cm)
	require.Equal(t, types.EvidenceStateCited, state)
	require.Len(t, capture.evidence, 1)
	envelope := capture.evidence[0]
	require.NoError(t, types.ValidateAnswerEvidence(envelope))
	kbIDs := []string{envelope.Citations[0].KnowledgeBaseID, envelope.Citations[1].KnowledgeBaseID}
	require.ElementsMatch(t, []string{"kb-own", "kb-shared"}, kbIDs, "两个知识源的引用都必须在场")
	revisions := map[string]int{}
	for _, citation := range envelope.Citations {
		revisions[citation.KnowledgeID] = citation.Revision
		require.Equal(t, types.EvidenceKindFact, citation.Kind)
		_, err := time.Parse(time.RFC3339, citation.RetrievedAt)
		require.NoError(t, err, "每条引用必须带可解析的检索时间")
	}
	require.Equal(t, 3, revisions["doc-own"])
	require.Equal(t, 5, revisions["doc-shared"])
	require.Len(t, envelope.Conclusions, 1)
	require.Equal(t, types.EvidenceKindModelInferred, envelope.Conclusions[0].Kind)
	require.Equal(t, "chat-model-1", envelope.Conclusions[0].ModelID)
	require.False(t, envelope.SemanticGraphUsed, "本地检索必须显式注明未使用语义图谱（ADR-0002）")
	require.Len(t, capture.other, 1, "引用帧恰好一帧")
	require.Equal(t, "references", capture.other[0])
	require.Empty(t, capture.fallback)
}

// 场景 1（撤权）：真实 RemoveShare 翻转权限后，跨租户源在交付前被丢弃；
// 剩余同租户源继续支撑 cited 态。
func TestKnowledgeQAEvidenceRevocationBeforeDeliveryDropsSource(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	ctx := knowledgeEvidenceCallerContext()

	before := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-own", "doc-own", "kb-own", "手册", 3),
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
	})
	require.Equal(t, types.EvidenceStateCited, svc.deliverKnowledgeEvidence(ctx, before))

	// 真实撤权：share-1 的原始分享者移除共享（真实 service 方法 + 真实行删除）。
	sharerCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(3))
	sharerCtx = context.WithValue(sharerCtx, types.UserIDContextKey, "user-3")
	require.NoError(t, svc.kbShareService.RemoveShare(sharerCtx, "share-1", "user-3", 3))

	capture.evidence = nil
	capture.other = nil
	after := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-own", "doc-own", "kb-own", "手册", 3),
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
		searchRow("chunk-denied", "doc-denied", "kb-denied", "无权文档", 1),
	})
	state := svc.deliverKnowledgeEvidence(ctx, after)
	require.Equal(t, types.EvidenceStateCited, state)
	require.Len(t, capture.evidence, 1)
	for _, citation := range capture.evidence[0].Citations {
		require.Equal(t, "kb-own", citation.KnowledgeBaseID, "已撤权与从未获准的源都不得出现在证据里")
	}
	require.Len(t, after.MergeResult, 1, "被撤销/无权的行必须同时从 prompt 上下文移除（不得继续喂养答案）")
	require.Equal(t, "chunk-own", after.MergeResult[0].ID)
}

// 场景 1（撤权·全失权）：唯一证据源被撤销 → 本回答作废（固定文案 fallback），
// 证据信封 state=revoked 且零引用零结论（不得降级为无证据继续作答）。
func TestKnowledgeQAEvidenceAllSourcesRevokedVoidsAnswer(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	ctx := knowledgeEvidenceCallerContext()

	sharerCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(3))
	sharerCtx = context.WithValue(sharerCtx, types.UserIDContextKey, "user-3")
	require.NoError(t, svc.kbShareService.RemoveShare(sharerCtx, "share-1", "user-3", 3))

	cm := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
	})
	state := svc.deliverKnowledgeEvidence(ctx, cm)
	require.Equal(t, types.EvidenceStateRevoked, state)
	require.Len(t, capture.evidence, 1)
	require.Equal(t, types.EvidenceStateRevoked, capture.evidence[0].State)
	require.Empty(t, capture.evidence[0].Citations)
	require.Empty(t, capture.evidence[0].Conclusions)
	require.NoError(t, types.ValidateAnswerEvidence(capture.evidence[0]))
	require.Len(t, capture.fallback, 1)
	require.Contains(t, capture.fallback[0], "作废", "作废回答必须以固定文案显式收尾（emitFallbackAnswer 可能按引用开关重写标记，断言语义而非整串）")
}

// 场景 1（撤权·不可归属）：KnowledgeBaseID 为空（无法证明归属）的检索行按撤权同形丢弃。
func TestKnowledgeQAEvidenceDropsUnattributableRows(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	orphan := searchRow("chunk-anon", "doc-anon", "", "无归属", 1)
	kept := searchRow("chunk-own", "doc-own", "kb-own", "手册", 3)
	cm := evidenceChatManage(bus, []*types.SearchResult{orphan, kept})

	state := svc.deliverKnowledgeEvidence(knowledgeEvidenceCallerContext(), cm)
	require.Equal(t, types.EvidenceStateCited, state)
	require.Len(t, capture.evidence[0].Citations, 1)
	require.Equal(t, "chunk-own", capture.evidence[0].Citations[0].CitationID)
}

// 场景 2（无证据）：检索为空 → 显式 no_evidence 信封（零引用零结论），随后既有 fallback。
func TestKnowledgeQAEvidenceNoEvidenceIsExplicit(t *testing.T) {
	svc, _, bus, capture := newKnowledgeEvidenceEnv(t)
	cm := evidenceChatManage(bus, nil)
	// 检索为空时管线不会进入 deliverKnowledgeEvidence 的 cited 路径；直接断言
	// no_evidence 信封的组装与发射（KnowledgeQAByEvent 的 ErrSearchNothing 分支调用同一方法）。
	svc.emitKnowledgeEvidenceEvent(knowledgeEvidenceCallerContext(), cm, types.EvidenceStateNoEvidence, types.EvidenceReasoning{})
	require.Len(t, capture.evidence, 1)
	envelope := capture.evidence[0]
	require.Equal(t, types.EvidenceStateNoEvidence, envelope.State)
	require.Empty(t, envelope.Citations)
	require.Empty(t, envelope.Conclusions)
	require.NoError(t, types.ValidateAnswerEvidence(envelope))
}

// ADR-0002：明确请求推理而部署未接入语义推理服务 → 推理未完成 + 重试入口，
// 零结论零引用，不用普通检索冒充推理成功。
func TestKnowledgeQAReasoningRequestShortCircuitsIncomplete(t *testing.T) {
	svc, _, bus, capture := newKnowledgeEvidenceEnv(t)
	err := svc.KnowledgeQA(knowledgeEvidenceCallerContext(), &types.QARequest{
		Session:       &types.Session{ID: "sess-1"},
		Query:         "A 是否间接依赖 C？",
		ReasoningMode: "rules",
	}, bus)
	require.NoError(t, err)
	require.Len(t, capture.evidence, 1)
	envelope := capture.evidence[0]
	require.Equal(t, types.EvidenceStateNoEvidence, envelope.State)
	require.Empty(t, envelope.Citations, "不得以检索结果冒充推理证据")
	require.Empty(t, envelope.Conclusions, "不得产出任何被标类结论")
	require.True(t, envelope.Reasoning.Requested)
	require.Equal(t, "rules", envelope.Reasoning.Mode)
	require.Equal(t, types.EvidenceReasoningIncomplete, envelope.Reasoning.State)
	require.True(t, envelope.Reasoning.Retryable)
	require.Len(t, capture.fallback, 1)
	require.Contains(t, capture.fallback[0], "推理未完成", "收尾文案必须显式声明推理未完成（emitFallbackAnswer 可能按引用开关重写标记，断言语义而非整串）")
	require.Empty(t, capture.other, "短路后不得再发 references/answer 等业务帧")
}
```

`internal/handler/session/qa_reasoning_mode_test.go`（新文件，完整内容）：

```go
package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseQARequestRejectsUnknownReasoningMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "session_id", Value: "sess-1"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/knowledge-chat/sess-1", strings.NewReader(`{"query":"q","reasoning_mode":"fast"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := &Handler{}
	_, _, err := h.parseQARequest(c, "KnowledgeQA")
	require.Error(t, err, "reasoning_mode 白名单外取值必须在进入任何服务调用前以 400 拒绝")
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/application/service/ -run 'TestKnowledgeQAEvidence|TestKnowledgeQAReasoning' -count=1 && go test ./internal/handler/session/ -run 'TestParseQARequestRejectsUnknownReasoningMode' -count=1`
Expected: FAIL（编译错误：`deliverKnowledgeEvidence`/`applyEvidenceRevocation`/`emitKnowledgeEvidenceEvent`/`knowledgeEvidenceRevokedCopy`/`QARequest.ReasoningMode` 未定义；handler 用例因请求字段被忽略而实际通过 `ShouldBindJSON` 后继续走到 session 查询——失败形态为非 400 的后续错误或 panic，确认非「接受未知模式」即可）

- [ ] **Step 3: 写最小实现**

3a. `internal/types/qa_request.go`：`QARequest` 结构体 `Attachments` 字段前追加：

```go
	// ReasoningMode is an explicit reasoning request (T15): "rules" or "model".
	// Empty means ordinary retrieval QA. When the deployment has no semantic
	// reasoning wired into the QA path the turn must end as reasoning-incomplete
	// with a retry entry — never masquerade retrieval as reasoning (ADR-0002).
	ReasoningMode string
```

3b. `internal/handler/session/types.go`：`CreateKnowledgeQARequest` 的 `Channel` 字段后追加：

```go
	ReasoningMode          string                       `json:"reasoning_mode,omitempty"` // Explicit reasoning request: "rules" | "model" (T15)
```

3c. `internal/handler/session/qa.go` 三处：
- `qaRequestContext` 结构体（`reqAgentID string` 行后）追加：

```go
	reasoningMode         string // T15: "rules" | "model" | ""
```

- `buildQARequest` 的 `WebSearchEnabled: rc.webSearchEnabled,` 行后追加：

```go
		ReasoningMode:        rc.reasoningMode,
```

- `parseQARequest` 中 `if request.Query == "" { ... }` 校验块后追加：

```go
	// T15: reasoning_mode is a closed enum; unknown values must fail as 400
	// before any session/service call, not silently degrade to normal QA.
	if request.ReasoningMode != "" && request.ReasoningMode != "rules" && request.ReasoningMode != "model" {
		logger.Error(ctx, "Invalid reasoning mode", request.ReasoningMode)
		return nil, nil, errors.NewBadRequestError(`reasoning_mode must be "rules" or "model"`)
	}
	rc.reasoningMode = request.ReasoningMode
```

3d. `internal/application/service/session_knowledge_qa.go`：

文件内（`emitKnowledgeReferencesEvent` 函数之后）新增方法与常量：

```go
const (
	// knowledgeEvidenceRevokedCopy 撤权作废的固定文案（ADR-0002：受影响结果作废）。
	knowledgeEvidenceRevokedCopy = "本回答所依据的知识访问已被撤销，结果已作废。请重新提问。"
	// knowledgeReasoningUnavailableCopy 显式请求推理但部署未接入语义推理服务的诚实收尾
	//（ADR-0002：显示推理未完成和重试入口，不能用普通检索结果冒充推理成功）。
	knowledgeReasoningUnavailableCopy = "推理未完成：当前部署尚未接入语义推理服务。请稍后重试，或改用普通知识问答。"
)

// deliverKnowledgeEvidence 在答案流开始前交付证据（T15）：交付前撤权重校验 →
// 引用帧（既有形状，现为过滤后行集）→ 证据信封帧。全部证据失权时按 ADR-0002
// 作废本回答（固定文案 fallback），返回状态供调用方跳过 completion。
func (s *sessionService) deliverKnowledgeEvidence(ctx context.Context, chatManage *types.ChatManage) types.AnswerEvidenceState {
	state := s.applyEvidenceRevocation(ctx, chatManage)
	emitKnowledgeReferencesEvent(ctx, chatManage)
	s.emitKnowledgeEvidenceEvent(ctx, chatManage, state, types.EvidenceReasoning{})
	if state == types.EvidenceStateRevoked {
		s.emitFallbackAnswer(ctx, chatManage, knowledgeEvidenceRevokedCopy)
	}
	return state
}

// applyEvidenceRevocation 在交付前对每条检索结果的知识库读权限做实时重校验
//（ADR-0002：删除或撤权立即阻止失效证据——执行中请求在交付前重校验，受影响
// 结果作废或重算）。被撤销、从未获准或不可归属（KnowledgeBaseID 为空/KB 已删除）
// 的行同时从证据与 prompt 上下文（MergeResult）移除。无法完成校验时按全失权处理
//（fail closed：不能证明权限即不得交付）。
func (s *sessionService) applyEvidenceRevocation(ctx context.Context, chatManage *types.ChatManage) types.AnswerEvidenceState {
	if len(chatManage.MergeResult) == 0 {
		return types.EvidenceStateNoEvidence
	}
	if s.knowledgeBaseService == nil || s.kbShareService == nil {
		chatManage.MergeResult = nil
		return types.EvidenceStateRevoked
	}
	kbIDs := make([]string, 0, len(chatManage.MergeResult))
	for _, result := range chatManage.MergeResult {
		if result != nil && result.KnowledgeBaseID != "" {
			kbIDs = append(kbIDs, result.KnowledgeBaseID)
		}
	}
	kbs, err := s.knowledgeBaseService.GetKnowledgeBasesByIDsOnly(ctx, uniqueNonEmptyStrings(kbIDs))
	if err != nil {
		chatManage.MergeResult = nil
		return types.EvidenceStateRevoked
	}
	kbTenant := make(map[string]uint64, len(kbs))
	for _, kb := range kbs {
		if kb != nil {
			kbTenant[kb.ID] = kb.TenantID
		}
	}
	permissions := kbReadPermissions(ctx, s.kbShareService)
	kept := make([]*types.SearchResult, 0, len(chatManage.MergeResult))
	for _, result := range chatManage.MergeResult {
		if result == nil || result.KnowledgeBaseID == "" {
			continue
		}
		tenantID, ok := kbTenant[result.KnowledgeBaseID]
		if !ok {
			continue // KB 已删除/不可见：与撤权同形
		}
		allowed, err := permissions.Check(result.KnowledgeBaseID, tenantID, types.OrgRoleViewer)
		if err != nil || !allowed {
			continue
		}
		kept = append(kept, result)
	}
	chatManage.MergeResult = kept
	if len(kept) == 0 {
		return types.EvidenceStateRevoked
	}
	return types.EvidenceStateCited
}

// emitKnowledgeEvidenceEvent 组装并发射证据信封（T15）。信封先过交付不变量
//（ValidateAnswerEvidence）：不自洽的证据不得上线。
func (s *sessionService) emitKnowledgeEvidenceEvent(ctx context.Context, chatManage *types.ChatManage, state types.AnswerEvidenceState, reasoning types.EvidenceReasoning) {
	if chatManage == nil || chatManage.EventBus == nil {
		return
	}
	if reasoning.State == "" {
		reasoning.State = types.EvidenceReasoningNotRequested
	}
	retrievedAt := time.Now().UTC()
	envelope := types.AnswerEvidence{
		State:             state,
		SemanticGraphUsed: false, // 本地检索路径显式注明未使用语义图谱（ADR-0002）
		RetrievedAt:       retrievedAt.Format(time.RFC3339),
		Citations:         []types.EvidenceCitation{},
		Conclusions:       []types.EvidenceConclusion{},
		Reasoning:         reasoning,
	}
	if state == types.EvidenceStateCited {
		citations := types.CitationsFromSearchResults(chatManage.MergeResult, retrievedAt)
		citationIDs := make([]string, 0, len(citations))
		for _, citation := range citations {
			citationIDs = append(citationIDs, citation.CitationID)
		}
		envelope.Citations = citations
		envelope.Conclusions = []types.EvidenceConclusion{types.ConclusionFromNativeAnswer(chatManage.ChatModelID, citationIDs)}
	}
	if err := types.ValidateAnswerEvidence(envelope); err != nil {
		logger.Errorf(ctx, "Refusing to emit invalid answer evidence: %v", err)
		return
	}
	if err := chatManage.EventBus.Emit(ctx, types.Event{
		ID:        generateEventID("evidence"),
		Type:      types.EventType(event.EventAgentEvidence),
		SessionID: chatManage.SessionID,
		Data:      event.AgentEvidenceData{Evidence: envelope},
	}); err != nil {
		logger.Errorf(ctx, "Failed to emit answer evidence event: %v", err)
	}
}

// emitReasoningIncomplete 处理显式推理请求的诚实不可用（ADR-0002）：证据信封标注
// reasoning.state=incomplete + retryable，随后以固定文案收尾；零结论零引用，
// 不执行检索，不用普通检索结果冒充推理成功。
func (s *sessionService) emitReasoningIncomplete(ctx context.Context, eventBus *event.EventBus, req *types.QARequest) {
	envelope := types.AnswerEvidence{
		State:             types.EvidenceStateNoEvidence,
		SemanticGraphUsed: false,
		RetrievedAt:       time.Now().UTC().Format(time.RFC3339),
		Citations:         []types.EvidenceCitation{},
		Conclusions:       []types.EvidenceConclusion{},
		Reasoning: types.EvidenceReasoning{
			Requested: true, Mode: req.ReasoningMode,
			State: types.EvidenceReasoningIncomplete, Reason: "semantic_reasoning_unavailable", Retryable: true,
		},
	}
	if err := eventBus.Emit(ctx, event.Event{
		ID:        generateEventID("evidence"),
		Type:      event.EventAgentEvidence,
		SessionID: req.Session.ID,
		Data:      event.AgentEvidenceData{Evidence: envelope},
	}); err != nil {
		logger.Errorf(ctx, "Failed to emit reasoning-incomplete evidence event: %v", err)
	}
	if err := eventBus.Emit(ctx, event.Event{
		ID:        generateEventID("fallback"),
		Type:      event.EventAgentFinalAnswer,
		SessionID: req.Session.ID,
		Data:      event.AgentFinalAnswerData{Content: knowledgeReasoningUnavailableCopy, Done: true, IsFallback: true},
	}); err != nil {
		logger.Errorf(ctx, "Failed to emit reasoning-unavailable fallback answer: %v", err)
	}
}
```

`KnowledgeQA` 方法内、`logger.Infof(...)` 首条日志之后、`setupCtx, setupSpan := langfuse...` 之前插入短路口：

```go
	// T15（ADR-0002）：明确请求推理时才推理。当前部署的 QA 链路尚未接入语义推理
	// 服务（Semantica rollout 试点门控），一律以「推理未完成」显式收尾并提供重试
	// 入口，不得用普通检索结果冒充推理成功。
	if req.ReasoningMode != "" {
		s.emitReasoningIncomplete(ctx, eventBus, req)
		return nil
	}
```

`KnowledgeQAByEvent` 循环内，把既有块：

```go
		if eventType == types.CHAT_COMPLETION_STREAM {
			emitKnowledgeReferencesEvent(ctx, chatManage)
		}
```

替换为：

```go
		if eventType == types.CHAT_COMPLETION_STREAM {
			// T15：交付前撤权重校验 + 引用帧 + 证据信封帧；全部证据失权时作废本
			// 回答（固定文案已发），不进入 completion。
			if s.deliverKnowledgeEvidence(ctx, chatManage) == types.EvidenceStateRevoked {
				return nil
			}
		}
```

`KnowledgeQAByEvent` 的 `err == chatpipeline.ErrSearchNothing` 分支，在 `s.handleFallbackResponse(ctx, chatManage)` 之前插入一行：

```go
			s.emitKnowledgeEvidenceEvent(ctx, chatManage, types.EvidenceStateNoEvidence, types.EvidenceReasoning{})
```

（保证客户端先看到 no_evidence 信封、再看到 fallback 答案文本。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/service/ -run 'TestKnowledgeQAEvidence|TestKnowledgeQAReasoning' -count=1 && go test ./internal/handler/session/ -run 'TestParseQARequestRejectsUnknownReasoningMode|TestEvidenceEventStreams' -count=1 && go test ./internal/application/service/ -run 'TestEmitKnowledgeReferencesEventIgnoresCitationOutputSetting' -count=1`
Expected: PASS（最后一项是既有引用帧行为回归——过滤后空集不再发射引用帧，与既有 `len(MergeResult) == 0` 早退一致）

- [ ] **Step 5: 提交**

```bash
git add internal/application/service/session_knowledge_qa.go internal/application/service/knowledge_evidence_test.go internal/types/qa_request.go internal/handler/session/types.go internal/handler/session/qa.go internal/handler/session/qa_reasoning_mode_test.go
git commit -m "feat(qa): deliver evidence envelope with pre-delivery revocation revalidation and honest reasoning-incomplete"
```

---

### Task 4: contracts——证据信封解析器

**Files:**
- Create: `packages/contracts/src/mobile/knowledge-evidence.ts`
- Modify: `packages/contracts/src/index.ts`（文件末追加一行导出）
- Test: `packages/contracts/test/mobile-knowledge-evidence.test.ts`

**Interfaces:**
- Consumes: 无（contracts 只含 wire 形状与解析器；不导入其他包）。
- Produces: `EvidenceKindWire = 'fact' | 'rule_derived' | 'model_inferred'`、`AnswerEvidenceStateWire = 'cited' | 'no_evidence' | 'revoked'`、`EvidenceReasoningStateWire = 'not_requested' | 'incomplete'`、`EvidenceCitationWire`/`EvidenceConclusionWire`/`EvidenceReasoningWire`/`AnswerEvidenceWire`（字段与 Task 1 的 Go JSON 键集逐字对应：`citation_id`/`knowledge_id`/`knowledge_base_id`/`title?`/`revision`/`start_at?`/`end_at?`/`quote?`/`kind`/`retrieved_at`；`kind`/`model_id?`/`rule_ids?`/`citation_ids?`；`requested`/`mode?`/`state`/`reason?`/`retryable`；`state`/`semantic_graph_used`/`retrieved_at`/`citations`/`conclusions`/`reasoning`）、`parseAnswerEvidence(value: unknown): AnswerEvidenceWire`（整体拒绝非法输入）。Task 6 消费。

- [ ] **Step 1: 写失败测试**

`packages/contracts/test/mobile-knowledge-evidence.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { parseAnswerEvidence, type AnswerEvidenceWire } from '../src/mobile/knowledge-evidence.ts';

const citedEnvelope = {
  state: 'cited',
  semantic_graph_used: false,
  retrieved_at: '2026-09-24T08:00:00Z',
  citations: [
    { citation_id: 'chunk-1', knowledge_id: 'doc-1', knowledge_base_id: 'kb-own', title: '手册', revision: 3, start_at: 10, end_at: 40, quote: '命中', kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
    { citation_id: 'chunk-2', knowledge_id: 'doc-2', knowledge_base_id: 'kb-shared', revision: 5, kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
  ],
  conclusions: [
    { kind: 'model_inferred', model_id: 'chat-model-1', citation_ids: ['chunk-1', 'chunk-2'] },
  ],
  reasoning: { state: 'not_requested' },
};

test('parseAnswerEvidence accepts a multi-source cited envelope and keeps wire fields verbatim', () => {
  const parsed = parseAnswerEvidence(citedEnvelope);
  assert.equal(parsed.state, 'cited');
  assert.equal(parsed.citations.length, 2);
  assert.equal(parsed.citations[0]!.revision, 3);
  assert.equal(parsed.citations[0]!.kind, 'fact');
  assert.equal(parsed.citations[1]!.knowledge_base_id, 'kb-shared');
  assert.deepEqual(parsed.conclusions[0]!.citation_ids, ['chunk-1', 'chunk-2']);
  assert.equal(parsed.semantic_graph_used, false);
});

test('parseAnswerEvidence accepts explicit no_evidence and reasoning-incomplete envelopes', () => {
  const noEvidence = parseAnswerEvidence({ state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { state: 'not_requested' } });
  assert.equal(noEvidence.state, 'no_evidence');
  assert.equal(noEvidence.citations.length, 0);
  const incomplete = parseAnswerEvidence({ state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'semantic_reasoning_unavailable', retryable: true } });
  assert.equal(incomplete.reasoning.state, 'incomplete');
  assert.equal(incomplete.reasoning.retryable, true);
});

test('parseAnswerEvidence rejects malformed and inconsistent envelopes wholesale', () => {
  const cases: unknown[] = [
    null,
    'cited',
    {},
    { ...citedEnvelope, state: 'maybe' },
    { ...citedEnvelope, retrieved_at: '2026-09-24 08:00:00' },
    { ...citedEnvelope, citations: [] },
    { ...citedEnvelope, conclusions: [] },
    { ...citedEnvelope, citations: [{ ...citedEnvelope.citations[0], kind: 'model_inferred' }] },
    { ...citedEnvelope, citations: [{ ...citedEnvelope.citations[0], revision: -1 }] },
    { ...citedEnvelope, citations: [{ ...citedEnvelope.citations[0], retrieved_at: 'yesterday' }] },
    { ...citedEnvelope, conclusions: [{ kind: 'rule_derived', citation_ids: ['chunk-1'] }] },
    { ...citedEnvelope, conclusions: [{ kind: 'model_inferred', citation_ids: ['missing'] }] },
    { ...citedEnvelope, conclusions: [{ kind: 'oracle', model_id: 'x' }] },
    { state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: citedEnvelope.citations, conclusions: [], reasoning: { state: 'not_requested' } },
  ];
  for (const value of cases) {
    assert.throws(() => parseAnswerEvidence(value), undefined, `must reject ${JSON.stringify(value).slice(0, 60)}`);
  }
});

test('parseAnswerEvidence type-level sanity: parsed cited envelope matches AnswerEvidenceWire', () => {
  const parsed: AnswerEvidenceWire = parseAnswerEvidence(citedEnvelope);
  assert.ok(Array.isArray(parsed.citations));
  assert.ok(Array.isArray(parsed.conclusions));
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/contracts/test/mobile-knowledge-evidence.test.ts`
Expected: FAIL（模块不存在，导入报错）

- [ ] **Step 3: 写最小实现**

`packages/contracts/src/mobile/knowledge-evidence.ts`（新文件，完整内容）：

```ts
// T15（Issue #45）：knowledge-chat SSE evidence 帧的信封契约。键集与
// internal/types/evidence.go 的 JSON tag 逐字对应；解析器整体拒绝（不部分渲染），
// 与服务端 ValidateAnswerEvidence 同一组不变量的客户端侧守门。

export type EvidenceKindWire = 'fact' | 'rule_derived' | 'model_inferred';
export type AnswerEvidenceStateWire = 'cited' | 'no_evidence' | 'revoked';
export type EvidenceReasoningStateWire = 'not_requested' | 'incomplete';

export interface EvidenceCitationWire {
  citation_id: string;
  knowledge_id: string;
  knowledge_base_id: string;
  title?: string;
  revision: number;
  start_at?: number;
  end_at?: number;
  quote?: string;
  kind: 'fact';
  retrieved_at: string;
}

export interface EvidenceConclusionWire {
  kind: EvidenceKindWire;
  model_id?: string;
  rule_ids?: string[];
  citation_ids?: string[];
}

export interface EvidenceReasoningWire {
  requested: boolean;
  mode?: string;
  state: EvidenceReasoningStateWire;
  reason?: string;
  retryable: boolean;
}

export interface AnswerEvidenceWire {
  state: AnswerEvidenceStateWire;
  semantic_graph_used: boolean;
  retrieved_at: string;
  citations: EvidenceCitationWire[];
  conclusions: EvidenceConclusionWire[];
  reasoning: EvidenceReasoningWire;
}

const rfc3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function stringField(row: Record<string, unknown>, key: string, path: string, optional = false): string | undefined {
  const value = row[key];
  if (value === undefined) {
    if (optional) return undefined;
    throw new Error(`${path}.${key} is required`);
  }
  if (typeof value !== 'string' || value === '') throw new Error(`${path}.${key} must be a non-empty string`);
  return value;
}

function intField(row: Record<string, unknown>, key: string, path: string): number {
  const value = row[key];
  if (typeof value !== 'number' || !Number.isSafeInteger(value)) throw new Error(`${path}.${key} must be an integer`);
  return value;
}

function parseCitation(value: unknown, path: string): EvidenceCitationWire {
  if (!isObject(value)) throw new Error(`${path} must be an object`);
  if (value.kind !== 'fact') throw new Error(`${path}.kind must be "fact"`);
  const revision = intField(value, 'revision', path);
  if (revision < 0) throw new Error(`${path}.revision must not be negative`);
  const retrievedAt = stringField(value, 'retrieved_at', path)!;
  if (!rfc3339.test(retrievedAt)) throw new Error(`${path}.retrieved_at must be RFC3339`);
  const citation: EvidenceCitationWire = {
    citation_id: stringField(value, 'citation_id', path)!,
    knowledge_id: stringField(value, 'knowledge_id', path)!,
    knowledge_base_id: stringField(value, 'knowledge_base_id', path)!,
    revision,
    kind: 'fact',
    retrieved_at: retrievedAt,
  };
  const title = stringField(value, 'title', path, true);
  if (title !== undefined) citation.title = title;
  const quote = stringField(value, 'quote', path, true);
  if (quote !== undefined) citation.quote = quote;
  if (value.start_at !== undefined) citation.start_at = intField(value, 'start_at', path);
  if (value.end_at !== undefined) citation.end_at = intField(value, 'end_at', path);
  return citation;
}

function parseConclusion(value: unknown, path: string, citationIds: Set<string>): EvidenceConclusionWire {
  if (!isObject(value)) throw new Error(`${path} must be an object`);
  const kind = value.kind;
  if (kind !== 'fact' && kind !== 'rule_derived' && kind !== 'model_inferred') {
    throw new Error(`${path}.kind must be fact | rule_derived | model_inferred`);
  }
  const conclusion: EvidenceConclusionWire = { kind };
  const modelId = stringField(value, 'model_id', path, true);
  const ruleIds = value.rule_ids;
  if (kind === 'model_inferred') {
    if (modelId === undefined) throw new Error(`${path} model_inferred requires model_id`);
    conclusion.model_id = modelId;
  }
  if (kind === 'rule_derived') {
    if (!Array.isArray(ruleIds) || ruleIds.length === 0 || !ruleIds.every((id) => typeof id === 'string' && id !== '')) {
      throw new Error(`${path} rule_derived requires non-empty rule_ids`);
    }
    conclusion.rule_ids = ruleIds as string[];
  }
  if (value.citation_ids !== undefined) {
    if (!Array.isArray(value.citation_ids) || !value.citation_ids.every((id) => typeof id === 'string')) {
      throw new Error(`${path}.citation_ids must be an array of strings`);
    }
    for (const id of value.citation_ids as string[]) {
      if (!citationIds.has(id)) throw new Error(`${path} references unknown citation ${id}`);
    }
    conclusion.citation_ids = value.citation_ids as string[];
  }
  return conclusion;
}

export function parseAnswerEvidence(value: unknown): AnswerEvidenceWire {
  if (!isObject(value)) throw new Error('answer evidence must be an object');
  const state = value.state;
  if (state !== 'cited' && state !== 'no_evidence' && state !== 'revoked') {
    throw new Error('answer evidence state must be cited | no_evidence | revoked');
  }
  if (typeof value.semantic_graph_used !== 'boolean') throw new Error('answer evidence semantic_graph_used must be a boolean');
  const retrievedAt = stringField(value, 'retrieved_at', 'answer evidence')!;
  if (!rfc3339.test(retrievedAt)) throw new Error('answer evidence retrieved_at must be RFC3339');
  if (!Array.isArray(value.citations)) throw new Error('answer evidence citations must be an array');
  if (!Array.isArray(value.conclusions)) throw new Error('answer evidence conclusions must be an array');
  const citations = value.citations.map((citation, index) => parseCitation(citation, `citations[${index}]`));
  const citationIds = new Set(citations.map((citation) => citation.citation_id));
  if (citationIds.size !== citations.length) throw new Error('answer evidence citation_id is duplicated');
  const conclusions = value.conclusions.map((conclusion, index) => parseConclusion(conclusion, `conclusions[${index}]`, citationIds));
  if (state === 'cited' && citations.length === 0) throw new Error('cited answer evidence requires at least one citation');
  if (state === 'cited' && conclusions.length === 0) throw new Error('cited answer evidence requires at least one conclusion');
  if (state !== 'cited' && (citations.length > 0 || conclusions.length > 0)) {
    throw new Error(`${state} answer evidence must not carry citations or conclusions`);
  }
  const reasoningValue = value.reasoning;
  if (!isObject(reasoningValue)) throw new Error('answer evidence reasoning is required');
  const reasoningState = reasoningValue.state;
  if (reasoningState !== 'not_requested' && reasoningState !== 'incomplete') {
    throw new Error('answer evidence reasoning.state must be not_requested | incomplete');
  }
  const reasoning: EvidenceReasoningWire = {
    requested: reasoningValue.requested === true,
    state: reasoningState,
    retryable: reasoningValue.retryable === true,
  };
  const mode = stringField(reasoningValue, 'mode', 'answer evidence reasoning', true);
  if (mode !== undefined) reasoning.mode = mode;
  const reason = stringField(reasoningValue, 'reason', 'answer evidence reasoning', true);
  if (reason !== undefined) reasoning.reason = reason;
  return { state, semantic_graph_used: value.semantic_graph_used, retrieved_at: retrievedAt, citations, conclusions, reasoning };
}
```

`packages/contracts/src/index.ts` 文件末追加：

```ts
export { parseAnswerEvidence } from './mobile/knowledge-evidence.ts';
export type { AnswerEvidenceStateWire, AnswerEvidenceWire, EvidenceCitationWire, EvidenceConclusionWire, EvidenceKindWire, EvidenceReasoningStateWire, EvidenceReasoningWire } from './mobile/knowledge-evidence.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/contracts/test/mobile-knowledge-evidence.test.ts`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add packages/contracts/src/mobile/knowledge-evidence.ts packages/contracts/src/index.ts packages/contracts/test/mobile-knowledge-evidence.test.ts
git commit -m "feat(contracts): answer evidence envelope parser with masquerade rejection"
```

---

### Task 5: mobile-core——知识问答域与 Task Office askKnowledge 入口

**Files:**
- Create: `packages/mobile-core/src/task-office/knowledge-qa.ts`
- Modify: `packages/mobile-core/src/task-office/task-office.ts`（import 一行；`TaskOfficePorts` 追加可选字段；`TaskOffice` 接口追加方法；`createTaskOffice` 返回对象 `followUp` 后追加实现——均标记「T15 追加区」）
- Modify: `packages/mobile-core/src/task-office/task-office-errors.ts`（错误码联合追加一员）
- Modify: `packages/mobile-core/src/index.ts`（追加导出块）
- Test: `packages/mobile-core/src/task-office/knowledge-qa.test.ts`
- Test: `packages/mobile-core/src/task-office/knowledge-qa-office.test.ts`

**Interfaces:**
- Consumes: Task 4 的 wire 形状（DTO 与之逐字一致但 camelCase 化，由 Task 6 的 remote 完成 wire→DTO 映射）；既有 `TaskBackendPort.createSession(input: { title: string }): Promise<{ sessionId: string }>`（task-office.ts:160）、`RuntimeScopeLease`/`leaseActive`（scope-lease.ts）、`TaskOfficeError`、office 测试夹具形态（task-office.test.ts:10-27 的 `leased()`/`officeWith` 模式）、`createScenarioTaskBackend`（in-memory-task-backend.ts，含 `createSession` handler）。
- Produces: `EvidenceKindValue`/`AnswerEvidenceStateValue`/`EvidenceReasoningStateValue`、`EvidenceCitation`/`EvidenceConclusion`/`EvidenceReasoning`/`AnswerEvidenceView`/`KnowledgeQATurn`/`KnowledgeQAAskInput`、`KnowledgeQABackendPort { ask(input: { sessionId: string; question: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<KnowledgeQATurnBody> }`（`KnowledgeQATurnBody = Omit<KnowledgeQATurn, 'sessionId'>`）、`EVIDENCE_KIND_LABEL`（`fact`→`原文事实`、`rule_derived`→`规则推导`、`model_inferred`→`模型推断`）、`evidenceRetryable(evidence: AnswerEvidenceView): boolean`、`createScenarioKnowledgeQABackend(handlers?)`、`TaskOfficePorts.knowledgeQA?`、`TaskOffice.askKnowledge(input: { question: string; sessionId?: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<KnowledgeQATurn>`、错误码 `TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE`。Task 6/7 消费。

- [ ] **Step 1: 写失败测试**

`packages/mobile-core/src/task-office/knowledge-qa.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  EVIDENCE_KIND_LABEL, createScenarioKnowledgeQABackend, evidenceRetryable,
  type AnswerEvidenceView,
} from './knowledge-qa.ts';

function citedEvidence(): AnswerEvidenceView {
  return {
    state: 'cited',
    semanticGraphUsed: false,
    retrievedAt: '2026-09-24T08:00:00Z',
    citations: [
      { citationId: 'chunk-1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-own', title: '手册', revision: 3, kind: 'fact', retrievedAt: '2026-09-24T08:00:00Z' },
      { citationId: 'chunk-2', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-shared', revision: 5, kind: 'fact', retrievedAt: '2026-09-24T08:00:00Z' },
    ],
    conclusions: [{ kind: 'model_inferred', modelId: 'chat-model-1', citationIds: ['chunk-1', 'chunk-2'] }],
    reasoning: { requested: false, state: 'not_requested', retryable: false },
  };
}

test('kind labels keep the three classes distinct for display', () => {
  assert.equal(EVIDENCE_KIND_LABEL.fact, '原文事实');
  assert.equal(EVIDENCE_KIND_LABEL.rule_derived, '规则推导');
  assert.equal(EVIDENCE_KIND_LABEL.model_inferred, '模型推断');
});

test('evidenceRetryable is true only for an explicitly requested, incomplete reasoning turn', () => {
  assert.equal(evidenceRetryable(citedEvidence()), false);
  const incomplete: AnswerEvidenceView = {
    state: 'no_evidence', semanticGraphUsed: false, retrievedAt: '2026-09-24T08:00:00Z',
    citations: [], conclusions: [],
    reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'semantic_reasoning_unavailable', retryable: true },
  };
  assert.equal(evidenceRetryable(incomplete), true);
  const unrequested: AnswerEvidenceView = { ...incomplete, reasoning: { requested: false, state: 'not_requested', retryable: false } };
  assert.equal(evidenceRetryable(unrequested), false);
});

test('scenario backend records ask calls and returns scripted turns', async () => {
  const backend = createScenarioKnowledgeQABackend({
    ask: async (input) => ({ answer: '答', isFallback: false, evidence: citedEvidence() }),
  });
  const turn = await backend.ask({ sessionId: 'sess-1', question: '问', knowledgeBaseIds: ['kb-own'] });
  assert.equal(turn.answer, '答');
  assert.equal(turn.evidence.citations.length, 2);
  assert.deepEqual(backend.calls, [{ kind: 'ask', input: { sessionId: 'sess-1', question: '问', knowledgeBaseIds: ['kb-own'] } }]);
  const empty = createScenarioKnowledgeQABackend();
  const none = await empty.ask({ sessionId: 'sess-1', question: '问' });
  assert.equal(none.evidence.citations.length, 0);
});
```

`packages/mobile-core/src/task-office/knowledge-qa-office.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';
import { createScenarioTaskBackend } from './in-memory-task-backend.ts';
import { createScenarioKnowledgeQABackend } from './knowledge-qa.ts';
import { createTaskOffice, TaskOfficeError } from './task-office.ts';

function leased() {
  const revocable = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.test', userId: 'user-1', tenantId: 'tenant-1' });
  return { revocable, lease: revocable.asScopeLease() };
}

function officeWithKnowledgeQA(leaseRef: { lease?: ScopeLease }, handlers: Parameters<typeof createScenarioKnowledgeQABackend>[0] = {}, created: string[] = []) {
  const backend = createScenarioTaskBackend({
    createSession: async (input) => {
      created.push(input.title);
      return { sessionId: `new-${created.length}` };
    },
  });
  const knowledgeQA = createScenarioKnowledgeQABackend(handlers);
  const office = createTaskOffice({ backend, knowledgeQA, lease: () => leaseRef.lease });
  return { backend, knowledgeQA, office };
}

test('askKnowledge creates a session for a quick question and returns the evidence turn', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const created: string[] = [];
  const { knowledgeQA, office } = officeWithKnowledgeQA(leaseRef, {
    ask: async (input) => ({ answer: '回答', isFallback: false, evidence: citedOfficeEvidence() }),
  }, created);

  const turn = await office.askKnowledge({ question: '  依赖关系是什么？  ', knowledgeBaseIds: ['kb-own'] });

  assert.equal(turn.sessionId, 'new-1');
  assert.equal(turn.answer, '回答');
  assert.equal(turn.evidence.citations.length, 2);
  assert.deepEqual(created, ['依赖关系是什么？'.slice(0, 60)]);
  assert.deepEqual(knowledgeQA.calls, [{ kind: 'ask', input: { sessionId: 'new-1', question: '依赖关系是什么？', knowledgeBaseIds: ['kb-own'] } }]);
});

test('askKnowledge reuses a provided sessionId and never creates a second session', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const created: string[] = [];
  const { knowledgeQA, office } = officeWithKnowledgeQA(leaseRef, {
    ask: async () => ({ answer: '追问回答', isFallback: false, evidence: citedOfficeEvidence() }),
  }, created);

  const turn = await office.askKnowledge({ question: '再展开讲讲', sessionId: 'sess-existing' });

  assert.equal(turn.sessionId, 'sess-existing');
  assert.deepEqual(created, [], '同一 Task 的追问绝不新建 session（ADR-0004）');
  assert.equal(knowledgeQA.calls[0]!.input.sessionId, 'sess-existing');
});

test('askKnowledge fails closed without a knowledgeQA port and rejects invalid input', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  const backend = createScenarioTaskBackend({});
  const office = createTaskOffice({ backend, lease: () => leaseRef.lease });
  await assert.rejects(
    office.askKnowledge({ question: '问' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE',
  );
  // 空白/超长问题在端口检查之前就被 INVALID_INPUT 拒绝（office 无端口也先命中输入校验）。
  await assert.rejects(
    office.askKnowledge({ question: '   ' }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
  );
  await assert.rejects(
    office.askKnowledge({ question: 'x'.repeat(8001) }),
    (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_INVALID_INPUT',
  );
});

test('a late ask response after the scope lease was revoked is rejected, never resolves with foreign-scope data', async () => {
  const { revocable, lease } = leased();
  const leaseRef: { lease?: ScopeLease } = { lease };
  let releaseAsk: ((value: { answer: string; isFallback: boolean; evidence: ReturnType<typeof citedOfficeEvidence> }) => void) | undefined;
  const askGate = new Promise<{ answer: string; isFallback: boolean; evidence: ReturnType<typeof citedOfficeEvidence> }>((resolve) => { releaseAsk = resolve; });
  const { office } = officeWithKnowledgeQA(leaseRef, { ask: () => askGate });

  const pending = office.askKnowledge({ question: '问' });
  revocable.revoke();
  releaseAsk!({ answer: 'foreign', isFallback: false, evidence: citedOfficeEvidence() });
  await assert.rejects(pending, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SCOPE_CHANGED');
});

test('askKnowledge invalidates in-flight list reads like followUp does', async () => {
  const leaseRef: { lease?: ScopeLease } = {};
  leaseRef.lease = leased().lease;
  // list 必须挂起：若它先返回，settle 已在旧 epoch 下完成，就测不到作废语义。
  let resolveList: ((page: { items: []; nextCursor?: undefined }) => void) | undefined;
  const listGate = new Promise<{ items: []; nextCursor?: undefined }>((resolve) => { resolveList = resolve; });
  const backend = createScenarioTaskBackend({ list: () => listGate });
  const knowledgeQA = createScenarioKnowledgeQABackend({ ask: async () => ({ answer: '答', isFallback: false, evidence: citedOfficeEvidence() }) });
  const office = createTaskOffice({ backend, knowledgeQA, lease: () => leaseRef.lease });

  const older = office.tasks({});
  await office.askKnowledge({ question: '问' });
  resolveList!({ items: [] });
  await assert.rejects(older, (error: unknown) => error instanceof TaskOfficeError && error.code === 'TASK_OFFICE_SUPERSEDED');
});

function citedOfficeEvidence() {
  return {
    state: 'cited' as const,
    semanticGraphUsed: false,
    retrievedAt: '2026-09-24T08:00:00Z',
    citations: [
      { citationId: 'chunk-1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-own', title: '手册', revision: 3, kind: 'fact' as const, retrievedAt: '2026-09-24T08:00:00Z' },
      { citationId: 'chunk-2', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-shared', revision: 5, kind: 'fact' as const, retrievedAt: '2026-09-24T08:00:00Z' },
    ],
    conclusions: [{ kind: 'model_inferred' as const, modelId: 'chat-model-1', citationIds: ['chunk-1', 'chunk-2'] }],
    reasoning: { requested: false, state: 'not_requested' as const, retryable: false },
  };
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/knowledge-qa.test.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 写最小实现**

3a. `packages/mobile-core/src/task-office/knowledge-qa.ts`（新文件，完整内容）：

```ts
/**
 * T15（Issue #45）知识问答域合同：答案证据信封（版本 + 时间 + 三类区分）与
 * Knowledge QA 后端端口。wire（snake_case）→ 本 DTO（camelCase）的映射由
 * api-client 的 remote 完成；mobile-core 不依赖传输层（module-seams §3）。
 */

/** 三类结论/引用类别（CONTEXT.md「证据引用」；与 contracts EvidenceKindWire 一致）。 */
export type EvidenceKindValue = 'fact' | 'rule_derived' | 'model_inferred';
export type AnswerEvidenceStateValue = 'cited' | 'no_evidence' | 'revoked';
export type EvidenceReasoningStateValue = 'not_requested' | 'incomplete';

export interface EvidenceCitation {
  citationId: string;
  knowledgeId: string;
  knowledgeBaseId: string;
  title?: string;
  revision: number;
  startAt?: number;
  endAt?: number;
  quote?: string;
  kind: 'fact';
  retrievedAt: string;
}

export interface EvidenceConclusion {
  kind: EvidenceKindValue;
  modelId?: string;
  ruleIds?: string[];
  citationIds?: string[];
}

export interface EvidenceReasoning {
  requested: boolean;
  mode?: string;
  state: EvidenceReasoningStateValue;
  reason?: string;
  retryable: boolean;
}

export interface AnswerEvidenceView {
  state: AnswerEvidenceStateValue;
  semanticGraphUsed: boolean;
  retrievedAt: string;
  citations: EvidenceCitation[];
  conclusions: EvidenceConclusion[];
  reasoning: EvidenceReasoning;
}

export interface KnowledgeQATurnBody {
  answer: string;
  isFallback: boolean;
  evidence: AnswerEvidenceView;
}

/** 一次完整问答回合：会话身份 + 答案 + 证据。sessionId = taskId（ADR-0004）。 */
export interface KnowledgeQATurn extends KnowledgeQATurnBody {
  sessionId: string;
}

export interface KnowledgeQAAskInput {
  sessionId: string;
  question: string;
  knowledgeBaseIds?: string[];
  signal?: AbortSignal;
}

export interface KnowledgeQABackendPort {
  ask(input: KnowledgeQAAskInput): Promise<KnowledgeQATurnBody>;
}

/** 展示文案：三类必须可区分地呈现，不得把模型推断显示成来源事实。 */
export const EVIDENCE_KIND_LABEL: Record<EvidenceKindValue, string> = {
  fact: '原文事实',
  rule_derived: '规则推导',
  model_inferred: '模型推断',
};

/** 只有「显式请求过推理且未完成」的回合提供重试入口（ADR-0002）。 */
export function evidenceRetryable(evidence: AnswerEvidenceView): boolean {
  return evidence.reasoning.requested && evidence.reasoning.state === 'incomplete' && evidence.reasoning.retryable;
}

/** Scriptable scenario Adapter（module-seams §12）。 */
export interface ScenarioKnowledgeQABackendHandlers {
  ask?: (input: KnowledgeQAAskInput) => Promise<KnowledgeQATurnBody>;
}

export interface ScenarioKnowledgeQABackend extends KnowledgeQABackendPort {
  calls: Array<{ kind: 'ask'; input: KnowledgeQAAskInput }>;
}

export function createScenarioKnowledgeQABackend(handlers: ScenarioKnowledgeQABackendHandlers = {}): ScenarioKnowledgeQABackend {
  const calls: ScenarioKnowledgeQABackend['calls'] = [];
  return {
    calls,
    async ask(input) {
      const normalized: KnowledgeQAAskInput = {
        sessionId: input.sessionId,
        question: input.question,
        ...(input.knowledgeBaseIds === undefined ? {} : { knowledgeBaseIds: [...input.knowledgeBaseIds] }),
        ...(input.signal === undefined ? {} : { signal: input.signal }),
      };
      calls.push({ kind: 'ask', input: normalized });
      if (handlers.ask === undefined) {
        return { answer: '', isFallback: false, evidence: { state: 'no_evidence', semanticGraphUsed: false, retrievedAt: '', citations: [], conclusions: [], reasoning: { requested: false, state: 'not_requested', retryable: false } } };
      }
      return handlers.ask(normalized);
    },
  };
}
```

3b. `packages/mobile-core/src/task-office/task-office-errors.ts`：错误码联合 `'TASK_OFFICE_LEGACY_UNAVAILABLE'` 后追加一员：

```ts
  | 'TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE';
```

3c. `packages/mobile-core/src/task-office/task-office.ts` 四处（全部标记「T15 追加区」）：

import 区（`legacy-tasks.ts` 导入块后）追加：

```ts
import type { KnowledgeQABackendPort, KnowledgeQATurn } from './knowledge-qa.ts';
```

`TaskOfficePorts`（`legacy?: LegacyTaskBackendPort;` 字段后）追加：

```ts
  /** T15（#45）知识问答端口；缺失时 askKnowledge() fail closed（TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE）。 */
  knowledgeQA?: KnowledgeQABackendPort;
```

`TaskOffice` 接口（`followUp(...)` 方法后）追加：

```ts
  /** T15（#45）：一次知识问答回合。无 sessionId 时先创建目标会话（快速问题也是 Task，ADR-0004），
   *  同一 Task 的追问传入原 sessionId（绝不新建 session）。答案携带版本/时间/三类证据。 */
  askKnowledge(input: { question: string; sessionId?: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<KnowledgeQATurn>;
```

`createTaskOffice` 返回对象内（`async followUp(...)` 方法之后）追加实现：

```ts
    /** T15 追加区（#45）：知识问答回合。 */
    async askKnowledge(askInput: { question: string; sessionId?: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<KnowledgeQATurn> {
      const lease = requireLease();
      const question = (askInput?.question ?? '').trim();
      if (question === '' || question.length > 8000) throw new TaskOfficeError('TASK_OFFICE_INVALID_INPUT');
      if (ports.knowledgeQA === undefined) throw new TaskOfficeError('TASK_OFFICE_KNOWLEDGE_QA_UNAVAILABLE');
      let sessionId = (askInput?.sessionId ?? '').trim();
      if (sessionId === '') {
        // 快速知识问题也是 Task（spec 故事 12/24）：一次初始提问创建目标会话。
        const session = await callBackend(() => ports.backend.createSession({ title: question.slice(0, 60) }));
        if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
        sessionId = session.sessionId;
      }
      const knowledgeBaseIds = Array.isArray(askInput.knowledgeBaseIds)
        ? askInput.knowledgeBaseIds.map((id) => id.trim()).filter((id) => id !== '')
        : undefined;
      const turn = await callBackend(() => ports.knowledgeQA!.ask({
        sessionId,
        question,
        ...(knowledgeBaseIds === undefined || knowledgeBaseIds.length === 0 ? {} : { knowledgeBaseIds }),
        ...(askInput.signal === undefined ? {} : { signal: askInput.signal }),
      }));
      if (!leaseActive(lease)) throw new TaskOfficeError('TASK_OFFICE_SCOPE_CHANGED');
      // 新会话/新追问都改变了列表事实：与 start/followUp 同规则作废在途读。
      listEpoch += 1;
      homeEpoch += 1;
      legacyListEpoch += 1;
      accumulated = undefined;
      legacyAccumulated = undefined;
      return { ...turn, sessionId };
    },
```

3d. `packages/mobile-core/src/index.ts` 文件末追加：

```ts
// T15（#45）：知识问答证据域与 Task Office askKnowledge 端口。
export { EVIDENCE_KIND_LABEL, createScenarioKnowledgeQABackend, evidenceRetryable } from './task-office/knowledge-qa.ts';
export type {
  AnswerEvidenceStateValue, AnswerEvidenceView, EvidenceCitation, EvidenceConclusion,
  EvidenceKindValue, EvidenceReasoning, EvidenceReasoningStateValue,
  KnowledgeQAAskInput, KnowledgeQABackendPort, KnowledgeQATurn, KnowledgeQATurnBody,
  ScenarioKnowledgeQABackend, ScenarioKnowledgeQABackendHandlers,
} from './task-office/knowledge-qa.ts';
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/mobile-core/src/task-office/knowledge-qa.test.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/task-office/task-office.test.ts`
Expected: PASS（第三项为既有 office 套件回归）

- [ ] **Step 5: 提交**

```bash
git add packages/mobile-core/src/task-office/knowledge-qa.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/task-office/knowledge-qa.test.ts packages/mobile-core/src/task-office/task-office.ts packages/mobile-core/src/task-office/task-office-errors.ts packages/mobile-core/src/index.ts
git commit -m "feat(task-office): askKnowledge entry with evidence turn and scope-lease fail closed"
```

---

### Task 6: api-client——Knowledge QA Remote Adapter（knowledge-chat SSE）

**Files:**
- Create: `packages/api-client/src/mobile/knowledge-qa.ts`
- Modify: `packages/api-client/package.json`（exports 追加 `"./mobile/knowledge-qa": "./src/mobile/knowledge-qa.ts"`）
- Test: `packages/api-client/src/mobile/knowledge-qa.test.ts`

**Interfaces:**
- Consumes: Task 4 的 `parseAnswerEvidence`/`AnswerEvidenceWire` 及各 wire 子类型（`@weknora/contracts` 根导出）；既有 `createServerSentEventParser`/`parseChatEvent`（`../chat/stream.ts`）、`responseType`（`@weknora/contracts`）、`requireDeploymentOrigin`（`./deployment-origin.ts`）、`ClientRequest`（`../client.ts`）、#44 的 fail-closed 终止帧判定先例（legacy-tasks.ts:106-121——api-client 不依赖 mobile-core，DTO 内联，本任务同例）。
- Produces: `RemoteEvidenceCitation`/`RemoteEvidenceConclusion`/`RemoteEvidenceReasoning`/`RemoteAnswerEvidence`（与 Task 5 mobile-core 同名 DTO 结构逐字一致——结构可赋值由 apps/mobile typecheck 证明，与 `RemoteLegacyTask` 同例）、`RemoteKnowledgeQATurn = { answer: string; isFallback: boolean; evidence: RemoteAnswerEvidence }`、`createMobileKnowledgeQARemote(options: { origin: string; request: (input: ClientRequest) => Promise<unknown>; stream: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void> })` 返回 `{ ask(input: { sessionId: string; question: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<RemoteKnowledgeQATurn> }`；错误字符串 `KNOWLEDGE_QA_FAILED`（error 帧）、`KNOWLEDGE_QA_MALFORMED_FRAME`（畸形 JSON 帧）、`KNOWLEDGE_QA_TRUNCATED`（无终止帧）、`KNOWLEDGE_QA_MISSING_EVIDENCE`（流结束无 evidence 帧——旧服务端/能力缺失 fail closed）。Task 7 消费。

- [ ] **Step 1: 写失败测试**

`packages/api-client/src/mobile/knowledge-qa.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { ClientRequest } from '../client.ts';
import { createMobileKnowledgeQARemote } from './knowledge-qa.ts';

const origin = 'https://weknora.example.test';

function sseFrame(payload: unknown): string {
	return `data: ${JSON.stringify(payload)}\n\n`;
}

function harness(frames: string[]) {
	const calls: ClientRequest[] = [];
	const remote = createMobileKnowledgeQARemote({
		origin,
		request: async () => {
			throw new Error('ask must use the stream transport only');
		},
		stream: async (input, onChunk) => {
			calls.push(input);
			for (const frame of frames) {
				onChunk(frame);
			}
		},
	});
	return { remote, calls };
}

const evidenceFrame = {
	response_type: 'evidence',
	data: {
		evidence: {
			state: 'cited',
			semantic_graph_used: false,
			retrieved_at: '2026-09-24T08:00:00Z',
			citations: [
				{ citation_id: 'chunk-1', knowledge_id: 'doc-1', knowledge_base_id: 'kb-own', title: '手册', revision: 3, kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
				{ citation_id: 'chunk-2', knowledge_id: 'doc-2', knowledge_base_id: 'kb-shared', revision: 5, kind: 'fact', retrieved_at: '2026-09-24T08:00:00Z' },
			],
			conclusions: [{ kind: 'model_inferred', model_id: 'chat-model-1', citation_ids: ['chunk-1', 'chunk-2'] }],
			reasoning: { state: 'not_requested' },
		},
	},
};

test('ask posts to knowledge-chat SSE with the selected knowledge and returns the evidence turn', async () => {
	const { remote, calls } = harness([
		sseFrame(evidenceFrame),
		sseFrame({ response_type: 'answer', content: '依赖', done: false }),
		sseFrame({ response_type: 'answer', content: '关系如下', done: true }),
		sseFrame({ response_type: 'complete' }),
	]);
	const turn = await remote.ask({ sessionId: 'sess 1', question: '依赖关系', knowledgeBaseIds: ['kb-own'] });
	assert.equal(turn.answer, '依赖关系如下');
	assert.equal(turn.isFallback, false);
	assert.equal(turn.evidence.state, 'cited');
	assert.equal(turn.evidence.citations.length, 2);
	assert.equal(turn.evidence.citations[0]!.citationId, 'chunk-1');
	assert.equal(turn.evidence.citations[0]!.revision, 3);
	assert.equal(turn.evidence.conclusions[0]!.kind, 'model_inferred');
	assert.equal(calls.length, 1);
	assert.equal(calls[0]!.method, 'POST');
	assert.equal(calls[0]!.path, '/api/v1/knowledge-chat/sess%201');
	assert.deepEqual(calls[0]!.body, { query: '依赖关系', knowledge_base_ids: ['kb-own'] });
	assert.equal((calls[0]!.headers as Record<string, string>)['accept'], 'text/event-stream');
});

test('ask records fallback answers and no-evidence envelopes', async () => {
	const { remote } = harness([
		sseFrame({ response_type: 'evidence', data: { evidence: { state: 'no_evidence', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { state: 'not_requested' } } } }),
		sseFrame({ response_type: 'answer', content: '抱歉，无法回答。', done: true, data: { is_fallback: true } }),
		sseFrame({ response_type: 'complete' }),
	]);
	const turn = await remote.ask({ sessionId: 's', question: 'q' });
	assert.equal(turn.isFallback, true);
	assert.equal(turn.evidence.state, 'no_evidence');
	assert.equal(turn.evidence.citations.length, 0);
});

test('ask fails closed on error frames, malformed frames, missing evidence and truncated streams', async () => {
	await assert.rejects(
		harness([sseFrame({ response_type: 'error' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_FAILED/,
	);
	await assert.rejects(
		harness(['data: {not json\n\n', sseFrame({ response_type: 'complete' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_MALFORMED_FRAME/,
	);
	await assert.rejects(
		harness([sseFrame({ response_type: 'answer', content: '答', done: true }), sseFrame({ response_type: 'complete' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_MISSING_EVIDENCE/,
		'旧服务端（无 evidence 帧）不得被当作有证据回答',
	);
	await assert.rejects(
		harness([sseFrame(evidenceFrame), sseFrame({ response_type: 'answer', content: '答', done: true })]).remote.ask({ sessionId: 's', question: 'q' }),
		/KNOWLEDGE_QA_TRUNCATED/,
		'无终止帧＝结果未知，不得静默判成功',
	);
	await assert.rejects(
		harness([sseFrame({ response_type: 'evidence', data: { evidence: { state: 'cited', semantic_graph_used: false, retrieved_at: '2026-09-24T08:00:00Z', citations: [], conclusions: [], reasoning: { state: 'not_requested' } } } }), sseFrame({ response_type: 'complete' })]).remote.ask({ sessionId: 's', question: 'q' }),
		/cited answer evidence requires/,
		'不自洽信封整体拒绝',
	);
});

test('constructor validates the deployment origin and ask validates its input', async () => {
	assert.throws(() => createMobileKnowledgeQARemote({ origin: 'http://insecure.example.test', request: async () => undefined, stream: async () => undefined }));
	const { remote } = harness([sseFrame({ response_type: 'complete' })]);
	await assert.rejects(remote.ask({ sessionId: ' ', question: 'q' }), /sessionId/);
	await assert.rejects(remote.ask({ sessionId: 's', question: '' }), /question/);
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/knowledge-qa.test.ts`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 写最小实现**

`packages/api-client/src/mobile/knowledge-qa.ts`（新文件，完整内容）：

```ts
import { parseAnswerEvidence, responseType, type AnswerEvidenceWire, type EvidenceCitationWire, type EvidenceConclusionWire } from '@weknora/contracts';
import { createServerSentEventParser, parseChatEvent } from '../chat/stream.ts';
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

/**
 * T15（Issue #45）：knowledge-chat SSE 的 Knowledge QA Remote。POST
 * /api/v1/knowledge-chat/:session_id，消费 evidence/answer/终止帧；evidence 帧缺失、
 * 流截断或信封不自洽一律 fail closed——不得把无证据回答冒充有据回答（module-seams §5.3）。
 * api-client 不依赖 mobile-core（与 legacy remote 同例）：下列 DTO 与 mobile-core
 * 同名类型结构逐字一致，结构可赋值由 apps/mobile typecheck 证明。
 */

export interface KnowledgeQARemoteOptions {
	/** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
	origin: string;
	/** 授权读通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
	request: (input: ClientRequest) => Promise<unknown>;
	/** 授权 SSE 通道（MobileRuntime.authorizedEventStream 或测试替身）。 */
	stream: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void>;
}

export interface RemoteEvidenceCitation {
	citationId: string;
	knowledgeId: string;
	knowledgeBaseId: string;
	revision: number;
	kind: 'fact';
	retrievedAt: string;
	title?: string;
	quote?: string;
	startAt?: number;
	endAt?: number;
}

export interface RemoteEvidenceConclusion {
	kind: 'fact' | 'rule_derived' | 'model_inferred';
	modelId?: string;
	ruleIds?: string[];
	citationIds?: string[];
}

export interface RemoteEvidenceReasoning {
	requested: boolean;
	mode?: string;
	state: 'not_requested' | 'incomplete';
	reason?: string;
	retryable: boolean;
}

export interface RemoteAnswerEvidence {
	state: 'cited' | 'no_evidence' | 'revoked';
	semanticGraphUsed: boolean;
	retrievedAt: string;
	citations: RemoteEvidenceCitation[];
	conclusions: RemoteEvidenceConclusion[];
	reasoning: RemoteEvidenceReasoning;
}

export interface RemoteKnowledgeQATurn {
	answer: string;
	isFallback: boolean;
	evidence: RemoteAnswerEvidence;
}

function citationOf(citation: EvidenceCitationWire): RemoteEvidenceCitation {
	return {
		citationId: citation.citation_id,
		knowledgeId: citation.knowledge_id,
		knowledgeBaseId: citation.knowledge_base_id,
		revision: citation.revision,
		kind: 'fact',
		retrievedAt: citation.retrieved_at,
		...(citation.title === undefined ? {} : { title: citation.title }),
		...(citation.quote === undefined ? {} : { quote: citation.quote }),
		...(citation.start_at === undefined ? {} : { startAt: citation.start_at }),
		...(citation.end_at === undefined ? {} : { endAt: citation.end_at }),
	};
}

function conclusionOf(conclusion: EvidenceConclusionWire): RemoteEvidenceConclusion {
	return {
		kind: conclusion.kind,
		...(conclusion.model_id === undefined ? {} : { modelId: conclusion.model_id }),
		...(conclusion.rule_ids === undefined ? {} : { ruleIds: [...conclusion.rule_ids] }),
		...(conclusion.citation_ids === undefined ? {} : { citationIds: [...conclusion.citation_ids] }),
	};
}

function evidenceOf(evidence: AnswerEvidenceWire): RemoteAnswerEvidence {
	return {
		state: evidence.state,
		semanticGraphUsed: evidence.semantic_graph_used,
		retrievedAt: evidence.retrieved_at,
		citations: evidence.citations.map(citationOf),
		conclusions: evidence.conclusions.map(conclusionOf),
		reasoning: {
			requested: evidence.reasoning.requested,
			state: evidence.reasoning.state,
			retryable: evidence.reasoning.retryable,
			...(evidence.reasoning.mode === undefined ? {} : { mode: evidence.reasoning.mode }),
			...(evidence.reasoning.reason === undefined ? {} : { reason: evidence.reasoning.reason }),
		},
	};
}

export function createMobileKnowledgeQARemote(options: KnowledgeQARemoteOptions) {
	requireDeploymentOrigin(options.origin);
	const stream = options.stream;
	return {
		async ask(input: { sessionId: string; question: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<RemoteKnowledgeQATurn> {
			const sessionId = input.sessionId.trim();
			const question = input.question.trim();
			if (sessionId === '') throw new Error('knowledge QA requires sessionId');
			if (question === '') throw new Error('knowledge QA requires a question');
			const knowledgeBaseIds = Array.isArray(input.knowledgeBaseIds)
				? input.knowledgeBaseIds.map((id) => id.trim()).filter((id) => id !== '')
				: [];
			let answer = '';
			let isFallback = false;
			let evidence: RemoteAnswerEvidence | undefined;
			let failed = false;
			let terminated = false;
			let frames = 0;
			const parser = createServerSentEventParser((frame) => {
				frames += 1;
				let event;
				try {
					event = parseChatEvent(frame);
				} catch {
					throw new Error('KNOWLEDGE_QA_MALFORMED_FRAME');
				}
				const type = responseType(event);
				if (type === 'error') {
					failed = true;
					return;
				}
				if (type === 'evidence') {
					// evidence 帧畸形/不自洽时 parseAnswerEvidence 整体拒绝（不部分渲染）。
					const data = (event.data ?? {}) as { evidence?: unknown };
					evidence = evidenceOf(parseAnswerEvidence(data.evidence));
					return;
				}
				if (type === 'answer') {
					if (typeof event.content === 'string') answer += event.content;
					const data = (event.data ?? {}) as { is_fallback?: unknown };
					if (data.is_fallback === true) isFallback = true;
					return;
				}
				if (type === 'complete' || type === 'stop') terminated = true;
			});
			await stream({
				method: 'POST',
				path: `/api/v1/knowledge-chat/${encodeURIComponent(sessionId)}`,
				headers: { accept: 'text/event-stream', 'content-type': 'application/json' },
				body: { query: question, ...(knowledgeBaseIds.length === 0 ? {} : { knowledge_base_ids: knowledgeBaseIds }) },
				...(input.signal === undefined ? {} : { signal: input.signal }),
			}, (chunk) => parser.push(chunk));
			parser.finish();
			if (failed) throw new Error('KNOWLEDGE_QA_FAILED');
			// Fail-closed 终止帧判定（与 #44 legacy followUp 同规则）：服务端正常完成必发
			// complete/stop；流结束无终止帧＝结果未知，不得静默判成功。
			if (!terminated) throw new Error(`KNOWLEDGE_QA_TRUNCATED: knowledge QA for session ${sessionId} ended without a terminal frame after ${frames} frame(s)`);
			// 能力 fail closed：无 evidence 帧的服务端（T15 之前）不能提供版本/时间/三类
			// 证据，本回合不得被当作有证据回答交付。
			if (evidence === undefined) throw new Error(`KNOWLEDGE_QA_MISSING_EVIDENCE: session ${sessionId} completed without an evidence frame`);
			return { answer, isFallback, evidence };
		},
	};
}
```

`packages/api-client/package.json` 的 `exports`（`"./mobile/legacy-tasks"` 条目后）追加：

```json
    "./mobile/knowledge-qa": "./src/mobile/knowledge-qa.ts",
```


- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm exec tsx --test packages/api-client/src/mobile/knowledge-qa.test.ts && pnpm exec tsx --test packages/api-client/src/mobile/legacy-tasks.test.ts`
Expected: PASS（第二项为同目录既有套件回归）

- [ ] **Step 5: 提交**

```bash
git add packages/api-client/src/mobile/knowledge-qa.ts packages/api-client/src/mobile/knowledge-qa.test.ts packages/api-client/package.json
git commit -m "feat(api-client): knowledge QA remote over knowledge-chat SSE with fail-closed evidence contract"
```

---

### Task 7: apps/mobile——知识问答视图、Screen、路由与接线

**Files:**
- Create: `apps/mobile/src/knowledge-qa-view.ts`
- Create: `apps/mobile/src/screens/KnowledgeQAScreen.tsx`
- Create: `apps/mobile/src/app/ask.tsx`
- Modify: `apps/mobile/src/composition.ts`（import 一行 + `taskOfficeFor` 内追加 `knowledgeQA` 装配）
- Modify: `apps/mobile/src/screens/HomeScreen.tsx`（一个入口按钮）
- Modify: `apps/mobile/src/app-smoke.test.tsx`（一个源级断言测试）
- Test: `apps/mobile/src/knowledge-qa-view.test.ts`

**Interfaces:**
- Consumes: Task 5 的 `TaskOffice.askKnowledge`/`KnowledgeQATurn`/`AnswerEvidenceView`/`EVIDENCE_KIND_LABEL`/`evidenceRetryable`；Task 6 的 `createMobileKnowledgeQARemote`；既有 `KnowledgeResource`（`@weknora/domain/mobile` resource-presentation.ts:14）、`activeTaskOffice()`/`activeMobileRuntime()`/`taskOfficeFor`（composition.ts:153,256,317）、Resource Shelf `browse().knowledge` 消费形态（app/new.tsx:26-30）、`/new` 路由生命周期宿主形态（app/new.tsx）。
- Produces: `createKnowledgeQAController(ports)`（`KnowledgeQAController`：`state()/subscribe/update({question})/toggleKnowledge(id)/refreshKnowledge/ask/retry/dispose`）、`KnowledgeQAViewState`、`EVIDENCE_NO_EVIDENCE_COPY`/`EVIDENCE_REVOKED_COPY`/`EVIDENCE_REASONING_INCOMPLETE_COPY`、`evidenceCitationLine(citation): string`、`<KnowledgeQAScreen state onUpdate onToggleKnowledge onAsk onRetry onRefreshKnowledge />`、`/ask` 路由、composition 的 `knowledgeQA` 装配。Task 8 消费控制器形态。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/knowledge-qa-view.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import type { KnowledgeResource } from '@weknora/domain/mobile';
import type { KnowledgeQATurn, TaskOffice } from '@weknora/mobile-core';
import {
  EVIDENCE_NO_EVIDENCE_COPY, EVIDENCE_REASONING_INCOMPLETE_COPY, EVIDENCE_REVOKED_COPY,
  createKnowledgeQAController, evidenceCitationLine,
} from './knowledge-qa-view.ts';

function turn(overrides: Partial<KnowledgeQATurn['evidence']> = {}): KnowledgeQATurn {
  return {
    sessionId: 'sess-1',
    answer: 'A 间接依赖 C',
    isFallback: false,
    evidence: {
      state: 'cited',
      semanticGraphUsed: false,
      retrievedAt: '2026-09-24T08:00:00Z',
      citations: [
        { citationId: 'chunk-1', knowledgeId: 'doc-1', knowledgeBaseId: 'kb-own', title: '手册', revision: 3, kind: 'fact', retrievedAt: '2026-09-24T08:00:00Z' },
        { citationId: 'chunk-2', knowledgeId: 'doc-2', knowledgeBaseId: 'kb-shared', title: '指南', revision: 5, kind: 'fact', retrievedAt: '2026-09-24T08:00:01Z' },
      ],
      conclusions: [{ kind: 'model_inferred', modelId: 'chat-model-1', citationIds: ['chunk-1', 'chunk-2'] }],
      reasoning: { requested: false, state: 'not_requested', retryable: false },
      ...overrides,
    },
  };
}

function knowledge(): KnowledgeResource[] {
  return [
    { id: 'kb-own', title: '手册', scanStatus: 'indexed', documentCount: 3, updatedAt: '2026-09-01T00:00:00Z' },
    { id: 'kb-shared', title: '指南', scanStatus: 'indexed', documentCount: 1, updatedAt: '2026-09-01T00:00:00Z' },
  ];
}

function controllerWith(scripted: KnowledgeQATurn[] = [turn()], asked: Array<{ question: string; knowledgeBaseIds?: string[] }> = []) {
  let call = 0;
  const office = {
    askKnowledge: async (input: { question: string; knowledgeBaseIds?: string[] }) => {
      asked.push({ question: input.question, ...(input.knowledgeBaseIds === undefined ? {} : { knowledgeBaseIds: [...input.knowledgeBaseIds] }) });
      const next = scripted[Math.min(call, scripted.length - 1)]!;
      call += 1;
      return next;
    },
  } as unknown as Pick<TaskOffice, 'askKnowledge'>;
  return createKnowledgeQAController({ office, knowledge: async () => knowledge() });
}

test('ask passes the selected knowledge scope and renders the evidence turn', async () => {
  const asked: Array<{ question: string; knowledgeBaseIds?: string[] }> = [];
  const controller = controllerWith([turn()], asked);
  await controller.whenInitialized();
  controller.update({ question: '  依赖关系是什么  ' });
  controller.toggleKnowledge('kb-own');
  controller.toggleKnowledge('kb-shared');
  controller.toggleKnowledge('kb-shared'); // 再点一次取消选择
  const result = await controller.ask();
  assert.deepEqual(asked, [{ question: '依赖关系是什么', knowledgeBaseIds: ['kb-own'] }]);
  assert.equal(result!.sessionId, 'sess-1');
  const state = controller.state();
  assert.equal(state.phase, 'answered');
  assert.equal(state.turn!.evidence.citations.length, 2);
  assert.equal(state.question, '', '成功后清空输入，防重复提交');
  assert.deepEqual(state.selectedKnowledgeIds, ['kb-own']);
});

test('citation lines show source, version, time and kind distinctly', () => {
  const line = evidenceCitationLine(turn().evidence.citations[0]!);
  assert.ok(line.includes('手册'));
  assert.ok(line.includes('v3'));
  assert.ok(line.includes('2026-09-24T08:00:00Z'));
  assert.ok(line.includes('原文事实'));
});

test('no-evidence, revoked and reasoning-incomplete states get honest copy and retry only when retryable', async () => {
  const noEvidence = controllerWith([turn({ state: 'no_evidence', citations: [], conclusions: [] })]);
  await noEvidence.whenInitialized();
  await noEvidence.ask();
  assert.equal(noEvidence.state().statusLine, EVIDENCE_NO_EVIDENCE_COPY);

  const revoked = controllerWith([turn({ state: 'revoked', citations: [], conclusions: [] })]);
  await revoked.whenInitialized();
  await revoked.ask();
  assert.equal(revoked.state().statusLine, EVIDENCE_REVOKED_COPY);

  const incomplete = controllerWith([turn({
    state: 'no_evidence', citations: [], conclusions: [],
    reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'semantic_reasoning_unavailable', retryable: true },
  })]);
  await incomplete.whenInitialized();
  await incomplete.ask();
  assert.equal(incomplete.state().statusLine, EVIDENCE_REASONING_INCOMPLETE_COPY);
  assert.equal(incomplete.state().retryAvailable, true);
  const asked2: Array<{ question: string }> = [];
  const retried = controllerWith([
    turn({ state: 'no_evidence', citations: [], conclusions: [], reasoning: { requested: true, mode: 'rules', state: 'incomplete', reason: 'x', retryable: true } }),
    turn(),
  ], asked2 as Array<{ question: string }>);
  await retried.whenInitialized();
  await retried.ask();
  await retried.retry();
  assert.equal(retried.state().phase, 'answered');
  assert.equal(asked2.length, 2, 'retry re-asks the same question');
});

test('retry is unavailable for non-retryable turns; ask failure keeps the question and surfaces the error', async () => {
  const cited = controllerWith([turn()]);
  await cited.whenInitialized();
  await cited.ask();
  assert.equal(cited.state().retryAvailable, false);
  const failing = createKnowledgeQAController({
    office: {
      askKnowledge: async () => {
        throw new Error('KNOWLEDGE_QA_MISSING_EVIDENCE');
      },
    } as unknown as Pick<TaskOffice, 'askKnowledge'>,
  });
  await failing.whenInitialized();
  failing.update({ question: '保留我' });
  await failing.ask();
  const state = failing.state();
  assert.equal(state.phase, 'failed');
  assert.equal(state.question, '保留我', '失败不清空输入（可修改后重提）');
  assert.ok(state.error!.includes('KNOWLEDGE_QA_MISSING_EVIDENCE'));
});
```

`apps/mobile/src/app-smoke.test.tsx` 追加一个测试（与既有源级断言测试同模式：动态 `import('node:fs')`/`node:path`/`node:url` 取 `here` 后读源文本；放在「the task detail error chain maps codes to copy」类源断言测试旁）：

```tsx
test('composition wires the knowledge QA remote into Task Office and /ask consumes the office only (T15)', async () => {
  const { readFileSync } = await import('node:fs');
  const { dirname, join } = await import('node:path');
  const { fileURLToPath } = await import('node:url');
  const here = dirname(fileURLToPath(import.meta.url));
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  const askRoute = readFileSync(join(here, 'app/ask.tsx'), 'utf8');
  const home = readFileSync(join(here, 'screens/HomeScreen.tsx'), 'utf8');
  assert.match(composition, /knowledgeQA:\s*createMobileKnowledgeQARemote\(/, 'Task Office 必须装配 knowledgeQA 端口');
  assert.match(composition, /import \{ createMobileKnowledgeQARemote \} from '@weknora\/api-client\/mobile\/knowledge-qa';/);
  assert.match(askRoute, /activeTaskOffice\(\)/, '/ask 只经组合根取 Task Office，不直连 api-client');
  assert.doesNotMatch(askRoute, /@weknora\/api-client/, 'Screen/路由禁止直连 wire 客户端（module-seams §10）');
  assert.match(home, /Ask knowledge/, '授权首页必须有知识问纳入口');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL（`src/knowledge-qa-view.test.ts` 导入不存在的模块而失败；app-smoke 新断言因 `src/app/ask.tsx` 不存在同样失败；既有其余用例保持绿）

- [ ] **Step 3: 写最小实现**

3a. `apps/mobile/src/knowledge-qa-view.ts`（新文件，完整内容）：

```ts
import type { KnowledgeResource } from '@weknora/domain/mobile';
import { EVIDENCE_KIND_LABEL, evidenceRetryable, type EvidenceCitation, type KnowledgeQATurn, type TaskOffice } from '@weknora/mobile-core';

/** T15（#45）知识问答屏控制器：Screen 只见状态与意图（module-seams §5.2/§10）。 */

export interface KnowledgeQAControllerPorts {
  office: Pick<TaskOffice, 'askKnowledge'>;
  /** 可检索知识（Resource Shelf browse 投影）；缺省空列表（检索范围是可选输入）。 */
  knowledge?(): Promise<readonly KnowledgeResource[]>;
}

export type KnowledgeQAPhase = 'loading' | 'idle' | 'asking' | 'answered' | 'failed';

export interface KnowledgeQAViewState {
  phase: KnowledgeQAPhase;
  question: string;
  knowledge: readonly KnowledgeResource[];
  selectedKnowledgeIds: readonly string[];
  asking: boolean;
  turn?: KnowledgeQATurn;
  statusLine?: string;
  retryAvailable: boolean;
  error?: string;
}

export const EVIDENCE_NO_EVIDENCE_COPY = '知识库中没有支持回答本问题的证据。';
export const EVIDENCE_REVOKED_COPY = '本回答所依据的知识访问已被撤销，结果已作废。';
export const EVIDENCE_REASONING_INCOMPLETE_COPY = '推理未完成：当前部署尚未接入语义推理服务。';

/** 证据行展示：来源 · 版本 · 检索时间 · 类别——四个可核对维度都必须可见。 */
export function evidenceCitationLine(citation: EvidenceCitation): string {
  const title = citation.title === undefined || citation.title === '' ? citation.knowledgeId : citation.title;
  return `${title} · v${citation.revision} · ${citation.retrievedAt} · ${EVIDENCE_KIND_LABEL[citation.kind]}`;
}

export interface KnowledgeQAController {
  state(): KnowledgeQAViewState;
  subscribe(listener: (state: KnowledgeQAViewState) => void): () => void;
  update(patch: { question?: string }): void;
  toggleKnowledge(knowledgeId: string): void;
  refreshKnowledge(): Promise<void>;
  ask(): Promise<KnowledgeQATurn | undefined>;
  retry(): Promise<KnowledgeQATurn | undefined>;
  whenInitialized(): Promise<void>;
  dispose(): void;
}

function statusLineOf(turn: KnowledgeQATurn): string | undefined {
  if (turn.evidence.reasoning.requested && turn.evidence.reasoning.state === 'incomplete') return EVIDENCE_REASONING_INCOMPLETE_COPY;
  if (turn.evidence.state === 'no_evidence') return EVIDENCE_NO_EVIDENCE_COPY;
  if (turn.evidence.state === 'revoked') return EVIDENCE_REVOKED_COPY;
  return undefined;
}

export function createKnowledgeQAController(ports: KnowledgeQAControllerPorts): KnowledgeQAController {
  let state: KnowledgeQAViewState = {
    phase: 'loading',
    question: '',
    knowledge: [],
    selectedKnowledgeIds: [],
    asking: false,
    retryAvailable: false,
  };
  let lastQuestion = '';
  let disposed = false;
  const listeners = new Set<(state: KnowledgeQAViewState) => void>();
  const publish = (next: KnowledgeQAViewState): void => {
    state = next;
    if (!disposed) for (const listener of [...listeners]) listener(state);
  };
  const initialized = (async (): Promise<void> => {
    try {
      const knowledge = ports.knowledge === undefined ? [] : await ports.knowledge();
      if (disposed) return;
      publish({ ...state, knowledge, phase: 'idle' });
    } catch {
      if (!disposed) publish({ ...state, phase: 'idle' }); // 知识列表失败不阻塞提问（范围是可选输入）
    }
  })();
  const runAsk = async (): Promise<KnowledgeQATurn | undefined> => {
    const question = state.question.trim();
    if (question === '') {
      publish({ ...state, error: '请输入问题。' });
      return undefined;
    }
    lastQuestion = question;
    publish({ ...state, asking: true, error: undefined });
    try {
      const selected = state.selectedKnowledgeIds;
      const turn = await ports.office.askKnowledge({
        question,
        ...(selected.length === 0 ? {} : { knowledgeBaseIds: [...selected] }),
      });
      publish({
        ...state,
        phase: 'answered',
        question: '',
        asking: false,
        turn,
        statusLine: statusLineOf(turn),
        retryAvailable: evidenceRetryable(turn.evidence),
      });
      return turn;
    } catch (error) {
      publish({ ...state, phase: 'failed', asking: false, error: error instanceof Error ? error.message : String(error) });
      return undefined;
    }
  };
  return {
    state: () => state,
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    update(patch) {
      publish({ ...state, ...(patch.question === undefined ? {} : { question: patch.question }) });
    },
    toggleKnowledge(knowledgeId) {
      const selected = state.selectedKnowledgeIds.includes(knowledgeId)
        ? state.selectedKnowledgeIds.filter((id) => id !== knowledgeId)
        : [...state.selectedKnowledgeIds, knowledgeId];
      publish({ ...state, selectedKnowledgeIds: selected });
    },
    async refreshKnowledge() {
      if (ports.knowledge === undefined) return;
      try {
        publish({ ...state, knowledge: await ports.knowledge() });
      } catch { /* 维持现状 */ }
    },
    ask: runAsk,
    async retry() {
      if (!state.retryAvailable) return undefined;
      publish({ ...state, question: lastQuestion });
      return runAsk();
    },
    whenInitialized: () => initialized,
    dispose() {
      disposed = true;
      listeners.clear();
    },
  };
}
```

3b. `apps/mobile/src/screens/KnowledgeQAScreen.tsx`（新文件，完整内容）：

```tsx
import { Button, ScrollView, Text, TextInput, View } from 'react-native';
import { EVIDENCE_KIND_LABEL, type EvidenceCitation, type KnowledgeQATurn } from '@weknora/mobile-core';
import type { KnowledgeResource } from '@weknora/domain/mobile';
import { evidenceCitationLine, type KnowledgeQAViewState } from '../knowledge-qa-view.ts';

/** T15：知识问答屏（presentation Adapter——只消费控制器状态与意图回调）。 */
export function KnowledgeQAScreen(props: {
  state: KnowledgeQAViewState;
  onUpdate(patch: { question?: string }): void;
  onToggleKnowledge(knowledgeId: string): void;
  onAsk(): void;
  onRetry(): void;
  onRefreshKnowledge(): void;
}) {
  const { state } = props;
  return (
    <ScrollView>
      <Text>知识问答</Text>
      <TextInput
        value={state.question}
        placeholder="提出知识问题（答案将携带可核对的来源、版本与时间证据）"
        multiline
        onChangeText={(text) => { props.onUpdate({ question: text }); }}
      />
      <Text>检索范围（不选＝会话默认可发现知识）</Text>
      {state.knowledge.map((resource: KnowledgeResource) => (
        <Button
          key={resource.id}
          title={`${state.selectedKnowledgeIds.includes(resource.id) ? '✓ ' : ''}${resource.title}`}
          disabled={state.asking}
          onPress={() => { props.onToggleKnowledge(resource.id); }}
        />
      ))}
      <Button title="刷新知识列表" disabled={state.asking} onPress={() => { props.onRefreshKnowledge(); }} />
      <Button title="提问" disabled={state.asking || state.phase === 'loading' || state.question.trim() === ''} onPress={() => { props.onAsk(); }} />
      {state.error !== undefined && <Text>{state.error}</Text>}
      {state.turn !== undefined && <TurnView turn={state.turn} />}
      {state.statusLine !== undefined && <Text>{state.statusLine}</Text>}
      {state.retryAvailable && <Button title="重试" disabled={state.asking} onPress={() => { props.onRetry(); }} />}
    </ScrollView>
  );
}

function TurnView({ turn }: { turn: KnowledgeQATurn }) {
  return (
    <View>
      <Text>{`问：${turn.sessionId}`}</Text>
      <Text>{turn.answer === '' ? '（无回答内容）' : turn.answer}</Text>
      <Text>证据</Text>
      {turn.evidence.citations.map((citation: EvidenceCitation) => (
        <Text key={citation.citationId}>{evidenceCitationLine(citation)}</Text>
      ))}
      <Text>结论类别</Text>
      {turn.evidence.conclusions.map((conclusion, index) => (
        <Text key={index}>{`${EVIDENCE_KIND_LABEL[conclusion.kind]}${conclusion.modelId === undefined ? '' : ` · ${conclusion.modelId}`}`}</Text>
      ))}
      {!turn.evidence.semanticGraphUsed && <Text>未使用语义图谱（本地检索）</Text>}
    </View>
  );
}
```

3c. `apps/mobile/src/app/ask.tsx`（新文件，完整内容；生命周期宿主形态与 app/new.tsx 一致）：

```tsx
import { useEffect, useState } from 'react';
import { Text, View } from 'react-native';
import type { KnowledgeResource } from '@weknora/domain/mobile';
import { activeMobileRuntime, activeTaskOffice } from '../composition.ts';
import { createKnowledgeQAController, type KnowledgeQAController, type KnowledgeQAViewState } from '../knowledge-qa-view.ts';
import { KnowledgeQAScreen } from '../screens/KnowledgeQAScreen.tsx';

/** /ask 的挂载生命周期宿主：controller 在 effect 内创建，卸载时 dispose。 */
export function KnowledgeQARouteLifecycle({ office }: { office: NonNullable<ReturnType<typeof activeTaskOffice>> }) {
  const runtime = activeMobileRuntime();
  const [state, setState] = useState<KnowledgeQAViewState | undefined>(undefined);
  const [controller, setController] = useState<KnowledgeQAController | undefined>(undefined);
  useEffect(() => {
    let disposed = false;
    let createdController: KnowledgeQAController | undefined;
    void (async () => {
      const created = createKnowledgeQAController({
        office,
        knowledge: async () => {
          const handle = runtime.resourceShelf();
          if (!handle) return [];
          return (await handle.browse()).knowledge as KnowledgeResource[];
        },
      });
      if (disposed) { created.dispose(); return; }
      createdController = created;
      setController(created);
      setState(created.state());
    })();
    return () => {
      disposed = true;
      createdController?.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- office/runtime 是 app 生命周期单例
  }, [office, runtime]);
  useEffect(() => (controller === undefined ? undefined : controller.subscribe(setState)), [controller]);
  if (controller === undefined || state === undefined) {
    return (
      <View>
        <Text>Loading</Text>
      </View>
    );
  }
  return (
    <KnowledgeQAScreen
      state={state}
      onUpdate={(patch) => { controller.update(patch); }}
      onToggleKnowledge={(knowledgeId) => { controller.toggleKnowledge(knowledgeId); }}
      onAsk={() => { void controller.ask(); }}
      onRetry={() => { void controller.retry(); }}
      onRefreshKnowledge={() => { void controller.refreshKnowledge(); }}
    />
  );
}

/** Expo Router 文件路由：/ask（快速知识问答——答案即 Task，自动进入历史）。只消费 Task Office 与 Resource Shelf Interface。 */
export default function KnowledgeQARoute() {
  const office = activeTaskOffice();
  if (!office) {
    return (
      <View>
        <Text>Sign in to ask a knowledge question.</Text>
      </View>
    );
  }
  return <KnowledgeQARouteLifecycle office={office} />;
}
```

3d. `apps/mobile/src/composition.ts` 两处：
import 区（`createMobileLegacyTaskRemote` 导入行后）追加：

```ts
import { createMobileKnowledgeQARemote } from '@weknora/api-client/mobile/knowledge-qa';
```

`taskOfficeFor` 的 `createTaskOffice({...})` 调用内（`legacy: createMobileLegacyTaskRemote({...}),` 之后）追加：

```ts
      // T15（#45）：知识问答端口——同一授权读/流通道（不新建传输），evidence 帧缺失 fail closed。
      knowledgeQA: createMobileKnowledgeQARemote({
        origin,
        request: (input) => activeRuntime.authorizedRequest(input),
        stream: (input, onChunk) => activeRuntime.authorizedEventStream(input, onChunk),
      }),
```

3e. `apps/mobile/src/screens/HomeScreen.tsx`：`<Button title="New task" ...>` 行后追加一行：

```tsx
      <Button title="Ask knowledge" onPress={() => router.push('/ask')} />
```

（`Button`/`router` 均为该文件既有导入，无新 import。）

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（含 app-smoke 新断言与全部既有套件回归）

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/knowledge-qa-view.ts apps/mobile/src/knowledge-qa-view.test.ts apps/mobile/src/screens/KnowledgeQAScreen.tsx apps/mobile/src/app/ask.tsx apps/mobile/src/composition.ts apps/mobile/src/screens/HomeScreen.tsx apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): /ask knowledge QA screen with evidence panel and retry entry"
```

---

### Task 8: apps/mobile——真实 HTTP 集成证据（AC3 移动面，opt-in）

**Files:**
- Create: `apps/mobile/src/knowledge-qa-integration-smoke.ts`
- Test: `apps/mobile/src/knowledge-qa-integration-smoke.test.ts`

**Interfaces:**
- Consumes: Task 5/6/7 的全部产出（`createTaskOffice` + `knowledgeQA` 端口 + `askKnowledge`）；既有集成冒烟形态：`taskOfficeIntegrationConfig(env)`（task-office-integration-smoke.ts:22 的 opt-in `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD` 校验——本任务同型自包含实现，含 credential-free HTTPS origin 校验）、`createMobileRuntime` + `createInMemoryCredentialStore` + `runtime.signIn` + `runtime.authorizedRequest` + `runtime.authorizedEventStream`（runtime-integration-smoke.ts:52-90 装配先例；无 `authorizedStream` port 时 `authorizedEventStream` 抛 `RUNTIME_UNAUTHORIZED`——本冒烟必须自带流通道：按 runtime-integration-smoke.ts 的 `authorizedTransport` 模式构造 `(input, accessToken)` 请求器并直接复用 Task 6 remote 的 `stream` 通道——见实现）。
- Produces: `KnowledgeQAIntegrationConfig`、`knowledgeQAIntegrationConfig(env)`、`KnowledgeQAIntegrationEvidence`、`runKnowledgeQAIntegration(config)`。无代码消费方——这是证据契约（对齐 #34/#35/#41/#44/#46 的集成证据先例）。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/knowledge-qa-integration-smoke.test.ts`（新文件，完整内容）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { knowledgeQAIntegrationConfig } from './knowledge-qa-integration-smoke.ts';

test('integration config stays opt-in and validates the credential-free HTTPS origin', () => {
  assert.deepEqual(knowledgeQAIntegrationConfig({}), {
    enabled: false,
    disposition: 'skip',
    reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD',
  });
  assert.equal(knowledgeQAIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://cloud.example.test', WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'p' }).enabled, true);
  const invalid = knowledgeQAIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'http://cloud.example.test', WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(invalid.enabled, false);
  assert.equal(invalid.disposition, 'invalid');
  const embedded = knowledgeQAIntegrationConfig({ WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://user:pass@cloud.example.test', WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c', WEKNORA_MOBILE_TEST_PASSWORD: 'p' });
  assert.equal(embedded.enabled, false);
  assert.equal(embedded.disposition, 'invalid');
});

test('runKnowledgeQAIntegration is exported and only runs against an opted-in real deployment', async (t) => {
  const { knowledgeQAIntegrationConfig: configOf, runKnowledgeQAIntegration } = await import('./knowledge-qa-integration-smoke.ts');
  assert.equal(typeof runKnowledgeQAIntegration, 'function');
  const config = configOf(process.env);
  if (!config.enabled) {
    t.skip(config.reason); // 无凭据即 skip——不得伪造通过
    return;
  }
  const evidence = await runKnowledgeQAIntegration(config);
  assert.equal(evidence.asked, 'answered');
  assert.notEqual(evidence.evidenceState, 'absent', '真实部署必须返回 evidence 帧（否则服务端早于 T15）');
  assert.equal(evidence.retrievedAtParsed, true, '每条引用的检索时间必须可解析');
});
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm --filter @weknora/mobile test`
Expected: FAIL（`src/knowledge-qa-integration-smoke.test.ts` 导入不存在的模块而失败；既有其余用例保持绿）

- [ ] **Step 3: 写最小实现**

`apps/mobile/src/knowledge-qa-integration-smoke.ts`（新文件，完整内容）：

```ts
import { createWeKnoraClient } from '@weknora/api-client';
import { createMobileKnowledgeQARemote } from '@weknora/api-client/mobile/knowledge-qa';
import { createMobileRuntimeRemote } from '@weknora/api-client/mobile/runtime';
import { createTaskOfficeRemote } from '@weknora/api-client/mobile/task-office';
import { createJsonTransport, type FetchLike } from '@weknora/api-client/transport';
import { CLIENT_PROTOCOL_VERSION } from '@weknora/domain/mobile';
import { createInMemoryCredentialStore, createMobileRuntime, createTaskOffice } from '@weknora/mobile-core';

/**
 * T15（Issue #45）AC3 移动面集成证据：真实部署 + 真实授权通道 + 真实
 * knowledge-chat SSE + Task Office 编排。opt-in（WEKNORA_MOBILE_TEST_*），无凭据
 * 即 skip——不得伪造通过。撤权/无证据两场景的服务端权威证据在 Go 侧
 *（knowledge_evidence_test.go，真实 sqlite + 真实 share 行翻转）；本冒烟验证
 * 移动面闭环：提问 → evidence 帧在场 → 引用携带版本与时间 → 探针任务可归档。
 */

export type KnowledgeQAIntegrationConfig =
  | { enabled: true; deploymentOrigin: string; email: string; password: string; question: string }
  | { enabled: false; disposition: 'skip' | 'invalid'; reason: string };

export interface KnowledgeQAIntegrationEvidence {
  deploymentOrigin: string;
  asked: 'answered' | 'failed';
  evidenceState: 'cited' | 'no_evidence' | 'revoked' | 'absent';
  citations: number;
  revisions: string[];
  retrievedAtParsed: boolean;
  semanticGraphUsed: boolean | 'absent';
  reasoningState: string | 'absent';
  archived: 'archived' | 'archive-failed' | 'skipped';
  commandTimestamp: string;
}

/** 与 T01 mobileRuntimeIntegrationConfig 相同的 opt-in 语义（自包含，不跨计划 import）。 */
export function knowledgeQAIntegrationConfig(env: Record<string, string | undefined>): KnowledgeQAIntegrationConfig {
  const deploymentOrigin = env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL?.trim();
  const email = env.WEKNORA_MOBILE_TEST_EMAIL?.trim();
  const password = env.WEKNORA_MOBILE_TEST_PASSWORD;
  if (!deploymentOrigin || !email || !password) {
    return { enabled: false, disposition: 'skip', reason: 'missing WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD' };
  }
  let parsed: URL;
  try { parsed = new URL(deploymentOrigin); } catch {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL is not an absolute URL' };
  }
  if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.pathname !== '/' || parsed.search || parsed.hash) {
    return { enabled: false, disposition: 'invalid', reason: 'WEKNORA_MOBILE_TEST_DEPLOYMENT_URL must be a credential-free HTTPS origin' };
  }
  const question = env.WEKNORA_MOBILE_TEST_KNOWLEDGE_QUESTION?.trim() || '用一句话介绍本部署知识库中最相关的文档。';
  return { enabled: true, deploymentOrigin: parsed.origin, email, password, question };
}

export async function runKnowledgeQAIntegration(config: Extract<KnowledgeQAIntegrationConfig, { enabled: true }>): Promise<KnowledgeQAIntegrationEvidence> {
  const evidence: KnowledgeQAIntegrationEvidence = {
    deploymentOrigin: config.deploymentOrigin,
    asked: 'failed',
    evidenceState: 'absent',
    citations: 0,
    revisions: [],
    retrievedAtParsed: false,
    semanticGraphUsed: 'absent',
    reasoningState: 'absent',
    archived: 'skipped',
    commandTimestamp: new Date().toISOString(),
  };
  const fetcher: FetchLike = (input, init) => fetch(input, init as RequestInit);
  const runtime = createMobileRuntime({
    credentialStore: createInMemoryCredentialStore(),
    clientVersion: CLIENT_PROTOCOL_VERSION,
    remoteFor(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return createMobileRuntimeRemote({ origin, request: client.request });
    },
    authorizedTransport(origin) {
      const client = createWeKnoraClient({ baseURL: origin, transport: createJsonTransport(fetcher) });
      return (input, accessToken) => client.request({ ...input, headers: { ...input.headers, authorization: `Bearer ${accessToken}` } });
    },
  });
  const snapshot = await runtime.signIn({
    deployment: { origin: config.deploymentOrigin, label: 'Integration deployment' },
    email: config.email,
    password: config.password,
  });
  if (snapshot.surface !== 'authorized') return evidence;

  const office = createTaskOffice({
    backend: createTaskOfficeRemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
    }),
    knowledgeQA: createMobileKnowledgeQARemote({
      origin: config.deploymentOrigin,
      request: (input) => runtime.authorizedRequest(input),
      stream: (input, onChunk) => runtime.authorizedEventStream(input, onChunk),
    }),
    lease: () => runtime.scopeLease(),
  });

  try {
    const turn = await office.askKnowledge({ question: config.question });
    evidence.asked = 'answered';
    evidence.evidenceState = turn.evidence.state;
    evidence.citations = turn.evidence.citations.length;
    evidence.revisions = turn.evidence.citations.map((citation) => `v${citation.revision}`);
    evidence.retrievedAtParsed = turn.evidence.citations.every((citation) => !Number.isNaN(Date.parse(citation.retrievedAt)));
    evidence.semanticGraphUsed = turn.evidence.semanticGraphUsed;
    evidence.reasoningState = turn.evidence.reasoning.state;
    try {
      await office.archive(turn.sessionId);
      evidence.archived = 'archived';
    } catch {
      evidence.archived = 'archive-failed';
    }
  } catch {
    evidence.asked = 'failed'; // 如实记录：不伪造 evidenceState
  }
  return evidence;
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: PASS（无凭据环境：集成用例 skip；其余全绿。具备 `WEKNORA_MOBILE_TEST_*` 环境时自动产出真实证据——`asked='answered'` 且 `evidenceState != 'absent'`。）

- [ ] **Step 5: 提交**

```bash
git add apps/mobile/src/knowledge-qa-integration-smoke.ts apps/mobile/src/knowledge-qa-integration-smoke.test.ts
git commit -m "test(mobile): opt-in real-deployment knowledge QA integration evidence"
```

---

## 计划级验证

在 worktree 根（`.worktrees/issue30-sweep`）执行（覆盖本计划全部定向测试；避免全量 flaky 套件）：

```bash
go test ./internal/types/ -run 'TestConclusionFrom|TestCitationsFromSearchResults|TestValidateAnswerEvidence' -count=1 && \
go test ./internal/handler/session/ -run 'TestEvidenceEventStreams|TestParseQARequestRejectsUnknownReasoningMode' -count=1 && \
go test ./internal/application/service/ -run 'TestKnowledgeQAEvidence|TestKnowledgeQAReasoning|TestEmitKnowledgeReferencesEvent' -count=1 && \
go build ./internal/... ./cmd/... && \
pnpm exec tsx --test packages/contracts/test/mobile-knowledge-evidence.test.ts && \
pnpm exec tsx --test packages/mobile-core/src/task-office/knowledge-qa.test.ts packages/mobile-core/src/task-office/knowledge-qa-office.test.ts packages/mobile-core/src/task-office/task-office.test.ts && \
pnpm exec tsx --test packages/api-client/src/mobile/knowledge-qa.test.ts && \
pnpm --filter @weknora/mobile test && \
pnpm --filter @weknora/mobile typecheck
```

（作者基线实跑：`go test ./internal/application/service/ -run 'TestEmitKnowledgeReferencesEventIgnoresCitationOutputSetting' -count=1` ok、`pnpm exec tsx --test packages/mobile-core/src/task-office/task-office.test.ts` 全绿（pass 8 / fail 0）——两项均为本会话在当前 HEAD 亲手执行。）

## Spec 覆盖对照（自审第 1 项）

| Spec/AC 条目 | 落点 |
|---|---|
| AC1「事实、规则推导和模型推断明确区分」 | Task 1（`EvidenceKind` 三类 + `ConclusionFromSemanticReason` 真实 proto wire 分类 + `ValidateAnswerEvidence` 混标拒绝）；Task 3（cited 信封：引用恒 fact、本地答案结论恒 model_inferred 带模型标识）；Task 4/5（解析器与 `EVIDENCE_KIND_LABEL` 三类可区分呈现）；Task 2（wire 键集锁定） |
| AC2「撤权…端到端验证」 | Task 3 `TestKnowledgeQAEvidenceRevocationBeforeDeliveryDropsSource`（真实 `RemoveShare` 行翻转→交付前丢弃）+ `TestKnowledgeQAEvidenceAllSourcesRevokedVoidsAnswer`（全失权作废）+ `TestKnowledgeQAEvidenceDropsUnattributableRows`（不可归属同形丢弃） |
| AC2「无证据…端到端验证」 | Task 3 `TestKnowledgeQAEvidenceNoEvidenceIsExplicit`（显式 no_evidence 信封、零引用零结论）；Task 6 fallback + no_evidence 解析测试；Task 7 `EVIDENCE_NO_EVIDENCE_COPY` 呈现 |
| AC2「多知识源…端到端验证」 | Task 3 `TestKnowledgeQAEvidenceMultiKnowledgeSourceAndKinds`（同租户 + 跨租户共享两源并存，逐引用 revision/retrieved_at）；Task 4/6 多源信封解析 |
| AC3「端到端行为通过最高稳定 Interface 验证」 | Go 面：Task 2（真实 EventBus→StreamManager→`buildStreamResponse` 的 wire 字节）+ Task 3（真实 sqlite 行 + 真实 KBShareService + 真实事件总线的服务级集成）；移动面：Task 5（Task Office Interface 场景测试）+ Task 6（真实 SSE 帧字节契约）+ Task 8（opt-in 真实部署全链路，blocked-env 声明）；明确不以单测/静态检查冒充（检索引擎为 remote-owned，按 Spec 以确定性夹具代入并声明） |
| ADR-0002「普通问答…显式注明未使用语义图谱」 | Task 3（`SemanticGraphUsed=false`）+ Task 2 键集断言 `"semantic_graph_used":false` + Task 7 屏显「未使用语义图谱」 |
| ADR-0002「明确请求推理但服务不可用…显示推理未完成和重试入口」 | Task 3 `TestKnowledgeQAReasoningRequestShortCircuitsIncomplete` + `TestParseQARequestRejectsUnknownReasoningMode`；Task 5 `evidenceRetryable`；Task 7 重试按钮 |
| ADR-0002「删除或撤权立即阻止失效证据…交付前重校验…作废或重算」 | Task 3 `applyEvidenceRevocation`（重校验 + 从 prompt 上下文同步移除＝重算；全失权＝作废） |
| CONTEXT.md「证据引用…包含来源和获取时间」 | Task 1 逐引用 `retrieved_at`/`revision`；Task 6/7 逐行呈现 `来源 · v{revision} · 时间 · 类别` |
| Spec「quick knowledge questions…auto-file into recent history」 | Task 5（`askKnowledge` 无 sessionId 先创建会话＝创建 Task）+ Task 8（探针任务可归档；#44 legacy 投影承接历史呈现） |
| module-seams §5.2/§10（Screen 不直连 wire） | Task 7（Screen 只消费控制器状态；contracts/api-client 仅出现在 composition 与 remote 层） |
| §5.3「Scope Lease 失效后丢弃迟到结果」「capability 缺失 fail closed」 | Task 5（迟到 `TASK_OFFICE_SCOPE_CHANGED` 测试）+ Task 6（`KNOWLEDGE_QA_MISSING_EVIDENCE` fail closed） |

## 边界声明（不做什么）

- **语义推理（Semantica）执行接入不在本计划**：`interfaces.SemanticClient.Search/Reason` 在 QA 链路的启用态调用、`SemanticScopeService.Issue` 的 per-KB 授权铸造与 `Resolve` 交付重校验，属 ADR-0002 试点 rollout（默认关闭、「本轮设计确认不等于实现或能力验收」）。本计划交付该 rollout 需要的三类分类函数（真实 proto wire 形态，Task 1）与不可用时的诚实语义（Task 3）。
- **不改 `SearchResult`/既有 `references` 帧**：版本与时间证据走新增 `evidence` 信封帧；旧引用形状（存储 Scan、既有前端）零改动，两帧并存。
- **同租户成员资格的 mid-flight 失效不在交付前重校验范围内**：请求 ctx 的成员资格由 JWT 中间件在请求入口校验；交付前重校验覆盖 KB 删除与跨租户 org share/grant 的实时行（`KBPermissions.Check` 读实时行）。同租户成员被移除的 mid-flight 场景由下一请求的中间件拒绝（新请求禁止使用），与 ADR 一致。
- **`act(TaskIntent)`/start(goal)/跨进程加密持久化**：分属 #37/#36/#40，本计划不触碰。
- **推理请求的移动端 UI 出口不在本计划**：`reasoning_mode` 是服务端 API 能力（web/API 消费方可用）；移动端 `/ask` 走普通检索问答，推理未完成的重试入口由 evidence 帧状态驱动（服务端短路时同样走该帧）。
- **`Task Grant` 运行面消费（`ResolveTaskAccess`/`TaskRoleCanRun` 接运行入口）**：属 #36/#37 声明的后续消费；本计划的知识获准范围走 QA 检索既有授权路径 + 交付前重校验。
- **「Tenant 允许发现」的独立策略开关不存在于当前代码，本计划不发明**：移动端可发现知识呈现复用 #33 Resource Shelf 的 Tenant 授权投影（`browse().knowledge`）；服务端强制＝请求时 KB 授权（`KBPermissions.Check`）+ 交付前重校验。若未来引入独立的 Tenant 级发现策略位，属新需求而非本计划静默扩展。
