Review complete: 1 finding(s) across 1 selected item(s).

─── internal/application/repository/craft_run_capture_promotion.go:150-157 ───
[performance · medium] 全局 fresh 扫描页把"已 attempted"的排除完全放在内存（filterUnattemptedPromotionRows），SQL 层不会跳过
completed=true 的终态回执。attempts/captures 均无清理路径（仅删除 Run 时级联），随 completed 历史增长，每个 ≤32
行的扫描页越来越多地被已完成回执占满：调度方 RecoverTick 每 15s 仅推进一页，游标绕一圈需 O(N/32) 个 tick，而新 sealed 回执 updated_at 最新、排在
keyset 末尾，其兜底发现延迟随历史量线性退化（如 N=5 万时约 6.5 小时），fresh 有效吞吐被稀释到接近零。建议在 freshPromotionPageQuery 的 SQL 中下推
`NOT EXISTS (... AND a.completed = TRUE)`（关联子查询不影响主表 INDEXED BY
对部分索引的使用），扫描页将只含未完成候选；内存过滤保留用于排除"未到期预约"的行，游标推进与回绕语义不变。

- 			freshScanLimit := limit - len(rows)
- 			if freshScanLimit > 0 {
- 				freshPageQuery := freshPromotionPageQuery(base(), cursor, freshScanLimit)
- 				if err := freshPageQuery.Scan(&scannedFreshRows).Error; err != nil {
- 					return err
- 				}
- 				var filterErr error
- 				freshRows, filterErr = filterUnattemptedPromotionRows(tx, scannedFreshRows)
+ func freshPromotionPageQuery(base *gorm.DB, cursor craftRunCapturePromotionCursorRow, limit int) *gorm.DB {
+ 	q := base.Where(`NOT EXISTS (
+ 		SELECT 1 FROM craft_run_capture_promotion_attempts AS a
+ 		WHERE a.tenant_id=c.tenant_id AND a.workspace_id=c.workspace_id AND a.run_id=c.run_id
+ 		AND a.completed = ?
+ 	)`, true)
+ 	if base.Dialector.Name() == "sqlite" {

