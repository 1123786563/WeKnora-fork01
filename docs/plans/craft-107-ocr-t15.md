Review complete: 4 finding(s) across 7 selected item(s).

─── packages/domain/src/craft/web-promotion.ts:11-12 ───
[maintainability · low] 该模块未注册进 `@weknora/domain` 的 exports 映射（package.json 中同级
`./craft/state`、`./craft/version-selection`、`./craft/capabilities` 等均有对应条目，apps/web 的 tsconfig paths
与 desktop 的 vite 别名同样未包含）。当前外部代码无法通过 `@weknora/domain/craft/web-promotion` 导入，只有同目录测试经相对路径可达。虽然 T15
报告把工作台接线列为可选后续，但按本仓库的模块消费惯例，落地时建议随接线一并补齐 exports（及 tsconfig/vite 别名），避免后续消费方按惯例导入时无法解析。



─── internal/application/repository/craft_preview_check.go:122-131 ───
[bug · high] UpdateWebProbeCheck 的读-改-写缺乏并发防护：事务内以无锁的 Take 读取 checks_json、在内存中合并后仅按 id 无条件 Update
写回。两个探测事实（preview_reachable 与 page_load）按设计来自独立的观察时机/回调，并发到达同一版本时双方都基于旧快照各自 append，后提交者整体覆盖
checks_json，导致：(1) 先记录的探测事实被静默丢弃，违背函数自己声明的 "the two probe facts are recorded independently by
construction"；(2) 不可变规则被绕过——回调 A 已提交 reachable=passed（或 failed）后，并发回调 B 基于过期快照看不到该记录，以不同 outcome
覆盖写回，"recorded facts are immutable" 契约（及父 Spec 的"历史版本证据固定"）被破坏，且存在 failed 被覆盖为 passed 的放宽门禁窗口；(3)
替换栅栏（page load 需 reachable 已 passed）同样基于可能过期的快照判断。同文件的 UpdatePreviewCheck 虽也是无锁模式，但它是单一 preview 事实的
replace 语义（最后写赢可接受）；本函数引入了更强的事实独立性/不可变性契约，需要相应的原子性支撑。建议在 Take
上加行锁（tx.Clauses(clause.Locking{Strength: "UPDATE"})，需导入 gorm.io/gorm/clause），或改用对 checks_json 的条件
CAS 更新并在 RowsAffected==0 时返回冲突。

  		checks = append(checks, check)
  		encoded, err := json.Marshal(checks)
  		if err != nil {
  			return err
  		}
- 		if e := tx.Model(&craftVersionRow{}).
- 			Where("id = ?", versionID).
- 			Update("checks_json", string(encoded)).Error; e != nil {
- 			return e
+ 		res := tx.Model(&craftVersionRow{}).
+ 			Where("id = ? AND checks_json = ?", versionID, row.ChecksJSON).
+ 			Update("checks_json", string(encoded))
+ 		if res.Error != nil {
+ 			return res.Error
+ 		}
+ 		if res.RowsAffected == 0 {
+ 			return fmt.Errorf("%w: version %s checks changed concurrently", craft.ErrConflict, versionID)
  		}


─── internal/application/service/craft_session.go:493-501 ───
[bug · medium] web 会话在选择器出错时的回退方向与 T15 门禁相悖且错误被完全吞掉：serr != nil 时静默降级到 legacy newest-version
规则，会把可能从未通过四检查（甚至从未被浏览器加载过）的最新版本推上默认预览席位，正是 T15 要防止的情形；同时 serr
无任何日志，选择器故障在生产中不可观测（对比：本文件其他次要读取失败路径也偏静默，但这里是策略性降级，值得留痕）。注释虽声明"the seat is a projection, never an
authorization boundary"，但 current_version 仍是前端预览入口指向的版本投影，建议至少记录 serr
日志；更稳妥的降级是保守方向——选择器失败时保持席位为空（或维持上一次已知默认），而不是放宽到未验证的最新版本。

- 		if selected, ok, serr := s.defaultVersionSelector(ctx, owner); serr == nil && ok {
+ 		if selected, ok, serr := s.defaultVersionSelector(ctx, owner); serr == nil {
+ 			if ok {
- 			view.CurrentVersion = &selected
+ 				view.CurrentVersion = &selected
- 		} else if serr != nil {
- 			versions, verr := s.versions.List(ctx, owner)
- 			if verr == nil && len(versions) > 0 {
- 				current := versions[0]
- 				view.CurrentVersion = &current
  			}
+ 		} else {
+ 			// Selector failure degrades conservatively: the seat stays empty
+ 			// rather than promoting an unverified newest version (T15 fail-closed).
+ 			logger.Warnf(ctx, "[CraftSession] default version selector failed for session %s: %v", session.ID, serr)
  		}


─── internal/application/service/craft_artifacts.go:244-246 ───
[bug · high] 修订栏栅只比较了 revision 数字，没有把 head 的身份绑定到被提升的 candidate，两个方向都能绕过它声称保证的不变量（"A late callback
for a superseded revision is a conflict, never a silent promotion of stale files"）：

1. 旧 candidate + 新 revision：head 已被后续 run Advance 到 revision N 后，只要回调把 req.Revision 报成 N，就能提升任意一个更早
run 的 candidate（candidate 绑定检查只要求 candidate 属于该 workspace/run 与请求一致）。该旧 candidate 会以当前时间戳 Publish
成为新版本，并被 SelectDefaultVersion 按 newest-first 选为默认预览席位——这正是注释声称不可能发生的 stale
promotion。测试（craft_artifact_promotion_t15_test.go Phase 3）只覆盖了"报旧 revision"方向，未覆盖"旧 candidate + 当前
head revision"方向。

2. 空 head + revision 0：candidate 在采集时即 staged，而 draft head 由 post-terminal capture 封存（Advance）。在
capture 之前 head 处于 empty/revision 0 状态时，req.Revision=0 即可通过栏栅，提升尚未封存的内容。

DraftHead 本身携带 State/SourceRunID/ManifestDigest（Read 已校验 selected head 与不可变 revision
行一致），足以做绑定校验，但此处全部未用。另外该栏栅是先读后写：Read 之后还要执行外部 probe（网络观察）再 Publish，期间 head 可被并发
Advance（TOCTOU），如需彻底闭合应在 Publish 事务内复查 head。

建议改为绑定校验，例如：
if head.Revision != req.Revision || head.State != craft.DraftHeadSelected ||
    head.SourceRunID != candidate.RunID || head.ManifestDigest != candidate.ManifestDigest {
    return craft.Version{}, fmt.Errorf("%w: workspace head (revision %d, run %s) does not bind
candidate %s", craft.ErrConflict, head.Revision, head.SourceRunID, candidate.ID)
}
并补充"旧 candidate 携带当前 head revision 必须被拒"的测试。

- 	if head.Revision != req.Revision {
- 		return craft.Version{}, fmt.Errorf("%w: stale workspace revision %d (head is %d) for run %s", craft.ErrConflict, req.Revision, head.Revision, candidate.RunID)
+ 	if head.Revision != req.Revision || head.State != craft.DraftHeadSelected ||
+ 		head.SourceRunID != candidate.RunID || head.ManifestDigest != candidate.ManifestDigest {
+ 		return craft.Version{}, fmt.Errorf("%w: workspace head (revision %d, run %s) does not bind candidate %s", craft.ErrConflict, head.Revision, head.SourceRunID, candidate.ID)
  	}

