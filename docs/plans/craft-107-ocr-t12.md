Review complete: 9 finding(s) across 6 selected item(s).

─── internal/handler/session/artifact_download.go:792-797 ───
[bug · high] 下载中途失败时（GetFile 出错、CreateHeader 失败、io.Copy 中断）仅记 Warn 后 break 并正常关闭 zipWriter，且三个固定文档的
Marshal 失败也被静默跳过。此时 HTTP 200 与响应头已发出，zip 中央目录仍完整写入，客户端会收到一个结构合法但缺成员/缺文档的 zip，与 export-manifest.json
声明的成员清单和 digest 不一致，且传输层完全无法察觉——破坏了本功能"完整且与清单绑定"的核心契约。仓库已有先例（craftegress adapter）：中途失败应
panic(http.ErrAbortHandler) 截断连接，让客户端看到异常传输而非降级的"成功"包。建议：任何成员/文档写入失败时记录日志后
panic(http.ErrAbortHandler)，而不是 break 后干净收尾。

  	for _, member := range bundle.Version.Files {
  		reader, err := h.files.GetFile(c.Request.Context(), member.Ref)
  		if err != nil {
- 			logger.Warnf(c.Request.Context(), "craft export bundle member %s read failed: %v", member.Path, err)
- 			break
+ 			logger.Errorf(c.Request.Context(), "craft export bundle member %s read failed: %v", member.Path, err)
+ 			// 200 与响应头已发出：截断连接，让客户端看到失败的传输，
+ 			// 而非一个结构合法但缺成员的 zip。
+ 			panic(http.ErrAbortHandler)
  		}


─── internal/handler/session/artifact_download.go:804-809 ───
[bug · medium] 成员字节仅 io.Copy，未按 member.SHA256 做流式摘要校验。export-manifest.json 把每个成员的内容摘要作为 bundle
身份的一部分，但底层对象存储内容损坏或 ref 指向被替换时，zip 内实际字节与清单声明的摘要不符且服务端无任何告警，溯源保证只在"清单"侧成立、不在"字节"侧成立。建议边拷贝边计算
sha256（io.MultiWriter），不匹配时记日志并 panic(http.ErrAbortHandler) 截断下载。

- 		_, copyErr := io.Copy(entry, reader)
+ 		hasher := sha256.New()
+ 		_, copyErr := io.Copy(io.MultiWriter(entry, hasher), reader)
  		reader.Close()
  		if copyErr != nil {
- 			logger.Warnf(c.Request.Context(), "craft export bundle member %s stream failed: %v", member.Path, copyErr)
- 			break
+ 			logger.Errorf(c.Request.Context(), "craft export bundle member %s stream failed: %v", member.Path, copyErr)
+ 			panic(http.ErrAbortHandler)
+ 		}
+ 		if hex.EncodeToString(hasher.Sum(nil)) != member.SHA256 {
+ 			logger.Errorf(c.Request.Context(), "craft export bundle member %s digest mismatch", member.Path)
+ 			panic(http.ErrAbortHandler)
  		}


─── internal/container/craft_export_wiring.go:47-49 ───
[bug · medium] ownerScopeOf 用 clause.Locking{Strength:"UPDATE"}（SELECT ... FOR UPDATE）读取
craft_versions，但整条调用链（ExportBundle → member.Get/VersionEvidence）没有任何显式事务；autocommit
模式下语句结束即释放行锁，注释声称的 "under a read lock so the read is consistent" 不成立——随后的 craft_workspaces
读取完全不在锁保护内。仓库内其余 30+ 处 clause.Locking 全部包裹在 tx 事务中，仅此处例外。此外版本行是不可变的（store 注释明确 rows are never
updated），对纯读路径加写强度锁只会在并发发布/导出间引入无谓锁竞争。建议直接去掉 Locking 子句（普通读即可满足需求），或确实需要一致性时改为在单个事务内完成两次读取。

- 	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
+ 	err := r.db.WithContext(ctx).
  		Table("craft_versions").Select("tenant_id, workspace_id").
  		Where("id = ?", versionID).Take(&version).Error


─── internal/container/craft_export_wiring.go:50-55 ───
[bug · low] ownerScopeOf 将任何数据库错误（连接失败、超时等瞬时故障）一律映射为 craft.ErrNotFound（HTTP
404）。跨任务隐藏存在性的目的可以达成，但可用性故障被伪装成"版本不存在"，客户端会误判为永久性缺失而不重试，监控/排障也无法区分"不存在"与"存储故障"。仓库先例（CraftAccessServi
ce.session，craft_access.go:74）只把 gorm.ErrRecordNotFound 译为 ErrNotFound，其余错误原样上抛（HTTP 层
craftShareHTTPError 译为 503，同样不泄露存在性）。建议对齐该先例。

- 	if err != nil {
+ 	if errors.Is(err, gorm.ErrRecordNotFound) {
  		return craft.Scope{}, craft.ErrNotFound
+ 	}
+ 	if err != nil {
+ 		// 瞬时 DB 故障保留错误类别（HTTP 层译为 503），不伪装成 404，
+ 		// 也不泄露跨任务存在性。
+ 		return craft.Scope{}, err
  	}
  	if version.TenantID != tenantID {
  		return craft.Scope{}, craft.ErrNotFound
  	}


─── internal/modules/craft/export_manifest.go:257-261 ───
[bug · medium] 三个平铺保留名（export-manifest.json、sources.json、build.json）实际未被任何校验拒绝：ValidateArtifactPath
只拒绝遍历/绝对路径/凭据名，全仓库也没有其他位置（收集门禁或发布链路）检查这些保留名，export_manifest.go 顶部注释"they never collide with a member
without the collection gate's path validation noticing first"与事实不符。生成产物完全可能含有根级
build.json/sources.json（常见前端工程文件名），此时 zip 中出现同名重复条目，解包行为依工具而异（通常后写的成员字节覆盖固定文档），清单与包内容的对应关系失去确定性。建议在
ValidateExportBundleMembers（发布侧校验亦可）显式拒绝与保留名相同的成员路径。

  func ValidateExportBundleMembers(files []File) error {
  	for _, member := range files {
  		if err := ValidateArtifactPath(member.Path); err != nil {
  			return err
+ 		}
+ 		switch member.Path {
+ 		case BundleManifestPath, BundleSourcesPath, BundleBuildPath:
+ 			return fmt.Errorf("%w: bundle member %q collides with a reserved bundle document", ErrInvalidInput, member.Path)
  		}


─── internal/application/service/craft_export.go:276-278 ───
[maintainability · low] 构造器注入的时钟 s.now 从未被使用：auditExport
的去重窗口起点（time.Now().Add(-craftDenyDedupWindow)）和审计行 CreatedAt（time.Now()）都直接调用
time.Now()，注入时钟成为死代码，且审计去重窗口的行为不可测试。建议统一改用 s.now()（CraftExportConfig.Now 注入初衷即在此），或删除该注入项。

  	versionID = strings.TrimSpace(versionID)
  	if outcome == "denied" {
- 		since := time.Now().Add(-craftDenyDedupWindow)
+ 		since := s.now().Add(-craftDenyDedupWindow)


─── internal/modules/craft/export_manifest.go:170-170 ───
[bug · medium] 空 evidence（Empty=true、零来源）是 ValidateVersionEvidence 一等支持的合法钉定状态，此时 origins
为空切片，`append([]ExportOriginRef(nil), origins...)` 返回 nil——每个成员的 Origins 都变成 nil。而 T00 冻结的消费方契约
ExportManifest.Validate（web_contracts.go:244）明确拒绝 `f.Origins == nil`（"invalid export file"），T13 的
ExportConsentView.Validate 也会透传失败；同时 zip 内 export-manifest.json 会序列化出 `"origins": null`。模块测试只对
citation manifest 覆盖了空 evidence，未覆盖此路径。建议改为对空切片也保持非 nil（如 `append([]ExportOriginRef{},
origins...)`），并补一条空 evidence 的 BuildExportManifest + manifest.Validate() 回归测试。

- 			Origins:    append([]ExportOriginRef(nil), origins...),
+ 			Origins:    append([]ExportOriginRef{}, origins...),


─── internal/application/service/craft_export.go:72-76 ───
[maintainability · low] CraftExportService.files 是从未被读取的死字段：ExportBundle 全程只使用
versions/evidence/titles/taskAccess/db，成员字节实际由 HTTP 层单独注入的 h.files 流式输出。但构造器把 cfg.Files
作为强制依赖校验（"fails closed without it"），且 CraftExportBundle 的文档注释声称"member BYTES stream separately
through CraftExportFileReader"——契约误导且无法通过构造器约束真正生效。建议从 CraftExportConfig/CraftExportService 中移除
Files（由 handler/wiring 直接持有），或让服务真正经它暴露成员读取，二选一。



─── internal/application/service/craft_export.go:112-117 ───
[bug · low] craft.Check（contracts.go:82-84）没有 json tag，json.Marshal(BundleBuildDocument{}) 会把 checks
输出为 `{"Name":...,"Status":...,"Detail":...}` 大写键；而 describe 端点 craftExportBody 对同一 checks 输出小写
name/status/detail。build.json 被声明为 bundle 固定契约文档，同一份数据在同一功能的两个出口键形不一致，机器消费方解析会踩坑。建议在本结构处投影为带
snake_case tag 的局部类型（与 craftExportBody 对齐），不要直接嵌入 craft.Check。

+ type BundleBuildCheck struct {
+ 	Name   string `json:"name"`
+ 	Status string `json:"status"`
+ 	Detail string `json:"detail"`
+ }
+ 
  type BundleBuildDocument struct {
- 	VersionID string        `json:"version_id"`
- 	RunID     string        `json:"run_id"`
- 	Kind      string        `json:"kind"`
- 	Checks    []craft.Check `json:"checks"`
+ 	VersionID string             `json:"version_id"`
+ 	RunID     string             `json:"run_id"`
+ 	Kind      string             `json:"kind"`
+ 	Checks    []BundleBuildCheck `json:"checks"`
  }


LLM retry report summary: 1 of 35 requests affected -- 1 request recovered after retry

Core review (1 request):
- internal/application/service/craft_export.go,internal/container/container.go,internal/container/craft_export_wiring.go,internal/handler/session/artifact_download.go,internal/modules/craft/export_manifest.go: rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
