package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

// CraftDockerOutputlessRequest names the exceptional output-discard behavior
// at the application boundary. It is not ordinary sandbox Exec.
type CraftDockerOutputlessRequest struct {
	Exec          sandbox.RemoteExecRequest
	DiscardOutput bool
}

// CraftDockerOutputlessResult states explicitly that output was unavailable.
// DurationSource names the only authority behind Duration: "docker_exec_events"
// (daemon exec_start/exec_die pair joined by the exact exec ID) or
// "unavailable" when no authoritative pair exists.
type CraftDockerOutputlessResult struct {
	Receipt           repository.DockerExecReceipt
	Accepted          bool
	State             sandbox.DockerOutputlessExecState
	ExitCode          *int
	Duration          time.Duration
	DurationAvailable bool
	DurationSource    string
	StdoutAvailable   bool
	StderrAvailable   bool
}

// CraftDockerRestrictedExec composes durable S2 send authorization with the
// private detached Docker operation. It is intentionally not wired into
// ordinary command routing.
type CraftDockerRestrictedExec struct {
	coordinator *CraftDockerSendCoordinator
	client      sandbox.DockerOutputlessExecClient
	events      sandbox.DockerExecEventPairObserver
	// policy is the T03 uploaded-material execution gate (#122). When the
	// central assembly provides it, Start is screened before the durable
	// send is prepared. nil keeps the unwired legacy behavior.
	policy     CraftExecutionPolicyGate
	rpcTimeout time.Duration
	mu         sync.Mutex
	running    map[string]bool
}

// WithExecutionPolicy attaches the T03 uploaded-material execution gate
// (#122) to this command face. It must be set by the central assembly
// before the service is exposed; the gate runs first in Start. The attach
// is logged so a deployment that forgot to wire the security gate is
// observable in its logs.
func (s *CraftDockerRestrictedExec) WithExecutionPolicy(policy CraftExecutionPolicyGate) *CraftDockerRestrictedExec {
	if s == nil {
		return s
	}
	if policy == nil {
		logger.Warnf(context.Background(), "[CraftDockerRestrictedExec] T03 execution gate NOT attached: the uploaded-material execution policy is nil (unwired assembly)")
		return s
	}
	s.policy = policy
	logger.Infof(context.Background(), "[CraftDockerRestrictedExec] T03 execution gate attached")
	return s
}

func NewCraftDockerRestrictedExec(coordinator *CraftDockerSendCoordinator, docker sandbox.DockerOutputlessExecClient, rpcTimeout time.Duration) (*CraftDockerRestrictedExec, error) {
	if coordinator == nil || docker == nil {
		return nil, fmt.Errorf("restricted Docker exec requires coordinator and Docker client")
	}
	if rpcTimeout <= 0 {
		rpcTimeout = sandbox.DefaultDockerHTTPTimeout
	}
	return &CraftDockerRestrictedExec{coordinator: coordinator, client: docker, rpcTimeout: rpcTimeout, running: make(map[string]bool)}, nil
}

// WithExecEventPairObserver attaches the daemon event replay seam that makes an
// authoritative exec duration observable. Without an observer, terminal waits
// keep reporting the explicit "unavailable" duration contract.
func (s *CraftDockerRestrictedExec) WithExecEventPairObserver(events sandbox.DockerExecEventPairObserver) *CraftDockerRestrictedExec {
	if s != nil {
		s.events = events
	}
	return s
}

// Start prepares the durable hold before inert create, binds that exact exec
// receipt, claims it durably, and consumes the permission immediately before
// one detached ExecStart. Any post-claim transport error is unknown.
func (s *CraftDockerRestrictedExec) Start(ctx context.Context, grantID, activityID string, binding CraftCallBinding, handle sandbox.RemoteSandboxHandle, request CraftDockerOutputlessRequest) (CraftDockerOutputlessResult, error) {
	if s == nil || s.coordinator == nil || s.client == nil || !request.DiscardOutput || request.Exec.Stdin != "" || request.Exec.OnOutput != nil {
		return CraftDockerOutputlessResult{}, fmt.Errorf("restricted Docker exec requires explicit output discard and forbids stdin/callback")
	}
	if handle == nil || handle.Provider() != sandbox.SandboxTypeDocker || handle.ID() == "" || strings.TrimSpace(request.Exec.Command) == "" || (request.Exec.Shell && len(request.Exec.Args) > 0) {
		return CraftDockerOutputlessResult{}, fmt.Errorf("restricted Docker exec target or command is invalid")
	}
	// T03 (#122): uploaded code stays data. Screen the command before the
	// durable send is prepared; a denial returns the member-visible refusal
	// and nothing is created, bound or sent.
	if s.policy == nil {
		return CraftDockerOutputlessResult{}, errors.New("restricted Docker exec refused: T03 execution policy gate is not assembled (fail-closed)")
	}
	{
		if err := s.policy.ReviewOutputlessExec(ctx, binding, request); err != nil {
			return CraftDockerOutputlessResult{}, err
		}
	}
	op, err := s.coordinator.Prepare(ctx, grantID, activityID, binding)
	if err != nil {
		return CraftDockerOutputlessResult{}, err
	}
	receipt, err := s.client.CreateOutputlessExec(ctx, handle, sandbox.DockerOutputlessExecRequest{Request: request.Exec, DiscardOutput: true})
	if err != nil {
		return CraftDockerOutputlessResult{}, err
	}
	durable := repository.DockerExecReceipt{Provider: "docker", ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}
	if err := op.Bind(ctx, durable); err != nil {
		return CraftDockerOutputlessResult{}, err
	}
	return s.claimAndStart(ctx, op, durable)
}

// ResumeBound starts only the exact persisted receipt left by a crash after
// bind. A consumed claim returns no permission and can never resend. The
// T03 gate screens this send path too: an operation bound before the gate
// existed (upgrade window or a mixed fleet) must not slip an unreviewed
// command through recovery — an unreviewable command is never sent, from
// either path.
func (s *CraftDockerRestrictedExec) ResumeBound(ctx context.Context, grantID, activityID string) (CraftDockerOutputlessResult, error) {
	if s.policy == nil {
		return CraftDockerOutputlessResult{}, errors.New("restricted Docker exec refused: T03 execution policy gate is not assembled (fail-closed)")
	}
	{
		if err := s.policy.ReviewOutputlessExec(ctx, CraftCallBinding{}, CraftDockerOutputlessRequest{}); err != nil {
			return CraftDockerOutputlessResult{}, err
		}
	}
	op, durable, err := s.coordinator.ResumeBound(ctx, grantID, activityID)
	if err != nil {
		return CraftDockerOutputlessResult{}, err
	}
	return s.claimAndStart(ctx, op, durable)
}

func (s *CraftDockerRestrictedExec) claimAndStart(ctx context.Context, op *DockerSendOperation, durable repository.DockerExecReceipt) (CraftDockerOutputlessResult, error) {
	claim, err := op.Claim(ctx, durable)
	if err != nil {
		return CraftDockerOutputlessResult{}, err
	}
	if ctx.Err() != nil {
		return outputlessUnknownResult(op, durable, ctx.Err())
	}
	if claim.Permission == nil {
		return outputlessUnknownResult(op, durable, nil)
	}
	permissionReceipt, ok := claim.Permission.Consume()
	if !ok || permissionReceipt != durable {
		return outputlessUnknownResult(op, durable, nil)
	}
	providerReceipt := sandbox.DockerOutputlessExecReceipt{ContainerID: durable.ContainerID, ExecID: durable.ExecID}
	if err := s.client.StartOutputlessExec(ctx, providerReceipt, s.rpcTimeout); err != nil {
		return outputlessUnknownResult(op, durable, err)
	}
	return CraftDockerOutputlessResult{Receipt: durable, Accepted: true, StdoutAvailable: false, StderrAvailable: false}, nil
}

// Wait polls only the exact durably claimed receipt until terminal state or
// the caller's bound. Duration comes only from the daemon exec_start/exec_die
// event pair joined by the exact exec ID (never from poll elapsed time or
// ExecInspect, which carries no timestamps); a missing, ambiguous or
// non-monotonic pair reports the explicit unavailable contract and the
// captured pair is persisted to the operation journal. Cancellation/deadline
// leaves the claimed hold unresolved.
func (s *CraftDockerRestrictedExec) Wait(ctx context.Context, grantID, activityID string, maxWait time.Duration) (CraftDockerOutputlessResult, error) {
	unknown := CraftDockerOutputlessResult{State: sandbox.DockerOutputlessUnknown, DurationSource: string(sandbox.DockerExecEventPairUnavailable)}
	if s == nil || s.coordinator == nil || s.client == nil || grantID == "" || activityID == "" || maxWait <= 0 {
		return unknown, fmt.Errorf("restricted Docker Wait requires grant, activity and positive bound")
	}
	if err := ctx.Err(); err != nil {
		return unknown, outputlessWaitUnknownError(activityID, repository.DockerExecReceipt{}, err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, maxWait)
	defer cancel()
	durable, err := s.coordinator.Observe(waitCtx, grantID, activityID)
	if err != nil {
		if waitCtx.Err() != nil {
			return unknown, outputlessWaitUnknownError(activityID, repository.DockerExecReceipt{}, waitCtx.Err())
		}
		return unknown, err
	}
	if !durable.Claimed || durable.Receipt == nil {
		return unknown, sandbox.ErrRemoteOperationUnknown
	}
	unknown.Receipt = *durable.Receipt
	// Exponential backoff 20ms→200ms: each round costs two DB queries plus
	// a Docker inspect, so a fixed 20ms ticker hammers both backends (~150
	// round trips per second per waiter) for the whole wait window.
	interval := 20 * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := waitCtx.Err(); err != nil {
			return unknown, outputlessWaitUnknownError(activityID, *durable.Receipt, err)
		}
		observation, observeErr := s.Observe(waitCtx, grantID, activityID)
		if observeErr != nil {
			if waitCtx.Err() != nil {
				return unknown, outputlessWaitUnknownError(activityID, *durable.Receipt, waitCtx.Err())
			}
			return unknown, outputlessWaitUnknownError(activityID, *durable.Receipt, observeErr)
		}
		if err := waitCtx.Err(); err != nil {
			return unknown, outputlessWaitUnknownError(activityID, *durable.Receipt, err)
		}
		switch observation.State {
		case sandbox.DockerOutputlessSucceeded, sandbox.DockerOutputlessFailed:
			result := CraftDockerOutputlessResult{Receipt: *durable.Receipt, Accepted: true, State: observation.State, ExitCode: observation.ExitCode,
				DurationSource: string(sandbox.DockerExecEventPairUnavailable), StdoutAvailable: false, StderrAvailable: false}
			result.Duration, result.DurationAvailable, result.DurationSource = s.resolveTerminalDuration(ctx, grantID, activityID, *durable.Receipt)
			return result, nil
		case sandbox.DockerOutputlessUnknown:
			// Unknown is not terminal proof. Continue until the caller's bound.
		}
		if interval < 200*time.Millisecond {
			interval *= 2
			if interval > 200*time.Millisecond {
				interval = 200 * time.Millisecond
			}
			ticker.Reset(interval)
		}
		select {
		case <-waitCtx.Done():
			return unknown, outputlessWaitUnknownError(activityID, *durable.Receipt, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

const craftDockerEventObserveTimeout = 3 * time.Second

// resolveTerminalDuration answers the authoritative duration for one terminal
// claimed receipt. Journal-persisted evidence wins without provider I/O;
// otherwise the daemon event pair is replayed from the durable claim moment
// and persisted. Anything missing, ambiguous, conflicting or non-monotonic —
// including a persistence failure — stays explicitly unavailable.
func (s *CraftDockerRestrictedExec) resolveTerminalDuration(ctx context.Context, grantID, activityID string, receipt repository.DockerExecReceipt) (time.Duration, bool, string) {
	const unavailableSource = string(sandbox.DockerExecEventPairUnavailable)
	unavailable := func() (time.Duration, bool, string) { return 0, false, unavailableSource }
	resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), craftDockerEventObserveTimeout)
	defer cancel()
	durable, err := s.coordinator.Observe(resolveCtx, grantID, activityID)
	if err != nil {
		return unavailable()
	}
	if durable.Claimed && durable.Receipt != nil && *durable.Receipt != receipt {
		return unavailable()
	}
	if durable.DurationSource != nil && *durable.DurationSource == string(sandbox.DockerExecEventPairCaptured) {
		if durable.ExecEventStartedNS == nil || durable.ExecEventFinishedNS == nil {
			return unavailable()
		}
		persisted := sandbox.DockerExecEventPair{ExecID: receipt.ExecID, StartedNano: *durable.ExecEventStartedNS, FinishedNano: *durable.ExecEventFinishedNS}
		if d, ok := persisted.Duration(); ok {
			return d, true, string(sandbox.DockerExecEventPairCaptured)
		}
		return unavailable()
	}
	if s.events == nil || durable.ClaimedAt == nil {
		return unavailable()
	}
	pair, status, err := s.events.ObserveExecEventPair(resolveCtx, sandbox.DockerOutputlessExecReceipt{ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}, *durable.ClaimedAt)
	if err != nil || status != sandbox.DockerExecEventPairCaptured {
		return unavailable()
	}
	duration, ok := pair.Duration()
	if !ok {
		return unavailable()
	}
	if err := s.coordinator.RecordExecEventPair(resolveCtx, grantID, activityID, receipt, pair.StartedNano, pair.FinishedNano); err != nil {
		return unavailable()
	}
	return duration, true, string(sandbox.DockerExecEventPairCaptured)
}

func outputlessUnknownResult(op *DockerSendOperation, receipt repository.DockerExecReceipt, cause error) (CraftDockerOutputlessResult, error) {
	result := CraftDockerOutputlessResult{Receipt: receipt, State: sandbox.DockerOutputlessUnknown, DurationAvailable: false, StdoutAvailable: false, StderrAvailable: false}
	activityID := ""
	if op != nil {
		activityID = op.journal.ActivityKey
	}
	return result, &sandbox.RemoteOperationError{State: sandbox.RemoteOperationUnknown, Ref: sandbox.RemoteOperationRef{Provider: sandbox.SandboxTypeDocker, ID: receipt.ExecID, OperationKey: activityID, SandboxID: receipt.ContainerID}, Err: cause}
}

func outputlessWaitUnknownError(activityID string, receipt repository.DockerExecReceipt, cause error) error {
	return &sandbox.RemoteOperationError{State: sandbox.RemoteOperationUnknown, Ref: sandbox.RemoteOperationRef{Provider: sandbox.SandboxTypeDocker, ID: receipt.ExecID, OperationKey: activityID, SandboxID: receipt.ContainerID}, Err: cause}
}

// Observe checks durable ownership first, then inspects exactly that receipt.
// Running evidence is retained only in process memory; after restart a
// terminal false/zero remains unknown.
func (s *CraftDockerRestrictedExec) Observe(ctx context.Context, grantID, activityID string) (sandbox.DockerOutputlessExecObservation, error) {
	unknown := sandbox.DockerOutputlessExecObservation{State: sandbox.DockerOutputlessUnknown}
	durable, err := s.coordinator.Observe(ctx, grantID, activityID)
	if err != nil {
		return unknown, err
	}
	if !durable.Claimed || durable.Receipt == nil {
		return unknown, nil
	}
	receipt := *durable.Receipt
	key := grantID + "\x00" + activityID
	s.mu.Lock()
	previouslyRunning := s.running[key]
	s.mu.Unlock()
	observed, err := s.client.ObserveOutputlessExec(ctx, sandbox.DockerOutputlessExecReceipt{ContainerID: receipt.ContainerID, ExecID: receipt.ExecID}, previouslyRunning)
	if err != nil {
		// Claimed-never-started reconciliation: the container is GONE (its
		// lifecycle owns the sandbox) and no start evidence exists — the
		// exec can never run. Converge to the failed terminal instead of an
		// eternal Unknown; with prior Running evidence the honest answer
		// stays Unknown (it may have produced effects before deletion).
		var remoteErr *sandbox.RemoteError
		isNotFound := errors.As(err, &remoteErr) && remoteErr.Kind == sandbox.RemoteErrorKindNotFound
		if !previouslyRunning && (isNotFound || strings.Contains(err.Error(), "No such container") || strings.Contains(err.Error(), "No such exec")) {
			return sandbox.DockerOutputlessExecObservation{State: sandbox.DockerOutputlessFailed, OutputAvailable: false}, nil
		}
		return unknown, err
	}
	if observed.State == sandbox.DockerOutputlessRunning {
		s.mu.Lock()
		// Cap mirrors the normal-exec table: deleted containers never reach
		// a terminal observation, so abandoned keys are bounded by reset.
		if len(s.running) >= 4096 {
			logger.Warnf(ctx, "[CraftDockerRestrictedExec] running table hit its cap; resetting")
			s.running = make(map[string]bool)
		}
		s.running[key] = true
		s.mu.Unlock()
	} else if observed.State == sandbox.DockerOutputlessSucceeded || observed.State == sandbox.DockerOutputlessFailed {
		// TERMINAL observation only: the "was Running" attribution (a
		// terminal zero/missing exit code reads as failure, not unknown) has
		// served its purpose for this receipt and the long-lived service
		// must not accumulate one key per exec forever. An Unknown
		// observation deliberately KEEPS the flag — attribution is still
		// pending for a later terminal read.
		s.mu.Lock()
		delete(s.running, key)
		s.mu.Unlock()
	}
	return observed, nil
}
