# 移动端 API 接入与增量契约

基线 `12737238...`。**A类是源码直接确认的路由/SDK形状；B类是本方案建议；C类是旧计划目标但本轮未完成当前路由核验。** 不把规划接口当作可立即调用的服务。

## 1. A 类：已读取的工作台路由

来源 `internal/router/routes_workbench.go`、工作台读handler、共享执行SDK。[S08](sources.md#s08)[S09](sources.md#s09)

| 方法 | `/api/v1` 下路径 | 使用位置 | 注意 |
| --- | --- | --- | --- |
| POST | `/workbench/executions` | 新建任务 | 依赖 admission 真接线；正文见下 |
| GET | `/workbench/executions/requests/:request_id` | ack丢失/重启对账 | pending/dispatching/admitted/rejected/unknown |
| GET | `/workbench/executions/:run_id` | 当前执行 | ownership-scoped；不要直接用用户传tenant信任范围 |
| GET | `/workbench/executions/:run_id/snapshot` | 初始化与游标过期恢复 | execution + watermark + events |
| GET | `/workbench/executions/:run_id/events` | 前台订阅 | Last-Event-ID；data形状需按G01修复 |
| GET | `/workbench/executions/:run_id/interactions` | 待处理交互 | 当前 handler/DTO与新SDK继续对齐 |
| POST | `/workbench/executions/interactions/:id/decisions` | 处理审批/问题 | 路由存在不等于本文已确认具体body |
| POST | `/workbench/executions/:run_id/commands` | cancel/steer | closed union + expected_revision |
| GET / POST | `/execution-targets` | 查看/管理目标 | 服务端具体授权，不因Viewer外层就推断所有用户可创建 |
| GET / DELETE | `/execution-targets/:id` | 详情/撤销 | 每项重新校验所有权/角色 |
| GET | `/execution-workspaces/:id` | 目标工作目录 | 不是SaaS Tenant |

`POST /workbench/executions/:run_id/source-events` 也在源码出现，但不列为移动端业务接口；按G09落实服务身份边界。

### 当前 StartExecutionInput

```ts
interface StartExecutionInput {
  request_id: string;
  session_id: string;
  agent_id: string;
  target_id: string;
  workspace_ref: string;
  text: string;
  budget_upper: number; // 当前SDK校验非负安全整数；具体准入仍由服务端决定
}
```

当前SDK没有在这个类型里声明 attachments/model/prompt_snapshot 等新字段，不可直接把线框所有表单值发到此body。会话创建、附件关联、知识/模型选择必须通过当前已有真实接口或经过版本化扩展；后端对最终执行配置重新授权。

```ts
type ExecutionCommandInput =
  | { action: 'cancel'; expected_revision: number }
  | { action: 'steer'; text: string; expected_revision: number };
```

SDK的get/snapshot/start/lookup/command通常解析 `{success:true,data:...}`。不要全局假定登录等其它API同一envelope；原生OIDC类型包含top-level token/user等字段。[S09](sources.md#s09)[S13](sources.md#s13)

## 2. B 类：建议新增/统一的薄读模型

以下路径/字段是设计草案，实施前检查全量现有API，已有等价能力优先复用。

| 建议接口 | 必要性 | 最小响应 |
| --- | --- | --- |
| GET `/workbench/bootstrap` | 一次获取当前用户、成员关系、所选空间能力、客户端最低协议 | profile、scope、membership、capabilities、limits、schema版本 |
| GET `/workbench/overview` | 工作台待处理/执行/最近产物，避免N+1 | counts、in_progress、pending_interactions、recent_artifacts、as_of |
| GET `/workbench/executions?status=...&cursor=...` | 可翻页的我的执行列表 | items、next_cursor、as_of；稳定排序与tie-breaker |
| GET `/workbench/inbox?cursor=...` | 应用收件箱，不能只靠推送历史 | items、unread_count、next_cursor；标记读需幂等操作 |

上述都是已有数据的聚合读模型，不创造第二套Run/审批/商业权威。初始只返回用户有权限的“我的”数据，团队视图显式授权后再引入，不以“同一个tenant”代表能看到所有人的私密会话。

能力建议仍复用现有三态：

```ts
type Capability = {
  state: 'supported' | 'unavailable' | 'forbidden';
  reason: string;
};
// supported 不豁免服务端再次验证；其它两态须提供可理解原因。
```

overview 返回字段应附display摘要而不暴露敏感审批参数；点详情重新拉完整可授权数据。稳定cursor不使用客户端时间作为唯一游标。

## 3. B 类：本地展示模型，不是新服务端 DTO

```ts
interface MobileRunView {
  runId: string;
  sessionId: string;
  runStatus: string;
  executionStatus: string;
  settlementStatus: string;
  revision: number;
  lastSyncedAt: string;
  stale: boolean;
  capabilities: Record<string, Capability>;
}
```

VM由既有DTO转换，保留未知状态的原值，未知状态不启用控制命令。展示文案例如“正在确认停止”由投影层解释，不擅自假定后端已有相同字符串枚举。

## 4. B 类：交互决定的设计要求

因为本轮没有完整读取workbench交互DTO，不在此伪造当前request body。实施者应先读当前 `internal/workbench/interaction.go` 和 command handler，再选择兼容扩展。

目标必须表达：interaction_id、interaction_kind、decision、expected_revision、幂等request_id；危险操作与服务端保存的输入摘要绑定；预算增加额仅用于budget类；问题回复仅用于question类。服务端拒绝类型不匹配、未知动作、任意RPC、旧revision与过期授权。

界面操作不是通用的 `{approve:true}`：

```text
工具写入：查看完整目标与参数 → 明确一次性批准 → 等待服务器接受 → 刷新Run
预算申请：显示当前上限/申请增额/审批范围 → 校验budget权限 → 决定
连接授权：打开受信系统浏览器 → 回跳一次性code → 查产品连接状态
问题：选择/输入文本 → 校验限制 → 提交 → 后端继续Run
```

## 5. C 类：原计划中的设备、通知与资源

原计划定义 PUT/DELETE `/mobile/devices/:id`、通知Outbox与供应商端口、会话附件上传、产物导入、语音会话控制。本文将这些作为W13–W16/W25–W31目标引用，未把所有具体路由宣称为已读取并可用。[S20](sources.md#s20)[S22](sources.md#s22)

原生OIDC SDK 已确认 `POST /auth/mobile/exchange`，body为code/state/redirect_uri/code_verifier；服务端当前route/DI与真实IdP回跳仍要核验。[S13](sources.md#s13)

## 6. 错误状态到 UX 的映射

| 情况 | 界面 | 是否自动重发业务写入 |
| --- | --- | --- |
| 网络未发出 | 保留草稿；明确离线 | 否，恢复后用户确认 |
| 发出后ACK未知 | 原request_id正在核实 | 否，先lookup |
| 401 | 单飞refresh；失败则登录 | 不能无界重复；按原请求身份处理 |
| 403 | 权限变化/联系管理员 | 否 |
| 404资源 | 不存在或无访问权限，不泄露元信息 | 否 |
| 409 revision | 已在另一端改变，重新拉详情 | 不沿用旧决定 |
| 409 cursor expired | 一致快照 + watermark恢复 | 不重启Run |
| capability unavailable | 保留说明，可更换已授权目标 | 不假装成功 |
| 结算处理中 | 结果可查看，费用标注未最终确认 | 不重复执行任务 |
| Provider结果unknown | 保留状态与人工/自动对账入口 | 不自动重复外写 |

## 7. 契约验收门槛

同一执行fixture从Go writer输出真实字节，原生SSE parser读取，SQLite端口提交并重启恢复；同一审批在两端并发只接受合法决定；SDK请求必须严格对应已注册路径与body；没有真实server/env时明确blocked-env，而不是把编译通过写成API可用。
