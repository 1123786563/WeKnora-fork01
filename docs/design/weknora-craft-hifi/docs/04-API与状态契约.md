# Craft API、命令与状态契约

本文件区分“上轮已见接口”“本轮已复核的代码语义”“目标内部端口”。不把原型事件名或 Typescript 类型直接当作当前服务端协议。

## 1. 已有接口清单与适配策略

前缀统一 `/api/v1`。来自上轮 handler/routes 审阅 [R00]，本轮没有重新逐个运行请求。`sid/rid/vid` 是文档占位参数名称，不要求重命名服务端。

| 操作 | 接口 | 适配规则 |
|---|---|---|
| 创建/列出作品会话 | POST/GET `/craft/sessions` | 复用认证与游标，创建幂等 |
| 初始权威状态 | GET `/sessions/{sid}/craft` | 快照、工作区、活动run与游标按现有DTO解析 |
| 关联输入 | POST `/sessions/{sid}/craft/inputs` | 上传完成后绑定；不可把选择文件当上传成功 |
| 提交修改 | POST `/sessions/{sid}/craft/runs` | 输入、知识、基线、幂等一并保留 |
| 事件流 | GET `/sessions/{sid}/runs/{rid}/events?after=N` | 全量主Run序号，断线续读 |
| 版本列表/详情 | GET `/sessions/{sid}/craft/versions`、`.../{vid}` | 不因版本失效偷偷替换成最新 |
| 版本文件 | GET `/sessions/{sid}/craft/versions/{vid}/files/*file_path` | 匹配manifest，path安全校验 |
| 预览授权 | POST `/sessions/{sid}/craft/versions/{vid}/preview` | 独立预览origin、带版本范围与期限 |
| 恢复快照列表 | GET `/sessions/{sid}/craft/snapshots` | 可恢复性≠有可下载文件 |
| 恢复 | POST `/sessions/{sid}/craft/restore` | 权限、互斥、CAS、幂等 |
| 用量与诊断 | GET `/sessions/{sid}/craft/usage` | 未知不是0；不能另造Credits |

取消、问题/审批路径必须由 T001 从既有主运行/交互路由绑定，本轮未读取完整路由表，故不编造 `/craft/approve` 等路径。文件ZIP、公开分享、知识回存、新建Project、编辑器协作也不伪装成已存在接口。

## 2. 目标命令端口

完整类型位于 `examples/command-bridge.types.ts`。这是 app/controller/views 之间的内部目标契约，底层 DTO 由 `packages/api-client` 适配。实现前核对字段实际名字与是否已经支持。

```text
submitDraft(DraftIntent) → RunReceipt
cancelRun(sessionId, runId, requestId) → accepted
answerQuestion(InteractionIdentity + answers) → DecisionReceipt
decidePermission(InteractionIdentity + allow_once/deny) → DecisionReceipt
restoreVersion(RestoreIntent) → RestoredWorkspace
refreshPreview(sessionId, versionId) → PreviewGrant
```

### 2.1 DraftIntent 语义

| 字段 | 含义 | 规则 |
|---|---|---|
| requestId | 一次用户提交意图 | 同意图网络重试复用；编辑后新建 |
| sessionId | 目标作品会话 | 必须在已授权scope中 |
| text | 文本目标 | trim后非空；长度按服务能力 |
| kind | 作品类型 | 服务器再次核验允许列表 |
| inputs | 上传引用/hash/citation ID | 只能提交本scope可读、已就绪引用 |
| knowledge | 选择的资源/可选revision | 授权即时解析；资源内容不是权限指令 |
| baseVersionId | 当前编辑基线 | 不是预览选中的历史版本 |
| expectedWorkspaceRevision | 用户确认时的工作区修订 | CAS失败由用户重新确认 |

不加入客户端可自证的 tenant_id/user_id、管理员权限、任意sandbox URL、模型密钥或计费余额。请求体也不直接携带任意文件系统绝对路径。

### 2.2 幂等与失败

同一scope和requestId且规范化payload摘要相同：返回既有receipt。相同键而payload不同：冲突，不复用旧结果“凑成功”。规范化必须定义字段排序、引用序、空字段处理；不要用未经约定的字符串化恰巧作为哈希契约。

提交响应丢失时 UI 保留 requestId 与冻结意图，先按现有幂等入口查询/重试。Controller 的 SSE 重连绝不触发这个提交重试。刷新后无已保存意图时，应先读服务器活动run，不根据浏览器草稿推断需要执行。

## 3. 服务器已有领域字段

本轮 `internal/craft/contracts.go` 已复核 [R03]：

```text
Scope: TenantID / UserID / SessionID （由服务器取得）
Workspace: ID / Scope / SandboxID / Generation / OpenCodeSessionID /
           RuntimeDigest / Revision
Task: ID / ToolCallID / Prompt / PromptMessageID / RequestHash /
      Scope / Fence / WorkspaceID / Inputs / SkillDigests / Deadline
Version: ID / WorkspaceID / RunID / Kind / Files / Checks
File: Path / Ref / SHA256 / MIME / Bytes
Check: Name / Status / Detail （passed|failed|not_run）
Result: TaskID / Status / Summary / Files / Checks
        Status = succeeded|failed|canceled|unknown
```

上述是Go领域结构，不保证序列化后字段同名。`Scope`是授权维度而不是持久化主键替代品。`unknown`保留未解决ToolCall；不能转换为completed来满足UI。

## 4. 事件与投影契约

本轮Controller已有：权威快照后订阅；重复seq丢弃；缺号/游标过期重读；原子替换；scope epoch；1/2/4/8/15秒封顶退避并带jitter [R04]。新增接入必须保留这些语义。

| 情况 | 行为 | 不能做 |
|---|---|---|
| 首次进入 | 读快照，按last_seq订阅 | 先订阅后猜初始状态 |
| 收到重复seq | 不再改变消息/工具/用量 | 新随机消息ID |
| 精确下一seq | 归约并推进游标 | 让UI自行改变主终态 |
| seq跳跃 | 重同步快照 | 丢事件后接着拼文本 |
| 非Craft事件 | 仍推进完整Run游标 | 先过滤再制造缺号 |
| 新generation | 清理旧代际投影和订阅 | 旧worker继续发布 |
| 切scope/session | abort并更新epoch | 旧请求覆盖新页面 |
| OpenCode idle/finish | 只更新委派观察 | 主Run直接成功 |
| 预览到期 | 重取该版本票据 | 新建Run或换最新版本 |

不要为接入assistant-ui先改造一套新WebSocket/AG-UI协议。确有消费者需求才追加无事实所有权的协议投影。

## 5. UI状态不是新后端状态机

下面标签是产品投影概念；映射到既有后端枚举，而不是直接增加同名服务端状态。

| 维度 | 显示概念 | 发送 | 查看旧版本 |
|---|---|---|---|
| 连接 | 同步中/已连接/中断 | 未确认最新状态时禁发 | 已授权静态交付可保留 |
| 主Run | 排队/执行/等待/取消中/终态 | 活动写任务期间禁新写 | 可以 |
| 委派 | 提交/执行/核对/不明 | 不明不能另起有副作用任务 | 可以 |
| 权限 | 可写/只读/拒绝 | 只读禁发，拒绝不读取 | 按资源读取权限 |
| 版本 | 无交付/发布/检查未通过 | 与运行独立 | 仅已发布可下载 |
| 预览 | 取票/有效/过期/403 | 不影响原任务执行 | 按票据与ACL |
| 输入 | 上传/处理/可用/失败 | 必要输入未就绪禁发 | 无影响 |

### 5.1 查看/恢复转移

```text
view(v1), base=v3 -> view=v1, base=v3, revision不变, versions不变
restore(v1, revision=7) -> base=v1, revision=8, versions不变
submit(edit, base=v1, revision=8) -> new run
publish -> new immutable v4, base=v4, v1文件/hash保持不变
```

恢复缺快照、无write、活动任务、stale revision均失败。恢复事务成功但响应丢失时，以requestId取回相同恢复结果，不再次提升revision。永久失败和可重试失败分别记录。

## 6. 人工交互合同

许可请求绑定 `interactionId + runId + requestRevision + payloadHash + scope/fence`。界面传回的是对指定请求的决定，不是任意新的可执行命令。

```text
pending -> submitting -> recorded -> delivery_pending -> delivered
                          |              |
                          |              -> unknown（核对，不重复批准）
                          -> expired/rejected（权威服务决定）
```

问题answer和许可decision分接口语义。请求内容变化后旧批准失效。取消先完成后到达的批准不得复活run。重复提交同一决定返回原receipt；冲突决定要求人工核对而非覆盖历史。

## 7. 错误呈现映射（目标设计）

已有领域err包括InvalidInput、Forbidden、NotFound、Conflict、Busy、Unknown、Unsupported [R03]。下表是目标响应语义；HTTP实际编码应由T001/T002核对后端现状，不盲改。

| 语义 | 期望HTTP类别 | UI |
|---|---|---|
| 输入无效 | 400/422宿主约定 | 字段错误，保留草稿，不自动重试 |
| 无读取权限 | 403或隐匿404 | 不显示受限内容 |
| 活动工作槽/修订冲突 | 409 | 重新同步，附着现有任务或再次确认 |
| 游标/票据过期 | 410或既有业务码 | 重读快照/只续票据 |
| 文件过大 | 413或既有校验码 | 文件级提示 |
| 限流/预算拒绝 | 429或既有商业错误码 | 展示恢复条件，不换模型绕过 |
| 不支持类型 | 已有Unsupported映射 | 禁用相应类型，保留目标 |
| 依赖暂不可用 | 503或既有码 | 有界重试；不明副作用先核对 |

错误日志记录trace、scope脱敏ID、session/run/task与阶段，不记录鉴权secret、票据、原文敏感数据。

## 8. 能力与前端按钮

目标 `ViewCapabilities` 从服务器/宿主权限映射，包含canRead、canWrite、allowedKinds及上传限制。没有独立能力API时由既有会话/配置装配，不凭空新增接口。界面隐藏仅提升体验，服务器仍完整校验。

发布与模板展示也按能力绑定。文件ZIP、生产分享、原文复制等未接后端能力时不提供伪可用按钮。HTML原型明确只导出示例HTML，不伪造Office文件。
