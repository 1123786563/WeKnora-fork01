package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// openAgentVersionServiceTestDB applies the REAL versioned SQLite migration
// stream (including 000108_agent_versions) so the service tests exercise the
// production schema and the real repository, not an AutoMigrate sketch.
func openAgentVersionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "agent-versions.db")
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

	db, err := gorm.Open(gormsqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, e := db.DB()
		if e == nil {
			_ = conn.Close()
		}
	})
	return db
}

// fakeAgentVersionSource fakes the TenantExpertAgentSource-shaped seam the
// freeze flow consumes: a tenant-scoped map, with the same not-found
// behavior as the real custom-agent service.
type fakeAgentVersionSource struct {
	agents map[string]*types.CustomAgent
}

func (f *fakeAgentVersionSource) GetAgentByIDAndTenant(
	_ context.Context, id string, tenantID uint64,
) (*types.CustomAgent, error) {
	if agent, ok := f.agents[fmt.Sprintf("%d|%s", tenantID, id)]; ok {
		return agent, nil
	}
	return nil, ErrAgentNotFound
}

func newAgentVersionServiceForTest(t *testing.T) (*AgentVersionService, *fakeAgentVersionSource) {
	t.Helper()
	source := &fakeAgentVersionSource{agents: map[string]*types.CustomAgent{}}
	svc := NewAgentVersionService(source, repository.NewAgentVersionRepository(openAgentVersionServiceTestDB(t)))
	return svc, source
}

func requireNotFound(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr), "expected an AppError, got %v", err)
	require.Equal(t, http.StatusNotFound, appErr.HTTPCode)
}

// TestAgentVersionServiceFreezeAppendsImmutableSnapshots proves the core
// service contract: consecutive freezes get increasing version numbers, the
// snapshot is a COPY (later edits to the live agent never rewrite history)
// and the digest is content-derived (identical state re-freezes to the same
// digest under a new number).
func TestAgentVersionServiceFreezeAppendsImmutableSnapshots(t *testing.T) {
	svc, source := newAgentVersionServiceForTest(t)
	ctx := context.Background()
	source.agents["1|agent-a"] = &types.CustomAgent{
		ID: "agent-a", TenantID: 1, Name: "Draft", Description: "first state",
		CreatedBy: "user-1", Config: types.CustomAgentConfig{SystemPrompt: "v1 prompt"},
	}

	first, err := svc.FreezeAgentVersion(ctx, 1, "user-1", "agent-a")
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)
	require.Equal(t, "agent-a", first.AgentID)
	require.Equal(t, 1, first.VersionNumber)
	require.Equal(t, "user-1", first.FrozenBy)
	require.Len(t, first.SourceSHA256, 64, "source digest is sha256 hex")
	require.False(t, first.CreatedAt.IsZero())

	// Edit the live agent AFTER the freeze: the frozen version must still
	// return the bytes captured at freeze time.
	source.agents["1|agent-a"].Name = "Renamed"
	source.agents["1|agent-a"].Config.SystemPrompt = "v2 prompt"

	second, err := svc.FreezeAgentVersion(ctx, 1, "user-1", "agent-a")
	require.NoError(t, err)
	require.Equal(t, 2, second.VersionNumber)
	require.NotEqual(t, first.ID, second.ID)
	require.NotEqual(t, first.SourceSHA256, second.SourceSHA256, "different content must hash differently")

	gotFirst, err := svc.GetAgentVersion(ctx, 1, first.ID)
	require.NoError(t, err)
	require.Equal(t, "Draft", gotFirst.Agent.Name, "the frozen snapshot must not follow live edits")
	require.Equal(t, "v1 prompt", gotFirst.Agent.Config.SystemPrompt)
	require.Equal(t, first.SourceSHA256, gotFirst.SourceSHA256)

	gotSecond, err := svc.GetAgentVersion(ctx, 1, second.ID)
	require.NoError(t, err)
	require.Equal(t, "Renamed", gotSecond.Agent.Name)
	require.Equal(t, "v2 prompt", gotSecond.Agent.Config.SystemPrompt)

	// Identical state re-freezes to the same digest under a new number.
	third, err := svc.FreezeAgentVersion(ctx, 1, "user-1", "agent-a")
	require.NoError(t, err)
	require.Equal(t, 3, third.VersionNumber)
	require.Equal(t, second.SourceSHA256, third.SourceSHA256,
		"the digest is content-derived: the same state hashes identically")

	// Listing is ascending and complete.
	views, err := svc.ListAgentVersions(ctx, 1, "agent-a")
	require.NoError(t, err)
	require.Len(t, views, 3)
	require.Equal(t, 1, views[0].VersionNumber)
	require.Equal(t, 2, views[1].VersionNumber)
	require.Equal(t, 3, views[2].VersionNumber)
}

// TestAgentVersionServiceReadsAreTenantScopedFailClosed proves cross-tenant
// reads fail closed: another tenant sees neither the version nor the
// listing, and gets not-found rather than data.
func TestAgentVersionServiceReadsAreTenantScopedFailClosed(t *testing.T) {
	svc, source := newAgentVersionServiceForTest(t)
	ctx := context.Background()
	source.agents["1|agent-a"] = &types.CustomAgent{
		ID: "agent-a", TenantID: 1, Name: "Owned by tenant 1",
	}

	frozen, err := svc.FreezeAgentVersion(ctx, 1, "user-1", "agent-a")
	require.NoError(t, err)

	_, err = svc.GetAgentVersion(ctx, 2, frozen.ID)
	requireNotFound(t, err)
	views, err := svc.ListAgentVersions(ctx, 2, "agent-a")
	require.NoError(t, err)
	require.Empty(t, views)

	// The owning tenant still reads it (fail-closed is the tenant
	// predicate, not a broken lookup).
	got, err := svc.GetAgentVersion(ctx, 1, frozen.ID)
	require.NoError(t, err)
	require.Equal(t, "Owned by tenant 1", got.Agent.Name)

	// Freezing another tenant's agent id is the wrong-tenant 404 too.
	_, err = svc.FreezeAgentVersion(ctx, 2, "user-2", "agent-a")
	requireNotFound(t, err)
}

// TestAgentVersionServiceFreezeRequiresExistingAgent proves the source must
// exist in the caller's tenant: unknown ids, blanks and unknown version ids
// on reads are not-found; the actor is recorded server-side.
func TestAgentVersionServiceFreezeRequiresExistingAgent(t *testing.T) {
	svc, _ := newAgentVersionServiceForTest(t)
	ctx := context.Background()

	_, err := svc.FreezeAgentVersion(ctx, 1, "user-1", "missing-agent")
	requireNotFound(t, err)
	_, err = svc.FreezeAgentVersion(ctx, 1, "user-1", "   ")
	requireNotFound(t, err)
	_, err = svc.GetAgentVersion(ctx, 1, "missing-version")
	requireNotFound(t, err)

	// The interface contract holds on the concrete type.
	var _ interfaces.AgentVersionService = svc
}

// TestAgentVersionServiceGetRejectsTamperedSnapshot proves the read path
// verifies the stored bytes against the recorded digest: a tampered row
// refuses to serve instead of returning content that no longer matches its
// Marketplace reference.
func TestAgentVersionServiceGetRejectsTamperedSnapshot(t *testing.T) {
	db := openAgentVersionServiceTestDB(t)
	source := &fakeAgentVersionSource{agents: map[string]*types.CustomAgent{
		"1|agent-a": {ID: "agent-a", TenantID: 1, Name: "Original"},
	}}
	svc := NewAgentVersionService(source, repository.NewAgentVersionRepository(db))
	ctx := context.Background()

	frozen, err := svc.FreezeAgentVersion(ctx, 1, "user-1", "agent-a")
	require.NoError(t, err)

	// Tamper directly in the same database — the only way, since the
	// repository exposes no update path.
	require.NoError(t, db.Exec(
		`UPDATE agent_versions SET snapshot = ? WHERE id = ? AND tenant_id = 1`,
		`{"name":"tampered"}`, frozen.ID,
	).Error)

	_, err = svc.GetAgentVersion(ctx, 1, frozen.ID)
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, http.StatusInternalServerError, appErr.HTTPCode)
	require.True(t, strings.Contains(appErr.Message, "digest"), "the failure must name the digest mismatch")
}
