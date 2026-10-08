package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCraftKnowledgeRecordSQLitePersistence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:craft107_t05_records?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CraftKnowledgeRecordRow{}))
	store := NewCraftKnowledgeRecordRepository(db)
	scope := craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session-a"}
	acquired := time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC)
	record := craft.KnowledgeRecord{Scope: scope, RunID: "run-a", RequestDigest: "request-digest", PackageDigest: "package-digest", PublicationState: craft.KnowledgePublicationPrepared, Sources: []craft.KnowledgeSourceRecord{{ID: "kc-a", Ref: craft.KnowledgeRef("kb-a", "k-a", "c-a"), Digest: "abcdef", TenantID: 1, AcquiredAt: acquired, ExcerptBytes: 12}}}
	ctx := context.Background()
	require.NoError(t, store.Save(ctx, record))
	loaded, err := NewCraftKnowledgeRecordRepository(db).Load(ctx, scope, "run-a")
	require.NoError(t, err)
	require.Equal(t, record, loaded)
	require.NoError(t, store.Save(ctx, record), "exact replay is idempotent")
	require.ErrorIs(t, store.MarkPublished(ctx, scope, "run-a", "wrong-package"), craft.ErrConflict)
	stillPrepared, err := store.Load(ctx, scope, "run-a")
	require.NoError(t, err)
	require.Equal(t, craft.KnowledgePublicationPrepared, stillPrepared.PublicationState)
	require.NoError(t, store.MarkPublished(ctx, scope, "run-a", "package-digest"))
	require.NoError(t, store.MarkPublished(ctx, scope, "run-a", "package-digest"), "exact publication transition is idempotent")
	published, err := store.Load(ctx, scope, "run-a")
	require.NoError(t, err)
	require.Equal(t, craft.KnowledgePublicationPublished, published.PublicationState)
	published.PublicationState = craft.KnowledgePublicationPrepared
	require.Equal(t, record, published, "publication transition must preserve accepted source facts")
	changed := record
	changed.Sources = append([]craft.KnowledgeSourceRecord(nil), record.Sources...)
	changed.Sources[0].Digest = "changed"
	require.ErrorIs(t, store.Save(ctx, changed), craft.ErrConflict)
	changed = record
	changed.RequestDigest = "changed-request"
	require.ErrorIs(t, store.Save(ctx, changed), craft.ErrConflict)
	changed = record
	changed.PackageDigest = "changed-package"
	require.ErrorIs(t, store.Save(ctx, changed), craft.ErrConflict)
	_, err = store.Load(ctx, craft.Scope{TenantID: 2, SessionID: scope.SessionID}, "run-a")
	require.ErrorIs(t, err, craft.ErrNotFound)
	_, err = store.Load(ctx, craft.Scope{TenantID: 1, SessionID: "session-b"}, "run-a")
	require.ErrorIs(t, err, craft.ErrNotFound)
	require.NoError(t, db.Model(&CraftKnowledgeRecordRow{}).Where("run_id = ?", "run-a").Update("record_json", "{}").Error)
	_, err = store.Load(ctx, scope, "run-a")
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftKnowledgeRecordLegacyPublicationStateLoadsConservatively(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:craft107_t05_legacy_records?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CraftKnowledgeRecordRow{}))
	store := NewCraftKnowledgeRecordRepository(db)
	scope := craft.Scope{TenantID: 7, UserID: "owner", SessionID: "legacy-session"}
	legacy := struct {
		Scope                        craft.Scope
		RunID                        string
		RequestDigest, PackageDigest string
		Sources                      []craft.KnowledgeSourceRecord
		Empty, Truncated             bool
	}{Scope: scope, RunID: "legacy-run", RequestDigest: "request", PackageDigest: "accepted"}
	raw, err := json.Marshal(legacy)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	require.NoError(t, db.Create(&CraftKnowledgeRecordRow{
		TenantID: scope.TenantID, SessionID: scope.SessionID, RunID: legacy.RunID,
		RecordJSON: string(raw), Digest: hex.EncodeToString(sum[:]), AcquiredAt: time.Now().UTC(),
	}).Error)
	loaded, err := store.Load(context.Background(), scope, legacy.RunID)
	require.NoError(t, err)
	require.Empty(t, loaded.PublicationState, "legacy records remain unpublished until exact package recovery")
	require.NoError(t, store.MarkPublished(context.Background(), scope, legacy.RunID, legacy.PackageDigest))
	loaded, err = store.Load(context.Background(), scope, legacy.RunID)
	require.NoError(t, err)
	require.Equal(t, craft.KnowledgePublicationPublished, loaded.PublicationState)
}
