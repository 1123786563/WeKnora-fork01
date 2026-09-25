Review complete: 8 finding(s) across 6 selected item(s).

─── internal/application/service/craft_share.go:203-210 ───
[bug · medium] view() 对非受限贡献（Restricted=false）投影矛盾线载荷：ShareStateOf 对非受限版本无条件返回 consented，但此处只要存在绑定的
rejected 决策，就会同时输出 status=consented + decision=rejected，且因 state==consented 还会带上 expires_at（一个
rejected 决策的过期时间）。下游 packages/views/src/craft/share.tsx 的 projectShareView 遇到 "consented 但 decision
非 approved" 会把状态降级为 private，导致非受限版本在 UI 上被显示为"私有"，与"非受限无需同意即可共享"的语义相矛盾。建议非受限贡献不投影绑定决策与 expires_at。

- 	if decision != nil && craft.DecisionBinds(decision.Decision, contribution) {
+ 	if decision != nil && contribution.Restricted && craft.DecisionBinds(decision.Decision, contribution) {
  		bound := decision.Decision
  		view.Decision = &bound
  		if state == craft.ShareStateConsented {
  			expires := decision.DecidedAt.Add(craft.ShareDecisionTTL)
  			view.ExpiresAt = &expires
  		}
  	}


─── internal/application/service/craft_share.go:249-251 ───
[security · medium] evidence digest 不匹配（ErrConflict）与下方 caller
身份不匹配（ErrForbidden）都是已通过权威检查后的拒绝，但两者均未调用 auditShare 直接返回，与包注释 "every decision, revocation and proven
refusal is audited" 的承诺不符。digest 冲突尤其值得审计——它是重放其他证据决策的信号。

  	if seenDigest != contribution.EvidenceDigest {
+ 		s.auditShare(ctx, scope, versionID, "craft.share_denied", "denied", map[string]string{
+ 			"attempted_decision": string(decision), "reason": "evidence_digest_mismatch",
+ 		})
  		return CraftShareView{}, fmt.Errorf("%w: the decision binds other evidence than the version's current evidence", craft.ErrConflict)
  	}


─── internal/application/service/craft_share.go:266-269 ───
[performance · low] DecideShare 在 upsert 成功后调用 view() 会完整重跑一遍 contribution()（versions.Get +
files.GetFile + records.Load），而本函数前段刚推导过同一贡献；RevokeShare 同样如此。ShareAuthority 作为 T13/T20 下游门控每次调用也伴随
3 次 I/O。同意操作频率低，影响有限，但至少 DecideShare 可直接复用已推导的 contribution 与新写入的行组装返回视图，省去一次完整的版本/文件/记录读取。

  	s.auditShare(ctx, scope, contribution.VersionID, "craft.share_decision_recorded", "success", map[string]string{
  		"decision": string(decision), "evidence_digest": contribution.EvidenceDigest,
  	})
- 	return s.view(ctx, scope, versionID)
+ 	// 复用已推导的 contribution，避免 view() 重复读取版本/清单/记录。
+ 	state := craft.ShareStateOf(contribution, &craft.RecordedShareDecision{Decision: craft.ShareDecision{
+ 		VersionID: contribution.VersionID, EvidenceDigest: contribution.EvidenceDigest,
+ 		OwnerID: scope.UserID, Decision: decision,
+ 	}, DecidedAt: now}, s.now())
+ 	view := CraftShareView{Contribution: contribution, State: state}
+ 	if state == craft.ShareStateConsented {
+ 		expires := now.Add(craft.ShareDecisionTTL)
+ 		view.ExpiresAt = &expires
+ 	}
+ 	return view, nil


─── internal/handler/session/craft_share.go:117-121 ───
[security · medium] DecideCraftShare 使用 c.ShouldBindJSON 解码请求体，绕过了本包 craft POST 路由的统一约定
decodeCraftBody（craft.go L209：1MiB MaxBytesReader + 拒绝未知字段）。项目没有全局 body 大小限制中间件，同包的 craft_access.go
成员管理路由都显式设置 1MiB 上限；此处遗漏意味着已认证用户可向该端点提交任意大小的 JSON body 造成内存放大。建议复用 decodeCraftBody。

  	var body craftShareDecisionRequest
- 	if err := c.ShouldBindJSON(&body); err != nil {
- 		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
+ 	if !decodeCraftBody(c, &body) {
  		return
  	}


─── packages/views/src/craft/share.tsx:161-164 ───
[bug · medium] 按钮回调以 void onDecide(...) / void onRevoke() 触发，而回调类型为 void | Promise<void>：返回的 Promise
被显式丢弃，宿主实现 reject 时会产生 unhandled promise
rejection，且面板本身无任何失败反馈，所有者可能误以为决策/撤回已生效。至少应捕获拒绝并给出可见错误（或与宿主约定回调不得 reject 并在注释中固化）。

      {isOwner && awaitingDecision && <div className="wk-craft-share-actions">
-       <button type="button" className="wk-craft-share-confirm" onClick={() => void onDecide('approved')}>{labels.confirm}</button>
-       <button type="button" className="wk-craft-share-decline" onClick={() => void onDecide('rejected')}>{labels.decline}</button>
+       <button type="button" className="wk-craft-share-confirm" onClick={() => { void runShareAction(() => onDecide('approved')); }}>{labels.confirm}</button>
+       <button type="button" className="wk-craft-share-decline" onClick={() => { void runShareAction(() => onDecide('rejected')); }}>{labels.decline}</button>
      </div>}
+ 
+ // 组件外定义：
+ // async function runShareAction(action: () => void | Promise<void>) {
+ //   try { await action(); } catch { /* 向宿主/用户呈现失败，避免 unhandled rejection */ }
+ // }


─── packages/views/src/craft/share.tsx:70-71 ───
[bug · low] 降级守卫未区分受限标记：非受限版本（restricted=false）本无同意要求，服务端状态即为 consented；若该版本历史上存在一条绑定的 rejected
决策（服务端会原样投影 decision），此守卫会把非受限版本错误降级显示为 private。信任条件应加上 restricted 为 true（服务端侧也应避免对非受限贡献投影决策，见
craft_share.go view() 的对应意见）。

    const effectiveStatus: CraftShareStatus =
-     status === 'consented' && decision?.decision !== 'approved' ? 'private' : (status as CraftShareStatus);
+     status === 'consented' && raw.restricted === true && decision?.decision !== 'approved' ? 'private' : (status as CraftShareStatus);


─── packages/views/src/craft/share.tsx:161-161 ───
[bug · low] owner 的决策按钮仅在 status === 'private'（awaitingDecision）时渲染：一旦 owner 选择拒绝（status 变为
declined），面板不再提供任何再次决策入口，而 revoke 按钮又只在 consented 显示——owner 拒绝后想改为同意只能绕过 UI 直接调 API。服务端 DecideShare
的 upsert 语义明确支持重新决策，建议 declined 状态也为 owner 提供决策操作。

-     {isOwner && awaitingDecision && <div className="wk-craft-share-actions">
+     {isOwner && view.restricted && (view.status === 'private' || view.status === 'declined') && <div className="wk-craft-share-actions">


─── internal/application/service/craft_share.go:338-343 ───
[security · medium] auditShare 的拒绝路径（craft.share_denied）每个请求无条件写一行 audit_logs，既没有 craft_access.go 中
auditTaskDenial 明确实现的 craftDenyDedupWindow 去重（注释原话："a probing client must not be able to flood
audit_logs through repeated denials of the same task"），也没有其 knownTask 守卫——session/task 根本不存在时
RequireTaskAccess 失败同样落审计行。任何已认证用户对 decision/revocation 端点反复 POST（甚至遍历伪造 session id）即可无上限地插入审计行，既造成
DB 资源放大，也会淹没真实审计事件。建议复用同样的 (tenant, actor, scope_id, target_id, action, outcome) 时间窗去重（或至少仅在任务可known
时记录）。

+ 	if outcome == "denied" {
+ 		// Mirror the craftDenyDedupWindow probe of auditTaskDenial: a
+ 		// probing client must not flood audit_logs through repeated
+ 		// share denials of the same task+version.
+ 		since := time.Now().Add(-craftDenyDedupWindow)
+ 		var recent int64
+ 		if err := s.db.WithContext(ctx).Model(&craftAccessAudit{}).
+ 			Where("tenant_id = ? AND actor_user_id = ? AND action = ? AND scope_id = ? AND target_id = ? AND outcome = ? AND created_at > ?",
+ 				scope.TenantID, actor, action, scope.SessionID, strings.TrimSpace(versionID), outcome, since).
+ 			Count(&recent).Error; err == nil && recent > 0 {
+ 			return
+ 		}
+ 	}
  	if err := s.db.WithContext(ctx).Create(&craftAccessAudit{
  		TenantID: scope.TenantID, ActorUserID: actor, Action: action,
  		ScopeType: "session", ScopeID: scope.SessionID, TargetType: "artifact_version",
  		TargetID: strings.TrimSpace(versionID), Outcome: outcome,
  		Details: types.JSON(raw), CreatedAt: time.Now(),
  	}).Error; err != nil {


LLM retry report summary: 2 of 74 requests affected -- 2 requests recovered after retry

Core review (2 requests):
- internal/application/service/craft_share.go,internal/container/container.go,internal/container/craft_share_wiring.go,internal/handler/session/craft_share.go,internal/modules/craft/share.go,packages/views/src/craft/share.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/application/service/craft_share.go,internal/container/container.go,internal/container/craft_share_wiring.go,internal/handler/session/craft_share.go,internal/modules/craft/share.go,packages/views/src/craft/share.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
