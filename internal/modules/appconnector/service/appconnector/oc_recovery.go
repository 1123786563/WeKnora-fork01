// oc_recovery.go implements T12: crash, unknown and settlement recovery for
// open-connector dispatches.
//
// DESIGN (coordinator rulings 1-8):
//
//   - ReplayAllowed is EXACTLY the plan sketch: a conservative permission
//     predicate over the ORIGINAL deadline. Phase one grants it to NO actor —
//     there is no automatic POST replay anywhere in this file; unknown never
//     re-enters Execute. Widening replay into an active retry requires its
//     own review.
//   - Stale dispatched records converge to unknown through the existing
//     fence CAS (FinishOCDispatch), after 90s without a heartbeat; the
//     dispatch call itself is bounded by a 30s client deadline, so a live
//     sequence (claim -> bounded call -> finish) can never look stale.
//   - The durable dispatch record IS the settlement outbox: its terminal
//     state plus reservation_id were written in ONE transaction (one row), so
//     a crash after that commit loses nothing. The loop delivers the
//     settlement with a STABLE fact (Action-ID-stable RunID/CallID/AttemptID,
//     UsageFact Revision=1) and only then marks delivery; a Finish failure
//     retries the settlement alone — the terminal result is never rewritten
//     and the provider is never re-called. The commercial settlement ledger
//     treats an identical-fact replay as an idempotent no-op.
//   - unknown->terminal transitions (ResolveUnknown) go through the same
//     record CAS and settlement delivery, closing the precheck fact-6 gap.
//   - Pre-allocations ride the EXISTING commercial recovery: this loop adds
//     no new ledger, and unknown outcomes keep their hold (unknown is not
//     free).
//
// SEAM FOR T13 (named explicitly per the T10-Q-1 carry): the periodic
// reconciliation loop is (*OCRecovery).RunOnce(ctx). T13 wires the production
// caller — e.g. a ticker goroutine constructed with NewOCRecovery(claims,
// service) that calls RunOnce once per period with a bounded context; no
// other integration point is required. The loop is idempotent and safe to run
// concurrently on multiple replicas: every state move is a database CAS.
package appconnector

import (
	"context"
	"errors"
	"fmt"
	"time"

	commsvc "github.com/Tencent/WeKnora/internal/application/service/commercial"
	"github.com/Tencent/WeKnora/internal/commercial"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
)

var (
	// ErrOCRecoveryInvalid covers a misconfigured reconciliation loop: a nil
	// recovery store or a nil action service.
	ErrOCRecoveryInvalid = errors.New("oc_recovery_invalid")
)

const (
	// ocDispatchStaleAfter is the no-heartbeat staleness bound: a dispatched
	// record untouched for this long is treated as a crashed worker and
	// parked unknown (ruling: 90s).
	ocDispatchStaleAfter = 90 * time.Second
	// ocSettlementRetryWindow bounds how long the loop keeps retrying a
	// settlement delivery for one terminal record. The delivery usually
	// succeeds on the first pass and is then durably marked; only failing
	// deliveries persist in the scan, and past this window they belong to the
	// existing commercial recovery (no new ledger here).
	ocSettlementRetryWindow = 24 * time.Hour
	// ocUnknownResolveWindow bounds how long a record parked in unknown stays
	// eligible for the loop's read-only provider query. A provider that keeps
	// answering "unknown" never refreshes the record, so it ages out instead
	// of being queried forever.
	ocUnknownResolveWindow = 24 * time.Hour
	// ocRecoveryBatch bounds one pass of each scan.
	ocRecoveryBatch = 32
	// ocRecoveryOpTimeout bounds every database/gate operation the loop and
	// the settlement delivery paths perform (T11-QF-3: no naked
	// context.Background() without a deadline in recovery-owned code).
	ocRecoveryOpTimeout = 10 * time.Second
	// ocFinishAttempts bounds the in-line retry that persists the ORIGINAL
	// provider result after a local database failure (ruling 4).
	ocFinishAttempts = 3
	// ocFinishRetryDelay spaces the bounded persistence retries.
	ocFinishRetryDelay = 50 * time.Millisecond
)

// ReplayAllowed reports whether a dispatch record may LEGALLY be replayed at
// now. It is the plan sketch VERBATIM and deliberately grants nothing: no
// code path in phase one replays automatically, and unknown actions never
// re-enter Execute. The boundaries:
//
//   - now == ReplayUntil is NOT allowed (the original deadline never extends);
//   - a clock running backwards (now before FirstSentAt) is NOT allowed;
//   - a ReplayUntil beyond FirstSentAt+24h is a corrupt/implausible window
//     and is never replayable.
func ReplayAllowed(r appconn.OCDispatchRecord, now time.Time) bool {
	return r.Key != "" && r.RuntimeID != "" && !r.FirstSentAt.IsZero() &&
		!now.Before(r.FirstSentAt) && now.Before(r.ReplayUntil) &&
		!r.ReplayUntil.After(r.FirstSentAt.Add(24*time.Hour))
}

// OCRecoveryStore is the recovery-side persistence surface of the durable
// dispatch record; it is implemented by repository/appconnector.OCStore. It
// is deliberately separate from the (frozen) claim face.
type OCRecoveryStore interface {
	ListStaleOCDispatches(ctx context.Context, now time.Time, staleAfter time.Duration, limit int) ([]appconn.OCDispatchRecord, error)
	ListMisalignedOCDispatches(ctx context.Context, limit int) ([]repoappconn.OCDispatchAlignment, error)
	ListUnknownOCDispatches(ctx context.Context, now time.Time, window time.Duration, limit int) ([]appconn.OCDispatchRecord, error)
	ListOCDispatchSettlementsDue(ctx context.Context, now time.Time, window time.Duration, limit int) ([]appconn.OCDispatchRecord, error)
	FinishOCDispatch(ctx context.Context, tenant uint64, actionID string, fence int64, from, to, executionID string) error
	MarkOCDispatchSettled(ctx context.Context, tenant uint64, actionID string, fence int64, now time.Time) error
}

// OCRecovery is the periodic reconciliation loop (the T13 seam). One RunOnce
// performs four idempotent passes, in dependency order:
//
//  1. sweep   — stale dispatched records CAS to unknown (record first, then
//     the action row), converging crashed workers;
//  2. align   — action rows left behind by a crash between the record write
//     and the action write are restored FROM the record;
//  3. resolve — records parked in unknown are offered to the read-only
//     provider query (only when a resolver is wired; production phase one
//     has none);
//  4. settle  — terminal records with an undelivered settlement receive it,
//     with the stable fact, then the delivery is durably marked.
//
// Every pass runs under bounded per-operation contexts and joins its errors:
// RunOnce reports what failed but never aborts the remaining passes.
type OCRecovery struct {
	claims  OCRecoveryStore
	service *ActionService

	staleAfter    time.Duration
	settleWindow  time.Duration
	resolveWindow time.Duration
	batch         int
	now           func() time.Time
}

// NewOCRecovery validates the wiring and builds the loop with the ruling
// defaults. Knobs are fields so operators (and tests) can tune them without
// rebuilding the wiring.
func NewOCRecovery(claims OCRecoveryStore, service *ActionService) (*OCRecovery, error) {
	if claims == nil {
		return nil, fmt.Errorf("%w: nil recovery store", ErrOCRecoveryInvalid)
	}
	if service == nil {
		return nil, fmt.Errorf("%w: nil action service", ErrOCRecoveryInvalid)
	}
	return &OCRecovery{
		claims:        claims,
		service:       service,
		staleAfter:    ocDispatchStaleAfter,
		settleWindow:  ocSettlementRetryWindow,
		resolveWindow: ocUnknownResolveWindow,
		batch:         ocRecoveryBatch,
		now:           time.Now,
	}, nil
}

// RunOnce executes one reconciliation pass. It is safe to call concurrently
// and from multiple replicas: every state move is a database CAS, the
// settlement fact is stable, and the commercial ledger is replay-idempotent.
func (r *OCRecovery) RunOnce(ctx context.Context) error {
	return errors.Join(
		r.sweepStale(ctx),
		r.alignMisaligned(ctx),
		r.resolveUnknowns(ctx),
		r.settleDue(ctx),
	)
}

func (r *OCRecovery) opCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, ocRecoveryOpTimeout)
}

// sweepStale parks crash-orphaned dispatched records in unknown via the fence
// CAS, then aligns the action row. A concurrent legitimate finish loses the
// race cleanly: its from-state no longer matches and the outcome stays with
// the provider query.
func (r *OCRecovery) sweepStale(ctx context.Context) error {
	var errs []error
	now := r.now().UTC()
	octx, cancel := r.opCtx(ctx)
	rows, err := r.claims.ListStaleOCDispatches(octx, now, r.staleAfter, r.batch)
	cancel()
	if err != nil {
		return fmt.Errorf("stale scan: %w", err)
	}
	for _, rec := range rows {
		octx, cancel := r.opCtx(ctx)
		err := r.claims.FinishOCDispatch(octx, rec.TenantID, rec.ActionID, rec.Fence,
			appconn.ActionDispatched, appconn.ActionUnknown, "")
		cancel()
		if err != nil {
			if errors.Is(err, repoappconn.ErrOCDispatchConflict) {
				continue // a legitimate finish won the race
			}
			errs = append(errs, fmt.Errorf("sweep %s: %w", rec.ActionID, err))
			continue
		}
		if aerr := r.service.recoverAlignAction(ctx, rec.ActionID, appconn.ActionUnknown,
			"recovered: no heartbeat; parked unknown pending provider query"); aerr != nil {
			errs = append(errs, aerr)
		}
	}
	return errors.Join(errs...)
}

// alignMisaligned restores action rows that disagree with their (authoritative)
// durable record after a crash between the two writes.
func (r *OCRecovery) alignMisaligned(ctx context.Context) error {
	var errs []error
	octx, cancel := r.opCtx(ctx)
	rows, err := r.claims.ListMisalignedOCDispatches(octx, r.batch)
	cancel()
	if err != nil {
		return fmt.Errorf("misalignment scan: %w", err)
	}
	for _, al := range rows {
		switch {
		case al.ActionState == appconn.ActionDispatched:
			if aerr := r.service.recoverAlignAction(ctx, al.Record.ActionID, al.Record.State,
				"recovered: terminal outcome restored from dispatch record"); aerr != nil {
				errs = append(errs, aerr)
			}
		case al.ActionState == appconn.ActionUnknown && al.Record.State != appconn.ActionUnknown:
			octx, cancel := r.opCtx(ctx)
			_, ferr := r.service.finishUnknownFromRecord(octx, al.Record, al.ActionFence)
			cancel()
			if ferr != nil {
				errs = append(errs, fmt.Errorf("align %s: %w", al.Record.ActionID, ferr))
			}
		}
	}
	return errors.Join(errs...)
}

// resolveUnknowns offers recently-parked unknown records to the read-only
// provider query. No resolver wired (production phase one) is a no-op. A
// provider answer that is not terminal leaves the record unknown.
func (r *OCRecovery) resolveUnknowns(ctx context.Context) error {
	if r.service.unknown == nil {
		return nil
	}
	var errs []error
	now := r.now().UTC()
	octx, cancel := r.opCtx(ctx)
	rows, err := r.claims.ListUnknownOCDispatches(octx, now, r.resolveWindow, r.batch)
	cancel()
	if err != nil {
		return fmt.Errorf("unknown scan: %w", err)
	}
	for _, rec := range rows {
		octx, cancel := r.opCtx(ctx)
		rerr := r.service.ResolveUnknown(octx, rec.ActionID)
		cancel()
		if rerr != nil && !errors.Is(rerr, ErrDispatchUnknown) {
			errs = append(errs, fmt.Errorf("resolve %s: %w", rec.ActionID, rerr))
		}
	}
	return errors.Join(errs...)
}

// settleDue delivers the settlements of terminal records whose delivery has
// not been marked, then marks them. Delivery failures stay due for the next
// pass — the terminal result is never rewritten and the provider is never
// re-called.
func (r *OCRecovery) settleDue(ctx context.Context) error {
	if r.service.gate == nil {
		return nil
	}
	var errs []error
	now := r.now().UTC()
	octx, cancel := r.opCtx(ctx)
	rows, err := r.claims.ListOCDispatchSettlementsDue(octx, now, r.settleWindow, r.batch)
	cancel()
	if err != nil {
		return fmt.Errorf("settlement scan: %w", err)
	}
	for _, rec := range rows {
		octx, cancel := r.opCtx(ctx)
		derr := deliverOCSettlement(octx, r.service.gate, rec.TenantID, rec.ActionID, rec.ReservationID)
		cancel()
		if derr != nil {
			errs = append(errs, fmt.Errorf("settle %s: %w", rec.ActionID, derr))
			continue
		}
		mctx, mcancel := r.opCtx(ctx)
		merr := r.claims.MarkOCDispatchSettled(mctx, rec.TenantID, rec.ActionID, rec.Fence, r.now().UTC())
		mcancel()
		if merr != nil && !errors.Is(merr, repoappconn.ErrOCDispatchConflict) {
			// Already marked by a concurrent replica: fine. Anything else
			// leaves the row due — the next pass re-delivers the identical
			// fact, which the ledger replays as a no-op.
			errs = append(errs, fmt.Errorf("mark %s: %w", rec.ActionID, merr))
		}
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// shared settlement helpers (used by Execute, ResolveUnknown and the loop)
// ---------------------------------------------------------------------------

// ocUsageFact builds the STABLE settlement fact of one dispatch: the
// Action-ID-stable identity (RunID/CallID/AttemptID) and Revision=1 with a
// connector_calls dimension. Every retry — in-line or from the loop —
// rebuilds the identical fact, so the commercial ledger replays it as an
// idempotent no-op instead of a second charge. The dimension mirrors the
// Execute convention: a terminal outcome consumed the pre-allocated call
// (succeeded or failed), while unknown keeps the hold and settles nothing.
func ocUsageFact(tenant uint64, actionID string, connectorCalls int64, occurredAt time.Time) commercial.UsageFact {
	return commercial.UsageFact{
		TenantID:     tenant,
		RunID:        "appaction:" + actionID,
		CallID:       actionID,
		AttemptID:    actionID,
		Funding:      commercial.FundingPlatform,
		Service:      commercial.ServiceConnector,
		PriceVersion: "v1",
		Revision:     1,
		OccurredAt:   occurredAt,
		Dimensions:   map[string]int64{commercial.DimensionConnector: connectorCalls},
		Status:       commercial.UsageStatusFinal,
	}
}

// deliverOCSettlement delivers one settlement under a bounded context. A
// reservation the commercial side no longer knows (release-reclaim interleave
// residue, expired hold) is terminal success: there is nothing to settle, and
// the record is marked delivered so the loop stops retrying. Any other
// failure leaves the durable intent in place — the caller (loop or service)
// retries the settlement alone.
func deliverOCSettlement(ctx context.Context, gate commercial.ExecutionGate, tenant uint64, actionID, reservationID string) error {
	if gate == nil || reservationID == "" {
		return nil
	}
	dctx, cancel := context.WithTimeout(ctx, ocRecoveryOpTimeout)
	defer cancel()
	if err := gate.Finish(dctx, reservationID, ocUsageFact(tenant, actionID, 1, time.Now().UTC())); err != nil {
		if errors.Is(err, commsvc.ErrGateReservationUnknown) {
			return nil
		}
		return err
	}
	return nil
}

// ocRetry runs fn up to ocFinishAttempts times with the bounded persistence
// delay, returning the last error. It exists for ruling 4: a local write
// failure retries with the ORIGINAL arguments — never a re-derived, never a
// re-dispatched result.
func ocRetry(fn func() error) error {
	var err error
	for i := 0; i < ocFinishAttempts; i++ {
		if i > 0 {
			time.Sleep(ocFinishRetryDelay)
		}
		if err = fn(); err == nil {
			return nil
		}
	}
	return err
}
