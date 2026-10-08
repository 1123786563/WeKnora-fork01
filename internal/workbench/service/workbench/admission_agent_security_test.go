package workbench

// T34 #64 Task 8E: the workbench admission security seam. A revoked release
// must refuse NEW work before any durable write, an already-admitted Run must
// replay untouched, and the server-resolved Version/Release pins must reach
// the runtime admission (the 8C transaction guard stays authoritative).

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	appservice "github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type admissionSecurityGateFunc func(context.Context, uint64, string) (interfaces.AgentSecurityVerdict, error)

func (f admissionSecurityGateFunc) VerdictForAgent(ctx context.Context, tenantID uint64, agentID string) (interfaces.AgentSecurityVerdict, error) {
	return f(ctx, tenantID, agentID)
}

// admissionStaticVersions answers the ResolvePublishedAgentVersion identity
// checks for the seeded marketplace rows without touching agent_versions.
type admissionStaticVersions struct{}

func (admissionStaticVersions) FreezeAgentVersion(context.Context, uint64, string, string) (interfaces.AgentVersionView, error) {
	return interfaces.AgentVersionView{}, nil
}

func (admissionStaticVersions) ListAgentVersions(context.Context, uint64, string) ([]interfaces.AgentVersionView, error) {
	return nil, nil
}

func (admissionStaticVersions) GetAgentVersion(_ context.Context, tenantID uint64, versionID string) (interfaces.AgentVersionSnapshot, error) {
	return interfaces.AgentVersionSnapshot{
		AgentVersionView: interfaces.AgentVersionView{ID: versionID, AgentID: "agent-market", VersionNumber: 1, SourceSHA256: "digest"},
		Agent:            &types.CustomAgent{TenantID: tenantID, ID: "agent-market"},
	}, nil
}

// admissionStaleVersionResolver freezes the pre-race pins: the published
// Variant has already moved on by the time the admission guard runs.
type admissionStaleVersionResolver struct{}

func (admissionStaleVersionResolver) ResolvePublishedAgentVersion(context.Context, uint64, string) (interfaces.AgentVersionSnapshot, string, bool, error) {
	return interfaces.AgentVersionSnapshot{AgentVersionView: interfaces.AgentVersionView{ID: "version-1", AgentID: "agent-market"}}, "release-1", true, nil
}

type admissionSecurityBudget struct{ releases atomic.Int32 }

func (*admissionSecurityBudget) Ensure(_ context.Context, _ uint64, _, requestID string, _ int64, _ time.Time) (string, error) {
	return "reservation/" + requestID, nil
}
func (b *admissionSecurityBudget) ReleaseUnstarted(context.Context, string) error {
	b.releases.Add(1)
	return nil
}

func admissionSecurityContext() context.Context {
	return context.WithValue(context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1)), types.UserIDContextKey, "u1")
}

// seedAdmissionMarketplace publishes one adopted agent with a resolvable
// dependency lock, so the real AgentSecurityService verdicts and pins work.
func seedAdmissionMarketplace(t *testing.T, db *gorm.DB) {
	t.Helper()
	// Admissions are actor-fenced: AgentRunStore.Admit requires an active
	// tenant_members row, which openAdmissionConcurrencyDB does not seed.
	require.NoError(t, db.Exec("INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES (1,'u1','owner','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)").Error)
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{TenantID: 1, ID: "listing", SourceAgentID: "source-agent", DisplayName: "Agent", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{TenantID: 1, ID: "adoption", ListingID: "listing", AcceptedReleaseID: "release-1", State: "active"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{TenantID: 1, ID: "variant", AdoptionID: "adoption", ReleaseID: "release-1", Name: "agent", State: "published", LocalAgentID: "agent-market", LocalAgentVersionID: "version-1"}).Error)
	require.NoError(t, db.Create(&types.CustomAgent{TenantID: 1, ID: "agent-market", Name: "agent"}).Error)
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: "version-1", TenantID: 1, AgentID: "agent-market", VersionNumber: 1, Snapshot: "{}", SourceSHA256: "digest"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{ID: "submission-1", TenantID: 1, ListingID: "listing", AgentVersionID: "version-1", SourceAgentID: "agent-market", SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b"), Status: "approved"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "release-1", ListingID: "listing", SubmissionID: "submission-1", AgentVersionID: "version-1", SourceAgentID: "agent-market", ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "digest", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b")}).Error)
}

func seedAdmissionReleaseRevocation(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, db.Create(&types.AgentReleaseRevocationEntity{
		TenantID: 1, ListingID: "listing", ReleaseID: "release-1", Reason: "compromised bundle",
		InFlightDisposition: interfaces.AgentSecurityInFlightAllow, RunCancellationState: interfaces.AgentSecurityRunCancellationComplete,
		RevokedBy: "admin", RevokedAt: now, CreatedAt: now,
	}).Error)
}

// switchAdmissionVariantToV2 republishes the variant onto version-2/release-2.
func switchAdmissionVariantToV2(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: "version-2", TenantID: 1, AgentID: "agent-market", VersionNumber: 2, Snapshot: "{}", SourceSHA256: "digest-2"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{ID: "submission-2", TenantID: 1, ListingID: "listing", AgentVersionID: "version-2", SourceAgentID: "agent-market", SemanticVersion: "2.0.0", BundleDigest: "digest-2", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b2"), Status: "approved"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{TenantID: 1, ID: "release-2", ListingID: "listing", SubmissionID: "submission-2", AgentVersionID: "version-2", SourceAgentID: "agent-market", ReleaseNumber: 2, SemanticVersion: "2.0.0", BundleDigest: "digest-2", ManifestJSON: "{}", DependencyLockJSON: `{"dependencies":[]}`, Bundle: []byte("b2")}).Error)
	require.NoError(t, db.Exec("UPDATE agent_adoption_variants SET local_agent_version_id = 'version-2', release_id = 'release-2' WHERE tenant_id = 1 AND id = 'variant'").Error)
}

func newAdmissionSecurityCoordinator(t *testing.T, db *gorm.DB, budget TaskBudgetPort) *AdmissionCoordinator {
	t.Helper()
	security := appservice.NewAgentSecurityService(repository.NewAgentSecurityStore(db), repository.NewAgentRunStore(db))
	security.SetAgentVersionService(admissionStaticVersions{})
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
	coordinator.SetAgentSecurityGate(security)
	coordinator.SetPublishedAgentVersionResolver(security)
	return coordinator
}

func admissionRunPin(t *testing.T, db *gorm.DB, requestID string) *string {
	t.Helper()
	var pin *string
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", requestID).Select("security_local_agent_version_id").Scan(&pin).Error)
	return pin
}

func TestAdmissionAgentSecurityReplayAfterRevocationReturnsExistingRun(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	seedAdmissionMarketplace(t, db)
	coordinator := newAdmissionSecurityCoordinator(t, db, nil)
	ctx := admissionSecurityContext()
	in := StartInput{SessionID: "s1", AgentID: "agent-market", TargetID: "platform", RequestID: "security-replay", Text: "hello", BudgetUpper: 1}

	first, err := coordinator.Start(ctx, in)
	require.NoError(t, err)
	require.NotEmpty(t, first.Key.RunID)
	pin := admissionRunPin(t, db, in.RequestID)
	require.NotNil(t, pin)
	require.Equal(t, "version-1", *pin, "the admitted run must carry the server-resolved version pin")

	seedAdmissionReleaseRevocation(t, db)
	replayed, err := coordinator.Start(ctx, in)
	require.NoError(t, err, "a revocation must not disturb the already-admitted replay")
	require.Equal(t, first.Key, replayed.Key)
	var runCount int64
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runCount).Error)
	require.EqualValues(t, 1, runCount)
}

func TestAdmissionAgentSecurityBlockedBeforeNewPendingIntent(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	seedAdmissionMarketplace(t, db)
	seedAdmissionReleaseRevocation(t, db)
	budget := &admissionSecurityBudget{}
	coordinator := newAdmissionSecurityCoordinator(t, db, budget)
	ctx := admissionSecurityContext()
	in := StartInput{SessionID: "s1", AgentID: "agent-market", TargetID: "platform", RequestID: "security-blocked-new", Text: "hello", BudgetUpper: 1}

	_, err := coordinator.Start(ctx, in)
	require.ErrorIs(t, err, ErrAgentSecurityBlocked)
	var requests, runs, messages int64
	require.NoError(t, db.Table("workbench_requests").Where("request_id = ?", in.RequestID).Count(&requests).Error)
	require.Zero(t, requests, "a blocked start must not create a durable request")
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
	require.Zero(t, runs)
	require.NoError(t, db.Table("messages").Where("request_id = ?", in.RequestID).Count(&messages).Error)
	require.Zero(t, messages)
	require.EqualValues(t, 0, budget.releases.Load())
}

func TestAdmissionAgentSecurityRejectsPendingAndReleasesReservationExactlyOnce(t *testing.T) {
	t.Run("two denied retries settle one rejection", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionMarketplace(t, db)
		seedAdmissionReleaseRevocation(t, db)
		budget := &admissionSecurityBudget{}
		coordinator := newAdmissionSecurityCoordinator(t, db, budget)
		ctx := admissionSecurityContext()
		in := StartInput{SessionID: "s1", AgentID: "agent-market", TargetID: "platform", RequestID: "security-pending-race", Text: "hello", BudgetUpper: 1}
		requests := repository.NewWorkbenchRequestRepository(db)
		req := repository.WorkbenchRequest{TenantID: 1, ActorID: "u1", RequestID: in.RequestID, RequestHash: requestHash(in), SessionID: in.SessionID, AgentID: in.AgentID, TargetID: in.TargetID, Text: in.Text, BudgetUpper: in.BudgetUpper}
		require.NoError(t, requests.CreatePending(ctx, req))
		req.ReservationRef = "reservation/security-pending-race"
		req.RunID = "security-pending-race-run"
		require.NoError(t, requests.UpdatePending(ctx, req, "pending", req.ReservationRef, req.RunID, ""))

		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := coordinator.Start(ctx, in)
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		blocked, rejected := 0, 0
		for err := range errs {
			require.Error(t, err)
			switch {
			case errors.Is(err, ErrAgentSecurityBlocked):
				blocked++
			case errors.Is(err, ErrRequestRejected):
				rejected++
			default:
				t.Fatalf("unexpected denial identity: %v", err)
			}
		}
		require.Equal(t, 1, blocked, "exactly one retry must observe the security denial")
		require.Equal(t, 1, rejected, "the losing retry must observe the terminal rejection")
		require.EqualValues(t, 1, budget.releases.Load(), "the pending reservation must be released exactly once")
		stored, err := requests.Get(ctx, 1, "u1", in.RequestID)
		require.NoError(t, err)
		require.Equal(t, "rejected", stored.State)
		var runs int64
		require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
		require.Zero(t, runs)
	})

	t.Run("committed run replay wins over denial", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionMarketplace(t, db)
		runs := repository.NewAgentRunStore(db)
		requests := repository.NewWorkbenchRequestRepository(db)
		ctx := admissionSecurityContext()
		in := StartInput{SessionID: "s1", AgentID: "agent-market", TargetID: "platform", RequestID: "security-committed-wins", Text: "hello", BudgetUpper: 1}
		req := repository.WorkbenchRequest{TenantID: 1, ActorID: "u1", RequestID: in.RequestID, RequestHash: requestHash(in), SessionID: in.SessionID, AgentID: in.AgentID, TargetID: in.TargetID, Text: in.Text, BudgetUpper: in.BudgetUpper}
		require.NoError(t, requests.CreatePending(ctx, req))
		req.RunID = "security-committed-run"
		req.ReservationRef = "reservation/security-committed"
		require.NoError(t, requests.UpdatePending(ctx, req, "pending", req.ReservationRef, req.RunID, ""))
		committed, err := runs.Admit(ctx, agentruntime.Admission{
			Key: agentruntime.RunKey{TenantID: 1, RunID: req.RunID}, SessionID: in.SessionID, AgentID: in.AgentID,
			LocalAgentVersionID: "version-1", ReleaseID: "release-1",
			UserID: "u1", RequestID: in.RequestID, AssistantMessageID: "security-committed-assistant", Driver: "platform",
			TargetID: "platform", BudgetRef: req.ReservationRef, RequestHash: req.RequestHash,
			Snapshot: json.RawMessage(`{"agent_id":"agent-market"}`), UserMessage: json.RawMessage(`{"role":"user","content":"hello"}`),
			AssistantMessage: json.RawMessage(`{"role":"assistant","content":""}`), Deadline: time.Now().Add(time.Hour),
		})
		require.NoError(t, err)

		seedAdmissionReleaseRevocation(t, db)
		budget := &admissionSecurityBudget{}
		coordinator := newAdmissionSecurityCoordinator(t, db, budget)
		replayed, err := coordinator.Start(ctx, in)
		require.NoError(t, err)
		require.Equal(t, committed.Key, replayed.Key)
		require.EqualValues(t, 0, budget.releases.Load(), "a committed run retains its reservation")
		stored, err := requests.Get(ctx, 1, "u1", in.RequestID)
		require.NoError(t, err)
		require.Equal(t, "admitted", stored.State)
	})
}

func TestAdmissionAgentSecurityInfraErrorFailsClosed(t *testing.T) {
	db := openAdmissionConcurrencyDB(t)
	seedAdmissionMarketplace(t, db)
	budget := &admissionSecurityBudget{}
	coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
	infra := errors.New("security verdict backend unavailable")
	coordinator.SetAgentSecurityGate(admissionSecurityGateFunc(func(context.Context, uint64, string) (interfaces.AgentSecurityVerdict, error) {
		return interfaces.AgentSecurityVerdict{}, infra
	}))
	ctx := admissionSecurityContext()
	in := StartInput{SessionID: "s1", AgentID: "agent-market", TargetID: "platform", RequestID: "security-infra", Text: "hello", BudgetUpper: 1}

	_, err := coordinator.Start(ctx, in)
	require.ErrorIs(t, err, infra, "an infrastructure failure must surface unwrapped")
	var requests, runs int64
	require.NoError(t, db.Table("workbench_requests").Where("request_id = ?", in.RequestID).Count(&requests).Error)
	require.Zero(t, requests, "an infra failure must not open the gate or create durable state")
	require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
	require.Zero(t, runs)
	require.EqualValues(t, 0, budget.releases.Load())
}

func TestAdmissionAgentSecurityStaleVersionRaceRejected(t *testing.T) {
	t.Run("stale resolved pins rejected without writes", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		require.NoError(t, db.Exec("INSERT INTO sessions (id,tenant_id,title,user_id,engine_type) VALUES ('s2',1,'s2','u1','trpc')").Error)
		seedAdmissionMarketplace(t, db)
		switchAdmissionVariantToV2(t, db)
		budget := &admissionSecurityBudget{}
		coordinator := NewAdmissionCoordinator(db, repository.NewAgentRunStore(db), budget, nil)
		coordinator.SetAgentSecurityGate(admissionSecurityGateFunc(func(context.Context, uint64, string) (interfaces.AgentSecurityVerdict, error) {
			return interfaces.AgentSecurityVerdict{State: interfaces.AgentSecurityVerdictOK}, nil
		}))
		coordinator.SetPublishedAgentVersionResolver(admissionStaleVersionResolver{})
		ctx := admissionSecurityContext()
		in := StartInput{SessionID: "s2", AgentID: "agent-market", TargetID: "platform", RequestID: "security-stale-race", Text: "hello", BudgetUpper: 1}

		_, err := coordinator.Start(ctx, in)
		require.ErrorIs(t, err, repository.ErrAgentSecurityReleaseUnresolvable, "the 8C transaction guard must reject the stale pin")
		var runs, messages int64
		require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runs).Error)
		require.Zero(t, runs)
		require.NoError(t, db.Table("messages").Where("request_id = ?", in.RequestID).Count(&messages).Error)
		require.Zero(t, messages)
		var slot *string
		require.NoError(t, db.Table("sessions").Where("id = ?", in.SessionID).Select("active_agent_run_id").Scan(&slot).Error)
		require.Nil(t, slot, "no writer slot may be reserved")
		stored, getErr := repository.NewWorkbenchRequestRepository(db).Get(ctx, 1, "u1", in.RequestID)
		require.NoError(t, getErr)
		require.Equal(t, "rejected", stored.State)
		require.EqualValues(t, 1, budget.releases.Load(), "the unstarted reservation is released once")
	})

	t.Run("admitted replay keeps the original version pin", func(t *testing.T) {
		db := openAdmissionConcurrencyDB(t)
		seedAdmissionMarketplace(t, db)
		coordinator := newAdmissionSecurityCoordinator(t, db, nil)
		ctx := admissionSecurityContext()
		in := StartInput{SessionID: "s1", AgentID: "agent-market", TargetID: "platform", RequestID: "security-pin-replay", Text: "hello", BudgetUpper: 1}
		first, err := coordinator.Start(ctx, in)
		require.NoError(t, err)

		switchAdmissionVariantToV2(t, db)
		replayed, err := coordinator.Start(ctx, in)
		require.NoError(t, err)
		require.Equal(t, first.Key, replayed.Key)
		pin := admissionRunPin(t, db, in.RequestID)
		require.NotNil(t, pin)
		require.Equal(t, "version-1", *pin, "the admitted run keeps its original version pin")
		var runCount int64
		require.NoError(t, db.Table("agent_runs").Where("request_id = ?", in.RequestID).Count(&runCount).Error)
		require.EqualValues(t, 1, runCount)
	})
}
