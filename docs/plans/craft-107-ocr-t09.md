Review complete: 2 finding(s) across 5 selected item(s).

─── internal/application/service/craft_collaborator_run.go:67-68 ───
[bug · medium] 幂等重放会重复写入 craft.run_started 时间线行：客户端携带同一 RequestID 重试时（幂等键的设计场景），StartRun 中 priorErr
== nil 命中已有 admission，Submit 重放同一 Run 并成功返回，随后此处无条件再次调用 auditCollaboratorRunStart，导致同一 TargetID（Run
ID）的 craft.run_started 审计行按重试次数重复追加——任务时间线会显示同一 Run 被同一成员"发起"多次；且重放时角色可能已变（如被撤销后由 owner 以同 key
重放），第二行的 role 细节与首次准入不一致。同库 craft_access.go 的拒绝审计对重复写入有明确的 dedup probe 纪律（172-185 行），此处应保持一致：要么由
StartRun 返回 admission 是否为重放并在重放时跳过审计，要么在写入前按 (tenant_id, action, scope_id, target_id) 探测已存在行。

- 	s.auditCollaboratorRunStart(ctx, scope, s.initiatingMemberRole(ctx, scope), run, req)
- 	return run, acquisition, nil
+ // 方案一（推荐，与 craft_access.go 去重纪律一致）：在 auditCollaboratorRunStart 写入前探测已有行
+ 	var recent int64
+ 	if err := s.db.WithContext(ctx).Model(&craftAccessAudit{}).
+ 		Where("tenant_id = ? AND action = ? AND scope_id = ? AND target_id = ?",
+ 			scope.TenantID, "craft.run_started", scope.SessionID, run.Key.RunID).
+ 		Count(&recent).Error; err == nil && recent > 0 {
+ 		return // 幂等重放：同一 Run 的 run_started 时间线行只写一次
+ 	}
+ // 方案二：由 StartRun 返回 admission 是否为重放（如新增 replayed bool 返回值），
+ // StartCollaboratorRun 仅在首次准入时调用 auditCollaboratorRunStart。


─── internal/application/service/craft_collaborator_run.go:55-59 ───
[performance · low] 同一次 StartCollaboratorRun 成功路径会对完全相同的 TaskWrite 权限做 3 次独立推导：本处前置
RequireTaskAccess、StartRun→writeSession 内部再次执行的同一 RequireTaskAccess（同 scope/同 action/同
checker，顺序紧邻，拒绝时的 denial 审计行为也完全一致），以及随后 initiatingMemberRole 的第三次 Role() 推导。每次推导含 session join +
membership pluck + grant 查询 2-3 次 DB 往返，即每个准入请求重复约 5-6 次冗余查询。前置检查相对 writeSession
内的检查无任何行为差异（含审计），属纯冗余；建议至少消除第三次推导（复用一次 Role 结果同时供 TaskWrite 判定与审计 detail 使用），或直接移除前置检查由 writeSession
统一裁决（注意语义微差：移除后 Viewer 提交非法 body 会先命中 Validate 返回 400 而非 403 且不产生 denial 审计）。

+ 	// Derive the caller's role once and reuse it for both the TaskWrite
+ 	// decision and the timeline detail below (writeSession re-checks the
+ 	// same authority as defense-in-depth).
+ 	role := s.initiatingMemberRole(ctx, scope)
  	if s.access != nil {
  		if err := craft.RequireTaskAccess(ctx, s.access, scope, craft.TaskWrite); err != nil {
  			return agentruntime.Run{}, craft.WriterAcquisition{}, err
+ 		}
- 		}
+ 	}
+ 	run, acquisition, err := s.StartRun(ctx, scope, req)
+ 	if err != nil {
+ 		return run, acquisition, err
  	}
+ 	s.auditCollaboratorRunStart(ctx, scope, role, run, req)

