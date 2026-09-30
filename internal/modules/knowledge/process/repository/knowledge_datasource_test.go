package repository

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertDatasourceKnowledge seeds one knowledges row carrying
// metadata.datasource_id = dsID in the given (tenant, kb) scope. When deleted
// is true the row is born soft-deleted (tombstone) so it must never surface.
func insertDatasourceKnowledge(t *testing.T, repo *knowledgeRepository, tenantID uint64, kbID, dsID, title string, deleted bool) string {
	t.Helper()
	id := uuid.New().String()
	metadata := fmt.Sprintf(`{"datasource_id":%q}`, dsID)
	var deletedAt interface{}
	if deleted {
		deletedAt = "2026-01-01 00:00:00"
	}
	require.NoError(t, repo.db.Exec(`
		INSERT INTO knowledges
		  (id, tenant_id, knowledge_base_id, type, title, source, parse_status, metadata, deleted_at)
		VALUES (?, ?, ?, 'document', ?, 'feishu', 'completed', ?, ?)
	`, id, tenantID, kbID, title, metadata, deletedAt).Error)
	return id
}

func TestFindKnowledgeIDsByDataSourceID(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 60
	kbID := uuid.New().String()
	dsA := uuid.New().String()
	dsB := uuid.New().String()

	// Three live docs from dsA, one live doc from another source, one
	// soft-deleted dsA tombstone that must stay invisible.
	idA1 := insertDatasourceKnowledge(t, repo, tenantID, kbID, dsA, "doc-a1", false)
	idA2 := insertDatasourceKnowledge(t, repo, tenantID, kbID, dsA, "doc-a2", false)
	idA3 := insertDatasourceKnowledge(t, repo, tenantID, kbID, dsA, "doc-a3", false)
	insertDatasourceKnowledge(t, repo, tenantID, kbID, dsB, "doc-b1", false)
	insertDatasourceKnowledge(t, repo, tenantID, kbID, dsA, "doc-gone", true)

	// limit <= 0 falls back to the default batch size (200), so all three
	// live dsA rows come back, ordered by id ascending.
	ids, err := repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbID, dsA, 0)
	require.NoError(t, err)
	expected := []string{idA1, idA2, idA3}
	sort.Strings(expected)
	assert.Equal(t, expected, ids)

	// Negative limit behaves the same as zero.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbID, dsA, -1)
	require.NoError(t, err)
	assert.Equal(t, expected, ids)

	// The count matches the live dsA rows only.
	count, err := repo.CountKnowledgeByDataSourceID(ctx, tenantID, kbID, dsA)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// The other source in the same KB sees exactly its own row.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbID, dsB, 0)
	require.NoError(t, err)
	assert.Len(t, ids, 1)

	count, err = repo.CountKnowledgeByDataSourceID(ctx, tenantID, kbID, dsB)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestFindKnowledgeIDsByDataSourceID_LimitBatchesStable(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 61
	kbID := uuid.New().String()
	dsID := uuid.New().String()

	all := []string{
		insertDatasourceKnowledge(t, repo, tenantID, kbID, dsID, "doc-1", false),
		insertDatasourceKnowledge(t, repo, tenantID, kbID, dsID, "doc-2", false),
		insertDatasourceKnowledge(t, repo, tenantID, kbID, dsID, "doc-3", false),
	}
	sort.Strings(all)

	// The drain loop (fetch batch -> delete -> repeat) relies on each call
	// returning the lowest remaining ids deterministically.
	batch1, err := repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbID, dsID, 2)
	require.NoError(t, err)
	require.Equal(t, all[:2], batch1)

	// Deleting the first batch advances the cursor: the next call returns
	// only the remaining row, in the same global order.
	require.NoError(t, repo.HardDeleteKnowledgeList(ctx, tenantID, batch1))

	batch2, err := repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbID, dsID, 2)
	require.NoError(t, err)
	assert.Equal(t, all[2:], batch2)

	count, err := repo.CountKnowledgeByDataSourceID(ctx, tenantID, kbID, dsID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestFindKnowledgeIDsByDataSourceID_ScopeIsolation(t *testing.T) {
	db := setupKnowledgeTestDB(t)
	repo := NewKnowledgeRepository(db).(*knowledgeRepository)
	ctx := context.Background()

	const tenantID uint64 = 62
	const otherTenantID uint64 = 63
	kbA := uuid.New().String()
	kbB := uuid.New().String()
	dsA := uuid.New().String()
	dsB := uuid.New().String()

	// Every row shares exactly two of the three (tenant, kb, ds) key parts
	// with the target row, so only the full triple matches it.
	want := insertDatasourceKnowledge(t, repo, tenantID, kbA, dsA, "target", false)
	otherKBRow := insertDatasourceKnowledge(t, repo, tenantID, kbB, dsA, "other-kb", false)
	otherDSRow := insertDatasourceKnowledge(t, repo, tenantID, kbA, dsB, "other-ds", false)
	otherTenantRow := insertDatasourceKnowledge(t, repo, otherTenantID, kbA, dsA, "other-tenant", false)

	ids, err := repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbA, dsA, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{want}, ids)

	// Wrong knowledge base: only that KB's own row, never the target.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbB, dsA, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{otherKBRow}, ids)

	// Wrong data source: only that source's own row — identical external
	// items from another source must never be swept by this query.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbA, dsB, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{otherDSRow}, ids)

	// A knowledge base with no datasource-synced rows at all: nothing.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, uuid.New().String(), dsA, 0)
	require.NoError(t, err)
	assert.Empty(t, ids)

	// A data source id that synced nothing here: nothing.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, tenantID, kbA, uuid.New().String(), 0)
	require.NoError(t, err)
	assert.Empty(t, ids)

	// Another tenant sees only its own row, never tenantID's.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, otherTenantID, kbA, dsA, 0)
	require.NoError(t, err)
	assert.Equal(t, []string{otherTenantRow}, ids)
	// A tenant with no rows at all: nothing.
	ids, err = repo.FindKnowledgeIDsByDataSourceID(ctx, 999, kbA, dsA, 0)
	require.NoError(t, err)
	assert.Empty(t, ids)

	// Count honors the same triple scoping.
	count, err := repo.CountKnowledgeByDataSourceID(ctx, tenantID, kbA, dsA)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = repo.CountKnowledgeByDataSourceID(ctx, tenantID, kbA, dsB)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = repo.CountKnowledgeByDataSourceID(ctx, tenantID, kbB, dsA)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = repo.CountKnowledgeByDataSourceID(ctx, tenantID, uuid.New().String(), uuid.New().String())
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
