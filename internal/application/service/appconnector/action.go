package appconnector

import (
	"context"
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
)

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
}

// DispatchOutcome is the provider result of one dispatch attempt. Status
// must be one of succeeded/failed/unknown; "unknown" means the outcome
// could not be observed (crash window) and parks the action for a provider
// query instead of a silent success or failure.
type DispatchOutcome struct {
	Status         string
	ProviderResult string
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

// A02Guard re-checks, on EVERY execute, that the connection is still
// usable: active state, unchanged auth_version, and live owner membership.
// It is satisfied by the package's CredentialResolver.
type A02Guard interface {
	Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error)
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
}

// NewActionService builds the service. gate/dispatcher/unknown may be nil
// (tests inject stubs); store and guard are required.
func NewActionService(
	store ActionStoreSource, guard A02Guard, gate commercial.ExecutionGate,
	dispatcher ActionDispatcher, unknown UnknownResolver,
) *ActionService {
	return &ActionService{
		store:        store,
		guard:        guard,
		gate:         gate,
		dispatcher:   dispatcher,
		unknown:      unknown,
		approvalTTL:  30 * time.Minute,
		budgetUpper:  1,
		budgetWindow: 10 * time.Minute,
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
	if a.ID == "" {
		a.ID = "act_" + uuid.NewString()
	}
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
	row, err := s.store.FindAction(ctx, id)
	if err != nil {
		return err
	}
	if row.State != appconn.ActionAuthorized {
		return fmt.Errorf("execute from %s: %w", row.State, ErrActionState)
	}
	// A02 re-check: connection still active, auth_version unchanged, owner
	// still a member. A permission revoked between approval and execute
	// blocks dispatch here, before any reservation or intent is written.
	if _, err := s.guard.Resolve(ctx, row.ConnectionID, row.AuthVersion); err != nil {
		return fmt.Errorf("a02 recheck: %w", err)
	}
	// U05 budget gate: Begin BEFORE the outbound call; denial is a hard
	// stop. This reserves fees only — it is never a write approval.
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
	if err := s.store.ClaimDispatch(ctx, id, row.ConnectionID, reservationID); err != nil {
		return err
	}
	claimed, err := s.store.FindAction(ctx, id)
	if err != nil {
		return err
	}
	snap := snapshotOf(claimed)
	out, derr := s.dispatcher.Dispatch(ctx, snap, claimed.ProviderKey)
	final := settleOutcome(out, derr)
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
	if final.Status == appconn.ActionUnknown {
		return ErrDispatchUnknown
	}
	return nil
}

func snapshotOf(row repoappconn.ActionRow) ActionSnapshot {
	return ActionSnapshot{
		ID: row.ID, TenantID: row.TenantID, ActorID: row.ActorID,
		ConnectionID: row.ConnectionID, Version: row.AppVersion,
		Target: row.Target, Risk: row.Risk, Digest: row.ArgsDigest,
		State: row.State, Fence: row.Fence, Args: []byte(row.ArgsSnapshot),
	}
}

// settleOutcome maps a dispatcher result to a terminal state; unknown
// outcomes park the action for provider-query resolution.
func settleOutcome(out DispatchOutcome, derr error) DispatchOutcome {
	if derr != nil {
		return DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: derr.Error()}
	}
	switch out.Status {
	case appconn.ActionSucceeded, appconn.ActionFailed, appconn.ActionUnknown:
		return out
	default:
		return DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: "invalid dispatch outcome"}
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
	out, qerr := s.unknown.QueryProvider(ctx, snapshotOf(row), row.ProviderKey)
	if qerr != nil {
		return qerr
	}
	switch out.Status {
	case appconn.ActionSucceeded, appconn.ActionFailed:
		return s.store.FinishUnknown(ctx, id, row.Fence, out.Status, out.ProviderResult)
	}
	return fmt.Errorf("%w: provider query not terminal: %s", ErrDispatchUnknown, out.Status)
}
