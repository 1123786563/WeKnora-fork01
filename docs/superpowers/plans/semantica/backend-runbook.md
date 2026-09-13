# 语义后端运行手册（W03）

## 状态模型

每个 (tenant, KB) 一行 `semantic_backend_states`：
- `desired_backend`：意图（native/semantic）——仅由产品设置保存或管理 API 写入
- `active_backend`：当前生效后端——仅由 Promote/Rollback 的 CAS 切换
- `active_generation`：语义侧当前发布 generation（I03 CAS 发布）
- `native_checkpoint` / `semantic_checkpoint`：两侧已追到的源 revision 高水位
- `last_error`：最近一次管理动作失败原因（O01 管理挂载后写入——今日无写入者）

## 影子构建（SetDesired）

`SetDesired(scope, "semantic")` **只写意图**：记录 desired（影子构建的消费编排归
O02 接线——今日尚无 desired 消费者），active 保持 native，正式查询不受影响。部署镜像
**永不**自动切换 active——切换必须经管理 API 显式动作（管理 API 挂载归 O01）。

## 正式切换（Promote）

前置检查（人工核对，管理 API 不代劳）：
1. 影子 manifest 完整（I03 complete=TRUE）
2. capability 满足（Q01 模式支持）
3. 验收 policy 通过（抽样问答一致性）
4. 当前源版本 = 影子构建基线

`Promote(scope, expectedActiveGeneration)`：CAS（active_backend=native AND
active_generation=expected——纯 I03 重发布围栏，CAS 不改写 generation）→ semantic。
并发 Promote 恰一胜；失败返回 `ErrPromotionRejected`。切换后旧请求沿 read lease
完成（read-lease 集成归 O02），且仍受最新 deny 约束。

## 回滚（Rollback）

**先补齐再切换**：native 检查点必须 ≥ 当前源最高 revision，否则
`ErrNativeCatchupRequired`（绝不静默回旧图）。追赶内容：切换期间的文档更新/删除
（同一 outbox 输入；**墓碑计入**——删除本身是 native 必须重放的变更）；追赶按源
MAX(revision) 高水位比较（清单逐项核对归 O02 消费者），CAS semantic→native；CAS 后
复检源 revision（TOCTOU 护栏——源已移动则报 ErrRollbackRejected 提示等待 outbox）。
native 不可用时保持语义后端或**明确禁用图能力**——不静默回退。

## 失败处理

| 症状 | 处置 |
|---|---|
| Promote 拒绝 | 核对 expected generation 与当前 active_generation；重取后重试 |
| Rollback 要求追赶 | 等待 native 影子消费 outbox 至检查点 ≥ 源 revision |
| 墓碑冲突 | I04 删除屏障不可跳过；核对 deny 清单后重放 |
| CAS 反复失败 | 检查并发管理动作；一次只允许一个切换流程（Promote 失败=ErrPromotionRejected；Rollback 失败=ErrRollbackRejected；未提升即回滚=ErrNotActiveSemantic） |

## 禁止事项

- 部署/镜像启动时自动切换 active
- 跳过 native 追回直接 Rollback
- 删除墓碑以"解决"回滚冲突
