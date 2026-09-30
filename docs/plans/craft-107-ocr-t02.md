Review complete: 4 finding(s) across 5 selected item(s).

─── internal/application/service/craft_archive.go:180-188 ───
[bug · high] 复用分支未校验存量行的 recognition 列是否为 NULL。migrations 000110/000189 通过 ALTER TABLE ADD COLUMN 增加
recognition 列,升级窗口内写入的行这三列均为 NULL(即代码注释多处提到的 "legacy content" 行);craftWorkspaceInputRow.input()
对此类行返回 Recognition == nil,随后发布事务中 `&manifest[i].Recognition.Accepted` 会直接空指针解引用,导致请求
panic(500)。事务回退分支自己都把 NULL recognition 视为冲突("already names different or legacy content"),复用门应加同样的守卫。

  		if err == nil {
- 			if existing.TenantID != session.TenantID || existing.Bytes != manifest[i].Bytes {
+ 			if existing.TenantID != session.TenantID || existing.Bytes != manifest[i].Bytes ||
+ 				existing.RecognitionAccepted == nil || existing.RecognitionUnderstood == nil ||
+ 				existing.RecognitionReason == nil {
  				rollback()
  				return nil, fmt.Errorf("%w: member %s collides with different legacy content",
  					craft.ErrConflict, manifest[i].Name)
  			}
  			manifest[i] = existing.input()
  			continue
  		}


─── internal/application/service/craft_archive.go:159-160 ───
[bug · low] context.WithoutCancel 同时剥离了取消与截止时间:回滚里的 DB Count 与 s3/oss/cos 等远端 DeleteFile
调用现在没有任何时限,后端卡滞时回滚会无限期阻塞,使请求超出 30 秒提取预算的设计意图。建议给分离上下文附加一个有限的清理预算(需补 time import)。

  	// would fail immediately and leak every stored object.
- 	cleanupCtx := context.WithoutCancel(ctx)
+ 	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
+ 	defer cleanupCancel()


─── internal/application/service/craft_inputs.go:187-188 ───
[bug · low] 同 craft_archive.go:context.WithoutCancel 去掉取消信号是必要的,但也一并去掉了截止时间,回滚中的 Count 与远端
DeleteFile 失去任何时限,后端卡滞时上传请求会无限期挂起。建议改为携带有限超时的分离上下文。

  	created := make([]string, 0, len(uploads))
- 	cleanupCtx := context.WithoutCancel(ctx)
+ 	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
+ 	defer cleanupCancel()


─── internal/application/service/craft_archive.go:176-179 ───
[bug · low] 同一归档内两个不同路径但同基名、同内容的成员（如 a/LICENSE 与 b/LICENSE）绕过了身份复用：预查发生在事务提交之前，两个成员都查不到 (workspace,
name, sha256) 行，于是各自 SaveBytes；生产对象存储每次铸造新 ref，而表主键是 (workspace_id, ref)，插入互不冲突，最终同一身份落成两行 +
两份重复对象，201 响应也会把同一输入列出两次。这与上一条注释声明的“同 (workspace, name, digest) 复用、只上传真正新的成员”不变式矛盾（该折叠只在确定性 ref
的测试后端里经 PK 冲突才发生）；并发调用两次 ExpandArchive 也会因同样的 check-then-act 竞态产生重复行。建议在构建 manifest 时先按 (name,
digest) 折叠同身份成员，再进入发布循环。

+ 	// 折叠同身份成员：同名同 digest 的多个归档成员是同一个输入
+ 	identity := make(map[string]struct{}, len(members))
+ 	unique := members[:0]
+ 	for _, member := range members {
+ 		key := path.Base(member.Path) + "\x00" + hex.EncodeToString(sha256.Sum256(member.Content)[:])
+ 		if _, dup := identity[key]; dup {
+ 			continue
+ 		}
+ 		identity[key] = struct{}{}
+ 		unique = append(unique, member)
+ 	}
+ 	members = unique
  	for i, member := range members {
  		var existing craftWorkspaceInputRow
  		err := s.db.WithContext(ctx).Where("workspace_id = ? AND name = ? AND sha256 = ?",
  			workspace.ID, manifest[i].Name, manifest[i].SHA256).Take(&existing).Error

