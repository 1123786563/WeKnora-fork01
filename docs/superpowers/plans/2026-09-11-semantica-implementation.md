# Semantica GraphRAG 与推理总实施计划 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付与 docreader 平级部署的语义知识服务，实现文档索引、授权 GraphRAG 检索、证据推理及可恢复的更新删除流程。

**Architecture:** Go 管理文档、授权、业务任务与回答交付；Python 服务管理语义操作、图谱、索引发布和查询计算。跨服务采用独立版本化 gRPC 契约，持久图先筛成授权子图再进入 Semantica 算法。

**Tech Stack:** 仓库 Go 1.26、Gin、GORM、Asynq、PostgreSQL/SQLite；Python、gRPC、Semantica、服务专属 PostgreSQL schema、独立 Neo4j；React 19、TypeScript 6、pnpm 10.28.2。Python/Semantica 精确版本由 V01 冻结。

**Spec:** [完整架构规格](../specs/2026-09-11-semantica-graphrag-reasoning-design.md)。所有执行者必须同时阅读规格、总计划与当前子计划。

## Global Constraints

- 已确认：独立 Python 服务、gRPC、Go 适配层；语义服务承担构图、GraphRAG 与推理；Go 保留业务资源及授权管理。
- 所有记录显式携带 tenant_id、kb_id。
- 处理顺序必须是授权事实过滤 → 有界图扩展 → 推理或摘要。
- 禁止把 PolicyEngine 等同于 WeKnora ACL，把自然语言解释等同于规则证明，把官方功能说明等同于中文质量或性能结论。
- Neo4j 内部 ID 不作为对外永久 ID。
- 首版只流式展示进度，证据和答案在最终授权检查之后交付。
- 首版每 KB 串行发布；查询强制按固定 generation 过滤。
- 来源事实表示“文档声称”，不表示客观真理；冲突断言并存，缺少事实不推导其否定。
- 不接管 docreader 解析、Go 标准分块、原有向量/全文索引、主 Agent 循环或会话记忆。不接入任意查询语言、跨租户合并及全库共享摘要。
- 所有业务后台、API、Agent 和缓存路径使用同一授权边界；不能为了性能绕过来源过滤。
- `2 跳、500 节点、1000 边、top_k 10；Search 10 秒、Reason 30 秒` 是可配置实验初值，不是性能承诺。
- 保护现有 React、商业能力与其他并行工作，只提交当前任务拥有的文件；不使用 `git add .`。新迁移号执行时重新核对，不覆盖他人迁移。
- 本计划只授权规划；实施、依赖安装、容器启动和真实模型调用由后续执行指令启动。示例代码是测试断言与关键实现片段，不声称当前可运行或已经通过。

---

## 1. 编写基线与状态

编写位置：`/Users/wuyongjun/trea/WeKnora-fork01`；静态基线 `5b670b23cad2c886c4c9aaf017536bf564eceb32`。开始时已有未提交的语义规格与 CONTEXT.md，保留这些文件。本轮未改业务代码、未安装依赖、未运行应用或创建 GitHub Issue。

已检查 docreader 协议与 Go 适配、图抽取任务、图仓库/类型、实体搜索、PG/SQLite 迁移、React contracts/domain/api-client/Web 入口。旧 Vue 文件仍存在，新功能落在 React 工作区。实施前重新核对基线和文件归属；不能把其他计划的拟议 API 当成现成接口。

## 2. 七个子计划、24 个任务

| 子计划 | 任务 | 可独立验收的产出 |
|---|---|---|
| [00 能力验证](2026-09-11-semantica-00-verification.md) | V01–V03 | 固定版本、可复现实验、能力矩阵和评估基线 |
| [01 协议与服务](2026-09-11-semantica-01-service.md) | C01–C03 | 双语契约、可认证服务、原文证据模型 |
| [02 索引生命周期](2026-09-11-semantica-02-indexing.md) | I01–I05 | 操作租约、业务 outbox、版本发布、删除、业务任务协调 |
| [03 权限与模型](2026-09-11-semantica-03-access-model.md) | A01–A03 | 可信访问范围、事实级过滤、受控模型与预算 |
| [04 检索与推理](2026-09-11-semantica-04-query.md) | Q01–Q04 | 授权 GraphRAG、规则推导、模型推断、Go 交付 |
| [05 产品集成](2026-09-11-semantica-05-product.md) | W01–W03 | API、React 状态/引用、后端切换与回滚 |
| [06 部署验收](2026-09-11-semantica-06-rollout.md) | O01–O03 | 部署、故障恢复、质量与发布门槛 |

任务总数：3+3+5+3+4+3+3=24。一个任务是独立 review/commit 单元；各任务内部的勾选步骤按 RED → 实现 → GREEN → 留证 → scoped commit 执行。

## 3. 依赖与执行顺序

| 任务 | 直接依赖 | 可阻断的下游 |
|---|---|---|
| V01 | 无 | 全部外部能力相关任务 |
| V02 | V01 | 授权子图与推理实现 |
| V03 | V02 | 正式模式及质量门槛 |
| C01 | V01 | 所有内部 DTO/RPC 使用者 |
| C02 | C01 | RPC 调用及在线操作 |
| C03 | C01,V02 | 事实与引用模型 |
| I01 | C02 | 持久操作与发布 |
| I02 | C01 | 业务 revision/outbox/scope epoch |
| A01 | C02,I02 | 授权范围签发和复查 |
| A03 | C02,I01,A01 | 模型抽取和模型推断 |
| I03 | C03,I01,I02,A03 | 可查询 generation |
| I04 | I03,A01 | 删除、恢复、回滚 |
| I05 | I04 | 业务文档闭环 |
| A02 | A01,I03,I04 | 图检索和推理 |
| Q01 | A02,V03 | GraphRAG 查询 |
| Q02 | Q01 | 规则推理 |
| Q03 | Q01,A03,V03 | 模型推断 |
| Q04 | Q02,Q03,I05 | Go 检索融合与 Agent 工具 |
| W01 | Q04,I05 | 用户 API/共享契约 |
| W02 | W01 | React 可用流程 |
| W03 | W01,I04,Q04 | 后端切换与回滚 |
| O01 | C02,I03,A03 | 可部署环境 |
| O02 | O01,I04,W03,Q04 | 恢复演练 |
| O03 | O02,W02,V03 | 上线证据 |

推荐线性顺序：V01 → V02 → V03 → C01 → C02 → C03 → I01 → I02 → A01 → A03 → I03 → I04 → I05 → A02 → Q01 → Q02 → Q03 → Q04 → W01 → W02 → W03 → O01 → O02 → O03。能力未通过只阻断相应能力，不能用测试跳过伪装通过。

## 4. 文件职责与共享契约

| 目录/文件 | 唯一职责与首次拥有任务 |
|---|---|
| `semantic/experiments/` | V01–V03 上游验证，不包含业务生产逻辑 |
| `semantic/proto/semantic.proto` | C01 唯一 wire contract，Go/Python 生成物同源 |
| `internal/types/semantic.go` | C01 Go 领域 DTO，tenant 使用 uint64，其他 ID 为 string |
| `semantic/semantic_service/contracts.py` | C01 对应 Python DTO，snake_case 与 wire 一致 |
| `semantic/semantic_service/server.py`、`auth.py` | C02 RPC 生命周期、内部服务认证 |
| `semantic/semantic_service/evidence.py`、`facts.py` | C03 来源模型、原文校验、稳定 UUID |
| `semantic/semantic_service/operations.py` | I01 操作幂等、租约、fencing |
| `internal/application/repository/semantic_outbox.go` | I02 Go 事务内事件和版本 |
| `semantic/semantic_service/indexing/` | I03/I04 generation 与删除 |
| `internal/application/service/semantic_tasks.go` | I05 业务任务协调 |
| `internal/application/service/semantic_scope.go` | A01 权限解析、scope 签发及交付前复查 |
| `semantic/semantic_service/access.py` | A02 授权事实与缓存隔离 |
| `internal/application/service/semantic_model.go` | A03 模型准入、预算和原始用量 |
| `semantic/semantic_service/query/` | Q01–Q03 检索与两种推理 |
| `internal/application/service/semantic_query.go` | Q04 查询编排与最终交付 |
| `internal/handler/semantic.go` | W01 用户 API，不能绕过领域授权 |
| `packages/*/src/semantic.ts`、`apps/web/src/semantic/` | W01/W02 客户端契约、状态与 UI |
| `internal/application/service/semantic_backend.go` | W03 后端迁移与回滚 |
| `docker/compose.semantic.test.yml`、`.github/workflows/semantic.yml` | O01/O03 环境与门禁 |

### 4.1 C01 必须一次定义的领域字段

以下名称是计划新增契约，不是上游 Semantica API；后续任务只能使用这里的拼写。ID 不含数据库内部标识。

```text
ScopeKey {tenant_id:uint64, kb_id:string}
DocumentRevision {scope:ScopeKey, document_id:string, revision:uint64,
  content_hash:string, deleted:bool}
ChunkSnapshot {chunk_id:string, text:string, content_hash:string}
IndexConfig {config_digest:string, engine_version:string, model_profile_ref:string,
  prompt_version:string, rule_set_version:string, schema_version:string}
ApplyRequest {document:DocumentRevision, chunks:ChunkSnapshot[], manifest_ref:string?,
  config:IndexConfig, idempotency_key:string, payload_hash:string}
Operation {operation_id:string, scope:ScopeKey, document_id:string, revision:uint64,
  state:string, stage:string, lease_token:uint64, result_generation:string?, error_code:string?}
Evidence {evidence_id:string, document_id:string, revision:uint64, chunk_id:string,
  content_hash:string, quote:string, start_char:uint32?, end_char:uint32?}
Assertion {assertion_id:string, scope:ScopeKey, subject_id:string, predicate:string,
  object_id:string?, value:string?, kind:source|rule|model,
  evidence_ids:string[], premise_ids:string[], valid_from:string?, valid_until:string?}
AccessScope {scope:ScopeKey, subject_id:string, scope_ref:string,
  scope_hash:string, permission_epoch:uint64, expires_at:string,
  audience:string, purpose:search|reason|index, budget_ref:string}
QueryLimits {max_hops:uint32, max_nodes:uint32, max_edges:uint32,
  top_k:uint32, max_tokens:uint32, deadline_ms:uint32}
SearchRequest {query_id:string, query:string, access_scope:AccessScope,
  limits:QueryLimits, mode:string}
SearchResponse {query_id:string, generation:string, mode:string, evidence:Evidence[],
  assertion_ids:string[], paths:string[][], stale:bool, partial:bool, truncated:bool}
ReasonRequest {search:SearchRequest, reasoning_mode:rules|model, rule_set_version:string}
ReasonResponse {retrieval:SearchResponse, status:string, conclusion:string,
  conclusion_kind:rule|model, premise_ids:string[], rule_ids:string[],
  model_version:string?, prompt_version:string?, limitations:string[]}
```

C01 定义 RPC `GetCapabilities/ApplyDocumentRevision/DeleteDocument/GetOperation/CancelOperation/Search/Reason` 的 request/response envelope，并保留 protobuf tag，禁止重新编号。Delete 使用 DocumentRevision 且 deleted=true；Get/Cancel 使用 ScopeKey+operation_id。能力字段必须说明可用性、原因和限制。签名令牌放 metadata；AccessScope 中的字段必须与已验证令牌一致。

Go DTO统一添加Semantic前缀：Python `ScopeKey/ApplyRequest/Operation/AccessScope/SearchRequest/SearchResponse/ReasonRequest/ReasonResponse` 对应 Go `types.SemanticScopeKey/types.SemanticApplyRequest/types.SemanticOperation/types.SemanticAccessScope/types.SemanticSearchRequest/types.SemanticSearchResponse/types.SemanticReasonRequest/types.SemanticReasonResponse`。其他DTO同样使用Semantic前缀；TS公共类型另由W01显式映射。Python DTO使用标准库 `@dataclass(frozen=True)`，代码中的 `replace` 指 `dataclasses.replace`，可变集合在构造时复制，不依赖Pydantic模型替换语义。

IndexManifest在I03定义scope、generation、base_generation、documents（document_id到revision/content_hash/不可变artifact IDs的完整映射）、config_digest、artifacts（kind/id/hash/location）、complete；ReadLease包含lease_id、generation、manifest、expires_at。manifest包含查询用到的全部派生依赖，不能只列本次更新的文档。

C01建立 `semantic/pyproject.toml`、生产包和pytest开发依赖，测试包名固定 `semantic_service`；C02扩展为RPC服务。`semantic/experiments` 为V01独立环境，不导入未实现的生产代码。Python测试文件显式导入当前任务定义的生产对象；fixture扩展统一在conftest.py，首次出现的fixture必须同任务实现。

### 4.2 测试与夹具约定

Python 从仓库根运行 `uv run --project semantic python -m pytest semantic/tests/... -q`。Go 使用 `go test ./确切包 -run '确切测试前缀' -count=1`；竞态任务增加 `-race`。共享 TS 使用 `pnpm exec tsx --test 确切测试文件`。这些都是实施期命令，本轮不执行。

持久化集成测试必须连接隔离环境，缺环境时显式失败；不得 Skip 后计为通过。C02 建立 `semantic/tests/conftest.py` 的 `rpc_client`，I01 扩展 `operation_store`，I03 扩展 `index_store`，A02 扩展 `access_graph`；均为真实适配器/受控 fixture，不是在内存里模拟生产事务。纯算法测试可用冻结时钟和内存输入。

证据 fixture：同一租户 KB 中 D1“甲公司控股乙公司”、D2“乙公司控股丙公司”、D3“甲公司不再控股乙公司”；另有受限 D4“丙公司的密钥代号是松柏”。期望：D1+D2 在注册控股传递规则下可推导甲→丙；无 D2 时不得推导；D4 不可见时任何答案、别名、路径及摘要不得暴露“松柏”。另一租户使用相同名称，必须完全隔离。业务规则仅作为测试规则，不默认适用于真实商业关系。

每个任务执行时先读取其测试片段和接口定义，补齐同文件 imports/fixtures 后运行 RED；RED 必须证明目标行为缺失，不能把无关环境错误算 RED。实现代码块给出关键不变量，工程师必须完成所列流程，不以片段代替完整实现。

### 4.3 状态与证据台账

[执行台账](semantica/progress.md) 是任务状态唯一入口。状态 pending/in_progress/blocked/implemented/verified；只有真实命令、退出码、运行环境、提交 SHA 和产物路径齐备才能 verified。blocked 写明具体依赖和下一步。代码已写/单测通过不等于真实模型或生产验收通过。

每个任务提交示例只包含该任务新增或明确修改文件。共享入口有他人改动时使用 hunk 暂存，不整文件覆盖。spec 未跟踪时执行者也须阅读，不得把它误删为临时文件。

## 5. 规格覆盖矩阵

| 规格章节 | 实施任务 |
|---|---|
| 1 目标/范围、2 能力依据 | V01–V03、W01–W03 |
| 3 所有权与部署 | C01–C02、I01、I03、O01 |
| 4 来源/实体/事实 | C03、I03、I04、A02 |
| 5 权限 | A01–A02、Q01–Q04、O02 |
| 6 operation/outbox/generation/删除 | I01–I05、W03、O02 |
| 7 RPC/限制/错误 | C01–C02、Q01–Q04 |
| 8 模型/预算/观测 | A03、O01、O03 |
| 9 Go 与用户流程 | I05、Q04、W01–W03 |
| 10 验收 | V03、各任务测试、O02–O03 |
| 11 备选和失败条件 | V03、W03、O03 |

## 6. 执行入口与完成标准

从 V01 开始；没有测试过的上游签名必须在 V01/V02 留证后才能进入生产适配。不为绕过失败修改验收断言。采用 Subagent-Driven 时每任务一个实现者、规格 review 和质量 review；普通串行执行同样保持每任务 review 与提交。当前编写过程没有启动子代理。

全部24任务 verified、跨语言合约通过、故障恢复通过且质量/成本阈值已有明确验收记录，才能声明完整交付。没有真实模型凭据时允许完成其余独立工作，但 V02/V03 的模型项与 Q03/O03 的真实模型验收必须保持未通过。没有用户确认的上线阈值时不得启用正式流量。不得默认部署、切换真实 KB 或发布外部服务。
