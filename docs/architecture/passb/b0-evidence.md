# B0 契约与所有权冻结证据（b0-evidence）

> 实施计划：`docs/plans/passb/00-contract-and-ownership-freeze.md`（B0.1–B0.6）。
> Spec：`docs/specs/2026-09-21-backend-domain-module-reorganization-design.md` §11–§17.2；分解框架：`docs/plans/2026-09-23-backend-modularization-pass-b-framework.md`。
> 节点分支：`codex/passb-b0`。**节点基线（DAG base_sha，b0 已固定）：`b1a3d6dd825e3263e12b1daac2a52dab80ac5813`**（裁定见 §4.1）。计划文中的 `78f18915f` 为 Pass A 验收基线，仅作为计划 Step 4 字面命令的比对 rev。
> 本文件所在提交即 B0.6 完成提交。**B1 详细计划的起点 SHA = 本分支评审合并进 integration 分支后的头**，由协调者按 DAG head_sha 规则回填（conventions §9）——不得以本分支中间提交作为 B1 起点。
> 计数断言口径（F5 / conventions §8）：`passbguard` 实测值 == `docs/architecture/evidence/pass-a-acceptance.md` 台账记录值 == 目录发现值，三方一致即通过；期望值一律参数化读取，禁止字面量断言。

## 1. 就绪行与计数基线（三方一致）

命令（freeze B0.6 Step 2/3）：

```bash
make check-passb-readiness        # = go run ./tools/passbguard -root .
make check-backend-architecture
make verify-module-moves
find migrations -type f | wc -l
```

输出与判定（执行日期 2026-09-23，节点分支工作树）：

| 指标 | 实测 | 台账/发现值 | 判定 |
|---|---|---|---|
| legacy 归属 | legacy=396（overlaps=0 missing=0） | 台账 `legacy_files 396 条全部带 passb_task`；manifest 发现 396 | ✅ 三方一致 |
| alias 义务 | aliases=99 | manifest `alias_obligations` 发现 99 | ✅ |
| import 例外 | exceptions=105 | 台账 `import 例外共 105 条在册`；guard 源解析 105 | ✅ 三方一致 |
| 契约/事件 | contracts=125 / events=29 | 独立 YAML 解析（CLI 测试内不经 passbguard 模型）125/29 | ✅ |
| 路由 | total=633（564 literal + 69 apiKeyRoute） | 台账 633（同分解） | ✅ 零漂移 |
| worker | redis=23 lite=23 | 台账 23 任务类型 / 23 | ✅ |
| 生命周期挂点 | hooks=58 | 台账 58 | ✅ |
| migrations | `find migrations -type f` = 537 | 台账 537 文件（270 assets） | ✅ |
| 模块 | modules=16 | `internal/modules/*` 目录 16 | ✅ |

就绪行原文（成功路径唯一 stdout 输出，退出码 0）：

```text
pass-b readiness: legacy=396 aliases=99 exceptions=105 contracts=125 events=29 overlaps=0 missing=0
```

396 legacy 的模块分布（`TestRealRepoOwnershipMatrixFreezesAllLegacy` 断言，与框架 freeze:150-155 冻结总量一致）：identity 27 / airesource 33 / commercial 8 / execution 21 / knowledge 84 / agentcatalog 55 / datasource 4 / appconnector 7 / agentruntime 45 / conversation 44 / channels 7 / insights 6 / workbench 19 / craft 30 / system 5 / policy 1（B1=89，B2=150，B3=102，B4=55）。

105 例外删除期限分布（exception-ledger.yaml）：ib1=2 / ib2=8 / ib3=80 / ib4=15，无 b5 兜底逃逸（`exception-b5-escape` 检查零命中）。99 alias 删除期限分布：ib1=22 / ib2=35 / ib3=32 / ib4=10。

## 2. 命令与退出码（B0.6 Step 3/Step 4 全量）

| 命令 | 退出码 | 关键输出 |
|---|---|---|
| `go test ./tools/passbguard ./tools/modulemove ./tools/architectureguard ./internal/bootstrap -count=1` | 0 | 4 包全 ok |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |
| `make check-backend-architecture` | 0 | `total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`，0 violations |
| `make check-passb-readiness` | 0 | §1 就绪行 |
| `go build ./...` | 0 | 仅 `ld: warning: ignoring duplicate libraries: '-lc++'`（链接器既有告警，非错误） |
| `go test ./internal/... -count=1 -timeout=25m` | 0 | 119 包 ok，0 FAIL（终树复跑） |
| `golangci-lint run --new-from-rev=b1a3d6dd8 ./...`（节点基线口径） | 0 | **0 issues** |
| `golangci-lint run --new-from-rev=78f18915f ./...`（计划字面 rev） | 1 | 1 issue：`internal/commercial/repository/commercial/benefits.go:105 QF1008`（§5.4，先于节点基线、非 b0 文件） |
| `git diff --check 78f18915f...HEAD` | 2 | 2 处 `packages/design-tokens/src/tdesign-theme.css` 尾随空白（ff7380052，前端 parity 流，先于节点基线）；framework doc EOF 空行已由 B0.6 修复（修复随本提交入库后此 finding 消失） |
| `git diff --check b1a3d6dd8...HEAD`（节点基线口径） | 0 | 零 finding |
| `git diff 78f18915f...HEAD -- cmd/desktop docreader client` | 0 | 输出为空（禁改范围零触碰） |

CLI 行为测试（`tools/passbguard/main_test.go`，RED→GREEN）：成功路径打印冻结就绪行、退出 0、stderr 为空；诊断路径（fixture 根）按 `check: path: message` 排序输出到 stderr、stdout 为空、退出 1，且 legacy-missing / alias-missing / exception-missing 三类缺属主诊断可机器分派。

## 3. 交付物与逐任务提交

| 任务 | 提交 | 内容 |
|---|---|---|
| 前置 | `a228d6e9d` / `5bf228a40` | execution DAG（33 节点）+ F1–F8 审校修正 |
| B0.1 | `9f87a809d` | 严格 schema 加载器（model.go/load.go/model_test.go + testdata 四 fixture） |
| F1-a | `a83d18b2c` | movemanifest 共享库提取（modulemove/passbguard 单一事实源） |
| B0.2 | `0bee2f239` | ownership-matrix.yaml（396+99）+ exception-ledger.yaml（105）+ discover/check + ownership_test |
| B0.3 | `5cfa5bc02` | overlap.go 裁定表 + 9 份 brief 消歧 + execution.md（新建） |
| B0.4 | `fac268d41` | contracts.yaml（125 行：16×4 组合面 + 3 基线行 + 符号契约）+ check/discover 扩展 + contracts_test |
| B0.5 | `3583773da` | event-catalog.yaml（29 事件）+ events_test |
| B0.6 | 本提交 | main.go CLI（目录翻转为 package main）+ main_test.go + Makefile `check-passb-readiness`（.PHONY 并入首行汇总）+ 本证据 + 框架完成记录；lint 清偿（§5.5） |

治理文件写权限合规：`ownership-matrix.yaml`/`contracts.yaml`/`event-catalog.yaml`/`exception-ledger.yaml` 仅 b0 建；`internal/modules/*/module.go` 仅注释（B0.4）；Makefile 仅追加目标与 .PHONY 并行（F6）。

## 4. 裁定记录（rulings）

1. **基线 SHA**：任务书称 main@29c1e5635、计划文称 78f18915f；会话实证 integration HEAD 为 `b1a3d6dd8`（多 2 个纯前端 parity 提交，不触及本节点文件）。b0 按 `b1a3d6dd8` 起算（DAG base_sha 已固定）；78f18915f 保留为计划 Step 4 字面命令的比对 rev，其增量发现全部可归因于节点基线之前（§2/§5.4）。
2. **F1（单一事实源）**：manifest schema/加载器提取为 `tools/internal/movemanifest`（modulemove 与 passbguard 共同消费，禁双实现）；import 例外的唯一在册事实源是 `tools/architectureguard/check.go` 的 `importExceptions` 复合字面量——passbguard 以 go/ast 解析该源码并与 exception-ledger 逐条对照（reason/PassBTask 字段级）。
3. **F5（计数三方一致）**：断言目标是 guard 实测 == pass-a 台账 == 目录发现；CLI/测试期望值从台账与发现值参数化读取而非字面量（freeze:349 修正；CLI 测试的 396/105 断言即由台账参数化承载）。
4. **B0.3 Agent Runtime**：`native_archive.go`（service + handler/session 双文件）→ `34-agentruntime-protocol`；`native_recovery.go` 与 native repository 状态/lease/pending/usage 族（commit/events/memory/oauth/schema/session/tool_journal）→ `33-agentruntime-engine`；纯协议包（agent/native、nativecontract、nativeprobe、trpc、opencode、recoverytest）→ 34；approval/run/attempt/checkpoint/decisions/events/inputs/lifecycle/tools journal → 33。
5. **B0.3 共享宿主 internal/handler/session 36 文件逐路径**：wiki_fixer_scope→23；workbench_*/artifact→40；craft*→41；agent run/stream→33；browserskill/sandbox_terminal_{bridge,ws}→13；其余 Session/Message/Feedback/share/attachment/stream→35。`internal/application/repository`、`internal/application/service` 共享文件同样逐路径入 matrix（396 全量）。
6. **B0.3 platform 保留**：`internal/handler/list_pagination.go`、`internal/handler/upload_limit.go` 保持 platform，任何子计划不得认领（`ruling-platform-claimed` 检查）。
7. **B0.3 housekeeping**：`knowledge_housekeeping.go` 业务清扫规则留 24-knowledge-process；System（42-system-policy）经窄 `KnowledgeHousekeeping` port 拥有调度/生命周期调用，禁第二套清扫实现。
8. **b0 节点审校裁定（knowledge.go 悬空）**：`internal/application/repository/knowledge.go`（escapeLikeKeyword 宿主，knowledge.yaml:87 在册但 4 份 K brief scope 未枚举）显式归 `24-knowledge-process`（destination `internal/knowledge/process`，integration_owner/delete_barrier=ib2），matrix 头部注明并由 `knowledge-process.md` 全路径消歧；`TestRealRepoKnowledgeRepositoryFileHasExplicitKOwner`/`TestRealRepoBriefDisambiguatesKnowledgeRepositoryFile` 机器锚定。
9. ***Handler 去方法化**：browserskill/sandbox_terminal_ws/wiki_fixer_scope/agent_run/artifact_download 等 `*Handler`（handler.go，conversation 属主）上的跨 owner 方法文件按 B0.3+IB1 裁定去方法化或推迟，DAG 各节点 required_contracts 已列明；`native_archive.go` 自带独立 `NativeArchiveHandler` 不属此列。
10. **歧义措辞零容忍**：治理目录（docs/architecture/passb/**.md|yaml）禁三类歧义表述（`CheckAmbiguity`，含行号定位），当前零命中。

## 5. 已知债务与遗留（B1+ 输入）

1. **105 条跨模块 import 例外**：全部在 exception-ledger.yaml 登记删除属主与期限（ib1=2/ib2=8/ib3=80/ib4=15）；对应模块边界收紧改走公开门面后由属主计划删除，guard 源与台账两侧漂移由 F1 机器校验拦截。
2. **99 条 alias 义务**：ownership-matrix.yaml 逐条登记删除属主（ib1=22/ib2=35/ib3=32/ib4=10）。
3. **Pass A 台账在册债务**（pass-a-acceptance.md §4）：13 条 agentruntime moved 文件 lint 债 → B-agentruntime；A2 payment `TestProvidersFromEnvRejectsPartialAlipay` map 序断言 → B-commercial。
4. **benefits.go QF1008（staticcheck）**：`internal/commercial/repository/commercial/benefits.go:105`（`s.db.Dialector.Name()` 可去嵌入字段选择子）。由 `b69b6980e`（EnsureSchema 方言修复，2026-09-22，先于节点基线 b1a3d6dd8 合入 main）引入；该文件属 commercial 模块（B1-CM 契约区），b0 无写权限，仅登记——建议 B1-CM 顺带清偿。节点基线口径 lint（--new-from-rev=b1a3d6dd8）为 0 issues，本项不阻塞 b0。
5. **B0.6 lint 清偿**：B0.1–B0.5 引入的 50 条 passbguard 告警（lll×45、errcheck×3、gofumpt×1、QF1001×1）已随 B0.6 全部清零（字符串内容逐字节保持，仅换行/拼接/格式化调整；QF1001 为 De Morgan 等价变换）；`tools/passbguard/testdata/contractrepo/**` fixture 不参与构建与 lint。
6. **前端 parity 流遗留**：`packages/design-tokens/src/tdesign-theme.css` 两处尾随空白（ff7380052）——非本节点文件，登记归属前端 parity 流；节点基线口径 `git diff --check` 零 finding。
7. **passbguard 形态**：目录为 package main（`go run ./tools/passbguard -root .` 要求），不可被其他 Go 包 import；校验能力唯一消费口是 `make check-passb-readiness`（B0.1 包注释即冻结此演进）。
8. **框架文档登记差异**：本框架 B0 节点原文写 `contracts.md` / `architectureguard checks`；实施按冻结计划落地为 `contracts.yaml` 与专用工具 `tools/passbguard`（B0 完成记录已注明）。语义一致，无契约面差异。

## 6. Review Focus 对照（freeze:26-30）

| Review Focus | 证据 |
|---|---|
| 遗漏/重复/改名/双认领必须失败 | ownership_test.go 表驱动 fixture（legacy-missing/overlap/nonexistent/undeclared/plan-module/barrier）+ 真实仓库零诊断 |
| Agent Runtime overlap 确定且与 brief 一致 | overlap_test.go（AgentRuntimeNativeRuling 17 路径 + brief 认领一致性）+ 两份 brief 已改写 |
| 共享宿主逐文件、禁整目录 | HandlerSessionRuling 36 文件全量 + matrix 逐路径（无目录前缀行；schema 禁通配符） |
| 契约消费方全量、未登记新消费方拒绝 | CheckContracts 的 contract-consumer-unrecorded/vanished + /adapters 与跨模块非公开子包禁用导入扫描 |
| 事件事实/命令区分 + 租户/版本/时间/幂等元数据 | validate() 事件 schema（命令动词黑名单、producer/version 唯一、tenant_id→occurred_at/event_id、idempotency_key 必含）+ events_test 家族覆盖 |

## 7. 无生产行为变更声明

B0 全部提交只触及：治理 YAML/MD、`tools/passbguard/**`（新建，含 contractrepo fixture 下的 acceptance 台账副本）、`tools/internal/movemanifest/**`（自 modulemove 提取，行为等价）、`tools/modulemove` 改为消费共享库（F1-a，行为等价）、`internal/modules/*/module.go` 注释、Makefile 追加目标。真实台账 `docs/architecture/evidence/pass-a-acceptance.md` 未被 b0 修改（B0.4 stat 中的同名行是 `tools/passbguard/testdata/contractrepo/` 下的 fixture 副本）。`cmd/desktop`、`docreader`、`client`、生产 SQL、既有迁移文件零触碰（§2 末行命令输出为空）；`internal/...` 全量测试 119 ok / 0 FAIL 佐证零行为漂移。
