# ACL 写入口盘点（A01）

日期：2026-09-11。方法：逐一阅读真实文件与函数，确认每个影响语义可见性的写入口及其事务边界，接入同事务 epoch 提升（`BumpTenantSemanticEpochsTx` / `BumpKBSemanticEpochsTx` / `BumpOrgSharedKBSemanticEpochsTx`）或经 `WithSemanticMutation`（删除屏障+epoch 同事务）。

## 已接线（同事务 epoch）

| 写入口 | 文件/函数 | 事务 | 接线 |
|---|---|---|---|
| 成员角色变更 | repository/tenant_member.go `UpdateRole` | 自有 tx | 全租户 KB epoch |
| Owner 降级 | repository/tenant_member.go `DemoteOwnerAtomically` | 自有 tx（行锁） | 全租户 KB epoch |
| Owner 移除 | repository/tenant_member.go `RemoveOwnerAtomically` | 自有 tx（行锁） | 全租户 KB epoch |
| 成员软删除 | repository/tenant_member.go `SoftDelete` | 自有 tx | 全租户 KB epoch |
| 分享权限变更 | repository/kbshare.go `Update` | 自有 tx | 源 KB epoch |
| 分享移除 | repository/kbshare.go `Delete` | 自有 tx（先读 share） | 源 KB epoch |
| 组织成员移除 | repository/organization.go `RemoveTenantMember` | 自有 tx | 该组织全部被分享 KB epoch |
| 组织成员角色变更 | repository/organization.go `UpdateTenantMemberRole` | 自有 tx | 该组织全部被分享 KB epoch |
| 文档删除/替换（语义侧） | repository/semantic_outbox.go `WithSemanticMutation` | 单事务 | deny 屏障+epoch（I02） |
| 文档移动/克隆 | service/knowledge_transfer.go `executeKnowledgeClone`（经 `bumpTransferEpochs`） | 删除批后+克隆批后（可恢复多文档流程，无单一事务；每批自有短事务） | 源+目标 KB epoch（SemanticEpochBumper，nil 安全） |
| 组织删除 | repository/kbshare.go `DeleteByOrganizationID`（service/organization.go `DeleteOrganization` 调用） | 自有 tx | 先 bump 该组织全部被分享 KB epoch 再删行（同事务，顺序保证 bump 可读被删行） |

## 盘点修正

| 原列项 | 核实结果 | 处置 |
|---|---|---|
| 临时文档到期（service/temporary_document.go CleanupExpired） | TemporaryDocument 无 KnowledgeBaseID（会话级资源，types/temporary_document.go:24-48），从不进入知识库、从不被语义索引 | 无需 epoch 接线——到期清理不影响任何语义可见性（非缺失） |

## 由后续任务接线（归属明确）

| 写入口 | 归属 | 说明 |
|---|---|---|
| 业务文档增删改的语义事件 | I05 | 经 `WithSemanticMutation`（deny+epoch 已内建）；业务删除必须走该路径 |
| KB 删除/迁移的 scope 版本 | I05/W03 | 目标态由 backend_states/generation 管理 |
| 权限服务不可用 | Q04 | ValidateDelivery fail-closed 已实现（epoch 读取失败=错误） |

## 已知缺口（记录）

- 规格 §6"敏感替换可指定立即隐藏旧版本"尚未建模：`AllowRetainedPrevious` 目前为全局常量 true，`SemanticMutation` 无按次隐藏旧版本字段（I05 建模）。
- Issue 的 scope key 目前由调用方给定；Q 任务接线时必须经 `resolveKBReadTenant` 取资源 owner tenant，并校验 subject 读权限（A01 评审条件 2）。

结论：**ACL 写入口已无漏接路径**（11 条已接线 + 1 条盘点修正豁免 + 3 条归属后续任务）；剩余上线条件为 Issue 生产调用方经 resolveKBReadTenant 取 owner tenant（Q04 接线时强制）。
