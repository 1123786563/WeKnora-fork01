package container

// F08 T-1: the web build dispatch orchestrator (design
// docs/plans/2026-10-03-craft-f08-dispatch-design.md §2.1). One server-owned
// dispatch of the fixed offline web build: resolve the admitted RunView,
// observe the live container, MINT the Docker handle from the observed
// identity only (a persisted RunView.Runtime.ContainerID is never a mint
// source — 2026-09-29 ruling), claim the web-build effect bound to the
// canonical request digest, pass the T04 command gate, execute through the
// T03-gated normal face and record the terminal receipt idempotently.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// craftWebBuildActivityKey is the fixed activity identity of the offline web
// build family. Distinct attempts stay separable through RequestSHA256; the
// binding facet is service.CraftWebBuildCallBinding (T-3).
const craftWebBuildActivityKey = service.CraftWebBuildCallDelegationID

const (
	craftWebBuildTimeoutMillis = int64(10 * time.Minute / time.Millisecond)
	// The durable output quota rides the request-validation ceiling: a lower
	// dispatcher quota would stage requests the repository contract calls
	// illegal, a higher one would be clipped anyway.
	craftWebBuildOutputLimit = repository.MaxCraftDockerOutputBytes
)

// craftWebBuildMintedHandle is the server-minted Docker handle. Its identity
// is exclusively the live-observed DockerID of the verified admission
// container; there is deliberately no constructor from persisted state.
type craftWebBuildMintedHandle struct{ id string }

func (h craftWebBuildMintedHandle) ID() string { return h.id }
func (h craftWebBuildMintedHandle) Provider() sandbox.RemoteProvider {
	return sandbox.SandboxTypeDocker
}
func (h craftWebBuildMintedHandle) Metadata() map[string]string { return nil }

// mintCraftWebBuildHandle mints a Docker handle from one live observation.
// A missing identity or a non-running state refuses the mint fail-closed.
func mintCraftWebBuildHandle(observed CraftRunViewContainerObservation) (sandbox.RemoteSandboxHandle, error) {
	if observed.DockerID == "" || observed.DockerID == "\x00" || observed.State != "running" {
		return nil, fmt.Errorf("%w: live container observation lacks a running Docker identity", craft.ErrConflict)
	}
	return craftWebBuildMintedHandle{id: observed.DockerID}, nil
}

type craftWebBuildNormalFace interface {
	Execute(context.Context, string, string, service.CraftCallBinding, sandbox.RemoteSandboxHandle, repository.CraftDockerNormalInputRequest) (service.CraftDockerNormalExecResult, error)
}

type craftWebBuildReceiptStore interface {
	RecordTerminal(context.Context, repository.CraftWebBuildReceipt) (repository.CraftWebBuildReceipt, error)
	Read(context.Context, repository.CraftWebBuildReceiptKey) (repository.CraftWebBuildReceipt, error)
	SealCandidateManifest(context.Context, repository.CraftWebBuildReceiptKey, string) (repository.CraftWebBuildReceipt, error)
}

// CraftWebBuildDispatcher composes the F08 production dispatch slice. It is
// assembled next to the command gate and the exec faces; every dependency is
// required (constructor fail-closed) and the binding identity comes only from
// service.CraftWebBuildCallBinding.
type CraftWebBuildDispatcher struct {
	runtime  *CraftRunViewRuntimeCoordinator
	gate     *CraftWebBuildCommandGate
	normal   craftWebBuildNormalFace
	receipts craftWebBuildReceiptStore
	pin      CraftWebToolchainPin
}

func NewCraftWebBuildDispatcher(
	runtime *CraftRunViewRuntimeCoordinator,
	gate *CraftWebBuildCommandGate,
	normal craftWebBuildNormalFace,
	receipts craftWebBuildReceiptStore,
	pin CraftWebToolchainPin,
) (*CraftWebBuildDispatcher, error) {
	if runtime == nil || gate == nil || normal == nil || receipts == nil {
		return nil, fmt.Errorf("%w: web build dispatcher requires the admitted runtime coordinator, command gate, normal exec face and receipt store", craft.ErrInvalidInput)
	}
	if strings.TrimSpace(pin.ToolchainDigest) == "" || strings.TrimSpace(pin.TemplateSHA256) == "" || strings.TrimSpace(pin.RuntimeDigest) == "" {
		return nil, fmt.Errorf("%w: web build dispatcher requires the loaded toolchain pin including the runtime digest", craft.ErrInvalidInput)
	}
	return &CraftWebBuildDispatcher{runtime: runtime, gate: gate, normal: normal, receipts: receipts, pin: pin}, nil
}

// craftWebBuildFixedRequest builds the T04 pinned build command with the
// durable Run identity. TaskID carries the Run's session-scoped task identity
// exactly like the T03/T04 gate fixtures (agent_runs.session_id).
func craftWebBuildFixedRequest(task craft.Task, pin CraftWebToolchainPin) (repository.CraftDockerNormalInputRequest, error) {
	if task.Scope.TenantID == 0 || strings.TrimSpace(task.Fence.RunID) == "" || strings.TrimSpace(task.Scope.SessionID) == "" ||
		strings.TrimSpace(task.WorkspaceID) == "" || strings.ContainsAny(task.WorkspaceID, "\x00\r\n") {
		return repository.CraftDockerNormalInputRequest{}, fmt.Errorf("%w: web build dispatch requires the complete durable Run identity", craft.ErrInvalidInput)
	}
	return repository.CraftDockerNormalInputRequest{
		TenantID: task.Scope.TenantID, TaskID: task.Scope.SessionID, RunID: task.Fence.RunID,
		ActivityKey: craftWebBuildActivityKey,
		Command: []string{
			"python3", craftWebBuildProgramPath,
			"--toolchain", craftWebBuildToolchainPath,
			"--input", "/workspace/material",
			"--output", "/workspace/output",
			"--runtime-digest", pin.RuntimeDigest,
		},
		WorkingDir:    "/workspace",
		TimeoutMillis: craftWebBuildTimeoutMillis,
		OutputLimit:   craftWebBuildOutputLimit,
		OutputPolicy:  "bounded-partial-v1",
	}, nil
}

// craftWebBuildRequestDigest is the canonical staged-request digest binding
// the web-build effect claim to the exact dispatch request (same JSON
// canonicalization the input repository stages).
func craftWebBuildRequestDigest(request repository.CraftDockerNormalInputRequest) (string, error) {
	return runViewCanonicalRequestDigest(struct {
		Domain  string                                   `json:"domain"`
		Request repository.CraftDockerNormalInputRequest `json:"request"`
	}{"weknora.craft.webbuild.dispatch.v1", request})
}

// Dispatch runs one fixed web build attempt for the admitted Task and returns
// its terminal receipt. A recorded receipt replays unchanged without touching
// the engine; a fresh dispatch is claim → gate → execute → record.
func (d *CraftWebBuildDispatcher) Dispatch(ctx context.Context, task craft.Task, grantID string) (repository.CraftWebBuildReceipt, error) {
	if d == nil {
		return repository.CraftWebBuildReceipt{}, fmt.Errorf("%w: web build dispatcher is not assembled", craft.ErrInvalidInput)
	}
	if err := ctx.Err(); err != nil {
		return repository.CraftWebBuildReceipt{}, err
	}
	request, err := craftWebBuildFixedRequest(task, d.pin)
	if err != nil {
		return repository.CraftWebBuildReceipt{}, err
	}
	requestSHA, err := craftWebBuildRequestDigest(request)
	if err != nil {
		return repository.CraftWebBuildReceipt{}, err
	}
	key := repository.CraftWebBuildReceiptKey{
		TenantID: task.Scope.TenantID, TaskID: task.Scope.SessionID, SessionID: task.Scope.SessionID,
		WorkspaceID: task.WorkspaceID, RunID: task.Fence.RunID,
		ActivityKey: craftWebBuildActivityKey, RequestSHA256: requestSHA,
	}
	// Idempotent replay per the receipt repository contract: a recorded
	// terminal receipt IS the answer; retries never resend.
	if prior, readErr := d.receipts.Read(ctx, key); readErr == nil {
		return prior, nil
	} else if !errors.Is(readErr, repository.ErrCraftWebBuildReceiptNotFound) {
		return repository.CraftWebBuildReceipt{}, readErr
	}

	view, observed, err := d.runtime.ResolveAdmittedVerifiedContainer(ctx, task)
	if err != nil {
		return repository.CraftWebBuildReceipt{}, err
	}
	handle, err := mintCraftWebBuildHandle(observed)
	if err != nil {
		return repository.CraftWebBuildReceipt{}, err
	}
	claim, maySend, err := d.runtime.beginAdmittedEffectWithDigest(ctx, task, view.Generation, craft.RunViewEffectWebBuild, requestSHA)
	if err != nil {
		return repository.CraftWebBuildReceipt{}, err
	}
	if err := d.gate.Review(ctx, request); err != nil {
		if maySend {
			_ = d.runtime.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateFailed, Receipt: "gate-refused"})
		}
		return repository.CraftWebBuildReceipt{}, err
	}
	result, execErr := d.normal.Execute(ctx, grantID, craftWebBuildActivityKey, service.CraftWebBuildCallBinding(), handle, request)
	if execErr != nil {
		logger.Warnf(ctx, "[CraftWebBuild] dispatch execute for run %s did not complete cleanly: %v", task.Fence.RunID, execErr)
	}
	if result.Receipt.ContainerID == "" {
		// Nothing was created: no terminal process fact exists to record, and
		// fabricating one would poison the immutable receipt row.
		if maySend {
			_ = d.runtime.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown})
		}
		if execErr == nil {
			execErr = fmt.Errorf("craft web build dispatch for run %s produced no receipt", task.Fence.RunID)
		}
		return repository.CraftWebBuildReceipt{}, execErr
	}
	recorded, recordErr := d.receipts.RecordTerminal(ctx, craftWebBuildTerminalReceipt(task, d.pin, request, requestSHA, view.Generation, result))
	if recordErr != nil {
		if maySend {
			_ = d.runtime.finishAdmittedEffect(ctx, claim, craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown})
		}
		return recorded, errors.Join(execErr, recordErr)
	}
	outcome := craft.RunViewEffectOutcome{State: craft.RunViewEffectStateUnknown, Receipt: recorded.ExecID}
	switch recorded.ProcessState {
	case "succeeded":
		outcome.State = craft.RunViewEffectStateSucceeded
	case "failed":
		outcome.State = craft.RunViewEffectStateFailed
	}
	if finishErr := d.runtime.finishAdmittedEffect(ctx, claim, outcome); finishErr != nil {
		return recorded, errors.Join(execErr, finishErr)
	}
	return recorded, execErr
}

// craftWebBuildTerminalReceipt maps the exec service result onto the immutable
// receipt contract. ProcessState is promoted to a terminal state only with
// start evidence and a matching exit code; everything else stays unknown.
// OutputComplete requires the collector-sealed candidate manifest (T-2), so a
// fresh record honestly stays false: it flips truthfully (complete only for a
// fully-transported build) when the collector lane seals the manifest digest.
func craftWebBuildTerminalReceipt(
	task craft.Task, pin CraftWebToolchainPin, request repository.CraftDockerNormalInputRequest,
	requestSHA, generation string, result service.CraftDockerNormalExecResult,
) repository.CraftWebBuildReceipt {
	commandSHA, _ := runViewCanonicalRequestDigest(request.Command)
	state := "unknown"
	if result.StartEvidence && result.Process.ExitCode != nil && *result.Process.ExitCode >= 0 && *result.Process.ExitCode <= 255 {
		if result.Process.State == sandbox.DockerNormalExecProcessSucceeded && *result.Process.ExitCode == 0 {
			state = "succeeded"
		}
		if result.Process.State == sandbox.DockerNormalExecProcessFailed && *result.Process.ExitCode != 0 {
			state = "failed"
		}
	}
	return repository.CraftWebBuildReceipt{
		TenantID: task.Scope.TenantID, TaskID: task.Scope.SessionID, SessionID: task.Scope.SessionID,
		WorkspaceID: task.WorkspaceID, RunID: task.Fence.RunID, ActivityKey: request.ActivityKey, RequestSHA256: requestSHA,
		CommandSHA256: commandSHA, RuntimeDigest: pin.RuntimeDigest, ToolchainDigest: pin.ToolchainDigest,
		TemplateVersion: pin.TemplateVersion, TemplateSHA256: pin.TemplateSHA256,
		TimeoutMillis: request.TimeoutMillis, OutputLimit: request.OutputLimit,
		Provider: string(sandbox.SandboxTypeDocker), ContainerID: result.Receipt.ContainerID, ExecID: result.Receipt.ExecID,
		ProcessState: state, ExitCode: result.Process.ExitCode, Started: result.StartEvidence,
		TransportComplete: result.Transport == sandbox.DockerNormalExecTransportComplete,
		OutputGeneration:  generation,
		ObservedAt:        time.Now().UTC(),
	}
}

// craftWebBuildReceiptExitCode reads the dispatcher's immutable receipt for
// the task's fixed build attempt and returns its server-observed exit code —
// only for a terminal receipt with start evidence; anything else reports the
// not-found sentinel so the evidence source stays fail-closed.
func craftWebBuildReceiptExitCode(receipts craftWebBuildReceiptStore, pin CraftWebToolchainPin) func(context.Context, craft.Task) (*int, error) {
	return func(ctx context.Context, task craft.Task) (*int, error) {
		request, err := craftWebBuildFixedRequest(task, pin)
		if err != nil {
			return nil, err
		}
		requestSHA, err := craftWebBuildRequestDigest(request)
		if err != nil {
			return nil, err
		}
		receipt, err := receipts.Read(ctx, repository.CraftWebBuildReceiptKey{
			TenantID: task.Scope.TenantID, TaskID: task.Scope.SessionID, SessionID: task.Scope.SessionID,
			WorkspaceID: task.WorkspaceID, RunID: task.Fence.RunID,
			ActivityKey: craftWebBuildActivityKey, RequestSHA256: requestSHA,
		})
		if err != nil {
			return nil, err
		}
		if !receipt.Started || receipt.ExitCode == nil ||
			(receipt.ProcessState != "succeeded" && receipt.ProcessState != "failed") {
			return nil, repository.ErrCraftWebBuildReceiptNotFound
		}
		return receipt.ExitCode, nil
	}
}

// craftWebBuildPromotionReceipts adapts the immutable receipt repository to
// the collector seal and the promotion fence (F08 T-2): the collector seals
// the candidate manifest digest it produced into the bound receipt, and the
// Run being promoted must carry that bound, terminal-succeeded, SEALED build
// receipt — an unsealed receipt binds build success but not build content and
// can never pass the fence.
type craftWebBuildPromotionReceipts struct {
	receipts craftWebBuildReceiptStore
	pin      CraftWebToolchainPin
}

func (a craftWebBuildPromotionReceipts) receiptKey(scope craft.Scope, workspaceID, runID string) (repository.CraftWebBuildReceiptKey, error) {
	task := craft.Task{Scope: scope, WorkspaceID: workspaceID}
	task.Fence.RunID = runID
	request, err := craftWebBuildFixedRequest(task, a.pin)
	if err != nil {
		return repository.CraftWebBuildReceiptKey{}, err
	}
	requestSHA, err := craftWebBuildRequestDigest(request)
	if err != nil {
		return repository.CraftWebBuildReceiptKey{}, err
	}
	return repository.CraftWebBuildReceiptKey{
		TenantID: scope.TenantID, TaskID: scope.SessionID, SessionID: scope.SessionID,
		WorkspaceID: workspaceID, RunID: runID,
		ActivityKey: craftWebBuildActivityKey, RequestSHA256: requestSHA,
	}, nil
}

// SealCandidateManifest seals the server-computed candidate manifest digest
// into the run's bound build receipt (fail-closed: no dispatched receipt, no
// seal — the promotion fence keeps refusing).
func (a craftWebBuildPromotionReceipts) SealCandidateManifest(ctx context.Context, scope craft.Scope, workspaceID, runID, manifestDigest string) error {
	key, err := a.receiptKey(scope, workspaceID, runID)
	if err != nil {
		return err
	}
	_, err = a.receipts.SealCandidateManifest(ctx, key, manifestDigest)
	return err
}

func (a craftWebBuildPromotionReceipts) VerifyPromotionBuild(ctx context.Context, scope craft.Scope, workspaceID, runID, manifestDigest string) error {
	key, err := a.receiptKey(scope, workspaceID, runID)
	if err != nil {
		return err
	}
	receipt, err := a.receipts.Read(ctx, key)
	if errors.Is(err, repository.ErrCraftWebBuildReceiptNotFound) {
		return fmt.Errorf("%w: run %s has no bound web build receipt; promotion requires the F08 dispatch receipt", craft.ErrConflict, runID)
	}
	if err != nil {
		return err
	}
	if receipt.ProcessState != "succeeded" || receipt.ExitCode == nil || *receipt.ExitCode != 0 {
		return fmt.Errorf("%w: run %s web build receipt is not a verified success (state %s)", craft.ErrConflict, runID, receipt.ProcessState)
	}
	if receipt.CandidateManifestSHA256 == "" {
		return fmt.Errorf("%w: run %s web build receipt seals no candidate manifest; the collector must seal the built output before promotion", craft.ErrConflict, runID)
	}
	if receipt.CandidateManifestSHA256 != manifestDigest {
		return fmt.Errorf("%w: run %s build receipt seals manifest %s, not the candidate's %s", craft.ErrConflict, runID, receipt.CandidateManifestSHA256, manifestDigest)
	}
	return nil
}

// newCraftWebBuildProductionDispatcher assembles the F08 production dispatch
// lane: the budget-backed send coordinator, the durable output projection
// and the T03-gated normal face over the RunView Docker engine's Exec
// surface — the same engine the coordinator's live observations come from.
// The returned trigger admits the Run's budget grant (idempotent per Run)
// and dispatches; both failures are logged, not fatal: a dispatch that never
// earned a receipt leaves the promotion fence refusing fail-closed.
func newCraftWebBuildProductionDispatcher(
	db *gorm.DB,
	assembly *CraftRunViewProductionAssembly,
	policy *service.CraftDelegateExecutionPolicy,
	gate *CraftWebBuildCommandGate,
	pin CraftWebToolchainPin,
) (func(context.Context, craft.Task), error) {
	if db == nil {
		return nil, fmt.Errorf("%w: web build dispatch wiring requires the database", craft.ErrInvalidInput)
	}
	engine, ok := assembly.Engine.(sandbox.DockerNormalExecEngine)
	if !ok || engine == nil {
		return nil, fmt.Errorf("%w: the RunView engine exposes no Exec surface", craft.ErrInvalidInput)
	}
	normalExec, err := sandbox.NewDockerNormalExecClient(engine, sandbox.DockerNormalExecConfig{})
	if err != nil {
		return nil, err
	}
	budget, err := service.NewCraftBudgetService(db, nil, defaultCraftBudgetPolicy())
	if err != nil {
		return nil, err
	}
	coordinator, err := service.NewCraftDockerSendCoordinator(budget, repository.NewCraftDockerSendClaimRepository(db))
	if err != nil {
		return nil, err
	}
	output, err := service.NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(db, craftWebBuildOutputLimit))
	if err != nil {
		return nil, err
	}
	normal, err := service.NewCraftDockerNormalExecService(coordinator,
		repository.NewCraftDockerNormalInputRepository(db), normalExec, output)
	if err != nil {
		return nil, err
	}
	normal.WithExecutionPolicy(policy)
	dispatcher, err := NewCraftWebBuildDispatcher(assembly.RuntimeCoordinator, gate, normal, repository.NewCraftWebBuildReceiptRepository(db), pin)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, task craft.Task) {
		grant, err := budget.Admit(ctx, task.Scope, task.Fence.RunID)
		if err != nil {
			logger.Warnf(ctx, "[CraftWebBuild] dispatch for run %s could not admit its budget grant: %v", task.Fence.RunID, err)
			return
		}
		if _, err := dispatcher.Dispatch(ctx, task, grant.ID); err != nil {
			logger.Warnf(ctx, "[CraftWebBuild] dispatch for run %s did not complete: %v", task.Fence.RunID, err)
		}
	}, nil
}

// wireCraftWebBuildDispatcher installs the F08 production dispatch trigger on
// the local craft runtime (owner sign-off 2026-10-05, option A sentinel
// binding). Without the local runtime dial, the pinned web toolchain or the
// complete RunView production assembly the wiring logs once and stays off —
// the promotion fence then keeps refusing fail-closed, exactly the recorded
// pre-wiring posture.
func wireCraftWebBuildDispatcher(
	executor craft.Executor,
	assembly *CraftRunViewProductionAssembly,
	db *gorm.DB,
	store craft.Store,
	audit interfaces.AuditLogService,
) error {
	runtime, ok := executor.(*localCraftRuntime)
	if !ok {
		logger.Infof(context.Background(), "[CraftWebBuild] dispatch wiring skipped: the local craft runtime dial is not assembled")
		return nil
	}
	gate := RegisteredCraftWebBuildCommandGate()
	if gate == nil {
		logger.Infof(context.Background(), "[CraftWebBuild] dispatch wiring skipped: no pinned web toolchain (%s)", craftWebToolchainDirEnv)
		return nil
	}
	if assembly == nil || assembly.Unavailable != "" || assembly.RuntimeCoordinator == nil {
		reason := "RunView production assembly is unavailable"
		if assembly != nil && assembly.Unavailable != "" {
			reason = assembly.Unavailable
		}
		logger.Infof(context.Background(), "[CraftWebBuild] dispatch wiring skipped: %s (web builds stay undispatched)", reason)
		return nil
	}
	webToolchainDir := strings.TrimSpace(os.Getenv(craftWebToolchainDirEnv))
	pin, err := LoadCraftWebToolchainPin(webToolchainDir)
	if err != nil {
		return fmt.Errorf("craft web build dispatch wiring: toolchain pin %s: %w", webToolchainDir, err)
	}
	pin.RuntimeDigest = craftRuntimeDigestFromEnv()
	delegate := service.NewCraftDelegateService(store, nil).WithAuditLog(audit)
	policy, err := service.NewCraftDelegateExecutionPolicy(delegate, db, craftDockerWorkspaceRoot)
	if err != nil {
		return fmt.Errorf("craft web build dispatch wiring: %w", err)
	}
	policy.WithAuditLog(audit)
	dispatch, err := newCraftWebBuildProductionDispatcher(db, assembly, policy, gate, pin)
	if err != nil {
		return fmt.Errorf("craft web build dispatch wiring: %w", err)
	}
	runtime.webBuildDispatch = dispatch
	logger.Infof(context.Background(), "[CraftWebBuild] production dispatch wired: successful web-kind delegations dispatch the fixed offline build")
	return nil
}
