//go:build semantic_integration

package repository

import (
	"context"
	"os"
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
	db := openRunTestDB(t)
	listingID, releaseID := seedAdoptionRelease(t, db, 1, "agent-end-race", "1.0.0")
	repo := NewAgentAdoptionRepository(db)
	adoption, _, err := repo.AdoptListing(context.Background(), &types.AgentAdoptionEntity{TenantID: 1, ListingID: listingID, AcceptedReleaseID: releaseID, CreatedBy: "admin"})
	require.NoError(t, err)
	other := reopenRunDB(t, db)
	insertPaused := make(chan struct{})
	releaseInsert := make(chan struct{})
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
	defer db.Callback().Create().Remove(name)
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
	go func() {
		_, err := NewAgentAdoptionRepository(other).EndAdoption(context.Background(), 1, adoption.ID, "admin", "closed")
		endDone <- err
	}()
	var waiting bool
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var count int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND query ILIKE '%agent_adoptions%'`).Scan(&count).Error)
		if count > 0 {
			waiting = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	require.True(t, waiting, "EndAdoption must be observed waiting on the parent gate")
	close(releaseInsert)
	require.NoError(t, <-createDone)
	require.ErrorIs(t, <-endDone, ErrAgentAdoptionTransition)
	stored, err := repo.GetAdoption(context.Background(), 1, adoption.ID)
	require.NoError(t, err)
	require.Equal(t, "active", stored.State)
	variants, err := repo.ListVariantsByAdoption(context.Background(), 1, adoption.ID)
	require.NoError(t, err)
	require.Len(t, variants, 1)
	require.NotEqual(t, "retired", variants[0].State)
}
