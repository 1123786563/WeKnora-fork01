# plan-t49.md 实施报告

## Task 2: 飞书 docx Adapter——Execute/Query、部分成功恢复、可靠核对

- **状态**: DONE（含 3 处计划样例代码内部矛盾的修正，详见「对计划样例代码的修正」）
- **提交**: `281b13b48` feat(appconnector): 飞书 docx Adapter——多步写入/进度恢复/语义核对（FE-03）
- **分支**: `codex/issue30-t49`（worktree `.worktrees/issue30-sweep-t49`）
- **变更文件**（仅任务授权文件，894 行新增，零删除）:
  - `internal/modules/appconnector/feishu_docx.go`（+450：追加 Adapter 与传输层）
  - `internal/modules/appconnector/feishu_docx_test.go`（+444：追加 fakeFeishuDocx 契约替身与 9 个测试）

---

## 1. 实现内容

### 1.1 Produces 接口（Task 3 按签名逐字消费，已 grep 核对逐字一致）

- `type FeishuDocxAdapter struct { Policy HTTPPolicy; Token func(ctx) (string, error); ConnectionCapabilities func(ctx, Action) ([]string, error); Recheck func(ctx, Action) error; LoadProgress func(Action) FeishuDocProgress; SaveProgress func(Action, FeishuDocProgress) error; MaxBatch int }`（feishu_docx.go:439-455）
- `var _ Adapter = (*FeishuDocxAdapter)(nil)`（:457）
- `func (m *FeishuDocxAdapter) Execute(ctx context.Context, a Action) (ActionResult, error)`（:511）
- `func (m *FeishuDocxAdapter) Query(ctx context.Context, a Action) (ActionResult, error)`（:645）
- `func ReadFeishuDocumentVersion(ctx context.Context, policy HTTPPolicy, token func(context.Context) (string, error), documentID string) (FeishuDocVersion, error)`（:864）

### 1.2 执行语义（与 Notion 家族逐点对齐，`notion_create.go:488-560` / `notion_update.go:399-505`）

- **Execute 顺序**：configError → A03 Recheck（吊销 → `ActionAwaitingApproval`，同 `feishu_send.go:284-288` 先例）→ `IsFeishuDocUpdateArgs` 路由 → update 分支先 `ReadFeishuDocumentVersion` 预读 + `DetectFeishuRevisionConflict`（预读是 GET，绝不产生写；失败=终局 failed）→ capability 检查（AC2，写之前零网络）→ 多步写入。
- **create**：`POST FeishuDocumentCreatePath`（body 仅 `{"folder_token"}`）→ provider 确认的 `document_id` 先落 progress（`FeishuDocProgress{DocumentID, 0}`）→ 按批 `POST children`（`block_id=document_id`、`index:-1`、每批 ≤`batchSize()`，`MaxBatch=0` 时=`FeishuDocAppendBatchLimit=50`）→ 每批成功后落 `BlocksDone`。恢复时持久化 document id 强制同一文档、绝不二次 create。
- **update**：预读 → 冲突检测 → 分批 append → 读回 `GET document`（读回丢失=unknown，不谎报 failed）→ 输出证据=读回 envelope（含新 revision）。
- **unknown 判定**：传输失败/5xx/响应不可解析 → `ErrFeishuPublishOutcomeUnknown`；progress 持久化失败 → unknown；快照畸形/capability 缺失/预读失败/provider 4xx → 终局 failed；HTTP 404 → typed `ErrFeishuPublishNotFound`。
- **Query（部分成功核对）**：无持久化 `DocumentID` → 诚实 unknown（飞书无可靠标题搜索等价物，绝不凭标题认领、绝不 re-create）；有 → 存在性 GET + `readAllChildren` 循环 `page_token` 收集全部子块（Review Focus 3）→ `feishuDocBlockContents` 语义比对（text_run content 序列，非整块字节，Review Focus 5）→ `containsPrefix` 连续段包含（外部协作者可在前后追加段落）→ 全部证实 → succeeded + 回执。
- **传输层**（`do`，feishu_docx.go:763-802）：`Policy.ValidateRequest` 先行（policy 拒绝=请求不出门）→ Bearer token → `Policy.NewClient().Do`（A04 DNS/重定向复验）→ 1MiB 限读 → 5xx=unknown / 404=typed NotFound / 其余 4xx=终局 provider error（body 截断 200 字节）。

### 1.3 测试（fakeFeishuDocx 契约替身，复刻官方 docx/v1 wire 形状）

9 个测试全绿：create 全回路（1 create + [2,1] 两批、内容逐段断言）、断点恢复（丢末批回复 → unknown、checkpoint=4 → 恢复时零重复 create、`appendSizes=[2,2,1,1]`、6 子块=5 批准+1 诚实重复尾）、update 冲突零写入（AC1）、update 快乐路径回执版本=写后 revision、只读 capability 拒绝零网络写（AC2）、Query 语义核对不重发、无 DocumentID 诚实 unknown 不 re-create、Query 循环分页（fake 每页 1 条）、`ReadFeishuDocumentVersion` 预读。

---

## 2. TDD 证据

### RED（先写测试，确认失败）

命令：

```
go test ./internal/modules/appconnector/ -run 'TestFeishuDocxAdapter|TestReadFeishuDocumentVersion' -count=1
```

输出（预期中的编译失败，与计划 Step 2 的 Expected 逐字一致）：

```
# github.com/Tencent/WeKnora/internal/modules/appconnector [github.com/Tencent/WeKnora/internal/modules/appconnector.test]
internal/modules/appconnector/feishu_docx_test.go:360:81: undefined: FeishuDocxAdapter
internal/modules/appconnector/feishu_docx_test.go:371:10: undefined: FeishuDocxAdapter
internal/modules/appconnector/feishu_docx_test.go:610:12: undefined: ReadFeishuDocumentVersion
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector [build failed]
```

### GREEN（实现后）

命令：

```
go test ./internal/modules/appconnector/ -run 'TestFeishuDocxAdapter|TestReadFeishuDocumentVersion' -count=1 -v
```

输出尾部：

```
--- PASS: TestFeishuDocxAdapterCreateFullLoop (0.01s)
--- PASS: TestFeishuDocxAdapterCreateResumesFromCheckpoint (0.01s)
--- PASS: TestFeishuDocxAdapterUpdateRevisionConflictRefusesBeforeWrite (0.00s)
--- PASS: TestFeishuDocxAdapterUpdateHappyPathAppends (0.00s)
--- PASS: TestFeishuDocxAdapterReadOnlyCapabilityRefused (0.00s)
--- PASS: TestFeishuDocxAdapterQueryReconcilesByContent (0.01s)
--- PASS: TestFeishuDocxAdapterQueryWithoutDocumentIDStaysUnknown (0.00s)
--- PASS: TestFeishuDocxAdapterQueryFollowsPagination (0.01s)
--- PASS: TestReadFeishuDocumentVersionPreRead (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	1.783s
```

注：计划 Step 4 写「11 个 Adapter 测试全绿」，按其 run pattern 实际命中的测试函数为 9 个（8 个 Adapter + 1 个 ReadFeishuDocumentVersion 预读）；以实际运行为准。

---

## 3. 回归与静态检查（均已实际运行）

- 计划 Step 5 既有域零回归：`go test ./internal/modules/appconnector/ -run 'TestNotion|TestFeishuSend|TestFeishuRealControlledSend' -count=1` → `ok ... 0.493s`
- 包全量：`go test ./internal/modules/appconnector/ -count=1` → `ok ... 0.525s`
- `go vet ./internal/modules/appconnector/` → 零输出，exit 0
- `gofmt -l internal/modules/appconnector/` → 零输出

---

## 4. 对计划样例代码的修正（均为计划内部自相矛盾，非需求偏离；对外 Produces 签名零改动）

1. **`feishuCreateAction` 的 want 与派生块不一致**：计划样例中 blocks 全部派生自 `"段落。"`（`strings.Repeat("段落。\n\n", n)`），而 `want` 断言为 `段落N。`——正确实现也必挂 `TestFeishuDocxAdapterCreateFullLoop` 的内容断言。修正：blocks 与 `want` 从同一组编号段落（`段落1。…段落N。`）共同派生，保持测试意图（逐段不同内容证明逐块保真）。
2. **`dropNextAppend` 单布尔标志与测试注释矛盾**：计划 fake 只能丢第一个 append 的回复，而两个测试注释明确要求丢最后一个批次（"batch3(1) lands REMOTELY but loses its reply"、"the final batch's reply is lost"）。修正为 `dropAppendAt int`（1-based 丢弃序号），首次运行实测复现了该矛盾（checkpoint=0 而非 4；"2 of 3 approved paragraphs present in order"），修正后语义与注释完全一致，且恢复测试真正覆盖「中段批次成功+末批丢失」的部分成功形状。
3. **`readAllChildren` 把 `?page_token=` 拼进 `url.URL.Path`**：query 文本会被 `url.URL.String()` 转义为路径（`%3Fpage_token%3D`），fake 永远收不到真 query，计划的分页测试自身必挂。修正：内部辅助 `do`/`targetURL` 沿用本包既有先例签名（`feishu_send.go:220` `targetURL(path string, query url.Values) *url.URL`、`feishu_send.go:242` `do(ctx, method, u *url.URL, …)`），`page_token` 走 `RawQuery`。对外接口不受影响。
4. **`executeUpdate` 中 `final` 声明未使用**（编译错误）：读回校验语义保留（读回必须能解析出真实 id/revision，否则 unknown），改为 `if _, perr := ParseFeishuDocumentVersion(raw); perr != nil` 形式。
5. **fake children 补注服务端字段**（按计划正文规范性要求，非样例代码偏离）：计划 Task 2 正文（Review Focus 5 及「fake 与 E2E 的 children 响应携带 block_id/parent_id 等服务端字段」）与测试注释均要求 children 携带服务端字段，但样例 fake 原样存储快照字节。fake 的 POST children 现为每个入库子块标注 `block_id`/`parent_id`，使「语义比对」成为唯一可通过路径——若实现退化为整块字节比对，`TestFeishuDocxAdapterQueryReconcilesByContent` 必挂。

---

## 5. 自审发现

- **完整性**：Produces 5 项导出签名 grep 核对与计划逐字一致；9 项执行语义点全部落实现；fake 逐字复刻 wire 形状（信封 `{code,msg,data}`、数值 `revision_id`、`page_token`/`has_more` 分页、子块服务端字段、丢回复=效果已落地的部分成功形状）。
- **纪律**：只改 2 个授权文件；未 import 官方 SDK（常量锚定，运行时零依赖）；未新增迁移、未动 #48 集成文件；未派发子代理；未推送远端。
- **边界**：`readAllChildren` 循环终止条件为 `!has_more || page_token==""`（双保险防服务端异常下的死循环——若 has_more 恒真且 token 恒非空会一直循环，真实服务端按官方合同不会出现，与 Notion 同位置同假设）。
- **无遗留疑虑**：未发现需要后续任务处理的缺口；Task 3 的 `FeishuBridge`/`ProviderProfile` 可直接按 Produces 签名消费。

---

## 6. 任务间接口快照（供 Task 3）

Task 1+2 合并后的 `appconnector` 包导出面（均已在 `feishu_docx.go`）：常量 5 + 包内 `feishuTextRunChunk`/`feishuDocMaxParagraphs`；哨兵 8（`ErrFeishuPublishNotFound` typed）；`FeishuDocProgress`/`FeishuDocCreateSnapshot`/`FeishuDocUpdateSnapshot`/`FeishuDocVersion`/`FeishuDocReceipt` + Parse 函数；`IsFeishuDocUpdateArgs`/`DetectFeishuRevisionConflict`/`FeishuTextBlocks`；`FeishuDocxAdapter`（Execute/Query）+ `ReadFeishuDocumentVersion`；包内 `feishuDocBlockContents`。

---
---

# Task 3: publish 包 provider 泛化（ProviderProfile）+ FeishuBridge

- **状态**: DONE（含 4 处计划样例代码内部矛盾的修正，详见「对计划样例代码的修正」）
- **提交**: `1c08c9de1` refactor(publish): 服务层 provider 中立化（ProviderProfile）+ FeishuBridge——飞书差异只存在于 Adapter
- **分支**: `codex/issue30-t49`（worktree `.worktrees/issue30-sweep-t49`）
- **变更文件**（仅任务授权文件，683 行新增 / 22 行删除）:
  - `internal/modules/appconnector/publish/provider.go`（新建：ProviderProfile + NotionProfile + FeishuProfile）
  - `internal/modules/appconnector/publish/feishu.go`（新建：FeishuBridge + denied stub + 预读 + 生产端口）
  - `internal/modules/appconnector/publish/plan.go`（修改：8 处点状替换 + struct profile 字段 + NewProviderPublishService；NewNotionPublishService 签名不变）
  - `internal/modules/appconnector/publish/dispatcher.go`（修改：NotionConnectionScope +Scopes 字段 + scope 构造 +1 行，恰好 +5 行）
  - `internal/modules/appconnector/publish/provider_test.go`（新建：双 profile 对称表驱动）
  - `internal/modules/appconnector/publish/feishu_test.go`（新建：Bridge 路由/AC2 门/预读/生产策略/profile 形状 8 测试）

---

## 1. 实现内容

### 1.1 Produces 接口（Task 4/5 与 #50 消费，签名逐字）

- `type ProviderProfile struct { AppID, Provider, ActionVersion, ConflictResultPrefix string; BlocksOf func(string) ([]json.RawMessage, error); CreateArgs func(parent, title string, blocks []json.RawMessage) ([]byte, error); UpdateArgs func(destination, expectedVersion, title string, blocks []json.RawMessage) ([]byte, error); ReadRemoteVersion func(ctx, connectionID, destination string) (string, error); ParseReceipt func(providerResult string) (externalID, externalVersion string, err error) }`（provider.go:23-46）
- `func NotionProfile(remote NotionRemoteReader) ProviderProfile`（provider.go:49）；`func FeishuProfile(read NotionRemoteReader) ProviderProfile`（provider.go:74）
- `func NewProviderPublishService(actions, store, pubs, artifacts, content, scopes, profile) *NotionPublishService`（plan.go:141）；`NewNotionPublishService` 签名不变、转发 NotionProfile（plan.go:120-134，#48 调用点零改动）
- `type FeishuBridge struct` + `func NewFeishuBridge(scopes, policies, tokens, pubs) *FeishuBridge`（feishu.go:39-47），实现三接口断言（feishu.go:49-51）：`appconnectorsvc.ActionDispatcher` + `appconnectorsvc.UnknownResolver` + `NotionRemoteReader`
- `const FeishuVersionConflictResult = "feishu_docx_revision_conflict"`（feishu.go:25）
- `func NewConstantFeishuPolicyProvider() NotionPolicyProvider`（feishu.go:225）——https/open.feishu.cn/GET+POST//open-apis/docx//30s/无 AuthorizedNetworks
- `NewFeishuCredentialTokenSource = NewCredentialTokenSource`（feishu.go:242）
- `NotionConnectionScope.Scopes []string`（dispatcher.go:34-39）+ `dbNotionScopeSource.NotionScope` 透传 `parsed.Scopes`（dispatcher.go:307），`InsertCapability` 投影行为不变

### 1.2 plan.go 8 处点状替换（全部落实，grep 核验）

① struct +`profile ProviderProfile` 字段（plan.go:113-122）；② AppID 检查走 `s.profile.AppID`（:174）；③ `BlocksOf`（:211）；④ **AC1 预读判定的飞书语义适配**（:215-228）：`ReadRemoteVersion` 错误非 typed not-found 才拒绝，update 恒要求 `(rerr==nil && version!="")`，create 允许空 baseline——update 的 fail-closed 保持（destination 恒为既往 published document id，被删即拒）；⑤ args 构造走 `CreateArgs/UpdateArgs`（:231/:233）；⑥ ActionVersion（:240）；⑦ Provider 行（:248）；⑧ `project` 的 Conflict 判定走 `s.profile.ConflictResultPrefix`（:320，带 `!=""` 零值防御）与 ParseReceipt/settle（:328-331）。副产物：`encoding/json` import 从 plan.go 移除（替换后无使用者）。`s.remote` 与 `"notion"` 字面量在 plan.go/feishu.go 零残留（grep 核验）。

### 1.3 FeishuBridge 结构

`adapterFor`：scope（非 feishu → ErrDispatchNotStarted）→ policy → token → **AC2 写能力门** → 构造 `appconn.FeishuDocxAdapter`（caps 闭包 + publication row progress_json 的 FE-03 checkpoint，load/save 用 `context.Background()`）。`run` 的 outcome 映射与 NotionBridge.run 逐点同构：FAILED → 终局 failed outcome（`ErrFeishuPublishRevisionConflict` 改写为 `FeishuVersionConflictResult` 前缀），UNKNOWN → unknown outcome，AwaitingApproval → failed("approval revoked")，wiring gap → ErrDispatchNotStarted。`ReadPageVersion`：经 ReadFeishuDocumentVersion 预读，`ErrFeishuPublishNotFound` 折叠为 `("", nil)` 空 baseline（folder 无 revision 概念的诚实回答；update 侧由 plan 层第二分支拒绝，fail closed）。

---

## 2. 测试命令与完整输出

### TDD RED（Step 2，实现前）

```
$ go test ./internal/modules/appconnector/publish/ -run 'TestPublishServiceProfiles|TestFeishuBridge|TestConstantFeishuPolicy|TestFeishuProfile|TestNotionProfileUnchanged' -count=1
internal/modules/appconnector/publish/feishu_test.go:25:88: undefined: FeishuBridge
internal/modules/appconnector/publish/feishu_test.go:26:9: undefined: NewFeishuBridge
internal/modules/appconnector/publish/feishu_test.go:32:3: unknown field Scopes in struct literal of type NotionConnectionScope
...
FAIL	github.com/Tencent/WeKnora/internal/modules/appconnector/publish [build failed]
```

失败原因即预期缺口：FeishuBridge/FeishuProfile/NewProviderPublishService/Scopes 字段尚未存在。

（实现中途另有两次编译级 RED，均为计划样例笔误，见第 4 节 1-3 条。）

### TDD GREEN（Step 4，实现后）

```
$ go test ./internal/modules/appconnector/publish/ -run 'TestPublishServiceProfiles|TestFeishuBridge|TestConstantFeishuPolicy|TestFeishuProfile|TestNotionProfileUnchanged' -count=1 -v
=== RUN   TestFeishuBridgeRefusesNonFeishuConnection
--- PASS: TestFeishuBridgeRefusesNonFeishuConnection (0.00s)
=== RUN   TestFeishuBridgeRefusesReadOnlyScope
--- PASS: TestFeishuBridgeRefusesReadOnlyScope (0.00s)
=== RUN   TestFeishuBridgeRoutesUpdateSnapshotToAdapter
--- PASS: TestFeishuBridgeRoutesUpdateSnapshotToAdapter (0.00s)
=== RUN   TestFeishuBridgeReadPageVersionDelegatesToAdapter
--- PASS: TestFeishuBridgeReadPageVersionDelegatesToAdapter (0.00s)
=== RUN   TestConstantFeishuPolicyProviderPinsContract
--- PASS: TestConstantFeishuPolicyProviderPinsContract (0.00s)
=== RUN   TestFeishuProfileSnapshotShapes
--- PASS: TestFeishuProfileSnapshotShapes (0.00s)
=== RUN   TestNotionProfileUnchanged
--- PASS: TestNotionProfileUnchanged (0.00s)
=== RUN   TestPublishServiceProfilesAreSymmetric
=== RUN   TestPublishServiceProfilesAreSymmetric/notion
=== RUN   TestPublishServiceProfilesAreSymmetric/feishu
--- PASS: TestPublishServiceProfilesAreSymmetric (0.01s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	2.715s
```

### 回归门

```
$ go test ./internal/modules/appconnector/publish/ -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	2.522s
```

**既有 `plan_test.go`/`dispatcher_test.go`/`blocks_test.go` 全绿——NewNotionPublishService 签名未变、Notion 行为逐点保持，是本任务最重要的回归证据。**

```
$ go test ./internal/modules/appconnector/ -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	1.292s
```

```
$ go build ./...
BUILD_OK   （exit 0；仅 cmd/server、cmd/desktop 的 ld duplicate-libraries 链接警告，与本改动无关的既有现象）
$ go vet ./internal/modules/appconnector/publish/
VET_OK
```

### 已知既有失败（与本任务无关，如实声明）

```
$ go test ./internal/handler/ -run 'TestNotion|TestAppPublications' -count=1
--- FAIL: TestAppPublicationsTableExistsAfterMigrations
--- FAIL: TestNotionPublishEndToEndCreateApprovePublishReceipt
--- FAIL: TestNotionPublishEndToEndUpdateConflict
--- FAIL: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst
```

三个 E2E 失败输出均为同一行（grep 计数 3）：`failed to open source, "file://…/migrations/sqlite": duplicate migration file: 000114_public_agent_marketplace.down.sql` —— 即计划 Global Constraints 第 6 条与 Task 0 记载的迁移撞号问题（`migrations/` 现状 000114 双占 + versioned 000193 双占，属 Task 0 授权范围）。失败发生在迁移装载阶段（打开 DB 即挂），未执行到任何 publish 代码；本任务 diff 零触碰 `migrations/`（`git status` 核验）。

---

## 3. 对计划样例代码的修正（内部矛盾，参照 Task 2 先例如实记录）

1. **`require.Error(t, execErr)` → `require.NoError(t, execErr)`**（provider_test.go 对称断言）：`ActionService.Execute` 对 definitive failed outcome 返回 `nil` error（`action.go:482`——只有 Unknown 才返回 `ErrDispatchUnknown`；既有 `TestExecuteUpdateConflictSettlesFailedReceipt` 佐证 failed outcome 时 `svc.Execute` 无错）。计划样例的 `require.Error` 必挂。修正后断言强度不减反增：补 `require.Equal(t, appconn.ActionFailed, outcome.ActionState)` + `PublicationFailed` settle 断言，两个 provider 对称成立。
2. **AC2 拒绝被 adapter configError 掩盖**：计划让桥的 caps 闭包在缺 `write_docx` 时返回错误、依赖 adapter 的 `requireWriteCapability` 报出含 `write_docx` 的 ProviderResult；但 Task 2 已提交实现（与计划 Task 2 样例一致）中 `Execute` 的 `configError()` 先于 capability 校验（`feishu_docx.go:512` vs `:531`），空测试 policy 下 ProviderResult 会是 `not_configured`，`TestFeishuBridgeRefusesReadOnlyScope` 的 `require.Contains(…, "write_docx")` 必挂。修正（实现侧，落在本任务授权文件 feishu.go）：`adapterFor` 在构造 adapter 之前先跑同一 caps 闭包，缺失时返回 `feishuCapabilityDenied` fail-closed stub（Execute/Query 直接返回 `ActionFailed` + `feishu_docx_missing_capability: connection lacks write_docx`，零网络调用）。caps 闭包原样传入真 adapter 保持双保险。语义与计划测试意图逐字一致。
3. **计划样例 import 缺漏 3 处**：feishu_test.go 缺 `repoappconn`（`repoappconn.NewPublicationStore` 使用处）；provider_test.go 的 `errors` 未使用（修正断言 1 后无 `errors.` 引用）；`NotionProfile` 的 `BlocksOf: appconn.NotionParagraphBlocks` 实际定义在 publish 包本地（`blocks.go:53`），改为包内直接引用。
4. **`TestFeishuProfileSnapshotShapes` 的 `raw` map 复用**：`json.Unmarshal` 到非 nil map 是合并语义，create 断言后的 3 键残留导致 update 的 `require.Len(raw, 4)` 实得 5（首跑即暴露）。修正：update 断言改用独立 `updateRaw` map。

（另按计划文本自身内联修正项落实：`newFeishuPlanEnv` 的 artifacts 用 `repository.ArtifactVersion` 字面量，未引入计划已自弃的 `planArtifactVersion`。）

---

## 4. 自审发现

- **完整性**：Produces 9 项全部落地且签名与计划逐字一致；plan.go 恰好 8 处点状替换 + struct 字段 + 新构造函数，无其它改写；dispatcher.go 恰好 +1 字段 +1 行赋值——与「并行批次约束」清单逐条吻合（`git diff --stat`：dispatcher.go +5、plan.go +49/-22、其余全为新文件）。
- **AC1 证据**：`TestPublishServiceProfilesAreSymmetric` 用同一组断言驱动 notion/feishu 双 profile 走完 FormPlan → publication row 品牌校验 → snapshot 形状校验 → Approve → Execute → PublicationFailed settle 全链，服务层零 provider 分支（`s.remote`/`"notion"` 字面量 grep 零残留）。
- **AC2 证据**：只读 scope（`read_docx`/`sync_content`）在桥层短路为终局 failed outcome，ProviderResult 携带 `write_docx` 缺失原因，零网络调用（denied stub 无任何 HTTP 面）。
- **边界**：`remote` 字段按计划 ① 的 new 定义保留在 struct 上（当前无读写处；`go vet` 通过；删除它会偏离计划逐字要求，故保留并在报告中声明）。
- **遗留**：`internal/handler/` 的 4 个既有失败为迁移撞号（Task 0 范围），Task 4/5 需 Task 0 先落地才能跑 E2E；本任务 Produces 已按签名就绪，Task 4 可直接消费。

## 5. 任务间接口快照（供 Task 4/5）

`publish` 包新增导出面：`ProviderProfile`/`NotionProfile`/`FeishuProfile`/`NewProviderPublishService`/`FeishuBridge`/`NewFeishuBridge`/`FeishuVersionConflictResult`/`NewConstantFeishuPolicyProvider`/`NewFeishuCredentialTokenSource`/`FeishuScopeSource`；`NotionConnectionScope` 增 `Scopes []string`。Task 4 装配点：`container` 侧用 `NewProviderPublishService(actions, store, pubs, artifacts, content, scopes, FeishuProfile(bridge))` + `NewConstantFeishuPolicyProvider` + `NewFeishuCredentialTokenSource` 组装 feishu 发布服务（镜像 notion 装配），handler 409 映射用 `FeishuVersionConflictResult` 前缀。

---

# Task 4: 飞书发布 HTTP 面——handler、router、container 装配

- **状态**: DONE（计划样例代码逐字落地，零计划修正）
- **提交**: `b297a9507` feat(feishu-publish): 飞书发布 HTTP 面——handler/router/container 装配
- **分支**: `codex/issue30-t49`（worktree `.worktrees/issue30-sweep-t49`）
- **变更文件**（仅任务授权文件，417 行新增，零删除；既有文件 diff 恰为计划声明的 +1 字段 +1 Register 行 +1 Provide 行）:
  - `internal/handler/app_connector_feishu_publish.go`（新建，219 行）
  - `internal/handler/app_connector_feishu_publish_test.go`（新建，123 行）
  - `internal/router/routes_app_feishu_publish.go`（新建，24 行）
  - `internal/router/router.go`（修改，+4：`AppFeishuPublishHandler` 参数字段 :146-148、`RegisterAppFeishuPublishRoutes` 注册行 :440）
  - `internal/container/feishu_publish.go`（新建，46 行）
  - `internal/container/container.go`（修改，+3：`must(container.Provide(newFeishuPublishHandler))` :1017）

## 1. 实现内容

### 1.1 Produces 接口（与计划签名逐字一致）

- `handler.AppFeishuPublishHandler`（app_connector_feishu_publish.go:23）+ `NewAppFeishuPublishHandler(db)`（:28）+ `SetFeishuPublishService(*publish.NotionPublishService)`（:34，nil 前 fail-closed 501）
- `RequireActionCapabilityForWrites()`（:40）——复用既有 `appRequireWriteCapability(appconnector.CanDriveActionWrites, "FORBIDDEN_ACTION_WRITE", …)`
- 四端点：`FormFeishuPublishPlan`（:83，201 Created）/ `PublishFeishuAction`（:117，冲突 409 码 `FEISHU_PUBLISH_REVISION_CONFLICT` :136）/ `ReconcileFeishuAction`（:144）/ `GetFeishuPublication`（:166）
- `router.RegisterAppFeishuPublishRoutes(r, h)`（routes_app_feishu_publish.go:12，nil handler 短路；与 Notion 路由同样刻意不入 API-key 路由授权表）
- `container.newFeishuPublishHandler(...)`（feishu_publish.go:19）——9 参数与 `newNotionPublishHandler`（notion_publish.go:28-38）逐一相同；装配链：`NewPublicationStore` → `NewDBNotionScopeSource`（单例复用传给 bridge 与 profile）→ `NewFeishuBridge(NewConstantFeishuPolicyProvider, NewCredentialTokenSource(NewCredentialResolver(creds)), pubs)` → `NewActionService(store, guard, gate, bridge, bridge)`（dispatcher 与 unknown resolver 同为 bridge）→ `NewProviderPublishService(…, FeishuProfile(bridge))` → `SetFeishuPublishService`
- wire 面：`POST /api/v1/apps/feishu-publish/{plans,actions/:id/publish,actions/:id/reconcile}` + `GET actions/:id`，组上挂 `RequireActionCapabilityForWrites`

### 1.2 与 Notion 镜像的差异（计划钉定的三处）

① 路由前缀/文案 `feishu-publish`/`the Feishu publish pipeline`；② 冲突码 `FEISHU_PUBLISH_REVISION_CONFLICT`（Notion 为 `PUBLISH_VERSION_CONFLICT`），消息用 "current revision"（Notion 为 "current version"）；③ `failPublish` 中 `ErrPublishUpdateTargetNotPublished` 文案 "only **documents** … may be updated"（Notion 为 "only pages …"）。其余谓词（`appTenantScope` 租户/角色/用户解析、personal-connection 属主判定 `ConnectionKindPersonal && OwnerID != userID` → 403 NOT_CONNECTION_OWNER、参数化租户查询 `Where("tenant_id = ? AND id = ?", …)`、错误→HTTP 码映射表）逐点镜像。

## 2. TDD 证据

### TDD RED（Step 2，实现前）

```
$ go test ./internal/handler/ -run 'TestFeishuPublish' -count=1
# github.com/Tencent/WeKnora/internal/handler [github.com/Tencent/WeKnora/internal/handler.test]
internal/handler/app_connector_feishu_publish_test.go:20:62: undefined: AppFeishuPublishHandler
internal/handler/app_connector_feishu_publish_test.go:35:7: undefined: NewAppFeishuPublishHandler
FAIL	github.com/Tencent/WeKnora/internal/handler [build failed]
FAIL
```

失败原因即预期缺口：handler 尚不存在。

### TDD GREEN（Step 4，实现后）

```
$ go test ./internal/handler/ -run 'TestFeishuPublish' -count=1 -v
=== RUN   TestFeishuPublishPlanGates
…（gorm record-not-found 日志为 conn-nope/跨租户 404 断言的预期路径）…
--- PASS: TestFeishuPublishPlanGates (0.01s)
=== RUN   TestFeishuPublishActionLookupIsTenantScoped
--- PASS: TestFeishuPublishActionLookupIsTenantScoped (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	3.408s
```

钉住的谓词（与 Notion 门测试同构）：未接线服务 501 `PUBLISH_PIPELINE_NOT_CONFIGURED`；viewer 角色 403；他人 personal 连接 403 `NOT_CONNECTION_OWNER`；双目标形状 400 `INVALID_REQUEST`；未知连接 404；跨租户 action 查询 404（publish 与 GET 两路）。

### 全链编译与回归门（均已实际运行）

```
$ go build ./...
BUILD_OK（exit 0；仅 cmd/server、cmd/desktop 的 ld duplicate-libraries 链接警告，Task 3 报告已记录的既有现象，与本改动无关）

$ go test ./internal/handler/ -run 'TestNotionPublishPlanGates|TestNotionPublishActionLookupIsTenantScoped' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	1.711s

$ go test ./internal/modules/appconnector/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	1.760s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	1.717s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	1.345s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	1.897s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	3.611s

$ go vet ./internal/handler/ ./internal/router/ ./internal/container/
VET_OK
```

## 3. 自检发现

- **范围合规**：`git status --short` 核验改动仅 6 个授权文件；对 #48 已集成文件（router.go/container.go）的修改恰为计划「并行批次约束」声明的 +1 参数字段 +1 Register 行 +1 Provide 行（另有 4 行说明性注释，零逻辑）。未触碰他人文件，未撤销任何人的修改。
- **scopeSrc 单例**：`newFeishuPublishHandler` 中 `scopeSrc := publish.NewDBNotionScopeSource(db)` 构造一次、同时传给 bridge 与 `NewProviderPublishService`——计划样例即如此（feishu_publish.go:29），避免重复构造。
- **已知遗留（非本任务范围，如实声明）**：本 worktree 中计划 Task 0（迁移撞号重编）**尚未执行**——`ls migrations/sqlite/` 实测 `000114_mobile_device_app.*` 与 `000114_public_agent_marketplace.*` 双占、versioned `000193_*` 双占；复跑 `go test ./internal/handler/ -run 'TestAppPublicationsTableExistsAfterMigrations' -count=1` 实测 FAIL。本任务两个门测试走 `AutoMigrate`、不经全量迁移装载，不受影响；Task 5 E2E 前必须先落 Task 0。
- **E2E 管线不在本任务钉**：发布闭环本体（FormPlan/Execute/Reconcile/Receipt 全链）由服务层测试（Task 3 provider_test.go）与 Task 5 全迁移 E2E 验证；本任务按计划只钉 HTTP 谓词。

---

# Task 5: E2E 端到端证据（最高稳定 Interface）

- **状态**: DONE（含两轮主控裁决的授权越界处置，均有 ruling 行在案，详见「裁决记录」）
- **提交**:
  - `893e316c7` fix(migrations): dedupe mobile_device_app to 000118/000197 (wave-level prerequisite, ruling via escalation)
  - `a0e23e2ef` fix(appconnector): feishu update Query falls back to approved snapshot document id for unknown reconciliation (ruling via escalation)
  - `4c4c22a9b` test(feishu-publish): 全迁移 E2E——创建/审批/发布/回执、版本冲突 409 零写、unknown→核对零重发、只读 scope fail closed（AC1/AC2/AC3）
- **分支**: `codex/issue30-t49`（worktree `.worktrees/issue30-sweep-t49`）
- **变更文件**:
  - 本任务授权文件：`internal/handler/app_connector_feishu_publish_e2e_test.go`（新建，533 行）
  - 裁决授权越界 1（Task 0 代执行）：`migrations/sqlite/000114_mobile_device_app.{up,down}.sql → 000118_*`、`migrations/versioned/000193_mobile_device_app.{up,down}.sql → 000197_*`（git mv，rename 100% 内容零变化）+ 5 个测试文件引用更新（`internal/handler/mobile_device_test.go`、`internal/application/repository/mobile_device_test.go`、`mobile_push_isolation_test.go`、`mobile_device_app_test.go`、`internal/modules/workbench/service/workbench/notification_app_policy_test.go`）
  - 裁决授权越界 2（Task 2 产出缺陷修复）：`internal/modules/appconnector/feishu_docx.go`（Query 核对目标选择，+11/-11 行内最小改动）+ `internal/modules/appconnector/feishu_docx_test.go`（钉子用例）

---

## 1. 实现内容

### 1.1 E2E 测试文件（计划 Task 5 Step 1 逐字落地）

`app_connector_feishu_publish_e2e_test.go`：全量生产迁移 sqlite（`openFeishuPublishE2EDB` 走 `migrations/sqlite` 的 golang-migrate 全量 `Up()`）+ 真实 `ActionService`/`PublicationStore`/`FeishuBridge`/`NewProviderPublishService`/`AppFeishuPublishHandler`/**既有审批端点**（`POST /api/v1/apps/actions/:id/approve`）；唯一替身是飞书 wire 端点 `e2eFeishu`（官方 docx/v1 合同形状：信封 `{code,msg,data}`、数字 `revision_id`、children 携带服务端字段；`dropNextAppend` 先记效果再丢回复的部分成功形状）。复用 #48 同包 helper：`e2ePassGuard`（notion_publish_e2e_test.go:392）、`rawStrings`（:265），零重复定义。

四个测试（AC1/AC2/AC3 全覆盖）：
1. `TestFeishuPublishEndToEndCreateApprovePublishReceipt`——AC3 全链：form（空 baseline，AC1 的 folder 语义）→ 既有端点审批 → publish → 回执（真实 document id + revision）→ `app_publications` 行 `provider='feishu'`/`published` → GET 可查 → 远端 2 段落（title 从不发送）。
2. `TestFeishuPublishEndToEndUpdateRevisionConflict`——AC1：审批后外部 revision 移动 → 409 `FEISHU_PUBLISH_REVISION_CONFLICT` + 零块写入。
3. `TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst`——部分成功核对：丢回复 → unknown → 盲目重发 409 拒绝 → reconcile 读远端核对成功落回执、零重发。
4. `TestFeishuPublishEndToEndReadOnlyScopeCannotPublish`——AC2：只读 scope（`read_docx`/`sync_content`）终局 failed，零 create 零 append。

### 1.2 裁决授权越界 1：Task 0 迁移重编代执行

E2E 硬前置 Task 0（可装载迁移轨道）在分支上不存在且迁移区双占复发（sqlite `000114_mobile_device_app` 与 `000114_public_agent_marketplace` 同号；versioned `000193_*` 同号；`go test ./internal/database/` 实测 FAIL `duplicate migration file: 000114_public_agent_marketplace.down.sql`）。升级裁决授权后代执行：git mv 至空闲目标号 000118/000197（执行时刻 `ls` 实证空闲，重编后占用表与裁决一致：119/198=t51、120/200=t61、121/201=t47、122/202=t62），5 个测试文件引用用 Edit 工具逐处更新（Mimosa hook 拒绝 Bash sed 直写，按其要求改用 Edit，7 处：6 处 sqlite 文件名 + 1 处 versioned 文件名），`grep -rn "000114_mobile_device_app\|000193_mobile_device_app"` 零命中（引用改净）。

### 1.3 裁决授权越界 2：Task 2 产出 Query 缺陷最小修复

迁移轨道恢复后 E2E 实测 3/4 绿，`TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst` FAIL。根因（读 `feishu_docx.go` 实证）：`executeUpdate` 的 `storeProgress` 只在 append 成功后调用（feishu_docx.go:606-622）——update **首批** append 丢回复时 progress 从未落盘；而 `Query` 只认 `progress.DocumentID`，为空即返回 `unknown: feishu_query_unverifiable: no persisted document id`（修复前 :649-652），把可核对的未知（审批快照里就有 `snap.DocumentID`）降级成不可核对的未知，违背 Spec US40 远端核对语义。升级裁决授权后按最小方案修复：Query 中 `documentID := progress.DocumentID`，为空且 `IsFeishuDocUpdateArgs` 时回退用已审批快照的 `snap.DocumentID`（其 destination 恒为 FormPlan 的 `ErrPublishUpdateTargetNotPublished` 门保证的「本连接此前 published 的 document id」——审批锚点、不可伪造）；后续存在性读/children 核对/回执读全部改用 `documentID`。**create 分支不变**（无快照 id 可用，诚实 unknown 语义保持，`TestFeishuDocxAdapterQueryWithoutDocumentIDStaysUnknown` 继续绿）。按裁决要求**未动** `executeUpdate` 的 progress 落盘时序（潜在改进另行裁决）。

## 2. TDD 证据

### RED 1（Query 缺陷的钉子单测，先写失败）

新增 `TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress`（feishu_docx_test.go）：update 首批丢回复（fake 用 Task 2 实现员的 `dropAppendAt=1` 索引形态，非计划草稿的 boolean——实现注释已声明该形态更忠实）→ `Execute` 返回 unknown 且 progress 空（钉住前置）→ `Query` 应经快照 document id 核对成功。

命令：

```
go test ./internal/modules/appconnector/ -run 'TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress' -count=1 -v
```

输出（修复前，失败原因即缺陷本身）：

```
=== RUN   TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress
    feishu_docx_test.go:571:
        	Error Trace:	.../feishu_docx_test.go:571
        	Error:      	Received unexpected error:
        	            	feishu_query_unverifiable: no persisted document id
        	Test:       	TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress
--- FAIL: TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress (0.01s)
FAIL
```

### GREEN 1（修复后同命令）

```
--- PASS: TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress (0.00s)
```

### GREEN 2（全部 Feishu Docx 单测无回归 + vet）

```
$ go test ./internal/modules/appconnector/ -run 'TestFeishuDocxAdapter|TestReadFeishuDocumentVersion' -count=1 -v
--- PASS: TestFeishuDocxAdapterCreateFullLoop (0.00s)
--- PASS: TestFeishuDocxAdapterCreateResumesFromCheckpoint (0.00s)
--- PASS: TestFeishuDocxAdapterUpdateRevisionConflictRefusesBeforeWrite (0.00s)
--- PASS: TestFeishuDocxAdapterUpdateHappyPathAppends (0.00s)
--- PASS: TestFeishuDocxAdapterReadOnlyCapabilityRefused (0.00s)
--- PASS: TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress (0.00s)
--- PASS: TestFeishuDocxAdapterQueryReconcilesByContent (0.00s)
--- PASS: TestFeishuDocxAdapterQueryWithoutDocumentIDStaysUnknown (0.00s)
--- PASS: TestFeishuDocxAdapterQueryFollowsPagination (0.00s)
--- PASS: TestReadFeishuDocumentVersionPreRead (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.481s
$ go vet ./internal/modules/appconnector/
（无输出）
```

## 3. 验收验证（Task 5 各步实际运行记录）

### Step 0（Task 0 代执行的复验门，裁决第 2 步）

```
$ grep -rn "000114_mobile_device_app\|000193_mobile_device_app" --include="*.go" --include="*.sql" . | grep -v node_modules
（无输出，grep-exit=1 = 改净）
$ go test ./internal/database/ -count=1
ok  	github.com/Tencent/WeKnora/internal/database	17.861s
```

（重编前同命令实测 FAIL：`duplicate migration file: 000114_public_agent_marketplace.down.sql`。）

### Step 2（四个 E2E，最终全绿）

```
$ go test ./internal/handler/ -run 'TestFeishuPublishEndToEnd' -count=1 -v
=== RUN   TestFeishuPublishEndToEndCreateApprovePublishReceipt
--- PASS: TestFeishuPublishEndToEndCreateApprovePublishReceipt (2.12s)
=== RUN   TestFeishuPublishEndToEndUpdateRevisionConflict
--- PASS: TestFeishuPublishEndToEndUpdateRevisionConflict (2.98s)
=== RUN   TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst
--- PASS: TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst (2.18s)
=== RUN   TestFeishuPublishEndToEndReadOnlyScopeCannotPublish
--- PASS: TestFeishuPublishEndToEndReadOnlyScopeCannotPublish (1.80s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	12.203s
```

（过程记录：重编前 4/4 FAIL 于 `duplicate migration file`；重编后首轮 3/4 绿——第 3 测 FAIL 即 1.3 节缺陷，修复后 4/4 绿。）

### Step 3（#48 E2E 回归，同包共存门）

```
$ go test ./internal/handler/ -run 'TestNotionPublishEndToEnd|TestAppPublicationsTableExists' -count=1 -v
--- PASS: TestAppPublicationsTableExistsAfterMigrations (1.69s)
--- PASS: TestNotionPublishEndToEndCreateApprovePublishReceipt (2.22s)
--- PASS: TestNotionPublishEndToEndUpdateConflict (2.35s)
--- PASS: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (2.53s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	11.948s
```

### 补充验证（Task 0 重编引用更新涉及的 3 个包）

```
$ go test ./internal/handler/ -run 'TestMobileDevice' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	1.972s
$ go test ./internal/application/repository/ -run 'MobileDevice|MobilePush' -count=1
ok  	github.com/Tencent/WeKnora/internal/application/repository	2.077s
$ go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlindStripsKind|TestHTTPNotificationProviderBlindOmitsKind|TestAppRoutingProviderDispatchesByApp|TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm|TestAppRoutingProviderRevokesOnlyOwnAppRegistration' -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	2.861s
```

## 4. 裁决记录（Ledger ruling 行）

- **Task 5: Ruling — Task 0 波级重编未落本 worktree（E2E 全部卡 duplicate 114），授权代执行（mobile_device_app→000118/000197，单独 commit），沿 t61/t62/t47 既定模式 — 代价：同批第 5 个分支重复执行同内容重编，集成时多副本内容级合并（主控裁决 via escalation）**
- **Task 5: Ruling — E2E 揭示 Task 2 产出 Query 缺陷（update 丢回复时 progress 未落盘即误报 unknown，弃用可用的审批锚点 snap.DocumentID），违反 Spec US40 远端核对语义；授权越界最小修复（Query 回退）+ 钉子单测 — 代价：越权 Task 2 已 review 文件（最小强化不推翻）；executeUpdate 落盘时序的潜在改进未在本轮处理（主控裁决 via escalation）**

## 5. 自检发现

- **范围合规**：三次 commit 各自独立（重编 / Query 修复 / E2E 测试），未混合；除裁决授权的两个越界点外，改动仅本任务授权文件。未撤销任何人的修改（git mv 前迁移区与 HEAD 一致，`git status --short migrations/` 为空）。
- **Mimosa hook 交互**：Bash sed 批量改 5 个测试文件被 hook 拒绝，按其指引改用 Edit 工具逐处替换（7 处），每次替换内容经 PreToolUse 检查通过。
- **计划样例代码与实现的两处偏差（均以实现为准）**：① Task 2 fake 用 `dropAppendAt int` 索引形态（计划草稿 boolean 只能丢首批，实现注释声明测试自身需要丢末批——本任务钉子测试恰需丢**首批**，用 `dropAppendAt=1` 表达）；② `executeUpdate` 读回用 `m.targetURL(...)` 组装而计划草稿为 `m.do(ctx, method, path, body)` 直拼（等价，本任务不触碰）。
- **E2E 的 unique 价值实证**：Query 缺陷在 Task 2 的 9 个单测全绿下漏网（create 的 document id 在 provider 确认时即落盘，单测无 update 丢首批场景）——恰是计划「底层单测不冒充端到端证据」（验收 3）的活例；修复后钉子单测补上该缺口防回归。
- **报告文件沿用共享追加惯例**：本报告追加于既有 Task 2/3/4 报告之后，未覆盖既有内容；报告文件保持未提交（与前三任务一致）。

---

# 修复轮 1/5：Task 6 补做（审查发现：opt-in 真实证据载体缺失）

- **审查发现核实**：成立。修复前三重验证全部命中——`ls internal/modules/appconnector/feishu_publish_real_test.go` → `No such file or directory`；`git log --all --oneline -- internal/modules/appconnector/feishu_publish_real_test.go` → 零输出；`grep -rln "FEISHU_APP_ID" --include="*.go" internal/` → 仅命中 `internal/modules/datasource/connector/feishu/wiki/connector_realapi_test.go` 与 `connector_seed_test.go`（既有 datasource 体系，非发布面）。#48 同构先例 `internal/modules/appconnector/notion_publish_real_test.go` 实测在库（已读其 NOTION_TOKEN 门控形态作参照）。
- **处置**：按计划 Task 6 逐字补做。
- **提交**: `1d1c8ddbb` test(appconnector): 飞书真实发布闭环证据（FEISHU_* 门控，本环境 blocked-env 如实 SKIP）
- **变更文件**（仅 Task 6 授权文件）: `internal/modules/appconnector/feishu_publish_real_test.go`（新建，137 行）

## 1. 实现内容

`TestFeishuRealPublishLoop` + `feishuTenantAccessToken`（计划 Task 6 Step 1 代码逐字落地）：`FEISHU_APP_ID`/`FEISHU_APP_SECRET`/`FEISHU_TEST_FOLDER_TOKEN` 三环境变量门控；凭据齐备时对 `open.feishu.cn` 跑真实闭环——tenant token 交换 → create（回执=真实 document id+revision）→ 同文档 append（预读 revision + 冲突检测 + 写后新 revision 回执）→ 陈旧 revision append 必须在零写下被拒（`ErrFeishuPublishRevisionConflict`）。token 交换走 `http.DefaultClient`（与 datasource feishu connector 同形的 internal auth——计划明文设计，测试专用 opt-in 路径，非生产出站面）。SKIP 文案逐字含「skip is not a pass — T19 real-provider evidence stays blocked-env」。

## 2. 测试证据（实际运行）

### SKIP 确认（计划 Task 6 Step 2——本环境无凭据，SKIP 是该验收项的诚实终态）

```
$ go test ./internal/modules/appconnector/ -run 'TestFeishuRealPublishLoop' -count=1 -v
=== RUN   TestFeishuRealPublishLoop
    feishu_publish_real_test.go:34: feishu real credentials not configured (FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN in artifacts/connector-real/feishu-publish.env); skip is not a pass — T19 real-provider evidence stays blocked-env
--- SKIP: TestFeishuRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	1.199s
```

### 编译/静态检查与全包回归（计划 Task 6 Step 3）

```
$ go vet ./internal/modules/appconnector/
VET_OK（无输出即成功，命令回显补记）
$ go test ./internal/modules/appconnector/ -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.616s
```

（真实 Provider 的正向闭环证据在本环境**未运行、未验证**——无凭据，永不伪造；`FEISHU_*` 就绪环境的 opt-in 运行即产生该证据。）

## 3. 审查「待核实项」的如实回应

1. **真实飞书 API 验收**：修复前连诚实 SKIP 都无法产生（审查属实）；修复后 SKIP 证据已在案（上方输出）。该验收项在本环境的终态是 **SKIP（非 PASS）**——是否在验收台账记「blocked-env 已挂账」由主控定夺；本报告能证明的仅为：载体已入库、门控纪律与 #48 先例同构、SKIP 路径实测可复现。
2. **两轮越界处置的授权链合规性**：无法由我补证——裁决确由主控经 escalation 作出（见上一报告 §4 两轮对话性记录），授权程序本身的核验在主控侧，本分支 diff 只能证技术内容（审查员已核实 git mv similarity 100%、修复 diff 最小 + 钉子单测）。
3. **executeUpdate 落盘时序改进**：按上轮裁决范围明确未动，本轮零改动、无可验证实现（`feishu_docx.go` 本轮零触碰，`git show 1d1c8ddbb --stat` 仅 1 个新测试文件）。
4. **000118/000197 时点声明**：本环境不可核实其它分支的占用时序；可核实部分上轮已留证（执行时刻 `ls` 空闲、重编后 `go test ./internal/database/` ok），本轮未重跑。

## 4. 自检发现

- 范围合规：本轮仅新增 Task 6 授权文件，未触碰任何其它文件（`git status --short` 仅报告文件 untracked）。
- 计划样例与实现的核对：本文件无与实现的偏差点（消费的全部符号 `FeishuAPIHost`/`FeishuDocxAdapter`/`FeishuCapabilityWriteDocx`/`ReadFeishuDocumentVersion`/`ParseFeishuDocReceipt`/`FeishuTextBlocks`/`ErrFeishuPublishRevisionConflict`/`FeishuDocProgress` 均为 Task 1/2 Produces 的逐字签名，vet 通过即类型闭环）。

---

# 复跑取证轮（新会话接手：代码已入库、报告补齐入库）

- **接手时状态核实**（本会话实际运行）：`git log --oneline -1` → `1d1c8ddbb test(appconnector): 飞书真实发布闭环证据（FEISHU_* 门控，本环境 blocked-env 如实 SKIP）`——Task 6 代码提交已在 HEAD；`git status --short` → 仅 `?? docs/plans/issue30-sweep/plans/plan-t49.md-report.md`（本报告文件 untracked，前轮未入库）。即：前轮已完成计划 Step 1/4（写文件+提交），本会话职责是按「检查只有本会话运行过才算通过」重跑全部验证并补齐报告入库。
- **载体与计划逐字对照**（本会话逐行比对 `internal/modules/appconnector/feishu_publish_real_test.go` vs 计划 plan-t49.md Task 6 Step 1 代码块）：三变量门控（`FEISHU_APP_ID`/`FEISHU_APP_SECRET`/`FEISHU_TEST_FOLDER_TOKEN`，:30-35）、真实闭环三步 create→append→陈旧 revision 零写拒绝（:62-102）、`feishuTenantAccessToken` internal 交换（:108-137）、SKIP 文案、import 清单——逐字一致，零偏差。

## 复跑命令与完整输出（本会话实际运行）

### 计划 Task 6 Step 2：SKIP 确认（原命令原样重跑）

```
$ go test ./internal/modules/appconnector/ -run 'TestFeishuRealPublishLoop' -count=1 -v 2>&1 | tail -6
=== RUN   TestFeishuRealPublishLoop
    feishu_publish_real_test.go:34: feishu real credentials not configured (FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN in artifacts/connector-real/feishu-publish.env); skip is not a pass — T19 real-provider evidence stays blocked-env
--- SKIP: TestFeishuRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.502s
```

**SKIP 非 PASS**：blocked-env 验收项的诚实终态不变。

### 计划 Task 6 Step 3：编译与静态检查（原命令原样重跑）

```
$ go vet ./internal/modules/appconnector/ && echo "VET_OK(exit=$?)"
VET_OK(exit=0)
```

（vet 成功即无输出，仅回显 echo。）

### 全包回归（补充证据）

```
$ go test ./internal/modules/appconnector/ -count=1 2>&1 | tail -3
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.530s
```

## TDD 说明（如实）

Task 6 是纯测试载体任务，计划未定义 RED 步骤（Step 2 直接是 SKIP 验证——被消费符号已由 Task 1/2 实现并入库）。RED 等价证据由前轮留档（上方「修复轮」开头：修复前 `ls` 文件不存在、`git log --all` 零命中）；本会话无法在不触碰 git 状态的前提下重放 RED（文件已在 HEAD `1d1c8ddbb`），如实声明、不作伪造。

## 复跑轮结论

- 计划 Task 6 全部 4 个 Step 终态：Step 1 ✅（载体入库 `1d1c8ddbb`）· Step 2 ✅（SKIP 可复现，本会话留证）· Step 3 ✅（vet 零输出，本会话留证）· Step 4 ✅（代码提交在库）+ 本报告入库（收尾）。
- 真实飞书 API 正向闭环证据：本环境**未运行、未验证**——无凭据，永不伪造；`FEISHU_APP_ID`/`FEISHU_APP_SECRET`/`FEISHU_TEST_FOLDER_TOKEN` 就绪环境的 opt-in 运行即产生该证据。

---

# Task 7: 计划级验证收尾（本会话实际运行）

**任务性质（如实）**：Task 7 是纯验证收尾任务——计划 Files 一节明确「无新文件（全量验证 + 收尾提交）」，无新代码故无 TDD RED/GREEN 环节；计划级验证命令（testCommand）本身就是本任务的交付物。以下每条命令均为本会话在 worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t49`（分支 `codex/issue30-t49`）原样运行并粘贴完整输出，无替代、无缩水。

## 1. 验证内容

逐字执行计划「Task 7 Step 1: 全量验证」的 6 条命令 + 「Step 2: 重复提交兜底」的工作树检查，并补一项 SKIP 语义复核（计划 testCommand 括号声明的预期：真实凭据未设置时真实 Provider 测试 SKIP 属预期）。

## 2. 验证命令与完整输出（本会话实际运行）

### 2.1 构建 + 静态检查（计划 Step 1 第 1 条）

```
$ go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && echo "BUILD_VET_OK"
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
BUILD_VET_OK
```

**判定**：build 通过；`go vet` 自身零输出（两行 `ld: warning: ignoring duplicate libraries` 是 macOS 链接器对 `-lc++` 重复链接的告警，出自 `go build` 环节而非 vet，非 vet 发现）。符合 Expected「全部 ok；go vet 无输出」。

### 2.2 全量迁移轨道（计划 Step 1 第 2 条）

```
$ go test ./internal/database/ -count=1 2>&1 | tail -5
ok  	github.com/Tencent/WeKnora/internal/database	60.371s
```

Task 0 重编（sqlite 000118 / versioned 000197）后的全量迁移轨道可装载。

### 2.3 appconnector 全部子包（计划 Step 1 第 3 条）

```
$ go test ./internal/modules/appconnector/... -count=1 2>&1 | tail -10
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	3.657s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	5.248s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.243s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	5.708s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.810s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	7.871s
```

6 个子包全 ok（合同层/Adapter/publish 服务层/仓储/服务/连接控制/开放连接）。

### 2.4 handler 发布面（计划 Step 1 第 4 条，-v 逐项）

```
$ go test ./internal/handler/ -run 'FeishuPublish|NotionPublish|TestAppPublicationsTableExists' -count=1 -v 2>&1 | grep -E "^(=== RUN|--- (PASS|FAIL|SKIP)|PASS|FAIL|ok)" | head -60
=== RUN   TestFeishuPublishEndToEndCreateApprovePublishReceipt
--- PASS: TestFeishuPublishEndToEndCreateApprovePublishReceipt (9.66s)
=== RUN   TestFeishuPublishEndToEndUpdateRevisionConflict
--- PASS: TestFeishuPublishEndToEndUpdateRevisionConflict (10.31s)
=== RUN   TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst
--- PASS: TestFeishuPublishEndToEndUnknownReconcilesRemoteFirst (7.03s)
=== RUN   TestFeishuPublishEndToEndReadOnlyScopeCannotPublish
--- PASS: TestFeishuPublishEndToEndReadOnlyScopeCannotPublish (8.03s)
=== RUN   TestFeishuPublishPlanGates
--- PASS: TestFeishuPublishPlanGates (0.13s)
=== RUN   TestFeishuPublishActionLookupIsTenantScoped
--- PASS: TestFeishuPublishActionLookupIsTenantScoped (0.00s)
=== RUN   TestAppPublicationsTableExistsAfterMigrations
--- PASS: TestAppPublicationsTableExistsAfterMigrations (3.48s)
=== RUN   TestNotionPublishEndToEndCreateApprovePublishReceipt
--- PASS: TestNotionPublishEndToEndCreateApprovePublishReceipt (5.14s)
=== RUN   TestNotionPublishEndToEndUpdateConflict
--- PASS: TestNotionPublishEndToEndUpdateConflict (3.26s)
=== RUN   TestNotionPublishEndToEndUnknownReconcilesRemoteFirst
--- PASS: TestNotionPublishEndToEndUnknownReconcilesRemoteFirst (2.14s)
=== RUN   TestNotionPublishPlanGates
--- PASS: TestNotionPublishPlanGates (0.00s)
=== RUN   TestNotionPublishActionLookupIsTenantScoped
--- PASS: TestNotionPublishActionLookupIsTenantScoped (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/handler	56.787s
```

12 项全 PASS、零 FAIL、零 SKIP：飞书 E2E 4 项（AC1 创建审批发布回执 / 409 版本冲突零写 / unknown→核对零重发 / AC2 只读 fail closed）+ 飞书 HTTP 门 2 项 + #48 迁移表 1 项 + Notion 既有 4 项（零回归）。AC1/AC2/AC3 三条验收的 E2E 证据均在本轮复跑留证。

### 2.5 移动设备迁移引用回归（计划 Step 1 第 5 条）

```
$ go test ./internal/application/repository/ -run 'MobileDevice|MobilePush' -count=1 2>&1 | tail -3
ok  	github.com/Tencent/WeKnora/internal/application/repository	5.217s
```

Task 0 重编波及的 4 个测试文件引用更新后零回归。

### 2.6 workbench 通知策略回归（计划 Step 1 第 6 条）

```
$ go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlindStripsKind|TestHTTPNotificationProviderBlindOmitsKind|TestAppRoutingProviderDispatchesByApp|TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm|TestAppRoutingProviderRevokesOnlyOwnAppRegistration' -count=1 2>&1 | tail -3
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	3.804s
```

Task 0 重编波及的第 5 个测试文件所属包零回归。

### 2.7 重复提交兜底（计划 Step 2）

```
$ git status --short
（无输出——工作树 clean）
```

Task 0–6 全部产出已在 HEAD（`041134224`…`5c8a39d0b` 共 9 个计划提交），无本计划范围内未提交文件，无需兜底提交。

### 2.8 SKIP 语义复核（补充证据，计划 testCommand 括号声明的预期）

```
$ go test ./internal/modules/appconnector/ -run 'TestFeishuRealPublishLoop' -count=1 -v 2>&1 | tail -6
=== RUN   TestFeishuRealPublishLoop
    feishu_publish_real_test.go:34: feishu real credentials not configured (FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN in artifacts/connector-real/feishu-publish.env); skip is not a pass — T19 real-provider evidence stays blocked-env
--- SKIP: TestFeishuRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.124s
$ echo "FEISHU_APP_ID=${FEISHU_APP_ID:-<unset>} FEISHU_APP_SECRET=${FEISHU_APP_SECRET:-<unset>} FEISHU_TEST_FOLDER_TOKEN=${FEISHU_TEST_FOLDER_TOKEN:-<unset>}"
FEISHU_APP_ID=<unset> FEISHU_APP_SECRET=<unset> FEISHU_TEST_FOLDER_TOKEN=<unset>
```

三凭据变量实测均未设置，`TestFeishuRealPublishLoop` 如实 SKIP（skip 消息自带「skip is not a pass」声明）——与计划声明一致，不冒充真实集成证据。

## 3. TDD 说明（如实）

Task 7 无新代码、无新测试文件（计划 Files：「无新文件」），TDD RED/GREEN 不适用于本任务；本任务的「测试」即上述 8 项验证命令本身，全部为本会话实际运行的真实输出。

## 4. 自检发现

1. **`ld: warning: ignoring duplicate libraries: '-lc++'`**：`go build ./...` 环节的 macOS 链接器告警（cmd/server、cmd/desktop 两处），非本计划引入（与本仓库既有构建行为一致），非 vet 发现，不构成失败；如实记录不掩盖。
2. **耗时量级**：`internal/database`（60.4s）与 `internal/handler`（56.8s）为两条重命令（全量迁移装载 + 全迁移 sqlite E2E），单次运行通过，未隐藏任何重跑。
3. **真实 Provider 正向闭环证据**：本环境仍为 blocked-env（三凭据未设置，实测留证于 2.8）——本计划对该验收项的诚实终态不变：opt-in 证据载体已入库（Task 6，`1d1c8ddbb`），凭据就绪环境运行即产生正向证据。
4. **兜底提交**：Step 2 实测工作树 clean，本任务自身无代码提交；唯二入库动作是本报告文件的追加（见下方提交记录）。

## 5. Task 7 结论

计划级验证命令（testCommand）全绿：build ✅ · vet 零输出 ✅ · database ok ✅ · appconnector/... 6 包 ok ✅ · handler 发布面 12 项 PASS ✅ · application/repository ok ✅ · workbench ok ✅ · 工作树 clean 无兜底提交 ✅ · blocked-env SKIP 如实 ✅。

**计划 Task 0–7 全部完成；#49「T19 飞书文档发布端到端闭环」实现收尾。**

---

# 任务 8（第 8/8 次派发）：计划级验证终态复跑留证（本会话实际运行）

**派发状态说明（如实）**：本次派发为「Issue #49 实施计划的第 8/8 个任务」，前置接口声明「Task 7 计划级验证收尾已留证入册，#49 Task 0–7 全部完成」。经 git log（`041134224`…`36828e4a2`）与报告上文核实，计划 Task 0–7 的产出确已全部落库，本派发对应计划最后一个任务（Task 7 计划级验证收尾）的终态确认。因此本会话的工作是：**逐字复跑计划 testCommand 全部 6 条命令 + Step 2 兜底检查，取得本会话第一手证据**（检查只有实际运行过才算数），确认终态仍然全绿，无任何代码改动、无撤销他人修改。

## 1. 本会话复跑的命令与完整输出（worktree `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep-t49`，分支 `codex/issue30-t49`，HEAD `36828e4a2`）

### 1.1 第 1 条：构建 + 静态检查

```
$ go build ./... && go vet ./internal/modules/appconnector/... ./internal/handler/ ./internal/router/ ./internal/container/ && echo "BUILD_VET_OK"
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
BUILD_VET_OK
```

判定：build 通过；`go vet` 自身零输出（两行 `ld: warning` 出自 `go build` 环节的 macOS 链接器，非 vet 发现，与上一轮留证一致）。

### 1.2 第 2 条：全量迁移轨道

```
$ go test ./internal/database/ -count=1
ok  	github.com/Tencent/WeKnora/internal/database	29.941s
```

### 1.3 第 3 条：appconnector 全部子包

```
$ go test ./internal/modules/appconnector/... -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.402s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol	5.286s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector	0.425s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/publish	3.604s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector	0.879s
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector	12.967s
```

### 1.4 第 4 条：handler 发布面

```
$ go test ./internal/handler/ -run 'FeishuPublish|NotionPublish|TestAppPublicationsTableExists' -count=1
ok  	github.com/Tencent/WeKnora/internal/handler	16.953s
```

### 1.5 第 5 条：移动设备迁移引用回归

```
$ go test ./internal/application/repository/ -run 'MobileDevice|MobilePush' -count=1
ok  	github.com/Tencent/WeKnora/internal/application/repository	3.459s
```

### 1.6 第 6 条：workbench 通知策略回归

```
$ go test ./internal/modules/workbench/service/workbench/ -run 'TestPushPayloadPolicyBlindStripsKind|TestHTTPNotificationProviderBlindOmitsKind|TestAppRoutingProviderDispatchesByApp|TestDisabledNotificationProviderPausesDurablyWithoutRetryStorm|TestAppRoutingProviderRevokesOnlyOwnAppRegistration' -count=1
ok  	github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench	1.698s
```

### 1.7 Step 2 兜底：工作树检查

```
$ git status --short
（无输出）
$ git log --oneline -1
36828e4a2 docs(appconnector): T19 Task 7 计划级验证收尾——testCommand 全绿留证与工作树 clean 确认（#49）
```

工作树 clean，无本计划范围内未提交文件，无需兜底提交。

### 1.8 SKIP 语义复核（补充）

```
$ echo "FEISHU_APP_ID=${FEISHU_APP_ID:-<unset>} FEISHU_APP_SECRET=${FEISHU_APP_SECRET:-<unset>} FEISHU_TEST_FOLDER_TOKEN=${FEISHU_TEST_FOLDER_TOKEN:-<unset>}"
FEISHU_APP_ID=<unset> FEISHU_APP_SECRET=<unset> FEISHU_TEST_FOLDER_TOKEN=<unset>
$ go test ./internal/modules/appconnector/ -run 'TestFeishuRealPublishLoop' -count=1 -v 2>&1 | tail -5
=== RUN   TestFeishuRealPublishLoop
    feishu_publish_real_test.go:34: feishu real credentials not configured (FEISHU_APP_ID/FEISHU_APP_SECRET/FEISHU_TEST_FOLDER_TOKEN in artifacts/connector-real/feishu-publish.env); skip is not a pass — T19 real-provider evidence stays blocked-env
--- SKIP: TestFeishuRealPublishLoop (0.00s)
PASS
ok  	github.com/Tencent/WeKnora/internal/modules/appconnector	0.144s
```

三凭据实测未设置，真实 Provider 测试如实 SKIP（计划 testCommand 括号声明的预期行为），不冒充真实集成证据。

## 2. TDD 说明（如实）

本派发对应计划 Task 7（纯验证收尾，计划 Files：「无新文件」），无新代码、无新测试文件，TDD RED/GREEN 不适用；本会话唯一产出是本复跑留证章节（文档），上述 8 项命令输出全部为本会话原样粘贴。

## 3. 自检发现

1. **派发与落库状态的时序差异**：本派发（第 8/8）开始时计划 Task 0–7 已全部提交（HEAD `36828e4a2`），工作树 clean——即 Task 7 的验证与留证已由前一次派发完成。本会话未重复提交任何代码，仅以本会话亲手复跑的 6+2 条命令输出作为终态确认证据，追加本章节并入库。
2. **全绿判定**：6 条命令全部 `ok`/通过，`go vet` 零输出；与上一轮 Task 7 留证结论一致，无回归、无波动。
3. **真实 Provider 正向闭环证据**：本环境仍无凭据（1.8 实测留证），该验收项诚实终态不变——opt-in 载体已入库（`1d1c8ddbb`），凭据就绪环境运行即产生正向证据。
4. **计划复选框未勾选**：plan-t49.md 中全部步骤复选框保持 `- [ ]`（37 处，`grep -c` 实测）——此前 0–7 各任务实现员均未改动计划文件复选框，本会话遵循同一惯例（计划文件是需求事实源，非本任务授权改动文件），完成状态以 git 提交与本报告为证。

## 4. 任务 8 结论

计划 testCommand 六条命令 + Step 2 兜底检查在本会话亲手复跑**全绿**；#49 计划 Task 0–7 全部完成且均已落库，工作树 clean。「T19 飞书文档发布端到端闭环」实现终态确认收尾。
