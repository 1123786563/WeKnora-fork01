package adapters

// Legacy-audit adapter tests (Wave 1, Task 7, brief Step 1): the ports.AuditReader
// contract over the legacy Session/Message/Feedback reads. Tenant scope must ride
// the session lookup itself (a foreign session is indistinguishable from a missing
// one), messages are capped at the legacy 200-row snapshot cap with the newest
// kept, and the export rows reuse the legacy audit predicates (source
// classification, tallies, exclusions, half-open window, 10,000-row cap).

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openAuditTestDB mirrors the repository package's SQLite test path: a real
// migrated schema (migrations/sqlite) in a per-test temp dir, seeded with the
// two tenants the scoping assertions need. PostgreSQL-only SQL branches stay
// covered by the repository package's dialect matrix (query-history tests).
func openAuditTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// adapters -> queryhistory -> conversation -> internal -> repo root
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	dbPath := filepath.Join(t.TempDir(), "queryhistory-adapters.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"

	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		`INSERT INTO tenants (id, name, business) VALUES (1, 'tenant-1', 'test'), (2, 'tenant-2', 'test')`,
	).Error)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// insertAuditMessage inserts one message with an explicit created_at so row
// ordering under the 200-message cap is deterministic.
func insertAuditMessage(t *testing.T, db *gorm.DB, id, sessionID string, created time.Time) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO messages (id, request_id, session_id, role, content, created_at)
		 VALUES (?, ?, ?, 'user', 'q', ?)`,
		id, "req-"+id, sessionID, created.Format("2006-01-02 15:04:05"),
	).Error)
}

func insertAuditFeedback(
	t *testing.T, db *gorm.DB, id int64, tenantID uint64, sessionID, messageID, rating string,
) {
	t.Helper()
	userID := "u1"
	if tenantID != 1 {
		userID = "u-other-tenant"
	}
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (id, tenant_id, user_id, message_id, session_id, rating)
		 VALUES (?, ?, ?, ?, ?, ?)`, id, tenantID, userID, messageID, sessionID, rating,
	).Error)
}

func TestLegacyAuditSnapshotAssemblesTenantScopedSnapshot(t *testing.T) {
	db := openAuditTestDB(t)
	audit := NewLegacyAudit(repository.NewSessionAuditRepository(db))
	ctx := context.Background()

	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, description, engine_type)
		 VALUES ('snap-1', 1, 'Audit', 'u1', '', 'builtin')`,
	).Error)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		insertAuditMessage(t, db, fmt.Sprintf("sm-%d", i), "snap-1", base.Add(time.Duration(i)*time.Minute))
	}
	insertAuditFeedback(t, db, 1, 1, "snap-1", "sm-0", types.FeedbackRatingLike)
	insertAuditFeedback(t, db, 2, 1, "snap-1", "sm-1", types.FeedbackRatingDislike)
	// A same-session feedback row from another tenant must never leak in.
	insertAuditFeedback(t, db, 3, 2, "snap-1", "sm-0", types.FeedbackRatingLike)

	snap, err := audit.Snapshot(ctx, 1, "snap-1")
	require.NoError(t, err)
	require.Equal(t, "snap-1", snap.Session.ID)
	require.Equal(t, uint64(1), snap.Session.TenantID)
	require.Equal(t, "u1", snap.Session.UserID)
	require.Len(t, snap.Messages, 3)
	require.Equal(t, "sm-0", snap.Messages[0].ID, "snapshot messages are oldest first")
	require.Equal(t, "sm-2", snap.Messages[2].ID)
	require.False(t, snap.Truncated)
	require.NotNil(t, snap.Feedback, "feedback must serialize as [] rather than null")
	require.Len(t, snap.Feedback, 2, "every tenant feedback row rides along; the foreign-tenant row does not leak")

	// Tenant scope rides the session lookup itself: a foreign session and a
	// missing one answer the SAME error, so ids cannot be probed across
	// workspaces.
	_, foreignErr := audit.Snapshot(ctx, 2, "snap-1")
	_, missingErr := audit.Snapshot(ctx, 1, "no-such-session")
	require.ErrorIs(t, foreignErr, apperrors.ErrSessionNotFound)
	require.ErrorIs(t, missingErr, apperrors.ErrSessionNotFound)
	require.Equal(t, missingErr.Error(), foreignErr.Error(),
		"the foreign-session miss must be indistinguishable from the plain miss")

	// Argument validation mirrors the legacy snapshot entrance.
	_, err = audit.Snapshot(ctx, 0, "snap-1")
	require.Error(t, err)
	_, err = audit.Snapshot(ctx, 1, "")
	require.Error(t, err)
}

func TestLegacyAuditSnapshotCapsMessagesAtTwoHundred(t *testing.T) {
	db := openAuditTestDB(t)
	audit := NewLegacyAudit(repository.NewSessionAuditRepository(db))
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, engine_type) VALUES ('snap-cap', 1, 'Cap', 'builtin')`,
	).Error)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 205; i++ {
		insertAuditMessage(t, db, fmt.Sprintf("cm-%03d", i), "snap-cap", base.Add(time.Duration(i)*time.Second))
	}

	snap, err := audit.Snapshot(context.Background(), 1, "snap-cap")
	require.NoError(t, err)
	require.Len(t, snap.Messages, 200, "the snapshot keeps at most the 200 most recent messages")
	require.True(t, snap.Truncated)
	require.Equal(t, "cm-005", snap.Messages[0].ID, "rows past the cap drop the OLDEST messages")
	require.Equal(t, "cm-204", snap.Messages[199].ID)
}

func TestLegacyAuditSnapshotEmptyRelationsAreNotNil(t *testing.T) {
	db := openAuditTestDB(t)
	audit := NewLegacyAudit(repository.NewSessionAuditRepository(db))
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, engine_type) VALUES ('snap-empty', 1, 'Empty', 'builtin')`,
	).Error)

	snap, err := audit.Snapshot(context.Background(), 1, "snap-empty")
	require.NoError(t, err)
	require.NotNil(t, snap.Messages)
	require.Empty(t, snap.Messages)
	require.NotNil(t, snap.Feedback)
	require.Empty(t, snap.Feedback)
}

// TestLegacyAuditExportRowsAggregationAndFilters ports the legacy export-row
// contract through the port: origin classification (IM platform / embed /
// api / web), message and like/dislike tallies, the audit-view exclusions
// (soft-deleted and skill-maintenance sessions), chronological order, and the
// admin drill-down filters (user, half-open time window, feedback rating).
func TestLegacyAuditExportRowsAggregationAndFilters(t *testing.T) {
	db := openAuditTestDB(t)
	audit := NewLegacyAudit(repository.NewSessionAuditRepository(db))
	ctx := context.Background()

	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	insertSession := func(id string, tenantID uint64, userID, description string, day int) {
		require.NoError(t, db.Exec(
			`INSERT INTO sessions (id, tenant_id, title, user_id, description, engine_type)
			 VALUES (?, ?, ?, ?, ?, 'builtin')`,
			id, tenantID, "title-"+id, userID, description,
		).Error)
		// Bind timestamps through GORM so the driver serializes them exactly
		// the way it serializes the query bounds.
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
	require.NoError(t, db.Exec(`UPDATE sessions SET deleted_at = ? WHERE id = 'exp-deleted'`, base).Error)
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

	insertAuditFeedback(t, db, 1, 1, "exp-web", "m1", types.FeedbackRatingLike)
	insertAuditFeedback(t, db, 2, 1, "exp-web", "m2", types.FeedbackRatingDislike)
	insertAuditFeedback(t, db, 3, 1, "exp-api", "m3", types.FeedbackRatingLike)
	// A same-id feedback row from another tenant must not leak into the counts.
	insertAuditFeedback(t, db, 4, 2, "exp-web", "m1", types.FeedbackRatingLike)

	rowsBy := func(tenantID uint64, filter domain.ExportFilter) map[string]domain.ExportRow {
		rows, err := audit.ExportRows(ctx, tenantID, filter)
		require.NoError(t, err)
		byID := make(map[string]domain.ExportRow, len(rows))
		for _, row := range rows {
			byID[row.SessionID] = row
		}
		return byID
	}

	// Whole-tenant view: every origin classified, tallies correct, exclusions
	// honored, chronological order.
	rows, err := audit.ExportRows(ctx, 1, domain.ExportFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 4)
	require.Equal(t, []string{"exp-web", "exp-api", "exp-embed", "exp-im"}, []string{
		rows[0].SessionID, rows[1].SessionID, rows[2].SessionID, rows[3].SessionID,
	}, "rows must be chronological (oldest first)")

	all := rowsBy(1, domain.ExportFilter{})
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

	// Tenant scope: the foreign tenant's export never sees tenant 1 rows.
	foreign := rowsBy(2, domain.ExportFilter{})
	require.Len(t, foreign, 1)
	require.Contains(t, foreign, "exp-other")

	// Per-user drill-down keeps the legacy NULL/'' owner rows visible.
	byUser := rowsBy(1, domain.ExportFilter{UserID: "u1"})
	require.Contains(t, byUser, "exp-web")
	require.NotContains(t, byUser, "exp-api")

	// Time window: half-open [start, end).
	window := rowsBy(1, domain.ExportFilter{
		StartTime: base.AddDate(0, 0, 2), EndTime: base.AddDate(0, 0, 4),
	})
	require.Contains(t, window, "exp-api")
	require.Contains(t, window, "exp-embed")
	require.NotContains(t, window, "exp-web")
	require.NotContains(t, window, "exp-im")

	// Feedback drill-down.
	liked := rowsBy(1, domain.ExportFilter{FeedbackRating: types.FeedbackRatingLike})
	require.Contains(t, liked, "exp-web")
	require.Contains(t, liked, "exp-api")
	require.NotContains(t, liked, "exp-embed")

	// The port's zero tenant id is refused before any SQL runs.
	_, err = audit.ExportRows(ctx, 0, domain.ExportFilter{})
	require.Error(t, err)
}

// TestLegacyAuditExportRowsCap pins the anti-explosion limit: a tenant with
// more sessions than the export cap (10,000) gets exactly the cap's worth of
// rows, keeping the OLDEST (the CSV is a chronological audit artifact).
func TestLegacyAuditExportRowsCap(t *testing.T) {
	db := openAuditTestDB(t)
	audit := NewLegacyAudit(repository.NewSessionAuditRepository(db))

	const total = 10000 + 1
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

	rows, err := audit.ExportRows(context.Background(), 1, domain.ExportFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 10000)
	require.Equal(t, "cap-000000", rows[0].SessionID)
	require.Equal(t, "cap-009999", rows[len(rows)-1].SessionID)
}
