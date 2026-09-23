# b2-ac-definition 实施报告（25a Agent 定义/版本/人格/专家/子代理/收藏）

> 节点：`b2-ac-definition`（Pass B 阶段 B2）；分支 `codex/passb-b2-ac-definition`；计划 `docs/plans/passb/25a-agent-definition-version.md`（v3）；执行公约 `.superpowers/sdd/passb/conventions.md`（§1.1/§1.2 格式）。
> 任务起点（计划撰写基线）：`8c45a8815`（DAG `base_sha` 口径）；实施实际起点 `02e7be617`（两差仅为计划文档审校修订提交，evidence §1.1）。集成与回滚边界：计划 §7。
> 本报告由 T5 收口撰写；逐 Task 细节见 evidence `docs/architecture/evidence/passb/b2-ac-definition.md` 与各任务级报告存档（主 checkout `.superpowers/sdd/passb/b2-ac-definition/T{1..4}-report.md`）。

## 1. 交付摘要（T1–T5）

| Task | Commit | 内容 |
|---|---|---|
| T1 基线锚定 | `9f330903b` | 基线门禁四条实跑全绿（633/23+23/58、16 manifests）；随迁测试宿主侧预跑 13 用例全 PASS（等价双跑「旧实现侧」）；前置门 `EXPORT-MISSING` 登记 |
| T2 repository 批次 | `57ef2990e` | 4 个 repository 文件搬入 `internal/modules/agentcatalog/repository` + 4 个原路径 shim + 2 测试随迁 + 2 特征化新测试 |
| T3 service+handler 批次 | `bd9e0f8e9` | `service/user_resource_favorite.go`、`handler/user_resource_favorite.go`、`handler/subagent.go` 搬入模块 + 3 个原路径 shim + 哨兵重指向（module 同名 var 实体）+ `subagent_test.go` 随迁 + 2 特征化新测试 |
| T4 门禁+等价证据 | `5d543c49d` | DAG 四条 gates 全绿；等价双跑 28/28 逐用例一致；高风险面 presence 核对；evidence 补全 |
| T5 Brief+报告 | 本提交 | `docs/architecture/passb/briefs/b2-ac-definition.md`（IB2 装配变更申请 + 13 行推迟台账 + 反向义务 + 拆分确认 + 计数声明）、本报告 |

代码终点 = `bd9e0f8e9`；`5d543c49d` 起为纯文档提交（`git diff --name-only bd9e0f8e9..HEAD` 仅 `.md` 文件，§4 实证）。

## 2. 命令清单与退出码（conventions §1.2）

### 2.1 T5 会话在最终 HEAD 实跑（2026-09-24）

| 命令 | 退出码 | 关键输出 |
|---|---|---|
| `go build ./...` | 0 | 仅 `cmd/desktop`、`cmd/server` 各一行 `ld: warning: ignoring duplicate libraries: '-lc++'`（既有现象，T1 基线已记录，非错误） |
| `go test -count=1 ./internal/modules/agentcatalog/...` | 0 | `?  …internal/modules/agentcatalog [no test files]`；`ok  …agentcatalog/handler 1.355s`；`ok  …agentcatalog/repository 1.292s`；`ok  …agentcatalog/service 0.625s` |
| `make check-backend-architecture` | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`architectureguard: OK (0 violations)` |
| `make verify-module-moves` | 0 | `modulemove: OK (16 manifests verified)` |

### 2.2 前序 Task 实跑存档（详见 evidence）

- T1 基线（BASE=02e7be617）：四条基线门禁全绿（evidence §1.3）；宿主三包全量 `go test -count=1 ./internal/application/{repository,service} ./internal/handler` ok（repository 489.119s、service 344.838s、handler 1.825s）；随迁测试宿主侧 13 用例 PASS（evidence §1.4）。
- T4：DAG 四条 gates 全绿 + 等价双跑 `go test -count=1 -run 'TestGetShareByAgentIDAndSource|TestSubagent|TestFavorite' ./internal/modules/agentcatalog/... -v` → 28/28 PASS（evidence §2.1/§2.2）。
- **T5 未重跑宿主三包全量测试**：T4/T5 均零代码改动（§4 差集实证），最后一次同代码内容全量实跑为 T3（service 100.288s ok、repository 218.697s ok、handler ok、router ok）；此为如实登记，非豁免声明。

## 3. 差集核对：变更文件 vs owned_files（conventions §1.2）

节点累计变更（`git diff --name-only 02e7be617`，22 文件；T4 evidence §2.4 实测口径，本会话未再变更代码）：

- 7 个模块生产新文件（批次 1：repository×4、service×1、handler×2）；
- 7 个宿主原路径 shim 重写（计划 §4-③ 行 1–7，逐符号别名/转发，零业务语句）；
- 3 份随迁测试（`rename (100%)`）；4 份特征化新测试（conventions §1.4 授权）；
- 3 份节点文档（evidence、Brief、本报告）。

**核对结论：与授权清单差集为空。** owned_files（DAG：manifest 25a 行 + `internal/modules/agentcatalog/**` definition/version 面）覆盖上述模块文件；宿主 7 路径为 §2.3 机制要求的同路径 shim 重写（框架偏差已按 conventions §1.5/framework:29 登记）；文档三件为计划 §2.3 授权的节点产出。禁改面零触碰：`internal/router/**`、`internal/container/container.go`、`internal/bootstrap/**`、`internal/modules/agentcatalog/module.go`、治理 YAML 四份、`docs/architecture/moves/*.yaml`、`tools/**`、`go.mod`/`go.sum` 均不在差集。例外行新增 0；既有例外行删除 0（exception-ledger 无 agentcatalog importer 行，计划 §8 口径）。

## 4. T5 自身差集与提交内容

- `git diff --name-only bd9e0f8e9`（T4 起累积至 T5 提交前）＝ `docs/architecture/evidence/passb/b2-ac-definition.md`（T4）+ `docs/architecture/passb/briefs/b2-ac-definition.md`（T5 新建）+ `docs/plans/passb/reports/b2-ac-definition.md`（T5 新建）——纯文档，零代码，零 .go 触碰。
- Brief 关键事实全部 HEAD 实测复核：7 shim 内容通读（纯别名/转发）；全量消费方 `grep -rwn` 重跑（行号更新至 `5d543c49d`）；13 行推迟台账逐符号 presence 复核（含 `agentService` 方法扩散 8 处、`applyTenantRoleCap` 3 点、`isUniqueViolation` 三定义、`sandboxConfigTenantID` 3 点、`pickUserDisplayName`/`AgentCreatorLookup`、conversation 7 符号 def:line 与 10 调用点）；拆分确认以集合划分脚本对 manifest 55 行实证 22/20/13 双向差集为空、两两交集为空。
- 上报事项（conventions §9，实施者不写 DAG）：DAG `status`/`head_sha`/`task_ids` 回填、review 发起归协调者；Brief ②§2.3/②§2.4 两项裁定请求（conversation 7 符号统一提前导出、§7.4 方法扩散收口）待裁。

## 5. 高风险差分证据指针（conventions §6）

- 批次 1 无 framework:40 高风险面；节点高风险面（共享代理 KB 可见性过滤三函数、`agentRequiresRerankModel` rerank 校验）全部位于批次 2 留宿文件，**差分义务随推迟件登记转移至 IB2/IB3 执行窗口**（Brief ②§2.2-4/-6 显式登记）。
- presence 核对：evidence §2.3（定义/消费点 grep 实测表）；等价双跑（批次 1，28 用例）：evidence §2.2 逐用例比对表。

## 6. 未完成项 / 推迟项（如实列出，禁止省略）

1. **批次 2 共 15 文件未搬迁**（计划 §2.2/§4-② 既有安排）：13 行推迟台账 + 解除条件 + 排序约束已全部登记 Brief ②§2.1；其中台账 #5 之前置门 B1-CM 导出当前 **`EXPORT-MISSING`** 未解除（evidence §1.5/§2.5-2），缺失即按 conventions §5 blocked 上报，不得自行实现导出。
2. **`agentRequiresRerankModel` 导出义务未在本节点执行**（调度指定）：因属主文件 `service/agent_share.go` 本体推迟至 IB2，导出 + 宿主一行委托 shim 随其 IB2 搬迁执行（Brief ②§2.2-1 IB2 执行单）。
3. **conversation 7 符号提前导出、§7.4 方法扩散收口**：裁定请求已提交（Brief ②§2.3/②§2.4），待协调者裁定后由集成工程师执行。
4. **7 个过渡 shim 未删除**：删除点 = IB2（Brief ①§ 三波顺序 + ①.2 manifest 行处置前置裁定）；宿主包终态（framework:29）在第三波核验。
5. **manifest 7 行处置**：schema LOCKED 无已搬迁标记，承载形式归协调者裁定（Brief ①.2），本节点禁改。
6. **宿主三包全量测试 T4/T5 未重跑**（§2.2 末条，代码零改动下的如实登记）。
7. **两项 OCR low findings**（swagger 漂移、错误回显加固）：调度已裁定 IB2 期执行（Brief ②§2.6），本节点按 T3 零逻辑约束保持原样。
8. **高风险差分双跑未在本节点执行**：随推迟件转移至 IB2/IB3 窗口（§5，计划 §5 既有安排）。
