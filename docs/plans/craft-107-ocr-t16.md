Review complete: 4 finding(s) across 7 selected item(s).

─── internal/application/repository/craft_workspace.go:822-826 ───
[security · medium] GetWriterLease 命中路径（租约行存在时）完全没有会话与 Owner 校验：未命中路径同时校验了 ws.SessionID（NotFound）与
ws.OwnerID（Forbidden），注释还声称"Keep the ACL shape of GetWorkspace"，但命中路径直接返回租约投影。同租户另一会话的调用方只要拿到
workspaceID 即可读到该工作区租约的 TaskID（会话 id）、RunID 与 revision，与 GetWorkspace 的 ACL 形状（跨会话不可见、非 Owner
禁止）相悖。虽然当前 service 层入口（WriterLease）会先经 GetWorkspace 过滤，但该仓储方法本身是新增的导出缝隙，应与未命中路径保持同一 ACL 形状。

  	if err != nil {
  		return nil, err
  	}
- 	return leaseRow.view(), nil
+ 	// 命中路径保持与未命中路径一致的 ACL 形状：跨会话不可见，非执行 Owner 禁止。
+ 	if leaseRow.SessionID != scope.SessionID {
+ 		return nil, fmt.Errorf("%w: workspace %s", craft.ErrNotFound, workspaceID)
+ 	}
+ 	var ws craftWorkspaceRow
+ 	if we := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&ws).Error; we != nil {
+ 		return nil, we
+ 	}
+ 	if ws.OwnerID != scope.UserID {
+ 		return nil, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, ws.OwnerID)
- }
+ 	}
+ 	return leaseRow.view(), nil


─── internal/application/repository/craft_workspace.go:769-772 ───
[security · medium] ReleaseWriterLease 命中路径缺少 workspace Owner 校验：未命中路径经 lockCraftWriterWorkspace
校验会话绑定与 OwnerID，GetWorkspace 对同会话非 Owner 也返回 ErrForbidden，但命中路径只校验 leaseRow.SessionID 与 RunID——同会话的非
Owner（协作者）只要知道 runID 即可通过此缝隙释放他人持有的写者围栏。建议在事务开头（取租约行锁之前）先调用 lockCraftWriterWorkspace：既补齐 Owner 校验，也使
Release 与 AcquireWriterLease 按 workspace→lease 的同一顺序加锁串行化，同时消除 takeOver CAS 零行分支中租约行被并发删除导致的
readLease ErrRecordNotFound 冒泡（见 AcquireWriterLease 内 takeOver）。注意锁序必须与 Acquire 一致（先 workspace 后
lease），否则构成 AB-BA 死锁。

- 		if leaseRow.SessionID != scope.SessionID || leaseRow.RunID != runID {
- 			return fmt.Errorf("%w: the lease of workspace %s is held by run %s, not run %s",
- 				craft.ErrForbidden, workspaceID, leaseRow.RunID, runID)
+ 	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
+ 		// 先授权并锁定 workspace 行（与 AcquireWriterLease 相同的锁序：
+ 		// workspace → lease），使释放与获取串行化，Owner 校验与未命中路径对齐。
+ 		if _, werr := lockCraftWriterWorkspace(tx, scope, workspaceID); werr != nil {
+ 			return werr
+ 		}
+ 		var leaseRow craftWriterLeaseRow
+ 		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
+ 			Where("workspace_id = ? AND tenant_id = ?", workspaceID, scope.TenantID).Take(&leaseRow).Error
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
+ 			return fmt.Errorf("%w: workspace %s holds no writer lease", craft.ErrNotFound, workspaceID)
  		}


─── internal/application/repository/craft_workspace.go:684-690 ───
[bug · low] takeOver 的 CAS 零行分支未处理租约行已被并发删除的情形：ReleaseWriterLease 命中路径只锁租约行、不锁 workspace
行，因此可以在本事务读取租约行之后、CAS 之前提交删除（持有 Run 终态且零未决写者时释放与接管的判定条件相同，二者确实可并发触发）。此时 readLease 返回裸
gorm.ErrRecordNotFound 并整体冒泡，AcquireWriterLease 以错误结束，StartRun 只能报告 unknown 并记录"fence
retained"——而实际上围栏已经不存在，正确结果应是本 Run 获取成功（行已空，直接走插入路径即可），客户端重试虽可自愈，但一次响应对围栏状态做出了错误陈述。建议将
ErrRecordNotFound 视作"回到未命中分支"：重新走 OnConflict DoNothing 插入，或在 Release 侧先锁 workspace 行使两者串行化。

  			if updated.RowsAffected != 1 {
  				after, aerr := readLease()
+ 				if errors.Is(aerr, gorm.ErrRecordNotFound) {
+ 					// 并发 Release 已删行：围栏为空，回到未命中分支重新插入。
+ 					row := craftWriterLeaseRow{
+ 						WorkspaceID: workspaceID, TenantID: scope.TenantID,
+ 						SessionID: scope.SessionID, RunID: runID, Revision: revision,
+ 					}
+ 					created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
+ 					if created.Error != nil {
+ 						return created.Error
+ 					}
+ 					if created.RowsAffected == 1 {
+ 						out = craftWriterAcquired(row)
+ 						return nil
+ 					}
+ 					after, aerr = readLease()
+ 				}
  				if aerr != nil {
  					return aerr
  				}
  				return takeOver(after)
  			}


─── internal/modules/craft/lifecycle.go:247-248 ───
[documentation · low] Revision 字段注释以现在时声称"every write and promotion of this Run is checked against
it"，但本增量（T16）中该围栏仅被记录与透传：全库检索确认除 StartRun 的获取/投影与测试外，没有任何草稿头写入或版本晋升路径消费 lease.Revision 做校验（唯一的序列化仍是
run slot 准入）。注释夸大了当前已存在的保证，容易让后续维护者误以为写路径已被围栏保护。建议改为标注这是后续 Ticket 的强制契约、当前增量仅持久化记录。

- 	// Revision is the draft-head revision fenced at acquisition: every write
- 	// and promotion of this Run is checked against it.
+ 	// Revision is the draft-head revision recorded at acquisition. T16
+ 	// persists it; the write/promotion paths enforce it as their CAS fence
+ 	// in a later ticket — nothing consumes it yet in this increment.


LLM retry report summary: 3 of 51 requests affected -- 3 requests recovered after retry

Review planning (1 request):
- internal/application/repository/craft_workspace.go,internal/application/service/craft_session.go,internal/application/service/craft_workspace.go,internal/container/container.go,internal/handler/session/craft.go,internal/modules/craft/lifecycle.go: rate limited (HTTP 429) -> succeeded

Core review (1 request):
- internal/application/repository/craft_workspace.go,internal/application/service/craft_session.go,internal/application/service/craft_workspace.go,internal/container/container.go,internal/handler/session/craft.go,internal/modules/craft/lifecycle.go: rate limited (HTTP 429) -> succeeded

File grouping (1 request):
- __grouping__: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
