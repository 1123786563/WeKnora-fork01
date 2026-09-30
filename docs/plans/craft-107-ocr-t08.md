Review complete: 56 finding(s) across 94 selected item(s).

─── docker/craft/web/build.py:75-75 ───
[security · high] EVENT_ATTR_RE 存在两个具体绕过，且它是内联脚本的最后防线：(1) `[^>]*?` 无法跨越带引号属性值中的 `>`——`<img src="a>b"
onerror="alert(1)">` 中 `onerror` 位于引号内 `>` 之后，正则永远够不到，而浏览器把 `src="a>b"` 解析为单一属性值后 onerror 照常执行；(2)
HTML 允许用 `/` 作标签/属性分隔符——`<img/onerror=alert(1)>` 在 `onerror` 前没有空白，`\s` 匹配失败。html 片段未经转义直接拼入
index.html，而预览响应的 CSP 是 `script-src 'self'
'unsafe-inline'`（internal/application/service/craft_preview.go:664），明确放行内联事件处理器，因此绕过即构成预览 origin
上的存储型 XSS，违反本模块"拒绝任何形态脚本执行"的自述契约。建议放弃用单个正则识别属性位置，改用 stdlib html.parser 对每个解码变体做结构性筛查：遍历所有
start-tag，凡携带名称以 `on` 开头的属性、或标签名属于 base/iframe/object/embed/form/script/meta 即拒绝（这同时天然覆盖 `/` 分隔与引号内
`>` 的解析语义）。

- EVENT_ATTR_RE = re.compile(r"""<[a-zA-Z][^>]*?\son[a-z]+\s*=""", re.IGNORECASE)
+ # 用 stdlib 解析器替代正则做结构性筛查（对 render_html 的每个解码变体执行）：
+ from html.parser import HTMLParser
+ 
+ class _HandlerScreen(HTMLParser):
+     def __init__(self):
+         super().__init__(convert_charrefs=False)
+         self.violation = None
+     def handle_starttag(self, tag, attrs):
+         if tag.lower() in {"base", "iframe", "object", "embed", "form", "script", "meta"} \
+                 or any(name.lower().startswith("on") for name, _ in attrs):
+             self.violation = tag
+ 
+ def screen_inline_handlers(candidate: str) -> None:
+     parser = _HandlerScreen()
+     parser.feed(candidate)
+     parser.close()
+     if parser.violation is not None:
+         raise BuildError(EXIT_CONTENT, "inline event handler or embedding tag")


─── docker/craft/web/build.py:303-303 ───
[security · medium] 变体闭包在达到 64 上限时 fail-open：`while queue and len(variants) < 64` 退出后未检查 queue
是否仍非空，剩余未探索的变体不经过任何 denylist 即放行。分段 html 是不可信 Agent 产出，攻击者可用超过 64 层的嵌套实体（如重复 `&amp;` 包裹
`url(//evil)`）或相互独立的实体/CSS 转义组合把浏览器最终解码形态推到 BFS 前沿之外，使 EXTERNAL_URL_RE/CSS_FETCH_RE
等全部筛查对真实执行形态失效。BFS 每层变体数按解码器组合增长，64 个名额在深度约 6 处即耗尽，截断并非不可达路径。应在达到上限且队列仍有待探索变体时直接拒绝构建（fail-closed）。

-     while queue and len(variants) < 64:
+     while queue:
+         if len(variants) >= 64:
+             raise BuildError(EXIT_CONTENT, "html section {!r} exceeds the decode-variant budget".format(heading))
+         candidate = queue.pop(0)


─── internal/container/craft_web_build.go:295-295 ───
[security · medium] 构建日志的可信度假设不成立：build-log.json 位于 Agent 可写的 output 树内，且按
skills/craft-web-build/SKILL.md:99 的流程，build.py 由 Agent 在沙箱内运行、`--runtime-digest` 直接随委派 prompt
下发，toolchain_digest/template_sha256 在镜像内（/opt/craft/web，chmod 0555）世界可读。因此 Agent 可以完全不运行
build.py、手写携带脚本的 index.html，再按已公开的摘要伪造一份 exit_code=0 的 build-log.json，通过 ParseCraftWebBuildLog
与本函数的全部一致性校验，令 W01 构建检查（版本晋级门禁的必要条件）在未经过任何 render_html 筛查的情况下通过。函数注释宣称的"the build ran and its REAL
exit status is the fact"实际是 Agent 自证而非平台观测事实。建议将退出状态绑定到服务端可独立观测的通道：例如从委派执行的 shell_exec
工具调用结果（服务端记录）读取 build.py 的真实退出码并与日志交叉校验，或由宿主侧 wrapper 在 Agent 不可写路径落盘/签名日志，或采集时在受控环境对 staged
content.json 确定性重渲染并与 index.html 比对。



─── internal/container/craft_web_build.go:316-316 ───
[bug · low] 导出函数直接解引用 `*log.ExitCode` 而未判 nil。字段被刻意声明为指针以区分"缺失 exit_code"与"真实 0"（见 CraftWebBuildLog
注释），但该防御只存在于 ParseCraftWebBuildLog 中；任何未经过 Parse 而直接构造 CraftWebBuildLog
调用本导出函数的使用方（例如未来的测试或另一装配点）传入缺失 exit_code 的日志即触发 nil 指针 panic。作为导出 API 应在此补一次防御并返回 ErrInvalidInput。

+ 	if log.ExitCode == nil {
+ 		return craft.ArtifactEvidence{}, fmt.Errorf("%w: craft web build log is missing its exit code", craft.ErrInvalidInput)
+ 	}
  	return craft.ArtifactEvidence{BuildRan: true, BuildExitCode: *log.ExitCode}, nil


─── cmd/craft-egress-adapter/main.go:52-59 ───
[bug · medium] 环境变量覆盖接受任意正值且无下限守卫。adapter.go 注释明确钉死不变式：适配器转发预算必须 ≥ 网关 ForwardTimeout（默认 5 分钟）+1
分钟余量，否则慢生成会在适配器侧被中止、attempt 停留 unknown-outcome 永久 parked，同指纹重试在网关 ACTIVITY_UNRESOLVED 409
上死锁直至人工对账。当前不变式仅靠两处注释维系，运维一旦配置如
60s（误以为模型调用很快）即静默触发该死锁。建议对低于网关默认预算（DefaultForwardTimeout()-time.Minute）的配置至少输出显著警告或直接拒绝。

- 	forwardTimeout := craftegress.DefaultForwardTimeout()
- 	if raw := strings.TrimSpace(os.Getenv("CRAFT_EGRESS_FORWARD_TIMEOUT")); raw != "" {
  		parsed, err := time.ParseDuration(raw)
  		if err != nil || parsed <= 0 {
  			log.Fatalf("craft-egress-adapter: CRAFT_EGRESS_FORWARD_TIMEOUT must be a positive duration, got %q", raw)
  		}
- 		forwardTimeout = parsed
+ 		if parsed < craftegress.DefaultForwardTimeout()-time.Minute {
+ 			log.Printf("craft-egress-adapter: WARNING: CRAFT_EGRESS_FORWARD_TIMEOUT=%s is below the gateway's default 5-minute forward budget; slow generations will park their activity IDs (409 retry wedge) until manual reconciliation", parsed)
- 	}
+ 		}
+ 		forwardTimeout = parsed


─── internal/modules/craftegress/adapter.go:48-50 ───
[bug · low] CraftEgressAdapterConfig.Now 字段被静默忽略：NewCraftEgressAdapter 通过
OpenCraftEgressAttemptJournal(config.JournalPath) 打开 journal，从未把 config.Now 传入（journal.now 固定
time.Now），声称的测试时钟注入实际不生效。建议在打开 journal 后接入该钩子，或删除该字段以免误导。

- 	MaxBodyBytes   int64
- 	ForwardTimeout time.Duration
- 	Now            func() time.Time
+ 	journal, err := OpenCraftEgressAttemptJournal(config.JournalPath)
+ 	if err != nil {
+ 		return nil, err
+ 	}
+ 	if config.Now != nil {
+ 		journal.now = config.Now // 同包内直接接线，保证测试时钟注入生效
+ 	}


─── internal/modules/craftegress/journal.go:222-223 ───
[bug · low] appendLocked 未防御 Close 后调用：Close() 将 j.file 置 nil，而 ServeHTTP 只检查 a.journal != nil
覆盖不到该状态。main 的 Shutdown 预算仅 10 秒，在途转发最长 6 分钟；Shutdown 超时返回后 deferred adapter.Close() 先于进程退出执行，仍在途的
handler 随后调用 Resolve→appendLocked 会对 nil *os.File 解引用 panic（被 net/http 逐连接 recover，表现为连接中断）。建议在
appendLocked 开头判 j.file == nil 并返回错误，让调用方复用既有的 503 journal-unavailable 语义。

  func (j *CraftEgressAttemptJournal) appendLocked(record CraftEgressAttemptRecord) error {
+ 	if j.file == nil {
+ 		return fmt.Errorf("craftegress: journal closed")
+ 	}
  	encoded, err := json.Marshal(record)


─── internal/modules/craftegress/journal.go:211-220 ───
[bug · low] ordinalLocked 的兜底路径会制造撞号：两个并发同指纹请求共享同一 parked 身任（AllocateIfNotParked 的设计行为），第一个
definitive Resolve 把记录从 unresolved 删除后，第二个 Resolve 在表中找不到该 attemptID，兜底返回 j.nextOrd 且不自增——该转移记录的
ordinal 与下一条 Allocate 铸造的记录完全相同，且同一 attemptID 被追加重复的 resolved 转移，损害 journal
的审计可读性（身份唯一性因随机熵后缀不受影响）。建议维护 attemptID→ordinal 的专用映射（Allocate 与 replay 时填充，不随 unresolved 删除），查表未命中返回
0。

+ // 在结构体增加 ordinals map[string]int64，Allocate/replay 时写入；
  func (j *CraftEgressAttemptJournal) ordinalLocked(attemptID string) int64 {
- 	// Transitions reuse the attempt's original ordinal when known; a foreign
- 	// id cannot occur because only Allocate minted ids reach this path.
- 	for _, record := range j.unresolved {
- 		if record.AttemptID == attemptID {
- 			return record.Ordinal
- 		}
+ 	if ordinal, ok := j.ordinals[attemptID]; ok {
+ 		return ordinal
  	}
- 	return j.nextOrd
+ 	return 0 // 未知 id：写入 0 而非 nextOrd，避免与后续铸造记录撞号
  }


─── internal/modules/craftegress/journal.go:84-88 ───
[performance · low] journal 无记录数/大小上限与压缩机制，replay 将整个文件读入内存。铸造发生在转发之前且不依赖网关往返：沙箱内客户端可用每次唯一的 body
指纹（发起后立即断开）以 fsync 速率持续追加记录，Run 生命周期内磁盘占用与重启回放内存无界增长。虽有每 Run 隔离与 fsync 速率的自然约束，仍建议增加记录数上限或在重启回放时基于
resolved 状态滚动压缩（保留每个指纹最新状态即可）。



─── internal/modules/craftegress/target.go:56-62 ───
[security · low] rejectPrivateIP 未覆盖两类非路由地址：受限广播 255.255.255.255（不在 224.0.0.0/4 多播段内，IsGlobalUnicast
为 false）与共享地址空间 100.64.0.0/10（CGNAT 段，Go 的 IsPrivate 不包含，云内网/内部负载均衡常见）。启动 DNS 校验与拨号时 Control
复检共用该函数，两类检查存在同样残留。建议追加 !ip.IsGlobalUnicast()（严格收紧，不误伤公网）并显式判定 100.64.0.0/10。

  func rejectPrivateIP(ip net.IP) error {
- 	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
- 		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
+ 	shared := false
+ 	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1]&0xC0 == 0x40 {
+ 		shared = true // 100.64.0.0/10 shared/CGNAT
+ 	}
+ 	if shared || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
+ 		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
  		return fmt.Errorf("craftegress: gateway host %s is loopback/private/reserved; set the explicit private-target opt-in for local deployments", ip)
  	}
  	return nil
  }


─── internal/application/repository/craft_docker_normal_input.go:144-146 ───
[bug · medium] 瞬时 DB 错误被吞并为持久冲突：此处在同一事务内刚通过 lockDockerNormalInputRun 的 no-op UPDATE 确认了 agent_runs
行存在，因此这里的 Take 失败几乎不可能是 ErrRecordNotFound，而只会是基础设施类错误（连接中断、statement timeout、ctx 取消、序列化中止）。把它们与
SessionID 不匹配一并返回 ErrCraftDockerNormalInputConflict（包装 craft.ErrConflict），调用链（PrepareNormal →
prepareOrRecover）会将其按"持久身份冲突"处理：触发 ResumeBoundNormal 补救后原样上抛，Agent
工具层看到的是不可重放的终态冲突而非可重试的临时故障，错误分类与重试语义被破坏。对比同变更中 craft_docker_output.go 的 Open 已正确区分 NotFound→Conflict
与其他错误→dockerOutputDBError。建议拆分判断：

- 		if err := tx.Table("agent_runs").Select("session_id, revision").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil || run.SessionID != request.TaskID {
+ 		if err := tx.Table("agent_runs").Select("session_id, revision").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
+ 			if errors.Is(err, gorm.ErrRecordNotFound) {
+ 				return ErrCraftDockerNormalInputConflict
+ 			}
+ 			return dockerOutputDBError(err)
+ 		}
+ 		if run.SessionID != request.TaskID {
  			return ErrCraftDockerNormalInputConflict
  		}


─── internal/application/repository/craft_docker_normal_input.go:169-172 ───
[bug · medium] 与上一处同源：journal Take 的任意错误（含瞬时故障/ctx 取消）被统一归类为 ErrCraftDockerNormalInputConflict。只有
ErrRecordNotFound（journal 缺失或不属于该活动）才是真正的持久冲突语义；其他错误应作为基础设施错误传播，否则调用方会把可重试的 DB 抖动当作不可恢复的身份冲突放弃操作。

  		if err := tx.Table("craft_charge_start_journal").Select("state, protocol, provider, send_claimed_at, run_revision").Where(
  			"tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&journal).Error; err != nil {
+ 			if errors.Is(err, gorm.ErrRecordNotFound) {
- 			return ErrCraftDockerNormalInputConflict
+ 				return ErrCraftDockerNormalInputConflict
+ 			}
+ 			return dockerOutputDBError(err)
  		}


─── internal/application/repository/craft_docker_normal_input.go:210-212 ───
[bug · medium] Read 同样把 agent_runs Take 的任意错误与 SessionID 不匹配合并为 Conflict。Read 是
BindDockerNormalExecReceipt/ClaimDockerNormalExecSend 的前置路径，DB 抖动会让绑定/领取被误判为持久冲突并上抛为终态错误，误导 Agent
对该活动的处置（放弃而非重试）。建议同 Stage：NotFound 与 SessionID 不匹配返回 Conflict，其余错误经 dockerOutputDBError 传播。

- 	if err := r.db.WithContext(ctx).Table("agent_runs").Select("session_id").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil || run.SessionID != row.TaskID {
+ 	if err := r.db.WithContext(ctx).Table("agent_runs").Select("session_id").Where("tenant_id = ? AND run_id = ?", key.TenantID, key.RunID).Take(&run).Error; err != nil {
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
+ 			return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
+ 		}
+ 		return CraftDockerStagedNormalInput{}, dockerOutputDBError(err)
+ 	}
+ 	if run.SessionID != row.TaskID {
  		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
  	}


─── internal/application/repository/craft_docker_normal_input.go:222-225 ───
[maintainability · low] normal input 链路（Stage/Read/Bind/Claim）的基础设施错误统一复用
dockerOutputDBError，被包装为输出域哨兵 ErrCraftDockerOutputUnavailable（"durable Docker output is
unavailable"）。输入暂存/回执绑定失败会被日志与按哨兵分支的上层误读为"输出存储不可用"，跨域哨兵误导诊断与监控归因（craft_docker_send_claim.go 的
normalInputExists 也是同款用法）。建议为 normal input 定义专属的 Unavailable 包装哨兵，或抽出域中立的 DB 错误包装供两域共用。



─── internal/application/repository/craft_docker_normal_input.go:115-117 ───
[bug · low] len(plain) != canonicalSize 意味着 craftDockerNormalCanonicalJSONSize 与 encoding/json
的实际输出不一致，属于内部不变量破坏（计算器缺陷），却被归类为调用方输入超限（ErrCraftDockerNormalInputTooLarge）。一旦触发，排障方向会被引向"用户输入过大"而非计算器
回归。建议为该分支使用独立的内部错误哨兵（或至少 Corrupt 类别），仅将真实的 len(plain) > MaxCraftDockerNormalRequestBytes 归为
TooLarge。



─── internal/application/repository/craft_docker_send_claim.go:237-241 ───
[maintainability · low] 两处错误契约问题：1) 用 == 直接比较 gorm.ErrRecordNotFound 而非 errors.Is，若 gorm
层未来改为包装哨兵会静默失效；2) 非 NotFound 的 DB 错误未经 dockerOutputDBError
包装直接返回，与同包其他仓储的契约不一致（bindDockerExecReceipt/claimDockerExecSend 事务内与 result.Error 的裸返回同理），调用方
errors.Is/As 判断可能失真。建议改用 errors.Is 并统一包装基础设施错误（需补充 errors import）。

  	err := r.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error
- 	if err == gorm.ErrRecordNotFound {
+ 	if errors.Is(err, gorm.ErrRecordNotFound) {
  		return craftDockerSendClaimRow{}, craftDockerSendConflict("operation not found")
  	}
- 	return row, err
+ 	if err != nil {
+ 		return craftDockerSendClaimRow{}, dockerOutputDBError(err)
+ 	}
+ 	return row, nil


─── internal/modules/craftegress/adapter.go:295-308 ───
[bug · high] unknown-outcome 判定与网关实际 502 语义不匹配：网关 Forward
在"绑定已开始但物理转发失败"路径（internal/handler/craft_model_gateway.go:547-560，g.forward 返回错误、attempted=true）返回
502 + code "UPSTREAM_ERROR"，同时将 charge-start 以 CraftChargeStartUnknown 落账（消息文本即 "the upstream model
request failed with an unknown activity outcome"）。本函数只认 "ACTIVITY_UNRESOLVED"，导致 ServeHTTP
把该实际未知结局的响应判为 definitive 并 unpark 指纹；同逻辑请求重试随即铸造全新 activity 身份并发起第二次物理发送——正是本文件注释自警的 "a definitive
resolve of an actually-unknown send would mint a fresh identity on retry and bill the same logical
request twice"，且旧 unresolved 绑定被绕过 409 对账路径、只能人工清理。注释列举的 "failed forward → ACTIVITY_UNRESOLVED"
与网关实现不符（initiation timeout 与 lost response 两条路径确实带该 code，唯独 failed-forward 用
UPSTREAM_ERROR）。建议：首选在网关侧把该路径的 code 改为 ACTIVITY_UNRESOLVED（与其消息语义一致）；若需在本侧保守兜底，可对 502 的
UPSTREAM_ERROR 一并视为 unresolved（DefinitelyNotStarted 的 !attempted 路径复用同 ID 重试可重新 BeginBinding，parking
它是安全的）。

  func gatewayReportsActivityUnresolved(status int, body []byte) bool {
  	if status != http.StatusConflict && status != http.StatusBadGateway {
  		return false
  	}
  	var envelope struct {
  		Error struct {
  			Code string `json:"code"`
  		} `json:"error"`
  	}
  	if err := json.Unmarshal(body, &envelope); err != nil {
  		return false
  	}
- 	return envelope.Error.Code == "ACTIVITY_UNRESOLVED"
+ 	switch envelope.Error.Code {
+ 	case "ACTIVITY_UNRESOLVED":
+ 		return true
+ 	// The gateway's failed-forward path (forward error after the binding
+ 	// started) resolves its charge start as Unknown while labelling the 502
+ 	// UPSTREAM_ERROR; that send's outcome is unknown all the same.
+ 	case "UPSTREAM_ERROR":
+ 		return status == http.StatusBadGateway
+ 	}
+ 	return false
  }


─── internal/modules/craftegress/journal.go:56-57 ───
[maintainability · low] resolved 索引只写不读：replay()（j.resolved[record.RequestDigest] = true）与
Resolve()（j.resolved[requestDigest] = true）均写入该 map，但全仓库（含测试）没有任何读取点；"指纹已 resolved → 下次同指纹是全新
attempt" 的语义实际由"不在 unresolved 中"隐式实现。这是无消费方的死状态，且随 distinct
指纹数量单调增长（与已确认的日志无界增长发现叠加）。建议删除该字段及两处写入，或明确接入其预期用途（例如作为将来 replay 压缩时保留每指纹最新状态的依据）。

  	unresolved map[string]CraftEgressAttemptRecord // requestDigest -> unresolved attempt
- 	resolved   map[string]bool                     // requestDigest -> has resolved attempt


─── internal/application/service/craft_preview.go:297-308 ───
[security · high] AcceptsPreviewHost 的默认端口归一化与上游 PreviewOriginAllowed
的源相异校验不一致，导致同主机配置可绕过源隔离。PreviewOriginAllowed（及构造器 panic 守卫）按裸字符串比较 scheme://host：当
AppOrigin=https://app.example.com:443 而
PreviewOrigin=https://app.example.com（或反向拼写）时，两串不等被判为"不同源"，构造器不 panic、Enabled()=true；但
AcceptsPreviewHost 两侧经 trimDefaultPort 归一后均为 app.example.com——请求打在主产品 Host
上即匹配通过，挂在共享路由、位于全局鉴权中间件之前的免鉴权 /p/:cap/*filepath 在主源激活。后果正是该门禁注释要阻止的：模型生成的预览页脚本与主应用同源，可携带 Host-only
会话 Cookie 访问主应用接口。同类变体：PreviewOrigin=https://app.example.com:8443 与裸 443 主源按 RFC 是不同源（守卫放行），但 Cookie
按域不按端口共享，Secure Cookie 仍会发往
https://app.example.com:8443，同源隔离同样失效。建议在归一化一侧补齐主机名重叠拒绝（比较剥离任意端口后的裸主机名，IPv6 保留/去括号后比较）。

  func (s *CraftPreviewService) AcceptsPreviewHost(host string) bool {
  	if !s.Enabled() {
  		return false
  	}
  	u, err := url.Parse(s.config.PreviewOrigin)
  	if err != nil {
  		return false
  	}
- 	// Normalize both sides: the config side may omit the default port while
- 	// the request carries it (or vice versa).
- 	return strings.EqualFold(trimDefaultPort(host, u.Scheme), trimDefaultPort(u.Host, u.Scheme))
+ 	normalized := trimDefaultPort(u.Host, u.Scheme)
+ 	// The app origin never redeems. PreviewOriginAllowed compares raw
+ 	// scheme://host strings, so https://app.tld:443 vs https://app.tld (or a
+ 	// non-default port on the same hostname) pass that guard while sharing
+ 	// one host-scoped cookie jar with the main site — refuse the overlap
+ 	// here, where both sides normalize to bare hostnames.
+ 	if app, err := url.Parse(s.config.AppOrigin); err == nil && app.Host != "" {
+ 		if previewBareHostname(normalized) == previewBareHostname(app.Host) {
+ 			return false
+ 		}
+ 	}
+ 	return strings.EqualFold(trimDefaultPort(host, u.Scheme), normalized)
+ }
+ 
+ // previewBareHostname reduces an authority to its bare hostname: strip any
+ // port (default or not) and IPv6 brackets, so host-scoped cookie overlap is
+ // detected regardless of port spelling.
+ func previewBareHostname(authority string) string {
+ 	h := strings.TrimSpace(authority)
+ 	if inner, _, err := net.SplitHostPort(h); err == nil {
+ 		h = inner
+ 	}
+ 	return strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(h, "["), "]"))
  }


─── internal/application/repository/craft_docker_normal_input.go:344-345 ───
[bug · medium] craftDockerJSONQuotedStringSize 对 \b(0x08) 和 \f(0x0C) 的转义长度计算错误：Go encoding/json
的字符串编码器只对 \\、\"、\t、\n、\r 使用 2 字节短转义，\b 和 \f 与其他 <0x20 控制字符一样输出 \u0008/\u000c（6 字节，encode.go
注释明确："This encodes bytes < 0x20 except for \t, \n and \r"）。此处按 2 字节计，每处出现低估 4
字节。后果：请求任一字符串字段（Command 参数、Environment 键值、User、WorkingDir 等）含 U+0008/U+000C
时（validCraftDockerNormalInput 只禁 \x00，允许这两者），Stage 中 craftDockerNormalCanonicalJSONSize 与
json.Marshal 实际输出必然不一致，合法请求 100% 被 len(plain) != canonicalSize 分支拒绝，且被归为
ErrCraftDockerNormalInputTooLarge（哪怕请求只有几百字节，严重误导排障方向）；decryptCraftDockerNormalInput 的同一校验也会将其误判为
Corrupt。建议将 \b、\f 从 2 字节 case 移除，由 r < 0x20 分支（6 字节）覆盖：case r == '"' || r == '\\' || r == '\n' || r
== '\r' || r == '\t': size += 2。该计算器目前无测试覆盖，建议补充与 json.Marshal 逐字节对比的用例（含 \b/\f/<>/&/U+2028/无效
UTF-8）。

- 	case r == '"' || r == '\\' || r == '\b' || r == '\f' || r == '\n' || r == '\r' || r == '\t':
+ 	case r == '"' || r == '\\' || r == '\n' || r == '\r' || r == '\t':
  		size += 2


─── internal/modules/craft/input_code.go:98-98 ───
[documentation · low] Reason 枚举注释与实现漂移：Review() 的 InterpreterScanInconclusive 分支会产出
`wrapper_shape`（p.deny("wrapper_shape", ...)），但该字段注释声明的稳定机器码枚举（input_target | interpreter_input |
shell_input | input_symlink | input_identity）未收录它。Reason
是下游审计/分析依赖的稳定标识，注释即契约，建议补齐枚举，避免消费方按注释穷举时漏掉该拒绝原因。

- 	Reason    string // input_target | interpreter_input | shell_input | input_symlink | input_identity
+ 	Reason    string // input_target | interpreter_input | shell_input | input_symlink | input_identity | wrapper_shape


─── packages/views/src/craft/sources.tsx:257-257 ───
[maintainability · low] 引用事实列表项使用 key={index}：条目自身有天然稳定标识 citationId（同一 JSX 中 data-craft-citation
已使用）。清单内容随交付物 manifest 变化时（如服务端补发/过滤条目），索引键会导致 React 复用错误的 DOM/状态。建议事实项改用
key={entry.citationId}；inference 条目无稳定 id，维持 inference-${index} 即可。

-                 <li key={index} className="wk-craft-citation-fact" data-craft-citation={entry.citationId}>
+                 <li key={entry.citationId} className="wk-craft-citation-fact" data-craft-citation={entry.citationId}>


─── internal/application/service/craft_budget.go:825-830 ───
[bug · medium] AuthorizeSandbox 的序列分配循环对任意 Create 错误一律 continue，未像同文件新增的
prepareCraftChargeStartWithProtocol 那样先用 isUniqueViolation(err) 判别。连接中断、ctx 已取消、死锁等真实数据库错误会被静默重试 16
次（craftCallSeqAttempts），最终误报为 "sandbox call sequence
contention"（ErrConflict），既掩盖根因又放大失败延迟。建议仅对唯一索引冲突重试，其余错误立即返回。

  			if err := s.db.WithContext(ctx).Create(&call).Error; err != nil {
- 				// A concurrent sandbox authorization took this sequence slot;
+ 				// Only a unique-index loss means a concurrent sandbox authorization
+ 				// took this sequence slot; any other database failure must surface
+ 				// immediately instead of being retried into a misleading conflict.
+ 				if !isUniqueViolation(err) {
+ 					return err
+ 				}
  				// re-read the fresh maximum and retry with the next one.
  				continue
  			}
  			allocated = true


─── internal/application/service/craft_budget.go:341-344 ───
[maintainability · low] StartBinding 在 resolveCraftChargeStart 失败时返回 resolveErr 并丢弃
startErr（外部回调的原始错误），而 BeginBinding 的对应路径已使用 errors.Join(err, resolveErr)
合并两个错误。此处不一致导致调用方排障时无法感知外部回调失败原因，建议同样合并：return outcome, errors.Join(startErr, resolveErr)。

  	if resolveErr := s.resolveCraftChargeStart(resolveCtx, journal, outcome); resolveErr != nil {
- 		return outcome, resolveErr
+ 		return outcome, errors.Join(startErr, resolveErr)
  	}
  	return outcome, startErr


─── packages/views/src/craft/usage.tsx:18-19 ───
[style · low] 新增的 React 默认导入在文件内没有任何 React.* 命名空间使用（已全文检索确认），且 apps/web 与 apps/desktop 的 tsconfig
均配置 jsx: "react-jsx"，JSX 转换不依赖显式 React 导入。该默认导入属死代码，建议移除，仅保留 type { CraftRunView } 导入。

- import React, { useCallback, useEffect, useState } from 'react';
+ import { useCallback, useEffect, useState } from 'react';
  import type { CraftRunView } from '@weknora/contracts';


─── internal/handler/session/artifact_download.go:813-816 ───
[security · high] 盘符兜底只匹配小写 a–z，`C:/evil`（大写盘符）可绕过。核证：真正的模块级校验
craft.ValidateArtifactPath（version.go:148-167）只拒绝反斜杠/NUL、前导 "/"、空/./..元素与凭据名，对首段 "C:" 完全放行——即该
handler 检查是下载路径上唯一的盘符防线，而它漏掉了大写。结果：一个被污染/恶意的沙箱输出可把 `C:/evil` 成员发布进版本（发布侧 ManifestDigest 同样走
ValidateArtifactPath，不拦截），导出时该名字原样写入 zip，在 Windows 资源管理器或旧式解压器中解析为驱动器根下的绝对目标，逃逸出解压目录（zip-slip
变体）。archive.go 自己的 isDriveLetter 就是双大小写实现，此处应保持一致；更优做法是把盘符拒绝下沉到 ValidateExportBundleMembers（见对
export_manifest.go 的同轮意见），使拒绝发生在写出 200 头之前——本检查位于 member 循环内，已在响应头发出之后，与注释 "Refuse it before the 200
head is written" 的声明不符。

- 		if first := member.Path; len(first) >= 2 && first[0] >= 'a' && first[0] <= 'z' && first[1] == ':' {
+ 		if first := member.Path; len(first) >= 2 && first[1] == ':' &&
+ 			((first[0] >= 'a' && first[0] <= 'z') || (first[0] >= 'A' && first[0] <= 'Z')) {
  			abortDownload("craft export bundle member %q uses a drive-letter path", member.Path)
  			continue
  		}


─── internal/modules/craft/export_manifest.go:271-277 ───
[security · medium] 建议在 ValidateExportBundleMembers 中直接拒绝盘符首段（大小写两种形态），使该拒绝成为打包前的服务端防线而非 handler 在
200 头之后的兜底。核证：本函数调用的 ValidateArtifactPath（version.go:148-167）对首段 "C:"/"c:" 均不拒绝（它只查反斜杠/NUL、前导
"/"、空/./..元素与凭据名），因此 `C:/evil` 这类成员能通过本函数进入 DownloadCraftExportBundle 的 member 循环；同文件 archive.go 的
ValidateArchiveEntryPath 已用双大小写的 isDriveLetter
拒绝同一形态，本模块的两处路径校验口径不一致。在此处补齐后，恶意成员在任何字节被打包、任何响应头写出之前即被整体拒绝，与函数 docstring "refuses the whole bundle
before any byte is packaged" 的承诺一致（handler 侧建议同步修复其小写-only 匹配，作为纵深防御）。

  	for _, member := range files {
  		if err := ValidateArtifactPath(member.Path); err != nil {
  			return err
+ 		}
+ 		if first := member.Path; len(first) >= 2 && first[1] == ':' &&
+ 			((first[0] >= 'a' && first[0] <= 'z') || (first[0] >= 'A' && first[0] <= 'Z')) {
+ 			return fmt.Errorf("%w: bundle member %q uses a drive-letter path", ErrInvalidInput, member.Path)
  		}
  		if reserved[member.Path] {
  			return fmt.Errorf("%w: bundle member %q collides with a fixed bundle document", ErrInvalidInput, member.Path)
  		}


─── internal/application/service/craft_archive.go:183-185 ───
[bug · high] cleanup 预算启动时机错误，超时路径失败时已上传对象将静默泄漏。核证：cleanupCtx 在成员上传循环（最多 20 次 Take + SaveBytes，累计可达
MaxArchiveExpandedBytes=100MiB）与发布事务之前创建，预算 30s（craftInputCleanupBudget，craft_inputs.go:30）与整个请求的
30s 提取预算（MaxArchiveExtractDuration）等长——当失败正是注释自述的最常见形态（ctx 超时/取消，例如对象后端慢导致在 30s 死线附近失败）时，rollback 里的
Count+DeleteFile 只剩接近 0 的预算，立即 context deadline exceeded，全部已存储对象泄漏；且 `_ = s.files.DeleteFile(...)`
把删除错误完全丢弃，连告警日志都没有。同 diff 组的 craft_inputs.go:196-218 对完全相同的陷阱已修复并在注释中写明缘由（"The budget starts HERE,
not when the upload loop began… would expire the Count/DeleteFile calls below immediately and
silently leak every stored object"），本新文件复刻了旧缺陷。修复：把 WithTimeout 移入 rollback 闭包内启动，并对 DeleteFile 失败记
Warnf（需补 logger import）。

+ 	rollback := func() {
+ 		// 预算必须在 rollback 内启动（对齐 craft_inputs.go 的同款修复）：
+ 		// 上传循环与发布事务消耗预启动预算后，超时驱动的失败会让
+ 		// Count/DeleteFile 立即到期，静默泄漏全部已存储对象。
- 	cleanupCtx, cleanupDone := context.WithTimeout(context.WithoutCancel(ctx), craftInputCleanupBudget)
+ 		cleanupCtx, cleanupDone := context.WithTimeout(context.WithoutCancel(ctx), craftInputCleanupBudget)
- 	defer cleanupDone()
+ 		defer cleanupDone()
- 	rollback := func() {


─── internal/modules/craft/archive.go:260-265 ───
[performance · medium] zip 中央目录的内存物化发生在任何条目上限生效之前，与模块声称的内存上限推理不符。核证：zip.NewReader 返回前会完整解析中央目录并物化整个
reader.File 切片；MaxInputBytes=20MiB 的输入在纯最小条目（每条中央记录 ≥46 字节）下可携带约 45 万个条目，先分配约 45 万个 zip.File 结构（含
Name 字符串，约百 MB 量级瞬时内存，约为输入体积的 5 倍以上），然后才在迭代第 21 项时被 reserveEntry 的 MaxArchiveEntries=20
拒绝。reserveEntry 的注释只论证了 seen 集合内存有界（"a malicious central directory can name hundreds of thousands of
directory entries…memory stays bounded by the same constant"），恰好漏掉了 reader.File
本身；ExtractArchive/ExpandArchive 文档中 "memory is bounded because members are buffered only up to the
cumulative expanded-byte ceiling" 的声明对该向量不成立。注意：NewReader 之后检查无法避免已发生的分配，只能避免后续处理；彻底规避需在调用 NewReader
前解析尾部 EOCD 的条目计数字段做预检。至少应在 NewReader 后立即按条目数快速失败（用宽裕常量做分配合理性上限，保留逐条目的精确语义，不改变现有接受行为），并修正文档声明。

  func extractZipArchive(data []byte) ([]ArchiveMember, error) {
  	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
  	if err != nil {
  		return nil, fmt.Errorf("%w: malformed zip container: %v", ErrInvalidInput, err)
+ 	}
+ 	// NewReader 已物化整个中央目录；至少立即按条目数快速失败，
+ 	// 避免继续为海量条目做逐项处理（真正的分配上限需在 NewReader
+ 	// 前预解析 EOCD 条目计数）。宽裕常量仅作 sanity cap，逐条目的
+ 	// 精确上限仍由 reserveEntry 执行。
+ 	if len(reader.File) > maxZipCentralEntries {
+ 		return nil, fmt.Errorf("%w: zip central directory lists %d entries over the sanity cap %d",
+ 			ErrInvalidInput, len(reader.File), maxZipCentralEntries)
  	}
  	budget := newArchiveBudget(int64(len(data)))


─── internal/application/service/craft_docker_normal_exec.go:62-67 ───
[security · medium] 注释宣称 "The attach is logged so a deployment that forgot to wire the security gate
is observable in its logs"，但函数体没有任何 logger 调用；且 `policy != nil` 守卫使 nil 闸门注入成为静默 no-op。第 1 轮
OCR（docs/plans/craft-107-ocr-t03.md:157-162）明确要求注入时输出启动日志使"安全特性未接线"可检测——本修复只添加了承诺日志的注释而未实现日志。全库检索确认
NewCraftDockerNormalExecService/WithExecutionPolicy 目前仅测试调用、生产装配点尚不存在：一旦后续装配 PR 遗漏挂接 T03
闸门（#122，"上传代码仅作数据不执行"），无编译错误、无日志可发现，安全审查将被静默绕过。建议：attach 成功与 policy==nil 两个分支均落一条日志（或改由构造参数强制注入）。

  func (s *CraftDockerNormalExecService) WithExecutionPolicy(policy CraftExecutionPolicyGate) *CraftDockerNormalExecService {
- 	if s != nil && policy != nil {
+ 	if s == nil {
+ 		return s
+ 	}
+ 	if policy != nil {
  		s.policy = policy
+ 		return s
  	}
+ 	logger.Warnf(context.Background(), "[CraftDockerNormalExec] T03 execution policy gate NOT attached; uploaded-material screening is inactive")
  	return s
  }


─── internal/application/service/craft_docker_restricted_exec.go:58-63 ───
[security · medium] 与 craft_docker_normal_exec.go 相同的模式：注释宣称 "The attach is logged so a deployment
that forgot to wire the security gate is observable in its logs"，但函数体没有 logger 调用，且 `policy != nil`
守卫使 nil 注入静默失效。第 1 轮 OCR 裁决要求注入时输出启动日志使未接线可检测；当前生产装配点不存在，后续装配 PR 遗漏挂接时该受限发送面将无任何可观测信号。建议与 normal
exec 服务同修：attach 与 nil 两个分支均落日志。

  func (s *CraftDockerRestrictedExec) WithExecutionPolicy(policy CraftExecutionPolicyGate) *CraftDockerRestrictedExec {
- 	if s != nil && policy != nil {
+ 	if s == nil {
+ 		return s
+ 	}
+ 	if policy != nil {
  		s.policy = policy
+ 		return s
  	}
+ 	logger.Warnf(context.Background(), "[CraftDockerRestrictedExec] T03 execution policy gate NOT attached; uploaded-material screening is inactive")
  	return s
  }


─── internal/application/service/craft_docker_restricted_exec.go:187-188 ───
[performance · medium] Wait 以固定 20ms ticker 轮询，且每轮 s.Observe 都触发 coordinator.Observe（loadGrant 一次 DB
查询 + journal 一次 DB 查询）再加一次 Docker daemon HTTP ExecInspect（docker_restricted_exec.go:123），即约 150
次网络/DB 往返每秒每等待者，持续整个 maxWait；30s 等待约 4500 次 DB 查询 + 1500 次 inspect。仓库 DB 轮询惯例为
100-500ms（agent_run.go 250ms、qa.go 500ms），且此处无退避。建议：采用递增退避（如首个 1s 内 20ms，之后升至
200ms+）或直接提高基准间隔；grant/journal 的 durable 重读若为撤销检测所需可保留，但频率应随退避下降。

- 	ticker := time.NewTicker(20 * time.Millisecond)
+ // Escalating poll interval: responsive at first, then back off to avoid
+ // 50Hz DB+inspect pressure over long waits.
+ interval := 20 * time.Millisecond
+ afterFast := time.After(time.Second)
+ ticker := time.NewTicker(interval)
- 	defer ticker.Stop()
+ defer ticker.Stop()
+ // in the select:
+ // case <-afterFast:
+ // 	ticker.Reset(200 * time.Millisecond)


─── internal/modules/craft/input_code.go:409-415 ───
[security · high] 非解释器分支仅做路径 containment 筛查，未拒绝 find 的执行转发标志，存在词法干净的绕过路径：`find . -type f -name
'*.py' -exec python3 {} \;`（或 `-execdir`、`-exec sh {} +`）的 argv 中没有任何指向 inputs 树的 token——`-exec` 经
flagValueCandidates 只产生 exec/xec/ec/c 等不落树的假值候选，`python3`、`{}`、`;` 均为普通操作数——策略放行（Allowed=true，审计为
generated.execute）；运行时 find 遍历 workspace 根必然覆盖 <root>/inputs 只读树，将上传文件路径填入 `{}`
交给解释器执行上传字节，违反本文件声明的硬边界（"no interpreter, shell, copy, link or alias path may execute the uploaded
bytes"）。与 glob/xargs-stdin 等依赖 adapter 证据契约兜底的残余风险不同，这条路径是纯 argv 形式、无
stdin、无运行时展开依赖，策略层自身即可封堵。建议在操作数筛查中对任意命令出现 `-exec`/`-execdir`（含 `-execdir{}` 附着形式）时按不可审查的执行转发直接
fail-closed 拒绝，语义与 shell 分支一致。

+ 		// Execution-forwarding flags are unreviewable: find -exec/-execdir
+ 		// fills its {} with paths enumerated at RUN time (including the
+ 		// read-only tree), so a lexically clean argv still executes uploaded
+ 		// bytes. Fail closed like the shell branch.
+ 		for _, arg := range req.Command[1:] {
+ 			if arg == "-exec" || arg == "-execdir" || strings.HasPrefix(arg, "-execdir") || (strings.HasPrefix(arg, "-exec") && len(arg) > len("-exec") && arg[len("-exec")] == '{') {
+ 				return p.deny("input_target", arg, "")
+ 			}
+ 		}
  		if !readOnlyCommands[path.Base(req.Command[0])] {
  			for _, arg := range req.Command[1:] {
  				for _, token := range shellTokens(arg) {
  					if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
  						return p.deny("input_target", abs, "")
  					}
  				}


─── internal/application/service/craft_budget.go:1245-1248 ───
[bug · low] BudgetPause 的 grant 读取把任意错误（含瞬时数据库故障）统一折叠为 craft.ErrNotFound，而紧随其后的 run
查询注释已明确声明相反纪律："数据库失败不是'无此暂停'的领域结论，应向上传播以便调用方重试"。瞬时 DB 故障会让 API 对确实处于暂停的 Run 返回"暂停不存在"（通常映射 404），且
ExtendAndResume 以 BudgetPause 成功为恢复前置，同样会被误导为 NotFound 而非可重试错误。建议与非授权路径一致地区分 NotFound 与其他错误。

  	var grant CraftBudgetGrantRow
  	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).Take(&grant).Error; err != nil {
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
- 		return craft.BudgetPause{}, craft.ErrNotFound
+ 			return craft.BudgetPause{}, craft.ErrNotFound
+ 		}
+ 		return craft.BudgetPause{}, err
  	}


─── internal/application/service/craft_budget.go:1300-1306 ───
[bug · low] ExtendAndResume 的 grant 读取存在与 BudgetPause 相同的错误折叠：任意数据库错误（连接中断、超时）都被当作 craft.ErrNotFound
返回，管理员对一个确实暂停中的 Run 发起"增加预算并恢复"会得到"未找到"而非可重试的 5xx，掩盖了瞬时故障并可能误导排障方向。建议同样区分 gorm.ErrRecordNotFound
与其他错误。

  	var grant CraftBudgetGrantRow
  	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND run_id = ?", scope.TenantID, runID).Take(&grant).Error; err != nil {
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
- 		return craft.ErrNotFound
+ 			return craft.ErrNotFound
+ 		}
+ 		return err
  	}
  	if _, err := s.BudgetPause(ctx, scope, runID); err != nil {
  		return err
  	}


─── internal/application/service/craft_archive.go:197-197 ───
[maintainability · low] rollback 中 DeleteFile 的失败被 `_ =` 完全静默丢弃，无任何日志。同一提交里姊妹路径 craft_inputs.go 的
AcceptInputRound rollback 已把完全相同的模式升级为记录 Warnf（其注释明确指出 "the errors were previously discarded
outright" 会导致对象静默泄漏、只能等 O03 兜底回收）。归档扩展路径沿用了旧模式：对象回滚失败（对象后端瞬时故障、ref 被并发关联等）零可观测性，泄漏既不出现在日志也无指标，运维无法与
confirmed finding 中超时泄漏路径区分归因。建议与 AcceptInputRound 对齐，记录删除失败日志。

- 			_ = s.files.DeleteFile(cleanupCtx, createdRef)
+ 			if err := s.files.DeleteFile(cleanupCtx, createdRef); err != nil {
+ 				logger.Warnf(cleanupCtx, "[CraftArchive] rollback delete failed for ref %s (object may leak until reclamation): %v", createdRef, err)
+ 			}
+ // 并补充 import "github.com/Tencent/WeKnora/internal/logger"


─── internal/modules/craft/citation.go:167-171 ───
[bug · low] 严格解码未拒绝尾随数据：json.Decoder.Decode 只消费输入中的第一个 JSON
值，`{"schema":1,"entries":[…]}\n{"schema":2,…}` 或 `{…}garbage`
会静默按第一个值通过，尾随的第二个文档/垃圾被丢弃。这与函数自身的严格契约（DisallowUnknownFields —— "unknown fields are rejected rather
than silently dropped"）不一致：citations.json 是模型生成的不可信输入，其原始字节会原样进入不可变版本，模型可用尾随内容夹带一份绕过结构审计的影子
manifest。建议在 Decode 成功后要求输入流已耗尽（decoder.More() 为 false，或再 Decode 一个哑值必须得到 io.EOF），使严格解码闭合。

- 	decoder := json.NewDecoder(bytes.NewReader(data))
- 	decoder.DisallowUnknownFields()
  	if err := decoder.Decode(&m); err != nil {
  		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest decode: %v", ErrInvalidInput, err)
+ 	}
+ 	if decoder.More() {
+ 		return WebCitationManifest{}, fmt.Errorf("%w: web citation manifest carries trailing data", ErrInvalidInput)
  	}


─── internal/container/craft_lifecycle.go:56-59 ───
[maintainability · low] 该 Warn 日志与实际行为不符：bindings 为 nil 时，下方 service.NewCraftLifecycle 会在
CraftLifecycleConfig.Bindings 为 nil 的校验处直接返回错误（service/craft_lifecycle.go:200-203 "craft: lifecycle
requires db, store, bindings, ..."），newCraftLifecycleService 随即返回 nil, lerr，DI
解析失败（启动中止）——并不是日志所称的"清扫保持惰性但仍继续运行"。该路径仅在共享单例构建失败（如 WEKNORA_REDIS_NAMESPACE 含非法字符导致
NewRedisSessionSandboxBindingStore 报错、sandbox.go:116 返回
nil）时可达，届时运维会先看到这条具有误导性的日志、再遭遇与之矛盾的启动失败。行为本身是更严格的 fail-closed（正确），建议修正日志文案以反映真实后果，避免误导排障。

  	if bindings == nil {
  		logger.Warnf(context.Background(),
- 			"[CraftLifecycle] shared binding store unavailable; lifecycle binding sweep stays inert (fail-closed)")
+ 			"[CraftLifecycle] shared binding store unavailable; lifecycle assembly will fail closed (nil bindings are rejected by NewCraftLifecycle)")
  	}


─── internal/application/service/craft_docker_normal_exec.go:205-209 ───
[performance · low] started map 只在 terminal 进程观察时删除 key,但存在可达的永久驻留路径:StartAttachedExecOnce 返回
startErr(如 attach 网络错误)且 Observation.State 为 Running/Unknown 时 key 被置位;此后若调用者不再 replay(Execute 返回
RemoteOperationUnknown 后上层放弃),或后续 observeClaimed 的 ObserveAttachedExec 持续失败(容器被删除导致 ExecInspect 永久
404、daemon 长期不可达),该 exec 永远不会产生 terminal 观察,key 便永久驻留。这与注释声明的意图("the long-lived service must not
retain one key per exec forever")不符——该意图目前只在 terminal 路径实现。长生命周期服务在系统性故障期间(每次失败 Execute 泄漏一个
key)会无界累积。建议为 started 增加容量上限(超限时丢弃最旧/全部 evidence,代价只是跨 replay 的 terminal 归因退化为
Unknown,与进程重启后的既有语义一致),或记录时间戳做 TTL 清理。

- 	if outcome.StartEvidence {
+ const normalStartEvidenceMaxEntries = 4096
+ 
+ func (s *CraftDockerNormalExecService) markStartEvidence(key string) {
- 		s.mu.Lock()
+ 	s.mu.Lock()
+ 	defer s.mu.Unlock()
+ 	if s.started == nil {
+ 		s.started = make(map[string]bool)
+ 	}
+ 	if len(s.started) >= normalStartEvidenceMaxEntries {
+ 		// Bound growth: entries whose exec can never reach a terminal
+ 		// observation (deleted container, unreachable daemon) would stay
+ 		// forever. Dropping evidence only degrades cross-replay terminal
+ 		// attribution to Unknown, the same as a process restart.
+ 		for k := range s.started {
+ 			delete(s.started, k)
+ 			if len(s.started) < normalStartEvidenceMaxEntries/2 {
+ 				break
+ 			}
+ 		}
+ 	}
- 		s.started[key] = true
+ 	s.started[key] = true
- 		s.mu.Unlock()
- 	}
+ }
+ 
+ // 使用:
+ // if outcome.StartEvidence {
+ // 	s.markStartEvidence(key)
+ // }


─── internal/application/service/craft_docker_restricted_exec.go:311-314 ───
[performance · low] running map 与 normal exec 的 started map 存在同样的无界驻留缺陷:running[key] 只在 terminal
观察(Succeeded/Failed)时删除,而注释明确 Unknown 观察会保留 flag。可达的永久驻留场景:exec 曾被观察到 Running 后容器被删除,此后
ObserveOutputlessExec 对该 receipt 持续返回 404 错误 → Observe 提前 return unknown, err,state 永远到不了 terminal
分支,key 永久驻留。Craft 沙箱容器属短生命周期资源,删除是常态操作,长生命周期服务中每个这类 exec 泄漏一个 key(grantID+activityID 字符串)。建议与 normal
exec 服务的 started map 一致地增加容量上限或 TTL——丢弃该 flag 只会让 "was Running" 归因退化为 Unknown(与进程重启后既有语义一致),安全方向不变。

+ const outputlessRunningMaxEntries = 4096
+ 
+ func (s *CraftDockerRestrictedExec) markRunning(key string) {
- 		s.mu.Lock()
+ 	s.mu.Lock()
- 		delete(s.running, key)
- 		s.mu.Unlock()
+ 	defer s.mu.Unlock()
+ 	if s.running == nil {
+ 		s.running = make(map[string]bool)
+ 	}
+ 	if len(s.running) >= outputlessRunningMaxEntries {
+ 		// Bound growth: a container deleted after a Running observation
+ 		// can never produce a terminal read, so its key would stay
+ 		// forever. Losing the flag only degrades attribution to Unknown,
+ 		// the same as a process restart.
+ 		for k := range s.running {
+ 			delete(s.running, k)
+ 			if len(s.running) < outputlessRunningMaxEntries/2 {
+ 				break
+ 			}
+ 		}
+ 	}
+ 	s.running[key] = true
- 	}
+ }
+ 
+ // 使用(替换 Observe 中的 s.running[key] = true):
+ // if observed.State == sandbox.DockerOutputlessRunning {
+ // 	s.markRunning(key)
+ // }


─── internal/application/service/craft_share.go:139-140 ───
[performance · low] io.ReadAll 对版本内 citations.json 对象无上限读取：本仓库所有 GetFile 读取均以 io.LimitReader 封顶（如
craft_snapshot.go:408 用记录的 o.Bytes+1，craft_run_capture.go:283 用 MaxFileBytes+1），而此处 craft.File 本身携带
Bytes int64 却未使用。虽然捕获侧有 MaxFileBytes、准入侧有清单校验约束正常路径大小，但对象存储损坏或 Ref 错位时这是请求路径上的无界内存读取点，建议按同一惯例用
file.Bytes+1 封顶。

- 		raw, err := io.ReadAll(reader)
+ 		raw, err := io.ReadAll(io.LimitReader(reader, file.Bytes+1))
  		reader.Close()


─── internal/application/service/craft_delegate.go:402-405 ───
[security · low] decision.Target 经 %s 直接拼入 Info 日志，但 Target
是"匹配到的规范路径或摘要"：其路径成分可源自上传材料（ValidateInputName 只拒绝
`/`、`\`、NUL，不拒绝换行等控制字符），沙箱内观察到的执行目标文件名也可含换行。logger.Infof 直接透传 logrus（无控制字符清洗），攻击者可用换行伪造后续日志行（log
forging）。审计行 writeAuditRow 经 json.Marshal 已安全，建议对这两处日志字段做单行化（如 %q 或清洗控制字符）。

  	logger.Infof(ctx,
- 		"[CraftMaterial] kind=%s allowed=%v reason=%s tenant=%d session=%s run=%s workspace=%s target=%s",
+ 		"[CraftMaterial] kind=%s allowed=%v reason=%s tenant=%d session=%s run=%s workspace=%s target=%q",
  		decision.AuditKind, decision.Allowed, decision.Reason,
  		p.scope.TenantID, p.scope.SessionID, p.runID, p.workspaceID, decision.Target)


─── internal/application/service/craft_delegate.go:419-421 ───
[security · low] event.Target = in.Ref，而 ValidateInputManifest 对 Ref
仅做非空校验（strings.TrimSpace(in.Ref) != ""），成员可声明含换行/ANSI 转义序列的 Ref 并经 %s 注入 Info 日志行，造成日志伪造/解析污染。该文件自身的
WarnWithFields 注释也指出审计相关事件宜用结构化字段而非自由文本。建议改用 %q 或对 Ref 做控制字符清洗。

  	logger.Infof(ctx,
- 		"[CraftMaterial] kind=%s tenant=%d session=%s run=%s workspace=%s ref=%s digest=%s",
+ 		"[CraftMaterial] kind=%s tenant=%d session=%s run=%s workspace=%s ref=%q digest=%s",
  		event.Kind, p.scope.TenantID, p.scope.SessionID, p.runID, p.workspaceID, event.Target, event.Digest)


─── internal/application/service/craft_share.go:124-127 ───
[bug · high] 成员读取路径在生产装配下会整体 403。contribution 以调用者 scope 调用 versions.Get，但容器注入的唯一 craft.VersionStore
实现（container.go:530 → repository.NewCraftVersionStore）在
authorizeCraftVersion（internal/application/repository/craft_version.go:110）强制 ws.OwnerID ==
scope.UserID，而 craft workspace 固定属于 session owner（craft_session.go:348 ownerScopeOf 的注释明确『the
workspace always belongs to the session owner』，且 craft_session.go:1096 读版本时特意改用
ownerScopeOf(session)）。结果是：collaborator/viewer 通过 RequireTaskAccess(TaskRead) 后，versions.Get 直接返回
ErrForbidden → 403，本文件声明的契约 "ShareView projects one version's consent summary to a task member (a
TaskRead fact)"、CraftShareView "projected to members" 以及 ShareAuthority（T13/T20 成员侧查询门）在生产装配下只有
owner 本人可用。服务测试与 HTTP 测试均使用忽略 scope 的桩 Store（craft_share_test.go t11VersionStore、handler
t11HTTPVersionStore），掩盖了该 ACL 冲突。建议：TaskRead 校验通过后按任务 owner 的 scope 读取版本——在 CraftShareConfig 增加
session-owner 解析端口（sessions.user_id），或在 craft_share_wiring.go 注入一个先解析 ownerScope 再转发的版本读取适配器，与
session service 的读法保持一致；并补一条真实 store 下成员读取的用例。

- 	version, err := s.versions.Get(ctx, scope, strings.TrimSpace(versionID))
+ 	// TaskRead 已通过；workspace/version 持久层属于 session owner
+ 	// （authorizeCraftVersion 是 owner-only ACL），按 owner scope 读取，
+ 	// 与 craft_session.go 的 ownerScopeOf 读法一致。
+ 	ownerScope, err := s.ownerScopeOfTask(ctx, scope)
+ 	if err != nil {
+ 		return craft.RestrictedContribution{}, err
+ 	}
+ 	version, err := s.versions.Get(ctx, ownerScope, strings.TrimSpace(versionID))
  	if err != nil {
  		return craft.RestrictedContribution{}, err
  	}


─── packages/views/src/craft/craft.css:160-161 ───
[maintainability · low] share.tsx 中 `<span className="wk-craft-share-expiry">` 用于渲染有效期，但
craft.css（及全仓样式）没有任何 .wk-craft-share-expiry
规则；同面板的其余类名（version/digest/state/notice/awaiting/decision/error/actions/confirm/decline/revoke）均有对应规
则，符合本文件『All static styling lives here』的纪律。补一条最小规则或去掉该类名，避免悬空样式钩子。

  .wk-craft-share-notice, .wk-craft-share-awaiting, .wk-craft-share-decision { color: var(--craft-secondary-text); margin: 0; }
  .wk-craft-share-decision code { overflow-wrap: anywhere; }
+ .wk-craft-share-expiry { color: var(--craft-secondary-text); }


─── internal/modules/execution/sandbox/docker_normal_exec.go:292-297 ───
[bug · medium] StdinEnabled 的常规 exec 在 TLS 远程 Docker daemon 配置下必然失败:租户 Docker 配置支持
TLSCertPath(internal/modules/execution/sandbox/docker_engine.go:200-213,配 *tls.Conn 的 hijacked 连接),而
tls.Conn 不实现 CloseWrite(),该类型断言在 TLS 场景恒为 false,导致所有带 stdin 的执行在写入 stdin 前直接报错。对比旧路径
streamExec(docker_remote_client.go:765-769)将 CloseWrite 作为可选降级("if ok"
才半关闭),此处将其升级为硬性门槛属于行为回归。且失败发生在 ExecAttach 已成功(命令已在容器内启动等待 stdin)、durable send claim
已消耗之后,用户侧表现为命令执行失败并烧掉一次预算 claim。建议参照 streamExec 的做法:CloseWrite 不可用时降级跳过半关闭(io.Copy 写完后依赖 closeStream
关闭写端使容器侧 stdin 收到 EOF),或至少在 CreateAttachedExec 阶段按 endpoint scheme 显式拒绝 StdinEnabled
请求并给出明确错误,避免启动后的中间态。

  			closer, ok := attached.Conn.(interface{ CloseWrite() error })
  			if !ok {
- 				result.err = errors.New("Docker hijacked stream does not support stdin half-close")
- 				inputChannel <- result
- 				return
+ 				// TLS-backed hijacked conns (*tls.Conn) have no CloseWrite.
+ 				// Degrade like streamExec instead of failing every StdinEnabled
+ 				// exec: skip the half-close and let closeStream end the write
+ 				// side so the container sees stdin EOF on connection teardown.
+ 				logger.Warnf(startCtx, "[docker-normal-exec] hijacked stream lacks CloseWrite; relying on stream close for stdin EOF")
  			}


─── internal/modules/execution/sandbox/remote_operation.go:58-63 ───
[maintainability · low] RemoteOperationError 的 Error() 无论 State 取值如何都固定输出 "remote operation outcome
unknown: " 前缀,而同类型的 Is() 却严格只在 State == RemoteOperationUnknown 时才匹配
ErrRemoteOperationUnknown——错误消息与状态语义自相矛盾。当前所有构造点(craft_docker_normal_exec.go:410、craft_docker_restri
cted_exec.go:272/276 及本文件 unknownOperationError)State 均为 Unknown,尚无实际错误输出失真;但该类型携带 State
字段的目的就是允许适配器表达非 Unknown 状态(如 Failed),一旦有构造点使用,日志与告警归因将系统性误导排障。建议让前缀跟随 State。

  func (e *RemoteOperationError) Error() string {
- 	if e == nil || e.Err == nil {
+ 	if e == nil {
  		return ErrRemoteOperationUnknown.Error()
  	}
- 	return fmt.Sprintf("%s: %v", ErrRemoteOperationUnknown, e.Err)
+ 	prefix := ErrRemoteOperationUnknown.Error()
+ 	if e.State != "" && e.State != RemoteOperationUnknown {
+ 		prefix = fmt.Sprintf("remote operation %s", e.State)
+ 	}
+ 	if e.Err == nil {
+ 		return prefix
+ 	}
+ 	return fmt.Sprintf("%s: %v", prefix, e.Err)
  }


─── internal/handler/craft_model_gateway.go:584-586 ───
[security · medium] 信息泄漏 + 破坏本文件自述的脱敏纪律：appFail 将 message 原样写入 HTTP 响应体（app_connector.go:32-34），此处
errors.Join(failure, resolveErr).Error() 把 readErr/closeErr（上游网络错误，典型形式如 `read tcp
10.x.x.x:port->...` 可暴露内网 IP/端口与上游拓扑）以及 resolveErr（GORM/DB 错误，可能引用表名/SQL，正如 523-525 行注释自述 "which may
quote SQL or table names"）原样返回给客户端。同函数 521-529 行与 recordCall
都明确执行"客户端只得不透明码、细节留服务端日志"的纪律，此分支却反其道而行；且它是唯一没有 logger.ErrorWithFields 记录 Resolve
失败的路径——客户端可见、服务端反而无日志。建议：failure 与 resolveErr 均记入服务端日志（带 run/call/attempt 身份），客户端仅返回固定文案。

  		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartUnknown)
- 		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", errors.Join(failure, resolveErr).Error())
+ 		if resolveErr != nil {
+ 			logger.ErrorWithFields(c.Request.Context(), resolveErr, map[string]any{
+ 				"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
+ 			})
+ 		}
+ 		logger.ErrorWithFields(c.Request.Context(), failure, map[string]any{
+ 			"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
+ 		})
+ 		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "the upstream response could not be read completely and the activity outcome is unknown")
  		return


─── internal/handler/craft_model_gateway.go:588-592 ───
[security · medium] 同类泄漏：Resolve(CraftChargeStartStarted) 的错误原文直接作为 appFail 的 message 返回客户端。该错误可能是
GORM 底层错误（DB 细节）或 resolveCraftChargeStart 的冲突包装文本，均属服务端内部信息；此处同样缺少 logger.ErrorWithFields 服务端记录，与
521-529 行、535-545 行等兄弟分支的处理不一致。建议与上一处一并按"细节进日志、客户端收不透明文案"的既定纪律修复。

  	if err := attempt.Resolve(c.Request.Context(), service.CraftChargeStartStarted); err != nil {
+ 		logger.ErrorWithFields(c.Request.Context(), err, map[string]any{
+ 			"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
+ 		})
  		g.recordCall(c, payload, callID, attemptID, model, nil)
- 		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", err.Error())
+ 		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "the activity outcome could not be recorded")
  		return
  	}


─── apps/web/vite.config.ts:115-115 ───
[bug · medium] 新增的 '@weknora/views/craft/access' 子路径别名未同步到 apps/desktop/vite.config.ts。桌面端配置第 56-58
行注释明确要求 craft 子路径 alias 与 apps/web 保持 lockstep：因对象键 alias 按前缀匹配，'@weknora/views'（index.ts）会把
'@weknora/views/craft/access' 重写为 '<index.ts>/craft/access'，rollup 将报 ENOTDIR 解析失败。当前尚无导入方所以未立即暴露，但
access.tsx（本轮 +81 行）由共享 craft 工作台引用在即，届时桌面端构建将确定性失败。建议在 apps/desktop/vite.config.ts 的 craft
组（interaction 之后）补齐同一条目；若 access 为 web 独有功能，请在 desktop 侧注释说明豁免原因。

+ // apps/desktop/vite.config.ts craft 组内同步补齐：
+       '@weknora/views/craft/interaction': fileURLToPath(new URL('../../packages/views/src/craft/interaction.tsx', import.meta.url)),
        '@weknora/views/craft/access': fileURLToPath(new URL('../../packages/views/src/craft/access.tsx', import.meta.url)),


─── packages/domain/src/craft/web-promotion.ts:35-40 ───
[maintainability · low] 业务字面量 'passed' 在 webCheckEvidenceReady 内重复硬编码 4 次。契约侧已有
CRAFT_WEB_CHECK_OUTCOMES 常量联合类型（'passed' | 'failed' |
'not_run'），此处拼写漂移只会被类型检查部分兜底（字符串写错会编译报错，但语义取值变更需改四处）。建议提取具名常量并锚定 CraftWebCheckOutcome
类型，后续若新增检查项（如安全头检查）也可复用。

+ import type { CraftWebCheckOutcome, CraftVersionView } from '@weknora/contracts';
+ 
+ const PASSED_OUTCOME: CraftWebCheckOutcome = 'passed';
+ 
+ export function webCheckEvidenceReady(evidence: WebCheckEvidenceFact | null): boolean {
+   if (evidence === null) return false;
    return (
-     evidence.build === 'passed' &&
-     evidence.entry === 'passed' &&
-     evidence.preview_reachable === 'passed' &&
-     evidence.page_loaded === 'passed'
+     evidence.build === PASSED_OUTCOME &&
+     evidence.entry === PASSED_OUTCOME &&
+     evidence.preview_reachable === PASSED_OUTCOME &&
+     evidence.page_loaded === PASSED_OUTCOME
    );
+ }


─── packages/domain/src/craft/web-promotion.ts:61-66 ───
[test · low] defaultPreviewVersion 的 createdAt 排序见证分支（Date.parse 比较、单侧 NaN
回退数组顺序、相等时间戳保留先者）是本模块最复杂、且注释明确承诺的 #107 "newest wins" 自强制逻辑，但 web-promotion.test.ts 的全部用例均未传入
createdAt，该分支当前零覆盖——例如把 '>' 误改为 '>=' 或破坏 NaN 回退语义都不会被任何测试捕获。建议补充：乱序 + createdAt 时最新者胜出、单侧缺失
createdAt 时回退数组顺序、相等时间戳保留先者等用例；接线时也需与调用方约定 createdAt 统一使用后端 RFC3339 格式，避免 Date.parse 依实现解析非标准格式。

-     const versionTime = Date.parse(version.createdAt ?? '');
-     const bestTime = Date.parse(best.createdAt ?? '');
-     if (!Number.isNaN(versionTime) && !Number.isNaN(bestTime)) {
-       if (versionTime > bestTime) best = version;
-       continue;
-     }
+ // web-promotion.test.ts 建议补充：
+ test('createdAt witness overrides array order among ready versions', () => {
+   const shuffled = [
+     { id: 'ver_old', webEvidence: all, createdAt: '2026-01-01T00:00:00Z' },
+     { id: 'ver_new', webEvidence: all, createdAt: '2026-02-01T00:00:00Z' },
+   ];
+   assert.equal(defaultPreviewVersion(shuffled)?.id, 'ver_new');
+ });
+ 
+ test('missing createdAt on one side falls back to newest-first array order', () => {
+   const withoutWitness = { id: 'ver_first', webEvidence: all };
+   const withWitness = { id: 'ver_second', webEvidence: all, createdAt: '2026-02-01T00:00:00Z' };
+   assert.equal(defaultPreviewVersion([withoutWitness, withWitness])?.id, 'ver_first');
+ });


─── packages/domain/src/craft/web-promotion.ts:19-25 ───
[maintainability · low] createdAt 排序见证当前无任何数据来源：本模块锚定的 CraftVersionView（乃至整个
packages/contracts）不含时间戳字段，后端版本 ID 也是内容摘要（craft.VersionID = 'ver_'+sha256，非时间有序），因此任何从契约构建 facts
的调用方都无法提供 createdAt——defaultPreviewVersion 的 Date.parse 自强制分支在生产路径不可达，注释承诺的 "the seat goes to the
NEWEST ready version even if the list arrives shuffled ... which is exactly what #107 requires"
在现有数据形态下无法兑现，排序实际仍完全依赖 newest-first 数组顺序（由服务端 created_at DESC, id ASC 保证，但该保证未在契约类型上表达）。与已确认的 #3
互补：该分支不仅零测试覆盖，也无真实数据可触发。建议在注释中明示 createdAt
需等待契约补充版本时间戳字段（或改为契约可携带的单调序号），并写明在此之前数组顺序（服务端排序契约）是唯一生效的排序依据，避免后续接入方按注释假设乱序安全而静默选错默认版本。

     * Optional ordering witness (creation timestamp of the version, newest
-    * wins). When callers supply it, the picker no longer depends on their
-    * array order alone: the seat goes to the NEWEST ready version even if the
-    * list arrives shuffled, which is exactly what #107's "the newest version
-    * passing the four checks becomes the default" requires.
+    * wins). NOTE: CraftVersionView carries no timestamp today and version ids
+    * are content digests, so callers cannot supply this until the contract
+    * exposes one; until then the newest-first server order
+    * (created_at DESC, id ASC) is the only effective ordering contract.
     */
    createdAt?: string;


─── internal/handler/craft_model_gateway.go:507-514 ───
[security · medium] 信息泄漏（与已确认两处同族的新调用点）：BeginBinding 的非冲突失败经 failBudget 默认分支（本文件 731-732 行）以
`appFail(..., "BUDGET_GATE_FAILED", err.Error())` 原样返回错误文本。BeginBinding 的错误来源包括
loadGrant/prepareCraftChargeStart 的裸 GORM/驱动错误（可含 SQL 片段、表名，如 `dial tcp 10.x.x.x:5432` 暴露内网 DB
拓扑），以及 craft_budget.go:220 `errors.Join(err, resolveErr)` 中的 resolveErr（GORM
错误）。这违反本文件自述的脱敏纪律（523-525 行注释、699-701 行注释）。建议：该分支同样按"细节进
logger.ErrorWithFields、客户端收不透明文案"处理，与已确认两处一并在本轮修复。

  	if err != nil {
  		if errors.Is(err, craft.ErrConflict) {
  			appFail(c, http.StatusConflict, "ACTIVITY_UNRESOLVED", "this model activity was already attempted; reconcile before retry")
+ 			return
+ 		}
+ 		if !isCraftBudgetRefusal(err) { // 已知 ErrGrantRevoked/ErrGrantExpired/ErrGrantExhausted/ErrBudgetDenied 之外的错误一律脱敏
+ 			logger.ErrorWithFields(c.Request.Context(), err, map[string]any{
+ 				"craft_run_id": payload.RunID, "craft_activity_id": activityID,
+ 			})
+ 			appFail(c, http.StatusInternalServerError, "BUDGET_GATE_FAILED", "the budget coordinator failed to authorize this activity")
  			return
  		}
  		g.failBudget(c, err, false)
  		return
  	}


─── internal/modules/execution/sandbox/docker_restricted_exec.go:68-72 ───
[bug · low] CreateOutputlessExec 缺少与 CreateAttachedExec 相同的 Timeout 上界校验。normal 路径在
docker_normal_exec.go 中显式拒绝 `timeout > time.Duration(1<<62)`(防溢出/荒谬值),而受限路径对 req.Timeout 只做 `<=0`
归一后就直接传入 dockerExecCommand。由于受限 exec 是 detached 启动:StartOutputlessExec 的 rpcTimeout 只覆盖 start
RPC、服务层 Wait 的 maxWait 只约束轮询退出而进程继续运行,容器内唯一的硬终止手段就是 wrapper 的 `timeout -s KILL <seconds>`;一个超大或损坏的
Timeout 会让该保护退化为近似永不生效,进程将无限消耗容器 CPU/内存,直到空闲回收器兜底。建议补齐同款上界拒绝以保持两个 provider 的输入契约一致。

  	timeout := req.Timeout
  	if timeout <= 0 {
  		timeout = DefaultTimeout
+ 	}
+ 	if timeout > time.Duration(1<<62) {
+ 		return DockerOutputlessExecReceipt{}, dockerInvalidRequest("RestrictedExec", "timeout is out of range")
  	}
  	created, err := c.api.ExecCreate(ctx, id, client.ExecCreateOptions{


─── internal/modules/execution/sandbox/docker_restricted_exec.go:103-108 ───
[maintainability · low] StartOutputlessExec 对 ExecStart 的方法集断言只有运行时检查,没有像 docker_exec_events.go
顶部那样加编译期钉子(其注释明确说明:客户端升级改变签名时应让构建在此处失败,而不是运行时断言静默降级为 unsupported)。当前若 moby client 升级调整了 ExecStart
签名,编译仍能通过,所有受限 exec 会在生产运行时以 "restricted Docker ExecStart unsupported" 全量失败,而这恰是 events
文件同批变更刻意防住的场景。建议补一个包级编译期断言保持两处策略一致。

+ // Compile-time pin mirroring docker_exec_events.go: a client upgrade that
+ // changes the ExecStart signature must break the build here instead of
+ // silently disabling every restricted exec at runtime.
+ var _ interface {
+ 	ExecStart(context.Context, string, client.ExecStartOptions) (client.ExecStartResult, error)
+ } = (*client.Client)(nil)
+ 
+ // (在 StartOutputlessExec 中)
  	starter, ok := api.(interface {
  		ExecStart(context.Context, string, client.ExecStartOptions) (client.ExecStartResult, error)
  	})
  	if !ok {
  		return fmt.Errorf("restricted Docker ExecStart unsupported")
  	}


LLM retry report summary: 11 of 700 requests affected -- 11 requests recovered after retry

Core review (11 requests):
- internal/application/repository/craft_version.go,internal/application/repository/craft_workspace.go,internal/application/service/craft_inputs.go,internal/application/service/craft_source_open.go,internal/application/service/craft_workspace.go,internal/modules/craft/contracts.go,internal/modules/craft/input_code.go,internal/modules/craft/version.go,packages/views/src/craft/sources.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/repository/craft_version.go,internal/application/repository/craft_workspace.go,internal/application/service/craft_inputs.go,internal/application/service/craft_source_open.go,internal/application/service/craft_workspace.go,internal/modules/craft/contracts.go,internal/modules/craft/input_code.go,internal/modules/craft/version.go,packages/views/src/craft/sources.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_access.go,internal/application/service/craft_session.go,internal/application/service/session.go,internal/container/craft_access_wiring.go,internal/modules/workbench/service/workbench/interaction.go,packages/views/src/craft/access.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_archive.go,internal/application/service/craft_artifacts.go,internal/application/service/craft_citations.go,internal/application/service/craft_export.go,internal/container/craft_export_wiring.go,internal/handler/session/artifact_download.go,internal/modules/craft/archive.go,internal/modules/craft/citation.go,internal/modules/craft/export_manifest.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_budget.go,internal/modules/commercial/repository/commercial/budget_reservation.go,internal/modules/commercial/repository/commercial/budget_task.go,packages/views/src/craft/usage.tsx: rate limited (HTTP 429) -> succeeded
- ... and 6 more

Per-attempt detail: --format json (retry_report).
