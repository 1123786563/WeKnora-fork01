# Open-connector 多空间与故障注入验收（T17）

> 任务：T17（验收链 T16✓）。基线 `b8800c75`（集成分支 HEAD）。
> 机器可读结果：`scripts/open-connector/acceptance-cases.json`；证据源：`go test -tags=integration -v` 输出的 `OC17-EVIDENCE` JSON 行（38 条，post-fix 实测）。
> 状态：矩阵 14/14 通过（回归含 -race，0 数据竞争）；T17-F1 生产缺陷**已按协调者 R20 授权在本任务内修复**（GatedOCClaims 转发 settle face；复现探针 + 直接类型断言双绿；end-state 残留清零），1 个规格观察（T17-F2）仍上报。

## 1. 环境与门禁（裁决 1、R8）

- 一次性容器：`weknora-oc-t17-pg-1`（`postgres:16-alpine`，宿主端口 **55417**，非 5432；`POSTGRES_DB=oc17`）。
- 迁移（宿主侧 `psql -v ON_ERROR_STOP=1`，逐个 exit 0）：000117 app_installations → 000124 open_connector_tool_bindings，共 8 个，产出 18 张表。
- 测试端在容器内再建**每次运行独立 schema**（`oc17_<unix-nano>`）并重放同一组 versioned `.up.sql`（隔离 + 可重复）；`tenant_members`（000043 父表）按仓库惯例用冻结模型 AutoMigrate 物化。
- 环境门禁 `TestOCIntegrationEnvironment` 逐字实现计划草图：缺失 `OC_TEST_DATABASE_URL` 时 **Fatal（blocked-env），绝不 Skip**——复跑 `env -u OC_TEST_DATABASE_URL go test ...` exit 1，三个入口全部 `blocked-env: OC_TEST_DATABASE_URL required`。
- 工具链：go1.26.3 darwin/arm64、docker 29.4.0、python3 3.9.6、node v26.7.0。

## 2. 拓扑（裁决 2）

两空间（tenant 91701/91702），各 Owner+Member，`member-a1` 跨两空间；共享私有 runtime `rt-oc17-shared`；同一 Provider `github` 两账号（`ext-account-a/alias-account-a`、`ext-account-b/alias-account-b`，绑定身份按 T03 契约终身不可变）。HTTP fake Provider 只保存**操作计数**与信封（T01 冻结信封逐字），并按上游幂等契约对同 key 到达做重放（不产生第二次副作用）；grants/OAuth 关联/审批摘要/claim/幂等键/恢复全部走真实 service/repository/recovery/container 代码路径。两套装配：`stack.prod`（`container.NewOCArmedActionService` 生产组合）与 `stack.t12`（同一批件 + 原始 claim store 直连 settle face = T12/T13 设计意图）。该双装配始于 T17-F1 时期的规避需要；R20 修复后 GatedOCClaims 已透明转发 settle face，两套装配行为一致，保留双装配作为持续性对照（探针钉住该一致性）。

## 3. 14 场景矩阵结果

| # | 场景 | 结果 | 关键观测（证据字段） |
| --- | --- | --- | --- |
| 1 | 跨空间 | PASS | B 于 A 连接 PrepareOC → `oc_definition_unreachable`；Begin → `connection_forbidden`；完成 A attempt → false；claim A action → `action_not_found`；A02 → `connection_version_stale`；Provider 请求数增量 **0** |
| 2 | 个人连接 | PASS | 同空间成员可 Prepare（租户目录链可达）但 Execute A02 复检 `connection_forbidden`，action 保持 authorized 未被消耗；Provider 增量 0。列表可见性见 T17-F2 |
| 3 | 空间连接 | PASS | 无 grant 成员与（记账）admin 均拒；对照：显式 grant 通过（拒绝由 grant 驱动）；生产 nil-grants 下 owner 也拒（fail-closed，container.go:506） |
| 4 | 换账号 | PASS | 旧审批 Execute → `connection_version_stale`（旧绑定 revoked v2），Provider 0 发送；空 alias 被 T02 客户端拒绝（`ErrAliasRequired`，无 default 回退）；新账号发送且 Provider 仅见 `alias-account-a2` |
| 5 | 撤销竞争 | PASS | revoke→claim：拒绝、approval_remaining=1 未消耗、0 发送；claim→revoke：在途原样完成（succeeded、恰 1 次 Provider 写） |
| 6 | 审批篡改 | PASS | 改 args → 旧摘要拒绝（mismatch）+ DB 篡改 args/risk/version 摘要重算可检出；同版本改 schema → 目录拒绝；v1 摘要 approve/execute → `action_reprepare_required` |
| 7 | 并发重复 | PASS | 20 并发 Execute：**1 胜 19 败**、Provider 副作用 **1**、approval 恰耗 1（remaining 0）、终态用量事实 **1**。CARRY T14-QI-1：20 并发 PrepareForTool 于真实 PG → **1 action 行 + 1 binding 行** |
| 8 | 不确定结果 | PASS | 写后断连（hijack）：`dispatch_unknown`、action+record 双 unknown、Provider 写恰 1；恢复环 & 手动 Execute 均**不重发**（`action_state_conflict`）；Go transport 在断连后同 key 重投 1 次被上游幂等契约吸收（end-state `key_replays_absorbed=1`）。无重发按钮/后台 POST（见 §5） |
| 9 | 本地失败 | PASS | 计费结算首投失败：原结果先落库（action+record 已 terminal）、Execute 报错；恢复环仅重投结算（同一事实 Revision=1）→ 恰 1 次成功交付、Provider 恒 1 次写 |
| 10 | 崩溃 | PASS | claim 后"杀死工人"（等价持久态）+ 心跳回拨 >90s：sweep 将 record+action 双 unknown；**key 与 runtime 前后一致**；恢复期间 Provider 0 写；重复 claim 拒绝。CARRY 观测：过期 attempt 停留非终态（无 sweeper，T08 Q-2/Q-3 → T18） |
| 11 | 期限 | PASS | ReplayAllowed 边界：窗口内 true、`now==ReplayUntil` false、时钟回拨 false、>24h 破损窗 false；时钟推进过截止后恢复环 **0 自动回放**，终态 record 不被扰动、key/runtime 保留 |
| 12 | 凭据 | PASS | Provider 收到的 Authorization 恰为当前 (tenant, connection, auth_version) 的受限 token（`Bearer tok-A1-v<n>-restricted`）；过期代/跨租户 token 拒发；admin 专用材料从未出现在任何 dispatch/DB 行/action blob/密文中；API-key 途密封无明文（§5 静态扫描） |
| 13 | 多副本 | PASS | 两副本装配（tuned：tenant2/conn1/provider8/global3）并发 6 单（3+3 双空间）：Provider 恰 6 写、峰值并发 ≤ 全局上限（实测 1，受 DB lease 约束）、两空间全部调度成功（3/3+3/3） |
| 14 | 回归 | PASS | 见 §6（Go 8 组 + Python 43 + web appconnector 7/7，-race 0 竞争；web 全量有 1 例基线即失败的 node 环境漂移，非 T17） |

收尾扫描（final_settlement_scan）：挂起 unknown 经 Provider 查询 resolver 全部收敛（场景 8 → succeeded，场景 10 → failed）；未解释开放记录 **0**、未交付结算 **0** → 清理不阻塞。修复前曾有 T17-F1 残留（record=dispatched/action=succeeded 一例，入原始 journal）；R20 修复后生产装配同样把 record 落终态，end-state `finding1_residue=null`、`open_records=null`（post-fix journal 实测）。

## 4. RED / GREEN 记录

- **门禁 RED**：无 DSN 运行 → exit 1，全部 `blocked-env`（不 Skip）。
- **业务 RED**（临时未提交篡改，跑完即恢复，`git diff` 干净）：
  - RED-A2（cross_space）：同时移除 `ocAuthorizer.Check` 与 `CanUseConnection` 的租户门 → exit 1，"want tenant-scope denial"。（仅移除前者时仍拒——纵深防御记录为观察。）
  - RED-B（replay_deadline）：`ReplayAllowed` 恒 true → exit 1，"replay allowed AT the cutoff"。
  - RED-C（unknown_outcome）：`settleOutcome` 把 unknown 伪造成 failed → exit 1，"want ErrDispatchUnknown"。
- **GREEN**：`OC_TEST_DATABASE_URL=<一次性PG> go test -tags=integration ./internal/application/service/appconnector -run 'TestOCIntegration$' -count=1` → **exit 0**（门禁 + 13 场景 + 收尾）。`-race` 同矩阵通过、0 数据竞争。
- 全量入口（含探针）修复后 **exit 0**（探针绿：record 经生产装配落终态 + settle face 类型断言命中 + 交付 fence 抬升）。修复前该入口 exit 1（仅探针红），即 T17-F1 的原始 RED 证据。

## 5. 凭据与"无重发"静态证据（场景 8/12 佐证）

- 路由面：`internal/router/routes_app_connectors.go` 无任何 replay/resend/retry POST 路由（actions 仅有 prepare/get/approve/execute）。
- 工具面：`internal/agent/tools/app_connector.go`（T14 facade）无 approve/execute 面、无凭据字段；DTO（连接视图、attempt 视图）无 credential 字段。
- 前端：T15 四视图与 `action-state.ts`/`connection-state.ts` 无重发按钮/重试提交逻辑（7/7 web 测试通过）。
- OC 客户端错误契约无 secret（T02 冻结，错误文本静态）；密文/DB 行扫描断言见场景 12。

## 6. 回归命令与退出码（场景 14）

| 套件 | 命令 | 退出码 |
| --- | --- | --- |
| service/appconnector（含 native/sync/OC 单元） | `go test ./internal/application/service/appconnector/...` | 0 |
| repository/appconnector | `go test ./internal/application/repository/appconnector/...` | 0 |
| connectorcontrol | `go test ./internal/connectorcontrol/...` | 0 |
| container | `go test ./internal/container/...` | 0 |
| handler | `go test ./internal/handler/...` | 0 |
| commercial（域+仓储） | `go test ./internal/commercial/... ./internal/application/repository/commercial/...` | 0 |
| appconnector 域（OAuth/MCP/Sync 原生） | `go test ./internal/appconnector/...` | 0 |
| Python 门禁（contract+deployment） | `python3 -m unittest discover -s scripts/open-connector -p 'test_*.py'` | 0（43 OK） |
| 集成矩阵 + race | `go test -race -tags=integration ...` | 全套件 PASS、0 数据竞争（探针亦绿，post-fix） |
| web T15 面向 | `node --import tsx --test src/appconnector/*.test.ts` | 0（7/7） |
| web 全量 | `npm test` | 1（`platform/legacy-session` 在**纯净基线 b8800c75 同样失败**，node v26.7.0 环境漂移，非 T17、非 appconnector 面） |

## 7. 发现（只报不修）

### T17-F1（生产缺陷 → 已按 R20 授权修复，探针双绿）

`container.GatedOCClaims` 只内嵌 claim 接口（ClaimOCDispatch/GetOCDispatch），而 `ActionService` 的 settle face 通过**类型断言**探测（`ocDispatchSettleOf`）。生产组合 `NewOCArmedActionService` 把包装器交给 `UseOCDispatchClaims` → 断言失败 → Execute 静默回退到 pre-T12 完成序：**durable record 永不落终态**。后果：每次成功派发的 record 90s 后被 stale sweep 判为 unknown（T16 "unknown 最老>5min" 告警常响）、结算 outbox 永不经 record 排水、对齐/解析通道看到终态 action 配非终态 record。复现（修复前 RED，exit 1）：`TestOCIntegrationWiringSettleParity`（生产路径 Execute 成功后 action=succeeded 而 record=dispatched）。**修复（R20 授权，追加 commit）**：`GatedOCClaims` 转发 `FinishOCDispatch/MarkOCDispatchSettled`（含编译期 `var _ OCDispatchSettleSource = (*GatedOCClaims)(nil)` 一致性断言；结算不走 shutdown 门——裁决 8 要求停机排空+结算）。修复后：探针转绿且新增两处直接断言（生产组合的 claims 类型断言命中 settle face；结算后 record fence 抬升超过 action fence = 交付已标记）；全套件 exit 0、-race 0 数据竞争、8 组回归全绿、end-state `finding1_residue=null`。

### T17-F2（规格观察）

连接列表接口对同租户所有成员返回**全部**连接的元数据投影（无凭据材料，含他人个人连接）："列表隐藏"目前由前端承担；建议后续任务加服务端 owner 过滤或可见性字段。

## 8. CARRY 结论

- **T14-QI-1（PG 并发 tool-binding 竞争）**：验收级关闭——20 并发 PrepareForTool 于真实 PostgreSQL 恰产生 1 行 action + 1 行 binding（场景 7 证据）。
- **T08-F-2/T12 stale-attempt 卫生**：观察确认——过期授权 attempt 停留 pending/authorizing/verifying（期末计数 2），一期无 sweeper。**T18 终局裁决：记为一期已知限制（不实现）**，发布侧以 [open-connector-release.md](./open-connector-release.md) §3 阈值监控兜底。
- **T16 validator 边角**：按预授权关闭——`check_deployment.py` 在过滤前对 networks 声明形状校验（+2 用例；RED/GREEN 双证：stash 关闭后新用例 exit 1，恢复后 exit 0；28/28 全绿）。

## 9. 清理纪律（裁决 6）

fixture manifest 捕获：一次性容器（`weknora-oc-t17-pg-1/-2/-3` 依次用于原始验收、R20 修复复验、终稿润色复验）、每次运行 schema（测试自删）、`t.TempDir()` token 目录。历次期末均无未解释 unknown/未决结算 → 清理不阻塞；R20 修复后 end-state 无任何残留（record 全部落终态）。各容器在其轮次报告落盘后停止并删除（记录见任务报告）。

## 10. T18 终局验收结论（发布门禁视角）

> 门禁工具 `scripts/open-connector/release_gate.py`（29 用例，含计划逐字 `test_mock_write_never_qualifies`）；七类证据现状、真实 provider_read 只读验证记录与缺项清单见 [open-connector-release.md](./open-connector-release.md) §5–§6。

- **验收结论：不可发布（不写"生产可用"）**。门禁现状对 provider_write、billing 两类 FAIL（blocked-env：本会话无真实 Provider 写授权、无商业计量环境授权）；provider_read 已在候选镜像上完成真实只读验证（GitHub `get_current_user` 200，清理零残留），但证据 fixture 已按 R21 落库（`scripts/open-connector/fixtures/provider_read_runtime.json`），WeKnora 全链真实读/OAuth connected 态仍 blocked-env。
- 六项 CARRY 终局裁决记录于 SDD ledger task-T18-report.md：T01 OAuth 子项永久 blocked-env；T11-QF-1 failed 封闭集复核**通过**（400/403 状态级封闭与契约 §3.3 一致，代码 `oc_dispatcher.go` classify）；T13-F-3 不设 auth URL 端点（T15 文案终局）；stale sweeper 不实现（T17 计数 2）；Retry-After 透传保持未实现（固定 30s 冷却）；T17-F2 列表可见性列规格积压。
- 门禁语义钉死：mock/doc 证据永不合格；`passed=true` 不被单独信任（CLI 复核 artifact 存在 + sha256 一致 + 同 commit/镜像/测试命名空间）；`open_unknown_count` 必须显式为 0。
