package container

// open_connector.go is the T13 container wiring of the open-connector
// product chain: the production OCTokenSource (T11 carry), the T10-F-3
// dispatch arming MANDATE, the periodic OCRecovery.RunOnce caller (T12
// seam), and the graceful-shutdown supervisor (ruling 8).
//
// ADMIN-SECRET BOUNDARY (ruling 8): nothing in this file — and nothing in
// the API process it wires — ever holds the open-connector runtime ADMIN
// secret. That secret lives in the control worker (T05,
// connectorcontrol.FileAdminSecretSource). What the API process reads back
// is the RESTRICTED, per-connection, version-scoped runtime TOKEN material
// the control worker minted (T07/T05 machinery) from the shared secret
// sink — a credential that can do nothing but execute the actions of its
// one connection generation.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/appconnector/openconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/connectorcontrol"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Environment contract (T16 owns deployment docs):
//
//	WEKNORA_OC_ENABLED           "true"/"1" arms the dispatch path; anything
//	                             else keeps the explicit refusing dispatcher.
//	WEKNORA_OC_RUNTIME_ADDR      the runtime's INTERNAL address (must sit in
//	                             the executor/admin allowlist).
//	WEKNORA_OC_TOKEN_DIR         the secret sink directory shared (read
//	                             side) with the control worker.
//	WEKNORA_OC_SLOT_OWNER        lease owner id of THIS replica (default:
//	                             hostname).
//	WEKNORA_OC_RECOVERY_INTERVAL recovery loop period (default 15s).
const (
	OCEnabledEnv          = "WEKNORA_OC_ENABLED"
	OCRuntimeAddrEnv      = "WEKNORA_OC_RUNTIME_ADDR"
	OCTokenDirEnv         = "WEKNORA_OC_TOKEN_DIR"
	OCSlotOwnerEnv        = "WEKNORA_OC_SLOT_OWNER"
	OCRecoveryIntervalEnv = "WEKNORA_OC_RECOVERY_INTERVAL"
)

// ocDefaultRecoveryInterval bounds how long an unresolved dispatch can sit
// before a sweep pass looks at it: comfortably below the 90s staleness
// bound the sweep itself applies.
const ocDefaultRecoveryInterval = 15 * time.Second

// ocDrainTimeout is the ruling-8 in-flight drain window: after new claims
// stop, live dispatches get 30s to finish; whatever is still outstanding
// is handed to recovery (its records stay dispatched until the stale sweep
// parks them unknown for a provider query).
const ocDrainTimeout = 30 * time.Second

// OCConfig is the deployment configuration of the OC dispatch path.
type OCConfig struct {
	Enabled          bool
	RuntimeAddr      string
	TokenDir         string
	SlotOwner        string
	RecoveryInterval time.Duration
	DrainTimeout     time.Duration
}

// OCConfigFromEnv reads the contract above; malformed durations fail to the
// safe default (the wiring itself stays fail-closed elsewhere).
func OCConfigFromEnv() OCConfig {
	cfg := OCConfig{
		Enabled:          strings.EqualFold(strings.TrimSpace(os.Getenv(OCEnabledEnv)), "true") || strings.TrimSpace(os.Getenv(OCEnabledEnv)) == "1",
		RuntimeAddr:      strings.TrimSpace(os.Getenv(OCRuntimeAddrEnv)),
		TokenDir:         strings.TrimSpace(os.Getenv(OCTokenDirEnv)),
		SlotOwner:        strings.TrimSpace(os.Getenv(OCSlotOwnerEnv)),
		RecoveryInterval: ocDefaultRecoveryInterval,
		DrainTimeout:     ocDrainTimeout,
	}
	if raw := strings.TrimSpace(os.Getenv(OCRecoveryIntervalEnv)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			cfg.RecoveryInterval = d
		}
	}
	if cfg.SlotOwner == "" {
		if host, err := os.Hostname(); err == nil && host != "" {
			cfg.SlotOwner = host + "-" + strconv.Itoa(os.Getpid())
		} else {
			cfg.SlotOwner = "weknora-oc-" + strconv.Itoa(os.Getpid())
		}
	}
	return cfg
}

// ---------------------------------------------------------------------------
// production OCTokenSource (T11 carry)
// ---------------------------------------------------------------------------

// OCSecretReader is the read side of the control worker's secret sink.
type OCSecretReader interface {
	GetSecret(ctx context.Context, ref string) (string, error)
}

// OCTokenRef is the deterministic secret reference of one connection
// generation's restricted runtime token. It MUST stay byte-identical to
// connectorcontrol's mintRuntimeToken ref — that parity is what lets the
// API process read back exactly the token the control worker cast for
// (tenant, connection, authVersion) and nothing else.
func OCTokenRef(tenant uint64, connectionID string, authVersion int64) string {
	return fmt.Sprintf("oc/runtime-token/%d/%s/%d", tenant, connectionID, authVersion)
}

// ocSinkFileName mirrors connectorcontrol.FileSecretSink's on-disk name
// (sha256 of the ref, hex, .secret) so tests can place material the same
// way the worker does.
func ocSinkFileName(ref string) string {
	sum := sha256Sum(ref)
	return sum + ".secret"
}

// FileBackedOCTokenSource is the production OCTokenSource: it reads the
// version-scoped restricted runtime token the control worker minted for
// EXACTLY the requested (tenant, connection, authVersion) from the shared
// secret sink. Before any read it re-verifies the tenant's LIVE binding —
// active and at exactly that authorization generation — so a revoked or
// rebound connection can never be handed a token, and a stale caller can
// never reach for an older generation. It never mints (no admin secret
// exists here) and never fabricates a token on a missing reference.
type FileBackedOCTokenSource struct {
	sink     OCSecretReader
	bindings appconn.OCBindingStore
}

// NewFileBackedOCTokenSource validates the wiring (both dependencies
// required) and builds the source.
func NewFileBackedOCTokenSource(sink OCSecretReader, bindings appconn.OCBindingStore) (*FileBackedOCTokenSource, error) {
	if sink == nil || bindings == nil {
		return nil, errors.New("open-connector token source: secret sink and binding store are required")
	}
	return &FileBackedOCTokenSource{sink: sink, bindings: bindings}, nil
}

// Token returns the restricted runtime token of exactly one connection
// generation, or a refusal. Refusals are pre-send rejections by
// construction: the OCDispatcher wraps them in ErrDispatchNotStarted, so
// no provider call ever leaves the process on their account.
func (s *FileBackedOCTokenSource) Token(ctx context.Context, tenant uint64, connectionID string, authVersion int64) (string, error) {
	binding, err := s.bindings.GetBinding(ctx, tenant, connectionID)
	if err != nil {
		return "", fmt.Errorf("token source: binding for connection %q: %w", connectionID, err)
	}
	if binding.State != appconn.OCBindingActive {
		return "", fmt.Errorf("token source: binding for connection %q is %s", connectionID, binding.State)
	}
	if binding.AuthVersion != authVersion {
		return "", fmt.Errorf("token source: connection %q is at authorization generation %d, token requested for %d",
			connectionID, binding.AuthVersion, authVersion)
	}
	token, err := s.sink.GetSecret(ctx, OCTokenRef(tenant, connectionID, authVersion))
	if err != nil {
		return "", fmt.Errorf("token source: no restricted token material for connection %q at generation %d: %w",
			connectionID, authVersion, err)
	}
	return token, nil
}

// ---------------------------------------------------------------------------
// graceful shutdown pieces (ruling 8)
// ---------------------------------------------------------------------------

// OCShutdownGate is the claim-side stop signal: once closed, new dispatch
// claims refuse (429 at the edge) while reads stay open so recovery and
// drain-time queries keep working.
type OCShutdownGate struct {
	mu     sync.RWMutex
	closed bool
}

// Close latches the gate (idempotent).
func (g *OCShutdownGate) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
}

// Closed reports whether new claims must stop.
func (g *OCShutdownGate) Closed() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.closed
}

// GatedOCClaims wraps the durable claim store with the shutdown gate. When
// the gate is closed, ClaimOCDispatch refuses with the queue-limit
// sentinel (repoappconn.ErrOCLeaseBusy — the handler maps it to 429); the
// refusal happens at the claim boundary, so the approval count is never
// consumed. A budget Begin that already ran under a refused claim settles
// through the commercial orphan-hold recovery (fail-safe over-hold, no
// double charge) — the frozen Execute order (slots → Begin → claim) cannot
// be reordered from the container.
type GatedOCClaims struct {
	appconnectorsvc.OCDispatchClaimSource
	gate *OCShutdownGate
}

// ClaimOCDispatch refuses once the shutdown gate is closed.
func (g *GatedOCClaims) ClaimOCDispatch(ctx context.Context, subject appconn.OCSubject, actionID, reservationID string) (appconn.OCDispatchRecord, error) {
	if g.gate.Closed() {
		return appconn.OCDispatchRecord{}, fmt.Errorf("%w: dispatch claims paused for shutdown", repoappconn.ErrOCLeaseBusy)
	}
	return g.OCDispatchClaimSource.ClaimOCDispatch(ctx, subject, actionID, reservationID)
}

// TrackingOCDispatcher counts in-flight outbound dispatches so shutdown
// can drain them (Wait). It changes nothing about the calls themselves.
type TrackingOCDispatcher struct {
	inner appconnectorsvc.ActionDispatcher
	wg    sync.WaitGroup
}

// Dispatch tracks one in-flight call.
func (d *TrackingOCDispatcher) Dispatch(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	d.wg.Add(1)
	defer d.wg.Done()
	return d.inner.Dispatch(ctx, snap, providerKey)
}

// Wait blocks until every tracked dispatch returned.
func (d *TrackingOCDispatcher) Wait() { d.wg.Wait() }

// OCRunOnce is one reconciliation pass (the OCRecovery/OCRecoveryRunner
// seam; a stub in tests).
type OCRunOnce interface {
	RunOnce(ctx context.Context) error
}

// OCRecoveryRunner is the periodic caller of OCRecovery.RunOnce (T12
// seam): interval-bounded, error-tolerant (a failed pass logs and waits
// for the next tick), and context-cancellable for graceful shutdown.
type OCRecoveryRunner struct {
	rec    OCRunOnce
	every  time.Duration
	onErr  func(error)
	opWait time.Duration
}

// NewOCRecoveryRunner validates the wiring; every must be positive.
func NewOCRecoveryRunner(rec OCRunOnce, every time.Duration, onErr func(error)) *OCRecoveryRunner {
	if every <= 0 {
		every = ocDefaultRecoveryInterval
	}
	if onErr == nil {
		onErr = func(error) {}
	}
	return &OCRecoveryRunner{rec: rec, every: every, onErr: onErr, opWait: 10 * time.Second}
}

// Run loops until ctx is done: one bounded RunOnce per tick.
func (r *OCRecoveryRunner) Run(ctx context.Context) {
	ticker := time.NewTicker(r.every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rctx, cancel := context.WithTimeout(ctx, r.opWait)
			if err := r.rec.RunOnce(rctx); err != nil {
				r.onErr(err)
			}
			cancel()
		}
	}
}

// OCSupervisor sequences the graceful shutdown (ruling 8):
//  1. stop new claims (the gate latches; the edge answers 429);
//  2. bounded in-flight drain (default 30s) — tracked dispatches may
//     finish and settle normally;
//  3. remainder → recovery: still-dispatched records are LEFT to the
//     recovery loop — the stale sweep parks them unknown for a provider
//     query (never a resend), and one final RunOnce pass is attempted
//     here under the remaining time budget.
type OCSupervisor struct {
	gate      *OCShutdownGate
	inflight  *TrackingOCDispatcher
	recovery  OCRunOnce
	drain     time.Duration
	stopRun   context.CancelFunc
	logNotice func(string, ...any)
}

// NewOCSupervisor builds the supervisor; drain must be positive.
func NewOCSupervisor(gate *OCShutdownGate, inflight *TrackingOCDispatcher, recovery OCRunOnce, drain time.Duration) *OCSupervisor {
	if drain <= 0 {
		drain = ocDrainTimeout
	}
	return &OCSupervisor{gate: gate, inflight: inflight, recovery: recovery, drain: drain, logNotice: func(string, ...any) {}}
}

// AttachRunnerCancel lets Shutdown stop the periodic runner after the
// drain (the loop must not race the final pass forever).
func (s *OCSupervisor) AttachRunnerCancel(cancel context.CancelFunc) { s.stopRun = cancel }

// Shutdown executes the ruling-8 sequence. It always returns; a stuck
// in-flight dispatch cannot extend the drain window.
func (s *OCSupervisor) Shutdown(ctx context.Context) {
	// 1. Stop new claims first.
	s.gate.Close()
	// 2. Bounded drain of in-flight dispatches.
	drained := make(chan struct{})
	if s.inflight != nil {
		go func() {
			s.inflight.Wait()
			close(drained)
		}()
	} else {
		close(drained)
	}
	timer := time.NewTimer(s.drain)
	defer timer.Stop()
	select {
	case <-drained:
	case <-timer.C:
	case <-ctx.Done():
	}
	// 3. Remainder → recovery: stop the periodic runner, then attempt one
	// final bounded pass (the records of unfinished dispatches stay
	// dispatched; the sweep parks them unknown on later passes).
	if s.stopRun != nil {
		s.stopRun()
	}
	if s.recovery != nil {
		rctx, cancel := context.WithTimeout(context.Background(), s.drain)
		if err := s.recovery.RunOnce(rctx); err != nil {
			s.logNotice("[OpenConnector] final recovery pass: %v", err)
		}
		cancel()
	}
}

// ---------------------------------------------------------------------------
// dispatch arming (T10-F-3 MANDATE) + wiring
// ---------------------------------------------------------------------------

// ArmOCDispatch wires the durable claim store and the slot limiter onto
// the ActionService, enforcing the T10-F-3 mandate: a non-nil dispatcher
// with a nil claim store is a WIRING VIOLATION — the error must surface at
// STARTUP, because the no-durable-record fallback it would enable is
// exactly what T10 retired. The disabled wiring (nil dispatcher, nil
// claims) stays legal: that is the explicit refusing dispatcher.
func ArmOCDispatch(svc *appconnectorsvc.ActionService, dispatcher appconnectorsvc.ActionDispatcher, claims appconnectorsvc.OCDispatchClaimSource, slots *appconnectorsvc.OCSlotLimiter) error {
	if svc == nil {
		return errors.New("open-connector wiring: the action service is required")
	}
	if dispatcher != nil && claims == nil {
		return errors.New("open-connector wiring violation (T10-F-3): a real dispatcher requires the durable claim store — UseOCSlotLimiter and UseOCDispatchClaims must be wired before/with any non-nil dispatcher")
	}
	if claims != nil {
		svc.UseOCDispatchClaims(claims)
	}
	if slots != nil {
		svc.UseOCSlotLimiter(slots)
	}
	return nil
}

// OCWiring is the armed result of WireOpenConnector.
type OCWiring struct {
	config     OCConfig
	dispatcher appconnectorsvc.ActionDispatcher
	claims     appconnectorsvc.OCDispatchClaimSource
	slots      *appconnectorsvc.OCSlotLimiter
	supervisor *OCSupervisor
}

// Enabled reports whether the dispatch path is armed.
func (w *OCWiring) Enabled() bool { return w != nil && w.config.Enabled }

// Dispatcher returns the armed (tracking-wrapped) dispatcher, nil when disabled.
func (w *OCWiring) Dispatcher() appconnectorsvc.ActionDispatcher { return w.dispatcher }

// Claims returns the (shutdown-gated) durable claim store, nil when disabled.
func (w *OCWiring) Claims() appconnectorsvc.OCDispatchClaimSource { return w.claims }

// Slots returns the armed limiter, nil when disabled.
func (w *OCWiring) Slots() *appconnectorsvc.OCSlotLimiter { return w.slots }

// Supervisor returns the graceful-shutdown supervisor (always present).
func (w *OCWiring) Supervisor() *OCSupervisor { return w.supervisor }

// WireOpenConnector arms the ActionService's open-connector dispatch path
// from the deployment config.
//
// Disabled (default): the dispatcher stays nil — that nil IS the plan's
// "explicit refusing dispatcher": ActionService.Execute fails closed with
// ErrNoDispatcher BEFORE consuming anything (no approval, no budget
// reservation, no HTTP call — asserted by the T13 handler matrix), and the
// edge maps it to 503. Injecting a non-nil stub instead would be WORSE: it
// would pass the nil gate and could settle outcomes.
//
// Enabled: the runtime address and the shared token dir are REQUIRED — an
// enabled-but-incomplete configuration is a STARTUP FAILURE, never a
// fallback to some default connection. The full path (restricted-token
// source → single-POST executor → OC dispatcher, gated durable claims,
// four-scope limiter) is armed atomically through ArmOCDispatch.
//
// budgetUpper CONFIG REQUIREMENT (ruling 7 carry): ActionService reserves
// budget with Upper=1 credit per dispatch (frozen T04-T12 face; no setter
// exists yet). Any action whose real price exceeds 1 credit therefore
// settles abnormal_cost — the settlement itself still persists. Until a
// price-table seam is added (additive-only change, future task), the
// deployment requirement is: only publish actions whose price is at most
// the default upper, or extend ActionService with a configurable upper
// before publishing priced actions.
func WireOpenConnector(svc *appconnectorsvc.ActionService, cfg OCConfig, ocStore *repoappconn.OCStore, httpClient *http.Client) (*OCWiring, error) {
	if svc == nil || ocStore == nil {
		return nil, errors.New("open-connector wiring: the action service and the OC store are required")
	}
	gate := &OCShutdownGate{}
	if !cfg.Enabled {
		return &OCWiring{
			config:     cfg,
			supervisor: NewOCSupervisor(gate, nil, nil, cfg.DrainTimeout),
		}, nil
	}
	if cfg.RuntimeAddr == "" || cfg.TokenDir == "" {
		return nil, fmt.Errorf("open-connector enabled but configuration incomplete: %s and %s are required (refusing to fall back to a default connection)",
			OCRuntimeAddrEnv, OCTokenDirEnv)
	}
	sink, err := connectorcontrol.NewFileSecretSink(cfg.TokenDir)
	if err != nil {
		return nil, fmt.Errorf("open-connector secret sink: %w", err)
	}
	tokens, err := NewFileBackedOCTokenSource(sink, ocStore)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	exec, err := openconnector.NewClient(cfg.RuntimeAddr, httpClient)
	if err != nil {
		return nil, fmt.Errorf("open-connector executor: %w", err)
	}
	inner := appconnectorsvc.NewOCDispatcher(exec, ocStore, tokens, ocStore)
	inner.UseOCRetryAfterSink(ocStore) // 429 cooldowns recorded durably
	tracking := &TrackingOCDispatcher{inner: inner}
	slots, err := appconnectorsvc.NewOCSlotLimiter(ocStore, appconnectorsvc.DefaultOCSlotLimits(), cfg.SlotOwner)
	if err != nil {
		return nil, fmt.Errorf("open-connector slot limiter: %w", err)
	}
	claims := &GatedOCClaims{OCDispatchClaimSource: ocStore, gate: gate}
	if err := ArmOCDispatch(svc, tracking, claims, slots); err != nil {
		return nil, err
	}
	return &OCWiring{
		config:     cfg,
		dispatcher: tracking,
		claims:     claims,
		slots:      slots,
		supervisor: NewOCSupervisor(gate, tracking, nil, cfg.DrainTimeout),
	}, nil
}

// ocHTTPClient is the dispatch-path HTTP client: 30s timeout, matching the
// dispatch deadline the recovery hook assumes (ocDispatchClientDeadline).
func ocHTTPClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// ---------------------------------------------------------------------------
// dig wiring (the container-facing constructors; error returns FAIL
// STARTUP, which is exactly what rulings 5/8 demand)
// ---------------------------------------------------------------------------

// newOCArmedActionService builds the ActionService and arms its
// open-connector dispatch path from the environment (see WireOpenConnector
// for the disabled / enabled semantics and the budgetUpper note).
func newOCArmedActionService(
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	oc *repoappconn.OCStore,
) (*appconnectorsvc.ActionService, *OCWiring, error) {
	svc := appconnectorsvc.NewActionService(store, guard, gate, nil, nil)
	wiring, err := WireOpenConnector(svc, OCConfigFromEnv(), oc, ocHTTPClient())
	if err != nil {
		return nil, nil, err
	}
	return svc, wiring, nil
}

// newOCProductServices builds the T13 LOCAL product services the handlers
// accept: the trusted OC preparer (risk/version/binding filled
// server-side) and the correlate-able connection lifecycle. The transient
// cipher is armed only when the deployment key exists — with no key the
// OAuth path works and the API-key handoff fails closed per attempt (503).
func newOCProductServices(
	svc *appconnectorsvc.ActionService,
	oc *repoappconn.OCStore,
	installs *repoappconn.InstallationStore,
	src appconnectorsvc.ConnectionCredentialSource,
) (*appconnectorsvc.OCPreparer, *appconnectorsvc.OCConnectionService, error) {
	if svc == nil || oc == nil || installs == nil || src == nil {
		return nil, nil, errors.New("open-connector product services: action service, OC store, installations and credential source are required")
	}
	catalog := appconnectorsvc.NewOCCatalog(oc, oc, installs, installs)
	preparer := appconnectorsvc.NewOCPreparer(svc, catalog, oc)
	var opts []appconnectorsvc.OCConnectionOption
	if cipher, err := appconnectorsvc.NewTransientCipherFromEnv(); err == nil {
		opts = append(opts, appconnectorsvc.WithTransientCipher(cipher))
	}
	connSvc := appconnectorsvc.NewOCConnectionService(src, installs, oc, opts...)
	return preparer, connSvc, nil
}

// startOCRecoveryRunner starts the periodic OCRecovery.RunOnce caller (T12
// seam; CARRY T11/T13) and registers the graceful-shutdown sequence
// (ruling 8: stop new claims → 30s in-flight drain → remainder handed to
// recovery) with the resource cleaner, so a server shutdown runs it after
// the HTTP listener has stopped accepting requests. The loop runs in every
// mode: with the OC path disabled there are no OC dispatch records to
// sweep, and a deployment that turns OC OFF keeps its stale records
// converging exactly as T12 designed.
func startOCRecoveryRunner(
	svc *appconnectorsvc.ActionService,
	oc *repoappconn.OCStore,
	wiring *OCWiring,
	cleaner interfaces.ResourceCleaner,
) error {
	recovery, err := appconnectorsvc.NewOCRecovery(oc, svc)
	if err != nil {
		return err
	}
	cfg := OCConfigFromEnv()
	runner := NewOCRecoveryRunner(recovery, cfg.RecoveryInterval, func(err error) {
		logger.Errorf(context.Background(), "[OpenConnector] recovery pass: %v", err)
	})
	// The supervisor hands still-draining records to this recovery on
	// shutdown; attach the runner's cancel so the periodic loop stops then.
	ctx, cancel := context.WithCancel(context.Background())
	wiring.Supervisor().AttachRunnerCancel(cancel)
	go runner.Run(ctx)
	cleaner.RegisterWithName("OpenConnector", func() error {
		wiring.Supervisor().Shutdown(context.Background())
		return nil
	})
	return nil
}

// sha256Sum is a tiny local helper (crypto/sha256) mirroring the sink's
// file naming; kept here so tests can place material identically.
func sha256Sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
