package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/google/uuid"
)

// Action lifecycle sentinel errors surfaced by the service.
var (
	// ErrActionDigestMismatch: the approval presented a digest that does not
	// match the action's current snapshot. An old approval can never
	// authorize new (modified) args.
	ErrActionDigestMismatch = errors.New("action_digest_mismatch")
	// ErrActionState: invalid lifecycle transition (e.g. executing an
	// action that is not authorized, or re-queueing an unknown one).
	ErrActionState = errors.New("action_state_conflict")
	// ErrInvalidAction: Prepare input missing bound identity fields.
	ErrInvalidAction = errors.New("invalid_action")
	// ErrDispatchUnknown: the provider outcome is unknown (crash window);
	// the action is parked in unknown and resolves ONLY via a provider
	// query, never by re-queueing into the normal dispatch path.
	ErrDispatchUnknown = errors.New("dispatch_unknown")
	// ErrActionRePrepareRequired: the row's digest was computed under the
	// legacy generation 1 (or an unknown one), whose material did not bind
	// the action identity, risk or OC execution fields. Old pending or
	// authorized v1 actions must be re-Prepared under the current digest
	// generation — an old approval is never auto-upgraded.
	ErrActionRePrepareRequired = errors.New("action_reprepare_required")
	// ErrNoDispatcher: Execute fails closed when no dispatcher is wired
	// (T04-QF-6 carry): the container injects a nil dispatcher in phase
	// one, and executing an approved action without the outbound boundary
	// must be a clean refusal — never a panic, never a consumed approval.
	ErrNoDispatcher = errors.New("no_dispatcher_configured")
	// ErrDispatchNotStarted proves a dispatch was rejected BEFORE any
	// network call left the process (T11): only such provably unstarted
	// rejections may settle as failed; every other dispatch error leaves the
	// provider outcome unknown. It is raised exclusively by pre-send gates —
	// nil/incomplete dispatcher wiring, a missing durable claim record,
	// snapshot/binding drift or a token failure — never around a call that
	// may already have reached the provider.
	ErrDispatchNotStarted = errors.New("dispatch_not_started")
	// ErrOCClaimsNotWired documents the transitional wiring contract: the
	// durable open-connector claim (key, fence, revocation serialization)
	// is not wired, so OC dispatches take the frozen claim path and leave
	// no durable idempotency record. Reserved for wiring diagnostics; the
	// container gains the claim store with the OC runtime tasks.
	ErrOCClaimsNotWired = errors.New("oc_claims_not_wired")
)

// ocSlotLeaseTTL bounds one dispatch's slot leases: a replica that crashes
// mid-dispatch simply expires and the slot is reclaimed; no release path
// ever re-sends a dispatched action.
const ocSlotLeaseTTL = 90 * time.Second

// ocDispatchClientDeadline bounds the outbound dispatch call (T12): a hung
// provider call converges to unknown under the bounded context (30s client
// deadline), strictly inside the 90s no-heartbeat staleness bound of the
// recovery sweep, so a LIVE dispatch sequence can never be mistaken for a
// crashed worker.
const ocDispatchClientDeadline = 30 * time.Second

// OCDispatchClaimSource is the durable open-connector claim surface
// (T10, implemented by repository/appconnector.OCStore): the atomic claim
// that consumes the approval, mints the operation-scoped key and serializes
// against revocations in ONE transaction.
type OCDispatchClaimSource interface {
	ClaimOCDispatch(ctx context.Context, subject appconn.OCSubject, actionID, reservationID string) (appconn.OCDispatchRecord, error)
	GetOCDispatch(ctx context.Context, tenant uint64, actionID string) (appconn.OCDispatchRecord, error)
}

// ActionSnapshot is the exact, already-approved payload of an action. Args
// are the NORMALIZED bytes persisted at Prepare time — the dispatch path
// sends these bytes and nothing else; approve-then-rewrite is impossible.
type ActionSnapshot struct {
	ID           string
	TenantID     uint64
	ActorID      string
	ConnectionID string
	Version      string
	Target       string
	Risk         string
	Digest       string
	State        string
	Fence        int64
	Args         []byte
	// AuthVersion, OC and DigestVersion carry the T09 full approval
	// snapshot: the connection's permission generation, the immutable
	// open-connector execution binding (nil for native actions), and the
	// digest generation the row's digest was computed under.
	AuthVersion   int64
	OC            *appconn.OCExecutionBinding
	DigestVersion int
}

// DispatchOutcome is the provider result of one dispatch attempt. Status
// must be one of succeeded/failed/unknown; "unknown" means the outcome
// could not be observed (crash window) and parks the action for a provider
// query instead of a silent success or failure. ExecutionID carries the
// provider's meta.executionId when the envelope exposed one (T13 R15
// additive extension): it lands on the durable dispatch record for
// correlation; the classification itself never depends on it.
type DispatchOutcome struct {
	Status         string
	ProviderResult string
	ExecutionID    string
}

// ActionDispatcher performs the real outbound call with the snapshot's
// normalized args. It is the only external boundary of the pipeline.
type ActionDispatcher interface {
	Dispatch(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error)
}

// UnknownResolver queries the PROVIDER (never the local queue) to resolve
// an action parked in unknown.
type UnknownResolver interface {
	QueryProvider(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error)
}

// A02Guard re-checks, on EVERY execute, that the PERSISTED subject may
// still use the connection: subject shape, tenant scope, strict
// auth_version, live membership, active installation, explicit grant for
// space connections and (for open-connector connections) an active
// tenant-scoped binding. It NEVER loads credential material — the native
// branch resolves credentials only after this check passes, and the OC
// branch never resolves them at all. It is satisfied by the package's
// OCAuthorizer via NewA02Guard/NewSubjectGuard.
type A02Guard interface {
	Check(ctx context.Context, subject appconn.OCSubject, connectionID string, expectedVersion int64) error
}

// ActionStoreSource is the persistence surface the service needs; it is
// implemented by repository/appconnector.ActionStore.
type ActionStoreSource interface {
	CreateAction(ctx context.Context, a appconn.Action, snapshot, digest, state string) error
	FindAction(ctx context.Context, id string) (repoappconn.ActionRow, error)
	SaveApproval(ctx context.Context, actionID, digest, actor string, expiry time.Time, remaining int64) error
	SetActionState(ctx context.Context, id, from, to string) error
	ClaimDispatch(ctx context.Context, id, providerKey, reservationID string) error
	FinishDispatch(ctx context.Context, id string, fence int64, state, result string) error
	FinishUnknown(ctx context.Context, id string, fence int64, state, result string) error
	SavePreAuthorization(ctx context.Context, p appconn.PreAuthorization) error
	ListPreAuthorizations(ctx context.Context, tenant uint64) ([]appconn.PreAuthorization, error)
}

// ActionService drives the persisted approval pipeline: Prepare snapshots
// and digests the exact call, Approve binds a human decision to that
// digest, Execute re-checks A02 + U05 and then consumes the approval and
// writes the dispatch intent in ONE transaction before the real call.
// The in-memory approval Gate (if wired at the edge) is display/wake only —
// this store, not any memory map, is the authority.
type ActionService struct {
	store        ActionStoreSource
	guard        A02Guard
	gate         commercial.ExecutionGate // U05; nil preserves non-commercial behavior
	dispatcher   ActionDispatcher
	unknown      UnknownResolver
	approvalTTL  time.Duration
	budgetUpper  commercial.Credits
	budgetWindow time.Duration
	// dispatchDeadline bounds the outbound dispatch call (T12 recovery hook);
	// defaults to ocDispatchClientDeadline, overridable for tests.
	dispatchDeadline time.Duration
	// ocClaims is the T10 durable claim store for open-connector actions;
	// slots is the optional distributed dispatch limiter. Both are wired
	// additively (nil = native behavior preserved) so existing faces stay
	// frozen.
	ocClaims OCDispatchClaimSource
	slots    *OCSlotLimiter
}

// UseOCDispatchClaims wires the durable open-connector claim store: with
// it, OC dispatches claim atomically (key, fence, revocation serialization
// in one transaction) and leave a durable idempotency record; without it the
// frozen ClaimDispatch path keeps serving the transitional wiring.
func (s *ActionService) UseOCDispatchClaims(src OCDispatchClaimSource) { s.ocClaims = src }

// UseOCSlotLimiter wires the distributed dispatch slot limiter. Without it
// dispatches are unlimited by default (native behavior preserved).
func (s *ActionService) UseOCSlotLimiter(l *OCSlotLimiter) { s.slots = l }

// NewActionService builds the service. gate/dispatcher/unknown may be nil
// (tests inject stubs); store and guard are required.
func NewActionService(
	store ActionStoreSource, guard A02Guard, gate commercial.ExecutionGate,
	dispatcher ActionDispatcher, unknown UnknownResolver,
) *ActionService {
	return &ActionService{
		store:            store,
		guard:            guard,
		gate:             gate,
		dispatcher:       dispatcher,
		unknown:          unknown,
		approvalTTL:      30 * time.Minute,
		budgetUpper:      1,
		budgetWindow:     10 * time.Minute,
		dispatchDeadline: ocDispatchClientDeadline,
	}
}

// Prepare normalizes the args, digests the exact bound call, and persists
// it as awaiting_approval. When a scope pre-authorization covers the action
// (its OWN risk category scope — a general write grant never applies here),
// the action is born authorized with a digest-bound approval whose expiry
// is the pre-authorization's validity window. Modified args later simply
// mean a NEW Prepare with a NEW digest: the old approval never carries over.
func (s *ActionService) Prepare(ctx context.Context, a appconn.Action) (string, error) {
	if a.TenantID == 0 || a.ActorID == "" || a.ConnectionID == "" || a.Target == "" || a.Risk == "" {
		return "", ErrInvalidAction
	}
	// Ruling 4: OC execution fields are filled ONLY server-side by
	// PrepareOC (from the tenant's live binding and the reviewed
	// definition). A client-supplied runtime/alias/external identity
	// arriving through the native path is rejected outright.
	if a.OC != nil {
		return "", fmt.Errorf("%w: client-supplied open-connector binding", ErrInvalidAction)
	}
	if a.ID == "" {
		a.ID = "act_" + uuid.NewString()
	}
	// New writes always carry the current digest generation; the migration
	// default 1 belongs to pre-existing rows only.
	a.DigestVersion = appconn.CurrentDigestVersion
	return s.persistPrepared(ctx, a)
}

// persistPrepared normalizes, digests and persists one fully-built action
// (oc_binding_json, args snapshot and digest land in the row's single
// INSERT), then applies any covering scope pre-authorization. It is the
// shared tail of the native Prepare path and of PrepareOC (oc_prepare.go),
// which builds its Action exclusively from server-side sources.
func (s *ActionService) persistPrepared(ctx context.Context, a appconn.Action) (string, error) {
	norm, err := appconn.NormalizeArgs(a.Args)
	if err != nil {
		return "", err
	}
	digest, err := appconn.ActionDigest(a)
	if err != nil {
		return "", err
	}
	if err := s.store.CreateAction(ctx, a, string(norm), digest, appconn.ActionAwaitingApproval); err != nil {
		return "", err
	}
	if pre, ok, perr := s.matchPreAuthorization(ctx, a, time.Now()); perr != nil {
		return "", perr
	} else if ok {
		if aerr := s.store.SaveApproval(ctx, a.ID, digest, "preauth:"+pre.ID, pre.ValidUntil, 1); aerr != nil {
			return "", aerr
		}
		if serr := s.store.SetActionState(ctx, a.ID, appconn.ActionAwaitingApproval, appconn.ActionAuthorized); serr != nil {
			return "", serr
		}
	}
	return a.ID, nil
}

// matchPreAuthorization finds the tenant pre-authorization covering this
// action under the domain's own-scope rules (a send pre-auth matches send
// only; a general write grant never satisfies send).
func (s *ActionService) matchPreAuthorization(ctx context.Context, a appconn.Action, now time.Time) (appconn.PreAuthorization, bool, error) {
	pres, err := s.store.ListPreAuthorizations(ctx, a.TenantID)
	if err != nil {
		return appconn.PreAuthorization{}, false, err
	}
	for _, p := range pres {
		if appconn.PreAuthorizationCovers(p, a, now) {
			return p, true, nil
		}
	}
	return appconn.PreAuthorization{}, false, nil
}

// Approve binds a human decision to the action's CURRENT digest. A digest
// mismatch means the decision was issued for different (old) args and is
// refused — modified args must go through a new snapshot + new digest.
func (s *ActionService) Approve(ctx context.Context, id, actor, digest string) error {
	row, err := s.store.FindAction(ctx, id)
	if err != nil {
		return err
	}
	if actor == "" {
		return fmt.Errorf("%w: empty actor", ErrInvalidAction)
	}
	// Ruling 3: legacy-generation rows never continue an old approval —
	// their digest material did not bind the action identity, risk or OC
	// fields, so the row must be re-Prepared under the current generation.
	if row.DigestVersion != appconn.CurrentDigestVersion {
		return fmt.Errorf("%w: digest generation %d", ErrActionRePrepareRequired, row.DigestVersion)
	}
	if digest != row.ArgsDigest {
		return ErrActionDigestMismatch
	}
	if row.State != appconn.ActionAwaitingApproval && row.State != appconn.ActionAuthorized {
		return fmt.Errorf("approve from %s: %w", row.State, ErrActionState)
	}
	if err := s.store.SaveApproval(ctx, id, digest, actor, time.Now().Add(s.approvalTTL), 1); err != nil {
		return err
	}
	if row.State == appconn.ActionAwaitingApproval {
		return s.store.SetActionState(ctx, id, appconn.ActionAwaitingApproval, appconn.ActionAuthorized)
	}
	return nil
}

// Execute runs the full pre-dispatch chain and, only when everything
// passes, consumes the approval and writes the dispatch intent in ONE
// transaction, then performs the real call with the persisted normalized
// bytes. On every execute the A02 connection guard and the U05 budget gate
// are re-checked — a fee/budget pass authorizes spending, never writes.
func (s *ActionService) Execute(ctx context.Context, id string) error {
	// Fail closed BEFORE consuming anything: no dispatcher wired means no
	// dispatch can ever happen — refuse instead of panicking at the call
	// (T04-QF-6; plan fact: the container injects a nil dispatcher).
	if s.dispatcher == nil {
		return fmt.Errorf("%w: execute refused, action %s untouched", ErrNoDispatcher, id)
	}
	row, err := s.store.FindAction(ctx, id)
	if err != nil {
		return err
	}
	if row.State != appconn.ActionAuthorized {
		return fmt.Errorf("execute from %s: %w", row.State, ErrActionState)
	}
	// Ruling 3: an old authorized v1 action must re-Prepare — its legacy
	// digest did not bind the full execution identity, so executing it on
	// the strength of an old approval is refused (no auto-upgrade).
	if row.DigestVersion != appconn.CurrentDigestVersion {
		return fmt.Errorf("%w: digest generation %d", ErrActionRePrepareRequired, row.DigestVersion)
	}
	// A02 re-check: the PERSISTED row's subject may still use the
	// connection (tenant scope, state, strict auth_version, membership,
	// installation, grant, OC binding). A permission revoked between
	// approval and execute blocks dispatch here, before any reservation or
	// intent is written. The subject is always built from the persisted
	// row's tenant/actor — never from the calling operator, and never a
	// synthetic admin identity, so background or recovery callers cannot
	// bypass a member's revocation.
	if err := s.guard.Check(ctx,
		appconn.OCSubject{TenantID: row.TenantID, ActorID: row.ActorID},
		row.ConnectionID, row.AuthVersion,
	); err != nil {
		return fmt.Errorf("a02 recheck: %w", err)
	}
	// Open-connector actions prefer the durable claim (key, fence,
	// revocation serialization). While the claim store is not wired the
	// frozen ClaimDispatch path keeps serving — the nil-dispatcher gate
	// above already blocks every real outbound dispatch in that wiring,
	// and the durable record appears the moment the store is wired in.
	oc, ocerr := ocBindingOf(row)
	if ocerr != nil {
		return ocerr
	}
	// T12 recovery hook: when the wired claim store also carries the settle
	// face (the repository OCStore does), the durable dispatch record is
	// settled FIRST below — it is the local linearization point and the
	// settlement outbox in one row. A claim source without the face (the
	// frozen stub wiring) keeps the pre-T12 finish order verbatim.
	var ocSettle OCDispatchSettleSource
	if oc != nil && s.ocClaims != nil {
		ocSettle, _ = s.ocClaims.(OCDispatchSettleSource)
	}
	var ocRec appconn.OCDispatchRecord
	// Dispatch slots BEFORE the budget Begin (ruling 5): a request that
	// cannot get a slot consumes no approval and no budget. The slots are
	// DATABASE leases — a replica crash simply expires them; releasing a
	// slot never re-sends a dispatched action.
	var releaseSlots func(context.Context) error
	if oc != nil && s.slots != nil {
		rel, serr := s.slots.AcquireOCSlots(ctx, row.TenantID, row.ConnectionID, oc.Provider, row.ID, time.Now().Add(ocSlotLeaseTTL))
		if serr != nil {
			return fmt.Errorf("oc slots: %w", serr)
		}
		releaseSlots = rel
	}
	defer func() {
		if releaseSlots != nil {
			// Best-effort release on every exit path; the lease TTL is the
			// backstop if this context is already dead.
			_ = releaseSlots(context.Background())
		}
	}()
	// U05 budget gate: Begin BEFORE the outbound call; denial is a hard
	// stop. This reserves fees only — it is never a write approval. The
	// key is the stable Action ID, so racing requests on one action share
	// ONE reservation (T10 ruling 5).
	var reservationID string
	if s.gate != nil {
		res, err := s.gate.Begin(ctx, commercial.BudgetRequest{
			TenantID: row.TenantID,
			RunID:    "appaction:" + row.ID,
			Key:      row.ID,
			Upper:    s.budgetUpper,
			Deadline: time.Now().Add(s.budgetWindow),
		})
		if err != nil {
			return fmt.Errorf("budget gate: %w", err)
		}
		reservationID = res.ID
	}
	// Consume the approval count and write the dispatch intent atomically.
	// A lost concurrency race aborts here with no over-consumption; a crash
	// before this commit leaves the action authorized with the count intact.
	// Open-connector actions with a wired claim store go through the
	// durable claim (operation-scoped key, connection-lock serialization
	// with revocations, loser reads the winner's reservation); native
	// actions — and OC actions in the unwired transitional wiring — keep
	// the frozen path.
	if oc != nil && s.ocClaims != nil {
		rec, cerr := s.ocClaims.ClaimOCDispatch(ctx,
			appconn.OCSubject{TenantID: row.TenantID, ActorID: row.ActorID},
			row.ID, reservationID,
		)
		if cerr != nil {
			// Lost race or blocked claim: the winner (if any) holds the
			// SHARED reservation — the loser must never cancel or settle
			// it. A claim rejected while the row is still authorized is
			// provably usage-free (the outbound call never happened), so
			// its pre-allocation is settled with ZERO usage here instead of
			// lingering as an orphan hold (T10-Q-1).
			return s.withClaimOrphanRelease(ctx, id, row.TenantID, cerr, reservationID)
		}
		ocRec = rec
	} else if err := s.store.ClaimDispatch(ctx, id, row.ConnectionID, reservationID); err != nil {
		return s.withClaimOrphanRelease(ctx, id, row.TenantID, err, reservationID)
	}
	claimed, err := s.store.FindAction(ctx, id)
	if err != nil {
		return err
	}
	snap, err := snapshotOf(claimed)
	if err != nil {
		return err
	}
	// T12: the outbound call runs under the bounded client deadline — a hung
	// provider call converges to unknown instead of pinning the action, and
	// stays strictly inside the 90s staleness bound of the recovery sweep.
	dctx, cancel := context.WithTimeout(ctx, s.dispatchDeadline)
	out, derr := s.dispatcher.Dispatch(dctx, snap, claimed.ProviderKey)
	cancel()
	final := settleOutcome(out, derr)
	if ocSettle != nil {
		// T12 recovery hook: settle the DURABLE RECORD first (terminal result
		// + settlement intent in ONE row), then the action row, then deliver
		// the settlement. The T11 classification above is untouched.
		if err := s.settleOCDispatchRecord(ctx, ocSettle, ocRec, claimed, final); err != nil {
			return err
		}
	} else {
		if err := s.store.FinishDispatch(ctx, id, claimed.Fence, final.Status, final.ProviderResult); err != nil {
			return err
		}
		if s.gate != nil && reservationID != "" && final.Status != appconn.ActionUnknown {
			if err := s.gate.Finish(ctx, reservationID, commercial.UsageFact{
				TenantID: claimed.TenantID, RunID: "appaction:" + claimed.ID,
				CallID: claimed.ID, AttemptID: claimed.ID, Funding: commercial.FundingPlatform,
				Service: commercial.ServiceConnector, PriceVersion: "v1", Revision: 1,
				OccurredAt: time.Now().UTC(),
				Dimensions: map[string]int64{commercial.DimensionConnector: 1},
				Status:     commercial.UsageStatusFinal,
			}); err != nil {
				return err
			}
		}
	}
	if final.Status == appconn.ActionUnknown {
		return ErrDispatchUnknown
	}
	return nil
}

// withClaimOrphanRelease settles the budget pre-allocation of a REJECTED
// claim with zero usage (T10-Q-1) and returns the claim error. The release
// fires only when the claim error proves the dispatch never started AND the
// row is still authorized — i.e. no winner exists for the Action-ID-stable
// shared reservation:
//
//   - ErrApprovalExhausted / ErrOCDispatchConflict leave the row authorized
//     (the claim transaction rejected before any state move), so the
//     reservation is provably usage-free and settles at zero;
//   - ErrOCDispatchClaimed (lost race) or any conflict where the row has
//     already moved to dispatched means a LIVE winner owns the shared
//     reservation — the loser must never cancel or settle it.
//
// A crash between Begin and claim still leaves an orphan for the commercial
// recovery; this path only covers rejections observable in-process.
func (s *ActionService) withClaimOrphanRelease(ctx context.Context, id string, tenant uint64, claimErr error, reservationID string) error {
	if s.gate == nil || reservationID == "" {
		return claimErr
	}
	if !errors.Is(claimErr, repoappconn.ErrApprovalExhausted) && !errors.Is(claimErr, repoappconn.ErrOCDispatchConflict) {
		// A lost race (or an ambiguous conflict): a winner may own the
		// shared reservation — never settle it from the losing request.
		return claimErr
	}
	current, err := s.store.FindAction(ctx, id)
	if err != nil {
		// The winnerless proof is unavailable: keep the hold (fail-safe
		// over-hold) and surface the claim error alongside the proof
		// failure.
		return errors.Join(claimErr, err)
	}
	if current.State != appconn.ActionAuthorized {
		// Another request's claim went through: the reservation has a
		// winner — leave it to the winner's own settlement.
		return claimErr
	}
	// Zero-usage settle: the commercial gate exposes no Cancel, and a final
	// fact of zero connector usage settles the reservation with no charge —
	// the orphan hold disappears.
	release := s.gate.Finish(context.Background(), reservationID, commercial.UsageFact{
		TenantID: tenant, RunID: "appaction:" + id,
		CallID: id, AttemptID: id, Funding: commercial.FundingPlatform,
		Service: commercial.ServiceConnector, PriceVersion: "v1", Revision: 1,
		OccurredAt: time.Now().UTC(),
		Dimensions: map[string]int64{commercial.DimensionConnector: 0},
		Status:     commercial.UsageStatusFinal,
	})
	if release != nil {
		return errors.Join(claimErr, release)
	}
	return claimErr
}

// ocBindingOf parses a row's immutable open-connector execution binding;
// nil means the action is native. A corrupt binding is a hard error — an OC
// action must never dispatch with a silently dropped binding.
func ocBindingOf(row repoappconn.ActionRow) (*appconn.OCExecutionBinding, error) {
	if row.OCBindingJSON == "" {
		return nil, nil
	}
	var b appconn.OCExecutionBinding
	if err := json.Unmarshal([]byte(row.OCBindingJSON), &b); err != nil {
		return nil, fmt.Errorf("corrupt oc_binding_json for %s: %w", row.ID, err)
	}
	return &b, nil
}

// snapshotOf rebuilds the full approval snapshot from a persisted row,
// including the connection's auth version, the digest generation and the
// parsed open-connector execution binding. A corrupt oc_binding_json is a
// hard error — an OC action must never dispatch with a silently dropped
// binding.
func snapshotOf(row repoappconn.ActionRow) (ActionSnapshot, error) {
	var oc *appconn.OCExecutionBinding
	if row.OCBindingJSON != "" {
		var b appconn.OCExecutionBinding
		if err := json.Unmarshal([]byte(row.OCBindingJSON), &b); err != nil {
			return ActionSnapshot{}, fmt.Errorf("corrupt oc_binding_json for %s: %w", row.ID, err)
		}
		oc = &b
	}
	return ActionSnapshot{
		ID: row.ID, TenantID: row.TenantID, ActorID: row.ActorID,
		ConnectionID: row.ConnectionID, Version: row.AppVersion,
		Target: row.Target, Risk: row.Risk, Digest: row.ArgsDigest,
		State: row.State, Fence: row.Fence, Args: []byte(row.ArgsSnapshot),
		AuthVersion: row.AuthVersion, OC: oc, DigestVersion: int(row.DigestVersion),
	}, nil
}

// settleOutcome maps a dispatcher result to a terminal state. The only
// error that may settle as FAILED is a proven pre-send rejection
// (ErrDispatchNotStarted — no network call ever left the process); every
// other dispatch error leaves the provider outcome UNKNOWN, because the
// request may already have reached the provider and produced side effects.
// Unknown outcomes park the action for provider-query resolution — they are
// never re-queued into the dispatch path.
func settleOutcome(out DispatchOutcome, derr error) DispatchOutcome {
	if derr != nil {
		if errors.Is(derr, ErrDispatchNotStarted) {
			return DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: "dispatch rejected before send"}
		}
		return DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: "provider outcome unavailable"}
	}
	switch out.Status {
	case appconn.ActionSucceeded, appconn.ActionFailed, appconn.ActionUnknown:
		return out
	default:
		return DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: "invalid provider outcome"}
	}
}

// ResolveUnknown resolves an action parked in unknown by querying the
// PROVIDER — it never re-queues the action into the normal dispatch path.
func (s *ActionService) ResolveUnknown(ctx context.Context, id string) error {
	row, err := s.store.FindAction(ctx, id)
	if err != nil {
		return err
	}
	if row.State != appconn.ActionUnknown {
		return fmt.Errorf("resolve from %s: %w", row.State, ErrActionState)
	}
	if s.unknown == nil {
		return fmt.Errorf("%w: no provider query configured", ErrDispatchUnknown)
	}
	snap, serr := snapshotOf(row)
	if serr != nil {
		return serr
	}
	out, qerr := s.unknown.QueryProvider(ctx, snap, row.ProviderKey)
	if qerr != nil {
		return qerr
	}
	switch out.Status {
	case appconn.ActionSucceeded, appconn.ActionFailed:
		return s.finishUnknownSettled(ctx, row, out.Status, out.ProviderResult)
	}
	return fmt.Errorf("%w: provider query not terminal: %s", ErrDispatchUnknown, out.Status)
}

// OCDispatchSettleSource is the T12 recovery-side extension of the claim
// source: settling the durable dispatch record under its fence CAS and
// marking the settlement delivery. It is satisfied by
// repository/appconnector.OCStore; a claim source without it (the frozen
// stub wiring) keeps the pre-T12 finish order verbatim.
type OCDispatchSettleSource interface {
	FinishOCDispatch(ctx context.Context, tenant uint64, actionID string, fence int64, from, to, executionID string) error
	MarkOCDispatchSettled(ctx context.Context, tenant uint64, actionID string, fence int64, now time.Time) error
}

// ocDispatchSettleOf returns the settle face of the service's wired claim
// source, or nil when the wiring does not carry it.
func ocDispatchSettleOf(s *ActionService) OCDispatchSettleSource {
	if s == nil || s.ocClaims == nil {
		return nil
	}
	src, _ := s.ocClaims.(OCDispatchSettleSource)
	return src
}

// settleOCDispatchRecord is the T12 finish path for open-connector dispatches
// under the recovery settle face. Order (record first — it is the local
// linearization point and the settlement outbox in one row):
//
//  1. dispatched -> final.Status on the RECORD, with a bounded retry that
//     persists the ORIGINAL outcome (ruling 4: a local write failure never
//     re-derives and never re-sends; exhaustion leaves both rows dispatched
//     for the stale sweep to park unknown);
//  2. dispatched -> final.Status on the ACTION row;
//  3. terminal outcomes deliver the settlement with the stable fact
//     (Revision=1); unknown keeps the hold — unknown is not free. A delivery
//     failure returns an error while the intent stays durable: OCRecovery
//     retries the settlement ALONE.
//
// If the recovery sweep won the record race meanwhile, the late outcome does
// not overwrite the parked unknown: the action row is parked unknown too and
// the provider query decides.
func (s *ActionService) settleOCDispatchRecord(ctx context.Context, src OCDispatchSettleSource, rec appconn.OCDispatchRecord, claimed repoappconn.ActionRow, final DispatchOutcome) error {
	// 1. Record (linearization point). The provider's execution id (when the
	// envelope exposed one) is persisted for correlation; the outcome
	// classification remains authoritative and never depends on it.
	rerr := ocRetry(func() error {
		octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
		defer cancel()
		return src.FinishOCDispatch(octx, rec.TenantID, rec.ActionID, rec.Fence,
			appconn.ActionDispatched, final.Status, final.ExecutionID)
	})
	if rerr != nil {
		if errors.Is(rerr, repoappconn.ErrOCDispatchConflict) {
			if aerr := s.recoverAlignAction(ctx, claimed.ID, appconn.ActionUnknown,
				"dispatch record swept to unknown; provider query required"); aerr != nil {
				return errors.Join(aerr, ErrDispatchUnknown)
			}
			return fmt.Errorf("%w: dispatch record swept to unknown during finish", ErrDispatchUnknown)
		}
		return rerr
	}
	// 2. Action row follows the record; a crash here is repaired by
	// OCRecovery's alignment pass.
	if err := ocRetry(func() error {
		octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
		defer cancel()
		return s.store.FinishDispatch(octx, claimed.ID, claimed.Fence, final.Status, final.ProviderResult)
	}); err != nil {
		if errors.Is(err, repoappconn.ErrActionState) {
			// The action row already moved (a concurrent writer); the record
			// is the authority — leave alignment to the loop.
			return nil
		}
		return err
	}
	// 3. Settlement delivery.
	if final.Status == appconn.ActionUnknown {
		return nil
	}
	if derr := deliverOCSettlement(ctx, s.gate, rec.TenantID, rec.ActionID, rec.ReservationID); derr != nil {
		return derr
	}
	// Best-effort delivery mark: a failure leaves the row due, and the loop
	// re-delivers the IDENTICAL fact (idempotent at the ledger) before
	// marking again.
	octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
	defer cancel()
	_ = src.MarkOCDispatchSettled(octx, rec.TenantID, rec.ActionID, rec.Fence, time.Now().UTC())
	return nil
}

// recoverAlignAction moves an action row into the state its durable dispatch
// record already holds, under the row's current fence — the recovery-side
// writer for the sweep and the alignment pass. A row that already agrees (or
// was moved on by a concurrent writer) is a no-op; terminal action rows are
// never rewritten by recovery.
func (s *ActionService) recoverAlignAction(ctx context.Context, actionID, target, note string) error {
	octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
	defer cancel()
	row, err := s.store.FindAction(octx, actionID)
	if err != nil {
		return fmt.Errorf("align %s: %w", actionID, err)
	}
	if row.State == target || (row.State != appconn.ActionDispatched && row.State != appconn.ActionUnknown) {
		return nil
	}
	octx2, cancel2 := context.WithTimeout(ctx, ocRecoveryOpTimeout)
	defer cancel2()
	if row.State == appconn.ActionDispatched {
		if err := s.store.FinishDispatch(octx2, actionID, row.Fence, target, note); err != nil {
			if errors.Is(err, repoappconn.ErrActionState) {
				return nil // a concurrent writer moved it first
			}
			return fmt.Errorf("align %s: %w", actionID, err)
		}
		return nil
	}
	if target == appconn.ActionUnknown {
		return nil
	}
	if err := s.store.FinishUnknown(octx2, actionID, row.Fence, target, note); err != nil {
		if errors.Is(err, repoappconn.ErrActionState) {
			return nil
		}
		return fmt.Errorf("align %s: %w", actionID, err)
	}
	return nil
}

// finishUnknownFromRecord settles an action parked in unknown into the
// terminal state its durable record already holds, then delivers the
// settlement. It backs the loop's alignment pass for records resolved
// terminally by a worker that crashed before writing the action row. The
// bool reports whether the action row moved.
func (s *ActionService) finishUnknownFromRecord(ctx context.Context, rec appconn.OCDispatchRecord, actionFence int64) (bool, error) {
	octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
	defer cancel()
	if err := s.store.FinishUnknown(octx, rec.ActionID, actionFence, rec.State,
		"recovered: terminal outcome restored from dispatch record"); err != nil {
		if errors.Is(err, repoappconn.ErrActionState) {
			return false, nil
		}
		return false, err
	}
	if derr := deliverOCSettlement(ctx, s.gate, rec.TenantID, rec.ActionID, rec.ReservationID); derr != nil {
		return true, derr
	}
	// Mark the delivery so the SAME RunOnce's settlement pass does not
	// re-deliver (QF-2); a mark failure only costs one idempotent replay on
	// the next pass.
	if src := ocDispatchSettleOf(s); src != nil {
		mctx, mcancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
		defer mcancel()
		_ = src.MarkOCDispatchSettled(mctx, rec.TenantID, rec.ActionID, rec.Fence, time.Now().UTC())
	}
	return true, nil
}

// finishUnknownSettled is the T12 ResolveUnknown tail (precheck fact 6
// closed): an unknown->terminal transition first settles the DURABLE RECORD
// under its fence CAS — the terminal result and the settlement intent land
// in ONE row — then the action row, then the settlement is delivered with
// the stable fact. A settlement failure surfaces as an error while the
// terminal writes stay durable; OCRecovery retries the settlement alone.
// Actions without a durable record (native path) settle their recorded
// pre-allocation directly.
func (s *ActionService) finishUnknownSettled(ctx context.Context, row repoappconn.ActionRow, state, result string) error {
	if src := ocDispatchSettleOf(s); src != nil {
		octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
		rec, gerr := s.ocClaims.GetOCDispatch(octx, row.TenantID, row.ID)
		cancel()
		if gerr == nil && rec.ActionID != "" {
			ferr := ocRetry(func() error {
				octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
				defer cancel()
				return src.FinishOCDispatch(octx, rec.TenantID, rec.ActionID, rec.Fence,
					appconn.ActionUnknown, state, "")
			})
			if ferr != nil {
				if errors.Is(ferr, repoappconn.ErrOCDispatchConflict) {
					// Another resolver already moved the record; its own
					// delivery owns this reservation.
					return fmt.Errorf("resolve record %s: %w", row.ID, ferr)
				}
				return ferr
			}
			if aerr := ocRetry(func() error {
				octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
				defer cancel()
				return s.store.FinishUnknown(octx, row.ID, row.Fence, state, result)
			}); aerr != nil {
				if errors.Is(aerr, repoappconn.ErrActionState) {
					return nil // a concurrent writer moved the action row
				}
				return aerr // record terminal + action unknown: loop aligns + settles
			}
			if derr := deliverOCSettlement(ctx, s.gate, rec.TenantID, rec.ActionID, rec.ReservationID); derr != nil {
				return derr
			}
			octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
			defer cancel()
			_ = src.MarkOCDispatchSettled(octx, rec.TenantID, rec.ActionID, rec.Fence, time.Now().UTC())
			return nil
		}
		// No durable record (mixed wiring): fall through to the direct
		// action-row settlement below.
	}
	if err := ocRetry(func() error {
		octx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
		defer cancel()
		return s.store.FinishUnknown(octx, row.ID, row.Fence, state, result)
	}); err != nil {
		return err
	}
	return deliverOCSettlement(ctx, s.gate, row.TenantID, row.ID, row.ReservationID)
}
