package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// TestUserUsageAddAccumulates pins the AddUsage write shape: the same
// dimension tuple accumulates in place (1+2=3 tokens, a legal accumulate
// rather than a no-op), every distinct dimension value opens its own bucket
// row, and tenants stay fenced at the storage level.
func TestUserUsageAddAccumulates(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			repo := NewUsageRepository(db)
			ctx := context.Background()
			day1 := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
			day2 := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
			day3 := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

			// Same dims twice: second write adds onto the first bucket.
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{
				TenantID: 1, UserID: "u1", WindowStart: day1, Model: "gpt-x", Flow: types.UsageFlowChat,
				InputTokens: 1, OutputTokens: 10, CostMicrocredits: 100,
			}))
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{
				TenantID: 1, UserID: "u1", WindowStart: day1, Model: "gpt-x", Flow: types.UsageFlowChat,
				InputTokens: 2, OutputTokens: 20, CacheReadTokens: 5, CacheWriteTokens: 7, CostMicrocredits: 200,
			}))
			// Each further dimension value gets its own bucket row.
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{TenantID: 1, UserID: "u1", WindowStart: day1, Model: "gpt-x", Flow: types.UsageFlowCraft, InputTokens: 4}))
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{TenantID: 1, UserID: "u1", WindowStart: day2, Model: "gpt-x", Flow: types.UsageFlowChat, InputTokens: 8}))
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{TenantID: 1, UserID: "u2", WindowStart: day1, Model: "gpt-x", Flow: types.UsageFlowChat, InputTokens: 16}))
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{TenantID: 1, UserID: "u1", WindowStart: day3, Model: "gpt-x", Flow: types.UsageFlowChat, InputTokens: 32}))
			// Foreign-tenant bucket must never leak into tenant-1 reads.
			require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{TenantID: 2, UserID: "u1", WindowStart: day1, Model: "gpt-x", Flow: types.UsageFlowChat, InputTokens: 64}))

			var chatRows []types.UserUsage
			require.NoError(t, db.Where("tenant_id = ? AND user_id = ? AND model = ? AND flow = ?", 1, "u1", "gpt-x", types.UsageFlowChat).
				Order("window_start").Find(&chatRows).Error)
			require.Len(t, chatRows, 3, "one bucket per day for the chat flow")
			require.Equal(t, "2026-09-15", chatRows[0].WindowStart.UTC().Format("2006-01-02"))
			require.Equal(t, int64(3), chatRows[0].InputTokens, "same-dims writes accumulate: 1+2=3")
			require.Equal(t, int64(30), chatRows[0].OutputTokens)
			require.Equal(t, int64(5), chatRows[0].CacheReadTokens)
			require.Equal(t, int64(7), chatRows[0].CacheWriteTokens)
			require.Equal(t, int64(300), chatRows[0].CostMicrocredits)
			require.Equal(t, int64(8), chatRows[1].InputTokens)
			require.Equal(t, int64(32), chatRows[2].InputTokens)

			var tenant1, tenant2 int64
			require.NoError(t, db.Table("user_usage").Where("tenant_id = ?", 1).Count(&tenant1).Error)
			require.NoError(t, db.Table("user_usage").Where("tenant_id = ?", 2).Count(&tenant2).Error)
			require.Equal(t, int64(5), tenant1, "u1/day1/chat (accumulated), u1/day1/craft, u1/day2/chat, u2/day1/chat, u1/day3/chat")
			require.Equal(t, int64(1), tenant2)
		})
	}
}

// TestUserUsageAggregations pins the three read shapes on both dialects:
// AggregateByUser groups by UTC day x model, AggregateAllUsers adds the user
// dimension and pages deterministically, ExportRows collapses to per-user
// totals; all of them fence tenants and the half-open [from, to) window.
func TestUserUsageAggregations(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t)
			repo := NewUsageRepository(db)
			ctx := context.Background()
			day1 := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
			day2 := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
			dayOut := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

			add := func(tenant uint64, user, model, flow string, day time.Time, in, out, cost int64) {
				t.Helper()
				require.NoError(t, repo.AddUsage(ctx, &types.UserUsage{
					TenantID: tenant, UserID: user, WindowStart: day, Model: model, Flow: flow,
					InputTokens: in, OutputTokens: out, CostMicrocredits: cost,
				}))
			}
			add(1, "u1", "gpt-x", types.UsageFlowChat, day1, 1, 10, 100)
			add(1, "u1", "gpt-x", types.UsageFlowChat, day1, 2, 20, 200) // accumulates -> 3/30/300
			add(1, "u1", "gpt-x", types.UsageFlowCraft, day1, 4, 0, 0)   // folds into the day x model row
			add(1, "u1", "gpt-x", types.UsageFlowChat, day2, 8, 0, 0)
			add(1, "u1", "glm-y", types.UsageFlowChat, day2, 50, 5, 500) // second model on day2
			add(1, "u2", "gpt-x", types.UsageFlowChat, day1, 16, 0, 0)
			add(1, "u1", "gpt-x", types.UsageFlowChat, dayOut, 32, 0, 0) // outside [from, to)
			add(2, "u1", "gpt-x", types.UsageFlowChat, day1, 64, 0, 0)   // foreign tenant

			from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

			byUser, err := repo.AggregateByUser(ctx, 1, "u1", from, to)
			require.NoError(t, err)
			require.Len(t, byUser, 3, "u1 has one row per in-window day x model")
			// Day 1: chat 3/30/300 plus craft 4 input -> 7/30/300.
			require.Equal(t, "2026-09-15", byUser[0].WindowStart)
			require.Equal(t, "gpt-x", byUser[0].Model)
			require.Equal(t, int64(7), byUser[0].InputTokens)
			require.Equal(t, int64(30), byUser[0].OutputTokens)
			require.Equal(t, int64(300), byUser[0].CostMicrocredits)
			// Day 2 rows are ordered by model: "glm-y" sorts before "gpt-x".
			require.Equal(t, "2026-09-16", byUser[1].WindowStart)
			require.Equal(t, "glm-y", byUser[1].Model)
			require.Equal(t, int64(50), byUser[1].InputTokens)
			require.Equal(t, int64(500), byUser[1].CostMicrocredits)
			require.Equal(t, "gpt-x", byUser[2].Model)
			require.Equal(t, int64(8), byUser[2].InputTokens)

			page1, err := repo.AggregateAllUsers(ctx, 1, from, to, 2, 0)
			require.NoError(t, err)
			require.Len(t, page1, 2)
			require.Equal(t, "2026-09-15", page1[0].WindowStart)
			require.Equal(t, "u1", page1[0].UserID)
			require.Equal(t, int64(7), page1[0].InputTokens)
			require.Equal(t, "2026-09-15", page1[1].WindowStart)
			require.Equal(t, "u2", page1[1].UserID)
			require.Equal(t, int64(16), page1[1].InputTokens)

			page2, err := repo.AggregateAllUsers(ctx, 1, from, to, 2, 2)
			require.NoError(t, err)
			require.Len(t, page2, 2, "the remaining day2 rows: u1/glm-y then u1/gpt-x")
			require.Equal(t, "2026-09-16", page2[0].WindowStart)
			require.Equal(t, "u1", page2[0].UserID)
			require.Equal(t, "glm-y", page2[0].Model)
			require.Equal(t, int64(50), page2[0].InputTokens)
			require.Equal(t, "gpt-x", page2[1].Model)
			require.Equal(t, int64(8), page2[1].InputTokens)

			page3, err := repo.AggregateAllUsers(ctx, 1, from, to, 2, 4)
			require.NoError(t, err)
			require.Empty(t, page3, "offset past the last row yields nothing")

			exported, err := repo.ExportRows(ctx, 1, from, to)
			require.NoError(t, err)
			require.Len(t, exported, 2, "per-user totals; the foreign tenant and out-of-window rows are excluded")
			require.Equal(t, "u1", exported[0].UserID)
			require.Equal(t, int64(65), exported[0].InputTokens)      // 7 + 8 + 50
			require.Equal(t, int64(35), exported[0].OutputTokens)    // 30 + 0 + 5
			require.Equal(t, int64(800), exported[0].CostMicrocredits) // 300 + 0 + 500
			require.Empty(t, exported[0].WindowStart, "export rows carry no day dimension")
			require.Empty(t, exported[0].Model, "export rows carry no model dimension")
			require.Equal(t, "u2", exported[1].UserID)
			require.Equal(t, int64(16), exported[1].InputTokens)
		})
	}
}
