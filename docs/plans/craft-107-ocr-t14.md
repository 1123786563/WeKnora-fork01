Review complete: 49 finding(s) across 93 selected item(s).

─── apps/desktop/vite.config.ts:71-71 ───
[bug · medium] 别名 '@weknora/views/craft/access' 被追加到裸前缀别名 '@weknora/views'（第 69 行，指向
views/src/index.ts）之后，违反了本文件第 16-18 行与 56-58 行注释明确记录的排序约定：Vite/rollup 对字符串别名按插入顺序做「精确或前缀（find +
'/'）」匹配、首个命中即生效。因此该 specifier 会先被 '@weknora/views' 前缀匹配捕获并改写为 '<views
index.ts>/craft/access'（不存在的路径），本条目永远不会生效，一旦桌面端模块图引入该导入即报 ENOTDIR 解析失败。目前引用点仅在
apps/web/src/features/craft/routes.tsx，桌面端暂未触达，属潜伏缺陷，但仍应与 apps/web/vite.config.ts（第 115
行，置于前缀条目之前）保持一致，将此条目上移到 craft 子路径别名组内、'@weknora/views' 之前。

+       '@weknora/views/craft/interaction': fileURLToPath(new URL('../../packages/views/src/craft/interaction.tsx', import.meta.url)),
        '@weknora/views/craft/access': fileURLToPath(new URL('../../packages/views/src/craft/access.tsx', import.meta.url)),
+       '@weknora/views': fileURLToPath(new URL('../../packages/views/src/index.ts', import.meta.url)),


─── cmd/craft-egress-adapter/main.go:28-28 ───
[other · medium] 集成缺口：本变更集交付了适配器二进制，但全仓检索显示 craftegress 包在生产代码中的唯一消费者就是本
main.go——docker/craft/Dockerfile 未构建该二进制，无任何组件注入 CRAFT_EGRESS_* 环境变量，也没有沙箱组装代码把 OpenCode 的 provider
端点指向 127.0.0.1:8787。而网关 Forward 在同一变更集中改为 fail-closed（缺少 X-Craft-Activity-ID 即 400
ACTIVITY_ID_REQUIRED），若按本变更集部署且无适配器在沙箱内运行，Craft 运行的所有模型转发都会被拒，模型出网端到端不可用。请确认 T19 lane 后续 ticket
包含沙箱侧部署接线（镜像内构建、每 Run journal 路径与凭证注入、provider 配置指向），否则此二进制为死代码。



─── docker/craft/web/build.py:345-351 ───
[security · high] 事件属性防线实际未接线：EVENT_ATTR_RE 在模块顶部定义并自称 cheap pre-filter，但 render_html 的 pattern
元组里没有它（全文件零调用点，纯死代码）；同时 _EventAttrScanner 基于 HTMLParser，对未闭合标签（如 `<img src=x onerror=alert(1)`，缺结尾
`>`）在 feed+close 时走 handle_data 分支、既不触发 handle_starttag 也不抛异常，scanner 静默放行。该片段可同时绕过其余五个正则（无 //、无
scheme、无 url(、img 不在 EMBED_TAG_RE 词表），最终经 `"<h2>{}{}"` 原样拼入 index.html——staged fragment 是不受信任的 agent
产物，预览 origin 与导出的静态 HTML（无预览 CSP）下均构成存储型 XSS。至少应把 EVENT_ATTR_RE 加入元组（`<img src=x onerror=` 形态
`[^>]*?\son[a-z]+=` 可命中，无需闭合 `>`）。

          for pattern, why in (
              (EXTERNAL_URL_RE, "external URL"),
              (ABSOLUTE_REF_RE, "absolute or scheme reference"),
              (ACTIVE_DATA_RE, "active data/javascript URI"),
              (CSS_FETCH_RE, "css url()/@import fetch"),
              (EMBED_TAG_RE, "embedding/script/navigation tag"),
+             (EVENT_ATTR_RE, "inline event handler attribute"),
          ):


─── docker/craft/web/build.py:96-100 ───
[security · high] 结构化扫描器存在未闭合标签盲区：CPython HTMLParser 在 close()（goahead
end=1）对不完整标签按数据回调处理，handle_starttag 永不触发、也不抛异常，fail-closed 的 except 形同虚设。而浏览器解析嵌入页面的片段时，会用后续 markup
的 `>` 把悬空标签闭合（本模板中 `</main>`/`</body>` 的 `>` 都可充当），onerror 属性照常生效。修复：按浏览器等价行为给片段补一个哨兵 `>`
强制闭合，使悬空标签产生 starttag 事件被扫描到（顺带覆盖 `<img/onerror=…` 这类正则也不命中的斜杠分隔变体）；或在 close() 后断言 scanner.rawdata
为空，残留即拒绝。

  def fragment_has_event_attrs(fragment: str) -> bool:
      scanner = _EventAttrScanner()
      try:
-         scanner.feed(fragment)
+         # Force-close any dangling tag exactly like the browser will against
+         # the following page markup, so an unclosed "<img src=x onerror=..."
+         # still produces a starttag event instead of trailing raw data.
+         scanner.feed(fragment + ">")
          scanner.close()


─── docker/craft/web/build.py:65-65 ───
[security · high] URL scheme 检测可被控制字符走私绕过：浏览器 URL 解析器（WHATWG URL Standard）在解析 scheme 之前会先剔除 URL 内所有
ASCII tab/换行（U+0009/U+000A/U+000D）并修剪首尾 C0 控制字符，而 render_html 的变体闭包只叠加了 HTML 实体与 CSS
转义两种解码，没有任何一步模拟这一规范化。攻击片段 `href="jav&#9;ascript:alert(1)"` 经 unescape 得到 `jav\tascript:`
后：EXTERNAL_URL_RE 无 `//`/`://` 不命中；ABSOLUTE_REF_RE 的 scheme 模式 `[a-zA-Z][a-zA-Z0-9+.-]*:` 被 tab
截断不命中；ACTIVE_DATA_RE 要求字面 `javascript:` 也不命中 —— 片段被原样渲染进 index.html，用户在预览 origin
点击即执行脚本。`data&#10;:text/html`、`&Tab;`/`&NewLine;` 命名实体同理。建议在变体闭包中追加一个"浏览器视图"变体：对每个 candidate 剔除 C0
控制字符（至少 [\t\n\r]，fail-closed 方向可取 [\x00-\x1f\x7f]）后同样送入全部 denylist 与结构扫描，使走私变体落入被检测形态。

  ACTIVE_DATA_RE = re.compile(r"data:text/html|javascript:", re.IGNORECASE)
+ # Browsers strip ASCII tab/newline from URLs and trim leading/trailing C0
+ # control/space BEFORE scheme parsing (WHATWG URL Standard), so a smuggled
+ # control char (jav&#9;ascript:, data&#10;:text/html) resolves to an active
+ # scheme. Screen a control-stripped "browser view" of every variant too.
+ URL_CTRL_RE = re.compile(r"[\x00-\x1f\x7f]")
+ 
+ # 并在 render_html 的变体闭包中：
+ #         queue.append(html_mod.unescape(candidate))
+ #         queue.append(css_unescape(candidate))
+ #         queue.append(URL_CTRL_RE.sub("", candidate))


─── internal/application/repository/craft_run_capture.go:349-353 ───
[performance · low] 新增的 per-run 回收查询 `tenant_id = ? AND run_id = ? AND state IN (...)` 缺少匹配索引：表主键为
(tenant_id, workspace_id, run_id)，本查询未携带 workspace_id，只能走 tenant_id 前缀扫描；现有二级索引
idx_craft_run_captures_recovery(state, updated_at) 也无法服务租户+Run 定位。同时已 advanced 的 receipt
行永久保留（无清理路径），租户内行数随 Craft Run 数量无界增长，而该查询在每次 Run 终态 drain 时执行，扫描代价随之线性增长。建议在迁移中补充 (tenant_id,
run_id)（或 (tenant_id, run_id, state)）复合索引支撑该点查。



─── internal/application/service/craft_delegate.go:385-387 ───
[bug · medium] writeAuditRow 直接以 p.scope.UserID 写 ActorUserID，未经过同包
craftAuditActorUserID（craft_access.go 本轮为同一问题新增：audit_logs.actor_user_id 为 VARCHAR(36)，合成 API-key 主体
api_external_user:<tenant>:<extid> 可超长）。Craft 路由挂在 API-key 守卫下，此类主体可真实到达 MaterialPolicy；超长主体在
Postgres 上触发 'value too long for type character varying(36)'，审计行永远写不进（best-effort 仅记 Warn），T03 的
craft.input.read / craft.generated.execute / craft.input.execute_denied 对这些主体恰好全部丢失——与本组
craft_access.go 的修复口径不一致。同包函数可直接复用，建议对 actor 做同样的哈希截断并把完整形态放入 Details。

+ 	actor, actorFull := craftAuditActorUserID(p.scope.UserID)
+ 	detailsMap := map[string]string{
+ 		"run": p.runID, "workspace": p.workspaceID,
+ 		"target": target, "digest": digest, "reason": reason,
+ 	}
+ 	if actorFull != "" {
+ 		detailsMap["actor_user_id_full"] = actorFull
+ 	}
+ 	details, err := json.Marshal(detailsMap)
+ 	if err != nil {
+ 		return
+ 	}
  	entry := &types.AuditLog{
- 		TenantID: p.scope.TenantID, ActorUserID: p.scope.UserID,
+ 		TenantID: p.scope.TenantID, ActorUserID: actor,
  		Action: types.AuditAction(kind), Outcome: outcome,


─── internal/application/service/craft_preview.go:517-522 ───
[performance · medium] lookup 对每个静态资源请求（favicon、css、js、图片——一个预览页面轻松产生几十个请求）都重新执行
RequireTaskAccess（成员库查询）+ CheckPreviewNoEgress（binding 查询 + 沙箱配置加载 + 对本地 Docker daemon 的实检，单次上限
30s），且确认全链无任何结果缓存/去重（craftPreviewDockerClients 只缓存客户端连接）。正常情况下 unix socket 实检是毫秒级，但 daemon
抖动/卡顿时每个资源最多钉住 worker 30 秒并被资产数量放大；allowlist 短路只覆盖必然 404 的请求，命中清单的文件仍逐个实检。建议对 (scope) 维度引入短 TTL 的
no-egress 判定缓存（如以 capability 的短生命周期为上界），在新鲜度与放大之间折中；至少考虑对同一 capability 兑换窗口内的连续资源请求去重实检。



─── internal/application/service/craft_web_screen.go:125-129 ───
[security · high] 筛查谓词仅匹配 .html/.htm，但威胁模型（本文件头注释：Agent 可伪造 output
树任意文件）下，浏览器会解析执行脚本的其他成员完全绕过该服务端防线：.svg 经 craftArtifactMIME 得到 image/svg+xml、.xhtml 得到
application/xhtml+xml，预览 lookup（craft_preview.go: lookup 逐版本 manifest 文件服务）按该 MIME 透传，且 PreviewCSP 为
script-src 'self' 'unsafe-inline' —— 内联脚本/onload 在直接导航 /p/<cap>/evil.svg 时执行。已核实 web kind 无 manifest
门（craftManifestGateKind 对 web 返回 false），stageAndUpload 全量收集 output 目录，伪造产物只需附带一个通过筛查的干净
index.html（四检查照常通过）加一个带脚本的 evil.svg 即可让未筛查的主动内容进入已发布版本并在预览 origin 执行——这正是本筛查声称要拒绝的"script execution
of any shape"（build.py 注释口径）。建议将谓词扩为浏览器会解析执行的成员类型（至少 .svg/.xhtml，可含 .xht/.xml），或在预览服务端仅对白名单扩展以可渲染
MIME 内联服务、其余强制下载（Content-Disposition: attachment）。

- // craftScreenWebMemberReportsHTML reports whether a staged member is HTML.
+ // craftScreenWebMemberIsHTML reports whether a staged member is parsed (and
+ // potentially script-executing) HTML by a browser: .html/.htm documents and
+ // the XML document types whose recorded MIME (image/svg+xml,
+ // application/xhtml+xml) executes inline script on direct navigation.
  func craftScreenWebMemberIsHTML(rel string) bool {
  	lower := strings.ToLower(rel)
- 	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
+ 	switch {
+ 	case strings.HasSuffix(lower, ".html"),
+ 		strings.HasSuffix(lower, ".htm"),
+ 		strings.HasSuffix(lower, ".xhtml"),
+ 		strings.HasSuffix(lower, ".xht"),
+ 		strings.HasSuffix(lower, ".svg"):
+ 		return true
+ 	}
+ 	return false
  }


─── internal/application/service/craft_web_screen.go:121-123 ───
[maintainability · low] 死代码与冗余：`_ = craft.KindWeb` 无任何作用（craft 包已通过 craft.ErrInvalidInput 使用，import
并不会多余）；手写 slicesContains 在 go 1.26 下可直接用标准库 slices.Contains 替代。均为非阻塞清理项。

- 	_ = craft.KindWeb
  	return nil
  }
+ 
+ // （slicesContains 整体删除，调用点改用 slices.Contains(variants, candidate)）


─── internal/application/service/craft_web_screen.go:29-29 ───
[bug · critical] 服务端筛查把工具链 denylist 从『片段级』错误扩大为『整页级』，会拒绝全部合法 web 构建产物。证据链：(1) build.py 的
EMBED_TAG_RE（同样含 script|meta）只作用于 agent 提供的 content.json 片段（render_html），而最终 index.html =
template.html 占位符替换，模板壳是钉死的信任面；(2) docker/craft/web/template.html（Dockerfile COPY 至
/opt/craft/web，toolchain.lock.json/runtime-config.json 双重钉 sha256）必然产出含 `<meta
charset="utf-8">`、`<meta name="viewport"…>`、`<script src="assets/craft-web.js">` 的 index.html，写入
/workspace/output；(3) stageAndUpload 对 output 下每个 .html 成员调用 craftScreenWebHTMLMember，本正则命中
`<meta`/`<script` —— 每次合法构建的收集都以 ErrInvalidInput 拒绝整轮，web 主路径完全断裂：无法产生候选/版本，T15 四检门永远没有输入。测试仅用
`<h1>v1</h1>` 形态，未覆盖模板形态，故全绿但生产断裂。修复方向：识别信任模板壳（如按 build-log 的 template_sha256 验证 entry 的固定头/尾，仅筛查
CRAFT_CONTENT 区段），或精确放行固定 assets 引用（src="assets/craft-web.js"）与白名单 meta（charset/viewport），其余
script/meta 仍拒绝。注意与已确认的 .svg/.xhtml 绕过互补：那是漏报，这是把合法产物全杀。



─── internal/container/craft_run_capture_wiring.go:275-277 ───
[performance · medium] 即时 drain 与周期扫描之间缺少在途排除/租约：drain 进行中 receipt 处于 pending/capturing，都在周期扫描的
state 过滤范围内，且 `BeginCapture` 对 digest 相同的并发第二次捕获直接放行，因此耗时超过对齐窗口（平均 ~7.5s）的捕获必然被 ticker 扫到并发重做：两边各自全量
staging + upload。由于 `SaveBytes` 每次生成新的物理路径（local 后端 `_<UnixNano>`、S3 用 uuid
key），`resourceCatalog.Register` 仅按物理路径去重，无法复用——并发败方（`Seal` 因 Ref 不同返回 ErrConflict）的全部上传对象、以及被 15s
预算腰斩在 uploadCapture 中途的 drain
已上传对象，都会成为已注册但无引用的资源行，重试又全量重传，无回滚/清理路径，存储泄漏随每次竞争/中断累积（多副本部署按副本数放大）。正确性虽由 BeginCapture/Seal/Advance 的
CAS 保护，但建议：(a) 扫描跳过最近 N 秒内被 drain 触碰过的 receipt（如 `updated_at` 新鲜度门），或引入租约列；(b) 上传对象名按 (run, manifest
digest, path) 确定性命名以复用 Register 的物理路径去重，并在 Seal 冲突/失败时删除未封存的上传。



─── internal/container/craft_run_capture_wiring.go:250-253 ───
[performance · medium] 本 ticker 是 `RecoverPending`（含 `craftCaptureRecoveryInsertSQL` 合成）的首个生产调用方，每
15s 执行一次该多表 JOIN INSERT..SELECT：对全部历史终态 Craft Run 逐行求值 `agent_runs.snapshot` 的 JSON
谓词（json_type/json_extract，不可索引），且 SQL 无任何 created_at/updated_at 水位过滤，成本随历史 Run 总数线性增长并永久重复，与是否还有未决
receipt 无关。建议将"丢失 receipt 的补写"与"既有 receipt 的推进"解耦：合成按水位（如 `agent_runs.updated_at >
上次成功扫描时间`，或仅扫描最近终态的 Run）低频执行，15s tick 只跑有索引支撑的 state 查询推进已落库的 receipt。



─── internal/container/craft_web_build.go:343-344 ───
[bug · low] present-but-empty 的 build-log.json（err == nil 且 len(raw) == 0，即文件存在但为 0 字节）被并进 `if err
!= nil || len(raw) == 0` 分支静默返回，与"文件不存在"完全同权；而 1 字节的损坏日志会进入 ParseCraftWebBuildLog 失败路径并打出 "rejecting
malformed build log" 告警。该函数自己的注释声明"present-but-malformed log 是运维必须可见的 tamper/drift
信号"，清零截断恰是最典型的篡改形态，却正好落在静默缝隙里（决策仍 fail-closed 为 unobserved，但审计可见性缺口与既定契约不符）。建议把 err
分支与空文件分支拆开，空文件同样记 Warn。

  		raw, err := readLog(ctx, task)
- 		if err != nil || len(raw) == 0 {
+ 		if err != nil {
+ 			// Missing file is the common "no build ran" case and stays
+ 			// silent; a read failure beyond not-exist is worth a trace.
+ 			if !errors.Is(err, fs.ErrNotExist) {
+ 				logger.Warnf(ctx, "[CraftWebBuild] build log read failed for run %s: %v", task.Fence.RunID, err)
+ 			}
+ 			return evidence
+ 		}
+ 		if len(raw) == 0 {
+ 			// A present-but-empty log is a truncation/tamper signal, not the
+ 			// common missing-file case: keep the decision unobserved but make
+ 			// it visible, exactly like the malformed-log path below.
+ 			logger.Warnf(ctx, "[CraftWebBuild] build log for run %s is present but empty", task.Fence.RunID)
+ 			return evidence
+ 		}


─── internal/handler/craft_model_gateway.go:532-532 ───
[bug · medium] 机器码与落账状态不匹配，导致"确定未发出"的干净失败被转化为该 Run 模型出口的永久停靠：此路径 Resolve(DefinitelyNotStarted)
已成功落账，但响应码为 502 UPSTREAM_ERROR——出口适配器（internal/modules/craftegress/adapter.go
gatewayReportsActivityUnresolved）将 502+UPSTREAM_ERROR 一律判定为 unknown-outcome 并停靠（复用同一
activityID）；而协调器 prepareCraftChargeStartTx（craft_budget.go:396-401）对任何已存在的 journal 行（含
definitely_unstarted 状态）无条件返回 ErrConflict，重试将收到 409 ACTIVITY_UNRESOLVED，适配器继续停靠。净效果：一次确定安全的重试被永久
park，且该次 BeginBinding 已分配的 G4 预留与 CraftBudgetCallRow 配额（计入 used >= MaxCalls
暂停阈值）永久占用，只能人工对账恢复。触发窗口窄（BeginBinding 内部 ctx.Err() 检查通过后、initiation context
建立前客户端断连的微秒级竞态），但建议在此路径返回适配器视为 definitive 的机器码/状态（使其重试时铸造新身份），或让协调器允许 definitely_unstarted 行重新开始。



─── internal/handler/craft_model_gateway.go:602-603 ───
[bug · low] Resolve(Started) 失败时以 nil usage 落账，丢弃了已完整观测到的用量事实：此路径之前 respBody
已成功读取并通过大小校验，craftParseUsage(respBody) 的 token 计数是确定的物理观测值；usage ledger 以 UsageKey(tenant, callID,
attemptID) 幂等，记入已观测 usage 安全且更准确。记 nil（unknown observation）会让后续人工对账失去本可固定的计费依据。建议改为 g.recordCall(c,
payload, callID, attemptID, model, craftParseUsage(respBody))。



─── internal/handler/craft_model_gateway.go:58-59 ───
[bug · low] 3 秒硬上限使慢数据库下的 O01 usage 事实被静默丢弃且无补偿路径：改动前 recordCall 继承请求上下文，客户端保持连接时写入不设期限（慢 DB
下最终可持久化）；现在即使连接健康，单次写入超过 3
秒（锁竞争、主从切换）即丢弃。已确认全代码库（internal/application/service、internal/modules/craft、internal/application/repo
sitory）不存在任何 reconcile/补录实现，充电日志的 Unknown 状态也不携带 token
计数，一旦丢弃即不可再生——产生无归因的供应商消耗。当前兜底仅有结构化日志与关联响应头。建议确认此取舍可接受（例如对 usage
台账写入采用更长上限或引入基于充电日志的对账补录），或在文档中明确该数据丢失窗口由运维通过日志头关联手工补偿。



─── internal/handler/craft_model_gateway.go:576-579 ───
[bug · medium] 响应头已返回后仍 Resolve(Unknown)，把确定事实记成未知并停靠整个 Run：Do 成功返回 resp 即证明物理请求已开始（本分支随后的
recordCall 也确实把该物理调用作为事实写入 O01 台账，nil usage 仅表示用量未知），但此处与 resp.Body == nil 分支都
Resolve(CraftChargeStartUnknown) 并返回 502 ACTIVITY_UNRESOLVED。出口适配器 gatewayReportsActivityUnresolved
据此停靠该 activityID，重试复用同一 ID 撞 ErrConflict→409 永久停靠；agent_run.go 的 claimableSQL 还会把 journal 处于
intent/unknown 的 Run 排除出 lease 恢复，只能人工 reconcile，G4 hold 同时永久占用。触发条件现实存在：上游/中间层返回空 body 的 5xx、响应体超
8MB 被截断、连接重置中断读体。对比同分支语义：同样 5xx 只要带非空 body 即走 Resolve(Started) 确定性路径——同一故障是否停摆 Run 取决于错误页 body
是否为空，不一致。建议：headers 已返回的分支 Resolve(CraftChargeStartStarted)（usage 仍可为 nil，保留 unknown observation
语义），且响应码避开适配器视为 unknown 的 ACTIVITY_UNRESOLVED/UPSTREAM_ERROR（两者都会停靠），改用如 UPSTREAM_INCOMPLETE
的确定性码，让重试铸造新 activityID。

  	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, craftMaxForwardBody+1))
  	closeErr := resp.Body.Close()
  	if readErr != nil || len(respBody) == 0 || len(respBody) > craftMaxForwardBody || closeErr != nil {
  		g.recordCall(c, payload, callID, attemptID, model, nil)
+ 		// Headers were received: the physical call definitely started; only
+ 		// the usage observation is unknown. Resolve Started so the identity is
+ 		// definitive and the adapter mints a fresh one on retry.
+ 		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartStarted)
+ 		...
+ 		appFail(c, http.StatusBadGateway, "UPSTREAM_INCOMPLETE", "the upstream response could not be read; the call was charged")


─── internal/handler/craft_model_gateway.go:642-649 ───
[bug · low] err==nil 时强转 DeadlineExceeded 会把实际成功的 initiation 记为 Unknown：requestCtx 仅通过 AfterFunc
回调取消；若响应头在 30s 截止之后、回调取得 mu 之前返回（回调随后看到 responseHeadersReturned=true 会跳过取消），Do 以 err==nil 成功返回，但此处
startCtx.Err() != nil 仍被判为 initiationExpired，一次完整成功的请求被改写为 DeadlineExceeded→Resolve(Unknown)→适配器停靠该
activityID 且 Run 被排除出 lease 恢复。err==nil 恰恰证明取消未生效（initiation 在其执行语义内成功），该强转只会制造虚假 Unknown。建议仅当 err
!= nil 时才按过期处理，err==nil 走正常返回由调用方读体并 Resolve(Started)。

- 	if initiationExpired {
+ 	if initiationExpired && err != nil {
  		cancelRequest()
- 		if err == nil {
- 			err = context.DeadlineExceeded
- 		}
  		return resp, err, true, true, cancelRequest
  	}
+ 	// err == nil means the AfterFunc never cancelled: headers returned within
+ 	// the initiation window's enforcement semantics — treat as success.
  	return resp, err, true, false, cancelRequest


─── internal/handler/craft_model_gateway.go:752-755 ───
[bug · low] 日志字段 craft_grant_id 恒为空：全代码库没有任何 c.Set("craft_grant_id", ...)（internal 下甚至无 c.Set
调用），c.GetString 永远返回 ""。failBudget 的 default 分支正是 BeginBinding/Admit 基础设施失败（GORM 错误）需要对账的场景，而
Forward 调用点手头就有 payload.GrantID。建议给 failBudget 增加 grantID（或 runID）参数，由调用方显式传入，替代读取一个从未被设置的 gin key。

  		logger.ErrorWithFields(c.Request.Context(), err, map[string]any{
- 			"craft_grant_id": c.GetString("craft_grant_id"),
+ 			"craft_grant_id": grantID, // 由 failBudget 新增参数传入；Forward 传 payload.GrantID
  		})
  		appFail(c, http.StatusInternalServerError, "BUDGET_GATE_FAILED", "the budget gate could not be consulted; retry or contact the operator")


─── internal/modules/craft/input_code.go:415-416 ───
[security · high] readOnlyCommands 白名单对命中的命令完全跳过操作数与 flag 附加值筛查（含 flagValueCandidates），但 `sort`
并非纯读取命令：GNU coreutils 的 `sort --compress-program=PROG` 会通过 `sh -c` 将 PROG 作为子进程执行。因此 `sort
--compress-program=inputs/x.sh data.txt`（分离值形式）与 `--compress-program=inputs/x.sh`（= 附加形式）都会以 Allowed
放行，随后上传的 inputs/x.sh 字节被真实执行——直接突破「上传代码仅作数据不执行」的核心策略。已核实无其他防线兜底：生产适配器
ReviewNormalExec（craft_execution_policy.go:126-127）仅以 stdin 字节摘要填充 TargetSHA256，不填充文件目标摘要或
ResolvedTargetPath，所以第 1/2 层（symlink/byte-identity）对该向量均不触发；全库也无 compress-program 的其他拦截或测试固化。建议：将
`sort` 移出 readOnlyCommands（其普通操作数筛查随之生效，排序上传数据可改用 stdin 管道 `cat inputs/x | sort`），或至少对 readOnly 命令恢复
flagValueCandidates 筛查并显式拒绝 --compress-program 的取值落入只读树。

- 		if !readOnlyCommands[path.Base(req.Command[0])] {
+ 		// sort 执行 --compress-program 的程序（sh -c），不是纯读取命令；
+ 		// 移出白名单使其操作数/附加值纳入常规筛查。
+ 		if !readOnlyCommands[path.Base(req.Command[0])] || path.Base(req.Command[0]) == "sort" {
  			for _, arg := range req.Command[1:] {


─── internal/modules/craft/input_code.go:362-364 ───
[bug · low] carriesProgramTextFlag 的字母扫描存在明确的合法命令误拒：`java -jar app.jar`（'-jar' 含 'r'）被拒，而 -jar 是
java 执行生成 jar 的标准形式；`python3 -Werror ...` 同样命中 'e'/'r'。类似地 hasExecForwardFlag 对任何 argv 中出现 "-exec"
记号的命令一律拒绝，生成代码自身的 CLI 参数（如 `mytool -exec job1`）会被误伤。当前 KindWeb 产物几乎不会触发这些形态，但该策略是通用组件，注释也已声明
fail-closed 立场——建议至少为常见解释器补充已知的取值/无害标志表（如 java 的 -jar 按取值标志跳过整组、python 的 -W），或文档化这些已知误拒形态，避免后续 kinds
启用时可用性回归被当作策略缺陷排查。另注：注释中「bash -c \"python3 <path>\" 作为携带空白的脚本操作数到达」的路径实际不可达——-c 在进入 scriptOperand
阶段前已被上一行拒绝，该子 token 筛选只能被含空白的非标志首操作数触达。



─── internal/modules/craft/input_code.go:583-589 ───
[security · high] env 分支对 "--" 长选项的处理不对称:无 "=" 时返回 Inconclusive(fail-closed),带 "=" 时无条件 offset++
跳过,而 Review() 对 wrapper 前缀(req.Command[:offset])的筛查只做 shellTokens + 冒号拆分——shellTokens 不拆 "="
附加值,也没有像 rest 分支那样调用 flagValueCandidates。结果是分离值形式能被拦住(env -S BASH_ENV=inputs/x.sh bash gen.sh 中
"BASH_ENV=inputs/x.sh" 被 envAssignment 剥离后筛查),但附加值形式 env --split-string=BASH_ENV=inputs/x.sh bash
gen.sh 会整体放行:该 token canonical 后为 <wd>/--split-string=BASH_ENV=inputs/x.sh,不在 inputs
树内;rest=["gen.sh"] 干净;最终 Allowed。而 GNU coreutils env(≥8.30)会把 --split-string 的值按 argv
语义拆分处理,BASH_ENV=inputs/x.sh 成为环境变量,bash 非交互启动时 source 该文件——上传的 inputs/x.sh 字节被执行,绕过整条 T03
执行策略。同样模式也见于 default 分支(xargs/stdbuf 等)的 strings.Contains(next, "=") 跳过,如 xargs
--arg-file=inputs/x.sh -I{} bash {} gen.py 依赖运行期替换执行。建议:在 Review() 的 prefix 筛查中对以 "-" 开头的 token 补充
flagValueCandidates 拆值筛查(与 rest 分支对齐);同时将 env/xargs
的长选项收敛到已知白名单(--split-string、--block-signal、--arg-file 等按值语义建模),其余 --opt=value 返回 Inconclusive。

  				if strings.HasPrefix(next, "--") {
  					if strings.Contains(next, "=") {
+ 						// 仅当附加值经过拆值筛查后才能视为已结论:见 Review() 的
+ 						// prefix 筛查,需对 "-" 开头 token 补充 flagValueCandidates,
+ 						// 否则 env --split-string=BASH_ENV=<inputs>/x.sh 这类
+ 						// 启动钩子会绕过整条策略链。
  						offset++
  						continue
  					}
  					return InterpreterScanInconclusive, 0
  				}


─── internal/modules/craft/input_code.go:438-443 ───
[documentation · low] 这段注释与实际行为矛盾:cp/mv 并不在 readOnlyCommands 白名单中,`cp inputs/x.py /tmp/x.py`
会走上面的操作数筛查并以 input_target 拒绝——"byte-copying commands naming input material 不在此执行、副本下次执行时由 digest
层拦截"的残留模型在当前代码里并不存在(复制这一步本身就进不去)。当前行为更保守(方向安全),但注释会让维护者误以为复制放行是该策略文档化的已知残留,进而(例如)在调整白名单时按错误的语义扩展。建议改
写注释以反映真实行为:cp/mv 及一切非只读命令命名 inputs 路径均被拒;digest 层仅覆盖路径与字节都绕开 inputs 树标识的副本(需 adapter 供应
TargetSHA256)。

- 		// Read-only utilities and byte-copying commands (cp, mv) naming input
- 		// material do not execute it here; the digest layer catches the
- 		// copied bytes at their next execution WHEN the adapter supplies the
- 		// target digest — a server-side adapter without container filesystem
- 		// access documents that two-step copy/execute as a known residual
- 		// instead (see the adapter's evidence contract).
+ 		// Every command outside readOnlyCommands — cp, mv included — is
+ 		// denied for ANY operand inside the read-only tree above, so a
+ 		// copy out of inputs/ never starts here. The digest layer only
+ 		// covers copies whose path AND byte identity both escape the
+ 		// inputs tree (it fires WHEN the adapter supplies the target
+ 		// digest); a server-side adapter without container filesystem
+ 		// access documents that as a known residual (see the adapter's
+ 		// evidence contract).


─── internal/modules/craftegress/adapter.go:315-318 ───
[bug · high] 将 UPSTREAM_ERROR
一律按未知结局停泊与同变更集的网关契约（internal/handler/craft_model_gateway.go）矛盾，会造成不可自愈的请求死锁。证据链：(1) 网关现在唯一发出
UPSTREAM_ERROR 的路径是 forwardWithinInitiation 返回 !attempted（发起上下文在 Do 之前已过期），且该路径已成功
attempt.Resolve(CraftChargeStartDefinitelyNotStarted)——此时无物理发送、无 recordCall 记账，属确定性的"未开始"结局；(2) 网关
prepareCraftChargeStartTx 对任何已存在 activity_key 行（不看 state，包括 definitely_unstarted）一律返回
craft.ErrConflict → 409 ACTIVITY_UNRESOLVED。组合后果：适配器在收到 UPSTREAM_ERROR 后停泊该 ID，客户端同指纹重试复用该 ID → 网关
409 → 适配器继续停泊 → 该逻辑请求永久 409 死循环，只能人工对账（本变更集内无对账工具）。触发路径现实可达：BeginBinding/上游 resolver 的 DB 慢查询耗尽 30s
发起预算即触发。注释所述"historically ... recorded Unknown"描述的是旧网关行为，同仓同发的网关已改。建议从停泊集合移除 UPSTREAM_ERROR（现契约下它 ⇔
DefinitelyNotStarted，按定局 resolve 后重试铸造新 ID 是安全的），并同步修正 ocr_regression_test.go 中锁定旧行为的 round-3 断言。

  	switch envelope.Error.Code {
- 	case "ACTIVITY_UNRESOLVED", "UPSTREAM_ERROR":
+ 	case "ACTIVITY_UNRESOLVED":
  		return true
  	}
+ 	// UPSTREAM_ERROR 在当前网关契约下仅在 !attempted 路径发出，且该路径
+ 	// 已 Resolve(DefinitelyNotStarted)：无物理发送、无计费，属定局；
+ 	// 停泊它会让同指纹重试复用的 ID 被网关以既有 activity_key 冲突
+ 	// (409) 永久拒绝。


─── internal/modules/craftegress/journal.go:235-236 ───
[bug · low] appendLocked 不校验 record.Ordinal > 0，与 replay 的完好性判定不一致：Resolve 收到非本 journal 铸造的
attemptID 时 ordinalLocked 返回 0，会写出一条 JSON 合法但 Ordinal=0 的行；replay 将 Ordinal<=0
视为不可读记录，若该行处于文件中段（其后仍有追加），下次启动直接以"journal record unreadable"拒绝启动，全部 egress 503。当前生产调用方（ServeHTTP）只传
journal 铸造的 ID，属防御纵深缺口，但失败模式是 journal 自我中毒且不可恢复。建议在 appendLocked（或 Resolve 入口）对 ordinal<=0 显式报错，使防御与
replay 判定对称。

  func (j *CraftEgressAttemptJournal) appendLocked(record CraftEgressAttemptRecord) error {
+ 	if record.Ordinal <= 0 {
+ 		return fmt.Errorf("craftegress: journal record ordinal must be positive, got %d", record.Ordinal)
+ 	}
  	if j.file == nil {


─── internal/modules/craftegress/journal.go:190-190 ───
[maintainability · low] 导出的 Reuse 方法在全部生产代码中无调用方（ServeHTTP 仅使用 AllocateIfNotParked；唯一使用点是
ocr_regression_test.go）。这是身份协议的核心安全面，多余的导出 API 会诱导未来调用方绕过"check-and-mint 原子性"（见 AllocateIfNotParked
注释）单独复用身份。建议删除或降为私有；若为后续对账工具预留，请在注释中说明。



─── internal/modules/craftegress/journal.go:219-222 ───
[bug · low] Resolve(definitive=false) 只追加磁盘记录，不回填内存 unresolved 索引，在并发同 fingerprint 请求交错后破坏"每
fingerprint 至多一个 parked 身份"的核心不变量。路径：AllocateIfNotParked 会让两个并发相同摘要的请求共享同一 parked 身份 X；若 A 的
definitive Resolve 先执行（delete unresolved[digest]），B 随后的 unknown-outcome Resolve(false)（如网关
409/502+ACTIVITY_UNRESOLVED、或响应 readErr 路径）只在 journal 追加 X 的 unresolved 记录而不回填内存索引——此时内存中该 digest 无
parked，下一个同 digest 请求 mint 新身份 Y，而 journal 中 X 的最新状态仍是 unresolved。后果：(1) 进程内索引与 journal 终态持续分歧；(2)
重启 replay 时 `for _, record := range states` 按 attemptID 聚合后，同 digest 存在 X、Y 两个 unresolved 记录，map
遍历无序导致 unresolved[digest] 随机择一；若选中网关侧已完成的 X，后续同 fingerprint 重试将始终收到 409 ACTIVITY_UNRESOLVED → 再次
park → 死锁，直至请求体变化或人工对账（正是 adapter.go 注释声称要避免的双计费/死锁面）。建议：definitive=false 且 attemptID 为本 journal
铸造（ordinals 命中）时回填 j.unresolved[requestDigest]；或至少在 definitive 分支删除前校验 parked.AttemptID ==
attemptID，避免清掉仍在飞行的身份。

+ 	record := CraftEgressAttemptRecord{
+ 		Ordinal: j.ordinalLocked(attemptID), AttemptID: attemptID, RequestDigest: requestDigest,
+ 		State: state, GatewayStatus: gatewayStatus, CreatedNano: j.now().UnixNano(), ResolvedNano: resolvedNano,
+ 	}
+ 	if err := j.appendLocked(record); err != nil {
+ 		return err
+ 	}
  	if definitive {
+ 		if parked, ok := j.unresolved[requestDigest]; !ok || parked.AttemptID == attemptID {
- 		delete(j.unresolved, requestDigest)
+ 			delete(j.unresolved, requestDigest)
+ 		}
+ 	} else if _, minted := j.ordinals[attemptID]; minted {
+ 		if parked, ok := j.unresolved[requestDigest]; !ok || parked.AttemptID == attemptID {
+ 			j.unresolved[requestDigest] = record
+ 		}
  	}
  	return nil


─── internal/modules/execution/sandbox/docker_exec_events.go:129-130 ───
[bug · low] 重放窗口对时钟偏移的容差不对称：since 侧扣除了 dockerExecEventReplayGrace，而 until 侧没有任何宽限。Docker
守护进程用自身时钟为事件打 TimeNano 戳，但 since/until 是用本进程墙钟生成的。当守护进程时钟快于本服务（偏移超过"exec 终态观察到本次事件重放"的耗时，即可能小于
1s）时，exec_die 的守护进程时间戳会晚于 until（客户端 now），事件被截在窗口外，authoritative-duration 路径将系统性退化为
Unavailable（fail-closed，仅可用性受损，不会产生错误时长）。反向偏移（守护进程慢）时 until 相对守护进程在未来，事件流会滞留到守护进程时钟追上 until
才收束，阻塞时间等于偏移量（受调用方 ctx 兜底）。建议与 since 对称，为 until 追加同样的宽限。

- 	until := time.Now()
+ 	until := time.Now().Add(dockerExecEventReplayGrace)
  	result := streamer.Events(ctx, client.EventsListOptions{


─── internal/modules/execution/sandbox/docker_normal_exec.go:182-184 ───
[maintainability · low] ExecCreate 与随后的身份校验 ExecInspect 共用同一个 rpcCtx 超时预算：create 本身接近耗尽 rpcTimeout
时（慢守护进程下并不罕见），inspect 只剩残余 deadline，身份校验会因超时而失败，导致整个创建被判失败并在守护进程侧遗留一个已创建但永不启动的惰性
exec（只能随容器销毁回收）。inspect 是廉价调用，建议为其派生独立的超时上下文，与 StartAttachedExecOnce/inspectRaw 中"每次 RPC
独立计时"的做法保持一致。

- 	rpcCtx, cancel := context.WithTimeout(ctx, c.rpcTimeout)
- 	defer cancel()
- 	created, err := c.api.ExecCreate(rpcCtx, containerID, client.ExecCreateOptions{
+ 	createCtx, cancelCreate := context.WithTimeout(ctx, c.rpcTimeout)
+ 	defer cancelCreate()
+ 	created, err := c.api.ExecCreate(createCtx, containerID, client.ExecCreateOptions{
+ 		// ...(选项不变)
+ 	})
+ 	if err != nil {
+ 		return DockerNormalExecReceipt{}, dockerError("NormalExecCreate", err)
+ 	}
+ 	// ...
+ 	inspectCtx, cancelInspect := context.WithTimeout(ctx, c.rpcTimeout)
+ 	defer cancelInspect()
+ 	inspected, err := c.api.ExecInspect(inspectCtx, receipt.ExecID, client.ExecInspectOptions{})


─── packages/domain/src/craft/web-promotion.ts:67-72 ───
[test · low] 排序见证分支零测试覆盖：web-promotion.test.ts 的现有用例全部不传 createdAt，因此 Date.parse 比较与单侧 NaN
回退数组序的分支从未被执行过。该分支正是注释承诺的行为（"when facts carry createdAt the function enforces the rule itself (max
createdAt among ready facts) instead of trusting caller order"），其中混合见证场景（部分事实带
createdAt、部分不带时保留先出现者，即时间戳序与数组序的衔接）只靠注释描述、无任何断言保护。建议补充：乱序输入且全带 createdAt 时按 max 时间戳取胜、部分带 createdAt
的混合列表、相等时间戳保留先出现者等用例，确保未来 contracts 增加 createdAt 字段接线时该规则不会被无意改变。



─── packages/views/src/craft/access.tsx:17-25 ───
[test · medium] errorHint 的三个分支（status>=500 服务器错误、4xx 服务端拒绝、无 status 传输失败）没有任何测试触达：access.test.tsx
的失败用例只以普通 `new Error('network down')` 拒绝（无 status 字段），断言也只匹配 /could not be added/i，不校验 hint
文案本身。而"区分服务端拒绝与传输失败"正是本次重构的全部目的（注释原话 "the distinction is what makes the message
actionable"），两条新文案（'The server could not complete the request…' / 'The server refused the
request…'）从未被验证。建议在 access.test.tsx 增加带 status 的拒绝用例（如 reject(Object.assign(new Error('forbidden'),
{ status: 403 })) 与 { status: 500 }），分别断言两种服务端文案与回退文案。

- function errorHint(error: unknown): string {
-   const status = (error as { status?: number } | null)?.status;
-   if (typeof status === 'number') {
-     return status >= 500
-       ? 'The server could not complete the request. Try again later.'
-       : 'The server refused the request. Check the member ID and your permission.';
-   }
-   return 'Check your connection and try again.';
- }
+ // access.test.tsx 中补充（示意）：
+ await act(async () => rejectGrant(Object.assign(new Error('forbidden'), { status: 403 })));
+ assert.match(view.container.querySelector('[aria-live]')?.textContent ?? '', /server refused the request/i);
+ // 以及 { status: 500 } → /server could not complete the request/i


─── packages/views/src/craft/access.tsx:42-46 ───
[maintainability · low] runAction 用 null 哨兵区分成功/失败消息：message(null) 表示成功。但 catch 到的 error 本身也可能是
null/undefined（JS 中 Promise.reject(null) 合法），此时会执行 setFeedback({ kind: 'error', message:
message(null) }) —— 在 role="alert" 的错误提示里渲染出成功文案（如 'Member
added.'），产生自相矛盾的反馈。建议把成功与失败文案拆成两个回调，避免哨兵值与真实抛出值重叠。

+   async function runAction(key: string, pendingMessage: string, action: () => Promise<void>, success: () => string, failure: (error: unknown) => string) {
+     setPendingAction(key);
+     setFeedback({ kind: 'pending', message: pendingMessage });
      try {
        await action();
-       setFeedback({ kind: 'success', message: message(null) });
+       setFeedback({ kind: 'success', message: success() });
      } catch (error) {
-       setFeedback({ kind: 'error', message: message(error) });
+       setFeedback({ kind: 'error', message: failure(error) });
+     } finally {
+       setPendingAction(null);
+     }
+   }


─── packages/views/src/craft/craft.css:165-165 ───
[maintainability · low] 幽灵 CSS token：--craft-brand-contrast 在主题系统中没有任何定义（packages/ui/src/theme.css
只定义了 --craft-brand/-strong/-soft/-ink，docs/design 的 tokens.css 也没有），因此 var(--craft-brand-contrast,
#fff) 永远回退到硬编码 #fff。这与本文件其他 token 的用法不一致（--craft-danger、--craft-success-text 等均直接引用、不带回退），会让维护者误以为该
token 已存在、主题可覆盖，而实际上换主题/换品牌色时 #fff 对比度无人可控。建议在 packages/ui/src/theme.css 中正式定义
--craft-brand-contrast（映射到合适的颜色变量），或去掉未定义 token 直接写具体颜色并注明理由。



─── packages/views/src/craft/craft.css:167-167 ───
[maintainability · low] 跨文件缺口：sources.tsx 本次新增的引用清单区块（data-testid="craft-citations"）使用了
wk-craft-citations、wk-craft-citation-list、wk-craft-citation-fact、wk-craft-citation-inference、wk-craf
t-fact-label、wk-craft-inference-label 六个类，但全仓库任何 CSS 文件中都没有这些类的规则（已全局搜索确认）。同一 diff 里 T08 access 面板与
T11 share 面板都遵循"静态样式集中在 craft.css"的模式（注释也如此声明），唯独引用清单落在浏览器默认样式上（ul 带默认项目符号、li
无间距控制）。建议为这些类补上与兄弟面板一致的间距/列表样式规则，保持类系统完整性。



─── packages/views/src/craft/sources.tsx:158-160 ───
[documentation · low] 该守卫的注释声称 "a new citation kind added to the union fails HERE at compile
time"，但守卫参数类型是 { kind: string }（任何联合成员都可传入）且只匹配 'fact'——新增 kind 既不会在守卫处报错，也不会在 citedIds 处报错（filter
只是静默排除它）。真正的编译期检查发生在渲染分支：early-return 排除 'inference' 后访问 entry.citationId。更隐蔽的是口径分歧：若未来新增的 kind 也携带
citationId，citedIds（走守卫，排除新 kind）会把对应来源标为未引用，而渲染分支（kind !== 'inference' 即按 fact 渲染）却会给它渲染打开按钮——两处对新
kind 的处理不一致。建议修正注释为准确描述，并统一两处判断口径（渲染分支也基于 isCraftCitationFact 判定，else 走 inference/未知渲染）。

+ // isCraftCitationFact narrows the citation union without a silent cast. The
+ // render branch below must use the SAME guard (not kind !== 'inference') so a
+ // future citation kind cannot render as a fact while citedIds excludes it.
  function isCraftCitationFact(entry: { kind: string }): entry is CraftCitationFact {
    return entry.kind === 'fact';
  }
+ // 渲染处：
+ // if (isCraftCitationFact(entry)) { /* fact 渲染 */ } else { /* inference 渲染 */ }


─── packages/views/src/craft/usage.tsx:179-185 ───
[maintainability · low] CraftBudgetPauseNotice 只使用 CraftUsageStrings 16 个字段中的 4
个（pauseTitle/pauseCanExtend/pauseContactOwner/pauseRequestExtension），却要求传入完整面板字典：想本地化该通知的集成方必须连带提供
residencyLine、tokensLine、failureAt 等函数字段，否则无法通过类型检查。建议为该通知定义窄化的字符串类型（可从 CraftUsageStrings 中
Pick），降低采用成本。

+ export type CraftBudgetPauseStrings = Pick<
+   CraftUsageStrings,
+   'pauseTitle' | 'pauseCanExtend' | 'pauseContactOwner' | 'pauseRequestExtension'
+ >;
+ 
  export interface CraftBudgetPauseNoticeProps {
    pause: NonNullable<CraftRunView['budget_pause']>;
    /** Projected by the server's current Task owner/billing-admin check. */
    canExtend: boolean;
    onRequestExtension?: (runId: string) => void;
-   strings?: CraftUsageStrings;
+   strings?: CraftBudgetPauseStrings;
  }
+ // 组件内默认值：const pauseStrings = { ...CRAFT_USAGE_STRINGS_ZH, ...strings };
+ // 或单独导出 CRAFT_BUDGET_PAUSE_STRINGS_ZH


─── internal/modules/craft/archive.go:260-261 ───
[performance · medium] zip.NewReader 在任何 MaxArchiveEntries 检查生效之前，会把整个中央目录物化为 []zip.File（每条含
FileHeader、name、extra 分配）。一个 ≤20 MiB 的 zip64 恶意 zip（每个最小条目约 46–47 字节，可声明约 45 万条目，每条在解析器内部约占 150–250
字节）会在 reserveEntry 的 seen 上限（20）被触发之前，先放大出约 85–110 MB 内存——约为输入的 4–5 倍，且随并发线性叠加；这与本文件文档"memory is
bounded"的声明相悖（reserveEntry 的注释只覆盖了 seen 集合自身，未覆盖解析器内部分配）。tar/tar.gz 路径是流式的，无此问题；ExpandArchive 经
handler/session/craft.go 的 HTTP 入口对认证成员可达，建议在调用 NewReader 前先定位 EOCD（含 zip64 EOCD
"PK\x06\x06"）读取条目总数并在超过 MaxArchiveEntries 时拒绝，再进入解析；至少也应在新 Reader 之后立即检查 len(reader.File)。

  func extractZipArchive(data []byte) ([]ArchiveMember, error) {
+ 	// zip.NewReader materializes one File (plus name/extra allocations) for
+ 	// every central-directory record BEFORE any ceiling below applies; scan
+ 	// the (zip64-aware) EOCD entry count first so a tiny-entry zip bomb is
+ 	// refused pre-parse instead of allocating ~5x the input in memory.
+ 	if n, ok := zipCentralDirectoryCount(data); ok && n > MaxArchiveEntries {
+ 		return nil, fmt.Errorf("%w: archive exceeds the maximum entry count %d", ErrInvalidInput, MaxArchiveEntries)
+ 	}
  	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))


─── internal/modules/craft/archive.go:191-193 ───
[bug · medium] 目录条目与普通文件条目共同消耗 MaxArchiveEntries（20），但常量文档明确写的是 "caps the regular file members one
archive may add"。普通工具产生的归档普遍携带目录条目（Windows"发送到压缩文件夹"、Finder zip -r、GNU tar 均会写入目录 entry），例如 10 个子目录
+ 20 个文件的合法 zip 共 30 条目：第 21 次 reserveEntry 即被拒，尽管其普通文件成员只有 20 个、完全在预算内。readMember 中对 b.entries
的检查本已单独覆盖普通成员数，这里对 seen 集的总量上限只需承担流式 tar
下的内存界（防止目录名无限累积），用一个独立且更宽松的常量即可两全，否则归档展开功能会对最常见的带子目录归档系统性误拒（fail-closed 的可用性缺陷）。

- 	if len(b.seen) >= MaxArchiveEntries {
- 		return "", fmt.Errorf("%w: archive exceeds the maximum entry count %d", ErrInvalidInput, MaxArchiveEntries)
+ 	// Total reservations (files AND directories) only bound the seen set's
+ 	// memory under streaming tar; the regular-member budget itself stays
+ 	// enforced in readMember, so directories must not consume it.
+ 	if len(b.seen) >= maxArchiveTotalEntries {
+ 		return "", fmt.Errorf("%w: archive exceeds the maximum entry count %d", ErrInvalidInput, maxArchiveTotalEntries)
  	}
+ 
+ // maxArchiveTotalEntries bounds every reservation (files + directories)
+ // purely for the seen set's memory; regular members stay capped by
+ // MaxArchiveEntries in readMember.
+ const maxArchiveTotalEntries = 10 * MaxArchiveEntries


─── internal/application/service/craft_docker_normal_exec.go:179-188 ───
[bug · medium] claimed-never-started 死角无收敛路径：持久 claim 消费后（send_claimed_at 已落库，按设计不可重发），此处 ctx
取消（或随后的 OpenSink 失败、进程崩溃）直接返回 unknown——输出操作行未创建、无 seal。此后所有 Execute 重放走
observeClaimed，ObserveAttachedExec 因无正向启动证据永远返回 Unknown，活动永久卡死。internal/modules/craft/lifecycle.go 无
send_claimed 对账逻辑；craft_budget.go:336-338 的注释也确认搁浅的 intent 需人工 reconcile。建议为已 claim 的 docker 协议
intent 增加生命周期对账（如 exec inspect 判定从未运行后将 journal 收敛到失败终态/补 seal 空输出），或固化人工修复 runbook。



─── internal/application/service/craft_docker_restricted_exec.go:145-151 ───
[bug · medium] 与 normal exec 相同的 claimed-never-started 死角：durable claim 被消费后、StartOutputlessExec 之前
ctx 取消（或进程崩溃），ExecStart 永远不会发出且 claim 不可重放，后续 Wait/Observe 对该 exec 只能给出永久 Unknown（inspect
显示未运行但无正向启动证据），活动无任何自动收敛路径。建议随 normal 路径一并提供生命周期对账（inspect 判定从未启动后收敛 journal 到失败终态）或明确的人工修复流程。



─── internal/application/service/craft_docker_normal_exec.go:348-351 ───
[bug · low] readOutput 的错误被丢弃：当输出操作行不存在（claimed-never-started 场景）时，仓储 ReadAfter 对
ErrCraftDockerOutputNotFound 返回的是零值快照 {Sealed:false, Partial:false,
Unavailable:false}，对外呈现为"可用的开放空流"，而该流实际永远不会增长。同 PR 中 CraftDockerOutputSink.ReadAfter 与
CraftDockerOutputService.ReadAfter 在错误时均置 Unavailable=true，此处契约不一致，可能误导上层对输出状态的判定。建议对 NotFound/读失败置
Unavailable=true（或 Partial=true）后再返回。



─── internal/application/repository/craft_docker_send_claim.go:203-206 ───
[maintainability · low] 错误域哨兵错用：normalInputExists 读取的是输入域表 craft_docker_normal_inputs，却用
dockerOutputDBError（ErrCraftDockerOutputUnavailable）包装故障——与 craft_docker_normal_input.go 中
dockerNormalInputDBError
注释声明的"输入链路故障不得误读为输出存储故障"意图相悖；受限（outputless）路径在此读失败会被误诊为输出存储不可用，误导告警与排障路由。建议改用
dockerNormalInputDBError。同类问题：本文件 load() 对 journal 读失败用了输入域哨兵 dockerNormalInputDBError，对 outputless
调用方同样跨域误诊。

- 	err := db.WithContext(ctx).Table("craft_docker_normal_inputs").Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Count(&count).Error
  	if err != nil {
- 		return false, dockerOutputDBError(err)
+ 		return false, dockerNormalInputDBError(err)
  	}


─── internal/application/service/craft_docker_normal_exec.go:211-216 ───
[performance · low] started 映射仅在观察到终态时删除键：被调用方放弃重试、容器销毁或永远停留在 Unknown 的 exec（每键含 container/exec
标识字符串）会在长生命周期服务实例中永久累积。该服务一旦被装配为进程级单例（当前生产装配点尚缺，仅测试构造），即构成按废弃活动数缓慢增长的内存驻留。建议为键增加基于活动年龄/上限的清扫（如定期移除超过
会话时限的键）。



─── internal/application/service/craft_docker_restricted_exec.go:0-0 ───
[performance · low] running 映射与 normal exec 的 started 映射同型：键仅在终态 Observe 时删除，Wait
超时后被放弃、或永远观察不到终态的活动键（grantID+activityID）会在长生命周期实例中无限累积。建议同样引入按年龄/容量的清扫，避免装配为单例后按废弃活动缓慢泄漏。



─── internal/application/service/craft_docker_normal_exec.go:208-208 ───
[bug · medium] 请求级输出配额截断对持久化输出投影不可见：limitedNormalExecSink 在 request.OutputLimit 处仅做内存截断，而
CraftDockerOutputRepository.Open 写入操作行的 max_bytes 恒为全局 r.maxBytes，store 的 truncated
标志只反映存储层自身额度。当请求限额小于存储上限时，操作行 truncated=false，CraftDockerNormalExecService.ReadAfter（为 live
projection 设计的独立游标契约）会返回 Sealed=true、Truncated=false，下游无法区分"完整输出"与"在请求限额处被丢弃的字节"；Execute 仅用
wasTruncated() 在本次返回值里临时补偿，不落库。操作行 schema 已有 max_bytes 列且 append 已按 op.MaxBytes
实施配额与幂等重放——建议把请求限额下沉为该次操作的持久化上限（Open/OpenSink 接受 min(全局, request.OutputLimit)），让 durable truncated
标志成为唯一权威，同时使 store 既有的 quota 截断重放幂等语义同样覆盖请求限额。

- 	boundedSink := &limitedNormalExecSink{inner: sink, remaining: staged.Request.OutputLimit}
+ 	// 将请求限额下沉到持久化操作行（Open 时以 min(store maxBytes, staged.Request.OutputLimit)
+ 	// 作为该操作 max_bytes），使 durable truncated 标志对游标读者权威，
+ 	// 并可直接复用 store sink 的配额与幂等重放语义。
+ 	sink, err := s.output.OpenSink(ctx, outputScope, providerReceipt, staged.Request.OutputLimit)


─── internal/modules/craft/export_manifest.go:276-277 ───
[bug · medium] 盘符路径校验在模块层与 handler 层条件不一致，导致注释承诺的"打包前干净拒绝"对部分变体失效：此处仅拒绝第一段长度恰为 2（即 `c:/evil`）的路径，而
`c:evil`、`c:Users/x` 等 Windows 驱动器相对路径（`ValidateArtifactPath` 不拒绝 `:`，均可通过）能穿过这道服务端防线，只在
artifact_download.go 的成员循环里被全路径前缀检查（`len(first) >= 2 && first[1] == ':'`）捕获——此时 200 头与三个固定文档已写出，只能
mid-stream abort 连接。这既违反本函数注释 "Refused BEFORE any byte is packaged"，也使 handler 注释 "Refuse it before
the 200 head is written" 失实：被污染的版本行会产出反复的残缺下载而非打包前的干净拒绝。另外 `foo/c:evil`（冒号不在路径首二字符）两层检查均不拦截。建议将本检查与
handler 对齐为对整个路径的前缀判定（first two bytes form `X:`），或直接拒绝首段中含 `:` 的路径，使所有盘符变体在 ExportBundle 阶段统一拒绝。

- 		if first := strings.SplitN(member.Path, "/", 2)[0]; len(first) == 2 && first[1] == ':' &&
- 			((first[0] >= 'a' && first[0] <= 'z') || (first[0] >= 'A' && first[0] <= 'Z')) {
+ 		if p := member.Path; len(p) >= 2 && p[1] == ':' &&
+ 			((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) {
+ 			// Windows drive-letter path (c:/evil or c:evil): a zip-slip
+ 			// variant that older extractors resolve as an absolute target
+ 			// outside the extraction directory. Refused BEFORE any byte is
+ 			// packaged.
+ 			return fmt.Errorf("%w: bundle member %q uses a drive-letter path", ErrInvalidInput, member.Path)
+ 		}


─── internal/modules/craft/export_manifest.go:284-286 ───
[security · medium] 保留名冲突检查为大小写敏感的精确匹配：成员 `EXPORT-MANIFEST.JSON`、`Sources.JSON`、`BUILD.JSON` 等可同时通过
ValidateArtifactPath（无大小写/保留名规则）与本检查进入 zip，且固定文档先写、成员后写。在大小写不敏感文件系统（Windows、macOS 默认
APFS）解包时，后写入的成员会覆盖权威固定文档——run 产出的不可信内容可替换携带 manifest digest（T13 owner consent 绑定的正是该 digest）的
export-manifest.json，破坏 bundle 的完整性契约，也使 zip 内出现双条目。建议对保留名做大小写不敏感比较（如 strings.EqualFold 遍历三个保留名）。

- 		if reserved[member.Path] {
+ 		for name := range reserved {
+ 			if strings.EqualFold(member.Path, name) {
- 			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
+ 				return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
+ 			}
  		}


─── internal/modules/craft/export_manifest.go:284-286 ───
[security · medium] 保留名冲突检查除已确认的大小写敏感问题外，还存在第三类绕过变体：Windows（及部分旧提取器）的 Win32 路径规范化会剥除文件名末尾的点号与空格，成员路径
`export-manifest.json.`、`sources.json `（尾随空格）、`build.json...` 均可同时通过
ValidateArtifactPath（无尾随点/空格规则）与本处的精确匹配进入 zip，且固定文档先写、成员后写——在 Windows 上解包时会覆盖权威的
export-manifest.json（T13 owner consent 绑定的正是该
digest）等固定文档，与大小写变体后果相同。由于本检查是逐字符精确匹配，仅改为大小写不敏感比较（已确认问题 #2 的修法）仍挡不住该变体，需在归一化时一并 TrimRight 空格与点号（例如对
zip 根层（不含 '/' 的成员名）做 `strings.ToLower(strings.TrimRight(name, " ."))`
后再比对保留名集合），使该检查的归一化语义与目标提取器的文件名规范化对齐。

- 		if reserved[member.Path] {
+ 		if !strings.Contains(member.Path, "/") {
+ 			// Zip 根层成员才会与根层固定文档碰撞：与提取器（Windows Win32
+ 			// 规范化：大小写不敏感 + 剥除尾随点/空格）对齐后比对保留名。
+ 			normalized := strings.ToLower(strings.TrimRight(member.Path, " ."))
+ 			if reserved[normalized] {
- 			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
+ 				return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
+ 			}
  		}

