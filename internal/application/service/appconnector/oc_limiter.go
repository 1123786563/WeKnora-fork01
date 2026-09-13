// oc_limiter.go implements T10's distributed dispatch concurrency control:
// four scope leases (connection, tenant, provider, global) held in the
// DATABASE — no single-process semaphore stands in for multi-replica
// limiting — plus the phase-one fair queue (round-robin by space) and the
// durable provider Retry-After gate.
package appconnector

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
)

var (
	// ErrOCSlotLimiterInvalid covers misconfigured limiters: a nil store,
	// an owner-less replica, or any non-positive limit (ruling 6: every
	// configured limit must be > 0).
	ErrOCSlotLimiterInvalid = errors.New("oc_slot_limiter_invalid")
	// ErrOCSlotsBusy reports that the request waited for dispatch slots
	// until its context gave up. Nothing was consumed.
	ErrOCSlotsBusy = errors.New("oc_slots_busy")
	// ErrOCProviderThrottled reports the provider's stored 429 Retry-After
	// is still in effect; new dispatches fail closed instead of queueing
	// behind a provider that said stop.
	ErrOCProviderThrottled = errors.New("oc_provider_throttled")
	// ErrOCRetryAfterInvalid covers unparseable Retry-After values.
	ErrOCRetryAfterInvalid = errors.New("oc_retry_after_invalid")
)

// ocRetryAfterMax bounds any stored Retry-After horizon; longer windows are
// clamped so a hostile/buggy header cannot wedge a provider for days.
const ocRetryAfterMax = 24 * time.Hour

// OCSlotLimits are the four dispatch concurrency scopes. Phase-one defaults
// (ruling 6): tenant=4, connection=1, provider=16, global=32 — all > 0.
type OCSlotLimits struct {
	PerTenant     int
	PerConnection int
	PerProvider   int
	Global        int
}

// DefaultOCSlotLimits returns the plan's phase-one defaults.
func DefaultOCSlotLimits() OCSlotLimits {
	return OCSlotLimits{PerTenant: 4, PerConnection: 1, PerProvider: 16, Global: 32}
}

// Validate rejects any non-positive limit before a limiter is built.
func (c OCSlotLimits) Validate() error {
	if c.PerTenant <= 0 || c.PerConnection <= 0 || c.PerProvider <= 0 || c.Global <= 0 {
		return fmt.Errorf("%w: all limits must be > 0 (tenant=%d connection=%d provider=%d global=%d)",
			ErrOCSlotLimiterInvalid, c.PerTenant, c.PerConnection, c.PerProvider, c.Global)
	}
	return nil
}

// Scope ids — the lease table's limit domains. The acquisition ORDER below
// is fixed for every replica, so racing acquisitions can never deadlock.
func ocConnectionScope(tenant uint64, connection string) string {
	return fmt.Sprintf("oc-conn:%d:%s", tenant, connection)
}

func ocTenantScope(tenant uint64) string { return fmt.Sprintf("oc-tenant:%d", tenant) }

func ocProviderScope(provider string) string { return "oc-provider:" + provider }

const ocGlobalScope = "oc-global"

// OCSlotLeaseStore is the persistence surface the limiter needs; it is
// implemented by repository/appconnector.OCStore.
type OCSlotLeaseStore interface {
	AcquireOCLease(ctx context.Context, scope string, limit int, owner string, tenant uint64, actionID string, now, until time.Time) (repoappconn.OCDispatchLeaseRow, error)
	ReleaseOCLease(ctx context.Context, id, owner string, fence int64, now time.Time) (bool, error)
	OCProviderRetryAfter(ctx context.Context, provider string, now time.Time) (time.Duration, error)
}

// ocSlotWaiter is one parked acquisition attempt.
type ocSlotWaiter struct {
	space string
	wake  chan struct{}
}

// ocSlotQueue is the in-process waiting queue, served ROUND-ROBIN BY SPACE:
// next() pops one waiter of the head space, then rotates that space to the
// back — one space's burst can never monopolize the slots while another
// space waits. (Cross-process fairness stays best-effort: the DB lease is
// the authority; this queue only orders who tries next on THIS replica.)
type ocSlotQueue struct {
	mu      sync.Mutex
	ring    []string
	waiting map[string][]*ocSlotWaiter
}

func (q *ocSlotQueue) anyWaiting() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.ring) > 0
}

func (q *ocSlotQueue) enter(space string) *ocSlotWaiter {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.waiting == nil {
		q.waiting = map[string][]*ocSlotWaiter{}
	}
	w := &ocSlotWaiter{space: space, wake: make(chan struct{})}
	if _, ok := q.waiting[space]; !ok {
		q.ring = append(q.ring, space)
	}
	q.waiting[space] = append(q.waiting[space], w)
	return w
}

func (q *ocSlotQueue) leave(w *ocSlotWaiter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.removeLocked(w)
}

func (q *ocSlotQueue) removeLocked(w *ocSlotWaiter) {
	ws, ok := q.waiting[w.space]
	if !ok {
		return
	}
	for i, cand := range ws {
		if cand == w {
			ws = append(ws[:i], ws[i+1:]...)
			break
		}
	}
	if len(ws) == 0 {
		delete(q.waiting, w.space)
		for i, s := range q.ring {
			if s == w.space {
				q.ring = append(q.ring[:i], q.ring[i+1:]...)
				break
			}
		}
		return
	}
	q.waiting[w.space] = ws
}

// next pops the head waiter of the ring's head space and rotates that space
// to the back (round-robin by space).
func (q *ocSlotQueue) next() (*ocSlotWaiter, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.ring) > 0 {
		space := q.ring[0]
		ws := q.waiting[space]
		if len(ws) == 0 {
			delete(q.waiting, space)
			q.ring = q.ring[1:]
			continue
		}
		w := ws[0]
		ws = ws[1:]
		if len(ws) == 0 {
			delete(q.waiting, space)
			q.ring = q.ring[1:]
		} else {
			q.waiting[space] = ws
			q.ring = append(q.ring[1:], space) // rotate: one grant per space per round
		}
		return w, true
	}
	return nil, false
}

// OCSlotLimiter acquires the four dispatch leases per open-connector
// dispatch. Every limit is enforced by the DATABASE lease — replicas share
// the exact same counters — while the in-process queue only sequences THIS
// replica's waiters fairly.
type OCSlotLimiter struct {
	store      OCSlotLeaseStore
	limits     OCSlotLimits
	owner      string
	retryEvery time.Duration // periodic retry that reclaims expired leases
	now        func() time.Time
	q          ocSlotQueue
}

// NewOCSlotLimiter validates the configuration (all limits > 0, non-empty
// owner) and binds the lease store.
func NewOCSlotLimiter(store OCSlotLeaseStore, limits OCSlotLimits, owner string) (*OCSlotLimiter, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: nil lease store", ErrOCSlotLimiterInvalid)
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if owner == "" {
		return nil, fmt.Errorf("%w: empty owner", ErrOCSlotLimiterInvalid)
	}
	return &OCSlotLimiter{
		store: store, limits: limits, owner: owner,
		retryEvery: 250 * time.Millisecond, now: time.Now,
	}, nil
}

// AcquireOCSlots takes one lease in each of the four scopes — in the FIXED
// order connection -> tenant -> provider -> global — and returns a release
// closure. The acquisition is all-or-nothing: a busy later scope releases
// the earlier ones before reporting busy. Waiters park in the round-robin
// space queue; releases (and the periodic retry that reclaims expired
// leases from crashed holders) wake them in rotation. A provider whose
// stored 429 Retry-After is still active fails closed immediately.
func (l *OCSlotLimiter) AcquireOCSlots(ctx context.Context, tenant uint64, connection, provider, actionID string, until time.Time) (func(context.Context) error, error) {
	if tenant == 0 || connection == "" || provider == "" || actionID == "" || until.IsZero() {
		return nil, fmt.Errorf("%w: tenant, connection, provider, action and until are required", ErrOCSlotLimiterInvalid)
	}
	// Provider cooldown first: never queue behind a provider that said stop.
	if wait, err := l.store.OCProviderRetryAfter(ctx, provider, l.now()); err != nil {
		return nil, err
	} else if wait > 0 {
		return nil, fmt.Errorf("%w: provider cooling down for %s", ErrOCProviderThrottled, wait)
	}
	scopes := []struct {
		scope string
		limit int
	}{
		{ocConnectionScope(tenant, connection), l.limits.PerConnection},
		{ocTenantScope(tenant), l.limits.PerTenant},
		{ocProviderScope(provider), l.limits.PerProvider},
		{ocGlobalScope, l.limits.Global},
	}
	// Fast path — but never barge past existing waiters.
	if !l.q.anyWaiting() {
		held, err := l.tryAcquire(ctx, tenant, actionID, scopes, until)
		if err == nil {
			return l.releaseFunc(held), nil
		}
		if !errors.Is(err, repoappconn.ErrOCLeaseBusy) {
			return nil, err
		}
	}
	space := strconv.FormatUint(tenant, 10)
	ticker := time.NewTicker(l.retryEvery)
	defer ticker.Stop()
	for {
		w := l.q.enter(space)
		select {
		case <-w.wake:
		case <-ticker.C: // crashed holders' leases expire; retry reclaims them
		case <-ctx.Done():
			l.q.leave(w)
			return nil, fmt.Errorf("%w: %v", ErrOCSlotsBusy, ctx.Err())
		}
		l.q.leave(w)
		held, err := l.tryAcquire(ctx, tenant, actionID, scopes, until)
		if err == nil {
			return l.releaseFunc(held), nil
		}
		if !errors.Is(err, repoappconn.ErrOCLeaseBusy) {
			return nil, err
		}
	}
}

// tryAcquire walks the scopes in order; on the first busy (or failing)
// scope it rolls back whatever it already holds.
func (l *OCSlotLimiter) tryAcquire(ctx context.Context, tenant uint64, actionID string, scopes []struct {
	scope string
	limit int
}, until time.Time) ([]repoappconn.OCDispatchLeaseRow, error) {
	var held []repoappconn.OCDispatchLeaseRow
	for _, sc := range scopes {
		lease, err := l.store.AcquireOCLease(ctx, sc.scope, sc.limit, l.owner, tenant, actionID, l.now(), until)
		if err != nil {
			for i := len(held) - 1; i >= 0; i-- {
				_, _ = l.store.ReleaseOCLease(ctx, held[i].ID, held[i].Owner, held[i].Fence, l.now())
			}
			return nil, err
		}
		held = append(held, lease)
	}
	return held, nil
}

// releaseFunc builds the release closure: frees the leases in reverse
// acquisition order (CAS-protected, idempotent) and wakes the next waiter
// in the space rotation. Releasing a slot NEVER re-sends anything — it only
// frees capacity; dispatch replay decisions live in the dispatch record.
func (l *OCSlotLimiter) releaseFunc(held []repoappconn.OCDispatchLeaseRow) func(context.Context) error {
	return func(ctx context.Context) error {
		var firstErr error
		for i := len(held) - 1; i >= 0; i-- {
			if _, err := l.store.ReleaseOCLease(ctx, held[i].ID, held[i].Owner, held[i].Fence, l.now()); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		l.wakeNext()
		return firstErr
	}
}

func (l *OCSlotLimiter) wakeNext() {
	if w, ok := l.q.next(); ok {
		close(w.wake)
	}
}

// ParseOCRetryAfter parses a Retry-After header value — delay-seconds or
// HTTP-date (RFC 9110) — into the absolute instant dispatches may resume,
// clamped to [now, now+24h]. The dispatcher stores this value but NEVER
// re-sends a possibly-side-effected request on its strength.
func ParseOCRetryAfter(raw string, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: empty value", ErrOCRetryAfterInvalid)
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs < 0 {
			return time.Time{}, fmt.Errorf("%w: negative delay %d", ErrOCRetryAfterInvalid, secs)
		}
		d := time.Duration(secs) * time.Second
		if d > ocRetryAfterMax {
			d = ocRetryAfterMax
		}
		return now.Add(d).UTC(), nil
	}
	if at, err := http.ParseTime(raw); err == nil {
		if at.Before(now) {
			at = now
		}
		if at.After(now.Add(ocRetryAfterMax)) {
			at = now.Add(ocRetryAfterMax)
		}
		return at.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("%w: %q", ErrOCRetryAfterInvalid, raw)
}
