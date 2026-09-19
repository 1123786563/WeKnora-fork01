package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// seedExportSessions fixtures live inline in
// TestExportSessionRowsAggregationAndFilters; no shared helper is needed.

func TestQueryHistoryExportJobRepoCRUD(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openQueryHistoryTestDB(t, dialect)
			repo := NewQueryHistoryExportJobRepository(db)
			ctx := context.Background()

			job := &types.QueryHistoryExportJob{TenantID: 1, RequestedBy: "u1"}
			require.NoError(t, repo.Create(ctx, job), "Create must default the status to pending")
			require.NotZero(t, job.ID)
			require.Equal(t, types.QueryHistoryExportPending, job.Status)

			got, err := repo.GetByID(ctx, 1, job.ID)
			require.NoError(t, err)
			require.Equal(t, types.QueryHistoryExportPending, got.Status)
			require.Equal(t, "u1", got.RequestedBy)

			// Cross-tenant probes and misses answer the sentinel: the caller
			// maps both to 404 without learning whether the id exists.
			_, err = repo.GetByID(ctx, 2, job.ID)
			require.ErrorIs(t, err, ErrQueryHistoryExportJobNotFound)
			_, err = repo.GetByID(ctx, 1, 999999)
			require.ErrorIs(t, err, ErrQueryHistoryExportJobNotFound)

			// Transition to failed with a message.
			require.NoError(t, repo.UpdateStatus(ctx, job.ID, types.QueryHistoryExportFailed, "", "boom"))
			got, err = repo.GetByID(ctx, 1, job.ID)
			require.NoError(t, err)
			require.Equal(t, types.QueryHistoryExportFailed, got.Status)
			require.Equal(t, "boom", got.ErrorMessage)

			// A later done transition rewrites both columns: no stale error
			// message survives a successful retry.
			require.NoError(t, repo.UpdateStatus(ctx, job.ID, types.QueryHistoryExportDone, "local://exports/1.csv", ""))
			got, err = repo.GetByID(ctx, 1, job.ID)
			require.NoError(t, err)
			require.Equal(t, types.QueryHistoryExportDone, got.Status)
			require.Equal(t, "local://exports/1.csv", got.FilePath)
			require.Empty(t, got.ErrorMessage)
		})
	}
}

// TestExportSessionRowsAggregationAndFilters pins the export row contract:
// origin classification, message/like/dislike tallies, the audit-view
// exclusions, and the admin listing filters.
func TestExportSessionRowsAggregationAndFilters(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openQueryHistoryTestDB(t, dialect)
			repo := NewQueryHistoryExportJobRepository(db).(*queryHistoryExportJobRepository)
			ctx := context.Background()

			// The shared test DB seeds sessions s1/s2 for the agent-run
			// fixtures; the export view must only see this test's rows.
			require.NoError(t, db.Exec(`DELETE FROM sessions WHERE id IN ('s1', 's2')`).Error)

			base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			insertSession := func(id string, tenantID uint64, userID, description string, day int) {
				require.NoError(t, db.Exec(
					`INSERT INTO sessions (id, tenant_id, title, user_id, description, engine_type)
					 VALUES (?, ?, ?, ?, ?, 'builtin')`,
					id, tenantID, "title-"+id, userID, description,
				).Error)
				// Bind timestamps through GORM so the driver serializes them
				// exactly the way it serializes the query bounds.
				created := base.AddDate(0, 0, day)
				require.NoError(t, db.Model(&types.Session{}).Where("id = ?", id).Updates(
					map[string]interface{}{"created_at": created, "updated_at": created},
				).Error)
			}
			insertSession("exp-web", 1, "u1", "", 1)
			insertSession("exp-api", 1, "api_tenant_key:k1", "", 2)
			insertSession("exp-embed", 1, "", "embed_channel:ch1", 3)
			insertSession("exp-im", 1, "", "", 4)
			insertSession("exp-deleted", 1, "u1", "", 5)
			require.NoError(t, db.Exec(
				`UPDATE sessions SET deleted_at = ? WHERE id = 'exp-deleted'`, base,
			).Error)
			insertSession("exp-skill", 1, "u1", "skill_maintenance:job1", 6)
			insertSession("exp-other", 2, "u2", "", 1)

			require.NoError(t, db.Exec(
				`INSERT INTO im_channel_sessions (id, platform, user_id, chat_id, session_id, tenant_id)
				 VALUES ('ics-1', 'feishu', 'fu1', 'chat1', 'exp-im', 1)`,
			).Error)

			insertMessage := func(id, sessionID string) {
				require.NoError(t, db.Exec(
					`INSERT INTO messages (id, request_id, session_id, role, content)
					 VALUES (?, ?, ?, 'user', 'q')`, id, "req-"+id, sessionID,
				).Error)
			}
			insertMessage("m1", "exp-web")
			insertMessage("m2", "exp-web")
			insertMessage("m3", "exp-api")

			insertFeedback := func(id int64, sessionID, messageID, rating string, tenantID uint64) {
				userID := "u1"
				if tenantID != 1 {
					userID = "u-other-tenant"
				}
				require.NoError(t, db.Exec(
					`INSERT INTO message_feedback (id, tenant_id, user_id, message_id, session_id, rating)
					 VALUES (?, ?, ?, ?, ?, ?)`, id, tenantID, userID, messageID, sessionID, rating,
				).Error)
			}
			insertFeedback(1, "exp-web", "m1", types.FeedbackRatingLike, 1)
			insertFeedback(2, "exp-web", "m2", types.FeedbackRatingDislike, 1)
			insertFeedback(3, "exp-api", "m3", types.FeedbackRatingLike, 1)
			// A same-id feedback row from another tenant must not leak into
			// the counts.
			insertFeedback(4, "exp-web", "m1", types.FeedbackRatingLike, 2)

			listRows := func(q *types.SessionListQuery) map[string]types.QueryHistoryExportRow {
				rows, err := repo.ExportSessionRows(ctx, q)
				require.NoError(t, err)
				byID := make(map[string]types.QueryHistoryExportRow, len(rows))
				for _, row := range rows {
					byID[row.SessionID] = row
				}
				return byID
			}

			// Whole-tenant view: every origin classified, tallies correct,
			// exclusions honored, chronological order.
			rows, err := repo.ExportSessionRows(ctx, &types.SessionListQuery{
				TenantID: 1, Source: types.SessionListSourceAll,
			})
			require.NoError(t, err)
			require.Len(t, rows, 4)
			require.Equal(t, []string{"exp-web", "exp-api", "exp-embed", "exp-im"}, []string{
				rows[0].SessionID, rows[1].SessionID, rows[2].SessionID, rows[3].SessionID,
			}, "rows must be chronological (oldest first)")

			all := listRows(&types.SessionListQuery{TenantID: 1, Source: types.SessionListSourceAll})
			require.Equal(t, "web", all["exp-web"].Source)
			require.Equal(t, "api", all["exp-api"].Source)
			require.Equal(t, "embed", all["exp-embed"].Source)
			require.Equal(t, "feishu", all["exp-im"].Source)
			require.Equal(t, int64(2), all["exp-web"].MessageCount)
			require.Equal(t, int64(1), all["exp-web"].LikeCount)
			require.Equal(t, int64(1), all["exp-web"].DislikeCount)
			require.Equal(t, int64(1), all["exp-api"].LikeCount)
			require.Equal(t, int64(0), all["exp-api"].DislikeCount)
			require.Equal(t, int64(0), all["exp-embed"].MessageCount)
			require.Equal(t, "u1", all["exp-web"].UserID)
			require.Equal(t, "builtin", all["exp-web"].EngineType)
			require.False(t, all["exp-web"].CreatedAt.IsZero())

			// Per-user drill-down keeps the legacy NULL/'' owner rows visible,
			// same rule as the listing.
			byUser := listRows(&types.SessionListQuery{TenantID: 1, UserID: "u1", Source: types.SessionListSourceAll})
			require.Contains(t, byUser, "exp-web")
			require.NotContains(t, byUser, "exp-api")

			// Time window: half-open [start, end).
			window := listRows(&types.SessionListQuery{
				TenantID: 1, Source: types.SessionListSourceAll,
				StartTime: base.AddDate(0, 0, 2), EndTime: base.AddDate(0, 0, 4),
			})
			require.Contains(t, window, "exp-api")
			require.Contains(t, window, "exp-embed")
			require.NotContains(t, window, "exp-web")
			require.NotContains(t, window, "exp-im")

			// Feedback drill-down.
			liked := listRows(&types.SessionListQuery{
				TenantID: 1, Source: types.SessionListSourceAll, FeedbackRating: types.FeedbackRatingLike,
			})
			require.Contains(t, liked, "exp-web")
			require.Contains(t, liked, "exp-api")
			require.NotContains(t, liked, "exp-embed")
		})
	}
}

// TestExportSessionRowsCap pins the anti-explosion limit: a tenant with more
// sessions than the export cap gets exactly the cap's worth of rows.
func TestExportSessionRowsCap(t *testing.T) {
	db := openQueryHistoryTestDB(t, "sqlite")
	repo := NewQueryHistoryExportJobRepository(db).(*queryHistoryExportJobRepository)
	require.NoError(t, db.Exec(`DELETE FROM sessions WHERE id IN ('s1', 's2')`).Error)

	// Bulk-insert cap+1 sessions with chunked raw SQL (no bound parameters).
	const total = queryHistoryExportRowLimit + 1
	var sb strings.Builder
	sb.WriteString("INSERT INTO sessions (id, tenant_id, title, engine_type, created_at, updated_at) VALUES ")
	for i := 0; i < total; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
		fmt.Fprintf(&sb, "('cap-%06d', 1, 't', 'builtin', '%s', '%s')", i,
			created.Format("2006-01-02 15:04:05"), created.Format("2006-01-02 15:04:05"))
	}
	require.NoError(t, db.Exec(sb.String()).Error)

	rows, err := repo.ExportSessionRows(context.Background(), &types.SessionListQuery{
		TenantID: 1, Source: types.SessionListSourceAll,
	})
	require.NoError(t, err)
	require.Len(t, rows, queryHistoryExportRowLimit)
	// The cap keeps the OLDEST rows (chronological audit artifact), dropping
	// the tail rather than the head.
	require.Equal(t, "cap-000000", rows[0].SessionID)
	require.Equal(t, "cap-009999", rows[len(rows)-1].SessionID)
}
