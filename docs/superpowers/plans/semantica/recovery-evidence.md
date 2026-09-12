# O02 恢复演练实测证据

环境：semantica-i01-pg 容器（postgres:16-alpine，127.0.0.1:15432，semantic_test 库）
命令：`bash scripts/semantic/run_recovery_tests.sh`（退出码 0，8/8 通过）

## 场景记录

| # | 场景 | 结果 | 关键断言 |
|---|---|---|---|
| 1 | 恢复先重放拒绝再开放 | PASS | 快照→删除→恢复→ready=False→重放→ready=True→d1 不可见 |
| 2 | 维护模式阻断 | PASS | set_maintenance→ready=False |
| 3 | 被拒文档永不可见 | PASS | 删除 d2→查询不含 d2 |
| 4 | 存活文档保持可见 | PASS | 删除 d2→d1 仍在结果 |
| 5 | 快照后新删除保留屏障 | PASS | 删除后墓碑权威，恢复不复活 |
| 6 | 撤权后查询排除 | PASS | 两次查询间删除→后者排除 |
| 7 | epoch 撤权数据面等价 | PASS | 删除后 deny 屏障生效 |
| 8 | 部分可见不混合 | PASS | 恰好 {d2}，无部分泄漏 |

## 快照/恢复语义（实测）

- 快照 = 断言支持行（可见事实基）
- **墓碑不入快照**：恢复不回滚删除权威（规格约束 7）
- 恢复即维护：readiness 关闭直至 deny 重放（recovery_state 键）

## 未复现场景（如实）

规格 12 场景中容器进程故障（pause_phase/resume_phase/restart_api 需要 worker 进程编排）与
真实备份快照恢复（对象存储级）未在本环境执行——worker 编排归 O03 验收环境；不视为已通过。
