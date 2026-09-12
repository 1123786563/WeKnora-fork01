# SaaS 商业迁移操作手册（O02：旧空间与数据源迁移、影子计量与注销保留）

适用组件：internal/commercial/migration.go（域规则）、cmd/saas-migrate（操作 CLI）、internal/application/service/tenant.go（注销保留门）。

核心原则：**迁移绝不发明商业状态**。不发付费额度、不虚构连接归属、不共享/伪造 Customer、旧余额不折成真钱 Grant、数据源保留直至新绑定验证、注销在途记录永不级联删除。

## 1. 迁移决策域规则

- InitialCommercialState(provenConnectionOwner)：所有迁移空间从 **basic（免费基础档）** 起步，IssueCredits=false；仅当 A07 绑定证据证明连接归属时 ConnectionState=ready_to_bind，否则 requires_reauthorization（沿用 A07 词表）。
- PlanTenantMigration（逐租户、幂等）：
  - 已有 F02 Customer 映射 → **复用**（ReuseExistingCustomer=true），绝不新建第二个 Customer；
  - 无映射但操作员显式提供 tenantID=customerID → 绑定该 Customer；
  - 两者皆无 → **跳过**（SkipReason=no_existing_customer_mapping_and_no_operator_provided_customer_id），凭据/Customer 绝不共享或编造；
  - LegacyBalanceConverted 恒为 false：旧余额仅记录，不折成真钱 Grant；
  - DataSourcesPreserved 恒为 true：数据源保留直至新绑定验证。
- ApplyAllowed(explicit)：只有显式 --apply 才允许任何写入；默认 dry-run 零写入。

## 2. dry-run：读报告

    go run ./cmd/saas-migrate -tenants 1,2,3 -db-driver sqlite -dsn file:weknora.db

- 无 -tenants/-tenants-file 直接拒绝：CLI **永不**对隐式全量租户执行。
- 报告逐租户输出：status（would-apply/skip）、plan、issue_credits、connection（requires_reauthorization/ready_to_bind）、owner_proven、customer_id、reuse、skip_reason、legacy_balance_converted、data_sources_preserved。
- status=skip 的租户：操作员需先在外部计费系统核实 Customer（-customer-map 8=cus_x）或确认该租户暂缓迁移。
- dry-run 不打开写路径：未提供 -dsn 也可对显式清单出计划（无既有映射证据时全部按无映射规划）。

## 3. apply：执行程序

    go run ./cmd/saas-migrate -tenants 1,2 -customer-map 2=cus_op_42 -proven-owners 1,2 -apply -db-driver sqlite -dsn file:weknora.db

1. 先 dry-run 同参数审阅一遍（去掉 -apply）。
2. -apply 与 -dsn 缺一不可，否则拒绝执行。
3. 逐租户经 F02 AccountStore.Bind 幂等绑定：INSERT ... ON CONFLICT DO NOTHING + 读回校验。
4. 任一租户失败：该租户标记 FAIL，继续处理其余租户，**进程以非零退出**并输出逐租户报告。
5. 迁移后核对：reuse=true（或 customer_id 与外部系统一致）、connection 状态符合预期、issue_credits=false。

## 4. 幂等保证

- 重复执行同一 apply：Bind 对相同 (tenant, customer) 是 no-op，**不会产生第二个 Customer**；不同 customer 的二次绑定返回 ErrAccountMappingConflict（F02 域错误）而非静默改绑。
- 重复 dry-run：纯只读，可任意次执行。
- 租户清单去重排序；文件清单支持 # 注释与空行。

## 5. 影子计量切换（shadow cutover）

域规则 ShadowBillable(callStartedAt, enabledAt, eventConfigVersion, cutoverConfigVersion)：

- 判定**只**依据可信调用开始时间与配置版本；上报/重放时间**不是**输入——旧影子事件在切换后重放永远不能变成收费事件。
- 调用开始时间早于 enabledAt → 永远保持 shadow（即使之后才上报）。
- 开始时间 ≥ enabledAt 且事件配置版本 == 切换配置版本 → 计费。
- 事件配置版本落后于切换版本 → 永远 shadow。
- 影子模式本身只记录 usage，不投外部消费。

## 6. 注销保留（DeleteTenant 商业门）

tenantService.DeleteTenant 在删除任何租户资源**之前**：

1. DisableNewScheduling：停新商业调度（已派发调用仍会结算）；
2. RevokeConnections：撤销 app-connector 连接；
3. PendingCommercialWork 列出在途记录（pending settlement/payment/refund）→ CheckDeletionReadiness：**有在途即拒绝删除**（更安全的选择），错误信息逐条列出；
4. 记录保留政策版本：AutoDelete 仅在「无在途 + 已配置保留政策版本」时为 true；**未配置长期保留策略绝不自动删除**，pending commercial 记录**永不**随租户删除级联删除。
5. 未接线 guard（商业模块未装配的部署）保持旧删除路径并输出 WARN 日志——guard 本身只做检查与显式状态翻转，不删除任何商业记录。

## 7. 安全边界

- CLI 只使用既有 store（F02 AccountStore 读写 commercial_accounts；其余只读），不触碰其他任务拥有的表。
- CLI 依赖：stdlib + 既有 internal 包（gorm sqlite/postgres 驱动）。
- 真实生产 fleet 迁移、真实 PostgreSQL 迁移执行、外部计费系统 Customer 创建均需在隔离环境演练后执行（当前证据为 SQLite/内存级）。
