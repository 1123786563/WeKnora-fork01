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
