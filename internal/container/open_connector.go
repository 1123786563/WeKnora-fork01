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
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Environment contract (T16 owns deployment docs):
//
//	WEKNORA_OC_ENABLED           "true"/"1" arms the dispatch path; anything
//	                             else keeps the explicit refusing dispatcher.
//	WEKNORA_OC_RUNTIME_ADDR      the runtime's INTERNAL address (must sit in
//	                             the executor/admin allowlist). When unset,
//	                             the yaml open_connector.runtime id resolves
//	                             through config.OpenConnectorRuntimeAddr.
//	WEKNORA_OC_TOKEN_DIR         the secret sink directory shared (read
//	                             side) with the control worker.
//	WEKNORA_OC_SECRET_KEY_FILE   when set, the read side uses the encrypted
//	                             sink (matches the control worker's mandated
//	                             CONNECTOR_CONTROL_SECRET_KEY_FILE); empty
//	                             keeps the legacy read-compat sink for dev.
//	WEKNORA_OC_SLOT_OWNER        lease owner id of THIS replica (default:
//	                             hostname).
//	WEKNORA_OC_RECOVERY_INTERVAL recovery loop period (default 15s).
const (
	OCEnabledEnv          = "WEKNORA_OC_ENABLED"
	OCRuntimeAddrEnv      = "WEKNORA_OC_RUNTIME_ADDR"
	OCTokenDirEnv         = "WEKNORA_OC_TOKEN_DIR"
	OCSecretKeyFileEnv    = "WEKNORA_OC_SECRET_KEY_FILE"
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
	SecretKeyFile    string
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
		SecretKeyFile:    strings.TrimSpace(os.Getenv(OCSecretKeyFileEnv)),
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

// ApplyOpenConnectorYAML layers the yaml open_connector section (T16) under
// the env contract: env values (the T13 WEKNORA_OC_* wiring) win when they
// are set; otherwise Enabled comes from the yaml opt-in and RuntimeAddr is
// resolved STRICTLY through the internal id map (config.OpenConnectorRuntimeAddr)
// — a config file can never aim the dispatcher at an arbitrary host. A nil
// config or section leaves env-only behavior unchanged.
func ApplyOpenConnectorYAML(out OCConfig, cfg *config.Config) OCConfig {
	if cfg == nil || cfg.OpenConnector == nil {
		return out
	}
	if strings.TrimSpace(os.Getenv(OCEnabledEnv)) == "" {
		out.Enabled = cfg.OpenConnector.IsEnabled()
	}
	if out.RuntimeAddr == "" {
		if addr, ok := config.OpenConnectorRuntimeAddr(cfg.OpenConnector.Runtime); ok {
			out.RuntimeAddr = addr
		}
	}
	return out
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

// T17-F1 fix (R20): the wrapper must stay TRANSPARENT for the T12 settle
// face. ActionService detects the settle face by type assertion on its wired
// claim source (ocDispatchSettleOf), so a GatedOCClaims that only forwards
// the claim methods silently hides the face — Execute then falls back to the
// pre-T12 finish order and the durable dispatch record never reaches a
// terminal state (spurious unknowns + a settlement outbox that never drains
// through the record). Forward both methods; settlement is deliberately NOT
// gated: draining and settling in-flight dispatches during shutdown is
// exactly what ruling 8 asks the recovery path to do.
func (g *GatedOCClaims) FinishOCDispatch(ctx context.Context, tenant uint64, actionID string, fence int64, from, to, executionID string) error {
	settle, ok := g.OCDispatchClaimSource.(appconnectorsvc.OCDispatchSettleSource)
	if !ok {
		return errors.New("open-connector wiring: gated claims wrap a claim store without the settle face")
	}
	return settle.FinishOCDispatch(ctx, tenant, actionID, fence, from, to, executionID)
}

// MarkOCDispatchSettled forwards the delivery mark of the T12 settle face.
func (g *GatedOCClaims) MarkOCDispatchSettled(ctx context.Context, tenant uint64, actionID string, fence int64, now time.Time) error {
	settle, ok := g.OCDispatchClaimSource.(appconnectorsvc.OCDispatchSettleSource)
	if !ok {
		return errors.New("open-connector wiring: gated claims wrap a claim store without the settle face")
	}
	return settle.MarkOCDispatchSettled(ctx, tenant, actionID, fence, now)
}

// Compile-time proof that the wrapper satisfies the settle face the
// forwarding above delegates to (PrepareOpenConnector always wires the
// repository *OCStore, which carries it).
var _ appconnectorsvc.OCDispatchSettleSource = (*GatedOCClaims)(nil)

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

// AttachRunner hands the supervisor the periodic runner's cancel AND the
// recovery instance the final shutdown pass drives (F-2 fix: without the
// recovery reference the final RunOnce in Shutdown would be dead code —
// both production call sites now attach the real recovery).
func (s *OCSupervisor) AttachRunner(cancel context.CancelFunc, recovery OCRunOnce) {
	s.stopRun = cancel
	if recovery != nil {
		s.recovery = recovery
	}
}

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

// ocServiceDispatcherNil reports whether the frozen ActionService's
// private dispatcher field is nil. The dispatcher rides the frozen
// NewActionService CONSTRUCTOR (no setter exists), so this nil-parity read
// is what lets the arming fail fast on the F-1 defect class: a wiring that
// built a dispatcher but never injected it. Only nil-ness is inspected —
// reading VALUES out of unexported fields would panic; IsNil does not.
func ocServiceDispatcherNil(svc *appconnectorsvc.ActionService) (bool, error) {
	if svc == nil {
		return true, errors.New("open-connector wiring: nil action service")
	}
	f := reflect.ValueOf(svc).Elem().FieldByName("dispatcher")
	if !f.IsValid() || f.Kind() != reflect.Interface {
		return true, errors.New("open-connector wiring: the action service's dispatcher field moved — frozen face changed, update the OC wiring")
	}
	return f.IsNil(), nil
}

// ArmOCDispatch wires the durable claim store and the slot limiter onto
// the ActionService, enforcing the T10-F-3 mandate: a non-nil dispatcher
// with a nil claim store is a WIRING VIOLATION — the error must surface at
// STARTUP, because the no-durable-record fallback it would enable is
// exactly what T10 retired. The disabled wiring (nil dispatcher, nil
// claims) stays legal: that is the explicit refusing dispatcher.
//
// F-1 guard (spec review): the dispatcher parameter is NOT advisory. The
// frozen ActionService accepts its dispatcher only through NewActionService,
// so the arming refuses to bless a service whose dispatcher does not match
// the wiring — constructing the service WITHOUT the wiring's dispatcher
// and arming it anyway is a startup error, never a silently dropped
// dispatch path (the enabled-path 503-everything defect).
func ArmOCDispatch(svc *appconnectorsvc.ActionService, dispatcher appconnectorsvc.ActionDispatcher, claims appconnectorsvc.OCDispatchClaimSource, slots *appconnectorsvc.OCSlotLimiter) error {
	if svc == nil {
		return errors.New("open-connector wiring: the action service is required")
	}
	if dispatcher != nil && claims == nil {
		return errors.New("open-connector wiring violation (T10-F-3): a real dispatcher requires the durable claim store — UseOCSlotLimiter and UseOCDispatchClaims must be wired before/with any non-nil dispatcher")
	}
	svcDispatcherNil, err := ocServiceDispatcherNil(svc)
	if err != nil {
		return err
	}
	if svcDispatcherNil != (dispatcher == nil) {
		return errors.New("open-connector wiring violation (T13 F-1): the action service's dispatcher does not match the wiring — construct it with the wiring's dispatcher (NewOCArmedActionService does this) before arming; an armed dispatcher must never be dropped")
	}
	if claims != nil {
		svc.UseOCDispatchClaims(claims)
	}
	if slots != nil {
		svc.UseOCSlotLimiter(slots)
	}
	return nil
}

// OCWiring is the armed result of PrepareOpenConnector.
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

// PrepareOpenConnector builds the complete open-connector dispatch wiring
// from the deployment config WITHOUT touching an ActionService. Splitting
// this out is the F-1 fix: the frozen ActionService takes its dispatcher
// ONLY through NewActionService, so the service must be constructed AFTER
// the wiring exists — NewOCArmedActionService composes the two and is the
// single production path.
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
// four-scope limiter) is built atomically; dispatcher and claims can never
// exist apart.
//
// budgetUpper CONFIG REQUIREMENT (ruling 7 carry): ActionService reserves
// budget with Upper=1 credit per dispatch (frozen T04-T12 face; no setter
// exists yet). Any action whose real price exceeds 1 credit therefore
// settles abnormal_cost — the settlement itself still persists. Until a
// price-table seam is added (additive-only change, future task), the
// deployment requirement is: only publish actions whose price is at most
// the default upper, or extend ActionService with a configurable upper
// before publishing priced actions.
func PrepareOpenConnector(cfg OCConfig, ocStore *repoappconn.OCStore, httpClient *http.Client) (*OCWiring, error) {
	if ocStore == nil {
		return nil, errors.New("open-connector wiring: the OC store is required")
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
	// Secret sink (T16 hardening): with a provisioned key file the read side
	// uses the ENCRYPTED sink the control worker writes through (it refuses
	// to start without a key, so production never writes plaintext). Without
	// a key the legacy read-compat sink stays — dev/test deployments whose
	// control worker runs against the same plaintext store — and a later key
	// rollout simply re-mints tokens; reads of sealed records under a missing
	// key fail closed at use time.
	sink, err := newOCSecretReader(cfg)
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
	return &OCWiring{
		config:     cfg,
		dispatcher: tracking,
		claims:     claims,
		slots:      slots,
		supervisor: NewOCSupervisor(gate, tracking, nil, cfg.DrainTimeout),
	}, nil
}

// ArmActionService wires this prepared wiring's claims and slots onto svc
// under the T10-F-3 + F-1 guards (dispatcher parity included — see
// ArmOCDispatch). The service must already have been constructed with
// w.Dispatcher(); NewOCArmedActionService is the composition that
// guarantees it.
func (w *OCWiring) ArmActionService(svc *appconnectorsvc.ActionService) error {
	if w == nil {
		return errors.New("open-connector wiring: nil wiring")
	}
	return ArmOCDispatch(svc, w.dispatcher, w.claims, w.slots)
}

// NewOCArmedActionService is the PRODUCTION constructor: it prepares the
// OC wiring, builds the ActionService WITH the wiring's dispatcher (the
// frozen constructor is the only injection point — this ordering is the
// F-1 fix), then arms claims+slots atomically. An error return fails
// container STARTUP (rulings 5/8). Tests must exercise dispatch arming
// through this path, never by bypassing it with NewActionService.
func NewOCArmedActionService(
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	ocStore *repoappconn.OCStore,
	cfg OCConfig,
	httpClient *http.Client,
) (*appconnectorsvc.ActionService, *OCWiring, error) {
	wiring, err := PrepareOpenConnector(cfg, ocStore, httpClient)
	if err != nil {
		return nil, nil, err
	}
	// F-1: the armed dispatcher rides the CONSTRUCTOR — this is the only
	// seam the frozen ActionService offers, so the wiring must exist first.
	svc := appconnectorsvc.NewActionService(store, guard, gate, wiring.Dispatcher(), nil)
	if err := wiring.ArmActionService(svc); err != nil {
		return nil, nil, err
	}
	return svc, wiring, nil
}

// ocHTTPClient is the dispatch-path HTTP client: 30s timeout, matching the
// dispatch deadline the recovery hook assumes (ocDispatchClientDeadline).
func ocHTTPClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

// ---------------------------------------------------------------------------
// dig wiring (the container-facing constructors; error returns FAIL
// STARTUP, which is exactly what rulings 5/8 demand)
// ---------------------------------------------------------------------------

// newOCArmedActionService is the dig constructor: it delegates to the
// PRODUCTION path (NewOCArmedActionService) with the merged config — env
// contract (T13) layered over the yaml open_connector section (T16) — so
// the dispatcher reaches the service through the frozen constructor and
// the T10-F-3/F-1 guards run at startup (see PrepareOpenConnector for the
// disabled/enabled semantics and the budgetUpper note).
func newOCArmedActionService(
	store appconnectorsvc.ActionStoreSource,
	guard appconnectorsvc.A02Guard,
	gate domain.ExecutionGate,
	oc *repoappconn.OCStore,
	cfg *config.Config,
) (*appconnectorsvc.ActionService, *OCWiring, error) {
	return NewOCArmedActionService(store, guard, gate, oc, ApplyOpenConnectorYAML(OCConfigFromEnv(), cfg), ocHTTPClient())
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
	// shutdown: attach BOTH the runner's cancel and the recovery itself so
	// the final Shutdown pass drives the real RunOnce (F-2).
	ctx, cancel := context.WithCancel(context.Background())
	wiring.Supervisor().AttachRunner(cancel, recovery)
	go runner.Run(ctx)
	cleaner.RegisterWithName("OpenConnector", func() error {
		wiring.Supervisor().Shutdown(context.Background())
		return nil
	})
	return nil
}

// newOCSecretReader builds the READ side of the control worker's secret
// sink for the API process: the encrypted sink when a key file is
// provisioned (production; matches the control worker's mandated key), the
// legacy plaintext sink otherwise (dev/test read compatibility — see the
// PrepareOpenConnector comment for why production can never end up on it).
func newOCSecretReader(cfg OCConfig) (OCSecretReader, error) {
	if cfg.SecretKeyFile != "" {
		return connectorcontrol.NewEncryptedFileSecretSink(
			cfg.TokenDir, connectorcontrol.FileSecretKeySource(cfg.SecretKeyFile))
	}
	return connectorcontrol.NewFileSecretSink(cfg.TokenDir)
}

// sha256Sum is a tiny local helper (crypto/sha256) mirroring the sink's
// file naming; kept here so tests can place material identically.
func sha256Sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
