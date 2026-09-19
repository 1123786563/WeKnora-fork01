package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// usageRepository stores daily token/cost buckets and aggregates them in
// SQL, in dialect-portable form, on both supported dialects. All windows are
// half-open [from, to) over UTC days.
type usageRepository struct {
	db *gorm.DB
}

// NewUsageRepository returns the SQL-backed UsageRepository.
func NewUsageRepository(db *gorm.DB) interfaces.UsageRepository {
	return &usageRepository{db: db}
}

// usageDayExpr renders the bucket day as a 'YYYY-MM-DD' string. window_start
// is a DATE column, so PostgreSQL needs no AT TIME ZONE cast; SQLite stores
// the bound timestamp as text, which date() normalizes (UTC calendar day).
func usageDayExpr(dialect string) string {
	if dialect == "sqlite" {
		return "date(window_start)"
	}
	return "to_char(window_start, 'YYYY-MM-DD')"
}

// usageWindowPredicate matches buckets in [from, to) independent of the
// session timezone: PostgreSQL compares in date space after pinning the
// timestamptz parameters to UTC; SQLite's date() parses both the stored
// value and the bound parameter.
func usageWindowPredicate(dialect string) string {
	if dialect == "sqlite" {
		return "date(window_start) >= date(@from) AND date(window_start) < date(@to)"
	}
	return "window_start >= (@from AT TIME ZONE 'UTC')::date AND window_start < (@to AT TIME ZONE 'UTC')::date"
}

// usageSumCols projects the five additive bucket counters.
const usageSumCols = `SUM(input_tokens) AS input_tokens,
       SUM(output_tokens) AS output_tokens,
       SUM(cache_read_tokens) AS cache_read_tokens,
       SUM(cache_write_tokens) AS cache_write_tokens,
       SUM(cost_microcredits) AS cost_microcredits`

// AddUsage upserts one bucket row. A conflict on the dimension tuple adds
// the deltas onto the stored bucket (a legal accumulation, not a no-op) —
// the write shape the chat/craft ingestion points need. The increment
// expressions are table-qualified so PostgreSQL's DO UPDATE SET cannot be
// ambiguous against the EXCLUDED row; SQLite accepts the same qualification.
func (r *usageRepository) AddUsage(ctx context.Context, u *types.UserUsage) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"}, {Name: "user_id"}, {Name: "window_start"}, {Name: "model"}, {Name: "flow"},
		},
		DoUpdates: clause.Set{
			{Column: clause.Column{Name: "input_tokens"}, Value: gorm.Expr("user_usage.input_tokens + ?", u.InputTokens)},
			{Column: clause.Column{Name: "output_tokens"}, Value: gorm.Expr("user_usage.output_tokens + ?", u.OutputTokens)},
			{Column: clause.Column{Name: "cache_read_tokens"}, Value: gorm.Expr("user_usage.cache_read_tokens + ?", u.CacheReadTokens)},
			{Column: clause.Column{Name: "cache_write_tokens"}, Value: gorm.Expr("user_usage.cache_write_tokens + ?", u.CacheWriteTokens)},
			{Column: clause.Column{Name: "cost_microcredits"}, Value: gorm.Expr("user_usage.cost_microcredits + ?", u.CostMicrocredits)},
			{Column: clause.Column{Name: "updated_at"}, Value: time.Now().UTC()},
		},
	}).Create(u).Error
}

// AggregateByUser returns one row per UTC day x model for a single user's
// buckets in [from, to), ordered by day then model.
func (r *usageRepository) AggregateByUser(ctx context.Context, tenantID uint64, userID string, from, to time.Time) ([]interfaces.UsageAggregate, error) {
	dialect := r.db.Dialector.Name()
	sql := `
SELECT ` + usageDayExpr(dialect) + ` AS window_start,
       model,
       ` + usageSumCols + `
FROM user_usage
WHERE tenant_id = @tenant AND user_id = @user
  AND ` + usageWindowPredicate(dialect) + `
GROUP BY 1, 2 ORDER BY 1, 2`
	var out []interfaces.UsageAggregate
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "user": userID, "from": from, "to": to,
	}).Scan(&out).Error
	return out, err
}

// AggregateAllUsers returns one row per UTC day x model x user in [from,
// to), ordered by day then user then model so limit/offset paging is stable.
func (r *usageRepository) AggregateAllUsers(ctx context.Context, tenantID uint64, from, to time.Time, limit, offset int) ([]interfaces.UsageAggregate, error) {
	dialect := r.db.Dialector.Name()
	sql := `
SELECT ` + usageDayExpr(dialect) + ` AS window_start,
       model,
       user_id,
       ` + usageSumCols + `
FROM user_usage
WHERE tenant_id = @tenant
  AND ` + usageWindowPredicate(dialect) + `
GROUP BY 1, 2, 3
ORDER BY 1, 3, 2
LIMIT @limit OFFSET @offset`
	var out []interfaces.UsageAggregate
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "from": from, "to": to, "limit": limit, "offset": offset,
	}).Scan(&out).Error
	return out, err
}

// ExportRows returns one row per user with the window totals (WindowStart
// and Model are empty on these rows), ordered by user.
func (r *usageRepository) ExportRows(ctx context.Context, tenantID uint64, from, to time.Time) ([]interfaces.UsageAggregate, error) {
	dialect := r.db.Dialector.Name()
	sql := `
SELECT user_id,
       ` + usageSumCols + `
FROM user_usage
WHERE tenant_id = @tenant
  AND ` + usageWindowPredicate(dialect) + `
GROUP BY 1 ORDER BY 1`
	var out []interfaces.UsageAggregate
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "from": from, "to": to,
	}).Scan(&out).Error
	return out, err
}
