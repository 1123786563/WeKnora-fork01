Review complete: 5 finding(s) across 4 selected item(s).

─── internal/application/service/craft_archive.go:151-156 ───
[bug · medium] 回滚闭包复用了请求级 ctx，而本路径的失败常正源于该 ctx 被取消：ctx 带 MaxArchiveExtractDuration（30s）硬超时，最多 20
个成员、累计 100MiB 的 SaveBytes 加事务提交完全可能逼近甚至超过 30s；超时触发后事务失败进入 rollback，此时 s.db.WithContext(ctx) 的 Count
直接返回 context.Canceled（err != nil 即 continue），DeleteFile 在透传 ctx 的对象存储后端上同样必然失败（错误被 `_ =` 吞掉）。结果是已
SaveBytes 的对象全部成为无关联行引用的孤儿，单次失败最多泄漏约 100MiB 存储且随失败累积。项目 go.mod 为 go
1.26，建议回滚改用脱离取消的上下文（context.WithoutCancel），必要时可叠加独立的清理超时。此模式复制自 craft_inputs.go 的 T01 回滚，可一并修复。

+ 		// 回滚必须脱离请求取消：失败常源于 30s 超时或客户端断连，
+ 		// 此时 ctx 已取消，Count/DeleteFile 都会失败而泄漏已存储对象。
+ 		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), craft.MaxArchiveExtractDuration)
+ 		defer cleanupCancel()
+ 		rollback := func() {
+ 			seen := make(map[string]struct{}, len(created))
+ 			for _, createdRef := range created {
+ 				if _, ok := seen[createdRef]; ok {
+ 					continue
+ 				}
+ 				seen[createdRef] = struct{}{}
- 			var associations int64
+ 				var associations int64
- 			if err := s.db.WithContext(ctx).Model(&craftWorkspaceInputRow{}).
+ 				if err := s.db.WithContext(cleanupCtx).Model(&craftWorkspaceInputRow{}).
- 				Where("ref = ?", createdRef).Count(&associations).Error; err != nil || associations != 0 {
+ 					Where("ref = ?", createdRef).Count(&associations).Error; err != nil || associations != 0 {
- 				continue
+ 					continue
- 			}
+ 				}
- 			_ = s.files.DeleteFile(ctx, createdRef)
+ 				_ = s.files.DeleteFile(cleanupCtx, createdRef)
+ 			}
+ 		}


─── internal/modules/craft/archive.go:128-131 ───
[bug · low] 对用户提供的输入字节的确定性拒绝（"bytes are not a supported archive"，以及 nestedArchiveMember 中"member is
itself an archive"两处 ErrUnsupported）经 handler 的 craftHTTPError 映射为 503 Service
Unavailable：这告诉客户端"服务端依赖未就绪、稍后可重试"，但重试同样的字节永远不可能成功，也污染 5xx 监控指标。这两类错误本质是对请求内容的 4xx 语义（400 或
415），建议改用 ErrInvalidInput（现有映射即 400），或在 craftHTTPError 中为该场景增加 415 映射。

  	format, ok := DetectArchiveFormat(archive)
  	if !ok {
- 		return nil, fmt.Errorf("%w: bytes are not a supported archive", ErrUnsupported)
+ 		return nil, fmt.Errorf("%w: bytes are not a supported archive", ErrInvalidInput)
  	}


─── internal/application/service/craft_archive.go:180-186 ───
[bug · high] 幂等重放分支在生产存储后端不可达，重复扩展会无限累积重复 input。所有生产 FileService 的 SaveBytes 每次调用都生成全新对象键（local.go:
UnixNano 时间戳；minio.go/s3.go: uuid.New()；resource_catalog.go 每次注册新 resource ref），返回的 ref
并非内容寻址——storageName 里的 SHA256 只是文件名提示，后端并不按它寻址。因此对同一归档每次重放 ExpandArchive：每个成员都 SaveBytes 出新 ref → 主键
(workspace_id, ref) 永不冲突 → 这里的 OnConflict DoNothing 分支和随后的逐字段比对在生产中是死代码；每次重放/并发双击都会插入最多 20 条新行、新存最多
100MiB 对象，且返回的成员 ref 与首次不同，违反 TestCraftT02Journey 断言的"重放不重复"契约（该测试仅因 fake craftT01Files 返回确定性 ref
而通过）。建议把幂等键改为内容维度：发布前按 (workspace_id, sha256[, name]) 查找已存在的成员行并复用其 ref，或派生以摘要为键的稳定 ref。



─── internal/modules/craft/archive.go:94-97 ───
[other · medium] cleaned == "." 把归档根目录条目（tar 中常见的 "./"）与真正的逃逸路径一并拒绝。GNU tar 用 `tar -czf x.tgz .`（或
`tar -C dir .`）打包时首个条目就是 "./"，这是很常见的合法布局；该条目被误报为 "escapes the archive root" 后整个归档永久不可解压，错误语义也有误导（"."
并不逃逸任何根）。建议把根目录条目 "./"（仅目录语义）视为无操作跳过，其余 ".." / "../" 判断保持不变。



─── internal/application/service/craft_archive.go:130-134 ───
[other · low] 第二道门会把含任意空成员的归档整体判死。readMember 允许读出 0 字节的常规成员，而 ValidateInputManifest 拒绝 Bytes <=
0（"no declared size"），因此归档中只要有一个空文件（如 __init__.py、.gitkeep，真实压缩包非常常见），整个 all-or-nothing
扩展永远失败，且报错指向成员名而非"归档含空文件"这一根因。若坚持复用 T01 规则，建议在 readMember/finish 阶段就显式拒绝（或跳过）空成员并给出针对性错误；否则应允许 0
字节成员通过。


