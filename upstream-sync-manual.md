# 未来同步上游 WeKnora 操作手册

适用对象：本 fork（origin = `1123786563/WeKnora-fork01`）的维护者，向 upstream（`Tencent/WeKnora`）周期性同步时照做。

事实来源：
- 一次针对 `upstream/main` 最近 10 个提交（基线 `bccb4b151`，至 `9fa9e7c29`）的 46 文件三方合并演练（`git merge-file`），结果：18 个干净合并 + 12 个上游新增文件（天然干净）+ **16 个冲突文件、共 51 个冲突块**；
- 本手册编写时在仓库内实测核实的命令与台账路径（见各节标注）；
- 仓库现状：`main` 分支已完成后端目录对齐上游，`internal/` 对齐率约 97.8%，因此未来冲突应主要是**内容级冲突**（同区域两套实现），而非目录搬移/重命名冲突。

---

## 1. 标准同步流程

### 步骤 0：前置检查

```bash
git status --porcelain        # 必须为空；有未提交改动先处理
git checkout main
git pull origin main
```

### 步骤 1：拉取上游并预览

```bash
git fetch upstream
git log --oneline main..upstream/main                 # 待合入的提交清单
git diff --stat main upstream/main -- internal/ cmd/ config/ migrations/ dataset/ docreader/ mcp-server/
```

> 实测：`git remote -v` 确认 upstream 指向 `https://github.com/Tencent/WeKnora.git`；`main` 与 `upstream/main` 的 merge-base 当前为 `bccb4b151`（即上次同步点）。

### 步骤 2：执行合并

推荐先在专用同步分支上做，门控全绿后再快进回 `main`（出了问题直接弃分支，`main` 零污染）：

```bash
git checkout -b sync/upstream-$(date +%Y%m%d) main
git merge upstream/main --no-ff
```

若不想多一层分支，也可直接在 `main` 上 `git merge upstream/main --no-ff`（回滚手段见第 4 节）。`--no-ff` 保留显式合并节点，便于日后 `git revert -m 1` 整体撤销。

### 步骤 3：冲突处理原则

目录已对齐，冲突会是**同一文件同一区域的两套内容**。按以下优先级裁决，逐条遵守：

1. **fork 功能与上游修复并存**：fork 自研功能（如 `qa.go` 的 T15 `reasoning_mode` 校验、`claimTurns`）与上游修复（如 `uploadOnlyQuery`/SSRF 清理）语义不重叠时，**两边都保留**，只做文本拼接。
2. **等价修复二选一**：fork 已独立移植过与上游同号的修复（演练实证：milvus `CopyIndices` #3883、notion 查询截断 #3845/#3836、qdrant/es 整批失败 #3847/#3842、sqlite FTS5 可用性跟踪等），但命名/接口不同。此时**择一保留**——默认取上游形态（减少未来 diff），仅当 fork 版有上游没有的能力时保留 fork 版并把增量叠加上去；另一侧整体删除，禁止两套并存。
3. **接口极性相反的实现必须显式裁决**（最大风险点，见第 2 节 A/B 类）：如 notion `connector.go` 的「返回 err 中止轮次」vs 上游「truncated 布尔聚合」、sqlite 的 `ftsReady` vs `ftsUnavailable`。不允许各留一半调用点，四个调用点必须统一到同一套接口。
4. **add/add 冲突**（上游新增文件 vs fork 移植版同名文件，如 `query_truncation_test.go`、`batch_update_test.go`）：逐段对比，保留并集断言；演练显示此类多为结构/注释措辞差异，内容近同。
5. 每解决完一个文件：
   ```bash
   grep -c '^<<<<<<<' <文件>    # 必须为 0（演练即用此法复核冲突块数）
   gofmt -w <文件>
   git add <文件>
   ```
6. 全部冲突解决后先跑 `git diff --check` 确认无残留标记，再进入门控。

### 步骤 4：门控（构建 + 静态检查 + 定向测试）

```bash
go build ./...
go vet ./...
```

定向测试按本次合并实际触碰的包选跑（对照第 2 节风险清单，最小集）：

```bash
go test ./internal/datasource/connector/notion/... -count=1
go test ./internal/application/repository/retriever/... -count=1
go test ./internal/agent/tools/... -count=1
go test ./internal/modelcontext/... -count=1
go test ./internal/handler/session/... -count=1
```

> 实测：`go build ./...` 在 `main` 基线上 exit 0（仅 `ld: warning: ignoring duplicate libraries: '-lc++'` 链接器警告，属既有噪音）。`go vet ./...` 为门控项但本手册编写会话未运行，首次执行时以实际结果为准。

### 步骤 5：守卫三工具——**基线对比，不新增诊断**

三个守卫在当前 `main` 基线上**均有既有诊断**（编写会话实测：architectureguard 报 `routes_chat.go` 路由重复注册等；passbguard 报 workbench 模块若干 legacy-missing；modulemove 报 career/commercial 模块源文件缺失；三者 exit 1 属基线现状）。因此门控不是「必须全绿」，而是**合并前后输出 diff 为空**：

```bash
# 合并前（步骤 1 之后、步骤 2 之前）在 main 上存基线：
go run ./tools/architectureguard 2>&1 | tee /tmp/ag-before.txt
go run ./tools/passbguard -root . 2>&1 | tee /tmp/pb-before.txt
go run ./tools/modulemove verify --all 2>&1 | tee /tmp/mm-before.txt

# 冲突全部解决后重跑并对比：
go run ./tools/architectureguard 2>&1 | tee /tmp/ag-after.txt
go run ./tools/passbguard -root . 2>&1 | tee /tmp/pb-after.txt
go run ./tools/modulemove verify --all 2>&1 | tee /tmp/mm-after.txt
diff /tmp/ag-before.txt /tmp/ag-after.txt
diff /tmp/pb-before.txt /tmp/pb-after.txt
diff /tmp/mm-before.txt /tmp/mm-after.txt     # 三份 diff 均必须为空
```

若出现**新增**诊断：多半是上游新文件落进了守卫管辖目录而未登记台账——先按第 3 节登记，登记后仍无法归零的列入第 4 节求助情形。

### 步骤 6：提交合并

```bash
git add -A
git commit            # 完成 merge commit（默认信息即可，或注明同步区间）
# 若走的是 sync/ 分支：门控全绿后
git checkout main && git merge --ff-only sync/upstream-YYYYMMDD
git push origin main
```

---

## 2. 已知的同步风险面（来自 46 文件演练）

演练覆盖 `upstream/main` 最近 10 提交中 `internal/`、`cmd/`、`config/`、`migrations/`、`dataset/`、`docreader/`、`mcp-server/` 的全部变更文件；18 个三方干净合并、12 个本地不存在（**全部为上游新增文件，F^ 基线中亦无，`notPresentLocally` 中 fork 已删/移走的风险文件为 0——即无 modify/delete 风险面**）、16 个冲突文件 51 个冲突块。冲突文件均在 `main` 上实测存在（`git cat-file -e main:<path>` 逐一核实）。

### A 类：等价修复、不同实现（择一，删除另一侧）

| 文件 | 块数 | 原因与处置 |
|---|---|---|
| `internal/application/repository/retriever/milvus/repository.go` | 2 | fork 已有 #3883 同款 `CopyIndices` 分块读（chunkBatchSize=64），上游同修复不同代码形态落同函数。择一。 |
| `internal/application/repository/retriever/qdrant/repository.go` | 1 | fork 的 #3835 修复（`failed==searched` 判定）vs 上游 #3847（`matchedCollections` 判定），同函数两套「整批失败返回 error」。择一。 |
| `internal/application/repository/retriever/elasticsearch/v7/repository.go` | 2 | fork 详细版 `_bulk` 错误检查 vs 上游 #3842 精简版（`bulkResponse.Errors` 字段）。择一。 |
| `internal/application/repository/retriever/elasticsearch/v8/repository.go` | 2 | 同 v7。择一。 |
| `internal/datasource/connector/notion/client.go` | 1 | fork 导出错误 `ErrQueryTruncated`+`maxPaginationHops`/`maxEmptyResultPages` 常量 vs 上游私有 `errQueryResultTruncated`，同一常量/错误定义区。择一（注意与 B 类 connector.go 联动，错误类型要跟接口裁决走）。 |
| `internal/datasource/connector/notion/types.go` | 1 | fork 的 `paginatedResponse.truncated()` vs 上游 `isIncomplete()`，同一截断判断逻辑不同名。择一。 |
| `internal/application/repository/retriever/sqlite/batch_update_test.go` | 1 | add/add：上游 `72b8a106c` 新增 222 行 vs fork 223 行移植版，唯一冲突是注释措辞。任取一份。 |
| `dataset/qa_dataset.py` | 1 | fork 默认 `model="gpt-4o"` vs 上游 #3965 `DEFAULT_QA_MODEL` 常量。取上游常量化形态。 |

### B 类：接口极性/形态相反（最大风险，需显式裁决并统一调用点）

| 文件 | 块数 | 原因与处置 |
|---|---|---|
| `internal/datasource/connector/notion/connector.go` | **13** | fork 已实现 #3836/#3845 同款修复但接口不同：fork 让 `fetchPage`/`fetchDatabaseIncremental` 返回 err 并中止轮次；上游返回 truncated 布尔标志、聚合后统一处理。四处调用点同区域两套实现。**必须整体二选一**，逐个调用点核对，禁止混用。文件体量 ours/base/theirs=1091/987/1056 行，属同区域冲突而非整文件分叉。 |
| `internal/application/repository/retriever/sqlite/repository.go` | **12** | fork `ftsReady` 字段方案 vs 上游 `ftsUnavailable`，语义极性相反、命名不同，FTS5 可用性跟踪的字段/判断/赋值点全部冲突。择一后全文件统一极性。 |
| `internal/datasource/connector/notion/query_truncation_test.go` | 4 | add/add：上游 #3845 新增 238 行测试 vs fork 373 行移植版，结构不同。按保留的 B 类实现裁剪测试集，断言并集。 |

### C 类：同区域各自展开（两边语义都要保，人工合并）

| 文件 | 块数 | 原因与处置 |
|---|---|---|
| `internal/agent/tools/query_knowledge_graph.go` | **11** | fork `graphSearchTerms`（词项大小写处理）与上游 #3885 `graphTruncation` 结构体同文件多处各自展开（ours/base/theirs=947/742/878 行）。两套功能并存，逐块拼接。 |
| `internal/application/repository/retriever/qdrant/repository_test.go` | 5 | 双方都大幅扩测（521→865 vs →764 行）：fork 已含 #3835 断言，上游新增 `os/slices` import 与新用例同区域。测试并集，import 去重。 |
| `internal/handler/session/qa.go` | 3 | fork 的 T15 `reasoning_mode` 校验、`claimTurns` 等本地功能与上游 `bccb4b151` 的 `uploadOnlyQuery`/SSRF 清理同在 `parseQARequest` 区域。两边都保留。 |
| `internal/handler/session/types.go` | 1 | fork 本地扩展（103→136 行）与上游 `bccb4b151` 新增字段（→121 行）同声明区插入。字段并集。 |
| `internal/modelcontext/model_output.go` | 1 | fork 版（880 行）与上游 #3885 版（916 行）在 model 输出截断可见化同方法区各自修改。按上游形态为准、保留 fork 增量。 |

**排序建议**：先解 A 类（机械择一）→ 再解 B 类（notion 三文件必须一起裁决，接口/错误类型/测试三者联动）→ 最后 C 类（逐块拼接）；B 类每一处裁决在 merge commit 信息里记一行「取了谁、为什么」。

---

## 3. 守卫台账的联动维护

合并后守卫出现**新增**诊断，几乎都是「上游新文件落进管辖目录但台账未登记」。登记位置（均为实测核实）：

- **搬迁/所有权 manifest**：`docs/architecture/moves/*.yaml`（17 份，schema 见 `docs/architecture/moves/README.md`）。`modulemove`（`ManifestsDir` 定义于 `tools/internal/movemanifest/manifest.go:17`）与 `architectureguard`（`tools/architectureguard/check.go:311` 的 `LoadManifestViews` 同样读取该目录）共用。上游合并带来的新路由要登记到对应模块 `integration_points.routes`，新文件落入 `legacy_files`/`owned_files`。
- **import 例外台账**：`docs/architecture/passb/exception-ledger.yaml`。以 `(from, to)` 边为键（`from`=ImporterFile、`to`=ImportedPath），`reason` 逐字、`remove_at` 为属主计划阶段屏障；`exc-id` 跨并行分支独立续号，撞号按边键重排。文件头注有完整的编号线索，改前先读。
- **冻结契约**：`docs/architecture/passb/contracts.yaml`（`symbol` 为 `file:Name` 顶层声明地址，`stability: frozen`）。若上游合并改变了契约签名/消费方/组合面，**须走串行契约修订 + 架构审查**，不允许在 merge 冲突解决中静默改约。
- passbguard 的治理目录常量 `GovernanceDir = "docs/architecture/passb"` 见 `tools/passbguard/load.go:18`。

操作顺序：解冲突时若动了受契约保护的符号 → 先在台账/契约里登记或发起修订 → 重跑步骤 5 的三守卫 → diff 归零才提交。**台账登记与代码合并放进同一个 merge commit**，避免「代码先进、台账欠账」。

---

## 4. 回滚与求助

**回滚**：

```bash
# 合并进行中（冲突解不下去 / 门控过不了）：
git merge --abort

# 合并已提交、尚未 push：
git reset --hard ORIG_HEAD          # 或 git reset --hard main@{1}

# 合并已 push（整棵撤销，保留历史）：
git revert -m 1 <merge-commit-sha>
```

**以下情形停下来找人工 review，不要自行拍板**：

1. B 类接口裁决：`notion/connector.go`、`sqlite/repository.go` 这类极性/接口相反的择一——选错会把 fork 的中止语义或 FTS5 行为静默丢掉；
2. 守卫新增诊断登记台账后仍无法归零，或涉及 `contracts.yaml` 冻结契约签名变化（需走契约修订流程而非个人裁决）；
3. 单文件冲突块 ≥ 5 且双方语义都需保留（`query_knowledge_graph.go`、`qdrant/repository_test.go`），合并结果建议二次读审；
4. 上游出现目录结构调整类提交（未来 `internal/` 布局再变）——那会破坏「内容级冲突」前提，需先评估是否重跑一轮目录对齐（参照 `restructure/upstream-align-round2` 分支的既有做法）而非硬 merge；
5. 门控任何一项（build / vet / 定向 test / 守卫 diff）失败且原因不明——先 `git merge --abort` 保住基线，再带 `/tmp/*-before|after.txt` 与失败输出找人。

---

*手册编写会话的实测记录：`git remote -v`、`git log upstream/main -5 --oneline`、`git merge-base main upstream/main`、16 个冲突文件 `git cat-file -e main:<path>` 存在性、三个守卫命令在 `main` 临时 worktree 上的基线输出、`go build ./...`（exit 0）均已实际执行；`go vet ./...` 与定向 `go test` 未在该会话运行。*
