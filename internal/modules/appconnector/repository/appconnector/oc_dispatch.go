// oc_dispatch.go implements T10's atomic dispatch claim, the operation-scoped
// idempotency key and the distributed concurrency lease.
//
// LOCKING CONTRACT (carried from T08, oc_revoke.go): every writer that
// mutates a connection's authorization generation takes the connections row
// lock FIRST. This claim follows the unified lock order
//
//	app_actions -> connections -> installations (read) -> binding (read)
//
// so a revocation racing a claim serializes ON the connection row: the claim
// either commits before the revoke (the dispatch survives as an already
// claimed operation) or re-reads the revoked connection / bumped auth
// version / flipped binding and fails closed — a revocation that landed
// after the service's A02 Check is rejected by the claim transaction itself
// (T04-QF-2 closure). The claim never takes a lock the revocation path
// needs first, so the two can never deadlock.
package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrOCDispatchInvalid covers malformed claim input rejected before any
	// database access: missing subject identity, a blank action id, a
	// subject that does not match the persisted row, or an action without
	// an open-connector execution binding (native actions use the frozen
	// ClaimDispatch path).
	ErrOCDispatchInvalid = errors.New("oc_dispatch_invalid")
	// ErrOCDispatchConflict covers rejected claims: a lost state/fence
	// guard, a revoked or version-drifted connection, a disabled
	// installation, a revoked binding, and result updates whose
	// tenant/action/fence/state guard matched zero rows.
	ErrOCDispatchConflict = errors.New("oc_dispatch_conflict")
	// ErrOCDispatchClaimed reports a LOST claim race: another request
	// already claimed this exact action. The winner's record is returned
	// alongside the error — the loser reads the winner's reservation (the
	// budget key is Action-ID-stable, so both racing requests share it) and
	// must never cancel or re-claim it.
	ErrOCDispatchClaimed = errors.New("oc_dispatch_claimed")
	// ErrOCLeaseInvalid covers malformed lease input rejected before any
	// database access: blank scope/owner, a non-positive limit or tenant,
	// or an until deadline not after now.
	ErrOCLeaseInvalid = errors.New("oc_lease_invalid")
	// ErrOCLeaseBusy reports a scope at its configured limit; the caller
	// queues (round-robin by space) or backs off. Expired and released
	// leases are reclaimed automatically by the next acquisition.
	ErrOCLeaseBusy = errors.New("oc_lease_busy")
)

// ocKeyPrefix is the fixed operation-scoped key namespace. A key is
// "wk-oc-" + a fresh uuid — distinct per (tenant, action) claim, NEVER a
// connection id or any other reusable identity, and separate from the
// provider_key the native pipeline stores.
const ocKeyPrefix = "wk-oc-"

// ocReplayWindow is the headroom under the upstream 24h idempotency window
// (T01): ReplayUntil = FirstSentAt + 23h50m, computed in the DATABASE, and
// never advanced by any later write.
const ocReplayWindow = 23*time.Hour + 50*time.Minute

// NewOCKey generates one operation-scoped dispatch idempotency key. The
// unique (tenant_id, action_id) dispatch record makes the key be generated
// exactly once per approved operation.
func NewOCKey() string { return ocKeyPrefix + uuid.NewString() }

// OCDispatchRecordRow persists the local linearization point of one approved
// open-connector dispatch. The composite primary key makes the claim
// single-shot per action; UNIQUE (runtime_id, key) pins the key to one
// runtime so a replayed key can never resolve to a different dispatch.
type OCDispatchRecordRow struct {
	TenantID      uint64    `gorm:"primaryKey;column:tenant_id"`
	ActionID      string    `gorm:"primaryKey;column:action_id"`
	RuntimeID     string    `gorm:"column:runtime_id;not null;uniqueIndex:uq_oc_dispatch_runtime_key"`
	Key           string    `gorm:"column:key;not null;uniqueIndex:uq_oc_dispatch_runtime_key"`
	ExecutionID   string    `gorm:"column:execution_id;not null;default:''"`
	State         string    `gorm:"column:state;not null"`
	ReservationID string    `gorm:"column:reservation_id;not null;default:''"`
	FirstSentAt   time.Time `gorm:"column:first_sent_at;not null"`
	ReplayUntil   time.Time `gorm:"column:replay_until;not null"`
	Fence         int64     `gorm:"column:fence;not null;default:1"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (OCDispatchRecordRow) TableName() string { return "connector_dispatch_records" }

// OCDispatchLeaseRow is one held concurrency slot. Scope is the fully
// qualified limit domain (connection/tenant/provider/global); Until is the
// lease deadline — a holder that crashes simply expires and the slot is
// reclaimed by the next acquisition. ReleasedAt marks an orderly release.
type OCDispatchLeaseRow struct {
	ID         string     `gorm:"primaryKey;column:id"`
	Scope      string     `gorm:"column:scope;not null"`
	TenantID   uint64     `gorm:"column:tenant_id;not null"`
	Owner      string     `gorm:"column:owner;not null"`
	ActionID   string     `gorm:"column:action_id;not null;default:''"`
	AcquiredAt time.Time  `gorm:"column:acquired_at;not null"`
	Until      time.Time  `gorm:"column:until;not null"`
	ReleasedAt *time.Time `gorm:"column:released_at"`
	Fence      int64      `gorm:"column:fence;not null;default:1"`
}

func (OCDispatchLeaseRow) TableName() string { return "connector_dispatch_leases" }

// ocDispatchRecordDomain maps a row to the shared domain type.
func ocDispatchRecordDomain(row OCDispatchRecordRow) appconnector.OCDispatchRecord {
	return appconnector.OCDispatchRecord{
		TenantID: row.TenantID, ActionID: row.ActionID, RuntimeID: row.RuntimeID,
		Key: row.Key, ExecutionID: row.ExecutionID, State: row.State,
		ReservationID: row.ReservationID, FirstSentAt: row.FirstSentAt,
		ReplayUntil: row.ReplayUntil, Fence: row.Fence,
	}
}

// ocDispatchInsertSQL writes first_sent_at from DATABASE time and computes
// replay_until in the database as first_sent_at + 23h50m — the stored pair is
// self-consistent regardless of any process clock skew.
func ocDispatchInsertSQL(dialect string) string {
	if dialect == "postgres" {
		return "INSERT INTO connector_dispatch_records (tenant_id, action_id, runtime_id, key, state, reservation_id, first_sent_at, replay_until, fence) VALUES (?, ?, ?, ?, 'dispatched', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '23 hours 50 minutes', ?)"
	}
	return "INSERT INTO connector_dispatch_records (tenant_id, action_id, runtime_id, key, state, reservation_id, first_sent_at, replay_until, fence) VALUES (?, ?, ?, ?, 'dispatched', ?, CURRENT_TIMESTAMP, datetime(CURRENT_TIMESTAMP, '+1430 minutes'), ?)"
}

// ocForUpdate returns the dialect-gated row-lock suffix (PG locks for real;
// sqlite's serialized single writer provides the same mutual exclusion).
func ocForUpdate(tx *gorm.DB) string {
	if tx.Dialector.Name() == "postgres" {
		return " FOR UPDATE"
	}
	return ""
}

// ClaimOCDispatch is the atomic dispatch claim for open-connector actions:
// inside ONE transaction it locks the action row, then the connection row
// (the T08 serialization point), re-verifies the connection's state and
// authorization generation, the installation and the tenant binding,
// consumes exactly one approval count under the guarded predicate
// (action_id + args_digest + remaining>0 + unexpired), inserts the dispatch
// record with a freshly generated operation-scoped key (database time for
// FirstSentAt, ReplayUntil = FirstSentAt + 23h50m) and walks the action
// authorized->dispatched with a bumped fence.
//
// A racing request that LOST sees the action already dispatched and gets the
// WINNER's record back with ErrOCDispatchClaimed — no second key, no second
// decrement, and the shared reservation belongs to the winner alone.
func (s *OCStore) ClaimOCDispatch(ctx context.Context, subject appconnector.OCSubject, actionID, reservationID string) (appconnector.OCDispatchRecord, error) {
	if subject.TenantID == 0 || subject.ActorID == "" || actionID == "" {
		return appconnector.OCDispatchRecord{}, ErrOCDispatchInvalid
	}
	var out appconnector.OCDispatchRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Action row lock FIRST (unified order: action first), then the
		// full read — every later decision below sees committed data that
		// cannot change until this transaction ends.
		lock := ocForUpdate(tx)
		var lockedID string
		if err := tx.Raw("SELECT id FROM app_actions WHERE tenant_id = ? AND id = ?"+lock, subject.TenantID, actionID).Scan(&lockedID).Error; err != nil {
			return err
		}
		var row ActionRow
		if err := tx.Where("tenant_id = ? AND id = ?", subject.TenantID, actionID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrActionNotFound
			}
			return err
		}
		// The claim is for the PERSISTED subject only — a caller cannot
		// claim on behalf of another actor's action.
		if row.TenantID != subject.TenantID || row.ActorID != subject.ActorID {
			return ErrOCDispatchInvalid
		}
		// 2. Lost race: the winner already claimed this exact action. The
		// loser reads the winner's record (shared reservation) and stops.
		if row.State == appconnector.ActionDispatched {
			rec, err := ocLoadDispatchRecord(tx, subject.TenantID, actionID)
			if err != nil {
				return err
			}
			if rec.ActionID == "" {
				// Dispatched by the frozen native path (no OC record).
				return ErrOCDispatchConflict
			}
			out = ocDispatchRecordDomain(rec)
			return ErrOCDispatchClaimed
		}
		if row.State != appconnector.ActionAuthorized {
			return ErrOCDispatchConflict
		}
		// Native actions have no execution binding and never take this path.
		if row.OCBindingJSON == "" {
			return ErrOCDispatchInvalid
		}
		var binding appconnector.OCExecutionBinding
		if err := json.Unmarshal([]byte(row.OCBindingJSON), &binding); err != nil || binding.RuntimeID == "" {
			return ErrOCDispatchInvalid
		}

		// 3. Connection row lock — the T08 revocation serialization point.
		var conn ConnectionRow
		if err := tx.Raw(
			"SELECT tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version FROM connections WHERE tenant_id = ? AND id = ?"+lock,
			subject.TenantID, row.ConnectionID).Scan(&conn).Error; err != nil {
			return err
		}
		if conn.ID == "" {
			return gorm.ErrRecordNotFound
		}
		if conn.State != appconnector.ConnectionActive || conn.AuthVersion != row.AuthVersion {
			// Revoked (or version-drifted) after Check: reject the dispatch.
			return ErrOCDispatchConflict
		}

		// 4. Installation still active (same freshness discipline as the
		// frozen ActivateOCAttempt).
		var inst InstallationRow
		if err := tx.Where("id = ? AND tenant_id = ?", conn.InstallationID, subject.TenantID).First(&inst).Error; err != nil {
			return err
		}
		if inst.State != appconnector.InstallationActive {
			return ErrOCDispatchConflict
		}

		// 5. Tenant binding still active at the SAME authorization
		// generation (revocation flips it together with the connection).
		var b OCBindingRow
		if err := tx.Where("tenant_id = ? AND connection_id = ?", subject.TenantID, row.ConnectionID).First(&b).Error; err != nil {
			return err
		}
		if b.State != appconnector.OCBindingActive || b.AuthVersion != row.AuthVersion {
			return ErrOCDispatchConflict
		}

		// 6. Consume exactly one approval count — the plan's guarded
		// predicate, exactly one row must match. The in-memory guard first
		// maps missing/exhausted/expired approvals to their sentinel (and
		// sidesteps sqlite text-timestamp comparison skew); the SQL guard
		// remains the concurrency authority.
		var ap ApprovalRow
		if err := tx.Where("action_id = ? AND args_digest = ?", row.ID, row.ArgsDigest).First(&ap).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrApprovalExhausted
			}
			return err
		}
		if ap.Remaining <= 0 || !time.Now().Before(ap.Expiry) {
			return ErrApprovalExhausted
		}
		now := time.Now().UTC()
		res := tx.Model(&ApprovalRow{}).
			Where("action_id = ? AND args_digest = ? AND remaining > 0 AND expiry > ?", row.ID, row.ArgsDigest, now).
			Update("remaining", gorm.Expr("remaining - 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrApprovalExhausted
		}

		// 7. The dispatch record: key generated exactly once, DB time for
		// FirstSentAt, ReplayUntil = FirstSentAt + 23h50m.
		key := NewOCKey()
		if err := tx.Exec(ocDispatchInsertSQL(tx.Dialector.Name()),
			subject.TenantID, actionID, binding.RuntimeID, key, reservationID, row.Fence+1).Error; err != nil {
			return err
		}
		rec, err := ocLoadDispatchRecord(tx, subject.TenantID, actionID)
		if err != nil {
			return err
		}

		// 8. Walk the action authorized->dispatched with the bumped fence
		// (same guard shape as the frozen ClaimDispatch).
		upd := tx.Model(&ActionRow{}).
			Where("id = ? AND tenant_id = ? AND state = ?", actionID, subject.TenantID, appconnector.ActionAuthorized).
			Updates(map[string]interface{}{
				"state":          appconnector.ActionDispatched,
				"provider_key":   row.ConnectionID,
				"reservation_id": reservationID,
				"fence":          row.Fence + 1,
			})
		if upd.Error != nil {
			return upd.Error
		}
		if upd.RowsAffected != 1 {
			return ErrOCDispatchConflict
		}
		out = ocDispatchRecordDomain(rec)
		return nil
	})
	if err != nil {
		return out, err
	}
	return out, nil
}

// ocLoadDispatchRecord reads one dispatch record row inside a transaction.
func ocLoadDispatchRecord(tx *gorm.DB, tenant uint64, actionID string) (OCDispatchRecordRow, error) {
	var rec OCDispatchRecordRow
	if err := tx.Where("tenant_id = ? AND action_id = ?", tenant, actionID).First(&rec).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return OCDispatchRecordRow{}, nil
		}
		return OCDispatchRecordRow{}, err
	}
	return rec, nil
}

// GetOCDispatch loads one tenant's dispatch record for an action. A missing
// record is gorm.ErrRecordNotFound — distinct from any claim outcome.
func (s *OCStore) GetOCDispatch(ctx context.Context, tenant uint64, actionID string) (appconnector.OCDispatchRecord, error) {
	if tenant == 0 || actionID == "" {
		return appconnector.OCDispatchRecord{}, ErrOCDispatchInvalid
	}
	var rec OCDispatchRecordRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND action_id = ?", tenant, actionID).First(&rec).Error; err != nil {
		return appconnector.OCDispatchRecord{}, err
	}
	return ocDispatchRecordDomain(rec), nil
}

// FinishOCDispatch settles a dispatch record under its
// tenant/action/fence/state guard, recording the provider execution id. It
// NEVER touches first_sent_at or replay_until — deadlines do not move on
// retries or settlements.
func (s *OCStore) FinishOCDispatch(ctx context.Context, tenant uint64, actionID string, fence int64, from, to, executionID string) error {
	if tenant == 0 || actionID == "" || fence < 1 || from == "" || to == "" {
		return ErrOCDispatchInvalid
	}
	switch to {
	case appconnector.ActionSucceeded, appconnector.ActionFailed, appconnector.ActionUnknown:
	default:
		return ErrOCDispatchInvalid
	}
	res := s.db.WithContext(ctx).Model(&OCDispatchRecordRow{}).
		Where("tenant_id = ? AND action_id = ? AND fence = ? AND state = ?", tenant, actionID, fence, from).
		Updates(map[string]interface{}{"state": to, "execution_id": executionID})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrOCDispatchConflict
	}
	return nil
}

// AcquireOCLease takes one concurrency slot in a scope if the scope's live
// lease count is below limit. The scope counter row serializes acquisitions
// (FOR UPDATE on PostgreSQL; sqlite's single writer serializes anyway), dead
// leases (expired or released) are reclaimed first, and the count then
// bounds the insert — the limit can never be overshot by racing replicas.
func (s *OCStore) AcquireOCLease(ctx context.Context, scope string, limit int, owner string, tenant uint64, actionID string, now, until time.Time) (OCDispatchLeaseRow, error) {
	if scope == "" || limit <= 0 || owner == "" || tenant == 0 || now.IsZero() || until.IsZero() || !until.After(now) {
		return OCDispatchLeaseRow{}, ErrOCLeaseInvalid
	}
	now, until = now.UTC(), until.UTC()
	var acquired OCDispatchLeaseRow
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO connector_dispatch_lease_scopes (scope, acquired_total) VALUES (?, 0) ON CONFLICT (scope) DO NOTHING", scope).Error; err != nil {
			return err
		}
		var total int64
		if err := tx.Raw("SELECT acquired_total FROM connector_dispatch_lease_scopes WHERE scope = ?"+ocForUpdate(tx), scope).Scan(&total).Error; err != nil {
			return err
		}
		// Reclaim dead slots: expired or already-released leases are free.
		if err := tx.Exec("DELETE FROM connector_dispatch_leases WHERE scope = ? AND (until <= ? OR released_at IS NOT NULL)", scope, now).Error; err != nil {
			return err
		}
		var live int64
		if err := tx.Model(&OCDispatchLeaseRow{}).Where("scope = ?", scope).Count(&live).Error; err != nil {
			return err
		}
		if live >= int64(limit) {
			return ErrOCLeaseBusy
		}
		row := OCDispatchLeaseRow{
			ID: uuid.NewString(), Scope: scope, TenantID: tenant, Owner: owner,
			ActionID: actionID, AcquiredAt: now, Until: until, Fence: 1,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE connector_dispatch_lease_scopes SET acquired_total = acquired_total + 1 WHERE scope = ?", scope).Error; err != nil {
			return err
		}
		acquired = row
		return nil
	})
	if err != nil {
		return OCDispatchLeaseRow{}, err
	}
	return acquired, nil
}

// ReleaseOCLease frees a held slot under the lease_owner+fence
// compare-and-swap: a stale owner (or a fence the row has moved past)
// matches ZERO rows — one replica can never release another's slot. false
// without error means the slot was already gone.
func (s *OCStore) ReleaseOCLease(ctx context.Context, id, owner string, fence int64, now time.Time) (bool, error) {
	if id == "" || owner == "" || fence < 1 || now.IsZero() {
		return false, ErrOCLeaseInvalid
	}
	res := s.db.WithContext(ctx).Model(&OCDispatchLeaseRow{}).
		Where("id = ? AND owner = ? AND fence = ? AND released_at IS NULL", id, owner, fence).
		Update("released_at", now.UTC())
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// ---------------------------------------------------------------------------
// T12 recovery queries and settlement CAS.
//
// The T10 claim semantics above are FROZEN; everything below is additive and
// read/CAS-only. The durable dispatch record doubles as the settlement
// outbox: a TERMINAL record (succeeded/failed) with a reservation_id IS the
// settlement intent -- it was written atomically with the terminal state, in
// the same row, so no second table is needed. Delivery is marked by a fence
// bump: the claim leaves record.fence == action.fence, and
// MarkOCDispatchSettled raises record.fence past action.fence, which is
// exactly the predicate ListOCDispatchSettlementsDue excludes on.
// ---------------------------------------------------------------------------

// OCDispatchAlignment pairs one dispatch record with its action row's live
// state and fence, for the reconciliation pass that re-aligns an action row
// after a crash between the record write and the action write.
type OCDispatchAlignment struct {
	Record      appconnector.OCDispatchRecord
	ActionState string
	ActionFence int64
}

// ocDispatchTerminalStates are the states whose reservation needs settling.
func ocDispatchTerminalStates() []string {
	return []string{appconnector.ActionSucceeded, appconnector.ActionFailed}
}

// ListStaleOCDispatches returns dispatched records whose last heartbeat
// (updated_at -- every claim/finish/sweep write refreshes it) is older than
// staleAfter, oldest first, at most limit rows. The 90s default is strictly
// slower than the 30s client deadline bound on the dispatch call, so a LIVE
// dispatch sequence (claim -> bounded call -> finish) can never look stale.
func (s *OCStore) ListStaleOCDispatches(ctx context.Context, now time.Time, staleAfter time.Duration, limit int) ([]appconnector.OCDispatchRecord, error) {
	if now.IsZero() || staleAfter <= 0 || limit <= 0 {
		return nil, ErrOCDispatchInvalid
	}
	var rows []OCDispatchRecordRow
	if err := s.db.WithContext(ctx).
		Where("state = ? AND updated_at <= ?", appconnector.ActionDispatched, now.UTC().Add(-staleAfter)).
		Order("updated_at ASC").Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]appconnector.OCDispatchRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, ocDispatchRecordDomain(r))
	}
	return out, nil
}

// ListMisalignedOCDispatches returns records whose state disagrees with their
// action row while the action row is still in a recoverable state
// (dispatched or unknown) -- the residue of a crash between the record write
// and the action write. The record is the linearization point, so the ACTION
// row is the one restored.
func (s *OCStore) ListMisalignedOCDispatches(ctx context.Context, limit int) ([]OCDispatchAlignment, error) {
	if limit <= 0 {
		return nil, ErrOCDispatchInvalid
	}
	var rows []struct {
		TenantID      uint64
		ActionID      string
		RuntimeID     string
		Key           string
		ExecutionID   string
		State         string
		ReservationID string
		FirstSentAt   time.Time
		ReplayUntil   time.Time
		Fence         int64
		ActionState   *string
		ActionFence   *int64
	}
	err := s.db.WithContext(ctx).Raw(
		"SELECT d.tenant_id, d.action_id, d.runtime_id, d.key, d.execution_id, d.state, "+
			"d.reservation_id, d.first_sent_at, d.replay_until, d.fence, "+
			"a.state AS action_state, a.fence AS action_fence "+
			"FROM connector_dispatch_records d "+
			"LEFT JOIN app_actions a ON a.tenant_id = d.tenant_id AND a.id = d.action_id "+
			"WHERE (a.state = ? AND d.state <> ?) OR (a.state = ? AND d.state IN ?) "+
			"ORDER BY d.updated_at ASC LIMIT ?",
		appconnector.ActionDispatched, appconnector.ActionDispatched,
		appconnector.ActionUnknown, ocDispatchTerminalStates(), limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]OCDispatchAlignment, 0, len(rows))
	for _, r := range rows {
		al := OCDispatchAlignment{
			Record: appconnector.OCDispatchRecord{
				TenantID: r.TenantID, ActionID: r.ActionID, RuntimeID: r.RuntimeID,
				Key: r.Key, ExecutionID: r.ExecutionID, State: r.State,
				ReservationID: r.ReservationID, FirstSentAt: r.FirstSentAt,
				ReplayUntil: r.ReplayUntil, Fence: r.Fence,
			},
		}
		if r.ActionState != nil {
			al.ActionState = *r.ActionState
		}
		if r.ActionFence != nil {
			al.ActionFence = *r.ActionFence
		}
		out = append(out, al)
	}
	return out, nil
}

// ListUnknownOCDispatches returns records parked in unknown whose last state
// change is not older than window, for the read-only provider-query pass.
// A record the provider keeps answering "unknown" never refreshes its
// updated_at, so it naturally ages out of this scan instead of being
// re-queried forever.
func (s *OCStore) ListUnknownOCDispatches(ctx context.Context, now time.Time, window time.Duration, limit int) ([]appconnector.OCDispatchRecord, error) {
	if now.IsZero() || window <= 0 || limit <= 0 {
		return nil, ErrOCDispatchInvalid
	}
	var rows []OCDispatchRecordRow
	if err := s.db.WithContext(ctx).
		Where("state = ? AND updated_at >= ?", appconnector.ActionUnknown, now.UTC().Add(-window)).
		Order("updated_at ASC").Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]appconnector.OCDispatchRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, ocDispatchRecordDomain(r))
	}
	return out, nil
}

// ListOCDispatchSettlementsDue returns terminal records carrying a
// reservation whose settlement has not been marked delivered, and whose
// terminal write is not older than window. "Not delivered" is the fence
// invariant: the claim leaves record.fence == action.fence, and the delivery
// mark raises record.fence above it. Rows older than the window stop being
// retried -- a settlement that kept failing that long belongs to the EXISTING
// commercial recovery, not a new ledger here.
func (s *OCStore) ListOCDispatchSettlementsDue(ctx context.Context, now time.Time, window time.Duration, limit int) ([]appconnector.OCDispatchRecord, error) {
	if now.IsZero() || window <= 0 || limit <= 0 {
		return nil, ErrOCDispatchInvalid
	}
	var rows []OCDispatchRecordRow
	err := s.db.WithContext(ctx).
		Table("connector_dispatch_records").
		Select("connector_dispatch_records.*").
		Joins("LEFT JOIN app_actions a ON a.tenant_id = connector_dispatch_records.tenant_id AND a.id = connector_dispatch_records.action_id").
		Where("connector_dispatch_records.state IN ?", ocDispatchTerminalStates()).
		Where("connector_dispatch_records.reservation_id <> ''").
		Where("connector_dispatch_records.updated_at >= ?", now.UTC().Add(-window)).
		Where("connector_dispatch_records.fence <= COALESCE(a.fence, connector_dispatch_records.fence)").
		Order("connector_dispatch_records.updated_at ASC").Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]appconnector.OCDispatchRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, ocDispatchRecordDomain(r))
	}
	return out, nil
}

// MarkOCDispatchSettled records that a terminal record's settlement was
// DELIVERED, by bumping the record's fence past the action row's fence under
// a terminal-state + fence CAS. ErrOCDispatchConflict means the row was
// already marked (or moved on) -- success for a duplicate mark. The mark is
// written only AFTER the settlement delivery: a crash in between leaves the
// row due again, and the next pass re-delivers the IDENTICAL fact, which the
// commercial settlement ledger treats as an idempotent replay.
func (s *OCStore) MarkOCDispatchSettled(ctx context.Context, tenant uint64, actionID string, fence int64, now time.Time) error {
	if tenant == 0 || actionID == "" || fence < 1 || now.IsZero() {
		return ErrOCDispatchInvalid
	}
	res := s.db.WithContext(ctx).Model(&OCDispatchRecordRow{}).
		Where("tenant_id = ? AND action_id = ? AND state IN ? AND fence = ?",
			tenant, actionID, ocDispatchTerminalStates(), fence).
		Updates(map[string]interface{}{
			"fence":      gorm.Expr("fence + 1"),
			"updated_at": now.UTC(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrOCDispatchConflict
	}
	return nil
}

// NoteOCProviderRetryAfter durably stores a provider's 429 Retry-After
// instant (fail-closed until it passes). The value only moves FORWARD — a
// stale observation can never shorten a newer cooldown. Releasing the
// cooldown happens by time passing, never by a write.
func (s *OCStore) NoteOCProviderRetryAfter(ctx context.Context, provider string, retryAfter, now time.Time) error {
	if provider == "" || retryAfter.IsZero() || now.IsZero() {
		return ErrOCLeaseInvalid
	}
	retryAfter, now = retryAfter.UTC(), now.UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			"INSERT INTO connector_provider_retry_state (provider, retry_after, observed_at) VALUES (?, ?, ?) ON CONFLICT (provider) DO NOTHING",
			provider, retryAfter, now).Error; err != nil {
			return err
		}
		res := tx.Exec(
			"UPDATE connector_provider_retry_state SET retry_after = ?, observed_at = ? WHERE provider = ? AND retry_after <= ?",
			retryAfter, now, provider, retryAfter)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// The stored instant is at or after the new one: keep it.
			return ErrOCDispatchConflict
		}
		return nil
	})
}

// OCProviderRetryAfter reports how much longer a provider is cooling down;
// zero means dispatches may proceed.
func (s *OCStore) OCProviderRetryAfter(ctx context.Context, provider string, now time.Time) (time.Duration, error) {
	if provider == "" || now.IsZero() {
		return 0, ErrOCLeaseInvalid
	}
	var until *time.Time
	if err := s.db.WithContext(ctx).Raw(
		"SELECT retry_after FROM connector_provider_retry_state WHERE provider = ?", provider).Scan(&until).Error; err != nil {
		return 0, err
	}
	if until == nil {
		return 0, nil
	}
	if d := until.Sub(now.UTC()); d > 0 {
		return d, nil
	}
	return 0, nil
}
