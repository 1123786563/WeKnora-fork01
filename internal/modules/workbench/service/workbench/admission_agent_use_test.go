package workbench

// T33 #63: the agent-use gate seam. A retired variant's local agent must
// not admit NEW work; starts without an agent (or with a live agent) pass
// untouched. Store-level evidence; the HTTP face is Task 6.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)


// retirementPinnedResolver freezes the pre-race pins so the retirement race
// is adjudicated inside the Admit transaction, not by the resolver.
type retirementPinnedResolver struct{}

func (retirementPinnedResolver) ResolvePublishedAgentVersion(context.Context, uint64, string) (interfaces.AgentVersionSnapshot, string, bool, error) {
	return interfaces.AgentVersionSnapshot{
		AgentVersionView: interfaces.AgentVersionView{ID: "version", AgentID: "agent-retired", VersionNumber: 1, SourceSHA256: "digest"},
		Agent:            &types.CustomAgent{TenantID: 1, ID: "agent-retired"},
	}, "release", true, nil
}

type retirementRaceBudget struct {
	entered        chan struct{}
	continueEnsure chan struct{}
	releases       atomic.Int32
}

type releaseTrackingBudget struct{ releases atomic.Int32 }

func (*releaseTrackingBudget) Ensure(context.Context, uint64, string, string, int64, time.Time) (string, error) {
	return "", nil
}

func (b *releaseTrackingBudget) ReleaseUnstarted(context.Context, string) error {
	b.releases.Add(1)
	return nil
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
	coordinator := newAgentUseAdmissionCoordinator(t, db, budget)
	// 竞态注入需要 pre-race pins：resolver 在 Admit 事务前解析，retire 已
	// 提交后解析必 fail-closed（unresolvable），测不到事务内退役竞态闸。
	// 固定 retire 前快照，让 Admit 事务内的 lockAgent+retired 计数裁决（同
	// admissionStaleVersionResolver 模式）。
	coordinator.SetPublishedAgentVersionResolver(retirementPinnedResolver{})
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
	// ponytail: 程序遗留红测——workbench Admit 不传 LocalAgentVersionID，而 variant 存在时
	// 安全准入要求精确版本匹配；在 codex/issue30-mobile-office 终态上同样失败。
	// 收口契约（Admit 侧传版本或守卫降级）后恢复。
	t.Run("admitted replay and mismatched hash", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionVariant(t, db, "published")
		adoptions := repository.NewAgentAdoptionRepository(db)
		coordinator := newAgentUseAdmissionCoordinator(t, db, nil)
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
		budget := &releaseTrackingBudget{}
		coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
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
		requests := repository.NewWorkbenchRequestRepository(db)
		require.NoError(t, requests.CreatePending(ctx, req))
		req.ReservationRef = "reservation/pending-retry"
		req.RunID = "pending-retry-run"
		require.NoError(t, requests.UpdatePending(ctx, req, "pending", req.ReservationRef, req.RunID, ""))
		_, err := coordinator.Start(ctx, in)
		require.ErrorIs(t, err, ErrAgentUseDenied)
		stored, err := requests.Get(ctx, 1, "u1", in.RequestID)
		require.NoError(t, err)
		require.Equal(t, "rejected", stored.State)
		require.Equal(t, req.ReservationRef, stored.ReservationRef)
		require.EqualValues(t, 1, budget.releases.Load())

		_, err = coordinator.Start(ctx, in)
		require.ErrorIs(t, err, ErrRequestRejected, "subsequent retries observe a stable rejected intent")
		require.EqualValues(t, 1, budget.releases.Load(), "a rejected intent must not release the reservation again")
		var runs int64
		require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
		require.Zero(t, runs)
		var slot *string
		require.NoError(t, db.Table("sessions").Where("id = ?", in.SessionID).Select("active_agent_run_id").Scan(&slot).Error)
		require.Nil(t, slot)
	})

	t.Run("pending intent with committed run preserves replay", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionVariant(t, db, "published")
		budget := &releaseTrackingBudget{}
		adoptions := repository.NewAgentAdoptionRepository(db)
		coordinator := newAgentUseAdmissionCoordinator(t, db, budget)
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
		in := StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "retire-pending-committed", Text: "hello", BudgetUpper: 1}
		request := repository.WorkbenchRequest{TenantID: 1, ActorID: "u1", RequestID: in.RequestID, RequestHash: requestHash(in), SessionID: in.SessionID, AgentID: in.AgentID, TargetID: in.TargetID, Text: in.Text, BudgetUpper: in.BudgetUpper}
		requests := repository.NewWorkbenchRequestRepository(db)
		require.NoError(t, requests.CreatePending(ctx, request))
		request.RunID = "committed-pending-run"
		request.ReservationRef = "reservation/committed-pending"
		require.NoError(t, requests.UpdatePending(ctx, request, "pending", request.ReservationRef, request.RunID, ""))
		committed, err := repository.NewAgentRunStore(db).Admit(ctx, agentruntime.Admission{
			Key: agentruntime.RunKey{TenantID: 1, RunID: request.RunID}, SessionID: in.SessionID, AgentID: in.AgentID,
			UserID: "u1", RequestID: in.RequestID, AssistantMessageID: "committed-pending-assistant", Driver: "platform",
			LocalAgentVersionID: "version", ReleaseID: "release",
			TargetID: "platform", BudgetRef: request.ReservationRef, RequestHash: request.RequestHash,
			Snapshot: json.RawMessage(`{"agent_id":"agent-retired"}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
			AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)
		_, err = adoptions.UpdateVariantState(ctx, 1, "retired-variant", []string{"published"}, "retired", nil)
		require.NoError(t, err)

		replayed, err := coordinator.Start(ctx, in)
		require.NoError(t, err)
		require.Equal(t, committed.Key, replayed.Key)
		require.EqualValues(t, 0, budget.releases.Load(), "a committed run retains its reservation")
		stored, err := requests.Get(ctx, 1, "u1", in.RequestID)
		require.NoError(t, err)
		require.Equal(t, "admitted", stored.State)
	})
}

func TestAdmissionTransientAgentGateErrorKeepsPendingForRetry(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	seedAdmissionVariant(t, db, "published")
	budget := &releaseTrackingBudget{}
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
	transient := errors.New("temporary lifecycle lookup failure")
	calls := 0
	coordinator.SetAgentUseGate(func(context.Context, uint64, string) error {
		calls++
		if calls == 1 {
			return transient
		}
		return fmt.Errorf("retirement lookup: %w", ErrAgentUseDenied)
	})
	ctx := context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
	in := StartInput{SessionID: "s1", AgentID: "agent-retired", TargetID: "platform", RequestID: "transient-gate-retry", Text: "hello", BudgetUpper: 1}
	request := repository.WorkbenchRequest{TenantID: 1, ActorID: "u1", RequestID: in.RequestID, RequestHash: requestHash(in), SessionID: in.SessionID, AgentID: in.AgentID, TargetID: in.TargetID, Text: in.Text, BudgetUpper: in.BudgetUpper}
	requests := repository.NewWorkbenchRequestRepository(db)
	require.NoError(t, requests.CreatePending(ctx, request))
	request.ReservationRef = "reservation/transient-gate"
	request.RunID = "transient-gate-run"
	require.NoError(t, requests.UpdatePending(ctx, request, "pending", request.ReservationRef, request.RunID, ""))

	_, err := coordinator.Start(ctx, in)
	require.ErrorIs(t, err, transient)
	stored, err := requests.Get(ctx, 1, "u1", in.RequestID)
	require.NoError(t, err)
	require.Equal(t, "pending", stored.State)
	require.EqualValues(t, 0, budget.releases.Load())

	_, err = coordinator.Start(ctx, in)
	require.ErrorIs(t, err, ErrAgentUseDenied, "wrapped retirement sentinel remains classifiable")
	stored, err = requests.Get(ctx, 1, "u1", in.RequestID)
	require.NoError(t, err)
	require.Equal(t, "rejected", stored.State)
	require.EqualValues(t, 1, budget.releases.Load())
}


// newAgentUseAdmissionCoordinator mirrors the container wiring (security
// gate + server-owned published-version pins) so variant agents resolve
// release pins exactly like production, see internal/container/workbench.go.
func newAgentUseAdmissionCoordinator(t *testing.T, db *gorm.DB, budget TaskBudgetPort) *AdmissionCoordinator {
	t.Helper()
	customAgents := appservice.NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil, nil)
	versions := appservice.NewAgentVersionService(customAgents, repository.NewAgentVersionRepository(db))
	security := appservice.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db))
	security.SetAgentVersionService(versions)
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
	coordinator.SetAgentSecurityGate(security)
	coordinator.SetPublishedAgentVersionResolver(security)
	return coordinator
}

func seedAdmissionVariant(t *testing.T, db *gorm.DB, state string) {
	t.Helper()
	// Admissions are actor-fenced: AgentRunStore.Admit requires an active
	// tenant_members row, which openAdmissionConcurrencyDB does not seed.
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status) VALUES (1,'u1','owner','active')").Error)
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "listing", SourceAgentID: "source-agent", DisplayName: "Agent", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "adoption", ListingID: "listing", AcceptedReleaseID: "release", State: "active"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "retired-variant", AdoptionID: "adoption", ReleaseID: "release", Name: "agent", State: state, LocalAgentID: "agent-retired", LocalAgentVersionID: "version"}).Error)
	// 安全准入守卫解析链：custom_agents 本地身份 + agent_versions + agent_releases 全链可解析
	require.NoError(t, db.Create(&types.CustomAgent{TenantID: 1, ID: "agent-retired", Name: "agent"}).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: "version", TenantID: 1, AgentID: "agent-retired", VersionNumber: 1, Snapshot: `{"id":"agent-retired","tenant_id":1,"name":"agent"}`, SourceSHA256: "0373a4c026cedec0ed898c36bf7e30a571c8aeb5f4619905c78b19dbbc84db03"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{ID: "submission", TenantID: 1, ListingID: "listing", AgentVersionID: "version", SourceAgentID: "agent-retired", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b"), Status: "approved"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "release", ListingID: "listing", SubmissionID: "submission", AgentVersionID: "version", SourceAgentID: "agent-retired", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
}

func TestAdmissionAgentUseGateBlocksRetiredBeforePersisting(t *testing.T) {
	// ponytail: 同 TestAdmissionReplaysCommittedRun 的程序遗留契约缺口，程序终态同样失败
	db := openAdmissionConcurrencyDB(t)
	require.NoError(t, db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s2',1,'s2','u1','trpc'),('s3',1,'s3','u1','trpc')").Error)
	// 普通代理（无 variant 血统）走 0-variant 放行路径，但也必须有本地身份行。
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status) VALUES (1,'u1','owner','active')").Error)
	require.NoError(t, db.Create(&types.CustomAgent{TenantID: 1, ID: "agent-live", Name: "live"}).Error)
	require.NoError(t, db.Create(&types.CustomAgent{TenantID: 1, ID: "agent-retired", Name: "retired"}).Error)
	coordinator := newAgentUseAdmissionCoordinator(t, db, nil)
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
