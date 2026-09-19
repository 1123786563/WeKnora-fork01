package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// analyticsRepository computes the console analytics aggregations read-time,
// in SQL, on both supported dialects. Every window is half-open [from, to)
// and every query is tenant-scoped plus soft-delete filtered.
type analyticsRepository struct {
	db *gorm.DB
}

// NewAnalyticsRepository returns the SQL-backed AnalyticsRepository.
func NewAnalyticsRepository(db *gorm.DB) interfaces.AnalyticsRepository {
	return &analyticsRepository{db: db}
}

// dayExpr renders the UTC calendar-day expression for a column. PostgreSQL
// needs the explicit AT TIME ZONE cast; SQLite's date() is already UTC.
func dayExpr(dialect, column string) string {
	if dialect == "sqlite" {
		return "date(" + column + ")"
	}
	return "to_char(" + column + " AT TIME ZONE 'UTC', 'YYYY-MM-DD')"
}

// countExpr renders a dialect-portable per-row conditional counter that also
// works inside grouped LEFT JOIN queries (non-matching rows count as 0):
// PostgreSQL uses the FILTER clause, SQLite the SUM(CASE) emulation.
func countExpr(dialect, cond string) string {
	if dialect == "sqlite" {
		return "COALESCE(SUM(CASE WHEN " + cond + " THEN 1 ELSE 0 END), 0)"
	}
	return "COUNT(*) FILTER (WHERE " + cond + ")"
}

// analyticsSourceExpr buckets a session into the console source vocabulary.
// sessions has no physical source column — the owner identity and the embed
// description marker are the canonical derivation (see sessionRepository
// QueryPaged). Unknown/legacy rows fall into "web", matching the frontend's
// "empty source renders as web" convention.
func analyticsSourceExpr() string {
	return "CASE " +
		"WHEN s.user_id LIKE '" + types.SessionOwnerAPITenantKeyPrefix + "%' " +
		"OR s.user_id LIKE '" + types.SessionOwnerAPIExternalUserPrefix + "%' THEN 'api' " +
		"WHEN s.description LIKE '" + types.EmbedSessionMarkerPrefix + "%' THEN 'embed' " +
		"ELSE 'web' END"
}

// QueryTrend aggregates per UTC day: user questions asked, and like/dislike
// feedback rows on that day's messages. The LEFT JOIN message_feedback fans
// one message out per rater, so the queries counter must count DISTINCT
// message ids (inlined here on purpose — the other aggregations have no join
// fan-out), while likes/dislikes count feedback rows, which is exactly the
// per-rating volume wanted.
func (r *analyticsRepository) QueryTrend(ctx context.Context, tenantID uint64, from, to time.Time) ([]interfaces.QueryTrendPoint, error) {
	dialect := r.db.Dialector.Name()
	day := dayExpr(dialect, "m.created_at")
	likeExpr := countExpr(dialect, "f.rating = 'like'")
	dislikeExpr := countExpr(dialect, "f.rating = 'dislike'")
	var queryExpr string
	if dialect == "sqlite" {
		queryExpr = "COUNT(DISTINCT CASE WHEN m.role = 'user' THEN m.id END)"
	} else {
		queryExpr = "COUNT(DISTINCT m.id) FILTER (WHERE m.role = 'user')"
	}
	sql := `
SELECT ` + day + ` AS date,
       ` + queryExpr + ` AS queries,
       ` + likeExpr + ` AS likes,
       ` + dislikeExpr + ` AS dislikes
FROM messages m
JOIN sessions s ON s.id = m.session_id AND s.tenant_id = @tenant
LEFT JOIN message_feedback f ON f.message_id = m.id
WHERE m.created_at >= @from AND m.created_at < @to
  AND m.deleted_at IS NULL AND s.deleted_at IS NULL
GROUP BY 1 ORDER BY 1`
	var out []interfaces.QueryTrendPoint
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "from": from, "to": to,
	}).Scan(&out).Error
	return out, err
}

// ActiveUsers counts, per UTC day, the distinct session owners that asked at
// least one question (owner of a user-role message).
func (r *analyticsRepository) ActiveUsers(ctx context.Context, tenantID uint64, from, to time.Time) ([]interfaces.ActiveUsersPoint, error) {
	day := dayExpr(r.db.Dialector.Name(), "m.created_at")
	sql := `
SELECT ` + day + ` AS date,
       COUNT(DISTINCT s.user_id) AS active_users
FROM messages m
JOIN sessions s ON s.id = m.session_id AND s.tenant_id = @tenant
WHERE m.role = 'user' AND m.created_at >= @from AND m.created_at < @to
  AND m.deleted_at IS NULL AND s.deleted_at IS NULL
GROUP BY 1 ORDER BY 1`
	var out []interfaces.ActiveUsersPoint
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "from": from, "to": to,
	}).Scan(&out).Error
	return out, err
}

// ChannelSessions counts newly created sessions per UTC day and source
// bucket (web / api / embed derived from the session owner identity).
func (r *analyticsRepository) ChannelSessions(ctx context.Context, tenantID uint64, from, to time.Time) ([]interfaces.ChannelSessionsPoint, error) {
	day := dayExpr(r.db.Dialector.Name(), "s.created_at")
	sql := `
SELECT ` + day + ` AS date,
       ` + analyticsSourceExpr() + ` AS source,
       COUNT(DISTINCT s.id) AS sessions
FROM sessions s
WHERE s.tenant_id = @tenant AND s.created_at >= @from AND s.created_at < @to
  AND s.deleted_at IS NULL
GROUP BY 1, 2 ORDER BY 1, 2`
	var out []interfaces.ChannelSessionsPoint
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "from": from, "to": to,
	}).Scan(&out).Error
	return out, err
}

// AgentMessages counts one custom agent's assistant messages and the
// distinct session owners that triggered them, per UTC day.
func (r *analyticsRepository) AgentMessages(ctx context.Context, tenantID uint64, agentID string, from, to time.Time) ([]interfaces.AgentUsagePoint, error) {
	day := dayExpr(r.db.Dialector.Name(), "m.created_at")
	sql := `
SELECT ` + day + ` AS date,
       COUNT(DISTINCT m.id) AS messages,
       COUNT(DISTINCT s.user_id) AS unique_users
FROM messages m
JOIN sessions s ON s.id = m.session_id AND s.tenant_id = @tenant
WHERE m.agent_id = @agent AND m.role = 'assistant' AND m.created_at >= @from AND m.created_at < @to
  AND m.deleted_at IS NULL AND s.deleted_at IS NULL
GROUP BY 1 ORDER BY 1`
	var out []interfaces.AgentUsagePoint
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{
		"tenant": tenantID, "agent": agentID, "from": from, "to": to,
	}).Scan(&out).Error
	return out, err
}
