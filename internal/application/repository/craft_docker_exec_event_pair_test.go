package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordDockerExecEventPairPersistsClaimedTiming(t *testing.T) {
	forEachCraftDockerSendDB(t, func(t *testing.T, db *gorm.DB) {
		key, revision := craftDockerSendFixture(t, db, "run-event-pair", "activity-event-pair")
		claims := NewCraftDockerSendClaimRepository(db)
		ctx := context.Background()
		receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-event", ExecID: "exec-event"}
		require.NoError(t, claims.BindDockerExecReceipt(ctx, key, revision, receipt))
		claimed, err := claims.ClaimDockerExecSend(ctx, key, revision, receipt)
		require.NoError(t, err)
		require.True(t, claimed)

		require.NoError(t, claims.RecordDockerExecEventPair(ctx, key, receipt, 1758796800_000000000, 1758796801_500000000))
		var row struct {
			StartedNS  *int64  `gorm:"column:exec_event_started_at_ns"`
			FinishedNS *int64  `gorm:"column:exec_event_finished_at_ns"`
			Source     *string `gorm:"column:duration_source"`
		}
		require.NoError(t, db.Table("craft_charge_start_journal").Select("exec_event_started_at_ns, exec_event_finished_at_ns, duration_source").
			Where("tenant_id = ? AND run_id = ? AND activity_key = ?", key.TenantID, key.RunID, key.ActivityKey).Take(&row).Error)
		require.NotNil(t, row.StartedNS)
		require.EqualValues(t, 1758796800_000000000, *row.StartedNS)
		require.NotNil(t, row.FinishedNS)
		require.EqualValues(t, 1758796801_500000000, *row.FinishedNS)
		require.NotNil(t, row.Source)
		require.Equal(t, "docker_exec_events", *row.Source)

		// Identical replay is idempotent evidence, never a second opinion.
		require.NoError(t, claims.RecordDockerExecEventPair(ctx, key, receipt, 1758796800_000000000, 1758796801_500000000))
		// Any different pair for the same claimed receipt conflicts.
		err = claims.RecordDockerExecEventPair(ctx, key, receipt, 1758796800_000000000, 1758796802_000000000)
		require.ErrorIs(t, err, craft.ErrConflict)
	})
}

func TestRecordDockerExecEventPairRequiresExactClaimedReceipt(t *testing.T) {
	forEachCraftDockerSendDB(t, func(t *testing.T, db *gorm.DB) {
		key, revision := craftDockerSendFixture(t, db, "run-event-unclaimed", "activity-event-unclaimed")
		claims := NewCraftDockerSendClaimRepository(db)
		ctx := context.Background()
		receipt := DockerExecReceipt{Provider: "docker", ContainerID: "container-event", ExecID: "exec-event"}

		// Bound but not yet claimed: timing evidence cannot attach.
		require.NoError(t, claims.BindDockerExecReceipt(ctx, key, revision, receipt))
		require.ErrorIs(t, claims.RecordDockerExecEventPair(ctx, key, receipt, 1, 2), craft.ErrConflict)

		claimed, err := claims.ClaimDockerExecSend(ctx, key, revision, receipt)
		require.NoError(t, err)
		require.True(t, claimed)

		// A different receipt for the same operation must not be overwritten.
		foreign := DockerExecReceipt{Provider: "docker", ContainerID: "container-event", ExecID: "exec-other"}
		require.ErrorIs(t, claims.RecordDockerExecEventPair(ctx, key, foreign, 1, 2), craft.ErrConflict)

		// Non-monotonic evidence is refused by the store contract.
		require.Error(t, claims.RecordDockerExecEventPair(ctx, key, receipt, 5, 4))
	})
}
