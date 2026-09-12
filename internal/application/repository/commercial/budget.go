package commercial

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
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

// casRetry is the SHARED transaction/retry helper layer for every budget
// dimension: it runs one guarded try* transaction up to budgetCASAttempts
// rounds, re-reading fresh state after each version-CAS loss. The split
// dimension files (task, reservation, refund-lock, account, settlement,
// lease) all funnel through this helper so the retry discipline lives in
// exactly one place.
func (s *BudgetStore) casRetry(try func() (bool, error)) error {
	for attempt := 0; attempt < budgetCASAttempts; attempt++ {
		retry, err := try()
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrBudgetContention
}
