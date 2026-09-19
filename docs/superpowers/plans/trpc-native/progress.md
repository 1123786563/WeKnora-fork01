# tRPC 原生 Agent 迁移证据台账

| Task | 状态 | 交付 | 证据结论 | 阻塞 / 下一责任 |
| --- | --- | --- | --- | --- |
| P0-1 固定功能基线与证据台账 | passed（覆盖审查） | `baseline.md`、`features.tsv`、本台账 | 已将当前注册/调用入口、开关、失败状态、数据库与消费者逐项对应到稳定 ID。没有运行验收被标为 passed。 | P2 SDK probe、P3/P7 恢复、P1 数据权威、P4 能力接入、P5 客户端验收 |
| P0-2 原生 Runner / Session 探针 | pending | `sdk-probes.md`、`internal/agent/nativeprobe/runner_test.go` | 必须固定 v1.10.0 API 并证明工具往返及 SDK session key space；不能证明授权或生产数据库隔离。 | SDK module cache / Go race test |
| P0-3 恢复证据与运行语义 | pending | `recovery-gaps.md` | 必须分别检查 checkpoint、pending writes、未知工具、真实 Provider/数据库状态。 | provider / PostgreSQL 环境可标 `blocked-env` |
| P0-4 SDK 能力与扩展决策 | pending | `sdk-capabilities.tsv`、`interfaces.md` | source-only 不能升级为 verified；每个缺口指定原生、最小扩展、固定升级或规格修订。 | P0-2/3 结果 |
| P0-5 P0 门槛与 P1/P2 计划 | pending | `p0-decision.md` 与后续计划 | 关键 blocked、incompatible 或无恢复组合时 no-go。 | P0-1 至 P0-4 完成 |

## 证据规则

* `passed` 仅用于工作包完成条件；本任务的完成条件是文档覆盖审查。
* `unverified` 表示尚无对应行为证据，不因源码、Mock、SQLite、构建或单元测试而改为运行通过。
* `blocked-env` 仅在后续运行实际缺少凭据、服务、数据库或设备时使用；当前没有把未执行的项目伪装成环境阻塞。
* 每个 TSV 行都有目标责任阶段；新注册入口或消费者必须先补行，再改变迁移结论。
