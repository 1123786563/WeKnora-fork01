package commercial

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidBudgetRequest      = errors.New("invalid_budget_request")
	ErrBudgetAccountMissing      = errors.New("budget_account_missing")
	ErrTaskBudgetMissing         = errors.New("task_budget_missing")
	ErrTaskBudgetRootConflict    = errors.New("task_budget_root_conflict")
	ErrTaskBudgetExpired         = errors.New("task_budget_expired")
	ErrBudgetVerificationExpired = errors.New("budget_verification_expired")
	ErrInsufficientBudget        = errors.New("insufficient_budget")
	ErrTaskBudgetExhausted       = errors.New("task_budget_exhausted")
	ErrBudgetLotsInsufficient    = errors.New("budget_lots_insufficient")
	ErrReservationKeyConflict    = errors.New("reservation_key_conflict")
	ErrStaleWatermark            = errors.New("stale_watermark")
	ErrBudgetContention          = errors.New("budget_contention")
)

// errBudgetCASRetry is an internal sentinel: a guarded UPDATE lost the
// version race (or hit a transient unique-index race), the whole
// transaction rolled back, and the caller must re-read and retry. The try*
// helpers consume it with errors.Is to continue their retry loops on fresh
// reads, so it never crosses the API surface.
var errBudgetCASRetry = errors.New("budget_cas_retry")

// budgetCASAttempts bounds the re-read/retry rounds of one Reserve or
// LockRefunds call. Cross-process safety comes from the database CAS
// conditions themselves, never from a process-local mutex.
const budgetCASAttempts = 32

// BudgetAccountRow is the tenant's balance projection. Every transition —
// reservation holds, refund locks, external balance folds — happens through
// a version-CAS UPDATE on this row, so the version also guards the
// watermark: any reconciliation that advances the watermark bumps the
// version and forces concurrent reservers to re-read before accepting.
type BudgetAccountRow struct {
	TenantID          uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	VerifiedMicro     int64     `gorm:"column:verified_micro;not null;default:0"`
	UnreflectedMicro  int64     `gorm:"column:unreflected_micro;not null;default:0"`
	HeldMicro         int64     `gorm:"column:held_micro;not null;default:0"`
	RefundLockedMicro int64     `gorm:"column:refund_locked_micro;not null;default:0"`
	Watermark         string    `gorm:"column:watermark;not null;default:''"`
	Version           int64     `gorm:"column:version;not null;default:1"`
	VerifiedUntil     time.Time `gorm:"column:verified_until;not null"`
}

func (BudgetAccountRow) TableName() string { return "commercial_budget_accounts" }

// TaskBudgetRow caps one run's spend. RootRunID implements the parent/child
// no-copy rule: a child run's row is a mapping that points at the parent's
// budget row and carries NO independent limit, so a delegated child never
// duplicates its parent's budget. An empty RootRunID marks the owner row.
type TaskBudgetRow struct {
	TenantID   uint64    `gorm:"column:tenant_id;not null;uniqueIndex:uq_commercial_task_budget_run,priority:1"`
	RunID      string    `gorm:"column:run_id;not null;uniqueIndex:uq_commercial_task_budget_run,priority:2"`
	RootRunID  string    `gorm:"column:root_run_id;not null;default:''"`
	LimitMicro int64     `gorm:"column:limit_micro;not null;default:0"`
	SpentMicro int64     `gorm:"column:spent_micro;not null;default:0"`
	HeldMicro  int64     `gorm:"column:held_micro;not null;default:0"`
	Deadline   time.Time `gorm:"column:deadline;not null"`
	Version    int64     `gorm:"column:version;not null;default:1"`
}

func (TaskBudgetRow) TableName() string { return "commercial_task_budgets" }

// ReservationRow is one accepted hold, unique per (tenant, key): replaying
// the same key is resolved idempotently, never double-held. Fence is the
// single-use token later settlement must present.
type ReservationRow struct {
	TenantID   uint64    `gorm:"column:tenant_id;not null;uniqueIndex:uq_commercial_reservation_key,priority:1"`
	Key        string    `gorm:"column:key;not null;uniqueIndex:uq_commercial_reservation_key,priority:2"`
	RunID      string    `gorm:"column:run_id;not null"`
	UpperMicro int64     `gorm:"column:upper_micro;not null"`
	State      string    `gorm:"column:state;not null"`
	Deadline   time.Time `gorm:"column:deadline;not null"`
	Fence      string    `gorm:"column:fence;not null"`
	// Owner names the worker currently holding the lease (U04): a lease
	// takeover rewrites it together with a fresh fence, so a stale worker
	// that lost the lease can no longer commit results against this
	// reservation.
	Owner     string    `gorm:"column:owner;not null;default:''"`
	Version   int64     `gorm:"column:version;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (ReservationRow) TableName() string { return "commercial_reservations" }

func (r ReservationRow) toDomain() domain.Reservation {
	return domain.Reservation{
		ID:       r.Key,
		RunID:    r.RunID,
		State:    r.State,
		Upper:    domain.Credits(r.UpperMicro),
		Deadline: r.Deadline,
		Version:  r.Version,
	}
}

// BudgetLotRow is one issuance batch of credits. Holds are allocated from
// the earliest-expiring lot first so short-lived credits are consumed
// before they evaporate.
type BudgetLotRow struct {
	TenantID       uint64     `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	LotID          string     `gorm:"column:lot_id;primaryKey"`
	RemainingMicro int64      `gorm:"column:remaining_micro;not null;default:0"`
	HeldMicro      int64      `gorm:"column:held_micro;not null;default:0"`
	ExpiresAt      *time.Time `gorm:"column:expires_at"`
	IssuedAt       time.Time  `gorm:"column:issued_at;not null"`
}

func (BudgetLotRow) TableName() string { return "commercial_budget_lots" }

// BudgetLotAllocationRow records which lot backs which reservation, so a
// later settlement or expiry releases exactly the lot capacity it took.
type BudgetLotAllocationRow struct {
	TenantID       uint64 `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	LotID          string `gorm:"column:lot_id;primaryKey"`
	ReservationKey string `gorm:"column:reservation_key;primaryKey"`
	Micro          int64  `gorm:"column:micro;not null"`
}

func (BudgetLotAllocationRow) TableName() string { return "commercial_budget_lot_allocations" }

// BudgetStore reserves task budgets atomically across workers and
// processes. All coordination is database version-CAS inside one
// transaction; there is deliberately no process mutex.
type BudgetStore struct {
	db *gorm.DB
}

func NewBudgetStore(db *gorm.DB) *BudgetStore { return &BudgetStore{db: db} }

func newReservationFence() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func minI64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// EnsureTaskBudget registers the owner budget row of a run. Replaying the
// same run is an idempotent no-op; an existing row is never mutated here.
func (s *BudgetStore) EnsureTaskBudget(ctx context.Context, tenantID uint64, runID string, limit domain.Credits, deadline time.Time) error {
	if tenantID == 0 || runID == "" || limit < 0 || deadline.IsZero() {
		return ErrInvalidBudgetRequest
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
		Create(&TaskBudgetRow{TenantID: tenantID, RunID: runID, LimitMicro: int64(limit), Deadline: deadline.UTC(), Version: 1}).Error
}

// AttachChildRun records that childRun charges the budget owned by
// parentRun. The child row carries a zero limit and only points at the
// parent's row: a delegated child counts against its parent's budget
// exactly once and never receives a copied budget of its own.
func (s *BudgetStore) AttachChildRun(ctx context.Context, tenantID uint64, childRun, parentRun string) error {
	if tenantID == 0 || childRun == "" || parentRun == "" || childRun == parentRun {
		return ErrInvalidBudgetRequest
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent TaskBudgetRow
		err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, parentRun).First(&parent).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		root := parent.RootRunID
		if root == "" {
			root = parent.RunID
		}
		var existing TaskBudgetRow
		err = tx.Where("tenant_id = ? AND run_id = ?", tenantID, childRun).First(&existing).Error
		if err == nil {
			if existing.RootRunID == root {
				return nil
			}
			return ErrTaskBudgetRootConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&TaskBudgetRow{TenantID: tenantID, RunID: childRun, RootRunID: root, Deadline: parent.Deadline, Version: 1}).Error
	})
}

// Reserve holds req.Upper credits for one keyed call of one run, atomically
// across workers and processes. In a single transaction it CAS-updates the
// account and task counters (account availability, task headroom, deadline
// validity, watermark-stable version), inserts the reservation, and
// allocates lot capacity earliest-expiry-first. Any guarded step affecting
// a row count other than 1 rolls the whole transaction back for a re-read;
// a guard that still fails on fresh data is a rejection, not a retry.
// Replaying an identical (tenant, key) is idempotent.
func (s *BudgetStore) Reserve(ctx context.Context, req domain.BudgetRequest) (domain.Reservation, error) {
	if req.TenantID == 0 || req.RunID == "" || req.Key == "" || req.Upper <= 0 {
		return domain.Reservation{}, ErrInvalidBudgetRequest
	}
	if req.Deadline.IsZero() || !req.Deadline.After(time.Now()) {
		return domain.Reservation{}, ErrInvalidBudgetRequest
	}
	var outcome *domain.Reservation
	for attempt := 0; attempt < budgetCASAttempts && outcome == nil; attempt++ {
		res, retry, err := s.tryReserve(ctx, req)
		if err != nil {
			return domain.Reservation{}, err
		}
		if !retry {
			outcome = &res
		}
	}
	if outcome == nil {
		return domain.Reservation{}, ErrBudgetContention
	}
	return *outcome, nil
}

func (s *BudgetStore) tryReserve(ctx context.Context, req domain.BudgetRequest) (domain.Reservation, bool, error) {
	now := time.Now().UTC()
	var out domain.Reservation
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Idempotent replay: an existing identical hold succeeds without
		// touching counters; the same key with different content conflicts.
		var existing ReservationRow
		err := tx.Where("tenant_id = ? AND key = ?", req.TenantID, req.Key).First(&existing).Error
		if err == nil {
			if existing.RunID == req.RunID && domain.Credits(existing.UpperMicro) == req.Upper && existing.State == domain.ReservationStateHeld {
				out = existing.toDomain()
				return nil
			}
			return ErrReservationKeyConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Account hold: guarded on the read version, projected availability,
		// and the verification window. The version guard is also the
		// watermark acceptance: any reconciliation that moves the watermark
		// bumps the version, forcing this reservation to re-read first.
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", req.TenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		res := tx.Exec(`UPDATE commercial_budget_accounts
			SET held_micro = held_micro + ?, version = version + 1
			WHERE tenant_id = ? AND version = ?
			  AND verified_micro - unreflected_micro - held_micro - refund_locked_micro >= ?
			  AND verified_until > ?`,
			int64(req.Upper), req.TenantID, acct.Version, int64(req.Upper), now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return budgetAccountDenial(tx, req.TenantID, int64(req.Upper), now)
		}

		// Task budget: a child run resolves to its parent's owner row, so
		// parent and child draws share ONE budget and never duplicate it.
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", req.TenantID, req.RunID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		ownerRun := task.RunID
		if task.RootRunID != "" {
			ownerRun = task.RootRunID
			if err := tx.Where("tenant_id = ? AND run_id = ?", req.TenantID, ownerRun).First(&task).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTaskBudgetMissing
				}
				return err
			}
		}
		res = tx.Exec(`UPDATE commercial_task_budgets
			SET held_micro = held_micro + ?, version = version + 1
			WHERE tenant_id = ? AND run_id = ? AND version = ?
			  AND limit_micro - spent_micro - held_micro >= ?
			  AND deadline > ?`,
			int64(req.Upper), req.TenantID, ownerRun, task.Version, int64(req.Upper), now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return budgetTaskDenial(tx, req.TenantID, ownerRun, int64(req.Upper), now)
		}

		// Reservation row with its fence. A unique-index loss here means a
		// concurrent identical key committed first: roll back and resolve
		// the replay on the next pass.
		fence, err := newReservationFence()
		if err != nil {
			return fmt.Errorf("fence: %w", err)
		}
		row := ReservationRow{
			TenantID:   req.TenantID,
			Key:        req.Key,
			RunID:      req.RunID,
			UpperMicro: int64(req.Upper),
			State:      domain.ReservationStateHeld,
			Deadline:   req.Deadline.UTC(),
			Fence:      fence,
			Version:    1,
			CreatedAt:  now,
		}
		if err := tx.Create(&row).Error; err != nil {
			// A unique-index loss (or any transient insert failure) must
			// roll back the account/task increments already applied in
			// this transaction: return the sentinel so GORM rolls the
			// whole attempt back; returning nil would COMMIT partial
			// state.
			return errBudgetCASRetry
		}

		// Lot allocation, earliest-expiry-first, same transaction.
		var lots []BudgetLotRow
		if err := tx.Raw(`SELECT * FROM commercial_budget_lots
			WHERE tenant_id = ? AND remaining_micro - held_micro > 0
			  AND (expires_at IS NULL OR expires_at > ?)
			ORDER BY expires_at ASC`, req.TenantID, now).Scan(&lots).Error; err != nil {
			return err
		}
		need := int64(req.Upper)
		for _, lot := range lots {
			if need == 0 {
				break
			}
			take := minI64(lot.RemainingMicro-lot.HeldMicro, need)
			if take <= 0 {
				continue
			}
			res := tx.Exec(`UPDATE commercial_budget_lots
				SET held_micro = held_micro + ?
				WHERE tenant_id = ? AND lot_id = ? AND remaining_micro - held_micro >= ?`,
				take, req.TenantID, lot.LotID, take)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				// A concurrent hold took this lot's capacity between the
				// read and the guarded UPDATE: return the sentinel so GORM
				// rolls back every write of this attempt (earlier lot
				// holds included); returning nil would COMMIT partial
				// state and later replay it as success.
				return errBudgetCASRetry
			}
			if err := tx.Create(&BudgetLotAllocationRow{TenantID: req.TenantID, LotID: lot.LotID, ReservationKey: req.Key, Micro: take}).Error; err != nil {
				return err
			}
			need -= take
		}
		if need > 0 {
			return ErrBudgetLotsInsufficient
		}
		out = row.toDomain()
		return nil
	})
	if err != nil {
		// The sentinel means the transaction rolled back whole after a
		// guarded miss or transient race: consume it here so the retry
		// loop re-reads fresh data — it never crosses the API surface.
		if errors.Is(err, errBudgetCASRetry) {
			return domain.Reservation{}, true, nil
		}
		return domain.Reservation{}, false, err
	}
	return out, false, nil
}

// LockRefunds locks amount credits against pending refunds in the same
// CAS-disciplined transaction a reservation uses: the guard requires the
// projected availability to still cover the lock, so refund locks and
// reservations compete for one projection and never overdraw it together.
func (s *BudgetStore) LockRefunds(ctx context.Context, tenantID uint64, amount domain.Credits) error {
	if tenantID == 0 || amount <= 0 {
		return ErrInvalidBudgetRequest
	}
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := s.tryLockRefunds(ctx, tenantID, amount)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}

func (s *BudgetStore) tryLockRefunds(ctx context.Context, tenantID uint64, amount domain.Credits) (bool, error) {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		res := tx.Exec(`UPDATE commercial_budget_accounts
			SET refund_locked_micro = refund_locked_micro + ?, version = version + 1
			WHERE tenant_id = ? AND version = ?
			  AND verified_micro - unreflected_micro - held_micro - refund_locked_micro >= ?
			  AND verified_until > ?`,
			int64(amount), tenantID, acct.Version, int64(amount), now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return budgetAccountDenial(tx, tenantID, int64(amount), now)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// ApplyExternalBalance folds an independently read external balance into
// the local projection. It may advance verified_micro and the watermark —
// under version CAS and a monotonic-watermark guard — but it NEVER
// overwrites unreflected_micro, held_micro, or refund_locked_micro: those
// are local in-flight facts an external balance reader cannot see, and
// blindly refreshing them would erase unacknowledged spend.
func (s *BudgetStore) ApplyExternalBalance(ctx context.Context, tenantID uint64, verified domain.Credits, watermark string) error {
	if tenantID == 0 || verified < 0 || watermark == "" {
		return ErrInvalidBudgetRequest
	}
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := s.tryApplyExternalBalance(ctx, tenantID, verified, watermark)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}

func (s *BudgetStore) tryApplyExternalBalance(ctx context.Context, tenantID uint64, verified domain.Credits, watermark string) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		if watermark < acct.Watermark {
			return ErrStaleWatermark
		}
		if watermark == acct.Watermark {
			if domain.Credits(acct.VerifiedMicro) == verified {
				return nil // identical replay
			}
			return ErrStaleWatermark
		}
		res := tx.Exec(`UPDATE commercial_budget_accounts
			SET verified_micro = ?, watermark = ?, version = version + 1
			WHERE tenant_id = ? AND version = ?`,
			int64(verified), watermark, tenantID, acct.Version)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// budgetAccountDenial re-reads the account after a failed guard and turns
// the failure into either a hard rejection (expired verification, genuine
// insufficiency) or a retry sentinel (the guard would pass on fresh data —
// we merely lost the version race).
func budgetAccountDenial(tx *gorm.DB, tenantID uint64, upper int64, now time.Time) error {
	var fresh BudgetAccountRow
	if err := tx.Where("tenant_id = ?", tenantID).First(&fresh).Error; err != nil {
		return err
	}
	if !fresh.VerifiedUntil.After(now) {
		return ErrBudgetVerificationExpired
	}
	if fresh.VerifiedMicro-fresh.UnreflectedMicro-fresh.HeldMicro-fresh.RefundLockedMicro >= upper {
		return errBudgetCASRetry
	}
	return ErrInsufficientBudget
}

// ErrReservationNotFound rejects settlement of a reservation the store has
// no record of; ErrBudgetProtectionShortfall rejects releasing more
// unreflected protection than the account carries (a double release).
var (
	ErrReservationNotFound       = errors.New("reservation_not_found")
	ErrBudgetProtectionShortfall = errors.New("budget_protection_shortfall")
)

// SettleReservationHold is the U03 Finalize accounting, run as ONE local
// transaction with the caller's persist closure (usage fact, settlement
// record, settlement outbox event): the reservation's consumed part moves
// held → unreflected, the actually-unused part is released, the task budget
// converts its hold the same way, and lot capacity is returned. On the
// first settlement of an attempt the hold (res.Upper) is released and delta
// (the revision's charge) is protected; a later correction revision of the
// same attempt passes only its delta — external corrections and negative
// reversals adjust the protection without re-releasing the hold. The
// unreflected part stays protected until ApplyConfirmedSettlement runs under
// an explicit remote confirmation. Version-CAS guarded misses roll the WHOLE
// transaction back (persist writes included) and retry on fresh reads — the
// same errBudgetCASRetry discipline as Reserve/LockRefunds.
func (s *BudgetStore) SettleReservationHold(ctx context.Context, tenantID uint64, reservationKey string, delta domain.Credits, persist func(tx *gorm.DB) error) error {
	if tenantID == 0 || reservationKey == "" {
		return ErrInvalidBudgetRequest
	}
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := s.trySettleReservationHold(ctx, tenantID, reservationKey, delta, persist)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}

func (s *BudgetStore) trySettleReservationHold(ctx context.Context, tenantID uint64, reservationKey string, delta domain.Credits, persist func(tx *gorm.DB) error) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var res ReservationRow
		if err := tx.Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReservationNotFound
			}
			return err
		}
		first := res.State == domain.ReservationStateHeld
		switch res.State {
		case domain.ReservationStateHeld, domain.ReservationStateSettled:
		default:
			return ErrReservationKeyConflict
		}
		if res.State == domain.ReservationStateSettled && first {
			return ErrReservationKeyConflict // unreachable; keeps the invariant explicit
		}
		// Caller's durable writes (fact, settlement record, outbox event)
		// commit atomically with the accounting below — or roll back with it.
		if persist != nil {
			if err := persist(tx); err != nil {
				return err
			}
		}

		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		if first {
			// Consumed part held → unreflected; unused part released. Net
			// availability only grows (upper ≥ delta), so the guard is the
			// version CAS plus hold coverage; a miss is a retry, never a
			// partial commit.
			r := tx.Exec(`UPDATE commercial_budget_accounts
				SET held_micro = held_micro - ?, unreflected_micro = unreflected_micro + ?, version = version + 1
				WHERE tenant_id = ? AND version = ? AND held_micro >= ? AND unreflected_micro + ? >= 0`,
				res.UpperMicro, int64(delta), tenantID, acct.Version, res.UpperMicro, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		} else if delta != 0 {
			r := tx.Exec(`UPDATE commercial_budget_accounts
				SET unreflected_micro = unreflected_micro + ?, version = version + 1
				WHERE tenant_id = ? AND version = ? AND unreflected_micro + ? >= 0`,
				int64(delta), tenantID, acct.Version, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		}

		// Task budget resolves to the owner row exactly as Reserve does, so
		// the converted spend counts against its parent budget once.
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, res.RunID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		ownerRun := task.RunID
		if task.RootRunID != "" {
			ownerRun = task.RootRunID
			if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, ownerRun).First(&task).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTaskBudgetMissing
				}
				return err
			}
		}
		if first {
			r := tx.Exec(`UPDATE commercial_task_budgets
				SET held_micro = held_micro - ?, spent_micro = spent_micro + ?, version = version + 1
				WHERE tenant_id = ? AND run_id = ? AND version = ? AND held_micro >= ? AND spent_micro + ? >= 0`,
				res.UpperMicro, int64(delta), tenantID, ownerRun, task.Version, res.UpperMicro, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		} else if delta != 0 {
			r := tx.Exec(`UPDATE commercial_task_budgets
				SET spent_micro = spent_micro + ?, version = version + 1
				WHERE tenant_id = ? AND run_id = ? AND version = ? AND spent_micro + ? >= 0`,
				int64(delta), tenantID, ownerRun, task.Version, int64(delta))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		}

		if first {
			// Lot capacity: every allocation's hold is returned; the
			// consumed part reduces lot remaining earliest-expiry-first, and
			// the allocation rows are deleted. Corrections do not touch
			// lots — the hold was already returned by the first settlement.
			var allocs []BudgetLotAllocationRow
			if err := tx.Raw(`SELECT a.* FROM commercial_budget_lot_allocations a
				JOIN commercial_budget_lots l ON l.tenant_id = a.tenant_id AND l.lot_id = a.lot_id
				WHERE a.tenant_id = ? AND a.reservation_key = ?
				ORDER BY l.expires_at ASC`, tenantID, reservationKey).Scan(&allocs).Error; err != nil {
				return err
			}
			consume := int64(delta)
			for _, a := range allocs {
				take := minI64(a.Micro, consume)
				r := tx.Exec(`UPDATE commercial_budget_lots
					SET held_micro = held_micro - ?, remaining_micro = remaining_micro - ?
					WHERE tenant_id = ? AND lot_id = ? AND held_micro >= ? AND remaining_micro >= ?`,
					a.Micro, take, tenantID, a.LotID, a.Micro, take)
				if r.Error != nil {
					return r.Error
				}
				if r.RowsAffected != 1 {
					return errBudgetCASRetry
				}
				if err := tx.Where("tenant_id = ? AND lot_id = ? AND reservation_key = ?",
					tenantID, a.LotID, reservationKey).Delete(&BudgetLotAllocationRow{}).Error; err != nil {
					return err
				}
				consume -= take
			}
			r := tx.Exec(`UPDATE commercial_reservations
				SET state = ?, version = version + 1
				WHERE tenant_id = ? AND key = ? AND state = ?`,
				domain.ReservationStateSettled, tenantID, reservationKey, domain.ReservationStateHeld)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// ApplyConfirmedSettlement releases, in ONE transaction with the caller's
// persist closure (the settlement record flipping to confirmed), the
// protection covered by an explicitly confirmed settlement: the watermark
// advances monotonically, the protected (unreflected) amount is removed, and
// verified drops by the same amount — the confirmation's correlation
// evidence proves the provider already consumed it, so releasing protection
// without dropping verified would transiently re-admit spent credits. An
// older watermark never rewinds anything (ErrStaleWatermark); an equal
// watermark is an idempotent replay.
func (s *BudgetStore) ApplyConfirmedSettlement(ctx context.Context, tenantID uint64, amount domain.Credits, watermark string, persist func(tx *gorm.DB) error) error {
	if tenantID == 0 || amount < 0 || watermark == "" {
		return ErrInvalidBudgetRequest
	}
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := s.tryApplyConfirmedSettlement(ctx, tenantID, amount, watermark, persist)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}

func (s *BudgetStore) tryApplyConfirmedSettlement(ctx context.Context, tenantID uint64, amount domain.Credits, watermark string, persist func(tx *gorm.DB) error) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		if watermark < acct.Watermark {
			return ErrStaleWatermark
		}
		if watermark == acct.Watermark {
			return nil // identical replay: the protection is already released
		}
		if int64(amount) > acct.UnreflectedMicro {
			return ErrBudgetProtectionShortfall
		}
		if persist != nil {
			if err := persist(tx); err != nil {
				return err
			}
		}
		r := tx.Exec(`UPDATE commercial_budget_accounts
			SET unreflected_micro = unreflected_micro - ?, verified_micro = verified_micro - ?, watermark = ?, version = version + 1
			WHERE tenant_id = ? AND version = ? AND unreflected_micro >= ?`,
			int64(amount), int64(amount), watermark, tenantID, acct.Version, int64(amount))
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// budgetTaskDenial is the task-budget counterpart of budgetAccountDenial.
func budgetTaskDenial(tx *gorm.DB, tenantID uint64, ownerRun string, upper int64, now time.Time) error {
	var fresh TaskBudgetRow
	if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, ownerRun).First(&fresh).Error; err != nil {
		return err
	}
	if !fresh.Deadline.After(now) {
		return ErrTaskBudgetExpired
	}
	if fresh.LimitMicro-fresh.SpentMicro-fresh.HeldMicro >= upper {
		return errBudgetCASRetry
	}
	return ErrTaskBudgetExhausted
}

// ---- U04: cancel, expiry, extension and lease-recovery primitives ----
// All additions below are additive: Reserve, LockRefunds, and the
// settlement methods above are untouched; the new primitives reuse the
// same errBudgetCASRetry roll-back-whole-transaction discipline.

var (
	// ErrReservationNotHeld rejects releasing or taking over a reservation
	// that is no longer unstarted: dispatched/settling/settled reservations
	// may carry external spend and are NEVER zeroed by cancel or expiry.
	ErrReservationNotHeld = errors.New("reservation_not_held")
	// ErrLeaseFenceStale rejects a commit presented by a worker whose lease
	// was taken over: the stored owner/fence pair moved on, so the stale
	// worker result must be discarded.
	ErrLeaseFenceStale = errors.New("lease_fence_stale")
)

// TaskBudgetExtensionRow records one applied task-limit extension, unique
// per (tenant, run, key): replaying the same idempotency key never
// increases the limit a second time.
type TaskBudgetExtensionRow struct {
	TenantID   uint64    `gorm:"column:tenant_id;primaryKey;autoIncrement:false"`
	RunID      string    `gorm:"column:run_id;primaryKey"`
	Key        string    `gorm:"column:key;primaryKey"`
	ExtraMicro int64     `gorm:"column:extra_micro;not null"`
	AppliedAt  time.Time `gorm:"column:applied_at;not null"`
}

func (TaskBudgetExtensionRow) TableName() string { return "commercial_task_budget_extensions" }

// ReleaseReservation returns an unstarted reservation hold to the account,
// task budget, and lots in ONE transaction. The reservation flip is a CAS
// on BOTH state=held AND the read version: a concurrent dispatch that
// changed either makes the guarded UPDATE miss, rolls the whole attempt
// back, and the re-read then rejects with ErrReservationNotHeld. A
// reservation can therefore never be released after a concurrent dispatch
// won, and a second release of the same key rejects the same way, so there
// is no double free.
func (s *BudgetStore) ReleaseReservation(ctx context.Context, tenantID uint64, reservationKey string) error {
	if tenantID == 0 || reservationKey == "" {
		return ErrInvalidBudgetRequest
	}
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := s.tryReleaseReservation(ctx, tenantID, reservationKey)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}

func (s *BudgetStore) tryReleaseReservation(ctx context.Context, tenantID uint64, reservationKey string) (bool, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var res ReservationRow
		if err := tx.Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReservationNotFound
			}
			return err
		}
		if res.State != domain.ReservationStateHeld {
			return ErrReservationNotHeld
		}
		r := tx.Exec(`UPDATE commercial_reservations
			SET state = ?, version = version + 1
			WHERE tenant_id = ? AND key = ? AND state = ? AND version = ?`,
			domain.ReservationStateReleased, tenantID, reservationKey,
			domain.ReservationStateHeld, res.Version)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		r = tx.Exec(`UPDATE commercial_budget_accounts
			SET held_micro = held_micro - ?, version = version + 1
			WHERE tenant_id = ? AND version = ? AND held_micro >= ?`,
			res.UpperMicro, tenantID, acct.Version, res.UpperMicro)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, res.RunID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		ownerRun := task.RunID
		if task.RootRunID != "" {
			ownerRun = task.RootRunID
			if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, ownerRun).First(&task).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTaskBudgetMissing
				}
				return err
			}
		}
		r = tx.Exec(`UPDATE commercial_task_budgets
			SET held_micro = held_micro - ?, version = version + 1
			WHERE tenant_id = ? AND run_id = ? AND version = ? AND held_micro >= ?`,
			res.UpperMicro, tenantID, ownerRun, task.Version, res.UpperMicro)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		var allocs []BudgetLotAllocationRow
		if err := tx.Where("tenant_id = ? AND reservation_key = ?", tenantID, reservationKey).Find(&allocs).Error; err != nil {
			return err
		}
		for _, a := range allocs {
			r := tx.Exec(`UPDATE commercial_budget_lots
				SET held_micro = held_micro - ?
				WHERE tenant_id = ? AND lot_id = ? AND held_micro >= ?`,
				a.Micro, tenantID, a.LotID, a.Micro)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return errBudgetCASRetry
			}
			if err := tx.Where("tenant_id = ? AND lot_id = ? AND reservation_key = ?",
				tenantID, a.LotID, reservationKey).Delete(&BudgetLotAllocationRow{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// MarkReservationDispatched flips an unstarted reservation to dispatched
// with a version bump in one guarded UPDATE. This is the CAS complement of
// ReleaseReservation: once dispatch wins, any concurrent release attempt
// re-reads state dispatched and rejects with ErrReservationNotHeld, so a
// dispatched reservation is never released (zeroed) by cancel or expiry.
func (s *BudgetStore) MarkReservationDispatched(ctx context.Context, tenantID uint64, reservationKey string) error {
	if tenantID == 0 || reservationKey == "" {
		return ErrInvalidBudgetRequest
	}
	r := s.db.WithContext(ctx).Exec(`UPDATE commercial_reservations
		SET state = ?, version = version + 1
		WHERE tenant_id = ? AND key = ? AND state = ?`,
		domain.ReservationStateDispatched, tenantID, reservationKey, domain.ReservationStateHeld)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrReservationNotHeld
	}
	return nil
}

// ExtendTaskLimit raises one run task budget limit by extra credits in ONE
// transaction that first verifies, under the account version CAS, that the
// CURRENT funded availability (verified minus unreflected, held, and
// refund-locked) covers the extension and the verification window is still
// open: a space balance alone is never authorization. Expired quota does
// NOT extend validity: the deadline column is never touched, and an
// already-expired task budget refuses with ErrTaskBudgetExpired. Idempotency
// is the extension key: replaying the same (tenant, run, key) never
// increases the limit twice.
func (s *BudgetStore) ExtendTaskLimit(ctx context.Context, tenantID uint64, runID, key string, extra domain.Credits) error {
	if tenantID == 0 || runID == "" || key == "" || extra <= 0 {
		return ErrInvalidBudgetRequest
	}
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := s.tryExtendTaskLimit(ctx, tenantID, runID, key, extra)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}

func (s *BudgetStore) tryExtendTaskLimit(ctx context.Context, tenantID uint64, runID, key string, extra domain.Credits) (bool, error) {
	now := time.Now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing TaskBudgetExtensionRow
		err := tx.Where("tenant_id = ? AND run_id = ? AND key = ?", tenantID, runID, key).First(&existing).Error
		if err == nil {
			return nil // idempotent replay: this key already applied once
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var acct BudgetAccountRow
		if err := tx.Where("tenant_id = ?", tenantID).First(&acct).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBudgetAccountMissing
			}
			return err
		}
		r := tx.Exec(`UPDATE commercial_budget_accounts
			SET version = version + 1
			WHERE tenant_id = ? AND version = ?
			  AND verified_micro - unreflected_micro - held_micro - refund_locked_micro >= ?
			  AND verified_until > ?`,
			tenantID, acct.Version, int64(extra), now)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return budgetAccountDenial(tx, tenantID, int64(extra), now)
		}
		var task TaskBudgetRow
		if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, runID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTaskBudgetMissing
			}
			return err
		}
		ownerRun := task.RunID
		if task.RootRunID != "" {
			ownerRun = task.RootRunID
			if err := tx.Where("tenant_id = ? AND run_id = ?", tenantID, ownerRun).First(&task).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTaskBudgetMissing
				}
				return err
			}
		}
		if !task.Deadline.After(now) {
			return ErrTaskBudgetExpired
		}
		r = tx.Exec(`UPDATE commercial_task_budgets
			SET limit_micro = limit_micro + ?, version = version + 1
			WHERE tenant_id = ? AND run_id = ? AND version = ?`,
			int64(extra), tenantID, ownerRun, task.Version)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		return tx.Create(&TaskBudgetExtensionRow{
			TenantID: tenantID, RunID: runID, Key: key, ExtraMicro: int64(extra), AppliedAt: now,
		}).Error
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// TakeoverLease moves a reservation lease to newOwner and increments the
// fence in ONE guarded transaction (state=held AND version CAS). The
// returned fence is the new single-use commit token: the previous worker
// stored owner/fence pair no longer matches, so its late commit is
// rejected by VerifyLeaseCommit. Recovery never lets two workers commit
// against one hold.
func (s *BudgetStore) TakeoverLease(ctx context.Context, tenantID uint64, reservationKey, newOwner string) (string, error) {
	if tenantID == 0 || reservationKey == "" || newOwner == "" {
		return "", ErrInvalidBudgetRequest
	}
	var outcome string
	for attempt := 0; attempt < budgetCASAttempts && outcome == ""; attempt++ {
		fence, retry, err := s.tryTakeoverLease(ctx, tenantID, reservationKey, newOwner)
		if err != nil {
			return "", err
		}
		if !retry {
			outcome = fence
		}
	}
	if outcome == "" {
		return "", ErrBudgetContention
	}
	return outcome, nil
}

func (s *BudgetStore) tryTakeoverLease(ctx context.Context, tenantID uint64, reservationKey, newOwner string) (string, bool, error) {
	var fence string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var res ReservationRow
		if err := tx.Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReservationNotFound
			}
			return err
		}
		if res.State != domain.ReservationStateHeld {
			return ErrReservationNotHeld
		}
		f, err := newReservationFence()
		if err != nil {
			return fmt.Errorf("fence: %w", err)
		}
		r := tx.Exec(`UPDATE commercial_reservations
			SET owner = ?, fence = ?, version = version + 1
			WHERE tenant_id = ? AND key = ? AND state = ? AND version = ?`,
			newOwner, f, tenantID, reservationKey, domain.ReservationStateHeld, res.Version)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errBudgetCASRetry
		}
		fence = f
		return nil
	})
	if err != nil {
		if errors.Is(err, errBudgetCASRetry) {
			return "", true, nil
		}
		return "", false, err
	}
	return fence, false, nil
}

// VerifyLeaseCommit checks that the owner/fence pair still matches the
// stored lease before a worker result may commit. A worker that lost a
// takeover fails with ErrLeaseFenceStale; a reservation no longer held
// fails with ErrReservationNotHeld.
func (s *BudgetStore) VerifyLeaseCommit(ctx context.Context, tenantID uint64, reservationKey, owner, fence string) error {
	if tenantID == 0 || reservationKey == "" || owner == "" || fence == "" {
		return ErrInvalidBudgetRequest
	}
	var res ReservationRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND key = ?", tenantID, reservationKey).First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrReservationNotFound
	}
	if err != nil {
		return err
	}
	if res.Owner != owner || res.Fence != fence {
		return ErrLeaseFenceStale
	}
	if res.State != domain.ReservationStateHeld {
		return ErrReservationNotHeld
	}
	return nil
}
