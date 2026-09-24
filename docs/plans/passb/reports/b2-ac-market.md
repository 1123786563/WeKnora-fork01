# b2-ac-market（25c Marketplace）实施报告 — Pass B

> 状态：**T1 完成**（T2–T5 未开始）。分支 `codex/passb-b2-ac-market`；对齐后任务基线 `c0ddf768a`（见下）。

## T1（25c.1）基线锚定与前置门核验 — done @ 本 commit

### 基线对齐（Ruling 2026-09-24-WAVE-DEP-BASELINE）

- 派发基线 `dfcec6067` 不含 25a/25b 模块树（FACADES 核验失败 + `merge-base --is-ancestor` 实测 + 当时仓库不存在含两者的 commit），升级后协调者裁定选项 A：先对齐后实施。
- merge 25a `938087598` → `ad53c2185`（零冲突）；merge 25b `5ed64d324` → `c0ddf768a`（唯一冲突 `docs/architecture/passb/execution-ledger.md` 纯追加型，两侧条目全保留，机械解决）。
- **基线对齐发生于 IB2 之前，IB2 正式职责不变**（非集成合并，未触碰 `codex/passb-integration`）。
- **对齐后基线（T2–T5 的 PASSB_BASE_SHA）= `c0ddf768abc8e0f61e643bbdfba41a1606251fc6`**。

### 已运行命令与结果（全部本会话实跑）

| 命令 | 退出码 | 输出摘要 |
|---|---|---|
| 前置门面链（计划字面式 `grep -q "CatalogInstallResult = "`） | 1 | FACADES-MISSING——gofmt 对齐多空格致单空格模式不匹配（字面模式过严，非门面缺失） |
| 等价链 `grep -Eq "CatalogInstallResult[[:space:]]+="` + 两 `test -f` | 0 | FACADES-OK（别名在宿主残差 `tenant_skill_service.go:48`） |
| `go build ./...` | 0 | 仅 cmd/desktop、cmd/server 两条 ld duplicate-libraries 告警 |
| `go test -count=1 ./internal/application/repository -run 'TestAgentMarketplace|TestExpertInstall|TestPublishedExpertRepo|TestPublishedSkillRepo'` | 0 | `ok 14.425s`（21 用例，-v 计数） |
| `go test -count=1 ./internal/application/service -run 'TestAgentMarketplaceSubmit|…七段原式'` | 0 | `ok 2.446s`（17 用例，-v 计数） |
| `go test -count=1 ./internal/router -run 'TestTenantAgentMarketplaceLifecycleAndAuthorization'` | 0 | `ok 2.136s`（1 用例） |
| `go test -count=1 ./internal/modules/agentcatalog/...` | 0 | 根 no test files；handler/repository/service 三包全 ok（25a/25b 随迁测试经对齐全绿） |
| `make check-backend-architecture` | 0 | `total=633 | redis=23 lite=23 | hooks=58 | modules=16`、`OK (0 violations)`——零漂移 |
| `make verify-module-moves` | 0 | `OK (16 manifests verified)` |

### T1 自查

- 39 用例（21+17+1）`-v` 旧实现基线清单已落 `docs/architecture/evidence/passb/b2-ac-market.md` §3（T5 双跑比对侧）。
- 与计划前置条件 4 的 e586552d1 记录相比：测试结果同为全绿；用例计数一致（21/17/1）；耗时差异属机器负载。模块包由 `[no test files]` 变为三包全 ok——系对齐 merge 带入 25a/25b 随迁测试，符合预期。
- 台账冲突解决为裁定 §3 授权的机械合并，无冻结签名/注册/语义取舍。

### 遗留

- T2–T5 未开始（后续任务按计划 §6 顺序执行）。
- 逐字 FACADES 命令链与 gofmt 排版的不匹配已如实记录（§上表首行）；建议后续计划引用该链时采用空白容忍式。

## T2（25c.2）repository 批次搬迁 — pending

## T3（25c.3）service 批次搬迁 + MarketHostAdapters — pending

## T4（25c.4）handler 搬迁 + marketTenantID — pending

## T5（25c.5）差分证据收口 + Brief + 节点门禁 — pending
