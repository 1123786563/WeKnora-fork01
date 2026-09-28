package workbench

// T33 #63: the agent-use gate seam. A retired variant's local agent must
// not admit NEW work; starts without an agent (or with a live agent) pass
// untouched. Store-level evidence; the HTTP face is Task 6.

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type retirementRaceBudget struct {
	entered        chan struct{}
	continueEnsure chan struct{}
	releases       atomic.Int32
}

func (b *retirementRaceBudget) Ensure(context.Context, uint64, string, string, int64, time.Time) (string, error) {
	close(b.entered)
	<-b.continueEnsure
	return "reservation/race", nil
}

func (b *retirementRaceBudget) ReleaseUnstarted(context.Context, string) error {
	b.releases.Add(1)
	return nil
}

func TestAdmissionRetirementBetweenFastGateAndRunCommitIsDenied(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	seedAdmissionVariant(t, db, "published")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	budget := &retirementRaceBudget{entered: make(chan struct{}), continueEnsure: make(chan struct{})}
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
	coordinator.SetAgentUseGate(func(ctx context.Context, tenant uint64, agentID string) error {
		retired, gateErr := repository.NewAgentAdoptionRepository(db).RetiredVariantAgentExists(ctx, tenant, agentID)
		if gateErr != nil {
			return gateErr
		}
		if retired {
			return ErrAgentUseDenied
		}
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "retirement-race", Text: "hello", BudgetUpper: 1}
	startDone := make(chan error, 1)
	go func() { _, startErr := coordinator.Start(ctx, in); startDone <- startErr }()
	<-budget.entered // fast gate and pending request have completed; Admit has not started
	_, err = repository.NewAgentAdoptionRepository(db).UpdateVariantState(context.Background(), 1, "retired-variant", []string{"published"}, "retired", nil)
	require.NoError(t, err)
	close(budget.continueEnsure)
	require.ErrorIs(t, <-startDone, ErrAgentUseDenied)
	request, err := repository.NewWorkbenchRequestRepository(db).Get(ctx, 1, "u1", in.RequestID)
	require.NoError(t, err)
	require.Equal(t, "rejected", request.State)
	require.Equal(t, "reservation/race", request.ReservationRef)
	require.EqualValues(t, 1, budget.releases.Load())
	var runs int64
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
	require.Zero(t, runs)
	var slot *string
	require.NoError(t, db.Table("sessions").Where("id = ?", in.SessionID).Select("active_agent_run_id").Scan(&slot).Error)
	require.Nil(t, slot)
}

func TestAdmissionReplaysCommittedRunAfterRetirementButGatesPendingRetry(t *testing.T) {
	t.Run("admitted replay and mismatched hash", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionVariant(t, db, "published")
		adoptions := repository.NewAgentAdoptionRepository(db)
		coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), nil, nil)
		calls := 0
		coordinator.SetAgentUseGate(func(ctx context.Context, tenant uint64, agentID string) error {
			calls++
			retired, gateErr := adoptions.RetiredVariantAgentExists(ctx, tenant, agentID)
			if gateErr != nil {
				return gateErr
			}
			if retired {
				return ErrAgentUseDenied
			}
			return nil
		})
		ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
		in := StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "retire-replay", Text: "hello", BudgetUpper: 1}
		first, err := coordinator.Start(ctx, in)
		require.NoError(t, err)
		_, err = adoptions.UpdateVariantState(ctx, 1, "retired-variant", []string{"published"}, "retired", nil)
		require.NoError(t, err)
		replayed, err := coordinator.Start(ctx, in)
		require.NoError(t, err)
		require.Equal(t, first.Key, replayed.Key)
		changed := in
		changed.Text = "different immutable request"
		_, err = coordinator.Start(ctx, changed)
		require.ErrorIs(t, err, agentruntime.ErrConflict)
		require.Equal(t, 1, calls, "committed replay and hash conflict must be resolved before the fast gate")
	})

	t.Run("pending retry after retirement", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionVariant(t, db, "retired")
		coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), nil, nil)
		adoptions := repository.NewAgentAdoptionRepository(db)
		coordinator.SetAgentUseGate(func(ctx context.Context, tenant uint64, agentID string) error {
			retired, gateErr := adoptions.RetiredVariantAgentExists(ctx, tenant, agentID)
			if gateErr != nil {
				return gateErr
			}
			if retired {
				return ErrAgentUseDenied
			}
			return nil
		})
		ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
		in := StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "retire-pending", Text: "hello", BudgetUpper: 1}
		req := repository.WorkbenchRequest{TenantID: 1, ActorID: "u1", RequestID: in.RequestID, RequestHash: requestHash(in), SessionID: in.SessionID, AgentID: in.AgentID, TargetID: in.TargetID, Text: in.Text, BudgetUpper: in.BudgetUpper}
		require.NoError(t, repository.NewWorkbenchRequestRepository(db).CreatePending(ctx, req))
		_, err := coordinator.Start(ctx, in)
		require.ErrorIs(t, err, ErrAgentUseDenied)
		stored, err := repository.NewWorkbenchRequestRepository(db).Get(ctx, 1, "u1", in.RequestID)
		require.NoError(t, err)
		require.Equal(t, "pending", stored.State, "the early fail-closed gate leaves existing pending intent for explicit recovery")
	})
}

func seedAdmissionVariant(t *testing.T, db interface{ Create(value any) *gorm.DB }, state string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "listing", SourceAgentID: "source-agent", DisplayName: "Agent", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "adoption", ListingID: "listing", AcceptedReleaseID: "release", State: "active"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "retired-variant", AdoptionID: "adoption", ReleaseID: "release", Name: "agent", State: state, LocalAgentID: "agent-retired"}).Error)
}

func TestAdmissionAgentUseGateBlocksRetiredBeforePersisting(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	require.NoError(t, db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s2',1,'s2','u1','trpc'),('s3',1,'s3','u1','trpc')").Error)
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), nil, nil)
	calls := 0
	coordinator.SetAgentUseGate(func(_ context.Context, _ uint64, agentID string) error {
		calls++
		if agentID == "agent-retired" {
			return ErrAgentUseDenied
		}
		return nil
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")

	_, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "r1", Text: "hi", BudgetUpper: 1})
	require.ErrorIs(t, err, ErrAgentUseDenied)
	var pending int64
	require.NoError(t, db.Table("workbench_requests").Where("request_id = ?", "r1").Count(&pending).Error)
	require.EqualValues(t, 0, pending)

	run, err := coordinator.Start(ctx, StartInput{SessionID: "s1", AgentID: "agent-live", TargetID: "platform", RequestID: "r2", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)

	before := calls
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s2", TargetID: "platform", RequestID: "r3", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
	require.Equal(t, before, calls)

	coordinator.SetAgentUseGate(nil)
	_, err = coordinator.Start(ctx, StartInput{SessionID: "s3", AgentID: "agent-retired", TargetID: "platform", RequestID: "r4", Text: "hi", BudgetUpper: 1})
	require.NoError(t, err)
}
