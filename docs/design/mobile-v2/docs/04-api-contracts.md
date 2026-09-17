# API与状态契约 · 开发冻结说明

日期2026-09-17。继承A/B/C分类。`contracts/mobile-proposal.ts` 提供可类型检查的提案，不是可直接替换仓库契约的已验收SDK。所有业务路由前缀 `/api/v1`。

## 1. A类：上一轮直接读取的路由

| 方法 | 路径 | 页面/任务 | 需要复验 |
|---|---|---|---|
| POST | /workbench/executions | M05/MX-006/MX-015 | admission、预算、request幂等、真实DI |
| GET | /workbench/executions/requests/:request_id | M08/MX-006 | pending/dispatching/admitted/rejected/unknown |
| GET | /workbench/executions/:run_id | M08/MX-018 | owner/tenant、3状态、revision |
| GET | /workbench/executions/:run_id/snapshot | M07/MX-012 | 一致watermark、完整投影 |
| GET | /workbench/executions/:run_id/events | M07/MX-004 | Last-Event-ID、G01 envelope与控制帧 |
| GET | /workbench/executions/:run_id/interactions | M09/MX-019 | typed当前交互，不含越权摘要 |
| POST | /workbench/executions/interactions/:id/decisions | M09/MX-005 | 具体body须当前handler核对，不照提案盲发 |
| POST | /workbench/executions/:run_id/commands | M08/MX-018 | cancel/steer closed union与revision |
| GET/POST | /execution-targets | M15/MX-026 | 普通可见性不代表管理权限 |
| GET/DELETE | /execution-targets/:id | M15/MX-026 | 单目标授权/撤销 |
| GET | /execution-workspaces/:id | M15/MX-026 | 文件工作区不是Tenant |

source-events不是移动客户端业务API；服务身份入口与写入前绑定校验归MX-027。已有SDK中原生OIDC exchange路径可复验；当前部署是否装配、真实IdP链路是否通过不能由SDK存在推定。

### Start请求保持当前7字段

```json
{"request_id":"req-demo","session_id":"session-demo","agent_id":"agent-demo","target_id":"target-demo","workspace_ref":"workspace-demo","text":"整理本周反馈","budget_upper":100}
```

不得增加未经服务端注册的attachments/model/knowledge字段。任务准备需先复用现有会话配置和附件关联，或按版本化扩展另冻契约。ACK：`{success:true,data:{run_id,request_id,status}}`，SDK验证返回request_id与原请求一致。其他认证API不强制套同一envelope。

## 2. B类：聚合读模型提案

| 方法/路径 | 最小请求 | 最小响应 | 安全/一致性 |
|---|---|---|---|
| GET /workbench/bootstrap | 当前认证，选定空间上下文 | actor/memberships/selected_tenant_id/capabilities/limits/protocol/as_of | 不信任客户端tenant声明；选定空间须成员校验 |
| GET /workbench/overview | 无额外业务写字段 | counts/in_progress/pending_interactions/recent_artifacts/as_of | 每资源授权，卡片摘要不含秘密；不N+1 |
| GET /workbench/executions | cursor/status/q/page_size | items/next_cursor/as_of | 稳定分页，默认20最大50；仅当前actor可见 |
| GET /workbench/inbox | cursor/filter/page_size | items/unread_count/next_cursor/as_of | 通知投影非审批权威 |
| POST /workbench/inbox/read | notification_ids + request_id | 已读结果 | 幂等；不会执行通知所描述操作 |

上述最后一个已读路径也是新增建议，非已读A类接口。复用现有等价接口优先。Cursor建议携带排序边界、过滤摘要、空间/actor绑定与过期信息并签名；不能拿另一空间cursor遍历数据。角色、资源访问和snapshot范围须经当前服务端再授权。

## 3. B类：新版事件流示例

```text
id: 42
event: text.delta
data: {"schema_version":1,"run_id":"run-demo","attempt_id":"attempt-demo","seq":42,"type":"text.delta","occurred_at":"2026-09-17T08:00:00Z","payload":{"message_id":"m-demo","text":"已完成检索"}}

```

新协议所有关键字段来自服务端，不由客户端凭空补造。未知type保留payload并安全降级；未知schema_version拒绝控制路径并要求升级。seq范围≤Number.MAX_SAFE_INTEGER；大小、嵌套层数和单帧缓存上限从协议冻结时明确，建议帧上限1MiB，超限断流并记录协议错误，不能无界累加内存。

控制帧可以采用显式event `control`，data为`StreamControl`，不带业务id；这是版本化提案，须两端一起实现。心跳也可保持标准注释行。cursor_expired握手返回409时先快照，已经进入流后用控制帧通知重建；两种路径必须在parser/controller测试中分别存在。

## 4. B类：交互决定提案

```json
{"kind":"tool","decision":"approve","request_id":"decision-demo","expected_revision":4,"content_digest":"server-frozen-digest"}
```

digest是服务端冻结对象摘要，不由客户端以当前输入替代。服务端校验interaction id、tenant/actor、kind、状态、revision、digest、有效期和连接授权版本后CAS一次落地。重复request_id同内容返回原结果，不同内容冲突。budget approve单独增加authorized_upper；question answer单独带answer；connection authorize返回或关联可信授权尝试，不直接标为已连接。

ACK仅表达决定已记录。客户端据返回值或重读更新交互，并继续订阅Run；不乐观生成外部写入成功的消息。预算/权限/工具三域不复用一个approve布尔。

## 5. C类：依赖原计划的能力

设备注册、推送供应商、产物访问、临时附件、语音令牌、Provider授权URL等必须先核对当前router/DTO/DI。原文W13–W16/W25–W31/T13/T18是实现或验收依赖，不能仅凭设计路径在App里发请求。资源字段和目标路径由MX-001更新的契约矩阵锁定。

## 6. 错误协议与显示

| 错误 | 行为 | 自动业务重发 |
|---|---|---|
| 未发送即离线 | 保存草稿，禁写 | 否 |
| 已发出但ACK丢失 | 持久同一request_id，lookup | 先对账，不换ID |
| 401 | 单飞刷新一次，失败回登录 | 受幂等与scope约束 |
| 403/404 | 隐藏敏感字段，统一无法访问 | 否 |
| 409 revision | 重新拉当前交互，用户重新确认 | 否 |
| 409 input_hash | 提示当前ID与输入冲突 | 否 |
| 409 cursor_expired | 原子快照恢复与watermark | 不重启Run |
| 429 | 根据Retry-After退避，保留用户草稿 | 仅受控读取/已证明幂等请求 |
| 413/unsupported_file | 文件限制说明，保留文字 | 否 |
| Provider unknown | 保留不确定观察，独立对账 | 禁止重写 |
| settlement pending | 显示非最终消费，允许有权成果读取 | 不重复执行 |

## 7. 冻结与回归规则

每个新增DTO至少一份共享fixture，Go响应与TS parser往返校验，未知字段/缺字段/越权/并发分别测试。若需要改旧schema，以MX-003冻结版本和兼容窗口为前置，不在UI随意any断言。任务文档里的endpoint_proposed不是可上线事实；在实际repo中编写route/DI测试和受控集成证据后才标verified。
