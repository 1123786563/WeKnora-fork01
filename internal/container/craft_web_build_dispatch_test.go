package container

// F08 T-1: the web build dispatch orchestrator. The fixed offline build
// command is dispatched server-side end to end: resolve the admitted RunView,
// observe the live container, MINT a Docker handle from the observed identity
// (never a persisted ContainerID passthrough), claim the web-build effect with
// the canonical request digest, pass the T04 gate, execute through the T03
// normal face and record the terminal receipt idempotently.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// f08WebBuildNormalProvider is the working Docker engine fake: create returns
// the receipt the staged request implies, start observes a terminal exit-0
// process with complete transport. It records the handle identity it saw so
// the mint test can prove the exec ran against the OBSERVED DockerID.
type f08WebBuildNormalProvider struct {
	created, started, observed int
	capturedHandleID           string
}

func (p *f08WebBuildNormalProvider) CreateAttachedExec(_ context.Context, handle sandbox.RemoteSandboxHandle, request sandbox.DockerNormalExecRequest) (sandbox.DockerNormalExecReceipt, error) {
	p.created++
	p.capturedHandleID = handle.ID()
	sum := sha256.Sum256([]byte(request.Request.Stdin))
	return sandbox.DockerNormalExecReceipt{
		ContainerID: handle.ID(), ExecID: "exec-f08-build",
		StdinEnabled: request.StdinEnabled, StdinBytes: int64(len(request.Request.Stdin)),
		StdinSHA256: hex.EncodeToString(sum[:]), Timeout: request.Request.Timeout,
	}, nil
}

func (p *f08WebBuildNormalProvider) StartAttachedExecOnce(_ context.Context, receipt sandbox.DockerNormalExecReceipt, _ bool, _ []byte, sink sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error) {
	p.started++
	if err := sink.Seal(); err != nil {
		return sandbox.DockerNormalExecOutcome{}, err
	}
	code := 0
	return sandbox.DockerNormalExecOutcome{
		Receipt: receipt, Transport: sandbox.DockerNormalExecTransportComplete,
		Observation:   sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded, ExitCode: &code},
		StartEvidence: true,
	}, nil
}

func (p *f08WebBuildNormalProvider) ObserveAttachedExec(_ context.Context, receipt sandbox.DockerNormalExecReceipt, _ bool) (sandbox.DockerNormalExecObservation, error) {
	p.observed++
	code := 0
	return sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessSucceeded, ExitCode: &code}, nil
}

type f08WebBuildOutputless struct{}

func (f08WebBuildOutputless) CreateOutputlessExec(context.Context, sandbox.RemoteSandboxHandle, sandbox.DockerOutputlessExecRequest) (sandbox.DockerOutputlessExecReceipt, error) {
	return sandbox.DockerOutputlessExecReceipt{}, nil
}
func (f08WebBuildOutputless) StartOutputlessExec(context.Context, sandbox.DockerOutputlessExecReceipt, time.Duration) error {
	return nil
}
func (f08WebBuildOutputless) ObserveOutputlessExec(context.Context, sandbox.DockerOutputlessExecReceipt, bool) (sandbox.DockerOutputlessExecObservation, error) {
	return sandbox.DockerOutputlessExecObservation{}, nil
}

type f08WebBuildDispatchFixture struct {
	db          *gorm.DB
	task        craft.Task
	authority   *admittedRuntimeAuthorityFake
	provider    *admittedSplitRuntimeProviderFake
	store       *admittedRuntimeStoreFake
	coordinator *CraftRunViewRuntimeCoordinator
	normal      *f08WebBuildNormalProvider
	pin         CraftWebToolchainPin
	gate        *CraftWebBuildCommandGate
	receipts    *repository.CraftWebBuildReceiptRepository
	grantID     string
	dispatcher  *CraftWebBuildDispatcher
}

func newF08WebBuildDispatchFixture(t *testing.T) *f08WebBuildDispatchFixture {
	t.Helper()
	normal := &f08WebBuildNormalProvider{}
	return newF08WebBuildDispatchFixtureWithEngine(t, normal, normal)
}

// newF08WebBuildDispatchFixtureWithEngine lets a journey substitute the
// provider engine (e.g. a partial-transport variant); normal stays the
// counter probe shared with the substituted engine.
func newF08WebBuildDispatchFixtureWithEngine(t *testing.T, engine craftDockerNormalEngine, normal *f08WebBuildNormalProvider) *f08WebBuildDispatchFixture {
	t.Helper()
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	db := openCraftT20PolicyDB(t)
	task := admittedTask()

	now := time.Now().UTC()
	require.NoError(t, db.Exec("INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES (?, ?, 'f08 dispatch', ?, 'trpc')",
		task.Scope.SessionID, task.Scope.TenantID, task.Scope.UserID).Error)
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "build the region page", "model_id": "model-1",
		"agent_config":         json.RawMessage(`{}`),
		"craft_input_manifest": []craft.Input{},
		"craft_workspace_seed": service.CraftWorkspaceSeedSnapshot{WorkspaceID: task.WorkspaceID, State: craft.DraftHeadEmpty},
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs
		(tenant_id, run_id, session_id, owner_id, request_id, assistant_message_id, request_hash, snapshot, status, wait_reason, deadline, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'req-f08', 'am-f08', 'hash', ?, 'running', '', ?, ?, ?)`,
		task.Scope.TenantID, task.Fence.RunID, task.Scope.SessionID, task.Scope.UserID, string(snapshot), now.Add(time.Hour), now, now).Error)

	end := now.Add(2 * time.Hour)
	require.NoError(t, db.Create(&repocommercial.BudgetAccountRow{TenantID: task.Scope.TenantID, VerifiedMicro: 10000, Watermark: "w0", Version: 1, VerifiedUntil: end}).Error)
	require.NoError(t, db.Create(&repocommercial.BudgetLotRow{TenantID: task.Scope.TenantID, LotID: "lot-f08", RemainingMicro: 10000, IssuedAt: now}).Error)

	budget, err := service.NewCraftBudgetService(db, nil, service.CraftBudgetPolicy{
		GrantWindow: time.Hour, MaxCalls: 5, CallUpper: commercial.Credits(500), TaskLimit: commercial.Credits(5000),
	})
	require.NoError(t, err)
	grant, err := budget.Admit(context.Background(), task.Scope, task.Fence.RunID)
	require.NoError(t, err)

	authority := &admittedRuntimeAuthorityFake{view: admittedView(task), stateful: true, outcomes: map[craft.RunViewEffectKind]craft.RunViewEffectOutcome{}}
	provider := admittedProvider()
	store := &admittedRuntimeStoreFake{view: authority.view}
	coordinator, err := NewCraftRunViewAdmittedRuntimeCoordinator(store, authority, provider, "/srv/craft")
	require.NoError(t, err)

	audit := service.NewAuditLogService(repository.NewAuditLogRepository(db))
	faces, err := newCraftDockerExecCommandFaces(db, repository.NewCraftStore(db), budget, audit, engine, f08WebBuildOutputless{}, time.Second)
	require.NoError(t, err)

	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	pin.RuntimeDigest = "sha256:" + strings.Repeat("ab", 32)
	gate, err := NewCraftWebBuildCommandGate(faces.Policy, pin, craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	receipts := repository.NewCraftWebBuildReceiptRepository(db)
	dispatcher, err := NewCraftWebBuildDispatcher(coordinator, gate, faces.Normal, receipts, pin)
	require.NoError(t, err)
	return &f08WebBuildDispatchFixture{
		db: db, task: task, authority: authority, provider: provider, store: store, coordinator: coordinator,
		normal: normal, pin: pin, gate: gate, receipts: receipts, grantID: grant.ID, dispatcher: dispatcher,
	}
}

// TestCraftF08WebBuildDispatchRecordsVerifiedReceipt is the T-1 flip of the
// fail-closed not_run path: through the dispatcher the fixed build earns a
// verified server receipt (succeeded, exit 0, started), and a replay adopts
// the same receipt without touching the engine again.
func TestCraftF08WebBuildDispatchRecordsVerifiedReceipt(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()

	receipt, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err, "the fixed web build dispatch must earn a terminal receipt")

	require.Equal(t, "succeeded", receipt.ProcessState)
	require.NotNil(t, receipt.ExitCode)
	require.Zero(t, *receipt.ExitCode)
	require.True(t, receipt.Started)
	require.Equal(t, "docker", receipt.Provider)
	require.Equal(t, fixture.task.Fence.RunID, receipt.RunID)
	require.Equal(t, fixture.task.Scope.SessionID, receipt.TaskID)
	require.Equal(t, fixture.task.WorkspaceID, receipt.WorkspaceID)
	require.Equal(t, service.CraftWebBuildCallDelegationID, receipt.ActivityKey)
	require.Equal(t, fixture.pin.ToolchainDigest, receipt.ToolchainDigest)
	require.Equal(t, fixture.pin.RuntimeDigest, receipt.RuntimeDigest)
	require.NotEmpty(t, receipt.ExecID)
	require.NotEmpty(t, receipt.ContainerID)
	require.True(t, receipt.TransportComplete)

	// The exec ran against the OBSERVED Docker identity, not a persisted ID.
	require.Equal(t, fixture.normal.capturedHandleID, receipt.ContainerID)

	// The dispatch claimed the web-build effect bound to the request digest.
	claimed := false
	for _, claim := range fixture.authority.claims {
		if claim.Kind == craft.RunViewEffectWebBuild {
			claimed = true
			require.Len(t, claim.RequestDigest, 64)
		}
	}
	require.True(t, claimed, "the dispatch must claim the web-build RunView effect")

	// The stored receipt is readable through the repository contract.
	stored, err := fixture.receipts.Read(ctx, receipt.Key())
	require.NoError(t, err)
	require.Equal(t, receipt, stored)

	// Replay is idempotent: the same receipt comes back and the engine is
	// never touched again.
	createdBefore := fixture.normal.created
	replay, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)
	require.Equal(t, stored, replay)
	require.Equal(t, createdBefore, fixture.normal.created, "a recorded receipt replay must not re-create the exec")
}

// TestCraftF08WebBuildDispatchMintsOnlyFromObservedIdentity is the ruling's
// guard: a poisoned persisted RunView.Runtime.ContainerID must never become
// the minted handle — only the live observation mints.
func TestCraftF08WebBuildDispatchMintsOnlyFromObservedIdentity(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	const poisoned = "stale-passthrough-container"
	view := fixture.authority.view
	view.State = craft.RunViewStateBound
	view.Runtime = craft.RunViewRuntime{RuntimeID: "runtime-stale", ContainerID: poisoned, OpenCodeSessionID: "session-stale"}
	intentAt := time.Now().UTC()
	view.SessionCreateIntentAt = &intentAt
	fixture.authority.view = view
	fixture.store.view = view

	receipt, err := fixture.dispatcher.Dispatch(context.Background(), fixture.task, fixture.grantID)
	require.NoError(t, err)
	require.NotEqual(t, poisoned, receipt.ContainerID, "a persisted ContainerID must never mint the handle")
	require.Equal(t, fixture.normal.capturedHandleID, receipt.ContainerID)
	require.NotEqual(t, poisoned, fixture.normal.capturedHandleID)
	require.True(t, strings.HasPrefix(receipt.ContainerID, "docker-"), "the minted identity is the observed Docker ID")

	// The minter itself refuses observations without a live Docker identity.
	empty := CraftRunViewContainerObservation{State: "running"}
	_, err = mintCraftWebBuildHandle(empty)
	require.ErrorIs(t, err, craft.ErrConflict)
	stopped := CraftRunViewContainerObservation{DockerID: "docker-x", State: "exited"}
	_, err = mintCraftWebBuildHandle(stopped)
	require.ErrorIs(t, err, craft.ErrConflict)
}

// TestCraftF08WebBuildDispatchFailsClosedWithoutVerifiedContainer: a container
// that cannot be observed running refuses dispatch before any engine call and
// records no receipt; the constructor refuses an unassembled gate.
func TestCraftF08WebBuildDispatchFailsClosedWithoutVerifiedContainer(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	fixture.provider.observeStateErr = errors.New("engine gone before start")

	_, err := fixture.dispatcher.Dispatch(context.Background(), fixture.task, fixture.grantID)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Zero(t, fixture.normal.created, "an unresolved container must never reach the engine")

	var receipts int64
	require.NoError(t, fixture.db.Table("craft_web_build_receipts").Count(&receipts).Error)
	require.Zero(t, receipts)

	_, err = NewCraftWebBuildDispatcher(fixture.coordinator, nil, nil, fixture.receipts, fixture.pin)
	require.ErrorIs(t, err, craft.ErrInvalidInput, "an unassembled gate or face must fail constructor-closed")
}

// TestCraftF08WebBuildEvidenceFlipsOnServerReceipt is the T-2 flip: the same
// build log that stays not_run without a receipt establishes BuildRan once
// the dispatcher's immutable receipt supplies the trusted exit code.
func TestCraftF08WebBuildEvidenceFlipsOnServerReceipt(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	receipt, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)
	require.Equal(t, "succeeded", receipt.ProcessState)

	// A real build log from the shipped toolchain, matching the pins.
	workspace := t.TempDir()
	inputDir, outputDir := filepath.Join(workspace, "input"), filepath.Join(workspace, "output")
	craftWebTestContent(t, inputDir)
	code, log := runCraftWebBuild(t, craftWebToolchainAbsDir(t), inputDir, outputDir, fixture.pin.RuntimeDigest)
	require.Zero(t, code, "offline build must succeed:\n%s", log)
	rawLog, err := os.ReadFile(filepath.Join(outputDir, "build-log.json"))
	require.NoError(t, err)

	readReceipt := craftWebBuildReceiptExitCode(fixture.receipts, fixture.pin)
	inner := func(context.Context, craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{PreviewRan: true, PreviewPassed: true}
	}
	readLog := func(context.Context, craft.Task) ([]byte, error) { return rawLog, nil }

	merged := craftWebBuildEvidenceSource(inner, readLog, fixture.pin, readReceipt)(ctx, fixture.task)
	require.True(t, merged.BuildRan, "a matching log plus the server receipt must establish the build fact")
	require.Zero(t, merged.BuildExitCode)
	require.True(t, merged.PreviewRan, "preview verdicts still flow")

	// A run with NO receipt stays not_run — the fail-closed contract holds.
	foreignTask := fixture.task
	foreignTask.Fence.RunID = "run-without-receipt"
	unobserved := craftWebBuildEvidenceSource(inner, readLog, fixture.pin, readReceipt)(ctx, foreignTask)
	require.False(t, unobserved.BuildRan, "without a bound receipt the build fact stays unobserved")
	require.True(t, unobserved.PreviewRan)
}

// TestCraftF08WebBuildPromotionReceiptFence drives the promotion-side receipt
// re-read over the real repository: a bound succeeded receipt verifies only
// once the collector sealed its candidate manifest, an unsealed receipt
// refuses, a sealed mismatch refuses, and the seal latches exactly once.
func TestCraftF08WebBuildPromotionReceiptFence(t *testing.T) {
	fixture := newF08WebBuildDispatchFixture(t)
	ctx := context.Background()
	receipt, err := fixture.dispatcher.Dispatch(ctx, fixture.task, fixture.grantID)
	require.NoError(t, err)

	fence := craftWebBuildPromotionReceipts{receipts: fixture.receipts, pin: fixture.pin}

	// The dispatched receipt is UNSEALED until the collector lane seals it:
	// an unsealed receipt can never pass the promotion fence.
	err = fence.VerifyPromotionBuild(ctx, fixture.task.Scope, fixture.task.WorkspaceID, fixture.task.Fence.RunID, strings.Repeat("ff", 32))
	require.ErrorIs(t, err, craft.ErrConflict, "an unsealed receipt cannot confirm any candidate digest")
	err = fence.VerifyPromotionBuild(ctx, fixture.task.Scope, fixture.task.WorkspaceID, fixture.task.Fence.RunID, "")
	require.ErrorIs(t, err, craft.ErrConflict, "an unsealed receipt cannot promote")

	err = fence.VerifyPromotionBuild(ctx, fixture.task.Scope, fixture.task.WorkspaceID, "run-never-dispatched", strings.Repeat("ff", 32))
	require.ErrorIs(t, err, craft.ErrConflict, "a run without a bound receipt cannot promote")

	err = fence.VerifyPromotionBuild(ctx, fixture.task.Scope, "ws-foreign", fixture.task.Fence.RunID, strings.Repeat("ff", 32))
	require.ErrorIs(t, err, craft.ErrConflict, "a receipt bound to another workspace cannot verify")

	// The collector seals the candidate manifest digest into the bound
	// receipt; the seal flips OutputComplete truthfully for the complete
	// build, and the fence then binds build to candidate content.
	require.NoError(t, fence.SealCandidateManifest(ctx, fixture.task.Scope, fixture.task.WorkspaceID, fixture.task.Fence.RunID, strings.Repeat("aa", 32)))
	sealedReceipt, err := fixture.receipts.Read(ctx, receipt.Key())
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("aa", 32), sealedReceipt.CandidateManifestSHA256)
	require.True(t, sealedReceipt.OutputComplete, "a complete build's receipt flips truthful with the seal")
	require.Equal(t, "succeeded", sealedReceipt.ProcessState, "process facts stay frozen")

	require.NoError(t, fence.VerifyPromotionBuild(ctx, fixture.task.Scope, fixture.task.WorkspaceID, fixture.task.Fence.RunID, strings.Repeat("aa", 32)))
	err = fence.VerifyPromotionBuild(ctx, fixture.task.Scope, fixture.task.WorkspaceID, fixture.task.Fence.RunID, strings.Repeat("bb", 32))
	require.ErrorIs(t, err, craft.ErrConflict, "a sealed manifest digest that differs from the candidate's must refuse promotion")

	// The seal latches exactly once: a second, different digest means the
	// output mutated after the build and is refused.
	err = fence.SealCandidateManifest(ctx, fixture.task.Scope, fixture.task.WorkspaceID, fixture.task.Fence.RunID, strings.Repeat("cc", 32))
	require.ErrorIs(t, err, repository.ErrCraftWebBuildReceiptConflict)
}
