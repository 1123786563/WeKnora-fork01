# tRPC 原生 Agent 迁移证据台账

| Phase / task | Dependencies | Owner | Commit | Implementation | Spec review | Quality review | Verification layer | Command | Log path | Environment | Gaps / next owner |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P0-1 固定功能基线与证据台账 | 已确认规格、当前源码、活跃客户端 manifests | P0 Task 1 implementer | `da16fcf5`, `a64d2640`, `df82068b` | passed（文档覆盖审查；round 2 fix committed） | round 1 re-review finding addressed; re-review pending | round 1 re-review finding addressed; re-review pending | static inventory / TSV schema | `awk -F '\t' ... features.tsv`; `git diff --check` | `.superpowers/sdd/2026-09-19-trpc-native-agent-p0/task-1-report.md` | local isolated worktree; no service/provider/device started | P2 SDK probe、P3/P7 恢复、P1 数据权威、P4 能力接入、P5 客户端、P6/P7 archive/cutover |
| P0-2 原生 Runner / Session 探针 | P0-1 feature IDs、固定 SDK v1.10.0 | pending assignment | — | pending | not started | not started | deterministic SDK unit/race; source contract | `GOWORK=off go test -race ./internal/agent/nativeprobe -count=1 -v` | `docs/superpowers/plans/trpc-native/sdk-probes.md` | SDK module cache; no real provider required | P3 must add provider proof; Session key test does not prove authorization/database isolation |
| P0-3 恢复证据与运行语义 | P0-1、P0-2 runner evidence | P0 Task 3 implementer | pending Task 3 commit | complete（基线记录；无生产代码改动） | pending | pending | deterministic SQLite SDK probe; runtime race policy; isolated SQLite process/fault harness | `GOWORK=off go test ./internal/agent/trpc -run 'TestCheckpointProbe|TestSQLiteSaverPendingWrites' -count=1 -v`; `GOWORK=off go test -race ./internal/agent/runtime -count=1 -v`; `GOWORK=off go test -json ./internal/agent/recoverytest -count=1` | `docs/superpowers/plans/trpc-native/evidence/task-3-recovery-2026-09-19.md`; `/tmp/weknora-native-recovery-p0.json` (temporary) | PostgreSQL matrix/contention and standalone real-provider case are `blocked-env`: `TRPC_RECOVERY_PG_DSN` / `TRPC_RECOVERY_GRAPH_PROVIDER` unset | six recovery gaps in `recovery-gaps.md`; unknown non-idempotent calls stay `waiting_user`; v1.10.0 native Runner product execution remains blocked by P0-2 race gate; P1/P2/P3/P7 own remediation and acceptance |
| P0-4 SDK 能力与扩展决策 | P0-1 plus P0-2/P0-3 results | pending assignment | — | pending | not started | not started | fixed-version source plus referenced behavior evidence | planned matrix review | `sdk-capabilities.tsv`, `interfaces.md` | fixed v1.10.0 source; no floating main | source-only cannot unlock implementation; decide native/extension/upgrade/spec revision |
| P0-5 P0 gate and P1/P2 plans | P0-1 through P0-4 | pending assignment | — | pending | not started | not started | decision and planning review | planned P0 matrix check | `p0-decision.md` | evidence supplied by prior tasks | critical blocked/incompatible items produce no-go |

## 证据规则

* `passed` 仅用于工作包完成条件；本任务的完成条件是文档覆盖审查。
* `unverified` 表示尚无对应行为证据，不因源码、Mock、SQLite、构建或单元测试而改为运行通过。
* `blocked-env` 仅在后续运行实际缺少凭据、服务、数据库或设备时使用；当前没有把未执行的项目伪装成环境阻塞。
* 每个 TSV 行都有目标责任阶段；新注册入口或消费者必须先补行，再改变迁移结论。
