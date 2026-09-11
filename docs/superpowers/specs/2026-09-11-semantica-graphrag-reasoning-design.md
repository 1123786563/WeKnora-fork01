# WeKnora Semantica 语义知识服务架构规格

日期：2026-09-11。状态：用户已确认独立服务方向，并明确 Semantica 同时承担 GraphRAG 检索和推理；本文为细化规格，具体协议、存储与首版限制待评审。尚未实施或运行验收。

## 1. 目标与决策边界

用户上传文档后，由 docreader 解析、Go 保存标准正文和分块，语义服务异步构建带来源的知识图谱。用户或 Agent 提问时，经 Go 授权进入 GraphRAG 检索和推理，获得有原文引用的回答。更新、撤权、删除和失败均有可解释状态。

已确认：独立 Python 服务、gRPC、Go 适配层；语义服务承担构图、GraphRAG 与推理；Go 保留业务资源及授权管理。本文建议语义服务独占写自己的持久化图及衍生索引，Go 不再写其图数据。

首版包含单知识库内跨文档实体关联、多跳证据检索、注册规则推导及模型辅助推断。首版不包含跨租户合并、跨知识库统一图、任意 Cypher/SPARQL 执行、自动规则发布、全库社区报告、接管 Agent 会话记忆或主执行循环。分阶段交付不取消最终检索与推理范围。

## 2. 核查依据与能力限制

本地静态基线：`5b670b23cad2c886c4c9aaf017536bf564eceb32`。开始时工作区干净。

| 本地证据 | 支持的判断 |
|---|---|
| `docreader/proto/docreader.proto`、`internal/infrastructure/docparser/grpc_parser.go` | 可借鉴 RPC 与业务类型隔离、服务通信方式 |
| `internal/application/service/knowledge_post_process.go`、`extract.go` | 已有图抽取异步任务、attempt、子任务进度和取消相关机制 |
| `internal/types/extract_graph.go` | 当前关系类型缺少事实来源与版本；NameSpace 无显式 tenant 字段，不能直接充当新授权契约 |
| `internal/application/repository/retriever/neo4j/repository.go` | 当前节点合并包含文档标识；不能把旧数据直接当成知识库级实体模型 |
| `internal/application/service/chat_pipeline/search_entity.go` | 当前实体搜索通过图仓库接口执行，需要增加语义查询适配路径 |

外部资料查阅日期：2026-09-11。仅官方文档静态核查，不代表源码、版本兼容或生产能力验收。

- [Context 模块](https://docs.getsemantica.ai/reference/context/)：描述多跳检索、ContextRetriever；ContextGraph 是内存图，不能默认作为持久化数据库查询门面。
- [Reasoning 模块](https://docs.getsemantica.ai/reference/reasoning/)：区分规则 Reasoner 与 LLM GraphReasoner，可作为两类推理的适配候选。
- [Graph Store 模块](https://docs.getsemantica.ai/reference/graph_store/)：提供 Neo4j 适配，但文档列出部分图分析方法尚未实现，持久化与内存算法之间需要桥接验证。
- [抽取示例](https://docs.getsemantica.ai/quickstart/)：可输入文本执行实体关系抽取。
- [GraphRAG 数据流](https://microsoft.github.io/graphrag/index/default_dataflow/)：借鉴文本单元、来源关联和图增强的阶段划分；不照搬其完整索引流水线。

禁止把 PolicyEngine 等同于 WeKnora ACL，把自然语言解释等同于规则证明，把官方功能说明等同于中文质量或性能结论。具体 Semantica 版本须在能力验证阶段冻结。

## 3. 所有权与部署

| 数据或控制 | 唯一权威 |
|---|---|
| tenant、KB、文档、标准 chunk、文档 revision、ACL | Go 业务系统 |
| 上传解析、业务任务、业务删除状态、后端选择 | Go 业务系统 |
| 实体、事实、来源投影、语义索引 generation、operation | 语义服务 |
| GraphRAG 与推理执行 | 语义服务；输入由 Go 授权 |
| 模型凭据、授权、预算和原始用量接入 | Go 模型调用入口；语义服务通过内部适配调用 |
| 最终会话回答与对用户交付 | Go 会话系统 |

推荐部署 `semantic-api` 与 `semantic-worker`，使用同一镜像不同入口。首版可单副本，但 operation 必须持久化，不能依赖 API 进程内任务。服务内调度使用持久 operation 表和租约领取，Go Asynq 负责提交、查询及协调业务任务，不重复执行 Python 内部步骤。

推荐语义服务独立 PostgreSQL schema/账号保存操作、发布清单和控制元数据；独立 Neo4j 实例保存语义图，避免依赖现有 Neo4j 版本的多数据库能力。衍生向量使用隔离 collection，正文快照与构建产物使用对象存储隔离前缀。允许复用基础设施，但不共享业务写权限。数据库产品和资源规格属于待评审实施建议。

Neo4j 内部 ID 不作为对外永久 ID。语义派生数据可从标准分块、版本化配置和注册规则重建；模型重新执行不保证结果完全相同，回滚依赖保留的产物而非重新生成。

## 4. 领域数据与来源

所有记录显式携带 tenant_id、kb_id。实体身份首版限同一 tenant/KB；跨空间共享知识按资源所属空间存储，通过授权访问，不复制为请求者的空间事实。

| 对象 | 核心字段与约束 |
|---|---|
| DocumentRevision | document_id、单调递增 revision、内容哈希、删除标识；由 Go 产生 |
| ChunkSnapshot | chunk_id、revision、text、hash；采用 Go 已落库的标准分块 |
| Entity | 服务生成的稳定 UUID、类型；名称和别名是有来源的断言，不能只按名称生成全局 ID |
| Assertion | subject、predicate、object/value、可选有效时间；区分来源事实、规则推导、模型推断 |
| Evidence | document_id、revision、chunk_id、原文 quote、可选 span、content_hash |
| Derivation | conclusion、premise assertion IDs、rule_id/version 或 model/prompt version |
| IndexManifest | generation、完整文档 revision 清单、配置/模型/规则/引擎版本、产物位置与哈希 |

span 采用 chunk 原文的 Unicode code point 半开区间，Go 按 rune 解释；提取前若规范化文本须保留映射。无法准确定位允许 span 为空，但 quote 必须核对原文。禁止虚构位置。

实体属性、别名和等价关系均保留独立支持来源。自动合并候选经版本化规则决定，记录可撤销的等价断言；不销毁原始身份。查询不能通过不可见文档支持的别名或合并关系找到隐藏关联。

多个来源支持一个事实时，删除其中一个仅撤销其支持。所有支持消失后事实退出有效视图，依赖它的推导、摘要和缓存同时失效。来源事实表示“文档声称”，不表示客观真理；冲突断言并存，缺少事实不推导其否定。

## 5. 授权模型

Go 从现有权限解析生成短期 AccessScope：subject、owner tenant、KB、允许文档范围或不可变 scope_ref、permission_epoch、有效期、请求用途、预算上限。scope_ref 由语义服务通过受认证内部接口解析；签名/受众与服务身份共同验证，前端不能签发。

处理顺序必须是授权事实过滤 → 有界图扩展 → 推理或摘要。过滤应用于向量候选、别名、等价断言、每条路径边、来源和派生属性，不能只过滤最终引用。

首版从持久化存储构建有界的授权子图，再适配 ContextRetriever/Reasoner/GraphReasoner；不把全库 ContextGraph 加载后事后过滤。若上游算法无法保留过滤语义或需要全图数据，该模式返回 unsupported，不能扩大范围。

首版全库共享实体摘要与社区摘要不参与查询；所需摘要仅基于本次授权事实计算。缓存键至少包含 owner tenant、KB、generation、scope hash、permission epoch、模型/规则/配置版本及查询参数。

索引控制缓存不是 ACL 权威。查询开始验证 scope，长查询检查过期与撤销；Go 在交付前重新读取当前 permission epoch 和删除屏障。若发生变化，整份结果作废并按新 scope 重算，不能仅移除引文后继续交付已受污染的答案。首版只流式展示进度，证据和答案在最终授权检查之后交付；以该检查为请求授权线性化点。权限服务不可用时拒绝语义查询。

例：A 文档可见、B 文档不可见，B 才证明两家公司存在投资关系。即使公司实体合并，查询也不能用 B 的边、别名、摘要或推导作答。

## 6. 版本发布、事件与删除

Go 在业务变更事务中写入 semantic outbox；任务至少一次投递，服务端按 tenant/KB/document/revision/config digest 去重。同一键不同 payload hash 返回冲突。超时重试返回已有 operation；不声称跨模型调用天然 exactly-once。

operation 状态：accepted → running → staged → publishing → succeeded；允许 failed、cancelled、superseded。running 通过租约与 fencing token 防止失联 worker 提交；每个阶段可恢复。取消终态前释放租约并拒绝旧 worker 发布。已成功发布的 operation 不能通过 Cancel 回滚。

generation 是知识库级不可变清单，包含完整文档版本集合；可复用未变更文档的不可变产物。首版每 KB 串行发布，构建时记录 base_generation。所有图、来源、向量和必需衍生产物校验通过后，持久元数据库以 CAS 原子切换 active_generation。图数据库与元数据库没有分布式事务：先写不可见产物，最后发布指针，查询强制按固定 generation 过滤。CAS 失败重建清单或置 superseded，不覆盖并发更新。

查询在开始时固定 generation 并持有读取租约；GC 等待租约结束且保留期满足，清理孤儿 staging 产物。进程重启从持久 active 指针恢复，绝不把 staging 当 active。

普通更新允许继续使用上一已发布版本，响应标识文档版本与 stale 状态。敏感替换可指定立即隐藏旧版本。Go 的删除事务立即写入拒绝屏障并提高授权版本，新 scope 排除该文档；屏障应用于任何历史 generation 和回滚版本。服务不可达不影响业务侧阻止使用，物理清理保持 pending 并持续重试。

删除事件携带单调 revision/tombstone；晚到旧上传和旧 operation 发布必须失败。物理清理范围包括事实支持、等价断言、推导、摘要、向量、对象产物和缓存；完成后返回分存储 receipt。短期墓碑清理必须晚于消息重放窗口，不能让过期事件复活资源。备份按独立保留策略清理，恢复备份必须先重放删除屏障再提供查询。

## 7. RPC 契约草案

协议包 `weknora.semantic.v1`；领域 DTO 不暴露上游 Python 对象。所有方法传播 request/trace ID、截止时间及服务身份。

| RPC | 输入与语义 |
|---|---|
| GetCapabilities | 返回协议/引擎版本、可用模式、限制和 unavailable_reason |
| ApplyDocumentRevision | 文档 revision、chunk manifest、配置快照、幂等键；返回持久 operation |
| DeleteDocument | 文档 revision/tombstone；幂等返回清理 operation |
| GetOperation / CancelOperation | 校验 operation 所属 scope；返回阶段、错误、进度和终态 |
| Search | query、AccessScope、模式、top_k、hop/node/token 上限；返回证据及检索路径 |
| Reason | query/goal、AccessScope、规则或模型模式、允许规则集、预算；返回结论与支持 DAG |

大文档用带内容哈希和大小限制的 manifest 引用，服务只读批准的存储前缀，不接受任意 URL。小文档可内联有界 chunk 批次。对象读取失败可重试，哈希不匹配永久失败。

Search 返回 query_id、generation、实际模式、stale/partial/truncated、证据、排名和路径；不同引擎分数不可直接相加，由 Go 做排名融合后按 revision/chunk 去重。

Reason 返回 status（supported/insufficient_evidence/conflicting_evidence/budget_exhausted）、结论、结论类别、premises、规则/模型版本、证据和限制。规则证明是可核验前提与规则构成的 DAG；模型推断的说明仅是结果解释。无支持的结论不能标为 supported；不得把推断自动写回来源事实。

错误使用标准 gRPC 状态与业务 detail：INVALID_ARGUMENT、PERMISSION_DENIED、FAILED_PRECONDITION（版本/能力）、RESOURCE_EXHAUSTED、UNAVAILABLE、DEADLINE_EXCEEDED、CANCELLED。仅按明确策略重试可恢复错误。

建议首轮实验限制：2 跳、500 节点、1000 边、top_k 10；Search 10 秒、Reason 30 秒。均为可配置实验初值，不是性能承诺；必须经真实中文数据验证。达到限制明确 truncated，不暗示全图穷尽。

## 8. 模型、预算与运行边界

语义服务通过内部模型代理或等价适配使用短期、绑定 tenant/operation/request 的能力凭据；不接收前端任意模型 URL 或长期 API key。能力验证必须证实上游调用可路由该入口，否则对应模式不可用。

每个实际模型尝试单独记录 invocation_id、模型、输入输出用量、耗时、结果状态及父任务。重试可能发生新的真实调用，不能把幂等 operation 当成不会产生额外费用；结算按实际 invocation 去重，父任务不重复汇总计费。预算在调用前准入，调用后结算；未知结果执行对账。

注册规则使用有版本的受限规则集；不接受任意代码，不从文档内容自动发布规则。模型读取的文本仅为不可信数据，不可授予额外工具权限。取消、截止时间和节点/轮次预算同时作用于数据库、模型和推理循环。

服务启用内部认证与 TLS，镜像独立锁版本/依赖。观测记录 tenant/KB、operation、generation、阶段、授权拒绝、索引延迟、查询耗时及模型用量；默认不记录正文或完整 prompt。图与向量索引增长、模型重试和授权子图构造是主要成本来源，尚无成本或性能实测。

## 9. WeKnora 集成与用户流程

新增独立 SemanticIndexService / SemanticQueryService 领域接口和 gRPC 适配器。保留 native 图仓库，不把新服务硬塞进仅有 AddGraph/DelGraph/SearchNode 的旧接口。

候选变更入口：`internal/types/interfaces/`、`internal/infrastructure/semantic/`、`internal/application/service/extract.go`、`knowledge_post_process.go`、`chat_pipeline/search_entity.go`、`internal/agent/tools/query_knowledge_graph.go`、`internal/container/container.go`；新增 Python `semantic/` 服务及独立 proto/Docker/Compose/Helm 定义。实际文件拆分实施前复查，不在本轮创建运行代码。

KB 配置分开存储 desired_backend、active_backend、配置版本与 active_generation。每个 KB 正式查询只走一个图后端。Semantica 影子构建不混入正式答案；达到验收后原子切换 active_backend。切换后的写入进入新后端；回滚前 native 必须补齐切换期间更新和删除，否则不可直接切回陈旧图。

文档保留原解析状态，另有 semantic_status：disabled/queued/indexing/ready/stale/failed/deleting。Go 业务子任务只在 operation 终态时完成一次，以持久唯一键防止重复扣减；attempt 被替代后不能修改新 attempt 的计数。解析完成与语义失败分开呈现，允许单独重试语义索引。

用户看到索引进度、使用版本、失败重试、回答来源和推理类型。GraphRAG 不可用可按请求策略退回普通检索并注明实际模式；明确请求推理时，失败必须呈现推理未完成。主 Agent 经 Go 工具调用语义服务，语义服务不接管主 Agent 的执行循环。

## 10. 分阶段交付与验收

| 阶段 | 交付物 | 退出条件 |
|---|---|---|
| P0 能力验证 | 冻结版本与依赖；中文样本、持久图到授权子图桥接、模型代理、规则/模型推理报告 | 有可复现命令、实际输出；不可用能力明确禁用；证据不足不推进上线 |
| P1 索引基础 | proto、operation、outbox、版本发布、来源、删除屏障 | 重启恢复、重复提交、并发更新、删除晚到均通过 |
| P2 GraphRAG | 授权检索、图扩展、Go 融合、原文引用 | 完成单 KB 跨文档问答及权限反例验证 |
| P3 推理 | 注册规则与模型推断、证据 DAG、预算 | 可验证规则结果；冲突/无证据/预算耗尽不冒充成功 |
| P4 切换验收 | 影子评估、后端切换、恢复与回滚、产品状态 | 满足质量/延迟/成本验收阈值并通过故障演练 |

P0 在现有 native 基线上比较实体/关系准确性、证据定位、多跳问题正确性、无答案识别率、p50/p95 延迟、索引耗时和原始用量。上线数值阈值由样本评估后确认；未设阈值不得声称生产验收通过。

必测场景：

1. 同名实体分属两个 tenant，查询、缓存、合并和删除均不串联。
2. A 可见/B 不可见；B 独有边、别名和摘要不得影响答案或排名。
3. 查询期间撤权，旧 epoch 结果整体作废，不能继续生成并交付受污染答案。
4. 同一事实双来源，删一个保留另一支持；删最后来源使推导失效。
5. 更新 revision 2 后 revision 1 的 worker 才完成，不能回退 active manifest。
6. Apply 响应丢失、worker 崩溃、租约过期，恢复后单个 operation 只有一次发布。
7. 图已写/向量未写时崩溃，查询仍读旧 generation；重启能继续或清理 staging。
8. 取消与发布竞争，以持久 CAS 判定结果，不出现 cancelled 却已发布。
9. 旧模型调用结果迟到，不复活删除文档、不修改新 attempt 的完成计数。
10. 规则前提缺失、事实冲突、模型无引用、中文 span 错位均得到明确非成功结果。
11. 请求超过图规模、用量或时间限制可停止；普通检索降级不伪装推理成功。
12. 后端回滚及备份恢复先应用新增删除屏障，不复活已撤销内容。

## 11. 备选与评审结论边界

备选 A：Go 持有图，Python 只计算。运维轻，但完整 GraphRAG/推理会频繁跨服务搬图，算法与存储演进受 Go 接口限制。用户明确要求服务同时承担检索推理，本规格推荐服务持有语义图。

备选 B：Semantica 接管解析、向量检索、会话与 Agent 运行。减少部分重复链路，但迁移权限、标准分块、引用与会话生命周期的范围过大，不属于当前目标。

推荐方案增加一个有状态子系统、索引副本和恢复责任。如果冻结版本无法支持有界授权子图、可靠来源或模型代理，先补适配或缩小可用模式；若成本/质量不达标，维持 native 正式查询，不能仅因功能列表齐全而上线。

本文未创建业务代码、迁移、容器或 Issue；未运行测试、模型、性能基准。评审重点是服务数据所有权、事实级授权、KB generation 发布和首版能力范围。评审通过后才拆分逐项实施计划。
