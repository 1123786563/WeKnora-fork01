//go:build semantic_integration

package repository

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestAgentAdoptionEndWaitsThenSeesCommittedVariant reproduces the READ
// COMMITTED snapshot race with two independent DB handles. A callback pauses
// CreateVariant after its parent gate while EndAdoption starts and waits on
// that row; PostgreSQL pg_stat_activity confirms the wait before Create resumes.
func TestAgentAdoptionEndWaitsThenSeesCommittedVariant(t *testing.T) {
	if os.Getenv("TRPC_TEST_POSTGRES_DSN") == "" {
		t.Skip("TRPC_TEST_POSTGRES_DSN unset: PostgreSQL READ COMMITTED race evidence blocked-env")
	}
	t.Run("postgres", func(t *testing.T) {
		db := openRunTestDB(t)
		require.Equal(t, "postgres", db.Name(), "the postgres subtest must use the PostgreSQL migration harness")
		listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-end-race", "1.0.0")
		repo := NewAgentAdoptionRepository(db)
		adoption, _, err := repo.AdoptListing(context.Background(), &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, CreatedBy: "admin"})
		require.NoError(t, err)
		other := reopenRunDB(t, db)
		insertPaused := make(chan struct{})
		releaseInsert := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseInsert) }) }
		t.Cleanup(release)
		var paused bool
		name := "test:pause_variant_create_after_parent_gate"
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
			if paused || tx.Statement == nil || tx.Statement.Table != "agent_adoption_variants" {
				return
			}
			paused = true
			close(insertPaused)
			<-releaseInsert
		}))
		t.Cleanup(func() { _ = db.Callback().Create().Remove(name) })
		createDone := make(chan error, 1)
		go func() {
			_, err := repo.CreateVariant(context.Background(), &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "Sales"})
			createDone <- err
		}()
		select {
		case <-insertPaused:
		case <-time.After(10 * time.Second):
			t.Fatal("CreateVariant did not reach its paused insert after acquiring parent gate")
		}
		endDone := make(chan error, 1)
		endSQLDB, err := other.DB()
		require.NoError(t, err)
		conn, err := endSQLDB.Conn(context.Background())
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		var endPID int
		require.NoError(t, conn.QueryRowContext(context.Background(), "SELECT pg_backend_pid()").Scan(&endPID))
		go func() {
			// Keep this repository transaction on the same backend whose PID is
			// observed below.
			pinned, err := gorm.Open(other.Dialector, &gorm.Config{ConnPool: conn, Logger: other.Config.Logger})
			if err != nil {
				endDone <- err
				return
			}
			_, err = NewAgentAdoptionRepository(pinned).EndAdoption(context.Background(), 1, adoption.ID, "admin", "closed")
			endDone <- err
		}()
		var waiting bool
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			var waitType, query string
			err := db.Raw(`SELECT wait_event_type, query FROM pg_stat_activity WHERE pid = ?`, endPID).Row().Scan(&waitType, &query)
			require.NoError(t, err)
			if waitType == "Lock" && strings.Contains(query, "agent_adoptions") {
				waiting = true
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		require.True(t, waiting, "EndAdoption must be observed waiting on the parent gate")
		release()
		require.NoError(t, <-createDone)
		require.ErrorIs(t, <-endDone, ErrAgentAdoptionTransition)
		stored, err := repo.GetAdoption(context.Background(), 1, adoption.ID)
		require.NoError(t, err)
		require.Equal(t, "active", stored.State)
		variants, err := repo.ListVariantsByAdoption(context.Background(), 1, adoption.ID)
		require.NoError(t, err)
		require.Len(t, variants, 1)
		require.NotEqual(t, "retired", variants[0].State)
	})
}
