package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

const craftDockerNormalObserveTimeout = 2 * time.Second

type craftDockerNormalProvider interface {
	CreateAttachedExec(context.Context, sandbox.RemoteSandboxHandle, sandbox.DockerNormalExecRequest) (sandbox.DockerNormalExecReceipt, error)
	StartAttachedExecOnce(context.Context, sandbox.DockerNormalExecReceipt, bool, []byte, sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error)
	ObserveAttachedExec(context.Context, sandbox.DockerNormalExecReceipt, bool) (sandbox.DockerNormalExecObservation, error)
}

type craftDockerNormalInputReader interface {
	Read(context.Context, repository.CraftChargeStartKey) (repository.CraftDockerStagedNormalInput, error)
}

// CraftDockerNormalExecResult joins process evidence with durable output state.
// Cursor is the last committed output sequence and can be supplied to ReadAfter.
type CraftDockerNormalExecResult struct {
	Receipt        repository.CraftDockerNormalReceipt
	Process        sandbox.DockerNormalExecObservation
	Transport      sandbox.DockerNormalExecTransportState
	StartEvidence  bool
	Output         repository.CraftDockerOutputSnapshot
	OutputComplete bool
	Cursor         int64
}

// CraftDockerNormalExecService composes the staged normal request, immutable
// Docker receipt, durable send claim, one-use permission and durable output
// projection. It is deliberately not wired into ordinary sandbox routing.
type CraftDockerNormalExecService struct {
	coordinator *CraftDockerSendCoordinator
	inputs      craftDockerNormalInputReader
	provider    craftDockerNormalProvider
	output      *CraftDockerOutputService
	// policy is the T03 uploaded-material execution gate (#122). When the
	// central assembly provides it, every Execute is screened before any
	// create, bind, claim or send; a denial surfaces the member-visible
	// refusal instead of sending. nil keeps the unwired legacy behavior.
	policy  CraftExecutionPolicyGate
	mu      sync.Mutex
	started map[string]bool
}

// WithExecutionPolicy attaches the T03 uploaded-material execution gate
// (#122) to this command face. It must be set by the central assembly
// before the service is exposed; the gate runs first in Execute. The
// attach is logged so a deployment that forgot to wire the security gate
// is observable in its logs.
func (s *CraftDockerNormalExecService) WithExecutionPolicy(policy CraftExecutionPolicyGate) *CraftDockerNormalExecService {
	if s == nil {
		return s
	}
	if policy == nil {
		logger.Warnf(context.Background(), "[CraftDockerNormalExec] T03 execution gate NOT attached: the uploaded-material execution policy is nil (unwired assembly)")
		return s
	}
	s.policy = policy
	logger.Infof(context.Background(), "[CraftDockerNormalExec] T03 execution gate attached")
	return s
}

// limitedNormalExecSink enforces the request's durable output quota even when
// the provider itself is configured with a larger global limit.
type limitedNormalExecSink struct {
	inner     sandbox.DockerNormalExecChunkSink
	mu        sync.Mutex
	remaining int64
	truncated bool
}

func (s *limitedNormalExecSink) Append(ctx context.Context, receipt sandbox.DockerNormalExecReceipt, stream string, chunk []byte) error {
	if s == nil || s.inner == nil || len(chunk) == 0 {
		return repository.ErrCraftDockerOutputInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.remaining <= 0 {
		s.truncated = true
		return repository.ErrCraftDockerOutputQuota
	}
	if int64(len(chunk)) > s.remaining {
		prefix := append([]byte(nil), chunk[:int(s.remaining)]...)
		if err := s.inner.Append(ctx, receipt, stream, prefix); err != nil {
			return err
		}
		s.remaining = 0
		s.truncated = true
		return repository.ErrCraftDockerOutputQuota
	}
	if err := s.inner.Append(ctx, receipt, stream, chunk); err != nil {
		return err
	}
	s.remaining -= int64(len(chunk))
	return nil
}

func (s *limitedNormalExecSink) Seal() error {
	if s == nil || s.inner == nil {
		return repository.ErrCraftDockerOutputUnavailable
	}
	return s.inner.Seal()
}

func (s *limitedNormalExecSink) wasTruncated() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.truncated
}

func NewCraftDockerNormalExecService(coordinator *CraftDockerSendCoordinator, inputs craftDockerNormalInputReader, provider craftDockerNormalProvider, output *CraftDockerOutputService) (*CraftDockerNormalExecService, error) {
	if coordinator == nil || inputs == nil || provider == nil || output == nil {
		return nil, errors.New("normal Docker exec requires coordinator, staged input, provider and output store")
	}
	return &CraftDockerNormalExecService{coordinator: coordinator, inputs: inputs, provider: provider, output: output, started: make(map[string]bool)}, nil
}

// Execute performs one normal Docker exec. A preclaim replay either resumes
// the exact bound receipt or repeats only inert create for the exact staged
// request. Once claimed, every replay is observation-only and can never attach.
func (s *CraftDockerNormalExecService) Execute(ctx context.Context, grantID, activityID string, binding CraftCallBinding, handle sandbox.RemoteSandboxHandle, request repository.CraftDockerNormalInputRequest) (CraftDockerNormalExecResult, error) {
	if s == nil || s.coordinator == nil || s.inputs == nil || s.provider == nil || s.output == nil || handle == nil || handle.Provider() != sandbox.SandboxTypeDocker || handle.ID() == "" {
		return CraftDockerNormalExecResult{}, craft.ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return CraftDockerNormalExecResult{}, err
	}
	// T03 (#122): uploaded code stays data. Screen the staged command
	// against the Run's admitted input manifest before anything is created,
	// bound, claimed or sent; a denial returns the member-visible refusal.
	if s.policy == nil {
		// Fail-closed: an unwired T03 gate must never fall through to the
		// legacy allow-everything behavior — the upload-execute invariant
		// (#122) is the whole point of this command face.
		return CraftDockerNormalExecResult{}, fmt.Errorf("%w: T03 execution policy gate is not assembled", craft.ErrForbidden)
	}
	{
		if err := s.policy.ReviewNormalExec(ctx, request); err != nil {
			return CraftDockerNormalExecResult{}, err
		}
	}

	operation, staged, receipt, err := s.prepareOrRecover(ctx, grantID, activityID, binding, handle, request)
	if err != nil {
		return CraftDockerNormalExecResult{}, err
	}
	if operation == nil {
		return s.observeClaimed(ctx, grantID, activityID, request, receipt)
	}
	if staged.Receipt == nil {
		providerReceipt, createErr := s.provider.CreateAttachedExec(ctx, handle, craftDockerNormalProviderRequest(staged.Request))
		if createErr != nil {
			return CraftDockerNormalExecResult{}, createErr
		}
		receipt = repository.CraftDockerNormalReceipt{
			DockerExecReceipt: repository.DockerExecReceipt{Provider: string(sandbox.SandboxTypeDocker), ContainerID: providerReceipt.ContainerID, ExecID: providerReceipt.ExecID},
			StdinEnabled:      staged.StdinEnabled, StdinByteCount: staged.StdinByteCount, StdinSHA256: staged.StdinSHA256, TimeoutMillis: staged.Request.TimeoutMillis,
		}
		if !normalProviderReceiptMatches(providerReceipt, receipt) || providerReceipt.ContainerID != handle.ID() {
			return CraftDockerNormalExecResult{}, fmt.Errorf("%w: provider create receipt does not match the staged request", craft.ErrConflict)
		}
		if err := operation.Bind(ctx, receipt); err != nil {
			return CraftDockerNormalExecResult{}, err
		}
	} else {
		receipt = *staged.Receipt
	}

	claim, err := operation.Claim(ctx, receipt)
	if err != nil {
		return CraftDockerNormalExecResult{}, err
	}
	if claim.Permission == nil {
		return s.observeClaimed(ctx, grantID, activityID, request, receipt)
	}
	if err := ctx.Err(); err != nil {
		return s.unknownResult(ctx, activityID, receipt, err)
	}

	outputScope := normalOutputScope(request, receipt)
	providerReceipt := normalProviderReceipt(receipt)
	sink, err := s.output.OpenSink(ctx, outputScope, providerReceipt)
	if err != nil {
		return s.unknownResult(ctx, activityID, receipt, err)
	}
	// Opening the durable writer is part of the claimed protocol. Cancellation
	// at this point retains the claim and seals an empty/partial stream.
	if err := ctx.Err(); err != nil {
		sealErr := sink.Seal()
		return s.unknownResult(ctx, activityID, receipt, errors.Join(err, sealErr))
	}
	permissionReceipt, ok := claim.Permission.Consume()
	if !ok || permissionReceipt != receipt.DockerExecReceipt {
		sealErr := sink.Seal()
		return s.unknownResult(ctx, activityID, receipt, errors.Join(craft.ErrConflict, sealErr))
	}

	boundedSink := &limitedNormalExecSink{inner: sink, remaining: staged.Request.OutputLimit}
	outcome, startErr := s.provider.StartAttachedExecOnce(ctx, normalProviderReceipt(receipt), staged.StdinEnabled, append([]byte(nil), staged.Request.Stdin...), boundedSink)
	sealErr := sink.Seal()
	key := normalStartEvidenceKey(receipt)
	if outcome.StartEvidence {
		s.mu.Lock()
		// Abandoned receipts (start error + no replay, container deleted)
		// would otherwise retain their keys forever: past the cap the table
		// resets wholesale — dropped keys degrade only cross-replay
		// attribution to Unknown, the same semantics as a process restart.
		if len(s.started) >= 4096 {
			logger.Warnf(ctx, "[CraftDockerNormalExec] start-evidence table hit its cap; resetting (abandoned keys degrade to Unknown attribution)")
			s.started = make(map[string]bool)
		}
		s.started[key] = true
		s.mu.Unlock()
	}
	// A terminal process observation closes this receipt's start-evidence
	// window: later observations of an already-terminal receipt no longer
	// need the flag, and the long-lived service must not retain one key per
	// exec forever.
	if state := outcome.Observation.State; state == sandbox.DockerNormalExecProcessSucceeded || state == sandbox.DockerNormalExecProcessFailed {
		s.mu.Lock()
		delete(s.started, key)
		s.mu.Unlock()
	}
	result := CraftDockerNormalExecResult{
		Receipt: receipt, Process: outcome.Observation, Transport: outcome.Transport,
		StartEvidence: outcome.StartEvidence,
	}
	result.Output, result.Cursor, _ = s.readOutput(context.WithoutCancel(ctx), outputScope)
	if boundedSink.wasTruncated() {
		result.Output.Truncated = true
	}
	result.OutputComplete = startErr == nil && result.StartEvidence && result.Transport == sandbox.DockerNormalExecTransportComplete &&
		(result.Process.State == sandbox.DockerNormalExecProcessSucceeded || result.Process.State == sandbox.DockerNormalExecProcessFailed) &&
		result.Output.Sealed && !result.Output.Truncated && !result.Output.Unavailable && sealErr == nil
	if result.OutputComplete {
		// The output repository intentionally marks every sealed stream partial;
		// only this service can join the seal with provider transport/terminal proof.
		result.Output.Partial = false
	} else {
		result.Output.Partial = true
	}
	if startErr != nil || sealErr != nil {
		if outcome.Transport == sandbox.DockerNormalExecTransportComplete {
			result.Transport = sandbox.DockerNormalExecTransportPartial
		}
		return result, normalUnknownError(activityID, receipt, errors.Join(startErr, sealErr))
	}
	if !result.OutputComplete {
		return result, normalUnknownError(activityID, receipt, fmt.Errorf("normal Docker exec lacks complete start, terminal, transport, or sealed-output evidence"))
	}
	return result, nil
}

// ReadAfter verifies the exact staged request and claimed receipt before
// exposing cursor output. It never creates a writer or talks to Docker.
func (s *CraftDockerNormalExecService) ReadAfter(ctx context.Context, grantID, activityID string, request repository.CraftDockerNormalInputRequest, cursor int64, limit int) ([]repository.CraftDockerOutputChunk, repository.CraftDockerOutputSnapshot, error) {
	if s == nil || s.coordinator == nil || s.inputs == nil || s.output == nil || cursor < 0 || limit <= 0 {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, craft.ErrInvalidInput
	}
	observation, err := s.coordinator.Observe(ctx, grantID, activityID)
	if err != nil {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, err
	}
	if !observation.Claimed || observation.Receipt == nil {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, craft.ErrConflict
	}
	staged, err := s.readExactStage(ctx, request)
	if err != nil {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, err
	}
	if staged.Receipt == nil || staged.Receipt.ContainerID != observation.Receipt.ContainerID || staged.Receipt.ExecID != observation.Receipt.ExecID {
		return nil, repository.CraftDockerOutputSnapshot{Unavailable: true}, craft.ErrConflict
	}
	return s.output.ReadAfter(ctx, normalOutputScope(request, *staged.Receipt), cursor, limit)
}

func (s *CraftDockerNormalExecService) prepareOrRecover(ctx context.Context, grantID, activityID string, binding CraftCallBinding, handle sandbox.RemoteSandboxHandle, request repository.CraftDockerNormalInputRequest) (*DockerNormalSendOperation, repository.CraftDockerStagedNormalInput, repository.CraftDockerNormalReceipt, error) {
	observation, observeErr := s.coordinator.Observe(ctx, grantID, activityID)
	if observeErr == nil && observation.Claimed {
		if observation.Receipt == nil {
			return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, craft.ErrConflict
		}
		staged, err := s.readExactStage(ctx, request)
		if err != nil {
			return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, err
		}
		if staged.Receipt == nil || staged.Receipt.ContainerID != observation.Receipt.ContainerID || staged.Receipt.ExecID != observation.Receipt.ExecID || handle.ID() != observation.Receipt.ContainerID {
			return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, craft.ErrConflict
		}
		return nil, staged, *staged.Receipt, nil
	}
	if observeErr == nil && observation.Receipt != nil {
		operation, staged, err := s.coordinator.ResumeBoundNormal(ctx, grantID, activityID, request)
		if err != nil {
			return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, err
		}
		if staged.Request.TaskID != request.TaskID || staged.Request.TenantID != request.TenantID || handle.ID() != staged.Receipt.ContainerID {
			return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, craft.ErrConflict
		}
		return operation, staged, *staged.Receipt, nil
	}
	if observeErr != nil && !errors.Is(observeErr, craft.ErrNotFound) {
		return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, observeErr
	}
	operation, staged, err := s.coordinator.PrepareNormal(ctx, grantID, activityID, binding, request)
	if err != nil {
		// A receipt may have been bound between Observe and PrepareNormal. It is
		// safe to recover only through the exact preclaim receipt path.
		if errors.Is(err, craft.ErrConflict) {
			resumed, resumedStage, resumeErr := s.coordinator.ResumeBoundNormal(ctx, grantID, activityID, request)
			if resumeErr == nil {
				if handle.ID() != resumedStage.Receipt.ContainerID {
					return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, craft.ErrConflict
				}
				return resumed, resumedStage, *resumedStage.Receipt, nil
			}
		}
		return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, err
	}
	if staged.Request.TaskID != request.TaskID || staged.Request.TenantID != request.TenantID || staged.Request.RunID != request.RunID || handle.ID() == "" {
		return nil, repository.CraftDockerStagedNormalInput{}, repository.CraftDockerNormalReceipt{}, craft.ErrConflict
	}
	return operation, staged, repository.CraftDockerNormalReceipt{}, nil
}

func (s *CraftDockerNormalExecService) observeClaimed(ctx context.Context, grantID, activityID string, request repository.CraftDockerNormalInputRequest, receipt repository.CraftDockerNormalReceipt) (CraftDockerNormalExecResult, error) {
	if err := ctx.Err(); err != nil {
		return s.unknownResult(ctx, activityID, receipt, err)
	}
	startEvidence := false
	key := normalStartEvidenceKey(receipt)
	s.mu.Lock()
	startEvidence = s.started[key]
	s.mu.Unlock()
	observeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftDockerNormalObserveTimeout)
	process, observeErr := s.provider.ObserveAttachedExec(observeCtx, normalProviderReceipt(receipt), startEvidence)
	cancel()
	// Claimed-never-started reconciliation: the durable claim was consumed
	// but no start evidence exists and the provider reports the exec NOT
	// running — the process crashed between claim and start. Without this
	// convergence the receipt would stay Unknown forever (no replay can
	// re-send a consumed claim). Converge to a FAILED terminal with an
	// empty sealed output row so replays read a definitive state.
	if observeErr == nil && !startEvidence && process.State != sandbox.DockerNormalExecProcessRunning && process.State != sandbox.DockerNormalExecProcessUnknown {
		// The provider observed a DEFINITIVE not-running state with no
		// start evidence: the exec never started (claim consumed, process
		// crashed before Start). Report the definitive failed observation —
		// replays leave the Unknown limbo instead of parking forever.
		output, cursor, _ := s.readOutput(context.WithoutCancel(ctx), normalOutputScope(request, receipt))
		output.Unavailable = true // no output row can exist for a never-started exec
		result := CraftDockerNormalExecResult{Receipt: receipt, Process: process, Transport: sandbox.DockerNormalExecTransportComplete, StartEvidence: false, Output: output, Cursor: cursor}
		return result, nil
	}
	if process.State == sandbox.DockerNormalExecProcessSucceeded || process.State == sandbox.DockerNormalExecProcessFailed {
		// Terminal observation: drop the start-evidence key (same retention
		// rule as the start path) so recovered receipts stop accumulating.
		s.mu.Lock()
		delete(s.started, key)
		s.mu.Unlock()
	}
	result := CraftDockerNormalExecResult{Receipt: receipt, Process: process, Transport: sandbox.DockerNormalExecTransportUnavailable, StartEvidence: startEvidence}
	result.Output, result.Cursor, _ = s.readOutput(context.WithoutCancel(ctx), normalOutputScope(request, receipt))
	if result.Output.Sealed {
		result.Output.Partial = true // Recovery cannot reconstruct complete transport evidence.
	}
	return result, normalUnknownError(activityID, receipt, observeErr)
}

func (s *CraftDockerNormalExecService) readExactStage(ctx context.Context, request repository.CraftDockerNormalInputRequest) (repository.CraftDockerStagedNormalInput, error) {
	key := repository.CraftChargeStartKey{TenantID: request.TenantID, RunID: request.RunID, ActivityKey: request.ActivityKey}
	staged, err := s.inputs.Read(ctx, key)
	if err != nil {
		return repository.CraftDockerStagedNormalInput{}, err
	}
	if !reflect.DeepEqual(staged.Request, request) {
		return repository.CraftDockerStagedNormalInput{}, repository.ErrCraftDockerNormalInputConflict
	}
	return staged, nil
}

func (s *CraftDockerNormalExecService) unknownResult(ctx context.Context, activityID string, receipt repository.CraftDockerNormalReceipt, cause error) (CraftDockerNormalExecResult, error) {
	result := CraftDockerNormalExecResult{Receipt: receipt, Process: sandbox.DockerNormalExecObservation{State: sandbox.DockerNormalExecProcessUnknown}, Transport: sandbox.DockerNormalExecTransportUnavailable, Output: repository.CraftDockerOutputSnapshot{Unavailable: true, Partial: true}}
	return result, normalUnknownError(activityID, receipt, cause)
}

func (s *CraftDockerNormalExecService) readOutput(ctx context.Context, scope repository.CraftDockerOutputScope) (repository.CraftDockerOutputSnapshot, int64, error) {
	readCtx, cancel := context.WithTimeout(ctx, craftDockerNormalObserveTimeout)
	defer cancel()
	_, snapshot, err := s.output.ReadAfter(readCtx, scope, 0, 1)
	if err != nil {
		// A missing/unreadable operation row must NOT project as an open,
		// growing empty stream: mark it unavailable like the sink/service
		// ReadAfter wrappers in the same contract family.
		snapshot.Unavailable = true
	}
	return snapshot, snapshot.NextSequence, err
}

func craftDockerNormalProviderRequest(request repository.CraftDockerNormalInputRequest) sandbox.DockerNormalExecRequest {
	remote := sandbox.RemoteExecRequest{Command: request.Command[0], Args: append([]string(nil), request.Command[1:]...),
		Stdin: string(request.Stdin), Env: cloneNormalExecEnvironment(request.Environment), WorkDir: request.WorkingDir, User: request.User,
		Timeout: time.Duration(request.TimeoutMillis) * time.Millisecond}
	return sandbox.DockerNormalExecRequest{Request: remote, StdinEnabled: request.StdinEnabled}
}

func cloneNormalExecEnvironment(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func normalProviderReceiptMatches(provider sandbox.DockerNormalExecReceipt, stored repository.CraftDockerNormalReceipt) bool {
	return provider.ContainerID == stored.ContainerID && provider.ExecID == stored.ExecID && provider.StdinEnabled == stored.StdinEnabled &&
		provider.StdinBytes == stored.StdinByteCount && provider.StdinSHA256 == stored.StdinSHA256 && provider.Timeout == time.Duration(stored.TimeoutMillis)*time.Millisecond
}

func normalProviderReceipt(receipt repository.CraftDockerNormalReceipt) sandbox.DockerNormalExecReceipt {
	return sandbox.DockerNormalExecReceipt{ContainerID: receipt.ContainerID, ExecID: receipt.ExecID, StdinEnabled: receipt.StdinEnabled,
		StdinBytes: receipt.StdinByteCount, StdinSHA256: receipt.StdinSHA256, Timeout: time.Duration(receipt.TimeoutMillis) * time.Millisecond}
}

func normalOutputScope(request repository.CraftDockerNormalInputRequest, receipt repository.CraftDockerNormalReceipt) repository.CraftDockerOutputScope {
	return repository.CraftDockerOutputScope{TenantID: request.TenantID, TaskID: request.TaskID, RunID: request.RunID,
		ActivityKey: request.ActivityKey, ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}
}

func normalStartEvidenceKey(receipt repository.CraftDockerNormalReceipt) string {
	return receipt.Provider + "\x00" + receipt.ContainerID + "\x00" + receipt.ExecID
}

func normalUnknownError(activityID string, receipt repository.CraftDockerNormalReceipt, cause error) error {
	return &sandbox.RemoteOperationError{State: sandbox.RemoteOperationUnknown,
		Ref: sandbox.RemoteOperationRef{Provider: sandbox.SandboxTypeDocker, ID: receipt.ExecID, OperationKey: activityID, SandboxID: receipt.ContainerID}, Err: cause}
}
