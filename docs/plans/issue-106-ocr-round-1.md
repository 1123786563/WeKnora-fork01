Review complete: 45 finding(s) across 44 selected item(s).

─── examples/plugins/jira-todo-mcp/main.go:265-267 ───
[security · medium] 「/mcp 目录公开」意味着未认证方可随意 POST initialize，而 WithStateLess(false) 开启有状态会话：每个
initialize 都会在 mcp-go 的服务端会话存储中创建条目。本文件未配置任何会话数量上限或过期淘汰选项，会话释放依赖客户端主动 DELETE——若所用 mcp-go v0.52.0
的内存会话存储无默认 TTL 淘汰（该语义无法在本仓库内核实），未认证方即可通过高频 initialize 无界堆积会话内存，与 oauth.go 对全部 5 个令牌 map 严格施加
TTL+容量界的纪律不一致。另外，同一更新中的 internal/modules/plugins/plugintest/server.go:188-193 已验证
WithStateLess(true) + WithHTTPContextFunc 的鉴权注入模式在无状态下同样可用，本服务也不依赖服务端主动通知流（WeKnora 客户端路径在 plugintest
无状态伪服务上全部通过）——建议改为无状态；若确需有状态，请配置 SDK 的会话过期/上限选项（若该版本提供）并在注释说明依据。

  	streamable := sdkserver.NewStreamableHTTPServer(mcpServer,
- 		sdkserver.WithStateLess(false),
+ 		sdkserver.WithStateLess(true),
  		// 从每个入站 HTTP 请求提取 Bearer 并解析为已授权会话，注入工具
+ 		// handler 的 context（SDK 工具 handler 本身不透传 HTTP 头）。无状态模式
+ 		// 下未认证 initialize 不驻留服务端会话，内存面与 oauth.go 的有界纪律一致
+ 		// （同款组合见 plugintest/server.go）。
+ 		sdkserver.WithHTTPContextFunc(oauthSrv.contextFunc),
+ 	)


─── examples/plugins/jira-todo-mcp/main.go:221-224 ───
[bug · low] validateAllowedRedirectHosts 只拒绝钉 :80/:443 的条目，漏掉了空 host 带端口的条目（如
":8080"）：net.SplitHostPort(":8080") 成功返回 host=""、port="8080"，随后 handleRegister 的匹配循环中该条目与任何合法
redirect_uri 的 host（"example.com:8080"）或裸
host（"example.com"）都不相等——这类条目对真实主机永不命中，运营者的名单写法错误将静默失效（若名单只含此类条目则所有合法注册被拒），与已拒绝的默认端口条目属同一类「静默失效」配置错误
，应一并 fail-closed。

- 		_, port, err := net.SplitHostPort(entry)
+ 		host, port, err := net.SplitHostPort(entry)
  		if err != nil {
  			continue // 裸 host / 裸 IPv6 条目，无端口语义
+ 		}
+ 		if host == "" {
+ 			return fmt.Errorf("AllowedRedirectHosts entry %q pins a port with an empty host; use host or host:port instead", entry)
  		}


─── examples/plugins/jira-todo-mcp/manifest.json:19-22 ───
[test · low] 参考副本缺少 content_digest 字段，而运行时 Manifest() 会写入该字段（json tag content_digest），动态
/manifest.json 权威产物因此与副本结构不一致。防漂移测试 TestSelfHostedManifestMatchesContract（server_test.go:198-227）只断言
input_schema_digest 一致——副本上 name/description 等语义字段的漂移不会被捕获，与 main.go 中「副本的 digest
值由同一函数生成，并有测试断言副本与代码计算值一致」的声明不完全相符。由于 ManifestContentDigest 明确排除自身字段，对副本自身内容（含占位 endpoint）计算
content_digest 是自洽的、且能通过 FetchAndVerify 的 content_digest 校验——建议在副本补上该字段（值 =
plugins.ManifestContentDigest 对副本解析结果计算），让测试把两个 digest 一同比对。

        "input_schema_digest": "99334726611ccf58a148b0814696bfa6fe08c1b2d027e946beccf5a74331c9aa"
      }
-   ]
+   ],
+   "content_digest": "<由 plugins.ManifestContentDigest 对本副本解析结果计算（排除 content_digest 自身、含占位 endpoint），与测试一并断言>"
  }


─── examples/plugins/jira-todo-mcp/oauth.go:375-375 ───
[maintainability · low] 两点次要问题：1) json.Decoder.Decode 只消费首个 JSON 值——`{"redirect_uris":[...]} 附加垃圾`
这类携带尾随内容的注册体仍解析成功，与错误信息「must be a valid JSON registration document」的 fail-closed
表述不一致（gateCallToolAuth 对同形态已显式 400 拒绝），可在 Decode 后用 decoder.More() 检查尾随内容。2) 请求体上限写为裸字面量 1<<20，同包
main.go 已有等值常量 maxRPCBodyBytes，宜复用避免两处上限各自漂移。

- 	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&registration); err != nil {
+ 	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRPCBodyBytes))
+ 	if err := decoder.Decode(&registration); err != nil || decoder.More() {
+ 		writeJSON(w, http.StatusBadRequest, map[string]any{
+ 			"error": "invalid_client_metadata", "error_description": "request body must be a single valid JSON registration document with redirect_uris",
+ 		})
+ 		return
+ 	}


─── examples/plugins/jira-todo-mcp/oauth.go:836-839 ───
[bug · low] exchangeCode（refresh() 为同款模式）的 now 在进入临界区前捕获：code 过期判定（now.After(issued.ExpiresAt)）、新令牌
ExpiresAt 与响应 expires_in 都基于该时刻。而此刻到实际消费之间隔着 maybeSweepLocked 全量清扫（容量上限下 5 个 map 约 2
万条目）与锁排队——授权码以回溯时间判定（略放宽）、令牌实际存活期略短于声明的 expires_in。submitAuthorizeForm 已在本 PR 确立「TTL
必须用消费时刻时钟」纪律（OCR 二轮 F3：now = time.Now() 于锁内重取），exchangeCode/refresh 应与之统一，避免清扫成本增长时声明与实现脱节。

  	redirectURI := r.PostFormValue("redirect_uri")
- 	now := time.Now()
  	s.mu.Lock()
+ 	// TTL 判定与令牌签发必须用消费时刻时钟（与 submitAuthorizeForm 的纪律一致）。
+ 	now := time.Now()
  	s.maybeSweepLocked(now)


─── internal/application/service/mcp_service.go:534-540 ───
[bug · low] 同族的第四个凭据写面 ClearMCPCredential（本文件 574 行，DELETE
/mcp-services/{id}/credentials/{field}）未加同样的插件管理守卫：对 PluginInstallationID 非空的行，PUT /credentials
确定性地返回 409，而 DELETE /credentials/{field} 却返回 204 成功。由于插件行的 api_key/token 按构造恒为空，今天只是无状态的 204
而非真实变更，但这使通用凭据写面对插件行的语义不对称，违背本 diff 建立的"凭据写面确定性拒绝"意图；一旦未来任何路径能给插件行注入服务级凭据，这里就成为绕过面。建议在
ClearMCPCredential 的 IsBuiltin 检查后追加相同守卫（返回 ErrPluginManagedService），并在
MCPCredentialsHandler.DeleteField 中复用 pluginManagedConflict 映射 409。

- 	// Plugin-materialized rows carry no service-level credentials by
- 	// construction (materialization only sets AuthType/Scopes from the
- 	// verified baseline); injecting one via the generic path would override
- 	// the member-OAuth flow (跨任务转交 T04-OCR1-F6).
+ 	// (在 ClearMCPCredential 的 IsBuiltin 检查之后补齐同族守卫)
  	if existing.PluginInstallationID != nil {
- 		return nil, ErrPluginManagedService
+ 		return ErrPluginManagedService
  	}


─── internal/application/service/mcp_service.go:534-540 ───
[security · medium] 同文件紧邻的读取面 GetMCPServiceTools（本文件 479-506 行，路由 GET
/mcp-services/{id}/tools，Viewer+ 权限）未对插件物化行做任何快照处理：它直接 GetOrCreateClient + ListTools 返回原始 live 目录。对
PluginInstallationID != nil 的服务行，任何 Viewer 成员都能枚举超出已接受快照的工具（例如插件开发者事后在端点上新增的工具），以及漂移后的
schema/描述——这正是本次改动在写面（PUT/DELETE/凭据）和 Agent
运行时过滤（FilterToolsBySnapshot）上努力封堵的"未接受能力不可见"不变量，但通过这个通用读取面仍可直接泄露。OAuth
插件行因无服务级凭据可能连接失败，但无个人鉴权要求（no-auth/static）的插件行完全可达。建议：与写面一致，对该分支解析运行时快照后用 FilterToolsBySnapshot
过滤（或直接返回已接受快照中的工具；若不打算支持通用面展示插件行，则同样返回 ErrPluginManagedService 并在 handler 映射 409）。

- 	// Plugin-materialized rows carry no service-level credentials by
- 	// construction (materialization only sets AuthType/Scopes from the
- 	// verified baseline); injecting one via the generic path would override
- 	// the member-OAuth flow (跨任务转交 T04-OCR1-F6).
- 	if existing.PluginInstallationID != nil {
- 		return nil, ErrPluginManagedService
+ 	// 与写面及运行时过滤一致：插件物化行的 live 目录不得绕过已接受快照
+ 	if service.PluginInstallationID != nil {
+ 		return nil, ErrPluginManagedService // 或解析快照后经 FilterToolsBySnapshot 过滤再返回
  	}


─── internal/handler/mcp_service.go:76-81 ───
[documentation · medium] pluginManagedConflict 函数被插入在 CreateMCPService 的 swaggo 注释块中间：注释行 "//
pluginManagedConflict ..." 直接续在 "// @Router /mcp-services [post]" 之后，二者连成同一个注释组，且该注释组现在紧邻的是
pluginManagedConflict 而非 CreateMCPService。swaggo/swag 等按"注释组紧邻其后函数声明"配对的文档工具会把 POST /mcp-services
操作误绑定到这个未导出辅助函数上，而 CreateMCPService（当前位于第 89 行）则失去全部注解、从生成的 OpenAPI 规格中消失；godoc
展示同样错位。请把辅助函数（连同其说明注释）整体移到 "// CreateMCPService godoc" 注释块之前，恢复注解块与处理器的相邻关系。



─── internal/handler/plugin.go:796-802 ───
[bug · medium] 漂移明细四列表（Added/Removed/SchemaChanged/DescriptionChanged）使用 append([]string(nil), ...)
拷贝，空输入时结果为 nil，JSON 序列化为 null——违反本文件反复声明的「wire 字段唯一形态 []、绝不 null」约定（T01-R1-F1），且与同函数内
SnapshotToolNames 的 nil→[] 归一化自相矛盾。这不是纯理论问题：(1) 持久化类型 types.PluginDriftDetail 四字段带
omitempty，CheckDrift 落库时空列表被丢弃，GetDrift 经 parseDriftDetail 反序列化后这些字段为 nil；(2) 即便是新鲜 CheckDrift
响应路径，Go 语义下 append 零元素返回原接收者，非 nil 空切片 []string{} 也会拷贝成 nil。只要检测到漂移（四列表至多部分非空，如 description-only
漂移时另外三列表全空），响应中至少 3 个字段必为 null，按 string[] 类型消费的前端在 .map/.length 处会运行时崩溃。建议改为 make+copy（与其余所有 mapper
的约定一致）。

  		out.Detail = &dto.PluginDriftDetailDTO{
- 			Added:              append([]string(nil), report.Detail.Added...),
- 			Removed:            append([]string(nil), report.Detail.Removed...),
- 			SchemaChanged:      append([]string(nil), report.Detail.SchemaChanged...),
- 			DescriptionChanged: append([]string(nil), report.Detail.DescriptionChanged...),
+ 			Added:              make([]string, len(report.Detail.Added)),
+ 			Removed:            make([]string, len(report.Detail.Removed)),
+ 			SchemaChanged:      make([]string, len(report.Detail.SchemaChanged)),
+ 			DescriptionChanged: make([]string, len(report.Detail.DescriptionChanged)),
  			CheckedAt:          report.Detail.CheckedAt,
  		}
+ 		copy(out.Detail.Added, report.Detail.Added)
+ 		copy(out.Detail.Removed, report.Detail.Removed)
+ 		copy(out.Detail.SchemaChanged, report.Detail.SchemaChanged)
+ 		copy(out.Detail.DescriptionChanged, report.Detail.DescriptionChanged)


─── internal/modules/agentruntime/agent/tools/mcp_tool.go:562-566 ───
[security · low] loadPluginDirectory 只校验工具名/schema/description，而 loadMCPServiceTools 返回的 live 服务端
instructions 被原样透传：它随返回值写入 tool.serverInstructions，并经目录 describe 输出的 server_instructions
字段（mcp_catalog.go:696）进入模型上下文。instructions 不在已接受快照基线内（BuildVerifiedSnapshot/PluginToolSnapshot
均不含它），因此插件开发者可在管理员接受后随意改写服务端 instructions，向所有成员会话注入任意提示词——这与本函数把 description 漂移判定为 prompt-injection
通道的威胁模型是同一类。建议：snap != nil 时运行期不透传 live instructions（置空返回），或将其纳入快照/预览基线并在漂移校验中一并比对。

- 	definitions, instructions, err := loadMCPServiceTools(loadCtx, service, mcpManager, gate, oauthSess)
- 	if err != nil {
- 		return nil, "", err
- 	}
  	filtered, filterErr := FilterToolsBySnapshot(snap, definitions)
+ 	if filterErr != nil {
+ 		...
+ 	}
+ 	// 未接受基线覆盖 instructions：插件行不透传 live 服务端指令文本
+ 	instructions = ""


─── internal/modules/plugins/fetcher.go:105-109 ───
[security · medium] `url.Parse` 失败时返回的 `*url.Error` 的 `Error()` 原样内嵌完整原始 URL（`parse
"https://user:pass@host/%zz": invalid URL escape "%zz"`），经 `%w` 透传后由 handler 的 default 分原样拼入管理员 400
响应体（`internal/handler/plugin.go` mapPluginPreviewError），并落入服务端日志——含 userinfo 凭据的畸形 URL（如
`https://user:pass@host/%zz`、含控制字符等）确定可达此分支。这与紧随其后的 `u.User != nil` 分支注释宣称的"The message does not
echo the URL, which contains the credentials (OCR T01-R3-F2)"纪律直接矛盾（manifest.go 对 transport endpoint
的同类路径专门做了 maskEndpointCredentials + 剥离 *url.Error 包装）。建议剥掉 *url.Error 包装、对回显的 URL 先做 mask（同包已有
maskEndpointCredentials/echoQuoted 可复用）；`fetchLimited` 第 179 行的 `invalid manifest URL: %w`
是同一模式，建议一并处理。

  	if u, err := url.Parse(manifestURL); err != nil {
- 		return nil, fmt.Errorf("invalid manifest URL: %w", err)
+ 		var uerr *url.Error
+ 		reason := err
+ 		if errors.As(err, &uerr) {
+ 			reason = uerr.Err
+ 		}
+ 		return nil, fmt.Errorf("invalid manifest URL %s: %s",
+ 			echoQuoted(maskEndpointCredentials(manifestURL)), reason)
  	} else if u.User != nil {
  		return nil, fmt.Errorf("manifest URL must not embed userinfo credentials")
  	}


─── internal/modules/plugins/manifest.go:159-161 ───
[bug · medium] `strings.ReplaceAll(reason.Error(), userinfoOf(...), "REDACTED")` 在端点不含 userinfo
时会破坏错误文本：`userinfoOf` 对无 `@` 的 URL 返回空串，而 Go 的 `strings.ReplaceAll` 在 old 为空串时会在字符串开头及每个 UTF-8
序列之后各插入一次 new（k 个 rune 得到 k+1 处替换）。因此最常见的畸形端点（无凭据，如 `https://host:badport/`，url.Parse 报 `invalid
port ":badport" after host`）会把 reason 搅成 `REDACTEDiREDACTEDnREDACTEDvREDACTEDa...` 之类的乱码，经 handler 的
default 分支（`NewBadRequestError(err.Error())`）原样进入管理员 400 响应，诊断信息完全不可读。该路径确定可达，应先判空再替换。

  		masked := maskEndpointCredentials(m.Transport.Endpoint)
- 		maskedReason := strings.ReplaceAll(reason.Error(), userinfoOf(m.Transport.Endpoint), "REDACTED")
+ 		maskedReason := reason.Error()
+ 		if ui := userinfoOf(m.Transport.Endpoint); ui != "" {
+ 			maskedReason = strings.ReplaceAll(maskedReason, ui, "REDACTED")
+ 		}
  		return fmt.Errorf("invalid transport endpoint %s: %s", echoQuoted(masked), echoQuoted(maskedReason))


─── internal/modules/plugins/plugintest/fakejira.go:98-108 ───
[test · low] DenySearch/InvalidateAccount/SetMyselfDelay 在 email 未命中任何账本时静默返回，与同包 AddAccount 对重复
email 的 t.Fatalf 装配错误面不一致。集成测试的 email/token
均随机生成（jira_e2e_integration_test.go），调用方拼写不一致时故障/延迟注入会静默失效，表现为下游断言失败而难以定位根因。建议与 AddOwner 同样接受
testing.TB 并在未命中时 Fatal（或返回 bool 由调用方 require）。

  // DenySearch 注入该账本 /search/jql 的拒绝状态码（403 无权限 / 401 失效）。
- func (j *FakeJira) DenySearch(email string, statusCode int) {
+ // email 未命中账本时 Fatal——与 AddAccount 的装配错误面一致，避免注入静默失效。
+ func (j *FakeJira) DenySearch(t testing.TB, email string, statusCode int) {
+ 	t.Helper()
  	j.mu.Lock()
  	defer j.mu.Unlock()
  	for _, account := range j.accounts {
  		if account.Email == email {
  			account.SearchDeny = statusCode
  			return
  		}
  	}
+ 	t.Fatalf("fakejira: DenySearch: no account registered for %s", email)
  }


─── internal/modules/plugins/plugintest/server.go:217-221 ───
[test · medium] 替身把 Tool.Call 的失败呈现为 isError=true
的成功结果（err==nil），与它声称对齐的示例服务错误面契约相反：examples/plugins/jira-todo-mcp 的 handleSearchMyWeek 对同样的 Jira
故障返回 (nil, err)（协议级 JSON-RPC error），其 server_test.go 明确断言 require.Error(err) +
require.Nil(result)。WeKnora 客户端（mcp_tool.go:273-307）对两条路径行为完全不同：协议 error 会断开连接并重试一次、最终经
oauthAwareConnectError 包装（错误文本含 "401" 时会被 isAuthorizationRequired 判定替换为 "requires OAuth
authorization" 引导文案）；isError 则不重试、原样透传文本。因此 jira_e2e_integration_test.go 的
jira-403/jira-timeout/jira-token-invalid 三个场景断言验证的是替身特有路径而非示例契约——尤其 jira-token-invalid 的
Contains("401") 断言，若替身忠实复刻示例错误面，真实链路会走 oauthAwareConnectError 替换文案而失败。T13 对 #106 验收边界
7（上游故障如实浮出）的集成验证因此失真。建议与示例对齐返回协议级 error；若担心通用替身的既有断言依赖 isError 面（当前仅 plugin_pg_integration_test.go
一处防御性分支返回 error，影响面小），可给 Tool 增加错误面开关并仅在 Jira 形替身开启，同时同步修正 e2e 断言。

  		result, err := call(member)
  		if err != nil {
- 			return sdkmcp.NewToolResultError(err.Error()), nil
+ 			// 与示例服务 handleSearchMyWeek 的错误面一致：协议级
+ 			// JSON-RPC error（mcp-go 转为 INTERNAL_ERROR 响应），
+ 			// 而非 isError=true 的成功结果——两条路径在 WeKnora
+ 			// 客户端的重试与文案包装行为不同。
+ 			return nil, err
  		}
  		return sdkmcp.NewToolResultText(result), nil


─── internal/modules/plugins/snapshot.go:339-343 ───
[security · medium] `DiffLiveAgainstSnapshot` 是唯一直接消费未受限 live 目录的路径，却没有施加同文件 `BuildVerifiedSnapshot`
/ `ValidateLiveDirectoryForRebase` 强制的 `maxLiveTools` 上限与 `validateName` 名单卫生——而 drift
的前提恰是"端点可能已变节"。已确认两条调用路径均无前置约束：`plugin_install_service.go` 的 `CheckDrift`（1507 行，经
`liveDirectoryFromAcceptedEndpoint` 裸透传）和 `markDriftBestEffort`（131 行，运行在成员目录加载路径上，10s list
超时只约束抓取、不约束本地 O(n) 的 `ToolSchemaDigest` 解析/重编码与建图）。后果：1) 恶意端点可返回海量工具+超大 schema 造成无界 CPU/内存放大；2) 未过滤的
Added/Removed/SchemaChanged 名单（可含控制/格式字符、超长名字）被 `json.Marshal` 持久化进
drift_detail、整段写入日志（`detail.Added=%v
...`）并返回给管理员，违背本文件其余路径自述的有界回显纪律（maxVerificationProblems）。另外同名重复条目在此处 last-wins，与
BuildVerifiedSnapshot 的"重复即自相矛盾目录、拒绝"语义不一致，可被用来让检测面静默取到与首条不同的 schema。建议入口先复用
ValidateLiveDirectoryForRebase（返回 error，调用方按"无法产出 drift 判定"处理），保持与 install/rebase 路径同一套卫生门。

- func DiffLiveAgainstSnapshot(live []*types.MCPTool, snap []types.PluginToolSnapshot) *types.PluginDriftDetail {
+ func DiffLiveAgainstSnapshot(live []*types.MCPTool, snap []types.PluginToolSnapshot) (*types.PluginDriftDetail, error) {
+ 	// 与 BuildVerifiedSnapshot/ValidateLiveDirectoryForRebase 同一套卫生门：
+ 	// 规模上限 + 名单卫生 + 重复名拒绝，drift 判定只从可信的 live 观察中铸出。
+ 	if err := ValidateLiveDirectoryForRebase(live); err != nil {
+ 		return nil, err
+ 	}
  	accepted := make(map[string]types.PluginToolSnapshot, len(snap))
  	for _, tool := range snap {
  		accepted[tool.Name] = tool
  	}


─── internal/router/routes_plugins.go:57-57 ───
[bug · medium] 策略端点对名字含「/」的工具永远不可寻址，逐工具治理承诺被静默架空。工具名校验门 validateName（manifest.go，仅拒绝控制/格式/私用区字符，≤128
runes）允许 `/`，含 `/` 的名字可完整通过预览核验并进入已接受快照、出现在 GET .../tools 治理列表中；但本路由按 gin 已解码的 URL.Path 匹配（客户端即使
encodeURIComponent 成 %2F，net/url 也会在 Path 中还原为 `/`），请求 /installations/x/tools/a/b/policy 比模板多一个路径段 →
直接 404 NoRoute，根本到不了 handler 的 ErrInstallationToolNotFound 检查。后果：这类只读分类工具默认 Enabled=true
且管理员无法逐项停用，require_approval 也永远无法设置（写类工具倒是 fail-closed 地无法启用）。建议在清单/实时目录校验门（validateName 或
BuildVerifiedSnapshot）拒绝含 `/`（以及 `%`）的工具名，使「已接受快照内的工具必然可经本策略端点寻址」成为安装时即成立的不变量（手工 MCP 路由
routes_infra.go:188 同形，但插件流程有安装校验门，具备修复位置）。



─── package.json:34-34 ───
[test · medium] 新增回归入口未接入任何门禁：scripts/run-gates.mjs 的 GATES 仅含
test:shared/typecheck:shared/test:web/typecheck:web/check:integrity/build:web 六项，且 .github/workflows
下无任何步骤引用 test:node-gte26（已全量检索确认）——F08/F10/F12 的回归保护在 CI 与 `pnpm gates` 中都不会执行，等于只可手动运行。建议将其加入
GATES（node --test 对 node>=18 均可运行，不受 MIN_MAJOR 影响）。

-     "test:node-gte26": "node --test scripts/lib/node-gte26.test.mjs",
+ // scripts/run-gates.mjs 中补充：
+ const GATES = [
+   ["test:node-gte26", "pnpm run test:node-gte26"],
+   ["test:shared", "pnpm run test:shared"],
+   // ...
+ ];


─── packages/api-client/src/plugins.ts:370-371 ───
[bug · high] service_id 用 required（拒绝空串）与后端可观测行为矛盾：GetInstallation/SetInstallationState →
installationResult 直接透传 inst.ServiceID（plugin_install_service.go:929），并无 GetMyConnectionStatus
那种空串孤儿反查自愈（985-1003 行）；同文件 parsePluginMyConnection 的 T12-OCR1-F3 注释也明确「confirm
在物化服务持久化之前中断的安装行，service_id "" 是合法 200」。后果：对这类中断残留行执行详情/停用/启用，后端正常返回 200 却被此处解析器抛成 "service_id must
be a non-empty string"，面板整卡失败——与 MyConnection 解析器刻意「降级不丢行」的设计意图相悖。建议与 parsePluginMyConnection 同口径改用
optionalText（或推动后端在 installationResult 补 healing）。

      endpointUrl: required(data.endpoint_url, `${INSTALLATIONS_PATH}.data.endpoint_url`),
-     serviceId: required(data.service_id, `${INSTALLATIONS_PATH}.data.service_id`),
+     // 与 parsePluginMyConnection 同口径：confirm 中断窗口的安装行
+     // service_id 合法为 ""（后端 installationResult 直接透传，无自愈）。
+     serviceId: optionalText(data.service_id, `${INSTALLATIONS_PATH}.data.service_id`),


─── packages/api-client/src/plugins.ts:128-132 ───
[other · medium] 升级闭环缺口：后端已有 POST
/installations/:id/upgrade-accept（routes_plugins.go、dto.PluginUpgradeAcceptRequest{candidate_fingerp
rint}、handler.AcceptUpgrade 返回 PluginInstallationResponse），但本域 API 只有 previewUpgrade、没有
acceptUpgrade，且两个面板（PluginsSettingsPanel/PluginsPanel）中也无任何 accept 调用——注释自述 "The accepted version
never changes server-side"，但整个前端没有任何途径接受候选版本，验收第 2 条「新版需展示差异并由管理员手动接受」在前端断在展示一步。建议补
acceptUpgrade(installationId, candidateFingerprint)（响应用 parsePluginInstallation
解析），或在接口注释中明确该端点所属的后续切片，避免被当作遗漏。

     * POST the bodyless read-only upgrade-preview (Admin): re-fetches the
     * installation's manifest source, verifies the candidate and resolves to the
     * five-dimension diff. The accepted version never changes server-side.
     */
    previewUpgrade(installationId: string, signal?: AbortSignal): Promise<PluginUpgradePreview>;
+   /** POST the reviewed candidate fingerprint (Admin): switches the accepted
+    * version (backend AcceptUpgrade; body {candidate_fingerprint}). */
+   acceptUpgrade(installationId: string, candidateFingerprint: string, signal?: AbortSignal): Promise<PluginInstallation>;


─── packages/api-client/src/plugins.ts:643-643 ───
[maintainability · medium] disabledReason
的解析与函数自身的严格契约矛盾：同函数其余字段全部硬校验、模块自述「拒绝任何缺失/畸形字段」（parsePluginPreview doc 注释），而后端 dto.DisabledReason 是非
omitempty 的 string（恒存在），非 string 值即契约破坏，此处却静默降级为 ''，畸形数据会以空串流入 UI 掩盖问题。建议改用 optionalText/required
显式抛错。另外 `${INSTALLATIONS_PATH}.data.${index}` 的点号路径风格与 parsePluginInstallations 的 `.data[${index}]`
方括号风格不一致，排障日志定位时会互相误导，建议统一。

-       disabledReason: typeof row.disabled_reason === 'string' ? row.disabled_reason : '',
+       disabledReason: optionalText(row.disabled_reason, `${path}.disabled_reason`),


─── packages/api-client/src/plugins.ts:271-272 ───
[documentation · low] enabled 的注释与类型描述的 wire 契约已过期：实际 dto.PluginInstallationTool.Enabled 是 `bool
json:"enabled"`（非指针、无 omitempty），handler 的 installationResponseDTO 已把无策略行解析为确定值（nil →
Enabled=ReadOnly）并恒输出该键；detail 载荷中被 omitempty 省略的是 require_approval（该文件已在别处正确处理）。「absent key =
unknown, kept as null」的描述对应的是 T18 统一前的旧契约，null 分支实际不可达——`boolean | null`
类型与注释会误导调用方为不可达分支写空值处理。建议类型收窄为 boolean、解析改用 flag，并更正注释为 T18 确定值语义。

-   /** dto Enabled *bool omitempty: absent key = no explicit policy row = unknown, kept as null. */
-   readonly enabled: boolean | null;
+   /** dto Enabled bool（恒输出）：T18 统一后为确定值——无显式策略行时 Enabled=ReadOnly。 */
+   readonly enabled: boolean;


─── packages/api-client/src/plugins.ts:93-94 ───
[maintainability · low] 六个解析器各自重复实现同一段信封校验（envelope.success !==
true），parsePluginPreview/parsePluginInstallation 又各有一份完全相同的 transport_type 白名单校验。建议提取
assertEnvelope(value, path) 与 transportTypeOf(data, path) 两个小工具复用——该模块定位是「wire-format
解析的唯一来源」，新增端点时复制粘贴漂移的风险会随解析器数量增长。

-   const envelope = record(value, PREVIEW_PATH);
-   if (envelope.success !== true) throw new Error(`${PREVIEW_PATH}.success must be true`);
+ function assertEnvelope(value: unknown, path: string): RecordValue {
+   const envelope = record(value, path);
+   if (envelope.success !== true) throw new Error(`${path}.success must be true`);
+   return envelope;
+ }
+ 
+ function transportTypeOf(data: RecordValue, path: string): 'http-streamable' | 'sse' {
+   const transportType = required(data.transport_type, `${path}.transport_type`);
+   if (transportType !== 'http-streamable' && transportType !== 'sse') {
+     throw new Error(`${path}.transport_type is invalid`);
+   }
+   return transportType;
+ }


─── packages/api-client/src/plugins.ts:144-144 ───
[bug · medium] PluginsApi 缺失 drift 治理端点族,漂移处置闭环在前端断裂:后端 routes_plugins.go(L47-49)提供 GET
/installations/:id/drift(Viewer)、POST .../drift/check(Admin)、POST .../drift/resolve(Admin),并配有
dto.PluginDriftReportResponse(detail 含 added/removed/schema_changed/description_changed 工具名清单 +
checked_at,以及 snapshot_tool_names 基线)。但本域 API(自述为插件 wire 解析的唯一来源)没有这三个端点的任何方法;PluginsPanel.tsx L215
与 PluginsSettingsPanel.tsx L526 也只是把 driftState==='detected' 渲染成只读徽章,无 check/resolve 入口。后果:①验收
3「运行时以已接受快照为基准阻止能力漂移」与验收 6「候选版本不可达等状态有正确行为」要求的漂移明细查看与治理动作(复检/恢复)在前端均不可达,徽章一旦出现除
upgrade-accept(前端同样无入口,见已确认发现 2)外无路径消除;②漂移明细(哪些工具新增/移除/schema 变化)永远到不了管理员评审面。建议补齐三个方法及
PluginDriftReport 解析器,并在治理面板接入。

    listInstallationTools(installationId: string, signal?: AbortSignal): Promise<PluginToolPolicyRow[]>;
+   /** GET the persisted drift report (Viewer+): drift_state, per-form tool-name detail and the accepted snapshot's tool-name baseline. */
+   getInstallationDrift(installationId: string, signal?: AbortSignal): Promise<PluginDriftReport>;
+   /** POST the admin drift re-check: re-fetches the manifest source and updates the persisted drift state. */
+   checkInstallationDrift(installationId: string, signal?: AbortSignal): Promise<PluginDriftReport>;
+   /** POST the admin drift resolve: re-accepts the live directory as the new baseline. */
+   resolveInstallationDrift(installationId: string, signal?: AbortSignal): Promise<PluginDriftReport>;


─── packages/views/src/integrations/messages.ts:79-79 ───
[documentation · low] subtitle 的治理语义在非中文 locale 丢失：zh-CN 明确「写入类工具默认关闭」（验收第 5
条的关键治理承诺），en-US（ja-JP/ko-KR/ru-RU 同样）只译出「只能调用已接受版本中启用的工具」，未提写工具默认关闭——非中文用户看到的治理口径弱一档。建议四个 locale
补齐对应表述。

-     'integrations.plugins.subtitle': 'Plugins installed in this workspace; agents can only call tools enabled in the accepted versions.',
+     'integrations.plugins.subtitle': 'Plugins installed in this workspace; agents can only call tools enabled in the accepted versions, and write tools are disabled by default.',


─── packages/views/src/integrations/page.tsx:151-153 ───
[documentation · low] 该 JSDoc 中「在接线落位前,registry 的 plugins section 只会渲染 heading+description
空壳」已过期:同一变更集的 apps/web/src/integrations/IntegrationsRoutePage.tsx(L217)已经传入
pluginsSlot={<PluginsPanel client={client} />},接线已落位。这与 view.ts 中已被指认的过期注释(已确认发现
7)同源——留着这句会让后续维护者误以为插槽接线仍是待办。建议改述为「未传入插槽的复用方(如纯 packages 层测试)」或直接删除该过渡态描述。

-    * packages/views），面板由 apps/web 的集成路由页经此插槽传入——在接线落位
-    * 前，registry 的 plugins section 只会渲染 heading+description 空壳。
+    * packages/views），面板由 apps/web 的集成路由页经此插槽传入——集成路由页
+    * 已传入 PluginsPanel；仅当复用方未传插槽时，该 section 才渲染 heading+description 空壳。
     */


─── packages/views/src/integrations/view.ts:51-54 ───
[documentation · low] 该注释已过期：② 声称「plugins tab body 在 page.tsx 尚无挂载分支」，但本次 page.tsx 已新增 `tab ===
'plugins' ? pluginsSlot ?? null` 插槽分支，且 apps/web 的 IntegrationsRoutePage 已传入 PluginsPanel；① 描述的
SettingsPage 侧边栏裸 key 也已由本变更集的 settingsSectionLabel 直译兜底解决。留着这段会误导后续任务交接判断（让人以为插槽接线仍是待办），建议删改。



─── scripts/lib/node-gte26.mjs:84-85 ───
[bug · medium] win32 下 shim 条目名缺少 ".exe" 扩展名：cmd.exe 的 PATH 解析基于
PATHEXT（.COM/.EXE/.BAT/.CMD），不会命中无扩展名文件——tsx.cmd 等子进程内部调用 `node` 时将绕过 shim、继续解析到 PATH 上的旧版 node，shim
对这类子进程静默失效（复制回退路径同样受影响）。同文件 findNodeGte26 的 $PATH 扫描以 path.join(dir, "node") 拼接候选，win32 上应拼
"node.exe"，否则永远探测不到。建议按平台决定条目名。

    const shimDir = fs.mkdtempSync(path.join(os.tmpdir(), "weknora-node26-shim-"));
-   const shimNode = path.join(shimDir, "node");
+   // win32 的 cmd/PATHEXT 只解析带扩展名的可执行文件；无 .exe 的 shim 会被 tsx.cmd 等子进程静默绕过
+   const shimNode = path.join(shimDir, process.platform === "win32" ? "node.exe" : "node");


─── scripts/lib/node-gte26.mjs:86-86 ───
[maintainability · low] 死代码:上一行 mkdtempSync 刚创建的全新空目录中,shimNode 必然不存在,这行 rmSync
永远不会删除任何东西。它是旧方案(固定目录名 weknora-gates-node-shim,可能残留上次崩溃的符号链接)的遗留防御代码,与 F10 的安全语义——"mkdtempSync
目录必然私有且为空"——相矛盾,保留会误导读者以为 shim 目录可能被预置内容。建议删除。

-   fs.rmSync(shimNode, { force: true });
+   // mkdtempSync 创建的目录必然为空,无需先删除 shimNode
+   try {
+     linker(targetBin, shimNode);


─── scripts/lib/node-gte26.test.mjs:37-42 ───
[test · medium] 该测试在 win32 上确定性 false-red：existingPath 用了字面 POSIX 串 "/usr/bin:/usr/local/bin"（win32
下 `:` 只是普通字符），joinShimPath 以 ';' 连接后 split(';') 的 parts[1] 是 "/usr/bin:/usr/local/bin" 而非
"/usr/bin"；末尾的 `!joined.includes(":")` 断言在 win32 上三条件全假。另外同文件 "createNodeShim symlinks the target by
default" 未对 win32 跳过，非开发者模式下真实 fs.symlinkSync 会直接抛
EPERM（正是回退测试模拟的场景）。本套件定位为跨平台回归（T01-OCR1-F12），自身却不能在 win32 通过，建议改用 path.join 构造平台无关输入，并对依赖真实 symlink
的用例加 win32 skip/注入 linker。

  test("joinShimPath puts the shim dir first and uses the platform delimiter", () => {
-   const joined = joinShimPath("/tmp/shim-dir", "/usr/bin:/usr/local/bin");
-   assert.ok(joined.startsWith("/tmp/shim-dir" + path.delimiter), `shim dir must lead: ${joined}`);
+   const dirA = path.join(os.tmpdir(), "dir-a");
+   const dirB = path.join(os.tmpdir(), "dir-b");
+   const joined = joinShimPath(dirA, [dirA, dirB].join(path.delimiter));
    const parts = joined.split(path.delimiter);
-   assert.equal(parts[0], "/tmp/shim-dir");
-   assert.equal(parts[1], "/usr/bin");
+   assert.equal(parts[0], dirA);
+   assert.equal(parts[1], dirB);


─── scripts/run-gates.mjs:62-62 ───
[bug · medium] 与 scripts/run-with-node-gte26.mjs 相同的 win32 大小写重复键问题:`{ ...process.env, PATH: ... }`
在 Windows 上会同时产生 `Path=<旧值>`(展开物化的原始键)与 `PATH=<shim;旧值>` 两个条目,子进程环境块同名(不区分大小写)重复行为未定义、通常首个生效——shim
前置对 re-exec 子进程静默失效。即使已确认的 #2(shim 缺 .exe)修复后,该问题仍会独立存在。建议复用与 wrapper 相同的共享 helper(见
run-with-node-gte26.mjs 处的建议 shimEnv()),消除两个调用点的重复实现。

-     PATH: joinShimPath(shimDir, process.env.PATH),
+   const env = shimEnv(shimDir); // node-gte26.mjs 导出:按大小写无关方式覆盖既有 PATH 变体键
+   // (原 { ...process.env, PATH: joinShimPath(...) } 在 win32 会产生 Path/PATH 重复条目)


─── scripts/run-with-node-gte26.mjs:76-76 ───
[bug · high] win32 回归：command 为 `tsx`，pnpm 在 Windows 上将 bin 以 `tsx.cmd` 分发，spawnSync 未设置 shell 时无法执行
.cmd/.bat（libuv 不做 PATHEXT 解析，Node 对 .cmd 显式抛 EINVAL/ENOENT）。改造前 `tsx --test ...` 由 pnpm 经 shell
执行可正常命中 tsx.cmd；本包装器使 `pnpm test:shared` / `test:craft:shared` / `test:mobile-v2` 在 Windows
上确定性失败，与本模块 T01-OCR1-F12 声明的 win32 兼容目标直接矛盾。建议在 win32 下启用 shell（或显式解析 tsx.cmd 全路径）。

-   const result = spawnSync(command, args, { stdio: "inherit", env });
+   const result = spawnSync(command, args, {
+     stdio: "inherit",
+     env,
+     // pnpm 在 win32 以 tsx.cmd 分发 bin，无 shell 时 spawn 无法执行 .cmd（ENOENT/EINVAL）
+     shell: process.platform === "win32",
+   });


─── scripts/run-with-node-gte26.mjs:60-61 ───
[maintainability · low] 此错误文案与 scripts/run-gates.mjs 第 79 行逐字重复（仅前缀标签不同）——F11 抽取共享模块的目的正是消除这种复制漂移（前次
F09 缺口即源于两份拷贝失同步）。两处文案/发现顺序再度演进时极易只改其一。建议在 node-gte26.mjs 导出统一的 notFoundMessage()（或
noNodeFoundHint(currentVersion)）供两处调用。

-         `[with-node] node >= ${MIN_MAJOR} required (current ${process.version}); ` +
-           "no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)",
+ // scripts/lib/node-gte26.mjs 导出：
+ export function nodeNotFoundMessage(currentVersion) {
+   return `node >= ${MIN_MAJOR} required (current ${currentVersion}); no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)`;
+ }
+ // 两处调用方改为：
+ console.error(`[with-node] ${nodeNotFoundMessage(process.version)}`);


─── scripts/run-with-node-gte26.mjs:74-74 ───
[bug · medium] win32 环境变量大小写陷阱(与已确认的 #2 缺 .exe、#4 .cmd 无法 spawn 是不同根因):在 Windows 上 process.env
的实际存储键名是 `Path`(大小写保留),`{ ...process.env }` 会先物化出 `Path=<旧值>`,随后赋值 `PATH:` 在普通对象上又新增一个独立的
`PATH=<shim;旧值>` 键——子进程环境块中出现两个不区分大小写的同名条目属于未定义行为(通常取首个,即旧 `Path`),shim 前置会被静默忽略。本模块本轮明确以 win32
兼容为目标(T01-OCR1-F12),建议在 node-gte26.mjs 导出统一的 shimEnv() helper(按大小写无关方式覆盖既有 PATH 变体键)供 run-gates.mjs
与本文件共用,避免三处(含将来新增调用方)各自实现漂移。

-   const env = { ...process.env, PATH: joinShimPath(shimDir, process.env.PATH) };
+ // scripts/lib/node-gte26.mjs 中新增并导出:
+ export function shimEnv(shimDir) {
+   const env = { ...process.env };
+   const joined = joinShimPath(shimDir, process.env.PATH);
+   for (const key of Object.keys(env)) {
+     if (key.toUpperCase() === "PATH") {
+       env[key] = joined; // 覆盖 win32 上既有的 "Path" 键,避免大小写重复条目
+       return env;
+     }
+   }
+   env.PATH = joined;
+   return env;
+ }
+ 
+ // 本文件调用处:
+ const env = shimEnv(shimDir);


─── apps/web/src/settings/PluginsSettingsPanel.tsx:197-199 ───
[bug · medium] client 变化失效不完整：SettingsPage 的 section div 仅以 key={key}
挂载（SettingsPage.tsx:498/504），client 切换不触发面板重挂载，本 effect 是唯一的失效点——但它只清 toolPolicy 并自增
epoch，preview（含可消费的 previewId）、confirmError、upgradePreview、upgradeError
均留存。跨账号/工作区切换后，旧空间的预览卡与升级差异面板仍显示在新 client 视图下，且「确认安装」会以新 client POST 旧空间的 previewId。既然 toolPolicy
已按此威胁模型处理，建议在同一 effect 中同步清空这些临时态（另建议 confirmInstall 成功后也清 upgradePreview，避免安装后残留安装前的差异面板）。

      setToolPolicy(null);
+     // client 切换同样使预览态失效：旧空间的 previewId/升级差异不得在新 client 下展示或消费。
+     setPreview(null);
+     setConfirmError(null);
+     setUpgradePreview(null);
+     setUpgradeError(null);
      // 代数自增（T19-OCR2-F1）：已在途的 GET 的迟到回包凭旧代数被丢弃。
      toolPolicyEpoch.current += 1;


─── apps/web/src/integrations/PluginsPanel.tsx:130-132 ───
[maintainability · low] retryConnection 中 delete 后立即 add 是净空操作：重试按钮仅在 entry === "load-failed"
时可见，而该墓碑键只会在本 id 已发起过请求（即已加入 requestedRef）后落下，delete(id) + add(id) 对集合无任何效果；注释宣称的「清墓碑」实际由
loadConnection 覆盖 connections[id] 完成，墓碑并不在 requestedRef 里。这两行是误导性死代码，建议删除并修正注释指向真实机制。

      setPendingId(installationId);
-     requestedRef.current.delete(installationId);
-     requestedRef.current.add(installationId);
+     // 墓碑键位于 connections state，由随后的 loadConnection 覆盖清除；
+     // requestedRef 仅承担首请求去重，重试无需改写集合。
+     void loadConnection(installationId).finally(() => {
+       setPendingId(null);
+     });


─── apps/web/src/integrations/PluginsPanel.tsx:267-272 ───
[style · low] 徽标判定为两级嵌套三元（authorized / expired / 兜底），违反「禁止嵌套三元」基线；且 state 是封闭三态字面量联合，查表更直接。

-   const badge =
-     connection.state === "authorized"
-       ? { className: pluginBadgeOk, label: "已授权" }
-       : connection.state === "expired"
-         ? { className: pluginBadgeWarn, label: "已过期" }
-         : { className: pluginBadgeMuted, label: "未授权" };
+   const CONNECTION_BADGES: Record<PluginMyConnection["state"], { className: string; label: string }> = {
+     authorized: { className: pluginBadgeOk, label: "已授权" },
+     expired: { className: pluginBadgeWarn, label: "已过期" },
+     unauthorized: { className: pluginBadgeMuted, label: "未授权" },
+   };
+   const badge = CONNECTION_BADGES[connection.state];


─── apps/web/src/integrations/PluginsPanel.tsx:49-53 ───
[maintainability · low] 死参数：initialInstallations 在唯一生产挂载点（IntegrationsRoutePage 的
pluginsSlot）未被传入，面板挂载即自拉全量列表，该 prop 当前无任何生产消费方，属推测性 API 面。若近期无 SSR/首屏直出接线计划，建议移除以缩小面板契约。

  type Props = {
    client: WeKnoraClient;
-   /** SSR/首屏直出数据；挂载后仍会经 client 刷新（GET /plugins/installations，Viewer+）。 */
-   initialInstallations?: readonly PluginInstallationSummary[];
  };


─── apps/web/src/settings/PluginsSettingsPanel.tsx:37-39 ───
[maintainability · low] 三处重复：pluginBadgeOk/Info/Warn/Muted 四组常量与 McpSettingsPanel 的 mcpBadge*（第 24
行起）逐字相同，本文件与 PluginsPanel.tsx 又各复制一份；ApiError 分类逻辑（cause.name === "ApiError" ? message :
中文兜底）同样在本文件成助手、在 PluginsPanel 内联两套写法。样式或接口漂移时三处难以同步，建议提取到共享模块（如 apps/web 内 shared 样式/错误工具，或
@weknora/ui 导出）。



─── apps/web/src/settings/PluginsSettingsPanel.tsx:282-283 ───
[style · low] toggleState 末尾 `}  }` 双右括号连写（finally 结束括号与函数结束括号之间双空格），与文件其余函数排版不一致，建议按 prettier 口径拆行。

        setActionBusyId(null);
-     }  }
+     }
+   }


─── apps/web/src/settings/PluginsSettingsPanel.tsx:552-558 ───
[maintainability · low] 「停用/启用」按钮缺少跨行禁用：同组的「工具治理」「检查升级」均对其他行在途操作设 disabled，而本按钮仅
loading——actionBusyId 指向其他行时点击会被 toggleState 首行守卫静默吞掉，UI 无任何冻结反馈，交互口径不一致。

                      <Button
                        type="button"
                        loading={actionBusyId === item.installationId}
+                       disabled={actionBusyId !== null && actionBusyId !== item.installationId}
                        onClick={() => void toggleState(item)}
                      >
                        {item.state === "active" ? "停用" : "启用"}
                      </Button>


─── apps/web/src/settings/PluginsSettingsPanel.tsx:509-512 ───
[style · low] 空态判定为嵌套三元（length === 0 套 listError === null），违反基线；同批 PluginsPanel 的对应空态用 `length === 0
&& error === null` 单层实现，两个新面板口径不一致，建议统一为 && 形式并把列表渲染独立出来。

-         {installations.length === 0 ? (
-           listError === null ? <Status>暂无已安装插件</Status> : null
-         ) : (
+         {installations.length === 0 && listError === null ? <Status>暂无已安装插件</Status> : null}
+         {installations.length > 0 ? (
            <ul className="m-0 grid list-none gap-2 p-0">


─── apps/web/src/integrations/PluginsPanel.tsx:103-110 ───
[bug · medium] client 变化时连接状态缓存不失效：IntegrationsRoutePage 的数据 effect 依赖 [client, tab,
activeTenantId]（IntegrationsRoutePage.tsx:118），即 client 引用可在页面不重挂载的情况下变化；pluginsApi 随 client
重建后列表会刷新，但 requestedRef 去重集仍持有全部 installationId，本 effect 对每行直接 continue——旧 principal 的「已授权/已过期」徽标与
serviceId 将原样残留展示在新 client 视图下，直到用户手动操作才被覆盖。同批 PluginsSettingsPanel 为同类场景做了显式失效（setToolPolicy(null)
+ toolPolicyEpoch 自增），本面板的 connections/requestedRef 却没有任何失效路径，口径不一致。建议在 pluginsApi 变化时重置
requestedRef 与 connections。

+   // client/pluginsApi 变化：旧 principal 的连接缓存整体失效——重置去重集
+   // 并清空 connections，让下方 effect 以新 client 重发（挂载首跑为空操作）。
+   useEffect(() => {
+     requestedRef.current = new Set();
+     setConnections({});
+   }, [pluginsApi]);
+ 
    useEffect(() => {
      for (const row of installations) {
        if (!row.requiresPersonalAuth) continue;
        if (requestedRef.current.has(row.installationId)) continue;
        requestedRef.current.add(row.installationId);
        void loadConnection(row.installationId);
      }
    }, [pluginsApi, installations]);


─── apps/web/src/integrations/PluginsPanel.tsx:280-288 ───
[bug · low] 跨行 pending 静默吞点击：pendingId 是全面板单锁，authorizeConnection 轮询最长持有约 60 秒（AUTH_POLL_ATTEMPTS 40
× 1.5s），期间其他行的「撤销/去授权/重试」按钮因 pending 仅在本行 id 匹配时为 true 而呈现可点，但点击会被
authorizeConnection/revokeConnection/retryConnection 首行的 `if (pendingId !== null) return;`
静默吞掉，用户零反馈且易误以为授权已发起。与 PluginsSettingsPanel 中「工具治理/检查升级」按钮的跨行 disabled
口径不一致（同批已确认发现的同构问题），建议任一操作在途时禁用所有行的入口按钮。

-         <Button
-           type="button"
-           className="h-7 rounded-[6px] px-3 text-[12px]"
-           disabled={pending || noService}
-           title={noService ? "插件服务尚未就绪，请稍后重试" : undefined}
-           onClick={() => void onRevoke(connection)}
-         >
-           撤销
-         </Button>
+ <MemberConnectionControl
+   installationId={item.installationId}
+   entry={connections[item.installationId]}
+   pending={pendingId === item.installationId}
+   anyPending={pendingId !== null}
+   ...
+ />
+ 
+ // MemberConnectionControl 内：
+ disabled={pending || anyPending || noService}


─── apps/web/src/settings/PluginsSettingsPanel.tsx:351-352 ───
[maintainability · low] policyToggleBusy 组装的 `${row.name}:${field}` 键是死数据：全文件对该 state 只消费
`policyToggleBusy !== null`（守卫与两个开关的 disabled），字符串值从未被读取渲染——既没有行内 loading 指示，也没有据此定位哪个开关在途。要么改用
boolean（并另行补充行内 loading 反馈），要么用该键渲染被切换开关的加载态，避免维护者误以为存在按开关粒度的反馈。

      if (!canEdit || policyToggleBusy !== null || toolPolicyBusyId !== null) return;
-     setPolicyToggleBusy(`${row.name}:${field}`);
+     // 仅作全局在途标记：值无消费方，不组装键。
+     setPolicyToggleBusy("busy");
+     // 或保留键名并在开关上渲染：loading={policyToggleBusy === `${row.name}:${field}` }


─── internal/application/service/plugin_install_service.go:1228-1230 ───
[bug · medium] 升级接受路径缺少候选端点的 512 字符长度校验：PreviewFromManifest 在持久化前用 validatePluginURLLength 拦截超长
endpoint（注释明确是为了避免 endpoint_url 列溢出被误报为 500），且已确认 plugins.ValidateManifest 只校验
scheme/host、不校验长度。AcceptUpgrade 中 candidateEndpoint 直接进入
UpdateInstallationAccepted（plugin_installations.endpoint_url varchar(512)）和 svc.URL（mcp_services.url
varchar(512)）：远端清单声明一个 >512 rune 但可正常 ListTools 的端点时（可通过指纹守卫，因 PreviewUpgrade 同样不查长度），PostgreSQL 上
7a 首写即报 "value too long for type character varying(512)"，被包装成 ErrInstallationPersistFailed →
确定性输入问题误报为 500；SQLite 则静默落库超出列定义边界的数据。建议在指纹守卫之后、任何写入之前补上与预览路径一致的门禁（PreviewUpgrade
同步加可让管理员在预览期即见拒绝）。注意升级路径经 mapPluginInstallationError 映射、其 default 分支是 500，因此需挂到已映射为 400 的哨兵上而不是返回裸
fmt.Errorf。

+ 	if err := validatePluginURLLength("plugin transport endpoint", candidateEndpoint); err != nil {
+ 		return nil, fmt.Errorf("%w: %v", ErrPluginVerifyFailed, err)
+ 	}
  	candidateVersion := result.Manifest.Version
  	candidateEndpoint := result.Manifest.Transport.Endpoint
  	candidateSnapshot := types.PluginPreviewTools(result.Snapshot)


LLM retry report summary: 2 of 79 requests affected -- 2 requests failed

Review planning (1 request):
- internal/application/repository/plugin.go,internal/application/service/plugin_install_service.go,internal/application/service/plugin_service.go,internal/container/container.go,internal/container/plugin_lister.go,internal/types/interfaces/plugin.go,internal/types/plugin.go,internal/utils/digest.go: timed out -> failed

Core review (1 request):
- internal/application/repository/plugin.go,internal/application/service/plugin_install_service.go,internal/application/service/plugin_service.go,internal/container/container.go,internal/container/plugin_lister.go,internal/types/interfaces/plugin.go,internal/types/plugin.go,internal/utils/digest.go: timed out -> failed

Per-attempt detail: --format json (retry_report).
