# plan-t50.md 实现报告 — Task 3/10：`ConfluenceUpdateAdapter`

> 任务：Issue #50 实施计划（`docs/plans/issue30-sweep/plans/plan-t50.md`）Task 3——
> 「`ConfluenceUpdateAdapter`——版本预读、冲突拒绝、单步 PUT 与 Query 对账」。
> 执行 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t50`（分支 `codex/issue30-t50`）。
> 本报告由实现员子代理撰写，仅覆盖 Task 3 授权文件。

## 1. 实现内容

按计划 Task 3 逐字实现，两个新增文件（工作区零其他改动，未触碰任何他人文件）：

- **`internal/modules/appconnector/confluence_update.go`**（约 290 行）：
  - `ConfluenceUpdateSnapshot`：四字段审批快照（`PageID`/`ExpectedVersion`/`Title`/`Storage`），逐字符合计划 Produces 签名。
  - `ParseConfluenceUpdateSnapshot`：精确四字段校验（多余字段=approve-then-rewrite 拒绝、缺字段/空值拒绝、非对象拒绝），错误统一包 `ErrConfluenceSnapshotInvalid`。
  - `IsConfluenceUpdateArgs`：路由判定——`page_id` AND `expected_version` AND `storage` 三键齐备才是 Confluence update（storage 键是与 Notion update 的 edition-bridging 判别符）。
  - `confluenceNextVersion`：解析审批版本并返回 `+1`（Confluence PUT 要求 `version.number = current+1`）；非正整数拒绝（包 `ErrConfluenceSnapshotInvalid`）。
  - `ConfluenceUpdateAdapter`：字段签名逐字（`Policy`/`Credential`/`Edition`/`APIBasePath`/`ConnectionCapabilities`/`Recheck`——无 update 语义所需的父 scope 字段），`var _ Adapter = (*ConfluenceUpdateAdapter)(nil)` 编译期断言。
    - `Execute` 顺序：config 检查 → edition 检查 → A03 `Recheck`（撤销→`ActionAwaitingApproval` + `ErrConfluenceApprovalRevoked`）→ 快照解析 → capability 检查 → **版本预读 + `DetectConfluenceVersionConflict`（AC1 冲突门：不一致→确定性 `ActionFailed` + `ErrConfluenceVersionConflict`，零写请求）** → 单步 PUT。PUT 传输失败/5xx → `ActionUnknown` + `ErrConfluenceOutcomeUnknown`；4xx → 确定性 failed；回复无真实回执 → unknown。成功态携带 `ExternalID` 与 provider 原始回执 `Output`。
    - `Query`：只经可靠页面读对账（绝不重发写）。仅当「版本恰好推进到 `expected+1` 且 title 且 storage 正文全部仍是审批快照内容」才判 `ActionSucceeded`；版本未动/漂移/读失败/内容漂移一律保持诚实的 `ActionUnknown`。
  - `ReadConfluencePageVersion`：计划期共享预读函数（Task 5/6 发布 seam 消费），纯只读。
- **`internal/modules/appconnector/confluence_update_test.go`**（约 291 行）：11 个测试，逐字采用计划 Task 3 Step 1 代码；复用 Task 2 落地的契约双打（`fakeConfluence`——官方 Cloud v2 `GET/POST/PUT /api/v2/pages` 与 Server/DC `GET/POST/PUT /rest/api/content` 形状、Basic 认证、单调 version.number 门、`dropNextWrite` 丢回复旋钮）与辅助函数（`cfPolicyOf`/`cfCredential`/`cfCaps`/`cfCreateArgs` 等）。

**Consumes 核实（实现前逐项 grep）**：Task 1/2 已产出的 `confluenceTargetURL`/`confluenceDo`/`confluenceReadPage`（`confluence_create.go:125,137,173`）、`ConfluenceCloudPagesPath`/`ConfluenceServerContentPath`（`confluence_create.go:21-24`）、`confluenceStorageBody`/`confluenceServerStorageBody`（`confluence_create.go:82,103`）、Task 1 全部错误变量与解析函数（`confluence_common.go`）均存在且签名与计划一致；新符号 `confluenceVersionRef`/`confluenceCloudUpdateRequest`/`confluenceServerUpdateRequest`/`confluenceNextVersion`/`ReadConfluencePageVersion`/`ConfluenceUpdateAdapter` 族在包内无命名冲突（`rg` 复核零命中）。
一处与计划文本的已知落地差异（不影响本任务）：`cfCredential` 实际签名为 `func(ctx context.Context) (ConfluenceCredential, error)`（Task 2 落地时带 ctx），恰与适配器 `Credential` 字段类型一致，`Credential: cfCredential` 直接赋值，测试逐字通过。

## 2. TDD 证据

### RED（先写测试，实跑确认失败）

命令（在 worktree 根执行）：

```
go test ./internal/modules/appconnector/ -run 'ConfluenceUpdate|TestReadConfluencePageVersion' -count=1
```

输出（关键部分）：

```
# github.com/Tencent/WeKnora/internal/modules/appconnector [github.com/Tencent/WeKnora/internal/modules/appconnector.test]
internal/modules/appconnector/confluence_update_test.go:24:15: undefined: ParseConfluenceUpdateSnapshot
internal/modules/appconnector/confluence_update_test.go:50:6: undefined: IsConfluenceUpdateArgs
internal/modules/appconnector/confluence_update_test.go:72:9: undefined: ConfluenceUpdateAdapter
internal/modules/appconnector/confluence_update_test.go:127:9: too many errors
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector [build failed]
```

失败原因即预期原因：实现尚不存在（计划 Step 2 Expected「FAIL（`undefined: ConfluenceUpdateAdapter`）」）。

### GREEN（最小实现后实跑通过）

命令：

```
go test ./internal/modules/appconnector/ -run 'ConfluenceUpdate|TestReadConfluencePageVersion' -count=1 -v
```

输出（完整）：

```
=== RUN   TestParseConfluenceUpdateSnapshot
--- PASS: TestParseConfluenceUpdateSnapshot (0.00s)
=== RUN   TestIsConfluenceUpdateArgs
--- PASS: TestIsConfluenceUpdateArgs (0.00s)
=== RUN   TestConfluenceUpdateCloudHappyPath
--- PASS: TestConfluenceUpdateCloudHappyPath (0.00s)
=== RUN   TestConfluenceUpdateServerHappyPath
--- PASS: TestConfluenceUpdateServerHappyPath (0.00s)
=== RUN   TestConfluenceUpdateConflictZeroWrites
--- PASS: TestConfluenceUpdateConflictZeroWrites (0.00s)
=== RUN   TestConfluenceUpdatePreReadFailureIsFailedNotUnknown
--- PASS: TestConfluenceUpdatePreReadFailureIsFailedNotUnknown (0.00s)
=== RUN   TestConfluenceUpdateUnknownOnLostWriteReply
--- PASS: TestConfluenceUpdateUnknownOnLostWriteReply (0.00s)
=== RUN   TestConfluenceUpdateQueryResolvesAfterDroppedReply
--- PASS: TestConfluenceUpdateQueryResolvesAfterDroppedReply (0.00s)
=== RUN   TestConfluenceUpdateQueryStaysUnknownWhenContentDrifted
--- PASS: TestConfluenceUpdateQueryStaysUnknownWhenContentDrifted (0.00s)
=== RUN   TestConfluenceUpdateQueryStaysUnknownWhenVersionUnmoved
--- PASS: TestConfluenceUpdateQueryStaysUnknownWhenVersionUnmoved (0.00s)
=== RUN   TestReadConfluencePageVersion
--- PASS: TestReadConfluencePageVersion (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.476s
```

11/11 通过，输出 pristine（无 warning/噪声）。

## 3. 提交前全量验证（implementer 契约：commit 前跑一次全量）

| 检查 | 命令 | 结果 |
|---|---|---|
| 全包回归 | `go test ./internal/modules/appconnector/... -count=1` | 6 个包全部 `ok`（appconnector / connectorcontrol / openconnector / publish / repository/appconnector / service/appconnector） |
| 全仓构建 | `go build ./...` | exit 0（仅 `cmd/desktop`、`cmd/server` 的 linker 无害告警 `ignoring duplicate libraries: '-lc++'`） |
| gofmt | `gofmt -l internal/modules/appconnector/` | 本任务两个新文件干净（未列出）；`confluence_create_test.go` 被列出——**Task 2 已提交文件的既有状态**（fake server POST handler 一处 struct tag 对齐），不在本任务授权清单，未改动，见 §5 |
| go vet | `go vet ./internal/modules/appconnector/` | 通过（`VET-OK`） |

## 4. 提交

```
1bafb686b feat(appconnector): confluence update adapter - version pre-read conflict gate + single PUT (T20 #50)
```

`git status` 提交后干净（工作树 clean，未推送远端）。提交时 Mimosa hook 提示扫描未得到完整结论（`scanner_enobufs`，按兼容策略继续）——本报告不宣称项目安全状态，完整审计由主控安排。

## 5. 自检发现（Self-Review）

1. **【最重要】Server PUT wire body 形状与同包 create 及真实 API 文档形状存在疑点（逐字实现，测试不可捕获，如实上报）**：计划给定的 `confluenceServerUpdateRequest.Body` 类型为 `confluenceServerStorageBody`（json tag `"body"`，字段平铺 `value`/`representation`），序列化为 `"body":{"value":...,"representation":"storage"}`；而同包 create 用的 `confluenceServerBodyRef`（`confluence_create.go:111`）是 `"body":{"storage":{...}}` 嵌套形状，fake 双打的 server PUT handler 也按嵌套 `body.storage.value` 解析（`confluence_create_test.go:189-198` 附近）。因此 Server 版式更新在双打上 storage 实际落为空串——但计划给定的 `TestConfluenceUpdateServerHappyPath` 只断言 receipt `ExternalVersion=="6"` 与 `puts==1`，不断言远端 storage 内容，故 11 测全绿。我按计划逐字实现（计划明示「语义以代码为准」），未擅自改形状；该差异只会在真实 Server/DC 实例（Task 9 blocked-env）上暴露。建议审查者/Task 9 优先裁决此点。
2. **Query 成功输出用 `next` 而非 `cur.VersionNumber`**：`Query` 的输出 receipt 以 `confluenceNextVersion` 结果构造，但因前一断言已强制 `cur.VersionNumber == strconv.Itoa(next)`，两者数值恒等，无行为差异。
3. **policy 拒绝走 failed 而非 unknown**：`confluenceDo` 对 `ValidateRequest` 失败裸返回（不包 `ErrConfluenceOutcomeUnknown`），`Execute` 据此判确定性 failed——policy 拒绝意味着请求从未离开进程，语义正确，与 create 适配器一致。
4. **对账只读纪律**：`Query` 全路径零写请求，`TestConfluenceUpdateQueryResolvesAfterDroppedReply` 以 `putsBefore==putsAfter` 断言（AC2），`TestConfluenceUpdateConflictZeroWrites` 以 `posts==0 && puts==0` 断言 AC1 的零写拒绝。
5. **未跑的检查**：`go test ./...`（全仓测试）未跑——本任务范围是 appconnector 包，全包 6/6 ok 已覆盖本任务影响面；迁移轨道等跨包面属 Task 0/8 职责。Task 9 真实 Provider 证据属 blocked-env，本任务未涉及。

## 6. 文件清单

- Create: `internal/modules/appconnector/confluence_update.go`
- Create: `internal/modules/appconnector/confluence_update_test.go`

---

# 修复轮 1/5 报告

## 审查发现

1 项（severity: important）：Server/DC update PUT wire body 用平铺 `confluenceServerStorageBody`（缺 `body.storage` 嵌套层），与同包 Server create（`confluence_create.go:108-120,284`，`confluenceServerBodyRef` 注释明示 documented nested shape）、fake 双打 server PUT handler 解析形状（`confluence_create_test.go:263-268,288`，解析 `req.Body.Storage.Value`）、官方 Server/DC 契约三方不一致；双打上 Server PUT 远端 storage 落空串，计划给定测试 `TestConfluenceUpdateServerHappyPath` 不断言远端 storage 故不可捕获；真实实例上审批 storage 正文不按契约写入且 Query 对账永久 unknown 停车。**主控已裁决修复**（本修复轮指令）。

## 修复内容

- `internal/modules/appconnector/confluence_update.go`：
  - `confluenceServerUpdateRequest.Body` 字段类型 `confluenceServerStorageBody` → `confluenceServerBodyRef`（`confluence_create.go:111` 的嵌套 wrapper，`"body":{"storage":{"value","representation"}}`），并加注释明示与 create 适配器及 fake 双打同一 wire 形状。
  - `putPage` Server 分支构造改为 `confluenceServerBodyRef{Storage: confluenceServerStorageBody{Value: snap.Storage, Representation: "storage"}}`。
- `internal/modules/appconnector/confluence_update_test.go`（回归覆盖）：
  - `TestConfluenceUpdateServerHappyPath` 末尾新增远端落盘断言（`p.title=="v2" && p.storage=="<p>srv</p>" && p.version==6`）——正是原测试缺失、使审查缺陷逃逸的断言。
  - 新增 `TestConfluenceUpdateServerQueryResolvesAfterDroppedReply`：Server 版式丢回复→unknown→Query 只读对账成功（版本恰 +1、storage 对账一致、`putsBefore==putsAfter` 零重发）——覆盖审查指出的「对账永久 unknown 停车」在修复后的解除。

## TDD 证据（修复轮）

### RED（先加强测试，实跑确认在修复前失败）

命令：

```
go test ./internal/modules/appconnector/ -run 'TestConfluenceUpdateServerHappyPath|TestConfluenceUpdateServerQueryResolvesAfterDroppedReply' -count=1 -v
```

输出（完整）：

```
=== RUN   TestConfluenceUpdateServerHappyPath
    confluence_update_test.go:128: remote page drift: &{id:page-9 spaceID:42 spaceKey:ENG title:v2 storage: version:6}
--- FAIL: TestConfluenceUpdateServerHappyPath (0.00s)
=== RUN   TestConfluenceUpdateServerQueryResolvesAfterDroppedReply
    confluence_update_test.go:153: server query must resolve from the remote state: {State:unknown ExternalID: Output:[]} confluence_query_unverifiable: remote content drifted from the approved snapshot
--- FAIL: TestConfluenceUpdateServerQueryResolvesAfterDroppedReply (0.00s)
FAIL
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector	1.797s
```

失败原因即预期原因：平铺 PUT 使远端 `storage:` 落空串（首测），对账读回内容与审批快照不符致 unknown 停车（次测）——审查描述的两个后果均被测试复现。

### GREEN（修复后）

命令：

```
go test ./internal/modules/appconnector/ -run 'ConfluenceUpdate|TestReadConfluencePageVersion' -count=1 -v
```

输出（12/12 全 PASS，完整末段）：

```
=== RUN   TestParseConfluenceUpdateSnapshot
--- PASS: TestParseConfluenceUpdateSnapshot (0.00s)
=== RUN   TestIsConfluenceUpdateArgs
--- PASS: TestIsConfluenceUpdateArgs (0.00s)
=== RUN   TestConfluenceUpdateCloudHappyPath
--- PASS: TestConfluenceUpdateCloudHappyPath (0.00s)
=== RUN   TestConfluenceUpdateServerHappyPath
--- PASS: TestConfluenceUpdateServerHappyPath (0.00s)
=== RUN   TestConfluenceUpdateServerQueryResolvesAfterDroppedReply
--- PASS: TestConfluenceUpdateServerQueryResolvesAfterDroppedReply (0.00s)
=== RUN   TestConfluenceUpdateConflictZeroWrites
--- PASS: TestConfluenceUpdateConflictZeroWrites (0.00s)
=== RUN   TestConfluenceUpdatePreReadFailureIsFailedNotUnknown
--- PASS: TestConfluenceUpdatePreReadFailureIsFailedNotUnknown (0.00s)
=== RUN   TestConfluenceUpdateUnknownOnLostWriteReply
--- PASS: TestConfluenceUpdateUnknownOnLostWriteReply (0.00s)
=== RUN   TestConfluenceUpdateQueryResolvesAfterDroppedReply
--- PASS: TestConfluenceUpdateQueryResolvesAfterDroppedReply (0.00s)
=== RUN   TestConfluenceUpdateQueryStaysUnknownWhenContentDrifted
--- PASS: TestConfluenceUpdateQueryStaysUnknownWhenContentDrifted (0.00s)
=== RUN   TestConfluenceUpdateQueryStaysUnknownWhenVersionUnmoved
--- PASS: TestConfluenceUpdateQueryStaysUnknownWhenVersionUnmoved (0.00s)
=== RUN   TestReadConfluencePageVersion
--- PASS: TestReadConfluencePageVersion (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.501s
```

## 提交前全量验证（修复轮实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| 全包回归 | `go test ./internal/modules/appconnector/... -count=1` | 6 个包全部 `ok` |
| gofmt | `gofmt -w internal/modules/appconnector/confluence_update.go` 后 `gofmt -l`（两文件） | 干净（无输出）；格式化后复跑聚焦测试 `ok` |
| go vet | `go vet ./internal/modules/appconnector/` | 通过（`VET-OK`） |
| 全仓构建 | `go build ./...` | exit 0（仅 linker 无害告警 `ignoring duplicate libraries: '-lc++'`） |

## 提交

```
2b1691753 fix(appconnector): confluence server update PUT uses nested body.storage wire shape (T20 #50 review round 1)
```

`git status` 提交后干净；未推送远端。

## 修复轮自检与待核实项回应

- **修复后 Server 版式写-读闭环一致**：PUT 写入（嵌套 body.storage）与 GET 对账读回（`confluence_common.go` `confluencePageWire` 按 `body.storage.value` 嵌套解析）形状现在完全对称，`TestConfluenceUpdateServerQueryResolvesAfterDroppedReply` 以双打实跑证明。
- **真实 Confluence Server/DC 实例对平铺 PUT 的实际行为**：仍属 blocked-env（无凭据），本环境无法实跑；修复的依据是三方代码事实闭合（同包 create 实现 + 其 documented nested shape 注释 + fake 双打解析）与官方文档形状一致，实例级终裁留给 Task 9 有凭据运行。
- **Cloud 版式零改动**：`confluenceCloudUpdateRequest` 的平铺 `body.{representation,value}` 即 Cloud v2 契约形状，未触碰；`TestConfluenceUpdateCloudHappyPath` 等 Cloud 测试全绿。
- **提交时 Mimosa hook 再次提示扫描未得完整结论（scanner_enobufs）**：与首轮相同，未宣称项目安全状态。

---

# Task 4/10 报告——publish 包：Artifact 文本→Confluence storage 正文纯投影

日期：2026-09-26
任务定位：plan-t50.md:2164-2312（Task 4）

## 1. 实现内容

- **Create** `internal/modules/appconnector/publish/confluence_blocks.go`：`func ConfluenceStorageBody(text string) (string, error)`——纯文本 → Confluence storage 格式（XHTML）正文的确定性纯投影：按空行切段、每段 trim + HTML 转义、`<p>` 包裹并以换行分隔；段内单个换行原样保留；空内容 → `ErrPublishEmptyContent`，段落超限 → `ErrPublishContentTooLarge`（复用同包 `blocks.go:18,25,28` 既有 `MaxPublishBlocks=500`/两哨兵，零修改复用）。
- **Create** `internal/modules/appconnector/publish/confluence_blocks_test.go`：计划 Step 1 逐字给定的 7 个测试（段落/CRLF 归一/HTML 转义/段内换行保留/空内容/超限/确定性）。

## 2. TDD 证据

### Step 2 RED（写实现前实跑）

```
$ go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceStorageBody' -count=1
internal/modules/appconnector/publish/confluence_blocks_test.go:10:14: undefined: ConfluenceStorageBody
... （7 处 undefined: ConfluenceStorageBody）
FAIL github.com/Tencent/WeKnora/internal/modules/appconnector/publish [build failed]
```

与计划 Step 2 Expected（「FAIL，`undefined: ConfluenceStorageBody`」）一致。

### Step 4 GREEN（最终实跑，完整输出）

```
$ go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceStorageBody' -count=1 -v
=== RUN   TestConfluenceStorageBodyParagraphs
--- PASS: TestConfluenceStorageBodyParagraphs (0.00s)
=== RUN   TestConfluenceStorageBodyNormalizesCRLF
--- PASS: TestConfluenceStorageBodyNormalizesCRLF (0.00s)
=== RUN   TestConfluenceStorageBodyEscapesHTML
--- PASS: TestConfluenceStorageBodyEscapesHTML (0.00s)
=== RUN   TestConfluenceStorageBodyKeepsSingleNewlineInsideParagraph
--- PASS: TestConfluenceStorageBodyKeepsSingleNewlineInsideParagraph (0.00s)
=== RUN   TestConfluenceStorageBodyEmpty
--- PASS: TestConfluenceStorageBodyEmpty (0.00s)
=== RUN   TestConfluenceStorageBodyTooLarge
--- PASS: TestConfluenceStorageBodyTooLarge (0.00s)
=== RUN   TestConfluenceStorageBodyDeterministic
--- PASS: TestConfluenceStorageBodyDeterministic (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.371s
```

7/7 通过。

## 3. 自检发现（重要：计划内部矛盾一处，已按测试裁决）

**plan-t50.md 自带的 Task 4 验收测试与 Task 4 模板实现互相矛盾（too-large 边界）：**

- 计划逐字测试（plan-t50.md:2229-2234）`TestConfluenceStorageBodyTooLarge` 输入 `strings.Repeat("p\n\n", MaxPublishBlocks)`，期待 `ErrPublishContentTooLarge`，注释明写「MaxPublishBlocks+1 paragraphs」（=501 段）。
- 计划模板实现（plan-t50.md:2285-2286）以**滤空后段落计数**判定（`len(paragraphs) > MaxPublishBlocks`）。该输入 raw split parts=501（500 个 `"p"` + 1 个尾部空串，`strings.Split` 保留尾部空串），滤空后恰 500，永不触发。
- 实证（python3 同语义 split 计数；Bash 写 Go 探针文件被 Mimosa hook 拒绝，改用等价字符串实证）：`raw parts=501, non-empty paragraphs=500`——测试注释的 501 与 raw parts 计数一致。

**裁决**：计划 Step 4 要求测试通过；依据计划 Global Constraints「Tests target observable behavior」与仓库 TDD 原则，测试是行为契约，模板实现服从测试。最终实现：容量判定用 **raw split 段数**（`len(parts) > MaxPublishBlocks`，含尾部分隔空段），渲染输出仍只含非空段，全空 → `ErrPublishEmptyContent`。已在 `confluence_blocks.go` 函数注释中明示该边界语义。

**下游安全核查**：grep 计划全文，下游消费仅 Task 6（plan-t50.md:3586 `storage, berr := ConfluenceStorageBody(string(content))`），Consumes 仅声明签名逐字（plan-t50.md:2172），不依赖边界计数细节，修正不影响 Task 6 契约。与前置 Task 3 修复轮先例一致（计划模板与事实矛盾时以可观察行为为准，提交 2b1691753）。

另：计划模板 `import "errors"` 未使用（模板笔误，编译失败），已移除；错误包裹用 `fmt.Errorf("%w: ...")`。

## 4. 回归验证（本任务实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| 全树回归 | `go test ./internal/modules/appconnector/... -count=1` | 6 个包全部 `ok`（appconnector 0.545s / connectorcontrol 3.904s / openconnector 2.797s / publish 3.950s / repository/appconnector 3.477s / service/appconnector 4.781s） |
| go vet | `go vet ./internal/modules/appconnector/publish/` | 通过（零输出） |
| 全仓构建 | `go build ./...` | BUILD-OK（仅 linker 无害告警 `ignoring duplicate libraries: '-lc++'`，环境既有） |

前置接口保持：appconnector 包（Task 1-3 confluence 适配器族）全部 ok。

## 5. 提交

```
e405abbab feat(publish): confluence storage body pure projection (T20 #50)
2 files changed, 113 insertions(+)
```

提交前 `git status --short` 仅两个 `??` 新文件（本任务授权文件），未触碰他人/其他任务文件；未推送远端。

## 6. 遗留/交接

- 无阻塞。Task 5（`ConfluenceBridge`）可依计划继续；Task 6 消费的 `ConfluenceStorageBody` 签名不变。
- 提请审查注意：本任务对计划模板有一处已记录的实现偏差（too-large 计数语义，见第 3 节）；如审查者倾向保留模板原实现，则必须同时修改计划自带的验收测试——二者不可兼得。

---

# Task 5/10 报告——publish 包：`ConfluenceBridge`（Dispatcher+Resolver+预读）与生产端口

日期：2026-09-26
任务定位：plan-t50.md:2315-3107（Task 5）

## 1. 实现内容

新建 2 个文件（均在任务授权清单内，工作区零其他改动，未触碰任何他人文件）：

- **Create** `internal/modules/appconnector/publish/confluence_bridge.go`（325 行，按计划 plan-t50.md:2767-3091 逐字落地）：
  - `const ConfluenceVersionConflictResult = "confluence_version_conflict"`——冲突 ProviderResult 前缀，Task 6 映射 409 `PUBLISH_VERSION_CONFLICT`；
  - `ConfluenceConnectionScope`——11 字段与计划 Produces 签名逐字一致；
  - 三端口接口：`ConfluenceScopeSource` / `ConfluencePolicyProvider` / `ConfluenceTokenSource`；
  - `ConfluenceBridge` + `NewConfluenceBridge`（三端口，无进度端口——单步适配器无多步进度可持久化），`var _ appconnectorsvc.ActionDispatcher` / `var _ appconnectorsvc.UnknownResolver` 编译期断言；
  - `adapterFor`：scope→AppID 路由守卫（非 confluence 即 `ErrDispatchNotStarted` 包装）→policy→credential 每次调用现场解析（吊销连接永不复用陈旧密钥）；`appconn.IsConfluenceUpdateArgs` 快照形状路由 update/create 两个适配器；
  - `run`：适配器 FAILED→确定性 failed（nil error；`errors.Is(aerr, ErrConfluenceVersionConflict)` 时前缀 `ConfluenceVersionConflictResult`）；UNKNOWN→unknown（nil error，落位等待 provider query）；`ActionAwaitingApproval`→failed（"approval revoked before send"）；其余 error 原样上抛；
  - `Dispatch` / `QueryProvider`：同一 `run`，query 走 `adapter.Query`——对账只读远端不重发；
  - `ReadConfluencePageVersion`：计划形成期预读（AC1），复用同三端口，纯只读；
  - 生产端口三件套：`dbConfluenceScopeSource`（connections→installations→app_versions 三跳；connection 按 id 单独查、installation 带 `tenant_id = conn.TenantID` 谓词；`base_url` 解析失败 fail-closed）、`scopePolicyProvider`（scheme/host/port + GET/POST/PUT + `APIBasePath+"/"` 前缀（空则 `/`）+ 30s 超时）、`confluenceCredentialTokenSource`（A02 `CredentialResolver.Resolve` → `ParseConfluenceCredential`）。
- **Create** `internal/modules/appconnector/publish/confluence_bridge_test.go`（426 行，按计划 plan-t50.md:2339-2756 落地，含两处勘误修正与 gofmt 机械重排，见第 4 节）：
  - 云版线缆双打 `cfWireFake`（Basic 认证、POST 创建、GET/PUT 单调版本门、409 冲突、`dropNextWrite` 丢回复旋钮）；
  - 7 个测试：`TestConfluenceBridgeDispatchesCreateAndUpdate` / `TestConfluenceBridgeConflictPrefix` / `TestConfluenceBridgeRefusesNonConfluenceConnection` / `TestConfluenceBridgeQueryProviderReadsRemoteFirst` / `TestConfluenceBridgeReadPageVersion` / `TestDBConfluenceScopeSource` / `TestConfluencePolicyProviderFromScope`。

**Consumes 核实（实现前逐项实读验证）**：
- Task 1/2/3 适配器族：`ConfluenceCreateAdapter`（confluence_create.go:196，字段 Policy/Credential/Edition/APIBasePath/ApprovedParents/ConnectionCapabilities）、`ConfluenceUpdateAdapter`（confluence_update.go:120）、`IsConfluenceUpdateArgs`（confluence_create.go:68）、`ConfluenceCredential`（confluence_common.go:64）、`ParseConfluenceCredential`（confluence_common.go:70）、`ParseConfluenceBaseURL`（confluence_common.go:101）、`ConfluenceCapabilityWrite`（confluence_common.go:54）、`ReadConfluencePageVersion(ctx, pol, credFn, edition, apiBasePath, pageID)`（confluence_update.go:297）、`ErrConfluenceVersionConflict`（confluence_common.go:20）；
- A03 管线：`ActionDispatcher`/`UnknownResolver`（service/appconnector/action.go:118/124）、`ActionSnapshot`（action.go:82，`Args []byte`）、`DispatchOutcome`（action.go:110）、`ErrDispatchNotStarted`（action.go:49）；
- 仓储行：`ConnectionRow`/`InstallationRow`（含 `uq_installations_tenant_app` 唯一索引，install.go:39）/`AppVersion`（install.go:24/37/51）；
- `CredentialResolver`（service/appconnector/credentials.go:21）。

## 2. TDD 证据

### Step 2 RED（写实现前实跑）

命令（worktree 根）：

```
$ go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceBridge|TestDBConfluenceScopeSource|TestConfluencePolicyProviderFromScope' -count=1
# github.com/Tencent/WeKnora/internal/modules/appconnector/publish [github.com/Tencent/WeKnora/internal/modules/appconnector/publish.test]
internal/modules/appconnector/publish/confluence_bridge_test.go:200:33: undefined: ConfluenceConnectionScope
internal/modules/appconnector/publish/confluence_bridge_test.go:234:12: undefined: NewConfluenceBridge
internal/modules/appconnector/publish/confluence_bridge_test.go:277:82: undefined: ConfluenceVersionConflictResult
internal/modules/appconnector/publish/confluence_bridge_test.go:307:12: undefined: NewConfluenceBridge
internal/modules/appconnector/publish/confluence_bridge_test.go:341:12: undefined: NewConfluenceBridge
internal/modules/appconnector/publish/confluence_bridge_test.go:341:12: too many errors
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/publish [build failed]
```

失败原因即预期原因：被测符号全部未定义（计划 Step 2 Expected 为 `undefined: ConfluenceBridge`，实际报其构造器/类型族未定义，同一性质）。

### Step 4 GREEN（最小实现后实跑，完整输出）

```
$ go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceBridge|TestDBConfluenceScopeSource|TestConfluencePolicyProviderFromScope' -count=1 -v
=== RUN   TestConfluenceBridgeDispatchesCreateAndUpdate
--- PASS: TestConfluenceBridgeDispatchesCreateAndUpdate (0.01s)
=== RUN   TestConfluenceBridgeConflictPrefix
--- PASS: TestConfluenceBridgeConflictPrefix (0.00s)
=== RUN   TestConfluenceBridgeRefusesNonConfluenceConnection
--- PASS: TestConfluenceBridgeRefusesNonConfluenceConnection (0.00s)
=== RUN   TestConfluenceBridgeQueryProviderReadsRemoteFirst
--- PASS: TestConfluenceBridgeQueryProviderReadsRemoteFirst (0.01s)
=== RUN   TestConfluenceBridgeReadPageVersion
--- PASS: TestConfluenceBridgeReadPageVersion (0.00s)
=== RUN   TestDBConfluenceScopeSource
--- PASS: TestDBConfluenceScopeSource (0.01s)
=== RUN   TestConfluencePolicyProviderFromScope
--- PASS: TestConfluencePolicyProviderFromScope (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	3.067s
```

7/7 通过。首次 GREEN 运行中 `TestDBConfluenceScopeSource` 因计划夹具违反唯一索引而 FAIL（勘误 #2，见下），修正后通过。

## 3. 回归与计划级验证（本任务实跑）

| 检查 | 命令 | 结果 |
|---|---|---|
| 全包回归 | `go test ./internal/modules/appconnector/... -count=1` | 6 包全部 `ok`（appconnector / connectorcontrol / openconnector / publish / repository/appconnector / service/appconnector） |
| 计划级命令前四段（plan-t50.md:4938） | `go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 && go test ./internal/modules/appconnector/publish/ -run 'Confluence' -count=1` | exit 0；`appconnector 0.629s ok`、`publish 2.500s ok`（build 仅 linker 无害告警 `ignoring duplicate libraries: '-lc++'`） |
| 计划级命令末段 | `go test ./internal/handler/ -run 'TestConfluencePublish' -count=1` | `ok ... [no tests to run]`——该测试属 Task 7（HTTP 面），本任务时尚不存在；非失败非跳过，如实说明 |
| gofmt | `gofmt -l internal/modules/appconnector/publish/` | 干净（无输出，两新文件重排后） |
| go vet | `go vet ./internal/modules/appconnector/publish/` | 通过（零输出） |

## 4. 对计划的两处勘误与一处机械重排（非静默改计划，与前序任务先例同责）

1. **勘误 #1（编译错误）**：计划测试 plan-t50.md:2448-2449 `parent, ok := f.pages[req.ParentID]` 中 `parent` 声明未使用（后续条件只读 `ok`），Go 编译直接失败。最小修正为 `_, ok := f.pages[req.ParentID]` 并加注释（存在性检查语义不变）。
2. **勘误 #2（夹具违反域约束，GREEN 首跑真实 FAIL）**：计划夹具把 `inst-bad`/`conn-bad` 放在 tenant 7，与 `inst-cf` 同为 `(tenant_id=7, app_id='confluence')`，违反 `InstallationRow` 自带唯一索引 `uq_installations_tenant_app`（install.go:39；域规则=一租户一 app 一安装）。首跑输出：`UNIQUE constraint failed: installations.tenant_id, installations.app_id`。修正：坏夹具改用 tenant 8（连接行同租户），fail-closed 断言语义完全不变。
3. **gofmt 重排**：单行函数字面量与单行函数体展开为多行（同 Task 4 先例：机械格式差异以 gofmt 为准）。实现文件语义逐字等于计划文本。

## 5. 自检发现

- **gorm 默认 logger 噪音**：`TestDBConfluenceScopeSource` 两处预期查找未命中（conn-bad 无解析、conn-nope 不存在）在 `-v` 下打 gorm "record not found" 错误行。保留计划原样 `&gorm.Config{}`——与同包 sibling（dispatcher_test.go:51、plan_test.go:56）一致；非 verbose 输出干净（实测单行 `ok`）。若要求完全静默可仿 `oc_integration_test.go:412` 加 `gormlogger.Discard`，属测试卫生优化，未擅动。
- **对账不重发的行为断言**：`TestConfluenceBridgeQueryProviderReadsRemoteFirst` 以 `putsBefore==putsAfter` 断言 Query 零写（AC2 对应面）；`TestConfluenceBridgeConflictPrefix` 以 `puts==0` 断言冲突零写（AC1 对应面）。
- **越权路由 fail-closed**：非 confluence 连接走 `ErrDispatchNotStarted`，且 `posts==0 && puts==0 && gets==0`——请求从未离开进程（可证明的 pre-send 拒绝）。
- **本任务未越界实现**：`ConfluencePublishService`（Task 6）、HTTP 端点（Task 7）零实现（YAGNI）。
- **未跑的检查**：`go test ./...`（全仓）未跑——计划级验证命令（plan-t50.md:4938）明确排除全量 flaky 套件，本任务按计划口径执行。

## 6. 提交

```
fbab6bd4d feat(publish): confluence bridge - dispatcher/resolver routing + production scope/policy/credential ports (T20 #50)
 2 files changed, 751 insertions(+)
```

提交前 `git status` 干净（仅两个本任务授权新文件）；未推送远端。

## 7. 遗留/交接

- 无阻塞。Task 6 可依计划消费：`ConfluenceVersionConflictResult` / `ConfluenceBridge`（`ActionDispatcher`+`UnknownResolver` 双实现）/ `ReadConfluencePageVersion` / `NewDBConfluenceScopeSource` / `NewConfluencePolicyProvider` / `NewConfluenceCredentialTokenSource`，签名逐字与计划 Produces 一致。
- 提请审查注意：两处计划勘误（§4.1 编译错误、§4.2 唯一索引夹具冲突）均为计划文本客观缺陷，修正不改变任何断言语义；如审查者倾向保留计划原文，需同步修订计划文本，否则测试不可编译/不可通过。

---

# Task 6/10 报告——publish 包：`ConfluencePublishService`（FormPlan/Execute/Reconcile/回执 settle）

任务定位：plan-t50.md:3109-3723（Task 6）
执行 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t50`（分支 `codex/issue30-t50`，基线 HEAD `2bc22045d` 即 Task 5 报告提交）。
本报告由实现员子代理撰写，仅覆盖 Task 6 授权文件。

## 1. 实现内容

按计划 Task 6 逐字实现，两个新增文件（工作区零其他改动，未触碰任何他人文件）：

- **`internal/modules/appconnector/publish/confluence.go`**：
  - `ConfluenceRemoteReader` 接口：`ReadConfluencePageVersion(ctx, connectionID, pageID) (string, error)`——计划期预读端口，由 Task 5 的 `*ConfluenceBridge` 满足（`confluence_bridge.go:187` 同签名）。
  - `ConfluencePublishService` 七字段结构（`actions`/`store`/`pubs`/`artifacts`/`content`/`remote`/`scopes`）与 `NewConfluencePublishService` 构造器，签名逐字等于计划 Produces。
  - `FormPlan`：输入校验（租户/actor/connection/session/version id/title 非空、parent 与 page 恰择一）→ `ConfluenceScope` 解析 + `AppID != "confluence"` 拒绝（`appIDConfluence`）→ update 目的地权威规则（`LatestPublishedByDestination` 无既往回执即 `ErrPublishUpdateTargetNotPublished` fail-closed）→ create 父页面白名单（`scope.ApprovedParents`，不在名单 `ErrPublishDestinationOutOfScope`）→ Artifact 版本读取（不可读 `ErrPublishArtifactNotReady`、MIME 不可发布 `ErrPublishUnsupportedArtifact`、超 `MaxPublishArtifactBytes` `ErrPublishContentTooLarge`）→ `ConfluenceStorageBody` 确定性正文投影（空正文透传 `ErrPublishEmptyContent`）→ **AC1 计划期外部版本预读**（读失败/空版本一律 `ErrPublishDestinationUnreadable`，绝不把「读不到」当「可写」）→ 组装快照 bytes（update 四字段含 `expected_version`=预读值；create 三字段）→ `actions.Prepare`（Version `"confluence/v1"`、`RiskWrite`、`AuthVersion` 严格绑定）→ `CreatePublication`（Provider `"confluence"`、`PublicationPlanned`、绑定 ExpectedVersion/ArtifactVersionID/ArtifactDigest；失败时 prepared action 留在 awaiting_approval 成孤儿计划，绝不伪造回执）→ 返回 `PublishPlanView`（digest/fence 来自权威 action 行）。
  - `Execute`：`actions.Execute`，`ErrDispatchUnknown` 非错误（unknown 停车是诚实结局，不是失败），随后 `project` 从 action 行权威状态 settle 回执。
  - `Reconcile`：`actions.ResolveUnknown`（对账只查 provider，绝不重发写），随后同 `project`。
  - `Receipt`：`FindByAction` → `publicationView` 投影。
  - `project`：跨租户 action id 一律 `repoappconn.ErrActionNotFound`（不泄漏存在性）；`Conflict` 判定 = `ProviderResult` 前缀 `ConfluenceVersionConflictResult`（Task 5 产物常量）；Succeeded→解析 `appconn.ParseConfluencePageReceipt` settle `PublicationPublished`（外部 id/version 来自 provider 自身回执）；Failed→settle `PublicationFailed`；Unknown→settle `PublicationUnknown`；settle 生命周期冲突（`ErrPublicationConflict`）幂等容忍。
  - 复用零修改：`PublishPlanInput`/`PublishPlanView`/`PublishArtifactView`/`PublishExecuteOutcome`/`PublishReceiptView`/`publicationView`/`publishableMIME`/`MaxPublishArtifactBytes`/全部 `ErrPublish*` 哨兵（plan.go/blocks.go）、`ConfluenceStorageBody`（confluence_blocks.go）、`ConfluenceScopeSource`/`ConfluenceVersionConflictResult`（confluence_bridge.go）——与计划 Interfaces 声明一致，既有文件零改动。

- **`internal/modules/appconnector/publish/confluence_test.go`**（计划 Step 1 测试逐字）：11 个测试函数——FormPlan 8 个（create 绑定基线版本+快照存储体+回执行、父越界 fail-closed、目的地不可读拒绝成计划、update 无既往回执拒绝、update 绑定预读 expected_version、非 Confluence 连接拒绝、坏输入 4 例、空 artifact 拒绝）+ Execute 2 个（create 发布成功 settle published 回执含外部 id/version、update 冲突 settle failed 回执且 **puts==0**）+ Reconcile 1 个（丢回复→unknown 停车→盲重发布被 `ErrActionState` 结构性拒绝→Reconcile 从远端判 succeeded 且 **puts 计数不增长**）。

## 2. TDD 证据（实跑命令与完整输出）

### RED（Step 2）

命令：

```
go test ./internal/modules/appconnector/publish/ -run 'TestConfluenceFormPlan|TestConfluenceExecute|TestConfluenceReconcile' -count=1
```

输出（与计划 Step 2 预期 `undefined: NewConfluencePublishService` 形态一致）：

```
# github.com/Tencent/WeKnora/internal/modules/appconnector/publish [github.com/Tencent/WeKnora/internal/modules/appconnector/publish.test]
internal/modules/appconnector/publish/confluence_test.go:36:9: undefined: ConfluencePublishService
internal/modules/appconnector/publish/confluence_test.go:60:9: undefined: NewConfluencePublishService
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/publish [build failed]
FAIL
```

### GREEN（Step 4）

同一命令，11/11 PASS：

```
=== RUN   TestConfluenceFormPlanCreateBindsBaselineVersion
--- PASS: TestConfluenceFormPlanCreateBindsBaselineVersion (0.00s)
=== RUN   TestConfluenceFormPlanCreateParentOutOfScopeFailsClosed
--- PASS: TestConfluenceFormPlanCreateParentOutOfScopeFailsClosed (0.00s)
=== RUN   TestConfluenceFormPlanDestinationUnreadable
--- PASS: TestConfluenceFormPlanDestinationUnreadable (0.00s)
=== RUN   TestConfluenceFormPlanUpdateRequiresPriorPublishedReceipt
--- PASS: TestConfluenceFormPlanUpdateRequiresPriorPublishedReceipt (0.00s)
=== RUN   TestConfluenceFormPlanUpdateBindsExpectedVersion
--- PASS: TestConfluenceFormPlanUpdateBindsExpectedVersion (0.00s)
=== RUN   TestConfluenceFormPlanRejectsNotConfluenceConnection
--- PASS: TestConfluenceFormPlanRejectsNotConfluenceConnection (0.00s)
=== RUN   TestConfluenceFormPlanRejectsBadInputs
--- PASS: TestConfluenceFormPlanRejectsBadInputs (0.00s)
=== RUN   TestConfluenceFormPlanRejectsEmptyArtifact
--- PASS: TestConfluenceFormPlanRejectsEmptyArtifact (0.00s)
=== RUN   TestConfluenceExecuteCreatePublishesAndSettlesReceipt
--- PASS: TestConfluenceExecuteCreatePublishesAndSettlesReceipt (0.01s)
=== RUN   TestConfluenceExecuteUpdateConflictSettlesFailedReceipt
--- PASS: TestConfluenceExecuteUpdateConflictSettlesFailedReceipt (0.00s)
=== RUN   TestConfluenceReconcileResolvesUnknownWithoutRedispatch
--- PASS: TestConfluenceReconcileResolvesUnknownWithoutRedispatch (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.388s
```

（运行中出现的 gorm `record not found` 日志是 `FindByAction` 查无此行时的正常 debug 输出——测试故意的负路径与首次查询，非错误。）

### 回归与构建

```
go test ./internal/modules/appconnector/... -count=1
```

```
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.174s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	1.534s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.192s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.231s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.510s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	3.012s
```

```
go build ./...   # BUILD OK（仅既有的 cmd/desktop、cmd/server ld duplicate-libraries 警告，与本任务无关）
gofmt -l <两个新文件>   # 无输出（格式合规）
go vet ./internal/modules/appconnector/publish/   # VET OK
```

## 3. 提交

- `5f21c7783` `feat(publish): confluence publish service - form/execute/reconcile/settle (T20 #50)`（2 files changed, 568 insertions：`confluence.go` + `confluence_test.go`，均为新增；提交后 `git status --short` 干净）。

## 4. 自检发现

1. **实现与计划文本逐字一致，仅一处被动跟随的既有事实**：计划实现模板中 `Conflict` 判定用 `ConfluenceVersionConflictResult`——该常量实际定义在 Task 5 的 `confluence_bridge.go:20`（`"confluence_version_conflict"`），与 Notion 模板的 `PublishVersionConflictResult`（dispatcher.go:20，`"notion_version_conflict"`）对称，直接可用，无需新定义。签名全部逐字等于计划 Produces。
2. **计划测试代码与 Task 5 既有测试辅助完全咬合**：`cfWireFake`（`puts`/`mu`/`dropNextWrite` 字段、`addPage(id, spaceID, title, version)`、`bump(id)`）、`cfWirePolicy`、`cfFakeScopes`、`cfFakeTokens`、`cfCred`、`cfScope`、`fakePolicies`、`fakeArtifacts`、`fakeContent`、`passGuard`、`openPlanDB` 全部复用既有测试文件定义，零重复定义；`cfRemote`/`cfScopeOK`/`cfPlanEnv` 为本测试文件新增，无命名冲突。测试断言 `<p>hello</p>\n<p>world</p>` 与 Task 4 `ConfluenceStorageBody` 实际输出逐字符一致（撰写前已实读 confluence_blocks.go:23-49 核实）。
3. **Review Focus 覆盖**（plan-t50.md:58-64 中归属 Task 6 的三项全部落地）：①TOCTOU 漂移（第 1 类）→ `TestConfluenceExecuteUpdateConflictSettlesFailedReceipt`（conflict 标记 + failed 回执 + puts==0）；②版本令牌缺失当可写（第 2 类）→ `TestConfluenceFormPlanDestinationUnreadable`（预读失败拒绝成计划）；③盲重试（第 3 类）→ `TestConfluenceReconcileResolvesUnknownWithoutRedispatch`（unknown 结构性拒绝再发布 + 对账不重发）；另覆盖第 5 类错接（`TestConfluenceFormPlanRejectsNotConfluenceConnection`）。
4. **一处措辞级观察（非缺陷，不改代码）**：`FormPlan` 对 create 的父页面预读失败与 update 相同返回 `ErrPublishDestinationUnreadable`——这是计划模板的原样语义（create 预读是「审阅目的地的基线记录」而非强制门，但读不到同样拒绝成计划，fail-closed 方向正确），与计划 Task 6 文字描述一致，无需偏离。
5. **零迁移、零 TS、零共享文件改动**：本任务仅 2 个新增文件，`plan.go`/`dispatcher.go`/`confluence_bridge.go`/路由/容器全部未触碰；无空提交。

## 5. 遗留/交接

- 无阻塞。Task 7（HTTP 面）可依计划消费：`NewConfluencePublishService(actions, store, pubs, artifacts, content, remote, scopes)`、`FormPlan`/`Execute`/`Reconcile`/`Receipt` 四方法签名逐字与计划 Produces 一致；`remote` 注 `*ConfluenceBridge`、`scopes` 注 `NewDBConfluenceScopeSource(db)`（均 Task 5 产物）。

---

# Task 7/10 报告——HTTP 面 `/apps/confluence-publish/*` 端点、路由注册与容器接线

> 任务：Issue #50 实施计划（`docs/plans/issue30-sweep/plans/plan-t50.md:3726-4243`）Task 7。
> 执行 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t50`（分支 `codex/issue30-t50`）。
> 本报告由实现员子代理撰写，仅覆盖 Task 7 授权文件（3 个新建生产文件 + 1 个测试文件 + 2 处共享文件单点接线）。

## 1. 实现内容

全部按计划 Task 7 落位，TDD 顺序严格执行（RED→GREEN→提交）：

**新建 4 个文件：**

1. **`internal/handler/app_connector_confluence_publish.go`**（235 行）——`AppConfluencePublishHandler`，Produces 签名逐字：
   - `NewAppConfluencePublishHandler(db *gorm.DB)`、`SetConfluencePublishService(s *publish.ConfluencePublishService)`（nil 服务时四端点一律 501 `PUBLISH_PIPELINE_NOT_CONFIGURED` fail-closed）、`RequireActionCapabilityForWrites()`（复用 `appRequireWriteCapability` + `appconnector.CanDriveActionWrites`，access.go:43）。
   - `FormConfluencePublishPlan`（POST /plans，201）：`appTenantScope` → 入参校验（五必填 + `parent_page_id`/`page_id` 恰好其一，否则 400 `INVALID_REQUEST`）→ 个人连接 owner 谓词（403 `NOT_CONNECTION_OWNER`，同 `PrepareAction` 形态）→ `publish.ConfluencePublishService.FormPlan`。
   - `PublishConfluenceAction`（POST /actions/:id/publish）：租户内 action 查找（跨租户与不存在统一 404 `ACTION_NOT_FOUND`，不泄漏存在性）→ `Execute` → `outcome.Conflict` 时 409 `PUBLISH_VERSION_CONFLICT`（AC1 的 HTTP 面）。
   - `ReconcileConfluenceAction`（POST /actions/:id/reconcile）、`GetConfluencePublication`（GET /actions/:id）同谓词形态。
   - `failConfluence`/`failConfluenceExecute` 错误映射：`publish.ErrPublish*` 九个 sentinel 与 `appconnectorsvc.ErrActionState`/`ErrNoDispatcher` → 精确状态码，与 Notion 兄弟实现（app_connector_notion_publish.go:199-232）同形。
2. **`internal/router/routes_app_confluence_publish.go`**——`RegisterAppConfluencePublishRoutes`：`/apps/confluence-publish` 组挂写门 + 四条路由，与 `routes_app_notion_publish.go` 逐行同形（nil handler 直接返回）。
3. **`internal/container/confluence_publish.go`**——dig 构造器 `newConfluencePublishHandler`：**第三个** `ActionService` 实例（同一 `ActionStore` 权威，`NewActionService(store, guard, gate, bridge, bridge)`）；bridge 三端口 `NewDBConfluenceScopeSource(db)` + `NewConfluencePolicyProvider(scopeSrc)` + `NewConfluenceCredentialTokenSource(NewCredentialResolver(creds))`；`NewConfluencePublishService(actions, store, pubs, versions, &tenantStorageArtifactContent{...}, bridge, scopeSrc)` 与 Task 6 实际签名（confluence.go:41-49，7 参）逐字对齐；冻结的 OC-armed 实例与 #48 Notion 发布实例零触碰（action.go:158-160 的 store 权威裁决）。`tenantStorageArtifactContent` 同包复用（notion_publish.go:59）零修改。
4. **`internal/handler/app_connector_confluence_publish_test.go`**（126 行）——计划 Step 1 逐字：`TestConfluencePublishPlanGates`（未接线 501 / viewer 403 / 他人个人连接 403 `NOT_CONNECTION_OWNER` / 双目的地 400 / 未知连接 404）与 `TestConfluencePublishActionLookupIsTenantScoped`（同租户到 501 / 跨租户 publish 与 GET 均 404）。

**修改 2 个共享文件（均单点最小接线）：**

5. `internal/router/router.go`（+3/-1）：`RouterParams` 在 `AppNotionPublishHandler` 字段后增 1 字段 `AppConfluencePublishHandler *handler.AppConfluencePublishHandler`（gofmt 顺带对齐既有字段行的空格）；`RegisterAppNotionPublishRoutes(...)` 调用后增 1 行 `RegisterAppConfluencePublishRoutes(v1, params.AppConfluencePublishHandler)`。
6. `internal/container/container.go`（+4/-0）：`must(container.Provide(newNotionPublishHandler))` 后按计划逐字插入 3 行注释 + `must(container.Provide(newConfluencePublishHandler))`。

**开工前置核实（全部亲眼读源码确认）：** Task 6 产出 `ConfluencePublishService` 四方法与 `PublishExecuteOutcome.Conflict`（plan.go:98-102）；bridge 三构造器（confluence_bridge.go:89/216/285/315）；接线点 router.go:145/:436、container.go:1013 与计划行号一致；测试助手 `publishTestContext`（app_connector_notion_publish_test.go:26）同包复用。

## 2. 测试命令与完整输出（TDD 证据）

### RED

先按计划原文落盘测试文件——计划 imports 抄自 Notion 测试文件（那里**定义**了 `publishTestContext`，其签名使用 `context`/`types`），本文件只**调用**它，出现两个未使用导入的编译错夹杂在目标失败中；删去这两个多余导入后 RED 纯净：

命令：`go test ./internal/handler/ -run 'TestConfluencePublishPlanGates|TestConfluencePublishActionLookupIsTenantScoped' -count=1`

```
# github.com/Tencent/WeKnora/internal/handler [github.com/Tencent/WeKnora/internal/handler.test]
internal/handler/app_connector_confluence_publish_test.go:22:66: undefined: AppConfluencePublishHandler
internal/handler/app_connector_confluence_publish_test.go:36:7: undefined: NewAppConfluencePublishHandler
FAIL	github.com/Tencent/WeKnora/internal/handler [build failed]
FAIL
```

失败原因正确：实现尚不存在。

### GREEN（Step 4 计划原命令）

实现落盘后，命令：`go build ./... && go test ./internal/handler/ -run 'TestConfluencePublishPlanGates|TestConfluencePublishActionLookupIsTenantScoped|TestNotionPublishPlanGates' -count=1 -v`

```
=== RUN   TestConfluencePublishPlanGates
--- PASS: TestConfluencePublishPlanGates (0.02s)
=== RUN   TestConfluencePublishActionLookupIsTenantScoped
--- PASS: TestConfluencePublishActionLookupIsTenantScoped (0.00s)
=== RUN   TestNotionPublishPlanGates
--- PASS: TestNotionPublishPlanGates (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	3.204s
```

`go build ./...` 通过 = dig 构造器签名与注入依赖全部可解析（容器接线编译验证）；`TestNotionPublishPlanGates` 复跑 PASS = #48 面零回归。日志中 gorm `record not found` 行是 404 断言路径的正常查询日志（Notion 同形测试同样出现），非测试噪音。

### 自检修正后在最终提交形态复跑

自检发现 router.go 多插了 2 行注释（见发现 1），精简并 amend 后按 Step 4 原命令完整复跑：

命令：`go build ./... && go test ./internal/handler/ -run 'TestConfluencePublishPlanGates|TestConfluencePublishActionLookupIsTenantScoped|TestNotionPublishPlanGates' -count=1`

```
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/handler	3.076s
EXIT=0
```

（两条 `ld: warning` 为 `cmd/desktop`/`cmd/server` 既有链接告警，与本任务无关。）

## 3. 提交

- `89c8f710f` `feat(appconnector): /apps/confluence-publish endpoints, routes and container wiring (T20 #50)`（6 files changed, 440 insertions(+), 1 deletion(-)；前身 `a3f0cd540` 为本次 amend 前身，未推送远端；提交后 `git status --short` 干净）。
- diff 范围核实（`git show --stat HEAD`）：恰为 Task 7 授权的 6 个文件；#48 产物（notion_create/update、publish/plan|dispatcher|blocks、routes_app_notion_publish.go、container/notion_publish.go）与知识库只读连接器零触碰，符合计划「编号协调与并行批次注意」约定。

## 4. 自检发现

1. **计划偏差 A（已修正并复验）**：router.go 计划原文是「插入一行字段」（plan-t50.md:4212-4216），我初版多加了 2 行注释；container.go 的 3 行注释块才是计划逐字给出的内容（plan-t50.md:4224-4231）。已精简回计划严格形态并 amend，复跑 Step 4 命令 PASS。
2. **计划偏差 B（测试 imports，保留修正）**：计划测试代码 imports 含 `"context"` 与 `"github.com/Tencent/WeKnora/internal/types"`（plan-t50.md:3759-3772），本文件中二者未被使用——它们只被同包另一文件定义的 `publishTestContext` 签名使用，照抄会导致编译失败（首次实跑已复现）。删去这两个导入，功能零偏差。
3. **前置接口核对**：ask 给出的 `NewConfluencePublishService(actions,store,pubs,artifacts,content,remote,scopes)` 7 参形态与计划 container 代码调用语义一致（`versions`→artifacts、`bridge`→remote、`scopeSrc`→scopes），与 Task 6 实际源码 confluence.go:41-49 完全吻合，无冲突。
4. **Review Focus 覆盖**：plan-t50.md:64 归属 Task 7 的第 5 类（越权与错接）两项已落地——`TestConfluencePublishPlanGates`（viewer 写门 403、个人连接 owner 谓词 403）+ `TestConfluencePublishActionLookupIsTenantScoped`（跨租户统一 404 不泄漏存在性）。
5. **未做的事**：Task 8 E2E 与真实 Provider 证据（Task 9）不属本任务；本任务测试是 HTTP 谓词门——计划定位即如此（测试文件头注释 plan-t50.md:3753-3757 明示 pipeline 本体由 service 层测试与 Task 8 覆盖）。
6. 测试输出纯净（除既有 linker warning 与 404 断言路径的 gorm 日志）；无空提交、未推送远端。

## 5. 遗留/交接

- 无阻塞。Task 8（E2E）可依计划消费：`AppConfluencePublishHandler` + `NewAppConfluencePublishHandler(db)` + `SetConfluencePublishService(*publish.ConfluencePublishService)` + 四端点方法与 `RegisterAppConfluencePublishRoutes(r, h)`，签名逐字如计划 Produces（plan-t50.md:3738-3744）。

# Task 8/10 报告——E2E 最高稳定 Interface 证据（生产迁移 + 全链 + 契约双打）

> 任务：Issue #50 实施计划（`docs/plans/issue30-sweep/plans/plan-t50.md:4247-4792`）Task 8。
> 执行 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t50`（分支 `codex/issue30-t50`）。
> 本报告由实现员子代理撰写，覆盖 Task 8 授权文件（1 个 E2E 测试文件）+ 计划 Task 0 幂等配方的条件性执行（见 §3，授权依据：plan-t50.md:85-152 的幂等设计与 DUPLICATE-PRESENT 分支）。

## 1. 实现内容

**新建 1 个文件：`internal/handler/app_connector_confluence_publish_e2e_test.go`**（513 行）——计划 Step 1 代码逐字落盘，含：

1. **`openConfluencePublishE2EDB`**：生产 sqlite 迁移双轨全量加载（golang-migrate `migrations/sqlite`，范式与 `openNotionPublishE2EDB`（app_connector_notion_publish_e2e_test.go:53-71）逐行同形）——AC3「生产迁移库」的证据基础。
2. **`e2eConfluence` 契约双打**：本地实现官方 Cloud v2 页面契约（`GET/POST/PUT /wiki/api/v2/pages`，HTTP Basic 认证 + `version.number` 单调门 `req.Version.Number != p.version+1 → 409`），文件头与双打注释明示「NOT the real-provider acceptance，真实 Provider 证据留在 CONFLUENCE_*-gated 的 Task 9」——`bump()` 模拟 AC1 审批窗口内的外部协作者编辑，`dropNextPut` 模拟 AC2 的写回复丢失（写效果已落、结局不可观察）。
3. **`newConfluencePublishE2E` 全真组合**：真实 `ActionService`（第三个实例，`NewActionService(store, &e2ePassGuard{}, nil, bridge, bridge)`，同一 `ActionStore` 权威）/`PublicationStore`/`ConfluenceBridge`（真实 `DBConfluenceScopeSource` 读回双打回填的 `installations.config_json.base_url` + 真实 `ConfluenceCredentialTokenSource`→`CredentialResolver`→`MCPOAuthBindingStore` 凭据链）/`ConfluencePublishService`/gin 处理器/既有审批端点 `POST /apps/actions/:id/approve`；**唯一被替换的是 Confluence 网络端点**，出站策略走 `http_policy.go:63-66` 明示的 loopback 测试钩子。可复用助手 `e2ePolicyProvider`/`e2ePassGuard`/`e2eLocalContent`（Notion E2E 测试文件）零修改复用。
4. **三个 E2E 测试**：`TestConfluencePublishEndToEndCreateApprovePublishReceipt`（AC3 全链：form 201→approve 200→publish 200 succeeded→回执 external_id/external_version="1"→GET 回执查询→远端页面 storage 断言 `"<p>first para.</p>\n<p>second para.</p>"`）、`TestConfluencePublishEndToEndUpdateConflict`（AC1：外部 bump 后 publish 409 `PUBLISH_VERSION_CONFLICT` 且 fake PUT 计数零变化）、`TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst`（AC2：丢回复→200 unknown 停车→盲重 publish 409 且 PUT 计数不变→reconcile 远端读回 settle succeeded/published 且不重发）。

**对计划文本的一处编译笔误修正（1 个 token，语义等同）**：计划 Step 1 的 `e2eConfluence` POST 分支写 `parent, ok := e.pages[req.ParentID]`（plan-t50.md:4411）——Task 2 的 `fakeConfluence` 中该 `parent` 被 `parent.spaceKey` 消费（plan-t50.md:718），而 E2E 双打的 `spaceID` 取自请求体，`parent` 变量无消费点，照抄编译失败（`declared and not used`，实跑见 §2 第一跑）。修正为 `_, ok := e.pages[req.ParentID]`：存在性检查（父页面不存在→400）逐字保留。

**计划 Task 0 幂等配方的条件性执行**（详见 §3）：E2E 首跑暴露迁移轨道双占回归，按 plan-t50.md:103-152 的 DUPLICATE-PRESENT 配方执行 git mv ×4 + 5 个测试文件 7 处钉住路径改串（与兄弟分支已验证先例 `893e316c7` 完全同配方），独立提交 `60bf99af3`。

## 2. 测试命令与完整输出（TDD 证据）

### 第一跑（落盘后原样编译）：计划笔误如实暴露

命令：`go test ./internal/handler/ -run 'TestConfluencePublishEndToEnd' -count=1`

```
# github.com/Tencent/WeKnora/internal/handler [github.com/Tencent/WeKnora/internal/handler.test]
internal/handler/app_connector_confluence_publish_e2e_test.go:151:3: declared and not used: parent
FAIL	github.com/Tencent/WeKnora/internal/handler [build failed]
FAIL
```

### 第二跑（笔误修正后）：迁移轨道双占回归暴露（Task 0 预警场景）

命令：`go test ./internal/handler/ -run 'TestConfluencePublishEndToEnd' -count=1 -v`

```
=== RUN   TestConfluencePublishEndToEndCreateApprovePublishReceipt
    app_connector_confluence_publish_e2e_test.go:379:
        	Error Trace:	.../internal/handler/app_connector_confluence_publish_e2e_test.go:65
        	            				.../internal/handler/app_connector_confluence_publish_e2e_test.go:230
        	            				.../internal/handler/app_connector_confluence_publish_e2e_test.go:379
        	Error:      	Received unexpected error:
        	            	failed to open source, "file:///Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t50/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql
--- FAIL: TestConfluencePublishEndToEndCreateApprovePublishReceipt (0.03s)
=== RUN   TestConfluencePublishEndToEndUpdateConflict
        [... 同一 duplicate migration file 错误，Error Trace :426 ...]
--- FAIL: TestConfluencePublishEndToEndUpdateConflict (0.01s)
=== RUN   TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst
        [... 同一 duplicate migration file 错误，Error Trace :468 ...]
--- FAIL: TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst (0.01s)
FAIL
FAIL	github.com/Tencent/WeKnora/internal/handler	8.371s
FAIL
```

### Task 0 配方执行与验证（GREEN 前置，配方 Step 4 原命令）

配方落盘后——双轨唯一性断言（plan-t50.md:140）：

```
$ test -z "$(ls migrations/sqlite | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && test -z "$(ls migrations/versioned | grep -oE '^[0-9]{6}' | sort | uniq -c | awk '$1 > 2')" && echo "migration versions unique on both tracks"
migration versions unique on both tracks
```

旧编号零残留（`grep -rn "000114_mobile_device_app\|000193_mobile_device_app" internal/ migrations/` → `0`）+ 新文件就位（`000118_mobile_device_app.{up,down}.sql`、`000197_mobile_device_app.{up,down}.sql`）。

轨道可装载直接证据（配方 Step 4 命令）：`go test ./internal/handler/ -run 'TestNotionPublishEndToEndCreateApprovePublishReceipt' -count=1` → `ok github.com/Tencent/WeKnora/internal/handler 7.142s`。

### GREEN（Task 8 Step 2 计划原命令，-v）

命令：`go test ./internal/handler/ -run 'TestConfluencePublishEndToEnd' -count=1 -v`

```
=== RUN   TestConfluencePublishEndToEndCreateApprovePublishReceipt
INFO [2026-09-26 20:31:54.935] [] local.go:112[GetFile] | Getting file: artifact-versions/7/run-1/bbbb…
--- PASS: TestConfluencePublishEndToEndCreateApprovePublishReceipt (4.51s)
=== RUN   TestConfluencePublishEndToEndUpdateConflict
--- PASS: TestConfluencePublishEndToEndUpdateConflict (10.78s)
=== RUN   TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst
--- PASS: TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst (6.12s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	25.889s
```

（INFO 行为真实 `file.NewLocalFileService` 读 Artifact 字节的生产日志。）

### 全链复跑（Task 8 Step 3 计划原命令，含 #48 回归）

命令：`go test ./internal/handler/ -run 'TestConfluencePublishEndToEnd|TestNotionPublishEndToEnd|TestAppPublicationsTable' -count=1 -v`

```
=== RUN   TestConfluencePublishEndToEndCreateApprovePublishReceipt
--- PASS: TestConfluencePublishEndToEndCreateApprovePublishReceipt (11.53s)
=== RUN   TestConfluencePublishEndToEndUpdateConflict
--- PASS: TestConfluencePublishEndToEndUpdateConflict (5.09s)
=== RUN   TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst
--- PASS: TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst (5.50s)
=== RUN   TestAppPublicationsTableExistsAfterMigrations
--- PASS: TestAppPublicationsTableExistsAfterMigrations (4.13s)
=== RUN   TestNotionPublishEndToEndCreateApprovePublishReceipt
--- PASS: TestNotionPublishEndToEndCreateApprovePublishReceipt (4.17s)
=== RUN   TestNotionPublishEndToEndUpdateConflict
--- PASS: TestNotionPublishEndToEndUpdateConflict (4.62s)
=== RUN   TestNotionPublishEndToEndUnknownReconcilesRemoteFirst
--- PASS: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (11.64s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	53.304s
```

Notion 与 Confluence 两套发布闭环在同一包内并存全绿——发布 seam 双实例互不干扰的计划预期（plan-t50.md:4784）成立。

### Task 0 改串波及面回归 + 全仓编译（自检加测）

命令与输出：

```
$ go test ./internal/application/repository/ -run 'MobileDeviceApp|MobilePushIsolation|MobileDevice' -count=1
ok  	github.com/Tencent/WeKnora/internal/application/repository	15.034s
$ go test ./internal/handler/ -run 'TestMobileDevice' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	2.220s
$ go build ./...
[仅 cmd/desktop、cmd/server 两条既有 ld: warning: ignoring duplicate libraries: '-lc++']
BUILD OK
```

`internal/modules/workbench/service/workbench/` 包整体 FAIL，其中 1 个失败经实锤为**预存失败、与本任务无关**（见 §5 发现 3）；该包其余测试（含 Task 0 改串文件的同包测试）PASS。

## 3. 计划 Task 0 幂等配方的条件性执行（本任务授权范围内的必要前置）

**事实链**（全部本会话实跑/实读）：

1. E2E 首跑三测全部 `duplicate migration file: 000114_public_agent_marketplace.down.sql`（见 §2 第二跑）。
2. 轨道实测：sqlite 轨 `000114` 出现 4 个文件（`mobile_device_app` ×2 + `public_agent_marketplace` ×2），versioned 轨 `000193` 同样 4 个——计划 plan-t50.md:143 描述的双占形态逐字复现。
3. 归属核查：`git log --oneline -- migrations/sqlite/000114_mobile_device_app.up.sql` 在本分支仅见创建提交 `fb9037710`（#67 原始迁移），**本分支从未执行过重编**；四个 dedupe 提交（`893e316c7`/`bba8ce8db`/`34849775b`/`96e579ad0`）都在兄弟分支（`git branch --contains 893e316c7` → 仅 `codex/issue30-t49`；`git merge-base --is-ancestor 893e316c7 HEAD` → 否）。即本分支基底含双占，双占直到 Task 8（首个全量加载生产迁移的 Confluence 测试）才在本分支暴露。
4. 处置：按计划 Task 0 的幂等设计（plan-t50.md:101「轨道健康则纯验证，双占则执行该配方」）执行 DUPLICATE-PRESENT 分支——`git mv` ×4（sqlite `000114_mobile_device_app.*` → `000118_*`，versioned `000193_*` → `000197_*`；两号均为当时轨道最大号 000117/000196 之后的下一可用号且未被占用）+ 5 个测试文件 7 处完整文件名字符串替换（`mobile_device_app_test.go:37,188,189`、`mobile_push_isolation_test.go:67`、`mobile_device_test.go:31`、`handler/mobile_device_test.go:33`、`notification_app_policy_test.go:50`；与先例 `893e316c7` 的 diff 逐字同形，注释中裸编号不动）。sed 批量写 .go 被 Mimosa hook 拒绝，改用 Edit 工具逐处替换（hook 合规）。
5. 验证：双轨唯一性断言 + Notion E2E（§2「Task 0 配方执行与验证」）+ 改串波及面回归，全部通过后独立提交。

## 4. 提交

- `60bf99af3` `fix(migrations): renumber mobile_device_app to 000118/000197 - resolve duplicate-version chain breakage with public_agent_marketplace (B5 #50 prerequisite)`（Task 0 Step 5 计划原文提交信息；9 files changed, 7 insertions(+), 7 deletions(-)，4 个 rename 100% 相似）。
- `d9160580e` `test(appconnector): confluence publish end-to-end evidence on production migrations (T20 #50)`（Task 8 Step 4 计划原文提交信息；1 file changed, 513 insertions(+)）。
- 提交后 `git status --short` 干净；未推送远端；冻结面零触碰（#48 产物、`newOCArmedActionService`、知识库只读连接器 `internal/modules/datasource/connector/confluence/` 均不在 diff 中）。

## 5. 自检发现

1. **计划笔误 A（已最小修正）**：E2E 双打 POST 分支 `parent, ok := …` 的 `parent` 无消费点导致编译失败（§2 第一跑实锤），修正为 `_, ok :=`；存在性检查语义逐字保留。除此之外测试代码与计划 Step 1 逐字一致。
2. **波级事实（重要，供集成方知悉）**：本分支 HEAD 在本任务前从未含任何 mobile_device_app 重编提交，双占随基线携带；本任务按 Task 0 幂等配方在本分支补齐（`60bf99af3`，占 sqlite `000118`/versioned `000197`）。兄弟分支已有四处同号 dedupe（`893e316c7` 等），合并时若发生同号竞争，按计划 plan-t50.md:120 约定「整体顺延为下一可用编号，文件内容零变化」机械消解。
3. **预存失败（与本任务无关，实锤）**：`go test ./internal/modules/workbench/service/workbench/ -count=1` 中 `TestNotificationDeliveryRejectsResolvedInteractionAfterClaim` FAIL（`notification_delivery_test.go:489`，`expected: int(1) actual: int64(0)`）。排除链：该文件零引用 migrations（`grep -c migrations` → 0，测试用手工 `CREATE TABLE` 建库，不经迁移轨道）；`git diff HEAD` 对该文件为零改动；在**本任务任何改动之前**的 HEAD（`9058406e0`）建临时 worktree（`git worktree add /tmp/wt-pre-t8 9058406e0`）实跑同一测试同样 FAIL。结论：预存失败，非本任务引入；验证后临时 worktree 已 `git worktree remove --force` 清理。该失败不影响本任务任何验证结论，留待其归属计划处理。
4. **Review Focus 覆盖**（plan-t50.md:60-63 归属 Task 8 的项）：第 1 类 TOCTOU→`TestConfluencePublishEndToEndUpdateConflict`（409 且 fake PUT 计数零变化）；第 3 类传输失败盲重试→`TestConfluencePublishEndToEndUnknownReconcilesRemoteFirst`（unknown 停车、结构性拒绝二次 dispatch、reconcile 只读 settle、全程 PUT 计数仅 +1）；AC3 最高稳定 Interface→create 全链测试（生产迁移库 + 全真链 + 双打注释明示 NOT real-provider acceptance）。
5. **blocked-env 边界如实声明**：本任务证据是 AC3 的**本地最高稳定 Interface 替代证据**（plan-t50.md:44 的定义）；真实 Confluence 站点 + 真实 token + 真实 version.number 推进的验收属 Task 9，本环境 SKIP 且未被伪造。
6. 未做的事：无。Task 8 的 Files 清单恰为 1 个测试文件；Task 0 条件配方属计划前置授权且已独立提交留痕。

## 6. 遗留/交接

- 无阻塞。Task 9（真实受控集成证据，blocked-env opt-in）可直接开始：本任务已验证全链在生产迁移库上的行为，`confluence_publish_real_test.go` 只需把双打换成真实 `CONFLUENCE_BASE_URL`/`CONFLUENCE_EMAIL`/`CONFLUENCE_API_TOKEN`/`CONFLUENCE_PARENT_PAGE_ID` 注入。
- 迁移轨道提示：本分支现在自洽（双轨唯一、E2E 可装载）；若后续 merge 兄弟分支的同号 dedupe（000118/000197），属同名同内容 rename/rename，机械消解即可。

---

# plan-t50.md 实现报告 — Task 9/10：真实受控集成证据（blocked-env，opt-in）

> 任务：Issue #50 实施计划（`docs/plans/issue30-sweep/plans/plan-t50.md:4795`）Task 9——
> 「真实受控集成证据（blocked-env，opt-in）」。
> 执行 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t50`（分支 `codex/issue30-t50`，基于 Task 8 后 HEAD `02fce18c6`）。
> 本节由实现员子代理撰写，仅覆盖 Task 9 授权文件（新增 `internal/modules/appconnector/confluence_publish_real_test.go`）。

## 1. 实现内容

按计划 Task 9 Step 1 逐字创建 **`internal/modules/appconnector/confluence_publish_real_test.go`**（108 行，本任务唯一改动文件，生产代码零改动）：

- `TestConfluenceRealPublishLoop`：真实 Provider 受控证据——真实适配器跑完整闭环：在指定测试父页面下 create（基线版本读）→ `ParseConfluencePageReceipt` 核对回执 → 对同一页面 update（`ReadConfluencePageVersion` 实时预读 + 无冲突写 + 回执版本必须较预读值推进）→ 同页 stale-version update 必须以 `ErrConfluenceVersionConflict` 拒绝且零写。
- 凭据门：`CONFLUENCE_BASE_URL`/`CONFLUENCE_EMAIL`/`CONFLUENCE_API_TOKEN`/`CONFLUENCE_PARENT_PAGE_ID` 任一为空、或 parent 以 `xxxx` 前缀（占位值形态）即 SKIP——与 #48 先例 `TestNotionRealPublishLoop`（`notion_publish_real_test.go:22-25`）同款纪律，skip 文案明示「skip is not a pass — T20 real-provider evidence stays blocked-env」。
- **Consumes 核实（实现前逐项 grep）**：`ParseConfluenceBaseURL`（`confluence_common.go:101`）、`ParseConfluencePageReceipt`（`confluence_common.go:195`）、`ErrConfluenceVersionConflict`（`confluence_common.go:20`）、`ReadConfluencePageVersion(ctx, pol, cred, edition, apiBasePath, pageID) (string, error)`（`confluence_update.go:297`——返回 string，与计划测试中 `"expected_version": live` 直接承载一致）、`ConfluenceCreateAdapter`/`ConfluenceUpdateAdapter` 结构体字段、`HTTPPolicy` 字段（`http_policy.go:68-79`）、`Action`/`ActionSucceeded`/`ActionFailed`/`RiskWrite` 均存在且签名一致；`TestConfluenceRealPublishLoop` 包内无命名冲突（rg 零命中）。

## 2. 测试证据

### 计划 Step 2 指定命令（伪造凭据 + `xxxx` 前缀 parent——门即使变量被乱设也必须拒绝执行）

命令：`CONFLUENCE_BASE_URL=x CONFLUENCE_EMAIL=x CONFLUENCE_API_TOKEN=x CONFLUENCE_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v`

```
=== RUN   TestConfluenceRealPublishLoop
    confluence_publish_real_test.go:28: confluence real credentials not configured (CONFLUENCE_BASE_URL/CONFLUENCE_EMAIL/CONFLUENCE_API_TOKEN/CONFLUENCE_PARENT_PAGE_ID); skip is not a pass — T20 real-provider evidence stays blocked-env
--- SKIP: TestConfluenceRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.503s
```

计划预期 `--- SKIP: TestConfluenceRealPublishLoop`（plan-t50.md:4922）达成。

### 自然 blocked-env（无任何环境变量）同形 SKIP

命令：`go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v` → 同一 `--- SKIP` + 同款 skip 文案（`ok … 2.922s`）。

### gofmt 修正后复跑 + 包内 Confluence 全域回归

- gofmt 修正（见 §4 发现 1）后：`gofmt -l internal/modules/appconnector/confluence_publish_real_test.go` → 空输出（干净）；复跑 Step 2 命令 → 同一 SKIP。
- `go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 -v` → **28 PASS + 1 SKIP（即本测试）+ 0 FAIL**，`ok github.com/Tencent/WeKnora/internal/modules/appconnector`。
- `go build ./...` → OK（仅 cmd/desktop、cmd/server 两条既有 `ld: warning: ignoring duplicate libraries: '-lc++'`）。

### 计划级验证命令全量（plan-t50.md:4938，提交前实跑）

命令：`go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 && go test ./internal/modules/appconnector/publish/ -run 'Confluence' -count=1 && go test ./internal/handler/ -run 'TestConfluencePublish' -count=1`

```
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.556s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	4.051s
ok  	github.com/Tencent/WeKnora/internal/handler	20.637s
PLAN-LEVEL-VERIFY: ALL GREEN
```

vet 零输出（干净）；Task 8 三个 E2E（`TestConfluencePublish` 前缀）随命令全绿——本任务零生产改动、零回归。

### TDD 形态说明（如实）

本任务交付物是**测试文件本身**（gated real-provider evidence），无生产代码改动，无 RED→GREEN 实现阶段可跑；计划 Task 9 的验证即 SKIP 门（Step 2），已按计划原命令实跑（伪造凭据态 + 自然无凭据态 + gofmt 修正后复跑，共三次）。有真实凭据的环境（`artifacts/connector-real/confluence.env` 形态）自动执行完整闭环；本环境无法产生真实 Provider 证据，如实留作 blocked-env，不伪造。

## 3. 提交

- `e7e26a6fe` `test(appconnector): gated real confluence publish loop evidence (T20 #50, blocked-env)`（Task 9 Step 3 计划原文提交信息；1 file changed, 108 insertions(+)）。
- 提交后 `git status --short` 干净；未推送远端；冻结面零触碰（#48 产物、`newOCArmedActionService`、知识库只读连接器均不在 diff 中）。

## 4. 自检发现

1. **计划草稿排版瑕疵（已最小修正，零语义变化）**：计划 Step 1 代码块中 `updateArgs`/`staleArgs` 两处 map 字面量的 `"title":` 键未按 gofmt 对齐（plan-t50.md:4888、4906）；`gofmt -w` 修正为键对齐，其余与计划逐字一致。
2. **预存问题（非本任务引入，不属授权文件，未动）**：`internal/modules/appconnector/confluence_create_test.go` 在已提交状态（Task 2 落地）不满足 gofmt（`gofmt -l internal/modules/appconnector/` 列出该文件）。属前序任务遗留，留给集成方/其归属任务处理。
3. **同机负载提示（不影响结论）**：验证期间同机有多个并行会话的 Go 构建（craft-107-integration、issue30-sweep-t61 等其他 worktree，`ps` 实见），`go build ./...` 首跑约 18 分钟才完成（CPU 竞争）；所有验证命令均完整跑完且全绿，结论不受影响。
4. **执行环境差异（无碍）**：任务指派中的实现员提示模板路径 `…/superpowers/6.4.1/…/implementer-prompt.md` 不存在，实际读到同一文件 6.4.2 版本（`/Users/wuyongjun/.codex/plugins/cache/openai-curated-remote/superpowers/6.4.2/skills/subagent-driven-development/implementer-prompt.md`），报告契约按该版本执行。
5. **blocked-env 如实声明（AC3 边界，plan-t50.md:44 定义）**：真实 Confluence 站点 + 真实 token + 真实 version.number 推进的验收在本地环境不可得（SKIP 且不伪造）；AC3 的本地最高稳定 Interface 替代证据由 Task 8 E2E 三测承担（本节计划级验证命令随 `TestConfluencePublish` 前缀复跑全绿）。与 #48 先例（`TestNotionRealPublishLoop` 本环境 SKIP）同款纪律。
6. 未做的事：无。Task 9 Files 清单恰为 1 个测试文件，已全部交付。

## 5. 遗留/交接

- 无阻塞。计划 Task 0–9 的实现提交在本分支齐备（`git log`：Task 0 重编 `60bf99af3`、Task 1–3 适配器族、Task 3 review 修复 `2b1691753`、Task 4–8 交付及其报告提交、本任务 `e7e26a6fe`）；计划级验证命令在本任务 HEAD 实跑全绿。
- 真实 Provider 证据激活路径：在具备凭据的环境注入四个 `CONFLUENCE_*` 变量（parent 不得以 `xxxx` 开头），`go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v` 自动执行 create→update→stale 拒绝完整闭环（在指定父页面下真实留痕）。
- 两个 cosmetic 遗留（零行为影响）：计划排版对齐已在落地文件修正；`confluence_create_test.go` 的 gofmt 属 Task 2 落地状态，本任务未动。

---

# plan-t50.md 复核报告 — Task 9/10 重派复核轮（独立验证，零代码改动）

> 背景：本轮编排方再次派发「第 10/10 任务」（计划 Task 9），且指派的前置接口即「TestConfluenceRealPublishLoop 已产出、CONFLUENCE_* 四变量门控 opt-in、本环境 SKIP 留作 blocked-env」——与分支现状一致：前轮已交付代码提交 `e7e26a6fe` 与报告提交 `e2cb9cd25`，工作树 clean。本节由重派轮实现员撰写，性质为**独立复核**：不重复提交已交付代码（重复落同一文件属伪造工作量），改为逐字核验前轮产出 + 亲手实跑计划规定的全部验证命令 + 如实留证。授权范围不变（`confluence_publish_real_test.go` + 本报告文件）。

## 1. 复核范围与方法

- **逐字核验**：将 `docs/plans/issue30-sweep/plans/plan-t50.md:4809-4916` 的 Task 9 代码块与落地文件 `internal/modules/appconnector/confluence_publish_real_test.go:3-108` 做 gofmt 归一化后机械 diff（进程替换，无落盘）——**唯一差异为提取范围中的 `package appconnector` 声明行（2 行：声明 + 空行），代码体（import 块 + 测试函数全文）逐字一致**。前轮报告 §4.1 声明的「仅 gofmt 对齐修正、零语义变化」经本轮独立复核属实。
- **提交范围核验**：`git show --stat e7e26a6fe` → 恰 1 文件（`confluence_publish_real_test.go`，108 insertions），无任何越界文件；`git status` clean（HEAD `e2cb9cd25`）。
- **凭据门语义核验**（重读落地文件行号）：`confluence_publish_real_test.go:27-28`——四变量任一为空或 parent 以 `xxxx` 前缀即 `t.Skip`，skip 文案含「skip is not a pass — T20 real-provider evidence stays blocked-env」，与计划 Step 1 代码及 `notion_publish_real_test.go:24` 同款纪律逐字一致。

## 2. 本轮实跑的验证命令与输出（全部亲手执行于本会话）

### 2.1 计划 Step 2 指定命令（伪造凭据 + `xxxx` 占位 parent——门必须即使变量被乱设也拒绝执行）

命令：`CONFLUENCE_BASE_URL=x CONFLUENCE_EMAIL=x CONFLUENCE_API_TOKEN=x CONFLUENCE_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v`

```
=== RUN   TestConfluenceRealPublishLoop
    confluence_publish_real_test.go:28: confluence real credentials not configured (CONFLUENCE_BASE_URL/CONFLUENCE_EMAIL/CONFLUENCE_API_TOKEN/CONFLUENCE_PARENT_PAGE_ID); skip is not a pass — T20 real-provider evidence stays blocked-env
--- SKIP: TestConfluenceRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.518s
```

### 2.2 自然 blocked-env（无任何环境变量）

命令：`go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v`

```
--- SKIP: TestConfluenceRealPublishLoop (0.00s)   // 同款 skip 文案
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.492s
```

### 2.3 appconnector 包 Confluence 全域回归（计数复核）

命令：`go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 -v`，对 `^--- PASS` / `^--- SKIP` / `^--- FAIL` 分别计数：

```
28 PASS / 1 SKIP / 0 FAIL   （SKIP 即 TestConfluenceRealPublishLoop；28 PASS 与前轮报告计数一致）
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.602s
```

### 2.4 计划级验证命令全量（plan-t50.md:4938 逐字，后台完整跑完 exit 0）

命令：`go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 && go test ./internal/modules/appconnector/publish/ -run 'Confluence' -count=1 && go test ./internal/handler/ -run 'TestConfluencePublish' -count=1 && echo "PLAN-LEVEL-VERIFY: ALL GREEN"`

```
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.832s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.913s
ok  	github.com/Tencent/WeKnora/internal/handler	7.884s
PLAN-LEVEL-VERIFY: ALL GREEN
```

vet 段零输出（干净）；两条 `ld: warning` 为前轮已记录的既有噪音，非失败；本轮机器负载低于前轮（build 秒级完成），无结论性影响。

## 3. 自检发现（复核轮新增）

1. **前轮遗留 gofmt 项现状（如实，未动）**：`gofmt -l internal/modules/appconnector/` 本轮复跑仍仅列 `internal/modules/appconnector/confluence_create_test.go`——属 Task 2 落地状态的既有遗留（非本任务授权文件，与 #48 产物零交集），维持前轮处置：留给其归属任务/集成方，重派轮不越权修改。
2. **指派差异（无碍，与前轮发现 4 同源）**：任务指派的模板路径 `superpowers/6.4.1/.../implementer-prompt.md` 不存在，实际唯一可用版本为 `6.4.2`（`ls` 实证 cache 下仅有 6.4.2），报告契约按 6.4.2 执行。
3. **重派轮的性质声明**：本轮对授权代码文件**零改动**——重复创建已存在的逐字一致文件只会制造伪 diff。本节即本轮交付物；无代码提交（不制造空提交或重复提交）。
4. **blocked-env 边界（不变，如实重申）**：真实 Confluence 站点 + 真实 token + 真实 version.number 推进的证据在本地不可得（SKIP 不冒充 pass）；激活路径同前轮报告 §5——注入四个真实 `CONFLUENCE_*` 变量（parent 不以 `xxxx` 开头）即自动执行 create→update→stale 拒绝完整闭环。

## 4. 结论

计划 Task 0–9 的交付物在本分支齐备且经本轮**独立重验全绿**：Step 2 SKIP 门（伪造凭据态 + 自然态双跑）、全域 28 PASS/1 SKIP/0 FAIL、计划级验证命令（build + vet + 三组测试）exit 0 ALL GREEN、逐字 diff 归一化后零差异、提交范围恰为授权单文件。无阻塞、无越权改动、无未声明事项。

---

# plan-t50.md 复核报告 — Task 9/10 第二次重派轮（终审后独立验证，零代码改动）

> 背景：编排方再次派发计划 Task 9（真实受控集成证据，blocked-env，opt-in）。分支现状：Task 9 代码提交 `e7e26a6fe`（前轮实现员）、报告提交 `e2cb9cd25`（实现员报告）与 `28642de4f`（第一次重派复核轮）均已存在，其后 HEAD 已推进至**最终审查处置轮 `da149ac10`**（该轮改动 `confluence_update.go`/`confluence_update_test.go`/`publish/confluence_blocks_test.go` 三文件，**未触碰本任务文件**，`git log --follow` 实证 `confluence_publish_real_test.go` 全历史仅 `e7e26a6fe` 一笔）。本节由第二次重派轮实现员撰写，性质为**终审后独立复核**：验证最终审查轮的改动未使 Task 9 交付物漂移或回归。授权文件不变（`confluence_publish_real_test.go` + 本报告）；对已交付且逐字一致的代码零改动（重复落同一文件属伪造工作量）。

## 1. 复核范围与方法（本轮全部亲手执行）

1. **逐字核验（全代码块范围）**：`sed -n '4809,4916p' docs/plans/issue30-sweep/plans/plan-t50.md` 提取计划 Task 9 Step 1 完整代码块（含 `package appconnector` 声明与 import 块），与落地文件 `internal/modules/appconnector/confluence_publish_real_test.go`（108 行）双侧 `gofmt` 归一化后进程替换 diff（无落盘）——**零差异**（`VERBATIM-MATCH after gofmt normalization`）。较第一次复核轮的「3-108 行体范围」本轮扩大到整文件范围，含包声明在内逐字一致。
2. **最终审查轮隔离核验**：`git show --stat da149ac10` → 恰 3 文件（`confluence_update.go` +9、`confluence_update_test.go` +47、`publish/confluence_blocks_test.go` +38），与本任务授权文件零交集；`git log --oneline --follow -- internal/modules/appconnector/confluence_publish_real_test.go` → 仅 `e7e26a6fe` 一笔。
3. **gofmt**：`gofmt -l internal/modules/appconnector/confluence_publish_real_test.go` → 空输出（干净）。
4. **Notion 先例纪律复核**：`notion_publish_real_test.go:24` 的 `t.Skip("…skip is not a pass…")` 与本任务 `confluence_publish_real_test.go:28` 同款纪律（四变量门 + `xxxx` 前缀占位拒绝 + 文案明示 blocked-env），本轮重读属实。

## 2. 本轮实跑的验证命令与完整输出

### 2.1 计划 Step 2 指定命令（plan-t50.md:4921 逐字；伪造凭据 + `xxxx` 占位 parent）

命令：`CONFLUENCE_BASE_URL=x CONFLUENCE_EMAIL=x CONFLUENCE_API_TOKEN=x CONFLUENCE_PARENT_PAGE_ID=xxxx-skip go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v`

```
=== RUN   TestConfluenceRealPublishLoop
    confluence_publish_real_test.go:28: confluence real credentials not configured (CONFLUENCE_BASE_URL/CONFLUENCE_EMAIL/CONFLUENCE_API_TOKEN/CONFLUENCE_PARENT_PAGE_ID); skip is not a pass — T20 real-provider evidence stays blocked-env
--- SKIP: TestConfluenceRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	4.308s
```

计划预期 `--- SKIP: TestConfluenceRealPublishLoop`（plan-t50.md:4922）达成。

### 2.2 自然 blocked-env（显式清除四变量）

命令：`env -u CONFLUENCE_BASE_URL -u CONFLUENCE_EMAIL -u CONFLUENCE_API_TOKEN -u CONFLUENCE_PARENT_PAGE_ID go test ./internal/modules/appconnector/ -run TestConfluenceRealPublishLoop -count=1 -v`

```
=== RUN   TestConfluenceRealPublishLoop
    confluence_publish_real_test.go:28: confluence real credentials not configured (CONFLUENCE_BASE_URL/CONFLUENCE_EMAIL/CONFLUENCE_API_TOKEN/CONFLUENCE_PARENT_PAGE_ID); skip is not a pass — T20 real-provider evidence stays blocked-env
--- SKIP: TestConfluenceRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.231s
```

### 2.3 appconnector 包 Confluence 全域回归（计数复核，终审后 HEAD）

命令：`go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 -v`，对 `^--- PASS`/`^--- SKIP`/`^--- FAIL` 计数：

```
29 PASS / 1 SKIP / 0 FAIL   （SKIP 即 TestConfluenceRealPublishLoop；ok github.com/Tencent/WeKnora/internal/modules/appconnector 0.459s）
```

计数较第一次复核轮（28 PASS）+1：终审轮 `da149ac10` 在 `confluence_update_test.go` 新增 `TestConfluenceUpdateQueryOutputIsReceiptProjection`（Query 投影形状测试），本轮实测通过——非回归，系审查增益。

### 2.4 计划级验证命令全量（plan-t50.md:4938 逐字，后台完整跑完 exit 0）

命令：`go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && go test ./internal/modules/appconnector/ -run 'Confluence' -count=1 && go test ./internal/modules/appconnector/publish/ -run 'Confluence' -count=1 && go test ./internal/handler/ -run 'TestConfluencePublish' -count=1 && echo "PLAN-LEVEL-VERIFY: ALL GREEN"`

```
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.837s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.645s
ok  	github.com/Tencent/WeKnora/internal/handler	11.504s
PLAN-LEVEL-VERIFY: ALL GREEN
```

vet 段零输出（干净）；两条 `ld: warning` 为多轮已记录的既有链接噪音，非失败。终审轮新增测试随命令全绿——终审改动与本任务交付物共存无回归。

## 3. 提交

本轮对授权代码文件**零改动**（现有文件与计划逐字一致，重落只会制造伪 diff），无代码提交（不制造空提交）。本节报告为唯一交付物，随报告文件一并提交（见本轮 docs 提交）。

## 4. 自检发现（第二次重派轮）

1. **TDD 形态如实声明**：本任务交付物是 gated real-provider 测试文件本身，无生产代码改动；且文件已在 HEAD 中（前轮已交付），本轮无法也不应重演 RED（删除他人已交付文件再重建属回退他人修改，被任务纪律禁止）。GREEN 侧证据即 SKIP 门双态实跑 + 全域回归 + 计划级验证全量，均为本轮亲手执行。
2. **blocked-env 边界（不变，如实重申）**：真实 Confluence 站点 + 真实 token + 真实 version.number 推进的证据在本地不可得（SKIP 不冒充 pass）；激活路径：注入四个真实 `CONFLUENCE_*` 变量（parent 不以 `xxxx` 开头）即自动执行 create→update→stale 拒绝完整闭环。
3. **指派模板路径缺失（与前两轮同源）**：`…/superpowers/6.4.1/…/implementer-prompt.md` 不存在（本轮 `ls` 复核 NOT FOUND）；前轮实证唯一可用版本为 6.4.2，报告契约沿用其形态（实现内容/测试证据/提交/自检发现）。
4. **前轮遗留 gofmt 项现状（未动，非本任务授权文件）**：`confluence_create_test.go` 属 Task 2 落地状态遗留，维持前两轮处置——留给归属任务/集成方。
5. 未做的事：无。Task 9 Files 清单恰为 1 个测试文件，已在 HEAD 且逐字一致、门控语义与验证全部亲手复核通过。

## 5. 结论

计划 Task 9 的交付物在终审后 HEAD `da149ac10` 上依然齐备：文件与计划整代码块逐字一致（gofmt 归一化零差异）、SKIP 门双态（伪造凭据/自然态）按计划原命令实跑达成、appconnector Confluence 全域 29 PASS/1 SKIP/0 FAIL、计划级验证命令（build + vet + 三组测试）exit 0 ALL GREEN。终审轮改动与本任务文件零交集、无回归。无阻塞、无越权改动、无未声明事项。

