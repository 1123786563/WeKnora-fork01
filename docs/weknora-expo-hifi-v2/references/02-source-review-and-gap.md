# 当前源码阅读与差异清单

基线：`12737238aa7b9d6891e76b2397f6941024f45668`。以下是静态源码观察和设计推论，不是项目测试通过报告，也不是完整安全审计。文件内容通过 GitHub 连接器读取；完整 clone 因 DNS 失败未完成。

## 1. 已经拥有的基础

| 能力 | 实际读取到的证据 | 判断 |
| --- | --- | --- |
| React Web | `apps/web/package.json`、`src/App.tsx`、共享 views 导出 | 当前已有 React 工程，不能照抄旧 OC 计划“仅 Vue”的判断 |
| Expo 原生端 | `apps/mobile/package.json` 与 `sources/weknora` | 不应从空 App 起步；依赖和接线仍需复验 |
| 统一执行 API | `routes_workbench.go` 与读 handler | 启动/查询/快照/SSE/命令/交互路由声明存在；DI、部署和权限全链另验 |
| 执行 SDK | `packages/api-client/src/mobile/executions.ts` | wire 校验、request lookup、revision 命令已有；不代表任意交互 SDK 都齐全 |
| 产品身份 | SecureStore、ProductAuthProvider、scope | 存在产品认证入口；冷启动/OIDC 后 scope 恢复仍需真实接线验证 |
| 原生 OIDC | `startNative`、`exchangeNative` | POST `/auth/mobile/exchange` SDK 方法已存在；不能沿用旧文档说完全没有 |
| 原生读流 | `stream-transport.ts` | 增量 parser 已有，但与已读后端的 data 形状存在差异 |
| 本地缓存 | `execution-storage.ts` | driver/cipher/游标边界已有；原生 SQLite API、加密和生命周期须复验 |
| Happy 交互资产 | SessionView import 与现有组件 | 输入/消息/语音/文件/Diff 投入可复用；不能等同产品业务链已解耦 |
| OC 集成 | T01–T18 进度表 | 记录大量已有实现/验收；T18 发布能力仍受真实写入、商业证据门禁限制 |

## 2. 优先修复项

P0 表示“应先解决再做核心链验收”，不是未经测试就断言系统已被利用。P1 表示“进入对应能力发布前处理”。

### G01 / P0：SSE data 形状不一致

**事实：** `flushWorkbenchEvents` 将 `event.Payload` 传给 `writeWorkbenchSSE`；writer 直接 compact 后写 `data:`。原生 parser 却对 data 执行 `parseExecutionEvent`，要求 run_id/attempt_id/seq/type/time/payload 完整对象。[S08](sources.md#s08)[S15](sources.md#s15)

**风险：** 常规业务 payload 无完整 envelope 时，当前两端不能直接互通。各自单位测试可绿，但真实串联失败。

**处理：** 产品流统一 envelope；必要时明确协商版本；控制帧另行解析；同一 Go 输出 fixture 喂给真实 TS parser。不要“客户端凑空 attempt_id/时间”来掩盖丢失语义。

**验收：** 正常文本、引用、工具、终态、未知type、409游标过期、UTF8/CRLF跨chunk均通过；一次终态至少重放至终态 watermark。

### G02 / P0：审批控件与命令接口不一致

**事实：** ConversationScreen 调用 `viewModel.commands.approve?.(...)` 和 `reject?.(...)`；已读 ConversationCommands 只有 cancel、steer、refreshPending。[S12](sources.md#s12)

**处理：** 与真实 interaction decision DTO 对齐，增加类型化决定端口和细化 VM；不要用 `any` 或静默 optional-chain 掩盖缺失。当前 refreshPending 调用的是 request lookup，应拆“刷新提交状态”与“刷新交互详情”。

**验收：** 原生 typecheck、挂载页面交互测试、真实 backend 允许/拒绝/409/过期/撤权；按钮不能只是“看起来可点”。

### G03 / P0：能力与 revision 硬编码

**事实：** 产品 VM 构造将 canCancel/canSteer 设为 true，command 默认 expectedRevision=0；服务端 DTO 支持更细的 capability 与 revision。[S09](sources.md#s09)[S12](sources.md#s12)

**处理：** 映射 supported/unavailable/forbidden + reason；无快照 revision 不允许控制命令；409 刷新当前状态而非重复发送旧 revision。

**验收：** 只读成员、禁用driver、远程未就绪、已终态、不支持steer均不误放行。

### G04 / P0：提交记录不应只活在内存

**事实：** ConversationScreen 点击发送生成 request ID；已读 send controller 是内存单飞；createProductConversationViewModel 在 start 后未在该函数中消费 ack，并在内存保存 latestRequestID。[S12](sources.md#s12)

**范围说明：** 这是已读接缝的缺口，不能据此宣称仓库其它地方完全没有恢复逻辑。

**处理：** 在发送前保存 request_id 与 input_hash；未知结果查既有 lookup；同一次用户意图不分配新 ID。消费 ack 后绑定 Run，再接快照和持久事件流。

**验收：** ack 丢失、杀进程、重复点击、重启恢复均能找到同一 Run；相同 ID 不同输入返回明确冲突。

### G05 / P0：原生版本组合漂移

**事实：** Expo55/RN0.83.1 与 React19.3.0、renderer19.0.0 同时声明；根 pnpm10.28.2、移动 packageManager10.11.0；旧设计采用 React19.2.0。[S04](sources.md#s04)[S05](sources.md#s05)

**处理：** 先以官方 SDK55 兼容矩阵复现并对齐版本/renderer/锁文件；SDK57 单独验证。不能声称“这些版本一定运行崩溃”，也不能当兼容已证实。

**验收：** 锁文件解析记录、doctor、原生typecheck、双平台构建、关键native模块运行。不得未经审查运行包含删除原生目录的 prebuild 脚本。[O01](sources.md#o01)[O02](sources.md#o02)

### G06 / P0：SQLite 接口不等于实际 SDK 接口

**事实：** 自定义 ExpoSQLiteDatabase 把 withTransactionAsync 声明成返回 Promise<T>；官方 Expo API 是 Promise<void>。当前 driver 直接返回该方法结果；当前 write/read 是异步事务且持有注入 cipher。[S16](sources.md#s16)[O06](sources.md#o06)

**处理：** 编写真实 Expo 适配器，wrapper 显式捕获结果；原生写队列+独占事务；避免不相关异步查询混入事务。使用原生 txn 对象，不以 fake 泛型接口当原生 API。

**验收：** 真机读回数组与 cursor、并发写、磁盘满、database locked、scope 清理、冷启动重开DB。AEAD key 初始化加 single-flight，验证并发首次加密、密钥丢失和AAD篡改。

### G07 / P1：产品 VM 没有消除 Happy 全局依赖

**事实：** SessionView 接受 VM，但仍读取 Happy useSession/useIsDataReady/useSetting、sync 与相关 operations。[S14](sources.md#s14)

**处理：** 产品渲染器与 legacy renderer 分离，复用纯组件和明确 ports。产品会话不应该必须造一个 Happy 假会话才能显示。

**验收：** 未登录 Happy、无 Happy daemon/sync 数据时，WeKnora 产品链仍能独立展示/发起/审批；原 Happy 路径有回归测试。

### G08 / P1：三种状态被压成一个状态

**事实：** createConversationViewModel 将 ExecutionDTO.execution_status 映射为单一 status；产品控制还需要 run_status、settlement_status。[S09](sources.md#s09)[S12](sources.md#s12)

**处理：** 同时保留任务进度、执行观察、结算状态及最后同步时间。未知不显示失败，申请取消不显示确认停止，任务完成不等于结算已完成。

### G09 / P1：Source event 的写入前授权边界

**事实：** 已读路由将 source-events 置于普通 workbench Viewer/API-key 分组；handler 在调用 ingestor 后比较返回 event.RunID 与请求资源。[S08](sources.md#s08)

**风险范围：** 尚未审完全部中间件与仓储，不能断言存在可利用路径；但移动设计不得将该入口当一般客户端可写。

**处理：** Bridge/worker 服务身份与实例/租约/绑定校验；在任何事件持久化前完成 tenant、run、binding 一致性验证。错误路径不先写入再404。

### G10 / P1：共享测试入口不能代表全部移动测试

**事实：** 根 test:shared 显式加入 api-client/mobile，但已读命令未包含 domain/src/mobile/*.test.ts；typecheck:shared 是显式文件列表，不是覆盖所有移动模块。[S05](sources.md#s05)

**处理：** 新建明确 mobile contract/domain/host/renderer 原生测试门禁；逐项记录发现测试数。不要把“shared过了”写成“整个原生链通过”。

### G11 / P1：进度台账与能力门禁分开

**事实：** W索引/台账仍是原始 pending；当前源码已出现多项 W 对应实现。OC T18 为passed，但包含 provider_write/billing blocked-env、发布门禁exit2。[S24](sources.md#s24)[S26](sources.md#s26)

**处理：** 不改写历史事实；建立当前基线证据矩阵，按 profile 和子门禁更新。task reviewed ≠ feature enabled ≠ production releasable。

### G12 / P1：聚合页面与资源 API 不可凭设计假定已存在

**事实：** 已读 routes_workbench 中有单Run GET和启动POST，未见该文件声明统一执行列表或首页聚合GET。[S08](sources.md#s08)

**处理：** 先核对整个 router/DI，再复用已有列表或新增薄聚合读模型；`04-api-contract-proposal.md` 中的聚合路径是建议，不是现有 endpoint 保证。不得移动端按每个会话串行N+1扫描全库。

## 3. 对旧计划的更新方式

保持原 W/T 编号，修正“Create”与“Modify”的实际文件状态、迁移保留号、React/Expo版本、测试入口与路由差异。将 G01–G12 映射至对应 W 任务补充验收，不建立第二份互相矛盾的完成台账。

本稿可用来指导下一轮实施，但未对原仓库发起 commit、PR、issue、push 或部署。它不将任何未经验证的能力标为 accepted。
