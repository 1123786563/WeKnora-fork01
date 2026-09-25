# b2-k-ingest 节点报告（K1.0–K1.7）

> 节点：`b2-k-ingest`（plan `docs/plans/passb/21-knowledge-ingest.md`；DAG depends_on=[b2-k0]）。
> worktree `.worktrees/passb-b2-k-ingest`（分支 `codex/passb-b2-k-ingest`）。
> 任务 K1.0–K1.7 全部执行完毕；本报告为 K1.7 收口（差分双跑 + gates 终跑 + 本文件）。
> 差分证据：`docs/architecture/evidence/passb/b2-k-ingest.md`（T0/新实现/逐用例比对三段齐备）。

## 1. 节点结论

- **9 文件搬迁完成**：repository/chunk.go、service/{chunk,chunk_write,extract,image_multimodal,ocr_sanitizer,parser_url_security}.go、handler/{chunk,chunker_debug}.go 全部落位 `internal/modules/knowledge/ingest`（M2 纯移动 rename 识别，spec §14.2）；宿主 9 路径 `ls` 全部 No such file（K1.7 会话实测，见 §3 #1）。
- **11 随迁测试 + 差分双跑等价**：T0（40 顶层 + 25 子用例，三包分跑全 PASS）vs K1.7 ingest 单包重跑（40 + 25，0 FAIL）逐名逐例等价（evidence 差分章节）；高风险面五锚点族（SeqID/revision 原子性/TypeIndexDelete tag 侧/FAQ diff/finalize-once）双侧 PASS。
- **节点 gates 四项全绿**（§3 #4-#7：build 0 / knowledge 全量 17 包 ok / architectureguard 0 violations / modulemove 16 manifests OK）。
- **验收项 6（宿主全绿）**：handler ok 5.399s、repository ok 577.784s、service ok（首轮 8 例环境性 DNS 失败，复跑 ok 467.336s，见 §5-1）、file ok。
- **ocr-r1-f1 收口**：legacy/README.md 镜像不变式恢复（commit fcb6bd449，§4）。
- **Integration Brief 十类齐备**：`docs/architecture/passb/briefs/b2-k-ingest.md`（K1.6 8a6157b43 扩写，212 行）。

## 2. 命令台账（K1.7 会话原文执行；K1.0–K1.6 见各自任务报告）

| # | 命令 | 退出码 | 关键输出 |
|---|---|---|---|
| 1 | `ls internal/application/repository/chunk.go internal/application/service/{chunk,chunk_write,extract,image_multimodal,ocr_sanitizer,parser_url_security}.go internal/handler/{chunk,chunker_debug}.go` | 1（预期） | 9 行全部 `No such file or directory`（验收 3 前半） |
| 2 | `go test -count=1 -v ./internal/modules/knowledge/ingest/ -run '<T0 全清单 17 模式合并>'` | 0 | `ok …ingest 1.391s`；顶层 40 PASS / 子用例 25 / FAIL 0（差分双跑，evidence「新实现」段） |
| 3 | `go test -count=1 ./internal/handler/ ./internal/application/...`（验收 6） | 1→0 | handler ok 5.399s、repository ok 577.784s、file ok；service 首轮 8 例 `TestQueryTemplates*` 环境性失败（`lookup api.e2b.app: no such host`，DNS）；service 单包复跑 `ok … 467.336s` 退出码 0（§5-1） |
| 4 | `go build ./...`（gate 1） | 0 | 仅既有 `ld: warning: ignoring duplicate libraries: '-lc++'`（先于本节点存在，K1.6 报告 #8 同款） |
| 5 | `go test -count=1 ./internal/modules/knowledge/...`（gate 2） | 0 | 17 包全 `ok`（ingest 2.433s、kbfreeze 2.142s、chunker/docparser/retriever*/searchutil/semantic） |
| 6 | `make check-backend-architecture`（gate 3） | 0 | `architectureguard: literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16` + `OK (0 violations)` |
| 7 | `make verify-module-moves`（gate 4） | 0 | `modulemove: OK (16 manifests verified)` |
| 8 | `PASSB_BASE_SHA=$(git merge-base origin/main HEAD)` → `b1a3d6dd8`；`git diff --stat "$PASSB_BASE_SHA"...HEAD` | 0 | 98 files, +22252/−336（K1.7 复核会话 2026-09-25 实测更正——首记 97/+22136 有误；含 K1.0 基线对齐 merge 引入与 ib1/b0 世系先于分支点的内容，见 §4 拆分与 §9 复核） |
| 9 | 镜像差集核验（python3 双向 comm + 逐条同序比对） | 0 | README 79 行 == knowledge.yaml legacy_files 79 行，双向差集空、顺序一致（§4 ocr-r1-f1） |

## 3. K1.7 差分双跑摘要（全文见 evidence）

- 命令：T0 三条命令的 17 个 `-run` 模式合并为一条对 `./internal/modules/knowledge/ingest/` 重跑（9 文件 + 11 测试已同包）。
- 结果：顶层 40（repo 13 + service 14 + handler 13 归并）、子用例 25、FAIL 0、退出码 0——与 T0 逐名逐例等价。
- 高风险锚点：`TestCreateChunks_SQLite_SeqID*` ×4、`TestSaveChunkRevisionIsAtomicAndOptimistic`、`TestTagFieldUpdatesReturnAllAffectedChunks`（+2 子）、`TestDiffFAQChunkIDsByContentHash*` ×4、`TestImageMultimodalHandle*` ×3 全 PASS；ImageMultimodal Handle→Drop→Finalize 日志链（image_multimodal.go:187/:203/:724）逐行复现。

## 4. 变更文件 vs 写权限逐条核对

**口径**：`git diff --name-only 6bda27b1d..HEAD`（6bda27b1d=节点分支点，plan §1 实测 base）共 42 文件 = 本节点自写 38 + K1.0 基线对齐 merge（3ff5febbb，Ruling WAVE-DEP-BASELINE）引入 4（b2-k0 产物：20-knowledge-program.md、evidence/reports b2-k0 两件、kbfreeze/freeze_test.go——非本节点自写；`git diff --name-only 3ff5febbb^1 3ff5febbb` 实测恰 4 文件，kbfreeze 包仅 freeze_test.go 一件）。另 merge-base(origin/main)=b1a3d6dd8 口径为 98 文件，其中 56 文件差额属分支点前的 ib1/b0 世系提交（passbguard 工具与 testdata、modulemove、DAG、他程序 plan/brief、Makefile 等，含 `tools/passbguard/passbguard` 二进制），非本节点产物（复核会话 comm 双向差集实测：42 节点侧 + 56 世系侧 = 98，两侧交集空）。首版误记 41/37/97，均差一或差读，已由 §9 复核会话勘误。

38 自写文件逐条核对（差集为空；构成 22 ingest + 5 shim + 6 治理 + 4 文档 + 1 README）：

| 文件（组） | 写权依据 |
|---|---|
| `internal/modules/knowledge/ingest/` 22 件（9 生产 + 11 随迁测试 + doc.go + seams.go） | plan §4.1 迁移表与新增表 |
| `internal/application/repository/chunk_ingest_shim.go`、`internal/application/service/chunk_ingest_shim.go`、`internal/application/service/span_trace_seam_adapter.go`、`internal/handler/chunk_ingest_shim.go` | plan §4.1（R1 shim §6.2/§6.4） |
| `internal/application/service/chunk_ingest_test_shim_test.go` | Ruling 2026-09-24-TEST-SUPPORT-SHIM（tshim-0001/0002，B5 清理范围，ledger 注释段台账） |
| `docs/architecture/moves/knowledge.yaml`、`docs/architecture/passb/ownership-matrix.yaml` | Ruling 2026-09-23-LEGACY-ROW-OWNERSHIP（9+9 删行）+ Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION（4+4 shim 补行） |
| `docs/architecture/passb/exception-ledger.yaml`、`tools/architectureguard/check.go` | Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY（6 数据行，exc-0106..0111） |
| `tools/passbguard/ownership_test.go` | Ruling TRANSITION-SHIM-ROW-REGISTRATION 第 2 条（wantPerModule knowledge 84→79 纯机械修正） |
| `docs/architecture/evidence/pass-a-acceptance.md` | 同裁定第 2 条台账登记（§1 计数行 + §6 表，先例 33f8c3ea3 同形） |
| `docs/architecture/passb/briefs/b2-k-ingest.md`、`docs/architecture/evidence/passb/b2-k-ingest.md`、`docs/plans/passb/reports/b2-k-ingest.md`（本文件）、`docs/plans/passb/21-knowledge-ingest.md`（计划本体 741cdb517 落盘） | plan §4.1 新增表 + conventions §1.1 |
| `internal/modules/knowledge/legacy/README.md` | **plan §4.2 禁改清单的登记例外**：OCR 工单 ocr-r1-f1（ocr-r1-legacy-readme-mirror，documentation·low）由协调者路由 K1.7 处置（ocr-context.md §六、K1.6 报告 §6-4）；本会话 commit fcb6bd449 删 9 已迁移行 + 补 4 过渡 shim 行，镜像核验 79==79 双向差集空且逐条同序 |

**验收 5（宿主属主文件零触碰）**：上述 38 文件中无 rbac_lookups.go、temporary_document.go、K2/K3/K4 属主宿主文件、router/container/bootstrap、go.mod/sum、migration——成立。

## 5. 未完成项与如实登记

1. **`go test ./internal/application/service/` 首轮 8 例 `TestQueryTemplates*` 失败（环境性，非本节点引入）**：根因 `sandbox: unsafe outbound URL: cannot resolve "api.e2b.app": lookup api.e2b.app: no such host`——外部 DNS 解析失败；同族单独复跑（`-run 'TestQueryTemplates'`）与 service 全包复跑均 `ok`（467.336s，退出码 0）；兄弟分支 b2-k-retrieval K2.1 T0 基线同族全 PASS（1.54s，网络可达时）。被测对象 tenant_sandbox_config（execution 域）与本节点 knowledge ingest 零关联。
2. **passbguard 60 条契约/事件漂移 + `go test ./tools/passbguard/` 全量在分支为既有红**：contracts.yaml/event-catalog.yaml 仅 barrier 可写（conventions §3），全部属 Brief §3 ib2 回写批；自 K1.1 显形，非本任务引入（K1.6 报告 §6-1/2 同项）。
3. **exc-0088 不删**：airesource 根门面零导出符号（实测），本节点无合法替代 import；ib2 删除批与前置已落 Brief §10（plan §7.3 处置）。
4. **计划评审文件 `docs/plans/passb/reviews/b2-k-ingest.md` 不存在**（前置 P5 登记）：计划以审校 F1 修正 + OCR 轮次（ocr_covered 7 区间）替代评审留痕；P5 的 approved 状态回填属协调者/评审者职责。
5. **PassBTask 口径 `B-knowledge` 偏差待协调者确认**：plan §7.2 字面 `21-knowledge-ingest` 不在 passbguard PassBTaskModule 映射表（tools/passbguard/check.go:83-93），沿 K2.3/K2.4 先例取 `B-knowledge`（K1.6 报告 §1-3，evidence §8 登记）。
6. **docs/docs.go swagger 生成物旧定义名**：纯字符串漂移，ib2/集成知悉（ocr-context.md §五-⑤）。
7. **exc-id 跨分支撞号可能**（本分支 0106..0111 vs 兄弟分支上界）：passbguard 判重以 (from,to) 边为键，集成侧按边键重排（K1.6 报告 §5-5 登记同项）。

## 6. 跨节点义务转办确认（plan §2 裁定 6）

- `kb_activity.go` 4 函数（kbActivityTrigger/withKBActivityTask/kbActivityAppendSampleTitles/recordKBActivity）导出义务：**b2-k-retrieval**（DAG notes F1）——本节点 9 文件与该 4 函数零调用关系（K1 会话 grep 实证，plan §2-6 转录）。
- `withKnowledgeCleanup`（knowledge_delete_plan.go:22）导出义务：**b2-k-process**（DAG required_contracts F1 补）——同上零调用。
- 别名 18 条删除：**K5/b2-k-integration**（knowledge.yaml:41-77；本节点零 import 变更涉别名）。
- 装配切换（container.go:205/:375/:409-411/:719 Provide 换 seam 接线版、路由切模块门面）：**集成工程师按本节点 Brief（K5/ib2）执行**——本节点以 shim 保旧装配可编译，ingest 新路径未接线不影响运行（plan 20 §11 回滚边界同律）。
- conversation 调用点改写（temporary_document.go:541/:560 直连 ingest 导出）：**ib2**（Brief §8 登记，本节点经 R1-10 shim 保编译）。
- identity 去方法化联动（rbac_lookups.go:103/:104/:130/:149 wrapper 删除批）：**ib2**（Brief §9）。

## 7. seam 具体化偏差登记（相对 plan §6.3 原案）

经实施/审校/OCR 修订的偏差（全部已在对应任务报告与 Brief 留痕，此处收口汇总）：

1. `SpanTraceSeam` 以 `any` 承载 `*Span` 不透明句柄（原案即此，无修订）；`tracker()` nil 回退语义对照 noopSpanTracker 实测（K1.2）。
2. **增量 seam**（K0 §6.2 未枚举、plan §6.3 已注明不构成新裁定）：`DataAnalysisToolSeam`/`GraphExtractorSeam`（K1.3 引入，消 agentruntime import 环，CYCLE-FORCED-COMPOSITION 同律；E3 例外因此无需登记，seams.go→airesource/models/chat 例外补登记）。
3. **orphan 测试哨兵孪生值**：image_multimodal_orphan_test.go 4 处 `repository.ErrKnowledgeNotFound/…BaseNotFound` 改测试本地 `errors.New` 孪生值注入同名字段（plan §6.3 末行原案；双跑 PASS 佐证比对语义不变）。
4. **K1.4-fix1/K1.5-fix1**：M2 转录漂移两批原文恢复（vlmOCRScannedPDFPrompt、Write 转录 9 行；a9cf368ed/3feed3bce）。

## 8. 提交清单（节点分支，6bda27b1d..HEAD 共 28 commits）

K1.0 对齐 merge 3ff5febbb + T0 fe8e5e459；K1.1 M2 2096b0cc5/M3 a603e37ea；K1.2 M2 afc0ee1eb/M3 635606502/ocr-r1-1 ec00305a3；K1.3 M2 5fc1bfbe8/M3 290157832/tshim-0002 0417f3952；K1.4 M2 450498c42/M3 1175b53cd/Brief eaa96e01d/fix1 a9cf368ed；K1.5 M2 38216e65d/M3 6c0bd6a0f/fix1 3feed3bce；K1.6 fc14f4c2e/253497b1f/8a6157b43；K1.7 README fcb6bd449 + 本 commit（差分证据与节点报告）。

## 9. K1.7 复核（重派会话，2026-09-25，BASE=64d1b4dc7）

K1.7 被调度方以 BASE=64d1b4dc7（首跑终 commit）重派；本会话对 K1.7 三 checkbox 全部检查独立复跑，并勘误首跑节点报告的计数错误（§2 #8、§4）：

| # | 命令（本会话原文执行） | 退出码 | 关键输出 |
|---|---|---|---|
| R1 | `go test -count=1 -v ./internal/modules/knowledge/ingest/ -run '<T0 全清单 17 模式合并>`（与首跑同串） | 0 | `ok …ingest 1.440s`；`grep -c '^--- PASS'`=40、`grep -c '^    --- PASS'`=25、FAIL=0；顶层 40 名 `sort`+`diff` 与 T0 转录清单**逐名相同**（diff 空）；子用例分布 2+2+14+2+5=25 与 T0 一致；五锚点族全 PASS，ImageMultimodal Handle→Drop→Finalize 日志链 image_multimodal.go:187/:203/:724 逐行复现 |
| R2 | `ls`（9 legacy 路径全列） | 1（预期） | 9 行全部 `No such file or directory` |
| R3 | `go build ./...`（gate 1） | 0 | 仅既有 `ld: warning: ignoring duplicate libraries: '-lc++'` |
| R4 | `go test -count=1 ./internal/modules/knowledge/...`（gate 2） | 0 | 17 有测试包全 `ok`（ingest 4.099s、kbfreeze 3.740s 等）+4 `[no test files]` |
| R5 | `make check-backend-architecture`（gate 3） | 0 | `literal=564 apiKeyRoute=69 handle=0 total=633 \| redis=23 lite=23 \| hooks=58 \| modules=16`；`OK (0 violations)` |
| R6 | `make verify-module-moves`（gate 4） | 0 | `modulemove: OK (16 manifests verified)` |
| R7 | `PASSB_BASE_SHA=$(git merge-base origin/main HEAD)`→`b1a3d6dd8`；`git diff --stat/--name-only ...HEAD` | 0 | **98 files, +22252/−336**；`comm` 双向差集拆分 = 42 节点侧（6bda27b1d..HEAD）+ 56 分支点前 ib1/b0 世系，交集空；42 = 38 自写 + 4 K1.0 merge（`git diff --name-only 3ff5febbb^1 3ff5febbb` 恰 4 行）——与 §4 逐条核对结论一致，owned_files 差集空 |

**复核结论**：K1.7 三 checkbox（差分双跑等价 / gates 四项 / evidence+报告落盘）在重跑下全部成立；首跑节点报告 §2 #8（97/+22136）与 §4（41/37）计数有误，本 commit 勘误为 98/+22252 与 42/38——纯粹计数勘误，逐条枚举与差集核对结论（无越权文件）不变。未复跑项如实声明：验收 6 的 `go test -count=1 ./internal/handler/ ./internal/application/...`（约 18 分钟全程）首跑已留痕（§2 #3），本会话未重跑；T0 旧位置侧不可复跑（9 文件已迁移，比对基准即 evidence 转录，符合 K1.7「以 K1.0 T0 用例清单为准」的语义）。
