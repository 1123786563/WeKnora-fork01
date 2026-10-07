# Pass A 验收证据（IA4 Acceptance）

分支：`backend-mod-passa`，验收基点 F0 `7b1d3cf017a50cc8061da166d67befc6b51773a0`（main），IA4 终态见 git log。
验收日期：2026-09-21。Spec：docs/specs/2026-09-21-backend-domain-module-reorganization-design.md §17.1。

## 1. 交付完整性（Spec §17.1 逐项）

| 验收项 | 结果 | 证据 |
|---|---|---|
| 16 模块均有所有者、README、module.go、代码地图、迁移台账 | ✅ | internal/modules/<id>/{README.md,module.go,legacy/README.md} ×16（F1 35ee7e088 起，各 A 任务维护）；归属 docs/architecture/backend-modules.yaml |
| 所有服务端生产文件与测试都有目标模块归属 | ✅ | F1 覆盖恒等式：manifests 并集 + platform/bootstrap 残留（moves/README.md）= F0 库存 128 包；F0 已审 |
| 天然成块代码物理归位，剩余 legacy 有精确清单 | ✅ | 14 个 A 任务搬迁：A1 5 包/65 文件、A2 7/116、A3 12/97、A4 10/99、A5 3/142、A6 12/188、A7 0（真空）、A8 5/25、A9 18/142、A10 1/47、A11 20/371、A12 4/36、A13 1/47、A14 1/13；legacy_files 341 条全部带 passb_task（manifests；IB2 集成合并树实测——基线 396 经 K 面（−9+4−3+1−2+1）、datasource（−1）、appconnector（−6）、上游布局回归收尾（−11，2026-10-07，R2.5 删 11 条文件已删的 shim/compat legacy 行）演进，变更登记见 §6；matrix 实测 352→341，agentruntime/insights 较本表旧冻结值另有预存 −3/−3 漂移） |
| router/container/Worker 按模块注册且无重复 | ✅ | architectureguard 唯一注册检查 0 violations；composition 全部指向 internal/modules/*（各 IA 证据） |
| 新业务代码不再进入旧横向业务目录 | ✅ | F2 legacy guard（service/repository/handler 三目录 + manifest 允许集）0 violations |
| 所有 Integration Barrier 构建/测试/守护/Review 不劣于基线 | ✅ | IA1 118 ok（+2 在册 unstable）/IA2 119 ok/IA3 119 ok/IA4 119 ok 全绿；四份 barrier review 均 Approved |
| cmd/desktop、docreader、client/SDK 未被改造 | ✅* | 零逻辑变更；4 个 cmd main.go（server/desktop/connector-control/saas-migrate）仅 import 路径行维护（1-2 行/文件，随内部包搬迁的机械性依赖修复，逐行核验纯 import）——controller 裁定：这不构成"改造" |

## 2. 基线比对（F0 → 终态）

| 指标 | F0 | 终态 | 判定 |
|---|---|---|---|
| 路由注册 | 633（564 literal + 69 apiKeyRoute） | 633（同分解） | ✅ 零漂移 |
| Redis/Lite worker | 23 任务类型 + 6 池 / 23 | 同 | ✅ 集合一致（guard 双侧计数） |
| container.Invoke hooks | 58 | 58 | ✅ |
| migrations | 537 文件（270 assets） | 537，git diff 零变更 | ✅ |
| 生产包 | 128（cmd/server + internal） | 139（F0 128 包全数迁移/索引 + 16 新模块根包 + internal/bootstrap，均有归属） | ✅ |
| 测试 | 118 ok + 2 flaky 注册 | 119 ok / 0 FAIL | ✅ 优于基线 |

## 3. Pass A 期间新增的治理资产

- `tools/modulemove`（严格 manifest 校验 + 已集成搬迁语义）与 `tools/architectureguard`（资产发现/唯一注册/跨模块禁导入/legacy 新文件拒绝），Make 目标 `verify-module-moves`、`check-backend-architecture`。
- guard 精确路径 import 例外共 **13 条**在册（原 Pass A 105 条 + Pass B K1-K4/25b/datasource 显形 37 条经 R2.2-R2.4 收敛至 130 条；上游布局回归收尾 R2.5，2026-10-07：117 条 ledger 行因 importer 已随 internal/agent、internal/models、internal/application、internal/handler 等归位移出 internal/modules 判定面或 import 已改写而悬空删除，130→13；guard 源同批另删 4 条 guard-only 悬空边，check.go 141→20），全部绑定 Pass B 任务（B-appconnector/B-workbench/B-knowledge），无通配。
- 冻结契约：docs/architecture/frozen-entrypoints-batch-a2.md（A9–A14 消费面）。
- Pass B 简报：docs/architecture/passb/（knowledge×4、conversation×2、agentruntime×4、workbench/craft/insights 各 1 + manifests 内 alias_obligations/legacy_files 义务）。

## 4. 在册债务与遗留（Pass B 输入）

- 13 条 moved 文件 lint 债（internal/modules/agentruntime/**，lll/revive/unused，预存显形；裁定不加 nolint 以保 R100 证据链）→ B-agentruntime。
- 预存横向耦合 113 条（105 条 Pass A 登记 + 8 条 Pass B 25b 迁移登记，Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY，见 §3）→ 对应模块 Pass B 边界收紧时改走公开门面后删除例外。
- A2 payment `TestProvidersFromEnvRejectsPartialAlipay`（map 序断言）→ B-commercial 修测试。
- system housekeeping hook 实现文件归 knowledge → B-knowledge/B-system 协调。
- 各模块 legacy_files（396 条）→ 对应 B-<module> 拆分。

## 5. 结论

Pass A（Move First）全部 17 个任务 + 4 个集成屏障完成并逐一独立审查通过；系统行为、外部契约、基线计数全程零漂移；16 模块归位完成，剩余 legacy 全部有精确清单与 Pass B 义务。**Pass A 验收通过**，Pass B 待另行规划。

## 6. Pass B 期基线变更登记（conventions §8 流程）

| 日期 | 变更 | 批准 | 原因与证据 |
|---|---|---|---|
| 2026-09-23 | legacy_files 基线 396 → 390（−6：appconnector 模块 6 个已迁 handler 行） | 协调者裁定 **Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP**（b2-appconnector 节点 B2-AC.2 执行前升级，方案 A） | Pass B 搬迁节点物理迁出 legacy 文件后 `tools/modulemove`（verify.go legacy_files 存在性校验）与 `tools/passbguard`（legacy-nonexistent）要求 manifest/matrix 行同窗删除。Ruling 授权搬迁节点在迁移 commit 同窗删行（行删除早于 delete_barrier，该字段按最后期限语义）、按本条登记基线变更、并允许 passbguard 测试字面量计数同窗机械修正。逐行去向（原路径 → 目标包 + 迁移 commit）：见 `docs/architecture/evidence/passb/b2-appconnector.md` §4。三方一致复验：guard 实测 390 == 本台账 390 == manifests 发现 390（`make check-passb-readiness` + `go test ./tools/passbguard` 于变更 commit 实跑）。例外基线 105 不变。 |
| 2026-09-25 | legacy_files 基线 396 → 391（−9：b2-k-ingest K1.1–K1.5 迁移行删除的补登记（Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 范畴，各迁移 commit 已同窗删行、本条补台账）；+4：b2-k-ingest 过渡 shim 成对补行） | 协调者裁定 **Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION**（K1.1 报告遗留 1 升级、K1.6 执行，方案 a） | K0 R1.3 强制宿主过渡 shim 落横向目录，architectureguard legacy-guard 豁免面（check.go:1240-1262）无登记位。Ruling 授权成对补行（knowledge.yaml legacy_files + ownership-matrix，reason「Pass B 过渡 shim，ib2 同 commit 随文件删行」、passb_task=B-knowledge、destination=internal/modules/knowledge/ingest、integration_owner=delete_barrier=ib2）；ib2 收口时 shim 文件与行同 commit 删除（写入 K5 Brief）。4 行：internal/application/repository/chunk_ingest_shim.go、internal/application/service/chunk_ingest_shim.go、internal/application/service/span_trace_seam_adapter.go、internal/handler/chunk_ingest_shim.go。三方一致复验（本分支实测）：guard/manifests 发现/matrix 均 391。注：并行兄弟分支（b2-appconnector 390 等）各自登记本分支基线，集成侧 ib2 按 F5 口径合并复核。 |
| 2026-09-25 | import 例外基线 105 → 111（+6：b2-k-ingest K1 搬迁显形 exc-0106..0111） | **Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY**（plan 21-knowledge-ingest §7.2） | K1 搬迁使预存横向包耦合显形：extract.go→airesource chat/embedding + policy/access、seams.go→airesource chat（消费侧 seam 类型签名）、image_multimodal.go→ollama/vlm；plan 预测 E3（agentruntime）经 K1.3 import 环裁决改走 seam 后无需登记。commit fc14f4c2e（check.go 数据行 + exception-ledger 同窗 + evidence §8 登记）；三方一致复验：guard 发现 == ledger 111。 |
| 2026-09-25 | legacy_files 基线 391 → 389（−3：b2-k-process K4.2 物理迁移行删除（knowledge_write/knowledge_index_content/knowledge_task_options，Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 同 commit 删行）；+1：K4.2 宿主 compat 过渡 shim 成对补行 kbprocess_passb_compat.go）；ownership_test wantPerModule knowledge 79→77 随窗机械修正 | **Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION**（compat 补行）+ **Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP**（迁移删行）；K4.2 收缩迁移依据 **Ruling 2026-09-25-DEFERRED-FILE-SPLIT**（环境安全策略根因类，同 K4.1 先例） | K4.2 迁移 service 独立面自包含子集 3 生产文件至 internal/modules/knowledge/process（write-family 4 函数/knowledgeWriteKB/KnowledgeBaseWriteLookup/buildKnowledgeIndexContent/task-options 两函数导出，knowledge_write.go requireKBWrite 直连 kbretrieval.RequireKBWrite）；span_tracker/housekeeping 两对生产+测试因 Mimosa 对 CREATE TABLE DDL 测试常量确定性误报全通道阻断（Write/Edit/Bash/git mv 实测）成对推迟（housekeeping_test 十处 svc.runSweep 未导出方法垫片不可达、span_tracker_test fitSpanName 15 行超 §5.4b 5 行 seam 限制），登记推迟件清单（解除条件=用户放行后逐字补迁+R100 复验，最迟 ib2）。compat 行 reason「Pass B 过渡 shim，ib2 同 commit 随文件删行」、passb_task=B-knowledge、destination=internal/modules/knowledge/process、integration_owner=delete_barrier=ib2。三方一致复验（本分支实测）：manifests 发现 == matrix == 389。注：并行兄弟分支各自登记本分支基线，集成侧 ib2 按 F5 口径合并复核。 |
| 2026-09-26 | legacy_files 基线 389 → 388（−2：b2-k-process K4.3 物理迁移行删除（handler/kb_access.go、handler/task_progress_auth.go，Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP 同 commit 删行）；+1：K4.3 宿主 compat 过渡 shim 成对补行 internal/handler/kbprocess_passb_compat.go）；ownership_test wantPerModule knowledge 77→76 随窗机械修正 | **Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP**（迁移删行）+ **Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION**（compat 补行） | K4.3 迁移 handler 层 2 文件至 internal/modules/knowledge/process/handler（kb_access 4 helper→ResolvedKBAccess/ResolveHandlerKBAccess/ResolveHandlerKBAccessFor/KBAccessHTTPError、requireTaskProgressTenant→RequireTaskProgressTenant 导出；kb_access.go:45 的 requireTenantAPIKeyKnowledgeBase 直连 types.AuthorizeTenantAPIKeyKnowledgeBases（两级单行转发链逐字等价，§5.4a 第 4 行）；随迁 task_progress_auth_test.go；宿主 compat 5 个同形一行委托保 knowledge.go/knowledgebase.go/knowledge_download.go/faq_k3_compat.go/kb_access_test.go 引用面零改动）。compat 行 reason「Pass B 过渡 shim，ib2 同 commit 随文件删行」、passb_task=B-knowledge、destination=internal/modules/knowledge/process、integration_owner=delete_barrier=ib2。三方一致复验（本分支实测）：matrix == 台账 == 388（knowledge 76）；manifests 发现 365 与 matrix 的 23 行差（33 条 K2/K3 已迁残留行 − 10 条 compat 缺配对行）为 BASE 既有漂移（K4.0-R 处置表登记、ib2 回写批），passbguard 诊断条目级 BASE/HEAD IDENTICAL（219==219，零新增）证明本节点零贡献。注：并行兄弟分支各自登记本分支基线，集成侧 ib2 按 F5 口径合并复核。 |
