# 恢复手册（O02）

## 快照恢复四步（RecoveryHarness 语义，如实）

> **如实边界**：演练中的"重放当前拒绝"从**服务侧墓碑权威**重推导可见性（断言行按墓碑置不可见）。生产接线中该步应由 Go 权威重放 deny/epoch（I04 屏障 + A01 epoch）——Go 侧重放编排归 O03 验收环境；维护模式在演练夹具中拒绝查询（readiness 门），生产 readiness 门已就绪（O01 server_entry /ready fail-closed）。

```
restore_persistent_artifacts(snapshot)   # 断言支持行恢复；墓碑不回滚
set_query_ready(False)                    # 恢复即维护：拒绝查询
replay_denials_from_business_authority()  # Go 权威重放当前 deny/epoch
verify_no_denied_sources_visible()        # 验证删除内容不可见
set_query_ready(True)                     # 验证后开放
```

## 失败退出与安全重试

- 演练脚本 PG 不可达时**退出非零**（不 skip）：先启动隔离环境再重跑
- 恢复中途失败：保持维护模式重试四步；不跳过 verify 步骤

## 孤儿清理

- 对象/向量孤儿：仅清理不被任何 manifest 或有效 read lease 引用的产物（I03 GC 保护）
- 墓碑：**永不清理**直至 GC 保留窗口（I04 sweep 守卫：清单闭包检查）

## 禁用语义能力 / native 追赶

- native 不可用：保持语义后端或明确禁用图能力（W03 Rollback 语义）
- native 追赶条件：native_checkpoint >= 源最高 revision（墓碑计入）
