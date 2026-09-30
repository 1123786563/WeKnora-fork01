//go:build semantic_integration

package repository

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
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
		createDone := make(chan struct{})
		var createErr error
		go func() {
			_, createErr = repo.CreateVariant(context.Background(), &types.AgentAdoptionVariantEntity{TenantID: 1, AdoptionID: adoption.ID, ReleaseID: releaseID, Name: "Sales"})
			close(createDone)
		}()
		select {
		case <-insertPaused:
		case <-time.After(10 * time.Second):
			t.Fatal("CreateVariant did not reach its paused insert after acquiring parent gate")
		}
		endDone := make(chan struct{})
		endSQLDB, err := other.DB()
		require.NoError(t, err)
		conn, err := endSQLDB.Conn(context.Background())
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		var endPID int
		require.NoError(t, conn.QueryRowContext(context.Background(), "SELECT pg_backend_pid()").Scan(&endPID))
		var endErr error
		go func() {
			// postgres.Dialector.Init replaces Config.ConnPool unless its own
			// Conn field is set. Bind the pinned sql.Conn at the dialector seam.
			pinnedDialector := postgres.New(postgres.Config{Conn: conn})
			pinned, err := gorm.Open(pinnedDialector, &gorm.Config{Logger: other.Config.Logger})
			if err != nil {
				endErr = err
				close(endDone)
				return
			}
			var actualPID int
			err = pinned.Raw("SELECT pg_backend_pid()").Scan(&actualPID).Error
			if err == nil && actualPID != endPID {
				err = fmt.Errorf("pinned End handle backend PID %d differs from observed PID %d", actualPID, endPID)
			}
			if err == nil {
				_, err = NewAgentAdoptionRepository(pinned).EndAdoption(context.Background(), 1, adoption.ID, "admin", "closed")
			}
			endErr = err
			close(endDone)
		}()
		// Always release the creator first, then wait a bounded time for both
		// goroutines before their DB handles or test schema are cleaned up.
		t.Cleanup(func() {
			release()
			for label, done := range map[string]<-chan struct{}{"CreateVariant": createDone, "EndAdoption": endDone} {
				select {
				case <-done:
				case <-time.After(15 * time.Second):
					t.Errorf("timed out joining %s goroutine during cleanup", label)
				}
			}
		})
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
		<-createDone
		require.NoError(t, createErr)
		<-endDone
		require.ErrorIs(t, endErr, ErrAgentAdoptionTransition)
		stored, err := repo.GetAdoption(context.Background(), 1, adoption.ID)
		require.NoError(t, err)
		require.Equal(t, "active", stored.State)
		variants, err := repo.ListVariantsByAdoption(context.Background(), 1, adoption.ID)
		require.NoError(t, err)
		require.Len(t, variants, 1)
		require.NotEqual(t, "retired", variants[0].State)
	})
}
