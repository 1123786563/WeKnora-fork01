package repository

// Session-audit repository tests (Wave 1, Task 7, brief Step 5): the temporary
// SessionAuditRepository seam the module's LegacyAudit adapter wraps. Snapshot
// reuses the legacy Session/Message/Feedback reads (tenant scope riding the
// session lookup, 200-message cap keeping the newest, every feedback row);
// ExportRows shares ONE predicate implementation with the legacy
// QueryHistoryExportJobRepository.ExportSessionRows entry.

import (
	"context"
	"fmt"
	"testing"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestSessionAuditSnapshotAssemblyAndScope(t *testing.T) {
	db := openQueryHistoryTestDB(t, "sqlite")
	audit := NewSessionAuditRepository(db)
	ctx := context.Background()

	require.NoError(t, db.Exec(`DELETE FROM sessions WHERE id IN ('s1', 's2')`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, description, engine_type)
		 VALUES ('sa-1', 1, 'Audit', 'u1', '', 'builtin')`,
	).Error)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Exec(
			`INSERT INTO messages (id, request_id, session_id, role, content, created_at)
			 VALUES (?, ?, 'sa-1', 'user', 'q', ?)`,
			fmt.Sprintf("sam-%d", i), fmt.Sprintf("sareq-%d", i),
			base.Add(time.Duration(i)*time.Minute).Format("2006-01-02 15:04:05"),
		).Error)
	}
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (id, tenant_id, user_id, message_id, session_id, rating)
		 VALUES (1, 1, 'u1', 'sam-0', 'sa-1', 'like'), (2, 2, 'foreign', 'sam-0', 'sa-1', 'like')`,
	).Error)

	snap, err := audit.Snapshot(ctx, 1, "sa-1")
	require.NoError(t, err)
	require.Equal(t, "sa-1", snap.Session.ID)
	require.Equal(t, "u1", snap.Session.UserID)
	require.Len(t, snap.Messages, 3)
	require.Equal(t, "sam-0", snap.Messages[0].ID, "messages are oldest first")
	require.False(t, snap.Truncated)
	require.Len(t, snap.Feedback, 1, "the foreign-tenant feedback row must not leak")

	// Tenant scope rides the lookup itself: a foreign session and a missing
	// one answer the SAME sentinel, so ids cannot be probed across tenants.
	_, foreignErr := audit.Snapshot(ctx, 2, "sa-1")
	_, missingErr := audit.Snapshot(ctx, 1, "nope")
	require.ErrorIs(t, foreignErr, apperrors.ErrSessionNotFound)
	require.ErrorIs(t, missingErr, apperrors.ErrSessionNotFound)
	require.Equal(t, missingErr.Error(), foreignErr.Error())

	// Argument validation mirrors the legacy snapshot entrance.
	_, err = audit.Snapshot(ctx, 0, "sa-1")
	require.Error(t, err)
	_, err = audit.Snapshot(ctx, 1, "")
	require.Error(t, err)
}

func TestSessionAuditSnapshotCapsMessagesAtTwoHundred(t *testing.T) {
	db := openQueryHistoryTestDB(t, "sqlite")
	audit := NewSessionAuditRepository(db)
	require.NoError(t, db.Exec(`DELETE FROM sessions WHERE id IN ('s1', 's2')`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, engine_type) VALUES ('sa-cap', 1, 'Cap', 'builtin')`,
	).Error)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 205; i++ {
		require.NoError(t, db.Exec(
			`INSERT INTO messages (id, request_id, session_id, role, content, created_at)
			 VALUES (?, ?, 'sa-cap', 'user', 'q', ?)`,
			fmt.Sprintf("scm-%03d", i), fmt.Sprintf("screq-%03d", i),
			base.Add(time.Duration(i)*time.Second).Format("2006-01-02 15:04:05"),
		).Error)
	}

	snap, err := audit.Snapshot(context.Background(), 1, "sa-cap")
	require.NoError(t, err)
	require.Len(t, snap.Messages, 200)
	require.True(t, snap.Truncated)
	require.Equal(t, "scm-005", snap.Messages[0].ID, "rows past the cap drop the OLDEST messages")
	require.Equal(t, "scm-204", snap.Messages[199].ID)
}

// TestSessionAuditExportRowsSharesPredicatesWithLegacyEntry pins the ONE
// predicate implementation: the domain-filter entry and the legacy
// SessionListQuery entry must return identical rows over the same data.
func TestSessionAuditExportRowsSharesPredicatesWithLegacyEntry(t *testing.T) {
	db := openQueryHistoryTestDB(t, "sqlite")
	audit := NewSessionAuditRepository(db)
	legacy := NewQueryHistoryExportJobRepository(db).(*queryHistoryExportJobRepository)
	ctx := context.Background()

	require.NoError(t, db.Exec(`DELETE FROM sessions WHERE id IN ('s1', 's2')`).Error)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	insertSession := func(id string, tenantID uint64, userID, description string, day int) {
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, description, engine_type)
			 VALUES (?, ?, ?, ?, ?, 'builtin')`,
			id, tenantID, "title-"+id, userID, description,
		).Error)
		created := base.AddDate(0, 0, day)
		require.NoError(t, db.Model(&types.Session{}).Where("id = ?", id).Updates(
			map[string]interface{}{"created_at": created, "updated_at": created},
		).Error)
	}
	insertSession("sa-web", 1, "u1", "", 1)
	insertSession("sa-api", 1, "api_tenant_key:k1", "", 2)
	insertSession("sa-embed", 1, "", "embed_channel:ch1", 3)
	insertSession("sa-im", 1, "", "", 4)
	insertSession("sa-skill", 1, "u1", "skill_maintenance:job1", 5)
	insertSession("sa-other", 2, "u2", "", 1)
	require.NoError(t, db.Exec(
		`INSERT INTO im_channel_sessions (id, platform, user_id, chat_id, session_id, tenant_id)
		 VALUES ('sa-ics-1', 'feishu', 'fu1', 'chat1', 'sa-im', 1)`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO messages (id, request_id, session_id, role, content) VALUES
		 ('sam-1', 'r1', 'sa-web', 'user', 'q'), ('sam-2', 'r2', 'sa-web', 'user', 'q'),
		 ('sam-3', 'r3', 'sa-api', 'user', 'q')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (id, tenant_id, user_id, message_id, session_id, rating) VALUES
		 (11, 1, 'u1', 'sam-1', 'sa-web', 'like'), (12, 1, 'u1', 'sam-2', 'sa-web', 'dislike'),
		 (13, 1, 'u1', 'sam-3', 'sa-api', 'like')`,
	).Error)

	cases := []struct {
		name   string
		filter domain.ExportFilter
		legacy types.SessionListQuery
	}{
		{
			name:   "whole tenant",
			filter: domain.ExportFilter{},
			legacy: types.SessionListQuery{Source: types.SessionListSourceAll},
		},
		{
			name:   "per-user drill-down",
			filter: domain.ExportFilter{UserID: "u1"},
			legacy: types.SessionListQuery{Source: types.SessionListSourceAll, UserID: "u1"},
		},
		{
			name: "half-open window with feedback drill-down",
			filter: domain.ExportFilter{
				StartTime:      base.AddDate(0, 0, 2),
				EndTime:        base.AddDate(0, 0, 4),
				FeedbackRating: types.FeedbackRatingLike,
			},
			legacy: types.SessionListQuery{
				Source:         types.SessionListSourceAll,
				StartTime:      base.AddDate(0, 0, 2),
				EndTime:        base.AddDate(0, 0, 4),
				FeedbackRating: types.FeedbackRatingLike,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.legacy.TenantID = 1
			legacyRows, err := legacy.ExportSessionRows(ctx, &tc.legacy)
			require.NoError(t, err)
			auditRows, err := audit.ExportRows(ctx, 1, tc.filter)
			require.NoError(t, err)
			require.Len(t, auditRows, len(legacyRows))
			for i := range legacyRows {
				require.Equal(t, legacyRows[i], auditRows[i],
					"both entries must answer from ONE predicate implementation")
			}
			require.NotEmpty(t, auditRows, "the case must be non-vacuous")
		})
	}

	// Tenant scope on the new entry.
	foreign, err := audit.ExportRows(ctx, 2, domain.ExportFilter{})
	require.NoError(t, err)
	require.Len(t, foreign, 1)
	require.Equal(t, "sa-other", foreign[0].SessionID)

	_, err = audit.ExportRows(ctx, 0, domain.ExportFilter{})
	require.Error(t, err)
}
