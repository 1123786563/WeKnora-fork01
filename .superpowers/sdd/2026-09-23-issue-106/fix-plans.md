# 整分支 OCR 第 2 轮修复批次计划（R6 轮，2026-09-24）

输入：整分支 OCR 第 2 轮 5 条有效 findings（ocr2-F1 medium + F2/F3/F5/F6 low）。
原则不变（证据优先、TDD、中文提交标注「整分支 OCR 二轮」、范围仅本 worktree）。
处置前已逐行核对涉案代码：snapshot.go:85/90-93/96 三处原始回显、manifest.go
validateName（文档注释已提 Co 但错误文案未提、Zl/Zp 不在拒绝集）、oauth.go
submitAuthorizeForm 消费临界区复用 L523 的旧 now、jira.go do() 无界 Decode、
同意页/错误页无防框架头——5 条全部属实。

## R6-A（medium·ocr2-F1）BuildVerifiedSnapshot 三处 manifest 侧回显未截断 → 全部改 echoQuoted [T01]

- 根因：上一轮 F3 只修了重复声明一处；同函数 L85（missing 回显 decl.Name）、
  L90-93（digest mismatch 回显 decl.Name 与 decl.InputSchemaDigest）、L96
  （validateDescription 的 where 前缀回显 decl.Name）仍是裸 %q/%s。函数注释
  「exported and must not silently rely on the caller having validated first」
  与 manifest.go「single joined error must stay bounded」书面承诺有界，
  maxVerificationProblems=32 只截条数不截单条长度——直连调用（fetcher_test.go
  已直连）时 ~1MiB 恶意字段经 %q 转义单条即 MiB 级，且 err.Error() 经
  NewBadRequestError 进 400 响应体。
- 文件：internal/modules/plugins/snapshot.go 三处改 `%s`+echoQuoted（64-hex
  合法 digest 恰为 maxEchoRunes=64 不截断；liveDigest 为本地计算恒有界，保持原样）。
- 回归测试：`TestBuildVerifiedSnapshotProblemEchoIsBounded`（fetcher_test.go，
  三个子场景：3000 rune 名 missing / 3000 字符假 digest mismatch / 3000 rune
  名 + 合法 digest + 非法 live description——断言错误有界且不含全文字段）。
- 完成条件：先 RED（三场景至少两处含全文）后 GREEN；`go test ./internal/modules/plugins/` 全绿。

## R6-B（low·ocr2-F2）validateName 文案失实 + Zl/Zp 漏拒 → 文案补全并入拒绝集 [T01]

- 根因：L195 实际拒绝 Cc/Cf/Co 三类，错误文案只写 "control or format"——
  私有区字符（U+E000 等）被拒却报 format（文档注释 L186-187 已写明 Co 拒绝，
  文案与实现/注释均不一致）；Zl/Zp（U+2028/U+2029 行/段分隔符）不在 Cc/Cf/Co
  中可通过对齐检查，在管理端审核界面引入换行布局干扰（评审实测 go1.26.3 验证）。
- 文件：internal/modules/plugins/manifest.go——拒绝条件增
  `unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)`；文案改
  "control, format or private-use characters"；注释同步补 Zl/Zp 一句。
- 回归测试：`TestValidateNameRejectsSeparatorsAndPrivateUse`（manifest_test.go：
  U+2028/U+2029 拒绝（RED：现通过）、U+E000 拒绝且文案含 "private-use"
  （RED：现文案不含））。
- 完成条件：先 RED 后 GREEN；`go test ./internal/modules/plugins/` 全绿。

## R6-C（low·ocr2-F3）state 过期判定与发码 TTL 复用 Myself 前旧时间点 → 消费临界区重取 now [T04]

- 根因：submitAuthorizeForm 的 `now`（oauth.go:523）在 Myself 跨网络调用
  （30s 超时窗口）前后复用：第二临界区 `now.After(consumed.ExpiresAt)` 用回溯
  时间做过期判定（跨 TTL 边界的 state 仍发码）、`ExpiresAt: now.Add(authCodeTTL)`
  回溯签发（实际存活期 = TTL − Myself 耗时，与声明不符）。同文件
  exchangeCode/refresh 均在临界区前即时取时，口径不一致。
- 文件：examples/plugins/jira-todo-mcp/oauth.go 消费临界区内 `now = time.Now()`。
- 回归测试：`TestAuthorizeStateExpiryJudgedAtConsumeTime`（pendingAuthTTL=30ms
  + fake Jira /myself 延时 60ms：POST 时未过期、Myself 返回后已过期 → 必 409
  不发码；RED：现 302 带码）。
- 完成条件：先 RED 后 GREEN；示例包 -race -count=2 ok。

## R6-D（low·ocr2-F5）Jira 响应体解码无大小上限 → LimitReader 封顶 fail-closed [T04]

- 根因：jira.go:137 直接 `json.NewDecoder(resp.Body).Decode(out)`；请求侧
  分页封顶只约束请求参数，响应体积由对端决定——被攻陷/异常 Jira 可在 30s
  超时窗口内推送超大 JSON 流造成解码内存放大（/myself 同路径）。
- 文件：examples/plugins/jira-todo-mcp/jira.go——`var jiraMaxResponseBytes =
  16<<20`（var 供测试改写），do() Decode 前 `io.LimitReader(resp.Body,
  jiraMaxResponseBytes)`；截断导致 unexpected EOF 天然 fail-closed（对齐
  maxManifestBytes/maxFormBodyBytes 做法）。
- 回归测试：`TestJiraResponseBodyBounded`（上限改 256 + fake Jira 回 1KB 合法
  JSON → SearchMyWeek 报解码错误；RED：现解析成功）。
- 完成条件：先 RED 后 GREEN；示例包 -race -count=2 ok。

## R6-E（low·ocr2-F6）凭据同意页/错误页可被 iframe 嵌入 → X-Frame-Options + CSP frame-ancestors [T04]

- 根因：oauth.go 写 HTML 页前仅设 Content-Type（:442 与 Myself 失败分支），
  无 X-Frame-Options / CSP frame-ancestors——凭据录入页可被第三方嵌入做
  视觉诱导（嵌入式钓鱼）。
- 文件：examples/plugins/jira-todo-mcp/oauth.go——抽 `setHTMLPageHeaders(w)`
  helper（Content-Type + `X-Frame-Options: DENY` + `Content-Security-Policy:
  frame-ancestors 'none'`），serveAuthorizeForm 与 Myself 失败分支（401/502
  两页共用一处头设置）改走 helper。
- 回归测试：`TestAuthorizePagesDenyFraming`（GET 表单页与凭据错误 401 页均
  断言两响应头存在；RED：现缺失）。
- 完成条件：先 RED 后 GREEN；示例包 -race -count=2 ok。

## R6 轮完成条件（总）

1. 5 个新回归测试全部先 RED 后 GREEN（记入 T01/T04 账本）；
2. `go test ./internal/modules/plugins/` 与 `go test -race -count=2
   ./examples/plugins/jira-todo-mcp/` 全绿；`go test ./internal/handler/` 邻域
   复跑（F2 文案变化可能影响跨包断言——预检无，仍复跑验证）；
3. 改动文件 gofmt/vet 干净；`go build ./...` exit 0；
4. 账本回写 T01.md（R6-A/R6-B）、T04.md（R6-C/D/E）与本文件完成记录；
5. 按 [T01]/[T04] 两个中文提交落盘，标注「整分支 OCR 二轮」。

### R6 轮完成记录（2026-09-24）

- 上述 1-5 全部满足。RED 形态：R6-A/R6-B 行为 RED（回显含 3000 字符全文 /
  U+2028 通过且文案无 private-use）；R6-D 编译期 RED（jiraMaxResponseBytes
  undefined，实现后一次 GREEN）；R6-C/R6-E 行为 RED（跨 TTL 边界 302 带码 /
  两响应头缺失）。实现中一处小修：jiraMaxResponseBytes 需为 int64
  （io.LimitReader 签名），首次编译报类型不符后改正。
- 门控（本轮真实运行）：`go test ./internal/modules/plugins/` ok（34 PASS）；
  示例包全量 PASS（25 测试）且 `go test -race -count=2` ok；邻域强制复跑
  `go test -count=1 ./internal/handler/ ./internal/application/service/` ok；
  改动文件 gofmt 无输出、vet 通过；`go build ./...` exit 0。
- 提交：见 git log「整分支 OCR 二轮修复」两条 [T01]/[T04]。

---

# 整分支 OCR 第 1 轮修复批次计划（R5 轮，2026-09-24）

输入：整分支 OCR 第 1 轮 5 条有效 findings（F1/F2/F3/F5/F7，编号沿用报告）。
原则不变：证据优先、TDD（RED→GREEN）、中文提交并标注轮次「整分支 OCR 一轮」；
范围仅 worktree `.worktrees/issue106`；不 push/merge/部署/关 Issue/发评论；
不动主 checkout、其他 `.worktrees/*` 与未跟踪的 `jira-todo-mcp` 二进制。
处置前已逐行核对涉案代码（repository/plugin.go、types/interfaces/plugin.go、
handler/dto/plugin.go、handler/plugin.go、service/plugin_service.go、
modules/plugins/snapshot.go 与 echoQuoted、jira-todo-mcp/oauth.go）与清理先例
（service/resource.go:244 `_ = s.repo.DeleteExpiredGrants(...)` best-effort、
repository/resource.go:113 gorm 参数化删除），5 条 findings 全部属实。

## R5-A（medium·F1）plugin_previews 只增不删 → 仓储补过期清理 + 预览成功路径惰性触发 [T02]

- 根因：repository/plugin.go 仅 Create/Get/MarkPreviewConsumed 三方法，
  MarkPreviewConsumed 只置 consumed_at；全仓无任何预览删除调用方；000189 表
  无 DB 级 TTL——被消费行与过期未消费行永久滞留（单行含 tools_snapshot
  JSONB 可达 MB 级）。这正是 T02.md Concerns #5 转交的语义缺口，本轮按 OCR
  指令落地：**过期即删**（expires_at 是已对外发布的 TTL，行是「TTL 绑定的
  临时审阅工件」而非审计凭证——注释语义如此声明）。
- 文件：interfaces/plugin.go（接口 +DeleteExpiredPreviews）、
  repository/plugin.go（gorm `Where("expires_at <= ?", before).Delete` 参数化，
  对齐 resource.go:113 先例）、service/plugin_service.go（CreatePreview 成功后
  `_ = s.pluginRepo.DeleteExpiredPreviews(ctx, time.Now())` best-effort，对齐
  resource.go:244；失败不影响预览主流程，新写入行 ExpiresAt=now+TTL 不受影响）。
- 回归测试：仓储 `TestDeleteExpiredPreviewsDropsOnlyExpiredRows`（过期未消费/
  过期已消费删除、未过期存活）；服务 `TestPreviewSweepsExpiredRowsOnSuccess`
  （成功路径恰触发 1 次、SSRF 拒绝与持久化失败路径 0 次——失败路径不得把
  清理副作用耦合进拒绝语义）。
- 完成条件：两测试先 RED 后 GREEN；`go test ./internal/application/repository/ ./internal/modules/plugins/ ./internal/handler/` 全绿。

## R5-B（medium·F2）types 接口包反向依赖 handler/dto → 接口回归 types 层结构 [T02]

- 根因：interfaces/plugin.go:6 import handler/dto，是该 61 文件接口包与整个
  types 树唯一真实 handler import；接口注释自述是后续切片扩展基点，倒置将被
  复制。评审已核实「循环依赖」说法不成立（types 根不 import interfaces），但
  分层倒置属实，按报告修复：接口返回 types 层结构，PluginHandler 转 DTO。
- 文件：types/plugin.go（新增 `PluginPreviewResult` + `PluginPreviewToolReview`，
  字段语义与现 DTO 审阅面一致）、interfaces/plugin.go（返回
  `*types.PluginPreviewResult`，删 dto import）、service/plugin_service.go
  （组装 types 结果）、handler/plugin.go（新增 result→dto 转换）、
  handler/plugin_test.go（stub 签名随改）。
- 回归测试：既有 handler 测试（响应 JSON 字段不变）+ 既有服务测试（字段名
  兼容）回归通过即为 GREEN；RED 形态为接口签名变更的编译期失败。契约不变的
  证明：`go test ./internal/handler/` 响应体断言原样通过。
- 完成条件：`grep -rn 'internal/handler/dto' internal/types/` 仅剩注释引用
  （mcp.go:355、secret.go:12，非 import）；三个包测试全绿。

## R5-C（low·F3）BuildVerifiedSnapshot 重复声明名裸 %q 回显 → echoQuoted 截断 [T01]

- 根因：snapshot.go:72-74 用 `%q` 原样回显未审核 decl.Name 且立即 return 绕过
  maxVerificationProblems 截断，与函数注释「不得依赖调用方先校验」的不变量
  及 echoQuoted 的动机（manifest.go:72-78）不一致。生产路径有
  ValidateManifest ≤128 runes 前置（现实暴露 ~130 字符），该口子只对未来直接
  调用方存在——一词修复，顺手闭环。
- 文件：modules/plugins/snapshot.go（`%q`→`%s` + echoQuoted）。
- 回归测试：`TestBuildVerifiedSnapshotDuplicateNameEchoIsBounded`（3000 rune
  重复声明名 → 错误文本长度有界，RED：现含全文）。
- 完成条件：测试先 RED 后 GREEN；`go test ./internal/modules/plugins/` 全绿。

## R5-D（medium·F5+F7）OAuth 动态注册 clients 无淘汰 + client_name 无封顶 → TTL 淘汰 + 长度上限 [T04]

- 根因：oauth.go s.clients 全部用法仅容量检查/插入/读取，无 delete；/register
  无认证（gate 仅包 /mcp）——未认证攻击者可在重启前永久填满 4096 上限（此后
  一律 429，register→authorize→token 链路整体不可用）；且 client_name 无上限，
  1MiB 注册体可携 ~1MiB 名 × 4096 ≈ 4GiB 常驻，击穿注释自述聚合界
  （4096×16×2KiB）。白名单不缓解（可注册指向名单内 host 的 client 占满）。
- 文件：jira-todo-mcp/oauth.go（registeredClient +ExpiresAt；var
  `clientRegistrationTTL = 24h` 可测改写；const `maxClientNameBytes = 256`
  并在注册校验段拒绝超长名；sweepExpiredLocked 增 clients 过期淘汰；
  handleRegister 容量检查前先清扫——淘汰即时释放 429 容量；修正 :65 文档
  注释使其与实现一致）、README.md（补 TTL/封顶说明）。
- 回归测试：`TestRegisterBoundsClientName`（300 字节名 → 400；256 → 201；
  RED：现 201 放行）、`TestRegisteredClientsExpiredByTTL`（上限 1、TTL 1ms：
  A 注册 201 → B 注册 429 → 过期后 B 再注册 201（容量已释放，RED：现仍 429）
  → A 的 authorize 400 unknown client_id（RED：现 200））。
- 完成条件：两测试先 RED 后 GREEN；`go test -race -count=2 ./examples/plugins/jira-todo-mcp/` ok。

## R5 轮完成条件（总）

1. 上述 5 条回归测试全部先 RED 后 GREEN（命令与输出记入 T01/T02/T04 账本）；
2. `go test ./internal/application/repository/ ./internal/application/...
   ./internal/modules/plugins/ ./internal/handler/` 与
   `go test -race -count=2 ./examples/plugins/jira-todo-mcp/` 全绿；
3. 改动包 `gofmt -l` / `go vet` 干净；`go build ./...` exit 0；
4. 账本回写：T02.md（F1/F2 处置 + Concerns #5 闭环）、T01.md（F3）、
   T04.md（F5/F7）、README；
5. 按 [T02]/[T01]/[T04] 三个中文提交落盘，标注「整分支 OCR 一轮」。

### R5 轮完成记录（2026-09-24）

- 上述 1-5 全部满足。RED 形态：F7/F3/F1-服务级行为 RED（257 字节名 201 放行 /
  错误文本 3040 字符 / 清理 0 次触发），F5/F1-仓储编译期 RED（clientRegistrationTTL
  undefined / DeleteExpiredPreviews undefined）。GREEN 首轮抓出 F5 一处缺口
  （validateAuthorizationRequest 读 clients 未查过期 → 过期注册仍可 200），已在
  其临界区内补「先清扫再查」，复跑通过。
- 门控（本轮真实运行）：repository -run 三测 PASS；`go test ./internal/modules/plugins/`
  ok（32 PASS）、`./internal/handler/` ok、`./internal/application/...` 全 ok；
  `go test -race -count=2 ./examples/plugins/jira-todo-mcp/` ok（22/22）；
  改动文件 gofmt 无输出、vet 通过；`go build ./...` exit 0。
- 提交：见 git log「整分支 OCR 一轮修复」三条 [T02]/[T01]/[T04]。

---

# 整分支最终评审修复批次计划（R3 轮，2026-09-23）

输入：整分支最终评审 3 条有效 findings（1 high + 2 medium）。原则：证据优先、TDD（RED→GREEN）、
中文提交并标注修复轮次 `[R3]`。范围约束：仅 worktree `.worktrees/issue106`（分支
`codex/issue-106-self-hosted-plugins`）；不 push / merge / 部署 / 关 Issue / 发评论；不动主 checkout
与其他 `.worktrees/*`。

## F2（medium）container 包 -count>=2 测试回归 —— 本轮代码修复

- 根因：`internal/container/plugin_lister_test.go:85-86,139-140,210-211` 三个测试各自
  `utils.SetSSRFWhitelistFromRaw("127.0.0.1")` + `t.Cleanup(utils.ResetSSRFWhitelistForTest)`。
  `ResetSSRFWhitelistForTest`（`internal/utils/security.go:1139-1144`）的语义是「清空白名单 + 回落
  env」，不是「恢复测试前状态」；container 包 TestMain（`internal/container/ssrf_test.go:11-17`）为
  整个测试二进制设置的 `127.0.0.1,::1,localhost` 在 plugin 测试清理时被清空，字母序在其后的
  `engine_factory_opensearch_test.go` 在第二轮（-count>=2）全部失败（`hostname 127.0.0.1 is restricted`）。
- RED（本轮实测，worktree 根）：`go test -race -count=2 ./internal/container/` exit=1，4 个 FAIL：
  3 个 OpenSearch（SSRF restricted，即本 finding）+ 1 个 `TestFileBackedOCTokenSourceIsVersionScoped`
  （`oc_binding_conflict`，与 SSRF 无关；评审已在基线 4bcad69ba 连续 3 次复现同一失败 → 基线固有，本轮不修、如实记录）。
- 修复：`internal/utils/security.go` 新增 `SnapshotSSRFWhitelistForTest()`——快照 `ssrfWhitelistAtomic`
  并返回恢复闭包（ForTest 契约与 Reset 一致：仅测试可用，无生产调用方，不改变任何生产 SSRF 语义）；
  `plugin_lister_test.go` 三处改为「先注册快照恢复清理，再收窄白名单」的 helper。
- 回归测试（TDD 顺序）：
  1. utils 语义锁定测试 `TestSnapshotSSRFWhitelistForTestRestoresPriorState`
     （RED：新函数未定义 → 编译失败；GREEN：恢复后 `::1` 重新放行，证明「恢复」而非「清空」）；
  2. 复现命令回到基线形态：`go test -race -count=2 ./internal/container/` 仅剩上述 1 个基线固有失败；
  3. 邻域复跑：`go test ./internal/modules/plugins/`、`go test ./internal/utils/`；
  4. 全量门控：`go test ./...`（environment.md §5 基线 exit 0 / 0 FAIL，「不新增失败」口径）。
- 完成条件：1、3、4 全绿；2 仅剩基线固有 1 项；`gofmt`/`go vet` 干净。

## F3（medium）Ledger T01.md 滞后于 HEAD —— 本轮文档修复

- 根因：第 4 个代码提交 `7d30d6fe6`（+375/−259：新增 `internal/container/plugin_lister.go`(84) /
  `plugin_lister_test.go`(258)，`fetcher.go` −88 行——`NewMCPEndpointLister` 移出 plugins 包、改为
  `container.NewPluginMCPEndpointLister`，哨兵契约与适配器测试随迁）落地后未回写 `tasks/T01.md`：
  其提交清单止于 3 个代码提交，且「18/18 PASS」的测试计数已过期（HEAD 实测 plugins 15 + container 4
  = 19，分布两个包）。恢复会话按 T01.md 重建会拿到过期的 API 位置与测试分布。
- 修复：T01.md 提交清单补录 `7d30d6fe6`；新增小节记录迁移事实与「plugins 15 + container 4」分布；
  并补记本轮 R3 修复条目（引用本轮提交 SHA），保持 Ledger 与 `git log 4bcad69ba..HEAD` 一致。
- 完成条件：T01.md 提交清单与基线..HEAD 的 T01 代码提交一一对应；测试分布与本轮实测一致。

# 整分支最终评审修复批次计划（R4 轮，2026-09-24）

输入：整分支最终评审 2 条有效 findings（1 high + 1 medium，即下文 F7/F6）。原则与
R3 轮一致：证据优先、TDD（RED→GREEN）、中文提交并标注修复轮次「整分支终评」。
范围约束不变：仅 worktree `.worktrees/issue106`（分支 `codex/issue-106-self-hosted-plugins`）；
不 push / merge / 部署 / 关 Issue / 发评论；不动主 checkout 与其他 `.worktrees/*`；
不提交 worktree 根下未跟踪的 31MB `jira-todo-mcp` 二进制。

## F6（medium）T04 跨轮转交安全 findings 闭环 —— 本轮代码修复 3 项 + 钓鱼链路缓解 + 外部语义核实 1 项

本轮已逐行复核 oauth.go / plugin_service.go / handler/plugin.go 现行代码（HEAD
dfc295b4e），评审所述 5 个子项全部属实（其中第 5 项经外部官方文档核实为误报）。

### F6.1 /authorize 与 /token 表单无 MaxBytesReader（r3-006/r3-007/r4-006/r4-007，四次报告）
- 根因：oauth.go:425（submitAuthorizeForm）与 :499（handleToken）直接
  `r.ParseForm()`，对比 /register（:249）有 1MiB 限制——未认证端点存在无界
  请求体解析面。
- 修复：新增 `maxFormBodyBytes = 1<<20`，两处 ParseForm 前套
  `http.MaxBytesReader(w, r.Body, maxFormBodyBytes)`；超限走既有 400 路径
  （/authorize → "invalid form"；/token → `{"error":"invalid_request"}`）。
- 回归测试：`TestAuthorizeAndTokenFormBodiesBounded`——用**有效 state + 有效凭据**
  POST >1MiB 表单体到 /authorize（RED：现行代码 302 放行）；POST >1MiB 到
  /token 断言 `error == "invalid_request"`（RED：现行走 unsupported_grant_type）。

### F6.2 state 消费 TOCTOU（r2-004/r3-008/r4-009，三次报告）
- 根因：submitAuthorizeForm 在 oauth.go:434-439 查询 pendingAuths 解锁后跨
  ~30s 网络调用（Myself），:468-475 第二次加锁 `delete` 时**不复查存在性**——
  并发同 state 双提交会双发授权码（PKCE 绑定不受影响，属协议卫生缺陷）。
- 修复：把「复查存在 + 过期即清 + 消费 + 发码」合并进单一临界区：消费时
  `_, still := s.pendingAuths[state]`，不存在（已被并发提交消费）→ 409
  "state already consumed"；凭据失败重试语义不变（失败路径不消费）。
- 回归测试：`TestAuthorizeStateConsumedExactlyOnceUnderConcurrency`——fake Jira
  /myself 延时 150ms 保证两请求重叠，同 state 双并发 POST：恰好一个 302 带
  code、一个 409（RED：现行代码两个 302 双发 code）。

### F6.3 同意页钓鱼链路（r2-012/r4-008，T04 账本未裁决）
- 根因：/register 开放注册 + /authorize 信任注册 redirect_uri + 同意页文案
  固定为「授权 WeKnora…」——攻击者注册自身 client 深链诱导成员提交 Jira
  凭据，凭据会话可被攻击者凭自己的 PKCE 兑换为 30 天 refresh Bearer。
- 裁决（记入 rulings.md R2）：教学示例保留开放动态注册（WeKnora 宿主自身经
  /register 注册是示例主流程），以两层缓解闭环——(a) 同意页透明化：渲染
  注册方 client_name（缺失回落 client_id）与授权码跳转目的地 host，全部
  html.EscapeString 转义，附核对警示；(b) 新增可选装配项
  `Options.AllowedRedirectHosts`（env `PLUGIN_ALLOWED_REDIRECT_HOSTS`，缺省
  空=维持现状），设置后 /register 仅接受 host 在名单内的 redirect_uri——
  生产部署启用即切断「任意注册方 + 任意跳转地」组合。README 补部署警示。
- 回归测试：`TestAuthorizeFormShowsClientAndRedirectTarget`（注册名含
  `<script>` 的 client → 表单出现转义后的注册名与 redirect host，RED：现行
  表单无此内容）；`TestRegisterRestrictedToAllowedRedirectHosts`
  （名单内 host 201 / 名单外 400，RED：字段未定义编译失败）。

### F6.4 ErrPreviewPersistFailed 底层 DB 错误泄漏进 500 响应体（r3-001）
- 根因：plugin_service.go:144 `fmt.Errorf("%w: %v", ErrPreviewPersistFailed, err)`
  把底层 DB 错误拼进错误文本，handler/plugin.go:82 `NewInternalServerError(err.Error())`
  原样进 500 响应体（信息泄漏）；底层错误 :143 已有服务端日志。
- 修复：改 `return nil, ErrPreviewPersistFailed`——哨兵文本进响应，细节只留
  服务端日志。所有权归 T02（plugin_service.go），处置记入 tasks/T02.md。
- 回归测试：`TestPreviewPersistFailureDoesNotLeakRepoError`——替身 repo 返回含
  `SECRET-DB-DETAIL` 的错误，断言 ErrorIs 哨兵且 err.Error() 不含该细节
  （RED：现行错误文本含细节）。

### F6.5 Jira 增强搜索响应字段（r3-005，high·bug）—— 经官方文档核实为误报，不改行为
- 本轮已直接核对 Atlassian 官方 API 参考
  （developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/
  #api-rest-api-3-search-jql-post）：POST /rest/api/3/search/jql 响应的
  issues[] 元素中字段值键名为 **`fields`**（无下划线），与现行 jira.go:83-89
  映射一致；OCR 主张的 `_fields` 不成立。处置：不改代码，jira.go 注释补记
  核实结论，裁决记入 rulings.md R3。

## F7（high）范围缺口（B2-B9 零实现/000190 未落地）—— 维持 F1 判定，本轮不修

本轮已在 HEAD dfc295b4e 重新复核：`grep -rn 'plugin_installations|PluginInstallation|
ErrPluginDrift|write_approval' internal/ examples/ --include='*.go' | grep -v _test.go`
→ 0 命中；`ls migrations/versioned | tail -3` → 最新 000189_plugin_previews（无
000190）；`git diff --name-only 4bcad69ba..HEAD | grep -c '^packages/\|^web/'` → 0；
routes_plugins.go 仅 POST /plugins/installations/preview。与 F1 一致：这是
「20 个任务中 17 个未执行」的范围缺口（T01/T02/T04 已完成），不是缺陷修复轮
可承载的补丁；不在本轮硬凑实现或伪造测试证据，FixOutcome.remaining 如实上报，
交回主流程按任务总表继续排期。分支不可按 Issue #106 整体验收通过——维持原判。

## R4 轮完成条件

1. 新增 5 个回归测试全部先 RED 后 GREEN（命令与输出记入 T04/T02 账本）；
2. `go test ./examples/plugins/jira-todo-mcp/`、`go test ./internal/modules/plugins/`、
   `go test ./internal/handler/`、`go test ./internal/application/...` 全绿；
3. `gofmt -l` / `go vet` 对改动包干净；`go build ./...` exit 0；
4. 账本回写：tasks/T04.md（F6.1-F6.3、F6.5 处置明细）、tasks/T02.md（F6.4）、
   rulings.md（R2/R3）、README（新 env 与钓鱼警示）、本文件（本节）；
5. 提交中文、标注「整分支终评」轮次；不提交 jira-todo-mcp 二进制。

### R4 轮完成记录（2026-09-24）

- 上述 1-5 全部满足（RED/GREEN 与门控输出见 tasks/T04.md 终评节、tasks/T02.md 终评节）。
- 提交：`b9567bf4a`（F6.1/F6.2/F6.3/F6.5，5 文件 +345/−33，[T04]）、
  `c66cb097a`（F6.4，2 文件 +48/−1，[T02]）。门控：示例包
  `go test -race -count=2` ok（20/20）；`./internal/modules/plugins/`、
  `./internal/handler/`、`./internal/application/...` 全 ok；`go build ./...`
  exit 0（仅既有 ld 告警）；改动包 gofmt/vet 干净（gofmt 对
  `internal/application/service/query_history_policy_test.go` 的告警为基线
  4bcad69ba 固有、非本轮改动文件，未动）。worktree 根未跟踪的 31MB
  `jira-todo-mcp` 二进制未提交。
- F7 维持 F1 判定：remaining 上报（见 FixOutcome）。

---

## F1（high）分支仅完成 T01，B2-B10 无实现 —— 本轮不修（范围缺口，缺陷修复轮不可承载）

- 定性：这是「20 个任务中 19 个（T02-T20）未执行」的范围缺口，不是缺陷修复轮可用补丁解决的代码缺陷。
  评审证据（11 个验收测试函数 grep 全 0 文件、Ledger tasks/ 仅 T01.md）与
  `docs/plans/2026-09-23-issue-106-trace-matrix.md:10-19`（B1-B10 → Task 映射）本轮已核对属实。
- 处置：不在本轮硬凑实现或伪造测试证据；在 FixOutcome.remaining 如实上报，交回主流程按
  `docs/plans/2026-09-23-issue-106-00-index.md` 任务总表继续排期（B1 产品面 T02/T03 优先，
  其后 B2-B10 对应 T04-T20）。分支当前不可按 Issue #106 验收通过——该结论维持评审原判。
- 完成条件（本轮）：remaining 明确列出 B1 产品面（T02/T03 未实施）与 B2-B10（T04-T20 未实施）
  无实现与测试证据；不在本轮宣称任何未实施的验收边界已满足。

---

## R5 轮（2026-09-24 整分支最终评审，3 项发现）

### F8（medium）OAuth 令牌表无窗口内总量上限 + lookupSession 持锁 O(n) 全量清扫（T01-R4-F1 转交未闭环）

- 根因：oauth.go 的 `s.tokens` 写路径（exchangeCode/refresh）无容量检查——
  持有任一有效 refresh_token 的调用方可零门槛高频 /token 刷新，每次刷新令
  一个新 access_token 存活 accessTokenTTL（1h），窗口内可把 s.tokens 推至
  任意大；且 lookupSession 持锁调 sweepExpiredLocked 对 5 个 map 做全量
  O(n) 清扫、每个 /mcp 请求经 gate 与 contextFunc 触发两次，表被推大后
  形成请求串行化 CPU/内存放大。与 pendingAuths 已有的同款不变量
  （maxPendingAuths，OCR T04-R2-3）不一致；T01 账本第八轮 T01-R4-F1 已
  转交 T04，至 HEAD 未闭环。
- 修复（examples/plugins/jira-todo-mcp/oauth.go）：
  (a) 新增 `var maxActiveTokens = 8192`（测试可改写）——exchangeCode 与
  refresh 在「先清扫再查容量」的写临界区内检查 `len(s.tokens)`，满则 429
  `temporarily_unavailable`；exchangeCode 满容时不消费 code（客户端可稍后
  重试）、refresh 满容时不消费旧 refresh_token（与「失败不烧毁有效条目」
  语义一致）。refreshTokens 表随 refresh 轮换 1 删 1 增不增长、exchangeCode
  写入受 code+真实 Jira 凭据门槛约束且随 tokens 满容一并被拒——封 s.tokens
  即封死零门槛路径；容量检查与写入跨临界区的并发 TOCTOU 仅允许临时超出
  并发请求数条，稳态上界 = maxActiveTokens + 并发数，非无界。
  (b) lookupSession 改 O(1)：只查目标条目，查到的过期条目即时删除并拒绝
  （T04-R1-2 过期即拒语义不变）；全量清扫职责收敛到写路径（/register、
  /authorize、/token 均仍调 sweepExpiredLocked）。tokens 表加封顶后过期
  条目短暂滞留有界，T04-R2-7「不留孤儿条目」退出路径仍存在。
- 回归测试：`TestActiveTokensBoundedOnExchangeAndRefresh`（maxActiveTokens=2：
  初始授权 + 1 次刷新占满后，第 2 次刷新 429 且旧 refresh_token 未被烧毁、
  新授权流 exchange 也 429；RED：maxActiveTokens 未定义编译失败）；
  `TestLookupSessionDoesNotSweepUnrelatedEntries`（查有效 token 成功且不
  顺带删无关过期条目、查过期 token 自身返回 nil 且该条被清；RED：现行
  lookupSession 全量清扫会删掉无关过期条目，第一断言失败）。

### F9（medium）validateDescription 无条件拒绝全部 Cf——复合 emoji 序列被连带拒绝（可用性回归）

- 根因：manifest.go validateDescription 对自由文本拒绝所有 Cf 类字符，但
  U+200C（ZWNJ）/U+200D（ZWJ）属 Cf 且是合法复合 emoji 序列（家庭组合、
  混肤色握手）与部分正字法（波斯语）的必要组成；清单/live 工具描述由远端
  开发者控制、管理员无法修正，良性插件在安装核验关口被整体拒绝（方向
  fail-closed，非安全洞）。整分支 OCR round-3 medium·bug，未修复。
- 修复（internal/modules/plugins/manifest.go validateDescription）：仅对自由
  文本豁免 `\u200c`/`\u200d` 两枚 rune（round-3 建议 diff），名称
  validateName 仍全量拒绝 Cf/Co/Zl/Zp 不变；注释记录残余权衡「ASCII 字符
  间夹 ZWJ 仍可隐藏文本」——豁免限于这两枚、bidi 覆盖/零宽空格/BOM 等
  其余 Cf 仍拒。manifest 校验与 BuildVerifiedSnapshot 共用该函数，live
  端点数据同步获得豁免，hygiene 面保持一致。
- 回归测试：`TestValidateManifestAcceptsZWJAndZWNJDescription`（描述含
  👨‍👩‍👧‍👦 与波斯语 ZWNJ 词 → 通过；RED：现行实现拒绝）；拒绝面加固：
  TestValidateManifestRejects 追加 description 含 U+200B/U+202E 两条、
  name 含 U+200D 一条（名称不豁免）。

### F10（high）范围缺口：0/10 验收边界完整满足 —— 维持 F1/F7 原判，本轮不修

- 本轮评审证据（11 个验收测试函数 grep 全 0、PluginInstallation/ErrPluginDrift
  生产代码 0 命中、无 000190/sqlite 000111 迁移、packages/ 与 web/ 前缀
  0 改动）与 F1/F7 复核一致，且 Ledger tasks/ 仅 T01/T02/T04 三本账。
- 处置：维持 F1/F7 原判——这是「20 个任务中 17 个未执行」的范围缺口
 （B2-B6、B8、B9 验收测试面零实现；B1 缺 T03、B7 缺 T05/T13、B10 缺
  T06），不是缺陷修复轮可承载的补丁；不在本轮硬凑实现或伪造测试证据，
  FixOutcome.remaining 如实上报，交回主流程按任务总表继续排期。分支不可
  按 Issue #106 整体验收通过——维持原判。

### R5 轮完成条件

1. F8/F9 的 4 项回归测试先 RED 后 GREEN（命令与输出记入本节完成记录）；
2. `go test ./examples/plugins/jira-todo-mcp/`、`go test ./internal/modules/plugins/`
   全绿（含 -count=1 与示例包 -race -count=2）；
3. `go build ./...` exit 0；改动包 gofmt/vet 干净；
4. 提交中文、标注「终评 R5」轮次，按任务分 [T04]/[T01] 两个提交；
5. F10 remaining 如实上报，不宣称任何未实施验收边界已满足。

### R5 轮完成记录（2026-09-24）

- F8：RED 先行——`go test -run 'TestActiveTokensBounded|TestLookupSessionDoesNotSweep'`
  先以 `undefined: maxActiveTokens` 编译失败（RED-1，ocr_fix_test.go:906-908）；
  补 var 声明（不含容量检查）后行为级 RED 双 FAIL（ocr_fix_test.go:928 满容
  刷新仍 200、:972 lookupSession 全量清扫删无关条目）。GREEN：oauth.go 加
  `maxActiveTokens = 8192`（exchangeCode/refresh 写临界区「先清扫再查容量」，
  满则 429 temporarily_unavailable，code/旧 refresh_token 不烧毁）+
  lookupSession 改单条 O(1)（查到的过期条目即时删除并拒，全量清扫收敛到
  写路径）。两测试 PASS。
- F9：RED——`TestValidateManifestAcceptsZWJAndZWNJDescription` FAIL 于
  manifest_test.go:142（现行拒绝复合 emoji 序列）；GREEN：validateDescription
  豁免 U+200C/U+200D（round-3 建议 diff），注释记录残余权衡；拒绝面加固
  4 条 case（description U+200B/U+202E 仍拒、name/工具名含 ZWJ/ZWNJ 仍拒）
  前后均 PASS。
- 门控：`go test -race -count=2 ./examples/plugins/jira-todo-mcp/` → ok
 （3.656s）；`go test -count=1 ./internal/modules/plugins/` → ok；
  `go build ./...` exit 0（仅既有 ld 告警，与 R4 记录一致）；两包
  `gofmt -l` 无输出（manifest_test.go 追加段曾报不齐，已 gofmt -w 后复检
  干净）；`go vet` 两包通过。
- 提交：两个（[T04] oauth 封顶+O(1) 化、[T01] ZWNJ/ZWJ 豁免），见
  FixOutcome.commits；未提交 docs/plans/* 未跟踪文档与 jira-todo-mcp 二进制。
- F10：维持 F1/F7 原判不修，FixOutcome.remaining 如实上报。
- Mimosa 重扫（响应 commit hook scanner_enobufs）：normal 深度、focus 本轮
  4 个改动文件，job scan-job-mueqfk9u completed（seal sha256:8534…79d3，
  全仓 151 findings 为存量）。聚焦文件仅命中 2 条 medium 静态 advisory
  污点（oauth.go:616/618「HTTP 请求输入 → cli/internal/secrets.go:71 文件
  路径操作」）：复核为不可达跨文件误报——`go list -deps ./examples/
  plugins/jira-todo-mcp/` 无 cli/internal/secrets（示例二进制不依赖该包），
  且本轮 diff（45fce2f9f..HEAD）未触碰 query.Set/http.Redirect 行（616/618
  为 submitAuthorizeForm 尾部既有代码）。非本轮引入，不阻断。

---

## R6 轮（2026-09-24 同组终评发现重复投递 —— 复核轮，零新 diff）

主流程再次投递与 R5 完全相同的三项终评发现（范围缺口/oauth 令牌封顶/
ZWJ-ZWNJ 豁免），其 evidence 基于修复前 HEAD。本轮逐项对当前 HEAD
（340906d16）复核，不硬凑新代码：

- F8（oauth）：已由 R5 df3570bdf 修复——oauth.go:74 `var maxActiveTokens
  = 8192`、:662/:714 写临界区容量检查、lookupSession O(1) 化均在位；按名
  重跑 TestActiveTokensBoundedOnExchangeAndRefresh 与
  TestLookupSessionDoesNotSweepUnrelatedEntries → PASS。
- F9（manifest）：已由 R5 340906d16 修复——manifest.go:252
  `\u200c`/`\u200d` 豁免在位；按名重跑 TestValidateManifest* 全组（含
  TestValidateManifestAcceptsZWJAndZWNJDescription）→ PASS。
- F10（范围缺口）：复核证据在当前 HEAD 仍全部成立——11 个验收测试函数
  grep 0 命中、PluginInstallation/ErrPluginDrift 生产代码 0 命中、
  migrations/versioned 最新 000189_plugin_previews（无 000190）、sqlite
  最新 000110（无 000111）、基线..HEAD 的 packages//web/ 前缀 0 改动。
  维持 F1/F7/F10 原判：remaining 上报，不在缺陷修复轮硬凑。
- 门控重跑：`go test -race -count=2 ./examples/plugins/jira-todo-mcp/` →
  ok（3.570s）；`go test -count=1 ./internal/modules/plugins/` → ok
 （0.670s）；两包 gofmt -l 无输出；go vet 通过。
- 本轮无代码改动、无新提交；工作区跟踪文件干净。

---

## R7 轮（2026-09-24 任务级 OCR 分流转交 14 项，统一修复）

来源：无明确归属任务或目标任务未完成的任务级 OCR 转交发现（跨任务修复，
不受任务文件所有权限制）。逐项对当前 HEAD 复核后分四组处置：

### 已闭环复核（零代码）

- T01-R2-F4（lookupSession 持锁全量清扫）：已由 R5 F8 修复（df3570bdf：
  lookupSession 单条 O(1)，全量清扫收敛写路径）——本轮复核在位。
- T01-R4-F1（tokens/refreshTokens/codes 无总量封顶，high）主体：已由 R5
  F8 修复（maxActiveTokens=8192 封 s.tokens，封死「持有效 refresh_token
  高频刷新」零门槛路径）。本轮补充：F19 给 refreshTokens 加
  maxRefreshTokens 封顶完全对齐建议字面（tokens/refreshTokens 双表）；
  codes 豁免论证——写入需消费 pendingAuth state（登记受 maxPendingAuths
  =1024 封顶）且每条都过真实 Jira 凭据验证，TTL 10min，非零门槛 DoS 面，
  满容时随 exchangeCode 一并被 429 拒绝，不为成员级威胁牺牲合法授权体验。

### R7-A internal 服务端（F11-F15）

- F11=T01-R1-F1 scopes 序列化 nil vs []：三处 append([]string(nil), ...)
 （snapshot.go:157 / plugin_service.go:175 / handler/plugin.go:79）在
  Scopes 为 nil 时复制出 nil → JSON `"scopes":null`，与 `[]` 两种形态；
  TS 按 string[] 消费会运行时报错。三处统一 make+copy 保证非 nil。
  回归：TestPreviewManifestHandlerEmitsEmptyScopesArray（nil scopes →
  `"scopes":[]` 非 null；RED：现行输出 null）。
- F12=T01-R2-F2 网络故障误报 400 且回显底层错误：plugins 包新增哨兵
  ErrManifestFetchFailed，fetcher.go 两处「manifest fetch failed」包装改
  %w；handler mapPluginPreviewError 增分支 → 503
 （NewServiceUnavailableError）+ 服务端日志，响应体不回显底层网络错误
 （HTTP_PROXY 下错误文本可携内网代理地址）。确定性输入校验失败仍 400。
  回归：TestPreviewManifestHandlerMapsFetchFailureTo503（classifier 返回
  含 10.0.0.5 代理地址的哨兵包装 → 503 且 body 不含该地址；RED：现行
  400 且回显地址）。
- F13=T01-R2-F3 swagger 缺 @Failure 500：补 500/503 两行（与 F12 后的
  实际状态码面一致）。文档行，无行为测试。
- F14=T01-R3-F1 pluginPreviewTTL 缺上限：新增 maxPluginPreviewTTL=24h，
  可解析超大正值（8760h）钳制到上限，其余回退语义不变。
  回归：TestPluginPreviewTTLCappedAtMax（t.Setenv 8760h→24h、48h→24h、
  1h→1h；RED：8760h 原样返回）。
- F15=T01-R2-F1 DeleteExpiredPreviews 全表扫描：两份建表迁移（000189 与
  sqlite 000110，均为本分支新增未发布，直接补行）加
  CREATE INDEX idx_plugin_previews_expires_at。
  回归：TestPluginPreviewMigrationsIndexExpiresAt（静态断言两文件含索引
  DDL）+ 既有 SQLite 迁移执行测试（craft_test 走 file://migrations/sqlite）。

### R7-B oauth.go（F16-F19）

- F16=T01-R1-F2 隐藏 state 用 %q 渲染：%q 的 Go 字面量转义（\\\\、\\uXXXX）
  浏览器不逆转变 → 含反斜杠/控制字符的 state 表单回传值 ≠ 注册键，
  合法未过期 state 400 中断授权流。改 value="%s"（html.EscapeString 已
  消除逃逸面）。回归：TestAuthorizeFormRoundTripsEscapedState（state 含
  反斜杠：GET 200 → POST 302；RED：现行 POST 400）。
- F17=T01-R2-F5 /register 截断 JSON 部分解码继续处理：err != nil 且
  RedirectURIs==nil 才拒——截断体 redirect_uris 已填充则带不完整元数据
  入库。改为 err != nil 一律 400 invalid_client_metadata，注释同步。
  回归：TestRegisterRejectsTruncatedJSON（截断体 → 400；RED：现行 201）。
- F18=T01-R2-F9 code_challenge 无长度封顶：未认证 GET /authorize 单条
  pendingAuth 可携约 1MiB code_challenge 驻留 10min，击穿
 「1024×~1MiB」名义界。validateAuthorizationRequest 加
  maxCodeChallengeBytes=128 + base64url 字符集校验（RFC 7636 S256 恰为
  43 字符 base64url）。回归：TestAuthorizeBoundsCodeChallenge（129 字符
  →400、含!→400、合法 43 字符→200；RED：前两个现行 200）。
- F19=T01-R4-F1 补充：maxRefreshTokens=4096（与 clients 对齐），
  exchangeCode 写临界区满容 429（code 不消费）；refresh 轮换 1 删 1 增
  不受影响。回归：TestRefreshTokensBoundedOnExchange（=1：首次授权 OK、
  第二次 exchange 429 且 code 保留；RED：maxRefreshTokens 未定义编译失败）。

### R7-C main.go/jira.go（F20-F23）

- F20=T01-R2-F6 Shutdown 超时缺 Close 兜底：streamable HTTP 活跃 SSE 连接
  永不 idle，ctx 取消必等满 5s。Shutdown 返回非 nil 时追加 srv.Close()
 （http.Server 文档约定）。守卫测试 TestRunShutdownReturnsAfterCancel
 （无连接时现行也返回 nil——活跃 SSE 连接的确定性 RED 不可构造，如实
  记录：守卫 + 代码审查修复）。
- F21=T01-R2-F7 BaseURL 回退 0.0.0.0 静默启动：validateBaseURL 增
  IsUnspecified 拒绝（覆盖 BaseURL/JiraBaseURL 两装配面），回退拼出的
  http://0.0.0.0:8020 fail-closed。回归：
  TestValidateBaseURLRejectsUnspecifiedHost（RED：现行通过）。
- F22=T01-R4-F5 validateBaseURL 缺 userinfo 拒绝：u.User != nil 拒绝，
  对齐 manifest/endpoint 两个面已有的 userinfo 卫生。回归：
  TestValidateBaseURLRejectsUserinfo（RED：现行通过）。
- F23=T01-R4-F2 工具输出无封顶：1000 事项 × 255 字符可达数百 KB 进
  CallToolResult。新增 renderIssuesOutput：行数界 200 + 字节界 64KiB
  （UTF-8 边界截断），超限追加与页数截断同风格显式标注；handleSearchMyWeek
  改走该函数。回归：TestRenderIssuesOutputBounded（250 条→行截断标注、
  超长行→字节截断标注；RED：函数未定义编译失败）。

### R7 轮完成条件

1. 上述回归测试先 RED 后 GREEN（F13 文档行与 F20 守卫测试如实说明）；
2. go test ./internal/modules/plugins/ ./internal/handler/ 
   ./examples/plugins/jira-todo-mcp/ 全绿（示例包含 -race -count=2）；
   craft_test 验证 sqlite 迁移可执行；
3. go build ./... exit 0；改动包 gofmt/vet 干净；
4. 提交中文、标注「跨任务转交修复」轮次，按组 3 个提交；
5. 已闭环项（T01-R2-F4/T01-R4-F1 主体）与 codes 豁免论证记入本节。

### R7 轮完成记录（2026-09-24）

- RED→GREEN 全链（命令实跑，输出行号即 FAIL 断言行）：
  A 组：TestPreviewManifestHandlerEmitsEmptyScopesArray RED（plugin_test.go:152
  输出 null）→ GREEN；TestPreviewManifestHandlerMapsFetchFailureTo503 RED
 （plugin_test.go:163 undefined: ErrManifestFetchFailed 编译失败）→ GREEN；
  TestPluginPreviewTTLCappedAtMax RED（plugin_service_test.go:26 undefined:
  maxPluginPreviewTTL）→ GREEN；TestPluginPreviewMigrationsIndexExpiresAt
  RED（plugin_migration_test.go:27 迁移无索引）→ GREEN。
  B 组：TestAuthorizeBoundsCodeChallenge RED（ocr_fix_test.go:1028 超长
  challenge 200）；TestAuthorizeFormRoundTripsEscapedState RED（:1013，
  构造修正：模拟浏览器从表单 HTML 提取属性值 + html.UnescapeString 后
  POST——直接 POST 原值测不出 %q 缺陷）；TestRegisterRejectsPartialDecodes
  RED（:1039，构造修正：json.Decoder 实验证实截断形态不发生部分 unmarshal
 （目标零值、现行已拒），真实放行路径是类型不匹配（client_name:123 →
  UnmarshalTypeError 且已解码字段保留 → 现行 201）；测试以类型不匹配为主
  断言、截断为对照）；TestRefreshTokensBoundedOnExchange RED（:1047
  undefined: maxRefreshTokens）→ 全部 GREEN。
  C 组：TestRenderIssuesOutputBounded RED（ocr_fix_test.go:408 undefined:
  renderIssuesOutput）；TestValidateBaseURLRejectsUnspecifiedHost RED
 （:354）；TestValidateBaseURLRejectsUserinfo RED（:367）→ 全部 GREEN；
  TestRunShutdownReturnsAfterContextCancel 为守卫测试（无连接快速路径现行
  也通过——活跃 SSE 长连接的确定性 RED 不可构造，如实记录：Close 兜底
  修复以代码审查保障，测试防 Run 挂死回归）。
- T01-R2-F5 部分误报记录：OCR 主张的「截断注册体带部分元数据入库」经
  go run 实验证伪（Decoder 对不完整值不做部分 unmarshal → RedirectURIs
  零值 → 现行条件已拒）；但同款不变量缺口在类型不匹配形态真实存在
 （client_name:123 → 201 入库），err!=nil 一律拒绝的修复仍然落地并覆盖
  两形态。
- 门控：go test -race -count=2 ./examples/plugins/jira-todo-mcp/ → ok
 （4.100s）；go test -count=1 ./internal/handler/ ./internal/modules/
  plugins/ ./internal/application/service/ ./internal/application/
  repository/ → 全 ok（service 96.9s、repository 121.6s）；
  go test ./internal/handler/session/ -run TestCraft → ok（SQLite 迁移含
  新索引 DDL 可执行）；go build ./... exit 0（仅既有 ld 告警）；gofmt/vet
  对本轮改动文件干净（gofmt -l 曾报 ocr_fix_test.go 追加段不齐与
  plugin_service_test.go，gofmt -w 后复检干净；其余 gofmt 报告文件经
  git diff 交集比对确认为基线固有、未动）。
- 提交：3 个（A 组 internal 服务端 [T01/T02 归属面]、B/C 组示例 oauth/
  main/jira），均标注「跨任务转交修复」。
- Mimosa 重扫（响应 commit hook scanner_enobufs，3 次提交后一次）：normal
  深度 focus 本轮 7 个改动源文件，job scan-job-muerhj78 completed（seal
  sha256:f664…b62e）。聚焦文件仅命中 2 条 medium 静态 advisory 污点
 （oauth.go:652/654「HTTP 请求输入 → secrets.go:71 文件路径操作」）——
  与 R5 复核的 616/618 为同一代码（submitAuthorizeForm 尾部 query.Set，
  行号随本轮插入偏移）：go list -deps 仍无 cli/internal/secrets（示例
  二进制不依赖该包，污点不可达），本轮 diff 未触碰 query.Set/http.Redirect
  行。维持误报判定，不阻断。其余 149 条为全仓库存量。

---

## R8 轮（2026-09-24 OCR 第 1 轮聚合批次，8 项）

### R8-A internal 服务端（OCR-F3/F4/F1/F7/F6）

- OCR-F3（medium）：manifest.go:154 对 url.Parse 失败已剥离 *url.Error 外壳，
  但内层 uerr.Err 以 %v 无界回显——parseHost 的 `invalid port %q after host`
  嵌入 authority 最后冒号起全部内容（不可信清单控制、可达 ~1MiB），评审
  实测 900145 字节错误文本直达 admin 400 响应体，且 146-148 行注释自述的
  「固定尺寸消息族」论证被证伪。修复：reason 一并 echoQuoted（截断回显），
  注释同步更正。
  回归：TestValidateManifestBoundsParseReasonEcho（endpoint="http://h:"+9000×"a"
  → err 长度 <1KiB 且不含长 a 串；RED：现行 ~9KB）。
- OCR-F4（low）：validateName 消息 "control, format or private-use characters"
  未命名实际拒绝的 Zl/Zp，与 213 行注释「names every rejected class」承诺
  不符。修复：消息补 "line/paragraph separator characters"。
  回归：TestValidateNameRejectsSeparatorsAndPrivateUse 加断言（RED：现行
  消息无 separator 字样）。
- OCR-F1（low）：pluginPreviewTTL 只钳上界，任意 0<d≤24h 原样返回——TTL
  小于 CreatePreview 往返耗时（如 1ms）时刚写入行被同请求
  DeleteExpiredPreviews 删除而响应仍返回 PreviewID（后续 confirm 必 0 行），
  与「误配降级到规范界」只覆盖上界方向不对称。修复：新增
  minPluginPreviewTTL=1min 下限钳制。
  回归：TestPluginPreviewTTLFlooredAtMin（1ms/30s→1min、90s→90s；RED：
  现行原样返回）。
- OCR-F7（low）+ OCR-F6（low）同函数修复：service 门禁
  validatePluginURLLength 之后、ValidateURLForSSRF/FetchAndVerify 之前新增
  纯函数预检 validateManifestURLInput(rawURL)：(a) url.Parse 失败 →
  ErrManifestURLRejected + 固定消息（*url.Error 逐字嵌完整 URL，对
  userinfo+畸形 URL 会把凭据带进 400，绕过 fetcher T01-R3-F2 的不回显
  纪律；评审实测 supersecret 出现在响应体）；(b) u.User != nil → 静默
  拒绝（固定消息不回显 URL）；(c) scheme 非 http/https（含空 scheme，如
  "example.com/manifest.json"——ValidateURLForSSRF 自动补 https 会使门禁
  通过而 fetchLimited 用原始 URL 必失败 → 503 误吞确定性输入错误）→
  400（OCR-F6，godoc @Failure 400 语义对齐）。
  回归：TestValidateManifestURLInputRejectsCredentialBearingAndSchemeless
 （纯函数：userinfo+畸形 → ErrorIs 哨兵且错误不含 URL/凭据；无 scheme →
  哨兵；ftp → 哨兵；合法 https → 通过预检；RED：函数未定义编译失败）。

### R8-B 示例插件（OCR-F8/F9/F5）

- OCR-F8（medium）：validateAuthorizationRequest 在 client_id 查询前持
  s.mu 执行 sweepExpiredLocked 全量清扫（5 个 map 达上限 ≈1.8 万条目），
  该锁同时被 /mcp 每请求两次的 lookupSession 依赖——R5 F8 已消除的 O(n)
  放大被重新挂到未认证 GET /authorize（垃圾 client_id 即触达），且
  serveAuthorizeForm 成功路径写入段再清扫一次（双倍成本）。修复：改
  O(1) 直接过期判定（与 lookupSession:268-271 既有模式一致：只查目标
  client，查到的过期注册即时删除拒绝；全量清扫职责保留在写路径）。
  回归：TestValidateAuthorizationRequestDoesNotSweep（塞过期+有效 clients
  与过期 pendingAuth，查有效 client 成功且无关过期条目不被清扫；RED：
  现行全量清扫删掉无关条目）。
- OCR-F9（medium）：codes 是唯一只有 TTL 没有窗口容量上限的令牌 map——
  本轮评审推翻 R7 的豁免论证（发码即消费 state 释放 pendingAuths 名额，
  1024 封顶不约束 codes 总量；持有有效 Jira 凭据者可循环「登记→发码」
  在 10min TTL 窗口内按吞吐无界堆积，威胁模型与 maxActiveTokens 注释
  同构）。修复：新增 maxActiveCodes=4096，发码临界区「先清扫再查容量」，
  满容不消费 state、429（与 exchangeCode 满容语义一致）。
  回归：TestCodesBoundedOnIssue（=1：首授权 OK；第二授权 POST 凭据 →
  429 且 state 保留可重试；RED：maxActiveCodes 未定义编译失败）。
- OCR-F5（low）：SearchMyWeek 分页循环最多 10 页、每页独立 30s 超时，
  工具调用层无总时限——上游持续慢响应时单次调用最坏 ≈5 分钟占用 /mcp
  连接与 goroutine（WeKnora 默认 30s 会切断，但 AdvancedConfig.Timeout
  可调大）。修复：handleSearchMyWeek 入口
  context.WithTimeout(maxToolCallTimeout=60s) 整体 deadline，var 供测试
  改写。回归：TestSearchMyWeekOverallDeadline（deadline=100ms + fake Jira
  每页 sleep 300ms → 返回错误；RED：现行无整体 deadline 会成功）。

### R8 轮完成条件

1. 8 项回归测试先 RED 后 GREEN（命令与输出记入完成记录）；
2. go test ./internal/modules/plugins/ ./internal/application/service/
   ./internal/handler/ ./examples/plugins/jira-todo-mcp/ 全绿（示例含
   -race -count=2）；go build ./... exit 0；改动包 gofmt/vet 干净；
3. 提交中文、标注「OCR 一轮修复」，R8-A/R8-B 两个提交；
4. R7 的 codes 豁免论证被推翻一事在本节记录修正。

### R8 轮完成记录（2026-09-24）

- RED→GREEN：R8-A：TestValidateNameRejectsSeparatorsAndPrivateUse 新断言
  RED（manifest_test.go:34 消息无 separator）→ GREEN；
  TestValidateManifestBoundsParseReasonEcho RED（:138 错误文本 ~9KB）→
  GREEN（reason 一并 echoQuoted，注释更正「非固定尺寸消息族」）；
  TestValidateManifestURLInputRejectsCredentialBearingAndSchemeless RED
 （plugin_service_test.go:68/:77 undefined: validateManifestURLInput 编译
  失败）→ GREEN；TestPluginPreviewTTLFlooredAtMin 的行为 RED 被同文件编译
  失败遮挡、未单独取得（现行 plugin_service.go 当时无 min 分支为本会话
  读码确认），GREEN 后通过（1ms/30s→1min、90s→90s）。
  R8-B：TestValidateAuthorizationRequestDoesNotSweep RED（ocr_fix_test.go:1215
  现行全量清扫删无关条目）→ GREEN；TestCodesBoundedOnIssue RED（:1241
  undefined: maxActiveCodes 编译失败）→ GREEN（=1：首授权 302、第二授权
  POST 429 且 state 保留重试仍 429）；TestSearchMyWeekOverallDeadline
 （引用 maxToolCallTimeout 编译失败同批）→ GREEN（deadline=100ms +
  fake 每页 300ms → 错误返回；现行无 deadline 时该场景成功——测试内
  注释记明对照逻辑）。
- R7 codes 豁免论证修正：R7 以「state 登记受 maxPendingAuths 封顶传递
  约束 codes」豁免 codes 表——本轮评审指出发码即消费 state 释放名额，
  封顶不约束 codes 总量（吞吐无界），且威胁模型与 maxActiveTokens 注释
  同构（持有有效 Jira 凭据的成员）。论证被推翻，maxActiveCodes=4096 已
  落地（本节 OCR-F9），本记录替代 R7 节的豁免结论。
- 门控：go test -race -count=2 ./examples/plugins/jira-todo-mcp/ → ok
 （4.772s）；go test -count=1 ./internal/modules/plugins/
  ./internal/application/service/ ./internal/handler/ → 全 ok（service
  75.5s）；go build ./... exit 0（仅既有 ld 告警）；gofmt -l 曾报
  ocr_fix_test.go 与 plugin_service_test.go 追加段不齐，gofmt -w 后复检
  干净且测试复跑全绿；go vet 四包通过。
- 提交：2 个（R8-A internal [T01/T02 归属面]、R8-B 示例 [T01/T04 归属面]），
  均标注「OCR 一轮修复」。
- Mimosa 重扫（响应 commit hook scanner_enobufs）：job scan-job-muesrazh
  completed（seal sha256:04d0…d76d）。聚焦 4 文件仅 2 条 medium advisory
  污点（oauth.go:674/676）——与 R5/R7 复核的同一误报面（submitAuthorizeForm
  尾部 query.Set，行号随插入偏移）：go list -deps 仍无 cli/internal/secrets，
  非本轮引入，维持误报判定。其余 149 条全仓库存量。

---

## R9 轮（2026-09-24 OCR 第 2 轮聚合批次，5 项，均示例插件）

- OCR-F1（medium）：http.Server 仅 ReadHeaderTimeout=10s，缺 ReadTimeout
  与 IdleTimeout——/mcp、/register、/authorize、/token 的 1MiB
  MaxBytesReader 只封顶字节数不封顶读取时间，慢速 body 连接可无限期占用
  连接与 goroutine，空闲 keep-alive 也无超时回收。修复：提取
  newHTTPServer(handler)——ReadTimeout 60s + IdleTimeout 120s +
  ReadHeaderTimeout 10s；WriteTimeout 不设（会切断 /mcp SSE 流式响应）。
  Run 改调该构造。回归：TestHTTPServerTimeouts（断言三个超时非零、
  WriteTimeout==0；RED：newHTTPServer 未定义编译失败）。
- OCR-F2（medium）：sweepExpiredLocked 在 5 个未认证写路径请求内全量
  遍历 5 个 map（达上限 21504 条目）且全程持 s.mu——与 /mcp 每请求两次
  的 lookupSession 争抢同一把锁（R5/R8 已修读路径，本条是写路径残留）；
  垃圾 refresh_token 的 POST /token 在校验前即触发。修复：新增
  maybeSweepLocked（sweepMinInterval=1s 节流，var 供测试改写；lastSweep
  由 s.mu 保护），5 个调用点全部替换。语义影响：过期条目的容量释放从
  「即时」弱化为「≤1s」（过期判定本身不受影响——各查找路径都有单条
  过期判定）；唯一依赖即时释放的 TestRegisteredClientsExpiredByTTL 在
  测试内设 sweepMinInterval=0 保持原断言语义。
  回归：TestSweepThrottledOnWritePaths（lastSweep=now + 塞过期条目 →
  窗口内 maybeSweepLocked 跳过清扫、过期条目仍在；窗口过后清扫执行；
  RED：现行无节流、过期条目立即被删）。
- OCR-F6（medium）：同意页请求方标识 client_name 仅 html.EscapeString
  渲染——Unicode Cf 类（U+202E RLO、U+2066-2069、U+FEFF、U+00AD）原样
  穿透，可视觉重排/隐藏 client_name 伪装可信请求方（正是 r2-012/r4-008
  同意页透明化要防的深链钓鱼面）；/register 缺省无认证。修复：注册期
  fail-closed 校验 validateClientName——拒 Cc/Zl/Zp 与 Cf（豁免
  U+200C/U+200D，与 manifest.go validateDescription 的复合 emoji 豁免
  同款双重标准：显示文本豁免两枚、其余 Cf 全拒）；Co 私用区有可见
  字形、按 validateDescription 先例保留。
  回归：TestRegisterRejectsFormatCharactersInClientName（RLO/LRI/BOM/
  soft hyphen/Zl → 400；ZWJ 复合 emoji 名 → 201；RED：现行全部 201）。
- OCR-F3（low）：validateBaseURL 不拒绝 path/query/fragment——
  "https://host#f"/"https://host?q"/"https://host/sub" 拼出的元数据/
  challenge/endpoint 全部不可用却静默启动。修复：新增
  validateServiceBaseURL（在 validateBaseURL 基础上拒绝非空
  path/query/fragment），Run 仅对服务自身 BaseURL 使用；JiraBaseURL
  保留 validateBaseURL（可合法携带路径，Jira DC 常部署 /jira 下）。
  回归：TestValidateServiceBaseURLRejectsPathQueryFragment（三种形态 +
  合法根/带端口通过；RED：函数未定义编译失败）。
- OCR-F4（low）：Shutdown 超时分支 Close 兜底后仍返回 Shutdown 超时
  错误——进程状态已干净却被 log.Fatalf 误判失败（有活跃 SSE 时 ctx 取消
  必走该分支）。修复：提取 shutdownGracefully(srv, timeout)——超时则
  Close 强制回收后返回 nil（宽限关闭与强制回收都算停机完成），Run 调
  shutdownGracefully(srv, 5*time.Second)。
  回归：TestShutdownGracefullyReturnsNilAfterClose（hung handler 占住
  活跃连接 + timeout=100ms → 返回 nil 且挂起请求被 Close 解除；RED：
  现行路径返回 DeadlineExceeded——以提取前行为对照记入账本，提取重构
  本身即修复载体）。

### R9 轮完成条件

1. 5 项回归测试先 RED 后 GREEN（F4 的 RED 形态如实说明）；
2. go test -race -count=2 ./examples/plugins/jira-todo-mcp/ 全绿；
   go test ./internal/modules/plugins/（validateDescription 豁免先例引用
   面）不回归；go build ./... exit 0；示例包 gofmt/vet 干净；
3. 提交中文、标注「OCR 二轮修复」，main.go 三项与 oauth.go 两项分两个
   提交。

### R9 轮完成记录（2026-09-24）

- RED→GREEN：OCR-F1/F3/F4 RED（ocr_fix_test.go:1304 undefined: newHTTPServer
  编译失败，三符号同批）→ GREEN：TestHTTPServerTimeouts（ReadTimeout/
  IdleTimeout/ReadHeaderTimeout 非零、WriteTimeout==0）、
  TestValidateServiceBaseURLRejectsPathQueryFragment、
  TestShutdownGracefullyReturnsNilAfterClose（hung handler + 100ms 宽限 →
  返回 nil 且挂起请求被 Close 解除）全 PASS。OCR-F6 行为 RED（:1365，
  现行 client_name 无字符校验全 201）→ GREEN（RLO/LRI/BOM/软连字符/
  Zl/Cc/LRM 全 400，ZWJ 复合 emoji 名 201）。OCR-F2：节流逻辑与 lastSweep
  字段一步实现，TestSweepThrottledOnWritePaths 未经历独立 RED——替代证据：
  函数级断言（窗口内跳过、窗口后执行）+ 5 个写路径调用点替换前后 grep
 （s.maybeSweepLocked ×5 @430/490/686/761/817）+ 全量 -race -count=2 回归。
  过程插曲（如实）：以脚本做全局替换时误将 maybeSweepLocked 函数体内的
  sweepExpiredLocked 调用一并替换为自身造成自递归，在跑测试前经 grep
  复核发现并改回（未流入任何提交）。
- 节流语义适配：TestRegisteredClientsExpiredByTTL 在测试内设
  sweepMinInterval=0——该测试原意「register 写路径惰性清扫释放容量」，
  节流后其通过依赖的是 authorize 读路径单条过期删除顺带腾名额（非本意），
  显式关掉节流回归原始断言语义。生产语义影响：过期条目的容量释放从
 「即时」弱化为「≤1s」，过期判定本身不受影响（lookupSession/
  validateAuthorizationRequest 均有单条判定）。
- 门控：go test -race -count=2 ./examples/plugins/jira-todo-mcp/ → ok
 （5.169s）；go test -count=1 ./internal/modules/plugins/ → ok（豁免先例
  引用面无回归）；go build ./... exit 0（仅既有 ld 告警）；gofmt -l 曾报
  main.go/ocr_fix_test.go 追加段不齐，gofmt -w 后复检干净且 R9 测试复跑
  绿；go vet 示例包通过。
- 提交：2 个（main.go 服务器生命周期与装配校验 [T04]、oauth.go 清扫
  节流与 client_name 卫生 [T04]），均标注「OCR 二轮修复」。
- Mimosa 重扫（响应 commit hook scanner_enobufs）：job scan-job-mueubh54
  completed（seal sha256:d011…1eeb）。聚焦 2 文件仅 2 条 medium advisory
  污点（oauth.go:720/722）——与 R5/R7/R8 复核同一误报面
 （submitAuthorizeForm 尾部 query.Set，行号随插入偏移）：go list -deps
  仍无 cli/internal/secrets，非本轮引入，维持误报判定。

---

# 范围缺口修复批次计划（R9 轮，2026-09-24）——按主流程指令转入实施

输入：整分支最终评审 critical 范围缺口 finding（B2/B3/B4/B5/B6/B8 零实现、
B1 缺 T03/T06、B7 缺 T13）。前序 F1/F7/F10 三轮均裁决「缺陷修复轮不可承载、
remaining 如实上报」；本轮主流程指令明确要求「按优先级修复（先写计划再 TDD
实施）」，据此转入实施模式，不再维持「本轮不修」。

## 裁定：单轮不可闭合全部缺口，按断点优先级切纵向片

缺口全景 = 17 个未执行任务（T03/T05-T20）。单轮硬凑全部任务必造假，故本轮
只承载评审 finding 点名的第一断点链「预览无法通向安装 → 成员无法发现 →
无法停用」（计划切片 03 的 Task 6/Task 7 后端核心，对应 B2 安装主链路、
B6 停用、B4 发现面、B10 确认重核），其余边界如实 remaining。本轮完成后
B1 的预览→安装后端闭环成立，但 B1（T03/T08 前端 UI）、B2 升级面（T14/T16）、
B3 运行时快照守卫（T09/T17）、B4 个人授权面（T11）、B5 审批闭环（T18/T19）、
B7 对话端到端（T13）仍未实现——不宣称任何验收边界已完整满足。

## R9-A（critical·范围缺口第一断点）plugin_installations 落地 + 确认安装闭环 [T06]

- 根因：计划 20 任务仅执行 T01/T02/T04，无 000190/000111 迁移、无
  PluginInstallation 模型/仓储/服务，preview 只增不消费，安装语义不存在。
- 文件（按计划 03 Task 6 裁决执行）：
  - 新增 `migrations/versioned/000190_plugin_installations.up/down.sql`
    （PG：表 + uq(tenant_id,plugin_id) + service_id 索引 +
    mcp_services 加列 plugin_installation_id；down 反序清派生行、
    保留手工服务）与 `migrations/sqlite/000111_*.up/down.sql` twin
    （JSONB→TEXT、TIMESTAMPTZ→DATETIME、BIGINT→INTEGER）。
  - `internal/types/plugin.go`：PluginInstallation 模型 + 状态常量
    （active|disabled、drift none|detected）+ 结果/摘要 types 层结构
    （维持 F2 裁决：interfaces 不依赖 handler/dto）。
  - `internal/types/mcp.go`：MCPService 追加 PluginInstallationID *string。
  - `internal/types/interfaces/plugin.go` + `internal/application/repository/plugin.go`：
    CreateInstallation/GetInstallation/GetInstallationByTenantPlugin/
    ListInstallationsByTenant/UpdateInstallationState/UpdateInstallationServiceID/
    DeleteInstallation（全部参数绑定）。
  - `internal/application/service/plugin_install_service.go`（方法挂
    pluginService，文件独立）：ConfirmInstallation 补偿式七步
    （取预览→过期判定→重复安装判定→FetchAndVerify 重抓 digest/fingerprint
    复核→写安装行→CreateMCPService 物化→逐工具 SetPolicy 只读 true/写
    false→MarkPreviewConsumed；5/6/7/8 步失败补偿删物化服务+安装行）；
    SetInstallationState（state 白名单 + 同步物化服务 Enabled + UpdatedAt
    刷新）。NewPluginService 追加三个依赖（MCPServiceService/
    MCPServiceRepository/MCPToolApprovalService，容器已注册自动解析）。
  - `internal/handler/dto/plugin.go`/`internal/handler/plugin.go`/
    `internal/router/routes_plugins.go`：POST /plugins/installations
    （Admin）、POST /:id/disable|enable（Admin）；错误映射 404/400/409/503/500。
  - `internal/database/migration_sqlite_versioned_schema_test.go` 登记
    plugin_installations 表与 mcp_services.plugin_installation_id 列。
- 回归测试（先 RED 后 GREEN，`internal/modules/plugins/install_service_test.go`）：
  TestConfirmInstallationRejectsStalePreview（不存在/跨租户/已消费/过期/
  重抓 digest 变化五场景零写入）、TestConfirmInstallationCreatesAndMaterializes
  （安装行/mcp_services 物化行/approvals 只读 enabled 写 disabled/预览消费）、
  TestConfirmInstallationIdempotent（同预览二消费拒绝；同 (tenant,plugin)
  重装拒绝）、TestConfirmInstallationCompensatesOnMaterializeFailure
  （物化失败→补偿后可重新确认）、TestSetInstallationStateSyncsService
  （disable/enable 双向 + 未知 state 拒绝）。
- 完成条件：新测试全绿；`go test ./internal/modules/plugins/`、
  `go test ./internal/handler/`、`go test ./internal/application/...`、
  `go test ./internal/database/ -run TestSQLiteMigrationsCreateVersionedSchema`
  全绿；`go build ./...` exit 0。

## R9-B（critical·范围缺口第二断点）成员发现 API 与跨空间隔离 [T07]

- 根因：无 ListInstallations/GetInstallation，成员侧无任何发现面（B4）。
- 文件：interfaces/plugin.go + repository/plugin.go（ListByTenant/GetByID
  参数绑定 WHERE tenant_id=?）、plugin_install_service.go（ListInstallations
  摘要映射不回传快照原文/GetInstallation 详情含审批 Enabled 合并）、
  dto/handler/routes（GET /plugins/installations 与 GET /:id，Viewer+）。
- 回归测试（先 RED 后 GREEN，`internal/modules/plugins/install_discover_test.go`）：
  TestListInstallationsTenantScoped（两租户互不可见）、
  TestGetInstallationRejectsForeignTenant（跨租户 not found 不泄露存在性）、
  TestManualMCPServiceUnaffectedByInstallations（手工服务列表语义不变）。
- 完成条件：同 R9-A 门控；handler 测试（plugin_install_test.go 追加
  List/Get 200 与 404 映射）全绿。

## R9 轮完成条件（总）

1. R9-A/R9-B 全部新测试先 RED（编译失败或断言失败）后 GREEN；
2. 上述四包测试 + `go build ./...` 全绿；改动文件 gofmt/vet 干净；
3. 中文提交标注「范围缺口 R9」轮次与 [T06]/[T07]；
4. remaining 如实上报：B1 前端（T03/T08）、B2 升级面（T14/T16）、
   B3 运行时守卫（T09/T17）、B4 个人授权（T11）、B5 审批闭环（T18/T19）、
   B7 端到端（T13）、B9 集成迁移测试（blocked-env）仍未实现。

### R9 轮完成记录（2026-09-24）

- R9-A/R9-B 全部落地，TDD 双 RED（服务层 `undefined: types.PluginInstallation`
  编译失败；handler 层 `h.ConfirmInstallation undefined`）→ GREEN。
- 实施中发现并修正两处：①场景 e 需「自洽漂移」（清单声明与 live 同步替换）
  才走到确认侧 digest 复核（只改 live 会被 BuildVerifiedSnapshot 先拒，同为
  零写入拒绝）；②SQLite twin down 需先 DROP INDEX 再 DROP COLUMN（SQLite 拒删
  带索引列，TestExecutionTargetSQLiteFullMigrationDownUp 守卫生效）；
  semantic_migration_test.go 终态 111/Steps(-5)（沿 T02 更新 110/-4 同一惯例）。
- 门控（本轮真实运行，-count=1）：`go test ./internal/modules/plugins/` ok；
  `go test ./internal/handler/` ok；`go test ./internal/application/...` ok
 （repository 164s/service 124s）；`go test ./internal/types/...`、
  `./internal/database/`、`./internal/router/`、`./internal/container/`、
  `./examples/plugins/jira-todo-mcp/` 全 ok；`go build ./...` exit 0；受影响包
  go vet 干净；本轮改动文件 gofmt 无输出（仓库另有 5 个基线既有未格式化文件，
  非本轮引入未触碰）。
- 账本：tasks/T06.md、tasks/T07.md 新建；本节回填。
- 提交：两条中文提交标注「范围缺口 R9」轮次 [T06]/[T07]。
- remaining 如实维持：B1 前端（T03/T08）、B2 升级面（T14/T16）、B3 运行时
  快照守卫与漂移阻断（T09/T17）、B4 个人授权 connections-me（T11）、B5 写工具
  审批闭环（T18/T19）、B7 对话内端到端（T13）、B9 PG 集成迁移测试
 （blocked-env）未实现——分支仍不可按 Issue #106 整体验收通过。

---

# 范围缺口修复批次计划（R10 轮，2026-09-24）——同 finding 重复投递，按优先级续切

输入：与 R9 同一 critical 范围缺口 finding 再次投递（evidence 基于 e050f5e90
旧 HEAD）。当前 HEAD（270dec1da）复核：000190/000111 迁移、PluginInstallation
生产代码、routes_plugins.go 6 条路由均已由 R9 落地——该 evidence 已过时；但
finding 主体（B3/B4/B5/B2 升级面/B1 UI/B7 仍缺）继续成立。本轮按指令续切
下一优先级：**B3 运行时快照守卫与漂移阻断（T09，计划 04）**——安全上最关键
的缺口：现状物化 mcp_services 会把远端当前全部工具暴露给 Agent 目录，
「已接受快照=运行时核验基准」（ID52/US25）不存在。

## R10-A（critical·范围缺口 B3）RegisterMCPTools 快照守卫注入 [T09]

- 根因：无运行时过滤——安装接受的 tools_snapshot 不参与 Agent 目录组装。
- 文件（按计划 04 Task 9）：
  - `internal/modules/agentruntime/agent/tools/mcp_tool.go`：
    `PluginRuntimeSnapshot`/`PluginSnapshotProvider`/`ErrPluginDrift`/
    `FilterToolsBySnapshot`（导出，T17 复用）；`RegisterMCPTools` 第 9 参
    `pluginGuard`（nil=现状逐字节一致）；loader 闭包在 loadMCPDirectory
    返回后过滤：guard 错误 fail-closed；快照外工具剔除；快照内 digest 不符
    → ErrPluginDrift。
  - `internal/types/interfaces/plugin.go` + `internal/application/repository/plugin.go`：
    `GetByServiceID`（参数绑定 WHERE tenant_id=? AND service_id=?）。
  - `internal/application/service/plugin_install_service.go`：
    `PluginSnapshotLookup(repo) tools.PluginSnapshotProvider`（未命中 (nil,nil)
    =非插件服务；命中反序列化快照）。
  - `internal/application/service/agent_service.go`：agentService 增
    pluginRepo 字段（NewAgentService 追加 1 参，dig 自动解析），registerMCPTools
    注入 provider；既有 8 处测试调用点补 nil。
- 回归测试（先 RED 后 GREEN，`tools/mcp_plugin_guard_test.go`，组装先例
  mcp_catalog_integration_test.go）：TestFilterToolsBySnapshot（纯函数三态）、
  TestRegisterMCPToolsFiltersPluginToolsToAcceptedSnapshot（快照外工具目录
  不可见）、TestRegisterMCPToolsRejectsSchemaDrift（digest 不符→ErrPluginDrift
  文案）、TestRegisterMCPToolsNilGuardKeepsLegacyBehavior（nil guard 与
  非插件服务全工具可见）、TestRegisterMCPToolsGuardErrorFailsClosed。
- 完成条件：新测试全绿；`go test ./internal/modules/agentruntime/agent/tools/`
  与 `./internal/modules/airesource/mcp/` 不回归；`go build ./...` exit 0。

## R10 轮完成条件（总）

1. 新测试先 RED（编译失败）后 GREEN；既有 MCP 目录/代理测试零回归；
2. agent 集成面（`./internal/application/service/`）复跑不回归；
3. 中文提交标注「范围缺口 R10」轮次 [T09]；账本 tasks/T09.md + 本节回填；
4. remaining 如实上报：B1 前端（T03/T08）、B2 升级面（T14/T16）、B4 个人
   授权（T11）、B5 审批闭环（T18/T19）、B7 端到端（T10/T13）、B9 PG 集成
   迁移测试仍缺——分支仍不可按 Issue #106 整体验收通过。

### R10 轮完成记录（2026-09-24）

- TDD：RED（`undefined: PluginRuntimeSnapshot` 编译失败）→ GREEN；新测试 6 个
 （tools 层 5 + repo 层 1）全绿。
- 实施中两处修正：①受控工具 schema 改 `sdkmcp.NewToolWithRawSchema` 钉死
  字节（`NewTool+WithRawInputSchema` 会双设冲突运行时报错）；②drift/guard-
  error 集成断言改「控制孪生 + fail-closed 消息」——catalog 既有设计把
  loader 错误折叠为确定性 error 态消息，漂移原文断言留在纯函数测试层，
  孪生排除连接失败假阳性（空洞断言风险已消除）。
- 门控（本轮真实运行，-count=1）：agentruntime 全树 19 包 ok（tools 37s 含
  5 新测试与全部既有目录/代理/曝光测试）；airesource/mcp ok；service ok
 （471s）；repository 单独重跑 ok（437s）；opencode 单独重跑 ok（23s）——
  首轮 ~20 包并行时 repository/opencode 触 10m 默认包级超时假红，单独复跑
  通过，与本轮改动无关（opencode 不导入任何本轮改动包）；container/types/
  handler/modules/plugins ok。`go build ./...` exit 0；改动文件 gofmt 无输出。
- 账本：tasks/T09.md 新建；本节回填。
- 提交：一条中文提交标注「范围缺口 R10」轮次 [T09]。
- Mimosa 重扫（响应 commit hook scanner_enobufs，本轮真实运行）：scan
  `scan-2026-09-24T04-31-53.621Z-01dc9b162ae9` completed（seal
  sha256:f1449e8a…2c9f），151 findings 经 findings.json 定位全部落在基线
  既有文件（app_connector_oauth.go/auth.go/jira-todo-mcp oauth.go 等——
  R8/R9 账本已记录的同一 advisory/污点误报面），本轮 8 个改动文件零命中。
- remaining 如实维持：B1 前端（T03/T08）、B2 升级面（T14/T16）、B4 个人授权
  connections-me（T11）、B5 审批闭环（T18/T19）、B7 端到端（T10/T13）、
  B9 PG 集成迁移测试（blocked-env）、T17 漂移检测持久化闭环（CheckDrift/
  ResolveDrift——本轮 ErrPluginDrift/FilterToolsBySnapshot 已为它备好导出面）
  未实现——分支仍不可按 Issue #106 整体验收通过。

---

# 任务级 OCR 转交 13 项复核轮（R11 轮，2026-09-24）——重复投递，零新 diff

输入：任务级 OCR 分流转交 13 项（T01-R1-F1/F2、T01-R2-F1/F2/F3/F4/F5/F6/
F7/F9、T01-R3-F1、T01-R4-F1/F2/F5）。逐项对当前 HEAD（c566f0473）复核，
全部已由前序轮次（R5 F8/F9、R7 F11-F19 及 85db4b1f8 跨任务转交批）修复
在位——与 R6 轮同型的重复投递，不硬凑新代码：

- T01-R1-F1 scopes [] 形态：plugin.go:85/310 make+copy（32022f0e7）。
- T01-R2-F1 expires_at 索引：两份迁移 000189:24 / sqlite 000110:22。
- T01-R2-F2 抓取故障 503：plugin.go:123 ErrManifestFetchFailed 分支。
- T01-R2-F3 swagger：plugin.go:36-38 @Failure 400/500/503。
- T01-R3-F1 TTL 上限：plugin_service.go maxPluginPreviewTTL=24h 钳制。
- T01-R1-F2 state 往返：oauth.go:532 value="%s"（1e9173815）。
- T01-R2-F4 lookupSession O(1)：oauth.go:299-311 单条查表（R5 F8，R7 复核闭环）。
- T01-R2-F5 register 严格解析：oauth.go:358-365 err!=nil 一律 400。
- T01-R2-F6 Shutdown 兜底：main.go:136 _ = srv.Close()（85db4b1f8）。
- T01-R2-F7 BaseURL 拒非具体 host：main.go:165 IsUnspecified。
- T01-R2-F9 code_challenge 封顶：oauth.go:56/599 maxCodeChallengeBytes=128。
- T01-R4-F1 令牌总量封顶：oauth.go:74-85 maxActiveTokens=8192/
  maxRefreshTokens=4096（R5 F8 + R7 F19）。
- T01-R4-F2 工具输出封顶：jira.go:241-259 maxToolOutputLines/Bytes +
  UTF-8 安全截断 + 显式 ⚠ 标注。
- T01-R4-F5 BaseURL 拒 userinfo：main.go:168 u.User != nil。

## 工作区卫生

按指令删除未跟踪编译产物二进制：examples/plugins/jira-todo-mcp/jira-todo-mcp
（31,096,082 B）与仓库根 jira-todo-mcp（31,079,298 B）——均为历史 go build
残留，未跟踪、未入库，直接删除（不采用 .gitignore 方案：仓库 .gitignore
的 `.*`/`migrations/` 等规则属既有配置，不为一次性产物增行）。未跟踪的
docs/plans/issue-106-*.md 报告文件非二进制产物，不属于本指令范围，保留。

## R11 轮完成条件

1. 13 项逐项复核证据在位（上行清单）；
2. 门控重跑：`go test -count=1 ./internal/modules/plugins/ ./internal/handler/`
   与 `go test -race -count=2 ./examples/plugins/jira-todo-mcp/` 全绿；
3. 工作区无二进制产物；本节回填；中文提交标注「跨任务转交复核」。

### R11 轮完成记录（2026-09-24）

- 13/13 逐项复核通过（证据见上行清单），零新代码 diff——维持 R6 同型
  重复投递处置；二进制产物已删（git status ?? 仅余 7 个 docs/plans
  报告文件，非产物）。
- 门控（本轮真实运行）：`go test -count=1 ./internal/modules/plugins/
  ./internal/handler/` ok；`go test -race -count=2
  ./examples/plugins/jira-todo-mcp/` ok（4.811s）。
- 提交：仅本文件复核记录（中文，标注「跨任务转交复核」）。

---

# OCR 第 1 轮修复批次计划（R12 轮，2026-09-24）——20 项（F01-F04/F06-F21）

输入：OCR 第 1 轮 21 条中 20 条有效发现（无 F05）。原则不变：证据优先、
TDD、中文提交标注「OCR 一轮 R12」、范围仅本 worktree。处置前逐项读码核实
（plugin.go:348-374、plugin_install_service.go 全文、manifest.go:135-172、
snapshot.go:60-90、run-with-node-gte26.mjs/run-gates.mjs 全文、package.json:13-15、
frontend.yml:41、oauth.go:405-425/538-556、main.go:188-208），20 条全部属实。
按域分五批，依赖关系：R12-D（共享节点模块）先行——F11 是 F09/F10 的载体；
R12-B 依赖 R12-C 的守卫语义（F06 的 fail-open 面由 F20 关闭）。

## R12-A（medium×2）安装确认错误映射面 [T06]：F01/F02

- 根因：mapPluginInstallationError default 分支把未映射错误判 400 并回显
  err.Error()（F01：第 7 步 MarkPreviewConsumed 非 NotFound 仓储故障经补偿
  原样上抛→驱动内部文本直达 400 响应体）；且漏列 ErrPreviewPersistFailed
  （F02：第 1 步 GetPreview 仓储故障哨兵落入 default 被判 400）。
- 文件：internal/handler/plugin.go——ErrPreviewPersistFailed 并入 500 case；
  default 改保守 500 + 固定文案（细节仅日志），FetchAndVerify 的确定性
  校验错误仍按既有 4xx 哨兵/IsOAuthProtected 分支先行命中。
- 回归测试（先 RED）：TestConfirmInstallationHandlerUnknownRepoFaultIs500Not400
  （stub 返回非哨兵 DB 错误→500 且 body 不含驱动文本）、
  TestConfirmInstallationHandlerPreviewPersistFailedIs500（RED：现 400）。

## R12-B（high×2+medium×2+low×2）补偿健壮性与状态一致性 [T06]：F14/F06/F16/F15/F17/F21

- 根因：①补偿复用已取消 ctx（F14）且物化服务软删留孤儿策略行（F21）；
  ②残留安装行锁死 uq(tenant,plugin) 无解锁入口（F06b）；③并发确认输家
  收 duplicate-key 原始错误被笼统 500（F15）；④SetInstallationState 两段写
  无回滚产生 disabled 安装+Enabled 服务的不一致（F16）；⑤Step 7 折叠语义
  把跨 TTL 过期误报为已消费（F17）。
- 文件：plugin_install_service.go（补偿改 context.WithoutCancel+独立 15s
  超时；MarkPreviewConsumed NotFound 分支重读 preview 区分消费/过期；
  CreateInstallation duplicate-key 改写 ErrPluginAlreadyInstalled；
  SetInstallationState 改方向序写入——disable 先服务后安装、enable 先安装
  后服务，任一失败面收敛 fail-closed 并尽力回滚安装行）；repository/plugin.go
  （CreateInstallation 识别唯一约束冲突→哨兵 ErrInstallationDuplicateKey；
  新增 HardDeleteServiceCascade 硬删物化服务+按 service_id 清策略行）；
  interfaces/plugin.go + dto/handler/routes（新增 DELETE
  /plugins/installations/:id 卸载入口（Admin）：删物化服务（硬删+策略行）+
  硬删安装行，释放唯一槽——计划 03「本版不提供卸载」的口径由本 finding
  裁定突破，docs 同步注明）。
- 回归测试：TestConfirmInstallationCompensatesWithCancelledContext（fake 记录
  ctx 取消后补偿仍执行）、TestMarkPreviewConsumedExpiredSurfacesExpiry（跨
  TTL→ErrPreviewExpired 非 AlreadyConsumed）、TestCreateInstallationDuplicate
  （fake 返回约束冲突→ErrPluginAlreadyInstalled→409）、
  TestSetInstallationStateFailureStaysFailClosed（服务同步失败→服务保持
  禁用向）、TestUninstallInstallationReleasesUniqueSlot（卸载后可重装）、
  handler 卸载 200/404 映射。

## R12-C（medium×1，high F06 的运行面）守卫 fail-closed [T09]：F20

- 根因：guard 仅以「查到安装行」判定插件物化——窗口期（第 6 步先提交
  Enabled 服务、绑定与策略后写）与孤儿物化服务（补偿删服务失败删安装成功）
  均解析 (nil,nil) 走 manual 未过滤路径，写工具无策略行默认 enabled。
- 文件：mcp_tool.go loader 闭包——snap==nil 且 service.PluginInstallationID
  != nil 时返回 fail-closed 错误（该字段随 CreateMCPService 原子落库，
  足以区分 manual NULL）。
- 回归测试：TestRegisterMCPToolsOrphanPluginServiceFailsClosed（服务带
  PluginInstallationID + guard (nil,nil) → 目录失败；manual 服务不受影响）。

## R12-D（high×2+medium×3+low×2）节点门禁基建 [scripts]：F07/F08/F09/F10/F11/F12/F13

- 根因：wrapper 只覆盖 test:shared（F07）；信号终止 status:null 假绿 +
  spawn 失败静默（F08）；Linux 无候选路径→CI 确定性红（F09，frontend.yml
  NODE_VERSION 24 与 engines >=26 不一致）；shim 目录名可预测可预创建
  （F10，CWE-377/426）；与 run-gates.mjs 逐行重复（F11，双份同步负担）；
  无信号清理（F12）；注释引用未入库文件（F13）。
- 文件：新建 scripts/lib/node-gte26.mjs（majorOf/candidateBin/
  findNodeGte26+PATH 扫描+Linux 候选/mkdtempSync 随机 shim 目录/信号
  清理 helper），run-with-node-gte26.mjs 与 run-gates.mjs 改 import 共享
  （run-gates 的固定 shim 目录一并换 mkdtempSync）；package.json
  test:craft:shared 加 wrapper 前缀；frontend.yml NODE_VERSION "24"→"26"；
  注释改注明 environment.md 仅存在于本地工作区。
- 验证：node 直跑 wrapper（exit 0/spawn 失败/信号杀死三态 exit code 与
  输出）；pnpm run test:craft:shared 真实跑通。

## R12-E（low×4）回显与示例服务卫生 [T01/T04]：F03/F04/F18/F19

- 根因：manifest parse 失败回显可泄 userinfo（F03）；未净化名称重复回显
  未按名去重违契约（F04）；AllowedRedirectHosts 裸主机名匹配任意端口且
  host:port 条目永Miss（F18）；HTML 页缺 Cache-Control: no-store（F19）。
- 文件：manifest.go（回显前脱敏 authority 中 userinfo）；
  snapshot.go（reportedUnvetted 按名去重）；oauth.go（名单匹配改
  parsed.Host，裸 host 条目=任意端口、host:port 条目=精确——注释注明；
  setHTMLPageHeaders 补 Cache-Control: no-store）；main.go（parseHostList
  注释同步，条目形态文档化）。
- 回归测试：TestValidateManifestParseFailureMasksUserinfo、
  TestBuildVerifiedSnapshotDedupesUnvettedNames、
  TestRegisterRedirectHostPortSemantics、TestAuthorizePagesNoStoreHeader。

## R12 轮完成条件（总）

1. 全部新测试先 RED 后 GREEN；受影响包
   （modules/plugins/handler/application/agentruntime tools/examples）复跑全绿；
2. `pnpm run test:craft:shared` 真实跑通；wrapper 三态 exit code 验证；
3. `go build ./...` exit 0；改动文件 gofmt/vet 干净；
4. 中文提交标注「OCR 一轮 R12」，按域分批 [scripts]/[T01]/[T06]/[T09]/[T04]；
5. 本节回填完成记录。

### R12 轮完成记录（2026-09-24，跨任务转交修复轮 2026-09-26 收敛回填）

R12 计划五批次中，本轮（跨任务转交统一修复会话）实际收敛的范围与证据：

**R12-D（scripts/CI）——全部完成**（前轮已写主体代码未提交，本轮验证+补缺口）：
- `scripts/lib/node-gte26.mjs` 抽取共享（F11）：两入口改 import，单一实现；
- Linux/`$PATH` 候选发现（F09）+ `frontend.yml` NODE_VERSION "24"→"26"（CI 与
  engines >=26 对齐）——T01-OCR1-F1 的两面修复；
- 退出码纪律（F08 = T01-OCR1-F2）：`spawnExitCode` 三态（干净透传/信号杀死
  exit 1 + 诊断/spawn 失败 exit 1 + ENOENT 输出），`exitWithSpawnResult` 委托；
- mkdtempSync 随机 shim 目录（F10）+ SIGINT/SIGTERM 清理（F12）；
- `test:craft:shared` 加 wrapper 前缀（F07 = T01-OCR1-F11）；
- 本轮增补：`createNodeShim` symlink EPERM → copyFileSync 兜底 + PATH 拼接改
  `path.delimiter`（T01-OCR1-F12 win32 兼容）；`test:mobile-v2` 同款 wrapper
  前缀（T04-OCR1-F16 尾巴）；新增 `scripts/lib/node-gte26.test.mjs` 十条单测。
- 证据：单测 10/10；wrapper 端到端四态 exit code（0 / 3 透传 / SIGTERM→1+
  诊断 / ENOENT→1+诊断）；`pnpm run test:craft:shared` 真跑 113/113；
  `pnpm run test:mobile-v2` 0 tests exit 0（目录仅 fixtures，与改造前行为一致）；
  frontend.yml NODE_VERSION="26" 就位（宿主无 pyyaml，YAML 深度解析未跑，
  逐行 diff 人工核对）。

**R12-E 之 F03（manifest 回显脱敏）——代码与测试已就位，本轮验证提交**：
`TestValidateManifestParseFailureMasksUserinfo` 真跑 ok。

**R12-A/B/C 及 R12-E 其余项（F01/F02/F04/F06/F14-F21 等）——本轮未实施**：
不在本次跨任务转交清单内（已按任务归属另行分流），状态如实保留为未做。

**本轮另完成的跨任务转交项**（详见对应提交信息）：
- T06-OCR1-F11：`ErrPluginManagedService` 三写面（PUT/DELETE/credentials PUT）
  经 `pluginManagedConflict` 映射 409（新增 handler 测试 4 条，RED→GREEN；
  handler/service/plugins 三包全量回归绿）；
- T07-OCR1-F9：handler `CreateMCPService` 绑定后剥离客户端提供的
  `plugin_installation_id`（物化路径只走插件安装服务内部调用）；
- T08-OCR1-F1 缺口①：`settingsSectionLabel` 对 `integration-plugins` 直译
  兜底（比照 T03 先例；@weknora/i18n 主表无 integrations.tabs.plugins 词条）；
- T08-OCR2-F5：`page.tsx` 新增 `pluginsSlot` view 层插槽 + `tab === 'plugins'`
  渲染分支，`IntegrationsRoutePage` 接线 `PluginsPanel`（面板本体 T08/T12 已
  建成但此前无任何挂载点）；
- 证据：page.test 11/11、SettingsPage.test 26/26、PluginsPanel.test 12/12、
  typecheck:web / typecheck:shared 退出 0、page.tsx 单独 tsc 通过。

**工作区卫生**：`examples/plugins/jira-todo-mcp/jira-todo-mcp` 与仓库根
`jira-todo-mcp` 二进制产物经 `test -f` 确认均不存在，无需清理。

---

# OCR 修复轮次计划（R1 轮，2026-09-26）——33 项（F01-F04/F06-F10/F12-F21/F23-F21 增项/F23-F45 缺号跳过）

输入：OCR R1 轮 33 条有效发现。多项直指上一轮跨任务转交修复自身的缺陷
（F08 swag 注释错位、F12 ReplaceAll 空串乱码、F27-F33 node-gte26 win32 系
列、F25/F26 过期注释）。原则不变：证据优先、TDD、中文提交标注「OCR R1」、
范围仅本 worktree。处置前逐项读码复核。按域分六批，依赖关系：
N1（api-client 方法）先行于 W1 的 F19/F23 面板接入；S1/G1/G2/G3 相互独立。

## 批次 S1（scripts，medium×5+low×1）：F17/F27/F28/F29/F30/F31/F33

- 根因：上一轮 node-gte26 抽库只做了 darwin/linux 正确性，win32 声明了
  目标面（EPERM 兜底、delimiter）却留四处缺口——shim 条目名缺 .exe
  （F27）、PATH/Path 大小写重复键使 shim 静默失效（F30/F33）、spawn 无法
  执行 tsx.cmd（F31）、测试套件自身 win32 false-red（F29）；另有 mkdtempSync
  后的死代码 rmSync（F28）与 test:node-gte26 未接任何门禁（F17）。
- 文件：scripts/lib/node-gte26.mjs（新增导出 shimEnv(shimDir)：大小写无关
  覆盖既有 PATH 变体键；shimNode/win32 node.exe 条目名；$PATH 扫描按平台
  拼 node(.exe)；删 rmSync 死代码）、run-gates.mjs 与 run-with-node-gte26.mjs
  （改用 shimEnv；wrapper win32 spawn 加 shell:true）、run-gates.mjs GATES
  增 test:node-gte26、.github/workflows/frontend.yml 增步、
  node-gte26.test.mjs（平台无关输入 os.tmpdir/path.join 构造、win32 skip
  装饰、新增 shimEnv 测试）。
- 回归测试：node --test scripts/lib/node-gte26.test.mjs（darwin 实跑；win32
  分支以注入 linker/平台无关断言覆盖）；wrapper 四态 exit code 复验。
- 完成条件：单测全绿（含 shimEnv 大小写覆盖）、GATES 首项前含 node-gte26
  自检、frontend.yml 增步、gofmt 无涉。

## 批次 G1（internal/modules/plugins+agentruntime，medium×4+low×1）：F10/F12/F15/F16/F45

- 根因：①manifest parse 失败分支 ReplaceAll 对 userinfoOf 空串（无凭据端
  点）按 Go 语义每 rune 后插 REDACTED 产出乱码（F12，上一轮 F03 修复引入
  的边角 bug）；②validateName 不拒 '/'/'%'，含 '/' 工具名可进快照但策略
  端点 :tool_name 单段匹配永 404，逐工具治理被架空（F16）；③
  DiffLiveAgainstSnapshot 是唯一直接消费未受限 live 目录的路径却无
  ValidateLiveDirectoryForRebase 三门（上限/重名/卫生），恶意端点无界放
  大+未过滤名单持久化回显（F15）；④AcceptUpgrade/PreviewUpgrade 候选端点
  缺 512 rune 长度校验，PG 首写 value too long 误报 500（F45）；⑤插件行
  live instructions 不在快照基线内却透传进模型上下文——与 description 漂
  移同一 prompt-injection 威胁模型（F10）。
- 文件：manifest.go（ReplaceAll 先判空；validateName 拒 '/' 与 '%'——百
  分号一并拒防 %2F 编码绕过）、snapshot.go（DiffLiveAgainstSnapshot 入口
  先跑 ValidateLiveDirectoryForRebase 同款三门，失败返回 nil 让调用方按
  无法产出判定处理——需读两调用方 CheckDrift/markDriftBestEffort 的 nil
  语义并适配）、plugin_install_service.go（AcceptUpgrade/PreviewUpgrade
  指纹守卫后补 validatePluginURLLength 挂 ErrPluginVerifyFailed 映 400）、
  mcp_tool.go（loadPluginDirectory 插件行 snap!=nil 时 instructions 置空
  不透传 live 值）。
- 回归测试：manifest_test.go（无凭据坏端口端点错误文本可读、名含 '/'、
  '%' 拒绝）、snapshot_test.go（超限/重名 live 目录 Diff 返回无法判定态、
  调用方不 panic）、install_service_test.go（超长候选端点 400 非 500）、
  agentruntime tools 包（插件行 instructions 置空断言）。
- 完成条件：各测试先 RED 后 GREEN；modules/plugins、agentruntime 全量绿。

## 批次 G2（MCP 服务/handler，medium×3+low×1）：F06/F07/F08/F09

- 根因：①ClearMCPCredential 缺 UpdateMCPCredentials 已有的插件守卫（凭据
  写面语义不对称，F06）；②GetMCPServiceTools（Viewer+）对插件行返回 live
  目录，绕过快照封堵泄露未接受能力（F07）；③pluginManagedConflict 连注
  释插进 CreateMCPService 的 swaggo 注释块与函数声明之间，swag 按「注释
  组紧邻其后函数」配对把 POST /mcp-services 误绑到辅助函数（F08，上一
  轮引入）；④driftReportResponseDTO 四列表 append([]string(nil),...) 空
  输入得 nil，wire 恒 null 违反「one wire shape」自述（F09）。
- 文件：mcp_service.go 服务层（两处补 PluginInstallationID 守卫返
  ErrPluginManagedService）、handler/mcp_credentials.go（DeleteField 复用
  pluginManagedConflict 映 409）、handler/mcp_service.go（GetMCPServiceTools
  handler 映 409；pluginManagedConflict 连注释整体移到首个 swag 注释块
  之前）、handler/plugin.go（四列表 make+copy 归一化）。
- 回归测试：mcp_plugin_guard_test.go 追加（Clear 插件行 409、tools 插件
  行 409）；plugin handler 测试（drift 响应空列表为 [] 非 null——
  json.Marshal 断言）；swag 位置以源码断言（注解块与 CreateMCPService 声
  明之间无函数声明）。
- 完成条件：新增测试 RED→GREEN；handler/service 包全量绿；go vet 干净。

## 批次 G3（示例服务+测试替身，medium×3+low×3）：F01/F02/F03/F04/F13/F14

- 根因：①jira-todo-mcp WithStateLess(false) 在 mcp-go v0.52.0 是空操作，
  未认证 initialize 无界堆积会话内存（F01）；②validateAllowedRedirectHosts
  漏拒 ":8080" 形态空 host 条目（F02）；③manifest.json 副本缺
  content_digest、防漂移测试只比 InputSchemaDigest（F03）；④handleRegister
  的 Decode 不拒尾随垃圾+1<<20 裸字面量（F04）；⑤fakejira DenySearch 等
  未命中静默返回，装配错误面与 AddAccount 不一致（F13）；⑥plugintest 替
  身把 Tool.Call 失败呈现为 isError 成功结果，与示例服务协议级 error 面
  相反，三场景 e2e 断言验证的是替身特有路径（F14）。
- 文件：examples/plugins/jira-todo-mcp/（main.go WithStateLess(true)+
  空 host 校验+常量复用说明、manifest.json 补 content_digest、oauth.go
  decoder.More()+maxRPCBodyBytes、server_test.go 双 digest 比对）、
  internal/modules/plugins/plugintest/（fakejira.go testing.TB+Fatal、
  server.go 返回协议级 error 并同步修正 jira_e2e_integration_test.go 三
  场景断言——401 场景改为断言授权引导文案替换后的形态）。
- 回归测试：examples 包与 plugintest 相关测试全量；e2e 三场景真实复跑。
- 完成条件：go test ./examples/plugins/jira-todo-mcp/ 与
  ./internal/modules/plugins/ 全绿。

## 批次 N1（api-client，high×2+medium 余项）：F18/F19/F20/F21/F23

- 根因：①parsePluginInstallation service_id 用 required，与后端「confirm
  中断的安装行 service_id 空串合法 200」矛盾，操作已生效面板却报错
  （F18）；②升级闭环前端断裂——后端 upgrade-accept 齐备而 api-client 无
  acceptUpgrade、面板无入口（F19）；③drift 治理端点族（get/check/resolve）
  整体缺失，漂移复审闭环前端不可达（F23）；④disabledReason 静默降级 +
  点号路径风格不一（F20）；⑤enabled 类型 boolean|null 与注释过期（F21）。
- 文件：packages/api-client/src/plugins.ts（service_id 改 optionalText 同
  parsePluginMyConnection 口径；新增 acceptUpgrade/getDrift/checkDrift/
  resolveDrift + PluginDriftReport 解析器（明细四列表与 R1-F09 的 [] 形态
  对齐、空列表归一 []）；disabledReason optionalText 显式化+方括号路径；
  enabled 类型收窄 boolean 改 flag 解析并更正注释）+ plugins.test.ts。
- 回归测试：plugins.test.ts 追加（空 service_id 解析通过、accept/drift
  wire 形态、drift 空列表为 []、enabled 缺 key 抛错口径如契约所定）。
- 完成条件：单测 RED→GREEN；typecheck:shared（含 api-client 入口）绿。

## 批次 N2+W1（views 注释/i18n + apps/web 面板，medium×3+low×6）：F24/F25/F26/F34/F35/F40/F42/F43/F44 + F19/F23 面板接入

- 根因：①四 locale subtitle 缺「写入类工具默认关闭」治理承诺（F24）；
  ②page.tsx/view.ts 三处注释描述已落地的过渡态（F25/F26）；③
  PluginsSettingsPanel client 失效只清 toolPolicy，preview/confirmError/
  upgradePreview/upgradeError 留存，跨账号切换后旧空间预览卡仍可「确认
  安装」（F34）；④PluginsPanel requestedRef 去重集 client 变化不重置，
  旧 principal 连接徽标残留（F42）；⑤retryConnection delete+add 净空死
  代码（F35）；⑥两面板跨行 pending/禁用口径不一致（F40/F43）；⑦
  policyToggleBusy 键值死数据（F44）；⑧升级差异面板接入接受按钮、漂移
  徽标接入 check/resolve 入口（F19/F23 的面板侧，依赖 N1 方法先落）。
- 文件：packages/views/src/integrations/messages.ts（四 locale subtitle 补
  齐表述）、page.tsx/view.ts（注释改述）、apps/web/src/settings/
  PluginsSettingsPanel.tsx（失效 effect 清全部临时态+confirmInstall 成功
  清 upgradePreview+停用/启用跨行 disabled+policyToggleBusy 收窄+
  接受升级按钮+drift check/resolve 入口）、apps/web/src/integrations/
  PluginsPanel.tsx（requestedRef/connections 随 pluginsApi 重置+删净空两
  行+anyPending 禁用）。
- 回归测试：registry.test.ts 追加四 locale subtitle 断言；两面板
  .test.tsx 追加（client 切换清临时态、连接缓存重置、跨行禁用、接受/
  drift 调用发出）——先 RED 后 GREEN。
- 完成条件：面板与 views 测试全绿；typecheck:web/typecheck:shared 退出
  0；page.test/SettingsPage.test/PluginsPanel.test 回归不破。

## R1 轮完成条件（总）

1. 33 项全部处置（修复或如实说明不可行）；各批次新测试先 RED 后 GREEN；
2. 受影响包复跑全绿（modules/plugins、application/service、handler、
   agentruntime tools、examples、api-client/panels/views 测试）；
3. `go build ./...` exit 0、gofmt/vet 干净；typecheck:web/shared 退出 0；
4. 中文提交按域分批标注「OCR R1」；本节回填完成记录。

### R1 轮完成记录（2026-09-26）

33 项全部修复，无剩余项。逐批证据：

**S1（F17/F27/F28/F29/F30/F31/F33）**：node-gte26.mjs 新增 shimEnv（大小
写无关覆盖 PATH 变体键）+ shimEntryName（win32 node.exe）+ $PATH 扫描按
平台 + 删 rmSync 死代码；两入口改用 shimEnv（wrapper win32 spawn 加
shell）；GATES 首项 node-gte26-selftest + frontend.yml 增步；测试平台无关
化（os.tmpdir/path.join 构造、win32 skip）+ 新增 5 测。证据：
`pnpm run test:node-gte26` 13/13；wrapper 四态 exit code 复验（0/透传 3/
SIGTERM→1/ENOENT→1）。

**G1（F10/F12/F15/F16/F45）**：manifest.go ReplaceAll 先判空（F12，无凭据
坏端口端点文本可读）+ validateName 拒 '/'/'%'（F16，快照工具必可经策略端
点寻址）+ ValidateManifest 补 maxEndpointRunes=512（F45 统一入口，覆盖
preview/upgrade-preview/upgrade-accept）；snapshot.go DiffLiveAgainstSnapshot
签名改 (detail, error) 入口先过 ValidateLiveDirectoryForRebase 三门
（F15），CheckDrift 映 ErrPluginVerifyFailed 400、markDriftBestEffort 静默
不落未审名单；mcp_tool.go 插件行（snap!=nil）instructions 置空（F10）。
证据：manifest 三测（乱码可读/'/' '%' 拒绝/超长端点）、drift 三门测、
upgrade 端点长度测（PreviewUpgrade 400 非 500）、tools 包 instructions 置
空测全过。

**G2（F06/F07/F08/F09）**：服务层 ClearMCPCredential/GetMCPServiceTools 补
插件守卫返 ErrPluginManagedService，handler DeleteField/tools 端点映 409；
pluginManagedConflict 连注释移到首个 swag 注释块之前（F08，注解不再被劫
持——源码断言测试锁定）；driftReportResponseDTO 四列表 make+copy（F09，
wire 恒 [] 非 null——json.Marshal 断言锁定）。证据：mcp_plugin_guard_test
新增四测先 RED（500/null/位置错）后 GREEN。

**G3（F01/F02/F03/F04/F13/F14）**：WithStateLess(true)（F01，未认证
initialize 不再无界堆积会话）；validateAllowedRedirectHosts 拒空 host 带
端口条目（F02）；manifest.json 副本补 content_digest + server_test 双
digest 比对（F03，值经 plugins.ManifestContentDigest 对副本计算得出）；oauth
decoder.More() 拒尾随垃圾 + 复用 maxRPCBodyBytes（F04）；fakejira 三注入
方法接受 testing.TB 未命中 Fatal（F13）；plugintest 替身 Tool.Call 失败改
协议级 error（F14）+ e2e 三场景断言同步（401 场景改断言真实链路的授权引
导文案替换分支；上游计数断言 2→GreaterOrEqual(2)——协议 error 断连重试一
次属真实链路语义）。证据：examples 包全测 ok（含新双 digest 断言）；
`go test -tags integration -run TestJira*`（真 PG 容器）四测 ok。

**N1（F18/F19/F20/F21/F23）**：service_id 改 optionalText（F18，confirm 中
断窗口的空串 200 不再被解析器抛错）；新增 acceptUpgrade（F19）与
getDrift/checkDrift/resolveDrift + PluginDriftReport 解析器（F23，空列表归
一 [] 对齐 F09 wire 形态）；disabledReason 改 optionalText + 方括号路径
（F20）；enabled 类型收窄 boolean + flag 严格解析 + 注释更正（F21）。
证据：plugins.test.ts 39/39（新增五测；旧的 missing-enabled-as-null 用例按
新契约改为缺 key 抛错）。

**N2+W1（F24/F25/F26/F34/F35/F40/F42/F43/F44 + F19/F23 面板接入）**：四
locale subtitle 补「写入类工具默认关闭」（F24，registry.test 五 locale 断
言）；page.tsx/view.ts 三处过期注释改述（F25/F26）；Settings 面板失效
effect 清全部 client-bound 临时态 + confirmInstall 成功清 upgradePreview
（F34）；policyToggleBusy 收窄 boolean（F44）；停用/启用补跨行 disabled
（F40）；升级面板接入「接受升级」（F19）与「漂移复审」面
（getDrift/checkDrift/resolveDrift + 四名单 + 重定基，F23）；PluginsPanel
pluginsApi 变化重置 requestedRef/connections（F42）、删 retryConnection 净
空两行（F35）、anyPending 全行冻结 + 行内 loading（F43）。证据：
PluginsSettingsPanel.test 32/32（新增 F19/F34/F23 三测）、PluginsPanel.test
13/13（新增 F42 重置测——未修复时第二次 connections/me 不可能发出）。

**全量回归**：`go build ./...` ok；gofmt 改动文件全净、go vet 干净；
`go test` plugins/handler/service/plugintest/agentruntime-tools/examples 全
ok（service 180s）；integration e2e 真 PG 四测 ok；typecheck:shared 与
typecheck:web 均 0 error；前端相关测试全绿（13/17/26/39/13/32）；
`pnpm run test:craft:shared` 113/113。

**计划文档同步**（接口/依赖变化）：DiffLiveAgainstSnapshot 签名（detail,
error）——两调用方与 drift_test 已同步；fakejira 三方法加 testing.TB——
e2e 调用点同步；api-client PluginsApi 接口新增五方法（acceptUpgrade/
getDrift/checkDrift/resolveDrift）+ PluginDriftReport 类型导出；PluginInstallationTool.enabled 类型 boolean。

---

# OCR 修复轮次计划（R2 轮，2026-09-26）——22 项

输入：OCR R2 轮 22 条有效发现，多项直指 R1 轮修复自身的残余缺陷（F33 掩码
协议相对 URL 失效、F34 '/' 拒绝误伤插件名、F29/F30 测试 win32 假红、
F02/F03 R1-F42 修复的竞态/首跑残余、F10/F11/F13/F14 面板失效不彻底）。原
则不变：证据优先、TDD、中文提交标注「OCR R2」、范围仅本 worktree。按域分
五批，依赖：C 批 F27（shimEnv baseEnv 参数）先行于 F29/F30 测试改造；A/B
相互独立；D/E 相互独立。

## 批次 A（Go 安全/并发，medium×3+low×2）：F33/F20/F21/F24/F34

- 根因：①userinfoOf/maskEndpointCredentials 在无 "://" 输入下把 rest 首个
  "/" 当 authority 终点，协议相对 URL "//user:pass@host:badport/" 的
  authority 切为空串——掩码 no-op，而该输入恰令 url.Parse 失败走失败回显
  路径，凭据明文进 400（F33，R1 F03 修复的旁路）；②UninstallInstallation
  与 AcceptUpgrade/ResolveDrift/CheckDrift 同域写路径不共持 per-installation
  锁，卸载级联删 approvals 后并发 accept 再写策略行→FK violation 误报 500
  （F20）；③flipInstallation 把 UpdateInstallationState 的 0 行更新
  （gorm.ErrRecordNotFound，仅并发卸载可致）包成 500，语义应 404（F21）；
  ④HardDeleteServiceCascade 接口文档只列 approvals，生产实现同事务还删
  oauth tokens/clients/mcp_metadata——按文档实现的 fake 与生产语义相悖
  （F24）；⑤R1-F16 的 '/'/'%' 拒绝加在插件 name 共用的 validateName，
  "CI/CD Helper" 这类合法展示名被拒（F34）。
- 文件：manifest.go（userinfoOf/maskEndpointCredentials 识别前导 "//"；
  拆 validateToolName 承载 '/'/%' 拒绝——validateName 保留 Unicode 卫生，
  四处工具名调用点换用 validateToolName）、plugin_install_service.go
  （UninstallInstallation 入口 defer lockUpgradeAccept；flipInstallation
  errors.Is(gorm.ErrRecordNotFound)→ErrInstallationNotFound）、
  types/interfaces/plugin.go（HardDeleteServiceCascade 文档枚举完整清扫范围）。
- 回归测试：manifest_test（协议相对 URL 掩码+parse 失败回显不含凭据；插件
  名含 '/' 通过、工具名含 '/' 仍拒）、install_service_test（并发卸载后
  SetInstallationState 404——用 fake UpdateInstallationState 返回
  gorm.ErrRecordNotFound 断言 ErrInstallationNotFound）。
- 完成条件：新测 RED→GREEN；modules/plugins、application/service 全量绿。

## 批次 B（示例+替身，high×1+low×2）：F18/F19/F35

- 根因：①SearchMyWeek 分页请求键 pageToken 与 Atlassian /search/jql 契约
  不符（应为 nextPageToken）——真实 Jira 忽略未知键恒返第一页，>100 条时
  循环抓重复页并误报 truncated；两处替身（examples ocr_fix_test 与
  fakejira）钉死/掩盖错误契约（F18）；②handleRegister 对 redirect_uri 未
  拒 fragment/userinfo（RFC 6749 §3.1.2）（F19）；③SetMyselfDelay 违反
  R1-F13 确立的注入方法 TB+Fatal 约定（F35）。
- 文件：examples jira.go（payload 键 nextPageToken）、ocr_fix_test.go（替身
  读 nextPageToken+断言同步）、oauth.go（注册校验补 Fragment/User 拒收）、
  plugintest/fakejira.go（SetMyselfDelay 加 TB 未命中 Fatal）、
  jira_e2e_integration_test.go:499 调用点。
- 回归测试：examples 分页测试改键后全过；注册 fragment/userinfo 拒收新测。
- 完成条件：examples 包与 plugintest 相关测试全绿。

## 批次 C（scripts，medium×3+low×2）：F01/F27/F28/F29/F30

- 根因：①frontend.yml 的 on.push/on.pull_request paths 不含 node-gte26 管
  线文件——只改管线的 PR 整体跳过 workflow，自测恰在最需要时失效（F01）；
  ②shimEnv 只覆写 find() 首个大小写变体键，双键共存（POSIX 显式注入）时
  未命中键的旧值仍传子进程（F27）；③copyFileSync 失败时 mkdtemp 私有目录
  泄漏（调用方未拿到 shimDir 无从清理）（F28）；④两条 shimEnv 测试操纵
  process.env.Path/PATH，win32 的大小写不敏感赋值/删除使断言必然假红或不
  稳定（F29/F30）。
- 文件：node-gte26.mjs（shimEnv 加 baseEnv 参数+删除全部变体键再写唯一
  键；createNodeShim copy 失败先 removeNodeShim 再抛）、node-gte26.test.mjs
  （双测改 baseEnv 对象字面量+大小写无关键查找断言，新增双键共存测试）、
  frontend.yml（两处 paths 补 scripts/lib/** 与 run-gates.mjs/
  run-with-node-gte26.mjs/package.json 的 test:node-gte26 相关——直接补
  "scripts/lib/**" 与两入口路径）。
- 回归测试：node --test 全绿（含新双键用例）。
- 完成条件：单测绿；frontend.yml 逐行核对。

## 批次 D（PluginsPanel，medium×2+low×4）：F02/F03/F04/F06/F07/F08

- 根因：①loadConnection 无代际守卫——client 切换后旧 principal 在途回包
  经函数式 setConnections 落键覆盖新视图（F02，R1-F42 修复的竞态残余）；
  ②重置 effect 首挂无条件执行，SSR 直出路径连接请求×2（F03）；③共享页
  section heading 与面板自带 h2/描述双重渲染且文案分叉（F04）；④未区分
  加载中/无数据——首拉期间闪现空态（F06）；⑤import * as React 死导入
  （F07）；⑥授权轮询缺卸载终止，后台请求照发 60s（F08）。
- 文件：PluginsPanel.tsx（connectionEpoch ref+loadConnection 代数比对丢弃；
  重置 effect 跳首跑；删内置标题块；listLoaded 态；删死导入；authorize
  轮询 aliveRef）、messages.ts（subtitle 并入「需个人授权」分句——五
  locale）。
- 回归测试：PluginsPanel.test.tsx 追加（client 切换后旧回包不覆盖、首挂
  仅一次 connections 请求、加载中不闪空态）；既有 13 测不破。
- 完成条件：面板测试全绿；typecheck:web 0 error。

## 批次 E（PluginsSettingsPanel，medium×3+low×2）：F10/F11/F12/F13/F14

- 根因：①acceptUpgradeInstall 成功未失效同安装 toolPolicy——升级切换快照
  后治理行停留在旧快照，PUT 旧工具 404、新工具不可见（F10）；②
  runDriftResolve 成功同样未失效（F11）；③五处动作驱动 refreshInstallations()
  直调无 isStale——旧 client 闭包动作完成时迟到列表响应覆盖新空间（F12）；
  ④checkUpgrade 失败无条件撤面板——误撤其他来源安装的已展开面板（F13）；
  ⑤client 切换失效块漏 setError(null)（F14）。
- 文件：PluginsSettingsPanel.tsx（accept/resolve 成功路径
  setToolPolicy(null)；refreshInstallations 内部默认绑定 epoch 代数比对；
  checkUpgrade 失败 setUpgradePreview 按来源安装条件撤；失效块补
  setError(null)）。
- 回归测试：追加（接受升级后治理面撤下、B 失败不撤 A 面板）；既有 32 测
  不破。
- 完成条件：面板测试全绿。

## R2 轮完成条件（总）

1. 22 项全部处置；各批次新测试先 RED 后 GREEN；
2. 受影响包复跑全绿（modules/plugins、application/service、examples、
   plugintest、面板/views/api-client 测试、node-gte26 单测）；
3. go build/gofmt/vet 干净；typecheck:web/shared 0 error；
4. 中文提交按域分批标注「OCR R2」；本节回填完成记录。

### R2 轮完成记录（2026-09-26）

22 项全部修复，无剩余项。逐批证据：

**A（F33/F20/F21/F24/F34）**：userinfoOf/maskEndpointCredentials 识别前导
"//"（协议相对 URL 的 authority 不再切空串——F33 的 RED 实测形态
masked 原样返回/凭据明文已由新测锁定）；UninstallInstallation 入口加
defer lockUpgradeAccept（F20——测试先无锁跑出 "must make no progress"
失败后恢复锁 GREEN，RED/GREEN 双验证）；flipInstallation 对 0 行更新
（gorm.ErrRecordNotFound）返 ErrInstallationNotFound（F21，updateStateErr
注入实测）；HardDeleteServiceCascade 文档枚举完整清扫范围（F24）；
validateToolName 拆分承载 '/'/'%' 拒绝、插件展示名只留 Unicode 卫生
（F34——"CI/CD Helper" 通过、工具名 "a/b" 仍拒双断言）。证据：五组新测
全绿 + plugins/service 两包全量 ok。

**B（F18/F19/F35）**：分页键 pageToken→nextPageToken（jira.go+替身 JSON
键与断言同步）；注册校验补 fragment/userinfo 拒收（新增两 URI 拒绝断
言）；SetMyselfDelay 加 testing.TB 未命中 Fatal（e2e 调用点同步）。证据：
examples 包全测 ok；-tags integration 慢验证 e2e ok。

**C（F01/F27/F28/F29/F30）**：frontend.yml 两处 paths 补 scripts/lib/**
与两入口（F01）；shimEnv 删全部大小写变体键再写唯一规范键+baseEnv 参
数（F27——新增 PATH/Path 双键共存测试）；createNodeShim copy 失败先
removeNodeShim 再抛（F28）；两条 shimEnv 测试改 baseEnv 对象字面量+大
小写无关键查找（F29/F30）。证据：test:node-gte26 14/14；wrapper exit 0
复验。

**D（F02/F03/F04/F06/F07/F08）**：connectionEpoch 代数守卫（F02——挂起
旧回包+切 client+放行的时序实测：旧 authorized 回包被丢弃、新视图保持
未授权）；重置 effect 跳首跑（F03）；删面板内置标题块+「需个人授权」
分句并入五 locale subtitle（F04）；listLoaded 加载态区分（F06——挂起列
表请求实测不闪空态、直出空数组视为已加载）；删 React 死导入（F07）；
授权轮询 aliveRef 卸载终止（F08）。证据：PluginsPanel.test 15/15（含
F02/F06 两新测）。

**E（F10/F11/F12/F13/F14）**：accept/resolve 成功路径失效同安装
toolPolicy（F10 新测实测展开→接受→治理面撤下；F11 同款代码路径）；
refreshInstallations 内部默认绑定 installationsEpoch（F12——五处直调共
享陈旧防护，effect 同步自增）；checkUpgrade 失败按来源安装条件撤面板
（F13——B 失败不撤 A 面板新测）；失效块补 setError(null)（F14）。证据：
PluginsSettingsPanel.test 34/34（含 F10/F13 两新测）。

**全量回归**：go build ok；本轮改动文件 gofmt 全净（三个 gofmt 项
plugin_drift_integration_test/tool_policy_test/query_history_policy_test
经 stash 比对确认为基线固有，未触碰）；go vet 六包无输出；Go 六包测试
全 ok；-tags integration 真 PG 五测 ok；typecheck:web/shared 0 error；
views 17/17、SettingsPage 26/26、api-client 39/39、PluginsPanel 15/15、
PluginsSettingsPanel 34/34、craft:shared 113/113。

**接口/依赖变化同步**：PluginsPanel Props 的 initialInstallations 去掉
默认参数（undefined=无直出/数组=直出含空列表，listLoaded 语义依赖该区
分）；fakejira SetMyselfDelay 加 testing.TB（调用点同步）；node-gte26
shimEnv 增可选 baseEnv 参数（默认 process.env，既有调用零改动）。

## B+A 裁决批次（2026-09-26）：后端安全/正确性 9 项（4 修复 + 5 已修复核）

用户裁决 B+A：修复经 OCR 多轮裁决确认有效的 9 个后端发现。逐一核实
现状后：4 项本轮修复（#1/#2/#4/#5），5 项经核实已被此前轮次修复（#3
R2-F20、#6 R1-F45、#7 R1-F09、#8 R2-F18、#9 R1-F08——记录证据、无需
改码）。

### #1 幂等短路崩溃窗口 fail-open（plugin_install_service.go:1295 族）

- 根因：AcceptUpgrade Step 6 / ResolveDrift Step 4 的幂等短路（版本/
  digest/端点/漂移/服务行同步全匹配 → 零写入）不校验策略行完备性。快照
  落库（7a/5a）后策略行写完（7c/5b）前被硬杀，重试命中短路零写入，
  MCPToolApproval「缺行=运行时门默认启用」使缺行写工具默认启用——触碰
  「新增写工具默认关闭」边界。ConfirmInstallation 同族：确认中途硬杀后
  重试一律 409「走升级路径」，而升级路径短路同样放行。
- 修复：新增 installationPolicyRowsComplete 助手；AcceptUpgrade/ResolveDrift
  短路前校验候选/rebased 快照每工具显式策略行（svc==nil 无行可键时维持
  原零写入读法），缺失落入写路径增量补齐（重写同值无害+只补缺行）；ResolveDrift
  的 svc 载入移到短路前。ConfirmInstallation 新增 completeCrashedConfirm：
  digest/版本/端点全等的同内容重试经 serviceIDByInstallation 反查自愈空
  service_id 锚（卸载自愈同款）→ 增量补齐缺行 → 消费预览；行完备保持
  既有 already-installed 409（TestConfirmInstallationIdempotent 判定不破）
  、反查无行（物化前被杀，fail-closed 无暴露面）保持 409。
- RED→GREEN：TestAcceptUpgradeIdempotentShortCircuitCompletesMissingPolicyRows
  （RED 实测：重试成功但 create_issue 行仍缺）、TestResolveDriftIdempotent-
  ShortCircuitCompletesMissingPolicyRows（RED 同形态）、TestConfirmInstallation-
  HealsCrashedPolicyRows + TestConfirmInstallationHealsCrashedServiceBinding
  （RED 实测：重试吃 ErrPluginAlreadyInstalled）。
- 提交 a9d0fd0ce。

### #2 SetInstallationState 缺 per-installation 锁（:689）

- 根因：状态切换与 AcceptUpgrade 同族读-改-写——syncService 先 GetByID
  载入物化服务再整行 Update（含 URL/AuthConfig），不持锁时与接受路径 7b
  交错可整行回滚 accept 刚切换的端点/OAuth 基线、或在 7a/7b 之间落「安
  装行 active+服务行停用」再被 7b 覆盖复活。
- 修复：入口 defer lockUpgradeAccept（与 AcceptUpgrade/ResolveDrift/
  CheckDrift/UninstallInstallation 同款串行化）。
- RED→GREEN：TestSetInstallationStateConcurrentAcceptSerializes（RED 实测
  「接受在途期间状态切换零进展」不成立；GREEN 后 accept 7b 中途并发切换
  全程阻塞+终态 URL=v2/Enabled=false/安装行 disabled）。提交 28fcaa96b。

### #3 UninstallInstallation 缺锁——已修复核（R2 F20）

- 证据：plugin_install_service.go UninstallInstallation 入口
  `defer lockUpgradeAccept(installationID)()`（注释标 OCR R2 F20）；
  TestUninstallConcurrentAcceptSerializes 本轮复跑仍绿。无需改码。

### #4 GetMCPServiceResources/TestMCPService 缺插件物化守卫（mcp_service.go:493 族）

- 根因：GetMCPServiceTools（R1 F07）拒绝插件行，但 GetMCPServiceResources
  （Viewer+）与 TestMCPService（Admin+）仍直连实时远端——资源列表/连通
  测试结果绕过已接受快照边界暴露未接受能力与漂移后目录；且 test 面
  handler 把服务层错误包成 200 失败结果、resources 面落默认 500。
- 修复：两方法 GetByID 后补 `PluginInstallationID != nil → ErrPluginManagedService`
  （与 R1 F07 同口径）；handler 两面补 pluginManagedConflict 409。
- RED→GREEN：服务层 TestGetMCPServiceResourcesRejectsPluginManagedRow /
  TestMCPServiceTestRejectsPluginManagedRow（RED：返回 nil/连通错误非哨兵）；
  handler 层 TestGetMCPServiceResourcesPluginManagedIsConflictNot500（RED
  500）/ TestMCPServiceTestPluginManagedIsConflictNotTestFailure200（RED 200
  包装）。提交 25a21de58。

### #5 maskEndpointCredentials/userinfoOf authority 未按 /?# 终止（manifest.go:446/451/474）

- 根因：R2 F33 修了前导 //，但 authority 仍只按 '/' 截断——query 内 '@'
  被当 userinfo（掩码把 host+query 整段吞成 "REDACTED@evil"、userinfoOf
  对无凭据 URL 误报）；真实 userinfo+query 含 '@' 时 LastIndex 取错 '@'
  掩码丢失 host 与 query；协议相对前导 // 在输出中丢失。
- 修复：新增 authorityEnd（IndexAny "/?#"，RFC 3986 §3.2）；mask 的协议
  相对分支 prefix 保留 "//"。
- RED→GREEN：TestUserinfoAndMaskTerminateAuthorityAtQueryAndFragment（RED
  实测 "https://REDACTED@evil.example" 失真形）。提交 318de31a7 + gofmt
  补遗 891f351d9。

### #6 升级接受路径候选端点 512 校验——已修复核（R1 F45）

- 证据：AcceptUpgrade 与 PreviewUpgrade 均有 validatePluginURLLength(
  "plugin transport endpoint", ...)（注释标 OCR R1 F45）；
  TestUpgradeCandidateEndpointLengthBounded 在册。无需改码。

### #7 drift 明细四列表 append(nil) → JSON null——已修复核（R1 F09）

- 证据：handler/plugin.go driftReportResponseDTO 四列表 make+copy（注释标
  OCR R1 F09）；TestDriftReportResponseListsNeverSerializeAsNull 在册且本轮
  复跑绿。无需改码。

### #8 jira.go pageToken→nextPageToken——已修复核（R2 F18）

- 证据：jira.go SearchMyWeek 以 payload["nextPageToken"] 跟页、
  NextPageToken 读回；ocr_fix_test.go 替身按 nextPageToken 键收发（354/
  373 行）；fakejira.go 全文无分页读取逻辑（分页契约由测试替身承载，R2
  记录所述「fakejira 分页读取」实为替身侧）。无需改码。

### #9 pluginManagedConflict 插在 swaggo 注释块中间——已修复核（R1 F08）

- 证据：handler/mcp_service.go pluginManagedConflict 位于首个 swaggo 注
  释块（// CreateMCPService godoc）之前，注释标 OCR R1 F08；TestSwag-
  AnnotationsPrecedeCreateMCPServiceDeclaration 在册且本轮复跑绿。
  无需改码。

### B+A 批次回归证据

- 受影响包：go test ./internal/modules/plugins/ ./internal/application/
  service/ ./internal/handler/ 全 ok（service 169.5s）。
- 本轮改动文件 gofmt 全净、go vet 三包无输出（仓库既有 gofmt 脏文件为
  基线固有，未触碰）。
- 全量门控（真实退出码）：
  - `go test ./...` → exit 0（129 包 ok，0 FAIL；链接器 -lc++ 重复库警告
    为既有基线现象）；
  - `pnpm run test:shared` → exit 0（tests 1024：pass 1023 / fail 0 /
    skipped 1）；
  - `pnpm run typecheck:shared` → exit 0（0 error）。
