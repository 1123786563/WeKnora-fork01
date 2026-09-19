package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedAnalyticsData plants a small cross-day analytics fixture via raw SQL,
// mirroring the production write shapes. Sessions carry the canonical owner
// identities that the source bucketing keys off: plain users ("web") and
// tenant-API-key owners ("api"). The window seedRunFixtures' s1/s2 rows are
// soft-deleted first because their CURRENT_TIMESTAMP created_at falls inside
// the queried window and would pollute the channel counts.
func seedAnalyticsData(t *testing.T, db *gorm.DB) {
	t.Helper()
	day1 := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 11, 9, 0, 0, 0, time.UTC)

	// Retire the shared fixtures for this test: their created_at is "now",
	// which sits inside the 2026-09 window under assertion.
	require.NoError(t, db.Exec(
		`UPDATE sessions SET deleted_at = ? WHERE id IN ('s1', 's2')`, day1,
	).Error)

	insertAnalyticsSession := func(id string, tenant uint64, owner string, createdAt time.Time, deletedAt *time.Time) {
		t.Helper()
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type, created_at, updated_at, deleted_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, tenant, "analytics-"+id, owner, "builtin", createdAt, createdAt, deletedAt,
		).Error)
	}
	insertAnalyticsMessage := func(id, session string, role, agentID string, createdAt time.Time, deletedAt *time.Time) {
		t.Helper()
		require.NoError(t, db.Exec(
			`INSERT INTO messages (id, request_id, session_id, role, content, agent_id, created_at, updated_at, deleted_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, "rq-"+id, session, role, "content-"+id, agentID, createdAt, createdAt, deletedAt,
		).Error)
	}

	// Two live sessions in tenant 1 across two days, one per source bucket.
	insertAnalyticsSession("as1", 1, "u1", day1, nil)                     // web
	insertAnalyticsSession("as2", 1, "api_tenant_key:1:key-1", day2, nil) // api
	// A soft-deleted session and a foreign-tenant session must never count.
	insertAnalyticsSession("as3", 1, "u9", day1.Add(2*time.Hour), &day2)
	insertAnalyticsSession("as9", 2, "u1", day1.Add(3*time.Hour), nil)

	// Tenant-1 messages: m1/m2 span two days on as1; m3 is the api question.
	insertAnalyticsMessage("m1", "as1", "user", "", day1.Add(time.Minute), nil)
	insertAnalyticsMessage("m2", "as1", "assistant", "a1", day2.Add(time.Hour), nil)
	insertAnalyticsMessage("m3", "as2", "user", "", day2.Add(2*time.Hour), nil)
	// Excluded rows: message on the soft-deleted session, soft-deleted
	// message, and a foreign-tenant message.
	insertAnalyticsMessage("m4", "as3", "user", "", day1.Add(2*time.Hour+time.Minute), nil)
	insertAnalyticsMessage("m5", "as1", "user", "", day1.Add(5*time.Minute), &day2)
	insertAnalyticsMessage("m9", "as9", "user", "", day1.Add(3*time.Hour+time.Minute), nil)

	// Two feedback rows on the SAME user message (two different raters) make
	// the QueryTrend LEFT JOIN fan out: the queries counter must stay
	// deduplicated while likes/dislikes count feedback rows.
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (tenant_id, user_id, message_id, session_id, rating, comment, created_at, updated_at)
		 VALUES (1, 'u1', 'm1', 'as1', 'like', '', ?, ?)`, day1, day1,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (tenant_id, user_id, message_id, session_id, rating, comment, created_at, updated_at)
		 VALUES (1, 'u2', 'm1', 'as1', 'dislike', '', ?, ?)`, day1, day1,
	).Error)
}

// TestAnalyticsAggregations pins the four read-time aggregations on both
// dialects: exact per-day numbers, join-fanout-safe query counts, soft-delete
// filtering, and tenant scoping.
func TestAnalyticsAggregations(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			repo := NewAnalyticsRepository(db)
			ctx := context.Background()
			seedAnalyticsData(t, db)
			from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

			trend, err := repo.QueryTrend(ctx, 1, from, to)
			require.NoError(t, err)
			require.Len(t, trend, 2, "one point per seeded day")
			require.Equal(t, "2026-09-10", trend[0].Date)
			require.Equal(t, "2026-09-11", trend[1].Date)
			// Day 1: one user question (fan-out from the two feedback rows on
			// m1 must not double it), one like, one dislike.
			require.Equal(t, int64(1), trend[0].Queries)
			require.Equal(t, int64(1), trend[0].Likes)
			require.Equal(t, int64(1), trend[0].Dislikes)
			// Day 2: m3 asks; m2 (assistant, no feedback) adds nothing.
			require.Equal(t, int64(1), trend[1].Queries)
			require.Equal(t, int64(0), trend[1].Likes)
			require.Equal(t, int64(0), trend[1].Dislikes)

			users, err := repo.ActiveUsers(ctx, 1, from, to)
			require.NoError(t, err)
			require.Len(t, users, 2)
			require.Equal(t, "2026-09-10", users[0].Date)
			require.Equal(t, int64(1), users[0].ActiveUsers) // u1 asked on day 1
			require.Equal(t, "2026-09-11", users[1].Date)
			require.Equal(t, int64(1), users[1].ActiveUsers) // api owner asked on day 2

			channels, err := repo.ChannelSessions(ctx, 1, from, to)
			require.NoError(t, err)
			byKey := map[string]int64{}
			for _, p := range channels {
				byKey[p.Date+"|"+p.Source] = p.Sessions
			}
			require.Equal(t, map[string]int64{
				"2026-09-10|web": 1, // as1; as3 soft-deleted, as9 foreign tenant
				"2026-09-11|api": 1, // as2
			}, byKey)

			agentStats, err := repo.AgentMessages(ctx, 1, "a1", from, to)
			require.NoError(t, err)
			require.Len(t, agentStats, 1)
			require.Equal(t, "2026-09-11", agentStats[0].Date)
			require.Equal(t, int64(1), agentStats[0].Messages)
			require.Equal(t, int64(1), agentStats[0].UniqueUsers)
		})
	}
}
