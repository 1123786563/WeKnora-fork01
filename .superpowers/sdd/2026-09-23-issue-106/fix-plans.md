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
