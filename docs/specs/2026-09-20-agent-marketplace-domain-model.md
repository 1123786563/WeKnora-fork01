# Agent Marketplace 领域模型

日期：2026-09-20
状态：已批准领域设计；首版只支持免费分发，不授权开始实现。

相关事实源：

- [统一领域语言](../../CONTEXT.md)
- [移动 AI Office 设计规格](./2026-09-20-mobile-ai-office-design.md)
- [ADR-0011：Release 与 Adoption 边界](../adr/0011-agent-marketplace-release-adoption-boundary.md)

## 1. 目的与边界

Agent Marketplace 让团队发现、审核、引入和升级可复用 Agent，同时保持 Tenant 对模型、知识、连接、Sandbox、预算和权限的控制。它服务移动 AI Office 的资源入口，但发布、审核、引入、映射、测试与升级仍由 Web 管理端治理。

Marketplace 拥有 Listing、Release、Manifest、Dependency Lock、lineage、Submission、Review、目录状态、Agent Adoption、升级建议、Evaluation 和隐私聚合 Metrics。

Marketplace 不拥有 Tenant 本地 Agent Definition、能力映射、Agent Version、Task、Run、Task Grant、运行凭据，也不拥有 Skill、Connector、模型、知识库或 Sandbox 的源实体。首版不承担 Publisher 收款、分成、税务和结算。

## 2. 统一目录

Agent Catalog 聚合三种来源，但不创建三套 Agent 模型：

| 来源 | Publisher | Reviewer | 可见范围 |
| --- | --- | --- | --- |
| Built-in | WeKnora | WeKnora 发布流程 | 支持该能力的 Deployment |
| Public Marketplace | Verified Publisher | Platform Marketplace Reviewer | 平台允许的 Tenant |
| Tenant Catalog | 当前 Tenant 的 Agent Author | Tenant Marketplace Reviewer | 当前 Tenant |

移动端只显示已完成映射、测试和本地版本发布的 Available Agent。Marketplace 搜索、Submission、Review、Adoption 和升级不在移动端执行。

首版不支持任意分享链接、跨 Tenant 私发、消费者五星评论或付费 Agent。免费分发不代表没有许可证，也不免除模型、Sandbox 和 Connector 的运行费用。

## 3. 聚合关系

关系链如下：

1. Agent Author 维护 Agent Definition；
2. 确定 Agent Version 形成 Release Submission；
3. Marketplace Review 通过后生成 Agent Release；
4. Marketplace Listing 让成员发现确定 Release；
5. Tenant 通过 Agent Adoption 接受 Listing 和 Release；
6. Adoption 派生一个或多个 Agent Variant；
7. 每个 Variant 完成本地能力映射、测试和 Agent Version 发布；
8. Available Agent 才能被成员用于创建 Task。

Agent Release 同时固定 Agent Manifest、Dependency Lock、内容摘要和 lineage。Agent Evaluation 只关联确定 Release，不关联可变 Listing。

## 4. 身份规则

- Listing 是 Marketplace Agent 产品的稳定身份；显示名称和 slug 不是身份；
- Release 属于一个 Listing，以独立 ID、语义版本和内容摘要标识，发布后不可变；
- Manifest、Dependency Lock 和 Release 内容共享同一摘要边界；
- Adoption 在一个 Tenant 内对一个 Listing 唯一，保存该 Tenant 接受过的 Release；
- 一个 Adoption 可以派生多个 Agent Variant；
- 每个 Variant 固定一个来源 Release，拥有独立本地 Definition、映射、成员范围和发布节奏；
- Task 固定本地 Agent Version，不直接运行 Marketplace Release。

## 5. Release 内容边界

Agent Release 可以携带：

- Agent 身份、描述、portable system behavior、starter 和交互说明；
- Persona、可移植配置和能力需求；
- 可再分发 Skill 与子 Agent 的 Dependency Lock；
- 来源 Listing、Release、Fork lineage 和修改说明；
- Agent Manifest、许可证与变更说明。

Agent Release 不得携带：

- Tenant 知识库、知识条目或成员身份；
- 模型 ID、API Key、Connection credential；
- Sandbox 绑定、Memory 内容、Task 历史、预算或审计；
- 未获再分发许可的依赖；
- 采用方遥测目标或绕过空间策略的回传配置。

Public、Built-in 与 Tenant Release 遵守同一边界。Tenant Catalog 复用本地资源时使用独立 Tenant Binding Preset；Preset 不属于 Release、不含凭据，也不得进入 Public Marketplace。

## 6. Agent Manifest

Manifest 至少声明：

- Listing 与 Release 身份、版本、摘要、Publisher 和时间；
- 来源、Fork lineage、许可证与支持语言；
- 适用和不适用场景；
- 模型能力、知识类型、Skill、子 Agent、Sandbox 和 Connector 需求；
- 可能读取的数据类别与可能产生的外部副作用；
- WeKnora capability 和最低兼容版本；
- Dependency Lock；
- 变更、弃用、安全撤回和替代版本。

Manifest 是发布者声明和审核输入，不是权限。自动扫描可以验证、发现遗漏或阻止 Submission，但不能授予 Tenant 资源或 Task Grant。

## 7. Publication 与 Review

Agent Author 维护 Definition 并对 Submission 内容负责。Submitter 发起确定 Agent Version 的 Submission。Marketplace Reviewer 在对应 Catalog scope 审核。Verified Publisher 只表示公共发布主体身份已验证，不表示每个 Release 安全或获批。

Submission 生命周期：

| 当前状态 | 可进入 |
| --- | --- |
| draft | submitted、withdrawn |
| submitted | in_review、withdrawn |
| in_review | approved、changes_requested、rejected |
| changes_requested | submitted、withdrawn |
| approved | released |
| rejected / withdrawn / released | 终态 |

Review 至少检查 Manifest 完整性、声明与内容一致性、secret 和 Tenant 数据泄漏、Dependency Lock、许可证、恶意内容、能力需求、数据类别、外部副作用、兼容性、Fork lineage 和 Evaluation 边界。

审核记录保留 Author、Submitter、Reviewer、时间、理由和所审核摘要。审核后任何字节变化都必须创建新 Submission 和 Release。

## 8. Adoption 与本地发布

Agent Adoption 是 Tenant 接受 Listing 的治理关系，不是成员个人安装，也不是直接运行。

采用流程：

1. 管理员选择确定 Release；
2. 检查 Deployment capability、许可、依赖和安全状态；
3. Tenant 接受 Release，并建立或更新 Adoption；
4. 创建固定到该 Release 的 Agent Variant 草稿；
5. 管理员完成 Local Capability Mapping；
6. Tenant Binding Preset 只作为可接受、替换或忽略的建议；
7. 运行兼容性检查和 Tenant 测试；
8. 发布本地 Agent Version；
9. 成员获授权后，移动资源页才显示 Available Agent。

Local Capability Mapping 解析模型、知识、Skill、连接、Sandbox、审批和成员范围。它不能回写 Release，也不能让 Release 自带能力获得权限。

同一 Adoption 可建立销售、法务等不同 Variant，并逐个采用不同 Release。每个 Variant 的来源、Release、映射和本地 Agent Version必须可追溯。

## 9. 升级、Fork 与退出

Listing 指向新 Release 时只生成 Agent Upgrade Proposal。管理员比较行为、依赖、能力、安全和许可变化，以新 Release 创建 Variant 升级草稿，重新映射、测试和发布。当前 Agent Version、Task 和其他 Variant不变化。

只修改本地资源和策略属于 Mapping。修改 portable core 时创建 Agent Fork。Fork 保留来源、许可和修改说明，不再跟随上游升级。许可证允许时，Fork 可经 Tenant Review 或 Verified Publisher 加 Platform Review 再发布；上游 Publisher 不对 Fork 质量负责。

退出分两层：

- Retire Variant：禁止新 Task，保留本地版本、历史 Task、Artifact 和审计；
- End Adoption：全部 Variant 退役后，禁止新 Variant 和升级建议，并归档 Adoption。

退出不删除 Listing、Release、许可证或 lineage。安全撤回是更强的独立流程。

## 10. 目录和 Release 状态

| 状态 | 新发现 | 新 Adoption | 新 Task/Run | 历史 |
| --- | --- | --- | --- | --- |
| Listed + Active | 是 | 是 | 由本地 Agent 状态决定 | 保留 |
| Unlisted | 否 | 否 | 已有本地版本可继续 | 保留 |
| Deprecated | 可见但不推荐 | 默认阻止或警告 | 已有本地版本可继续 | 保留 |
| Security Revoked | 否 | 否 | 禁止；运行中按风险暂停或终止 | 保留 |
| Dependency Security Blocked | 否 | 否 | 禁止，直到新 Release 锁定安全依赖 | 保留 |

Unlisted 表示停止分发，不等于风险。Deprecated 表示有替代版本。Security Revoked 表示 Release 自身有风险；Dependency Security Blocked 表示锁定依赖有风险。任何状态都不静默升级或删除历史。

Publisher 注销、失去验证或撤回 Listing 时停止新发现和 Adoption。Marketplace Custody 只按许可和保留规则保存已有 Adoption 继续使用所需的最小 Release、Manifest、许可、来源和 Review 记录。若原因是安全或法律禁止，则使用 Security Revocation。

## 11. Dependency 与传递影响

- 可再分发依赖固定类型、身份、约束、实际版本和摘要；
- 模型、知识、Connection、Sandbox 和凭据只作为 capability requirement；
- Adoption 检查锁定依赖在目标 Deployment 的可用性和许可；
- 安全撤回的依赖传递阻断所有引用它的 Agent Release；
- 运行时不得自动拉取最新依赖或按同名替换；
- 修复依赖需要新 Agent Release、Review 和 Upgrade Proposal。

## 12. Evaluation、Metrics 与隐私

Agent Evaluation 固定 Release、测试集、环境类别、时间和结果。它帮助审核和选择，不代表本地映射效果保证，也不授予权限。

首版信任信号包括 Verified Publisher、Reviewer、Review 时间、Release 新鲜度、兼容性、许可证、安全状态、Manifest 完整度、自动扫描、结构化 Evaluation，以及去标识化的 Adoption、升级和错误类别指标。

Publisher 默认不能获得 Tenant、成员、Prompt、输出、知识、Connection、工具参数、本地 Mapping 或 Task 内容。采用方主动提交诊断时，必须先展示并确认脱敏内容。Release、Skill 和 Connector 不得自带绕过 Tenant 策略的 Publisher telemetry。

## 13. 权限能力

以下是领域能力，不预设具体 RBAC 角色名：

- author_agent：维护 Agent Definition；
- submit_release：创建 Release Submission；
- review_tenant_release：审核 Tenant Catalog；
- review_public_release：审核 Public Marketplace；
- manage_listing：发布、下架或标记弃用；
- security_revoke_release：安全撤回；
- adopt_agent：建立 Tenant Adoption；
- configure_variant：完成本地能力映射；
- test_variant 与 publish_agent_version：测试并发布本地版本；
- retire_variant 与 end_adoption：退出使用。

创建 Agent、引入 Agent、审核目录和批准 Task Action 是不同能力。Catalog 权限不授予运行资源或 Task 内容访问。

## 14. 与现有代码的差异

当前 tenant expert market 已证明可以导出脱敏 Agent 快照、排除 KB/模型/Sandbox/Memory 等绑定、在同 Tenant 浏览和实例化，并在 Listing 下架后保留副本。

目标模型需要以下演进：

| 当前语义 | 目标语义 |
| --- | --- |
| 再发布刷新同一 publish row 和 snapshot | 内容变化创建不可变 Release，Listing 只改变推荐指向 |
| PublishedExpert 同时承担快照与目录项 | Release 与 Listing 分离 |
| Contributor 安装后创建个人 Agent | Tenant Adoption → Variant 草稿 → Mapping/Test → Agent Version |
| Skill 和 subagent 名称引用 | Dependency Lock 固定身份、版本与摘要 |
| 只有 Tenant market | Built-in、Public、Tenant 共用统一模型 |
| 没有 Submission 和 Review | 分离 Author、Publisher、Reviewer 和审核状态 |
| 没有显式升级与安全传播 | Upgrade Proposal、Revocation 和传递阻断 |

expert_installs、published_experts 和现有 API 名称是实现事实，不应成为新的领域词汇或强迫目标模型继续使用“Expert 安装”隐喻。

## 15. 验收场景

1. Tenant 引入 Public Release，缺少必需 Connector 时只能得到 mapping-required Variant，不能运行；
2. 同一 Adoption 创建销售和法务 Variant，绑定不同知识和策略，移动端显示为两个 Available Agent；
3. Listing 发布 v2 后，销售升级、法务继续 v1，既有 Task 保持原 Agent Version；
4. Publisher 下架 Listing，已有 Variant 可继续但不能新 Adoption；
5. Release 安全撤回后，新 Task/Run 被服务端拒绝，历史可审计；
6. 锁定 Skill 被撤回，依赖它的 Agent Release 被传递阻断，不能按名称替换；
7. Binding Preset 引用的 KB 被删除，Variant 进入重新映射状态，Release 不变化；
8. Fork 保留 lineage；许可证禁止再分发时 Submission 被拒绝；
9. Publisher 注销后，已有 Tenant 可按许可使用受保管 Release，但目录不继续公开原 Publisher；
10. Publisher 无法从 Metrics 反推出 Tenant 或读取 Task；
11. Reviewer 审核摘要与最终 Release 摘要不一致时发布失败；
12. Retire Variant 和 End Adoption 不删除版本、Task、Artifact、许可与审计。

## 16. 首版非目标

- 付费 Agent、订阅、试用、退款、Publisher 分成和结算；
- 消费者星级、自由评论和广告排名；
- 任意链接分享或跨 Tenant 私发；
- 自动升级、运行时依赖漂移或本地改动三方合并；
- Publisher 访问采用方遥测或 Task 内容；
- 移动端 Submission、Review、Adoption、Mapping 或 Agent 编辑；
- 把 Marketplace Review 宣称为适合所有 Tenant 的保证。

## 17. 完成边界

本规格确定统一语言、聚合关系、状态、权限和安全不变量，不定义数据库表、HTTP 字段、搜索引擎、扫描器供应商或 UI 视觉。实施前应验证现有 CustomAgent、PublishedExpert、ExpertInstall、Skill Market 和 Agent Version 如何迁移，再编写 Superpowers Implementation Plan。
