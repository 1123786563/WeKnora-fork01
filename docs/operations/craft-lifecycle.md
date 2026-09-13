# Craft 生命周期与资源回收（O03）

Craft 作品依赖三类物理资源：会话绑定的沙箱（Sandbox）、不可变产物版本（Version）与恢复快照（Snapshot）、受控存储里的对象（object）。本文说明这些资源各自的保留与回收规则、数据保证、配置默认值，以及排障路径。

核心约束（总计划）：**活跃（Active）、结果未知（Unknown）、决策待定（DecisionPending）、被引用（Referenced）的资源一律不回收**；代际锁与引用保护始终有效。回收决策永远按真实运行关系查询，不以调用方布尔作为唯一事实。

## 纯规则（internal/craft/lifecycle.go）

| 接口 | 语义 |
| --- | --- |
| `ResourceState{Active, Unknown, DecisionPending, Referenced bool; EligibleAt time.Time}` | 一个资源的回收判定输入，四个保护位 + 最早可回收时刻 |
| `CanDeleteResource(s, now)` | 四个保护位任一为真、或资格窗口未满，都不可回收 |
| `SandboxDormancy(idleFor, snapshotVerified, policy)` | 休眠判定：空闲超过阈值 **且** 已验证完整快照才可暂停；恢复一律是显式重启 |
| `ProviderTTLRisk(expiresAt, extendable, now, policy)` | provider TTL 无法延长且临近耗尽（或未知）时，提前给出 workspace 风险 |
| `LifecycleEventKey / SandboxDwell / BytesDay` | 驻留与存储计量；事件按内容身份去重，重投不重复计数 |
| `QuotaAllows(action, sandboxOver, storageOver)` | 配额超限只拒绝 **新增** 沙箱；下载与清理永远可用 |

## 休眠（dormancy）、重建（rebuild）与删除（deletion）的数据保证

### 休眠 = 暂停，不是删除

- 条件：沙箱空闲超过 `SandboxIdleThreshold`（默认 **30 分钟**）**并且** 存在一帧已验证完整的 C05 快照（静默捕获、files+session 双摘要齐全、可 `CanRestore`）。没有验证快照的空闲沙箱保持运行，直到快照落成——暂停唯一副本是不允许的。
- 保证：休眠只影响沙箱这个运行载体。**产物版本与快照对象永不因沙箱 TTL 被删除**，它们的保留跟随 session 现有政策（session 删除时级联）。
- 不承诺：**休眠不保存进程内存快照**。暂停杀掉沙箱内全部进程状态；恢复是明确的重启，中断中的长任务需要重新执行。

### 重建 = 新代际，不覆盖旧绑定

- 快照恢复（C05 Restore）在新代际里物化并验证文件与会话链后，才以一次 CAS 换绑；进程在换绑前死亡则旧绑定原样保留。
- 回收侧的对称保护：清理器清除绑定时必须 **generation 匹配**。工作区若已重绑到新代际（如恢复/重建产生 g1），旧代际（g0）的清理只删除旧沙箱实例，绝不触碰新绑定或新 workspace 行。

### 删除 = 先 tombstone，再核对，后删除

- 会话删除入口 `TombstoneSession` 顺序固定：**先**写 tombstone（状态 `deleting`，即刻阻断新 dispatch 与 restore），**再**取消活跃 Run，**最后**处理资源引用。
- Sweep 对每个候选资源：先持该会话的沙箱生命周期锁 → 重新查询活跃 Run / 未决 delegation（结果未知）/ 待决交互（craft_interactions pending）/ 引用关系 → `CanDeleteResource` 放行才 CAS 标 `deleting` → provider 删除 → generation 匹配清除绑定 → 终态 `deleted`。
- 结果未知的远端任务：**不回收，保留风险记录**（状态 `risk`，reason 列出任务 id），直到结果被证明。
- provider 删除失败：状态 `failed`，attempts/last_error 保留为重试记录，退避（默认 5 分钟）后下一批重试。

## 孤儿对象（orphan objects）

- 上传关联未完成等产生的无主对象：先记录 **candidate**，资格窗口（默认 **24 小时**）内不删除。
- 窗口到期后 Sweep 重新查引用（craft_version_files / craft_snapshot_objects / craft_workspace_inputs 三处真实清单）；仍被引用 → `kept` 并记录原因；无引用才经对象删除端口删除。

## 配置默认值（初始建议值，均可配置）

| 配置 | 默认 | 说明 |
| --- | --- | --- |
| `SandboxIdleThreshold` | 30m | 休眠所需空闲时长 |
| `OrphanCandidateWindow` | 24h | 孤儿对象候选窗口 |
| `SweepBatchSize` | 100 | Sweep 每批上限（`Sweep(ctx, 0)` 即取默认） |
| `SweepRetryBackoff` | 5m | 删除失败重试退避 |
| `CRAFT_LIFECYCLE_SWEEP_INTERVAL` | 10m | 周期清理间隔（容器装配） |
| `CRAFT_LIFECYCLE_SWEEP_BATCH` | 100 | 每批数量（容器装配） |
| `CRAFT_LIFECYCLE_SWEEP_DISABLED` | false | 关闭周期清理（tombstone 与守卫仍可用） |

周期清理由 `container.StartCraftLifecycleSweep` 注册到 ResourceCleaner，单批串行不重叠，关停时优雅停止。

## 计量与配额

- 事件（sandbox_start / sandbox_stop / storage_bytes）以内容身份为去重键写入 `craft_lifecycle_events`，**重投递只计一次**。
- 驻留时长按每个沙箱真实 start→stop 配对求和；存储按 bytes-day 积分（尾部观测计到当前时刻）。
- 配额超限：`AdmitNewSandbox` 拒绝新沙箱（`ErrCraftQuotaExceeded`）；**既有授权下载与清理不受影响**——超限的工作区始终可以减负。
- provider TTL 无法延长时，`WorkspaceTTLRisk` 提前返回风险原因（TTL 未知同样视为风险）：**不假设容器永久存活**。

## 排障速查

- "为什么没回收"：查 `craft_lifecycle_states.reason`——kept/risk 行写明了保护原因（活跃 Run / 未知任务 / 待决交互 / 会话仍存活 / 窗口未满）。
- "删除一直失败"：`state=failed` 行的 attempts 与 last_error 是重试记录；退避后自动重试。
- "会话删了沙箱还在"：优先确认是否存在结果未知的 delegation（risk 行）；其次看失败重试。
- worker 竞争：快照保存/恢复与回收共用同一把会话生命周期锁，且 tombstone（`deleting`）状态下 `GuardDispatch`/`GuardRestore` 直接拒绝，二者不可能交错破坏数据。

## 已知边界

- 本任务基线（de94676e）不依赖全栈启动验证（C06 已在另一分支修复 dig 启动 panic，尚未合入）；本文所述行为以单元/装配层测试为准。
- Redis 侧绑定若因 session 硬删除而残留，由 sandbox 包既有 `ReapOrphanSandboxes`（按租户元数据枚举）兜底，不在本 sweep 范围。
