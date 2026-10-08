package appconnector

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var (
	// ErrPlanNotFound: no plan row for the (tenant, id) pair — a foreign
	// tenant's plan and a missing one are indistinguishable by design.
	ErrPlanNotFound = errors.New("plan_not_found")
	// ErrPlanState: an invalid plan shape or lifecycle transition —
	// zero-item create, a CAS lost race, an approval whose digest does
	// not match the plan row's digest.
	ErrPlanState = errors.New("plan_state_conflict")
)

// Plan lifecycle states persisted on app_action_plans.state. There is no
// stored terminal plan state: completion is PROJECTED from the per-item
// action rows, which are the authority.
const (
	PlanStateAwaitingApproval = "awaiting_approval"
	PlanStateAuthorized       = "authorized"
)

// ActionPlanRow is the durable multi-action Action Plan (CONTEXT.md
// 操作计划): one approval decision binding an ORDERED set of already-
// prepared actions under ONE plan digest. ExcludedJSON carries the
// approval-time exclusions (排除单项) as a JSON int array; per-item
// results stay on the authoritative app_actions rows — this row never
// duplicates them.
type ActionPlanRow struct {
	ID       string `gorm:"primaryKey;column:id"`
	TenantID uint64 `gorm:"primaryKey;column:tenant_id"`
	ActorID  string `gorm:"column:actor_id;not null"`
	Digest   string `gorm:"column:digest;not null"`
	State    string `gorm:"column:state;not null"`
	// ExcludedJSON is the approval-time exclusion set ('' or "[]" = none);
	// it is frozen at first approval — a rewrite after execution started
	// is refused at the service layer.
	ExcludedJSON string     `gorm:"column:excluded_json;not null;default:''"`
	ApprovedBy   string     `gorm:"column:approved_by;not null;default:''"`
	ApprovedAt   *time.Time `gorm:"column:approved_at"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (ActionPlanRow) TableName() string { return "app_action_plans" }

// ActionPlanItemRow is one ordered member of a plan: seq positions the
// item (1-based, contiguous); action_id references the authoritative
// app_actions row the item dispatches through.
type ActionPlanItemRow struct {
	TenantID  uint64 `gorm:"primaryKey;column:tenant_id"`
	PlanID    string `gorm:"primaryKey;column:plan_id"`
	Seq       int    `gorm:"primaryKey;column:seq"`
	ActionID  string `gorm:"column:action_id;not null"`
	CreatedAt time.Time
}

func (ActionPlanItemRow) TableName() string { return "app_action_plan_items" }

// PlanStore persists action plans and their ordered items.
type PlanStore struct{ db *gorm.DB }

// NewPlanStore builds a PlanStore over a gorm DB.
func NewPlanStore(db *gorm.DB) *PlanStore { return &PlanStore{db: db} }

// CreatePlan inserts the plan row and its ordered items in ONE
// transaction. A plan without items is structurally impossible — an
// approval decision over nothing is refused, never persisted.
func (s *PlanStore) CreatePlan(ctx context.Context, plan ActionPlanRow, items []ActionPlanItemRow) error {
	if plan.ID == "" || plan.TenantID == 0 || plan.ActorID == "" || plan.Digest == "" ||
		plan.State != PlanStateAwaitingApproval || len(items) == 0 {
		return ErrPlanState
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&plan).Error; err != nil {
			return err
		}
		return tx.Create(&items).Error
	})
}

// FindPlan loads a plan scoped to the tenant.
func (s *PlanStore) FindPlan(ctx context.Context, tenantID uint64, planID string) (ActionPlanRow, error) {
	var row ActionPlanRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, planID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ActionPlanRow{}, ErrPlanNotFound
		}
		return ActionPlanRow{}, err
	}
	return row, nil
}

// ListPlanItems returns the plan's items in seq order.
func (s *PlanStore) ListPlanItems(ctx context.Context, tenantID uint64, planID string) ([]ActionPlanItemRow, error) {
	var rows []ActionPlanItemRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND plan_id = ?", tenantID, planID).
		Order("seq ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ApprovePlan is the guarded approval CAS: it moves the plan to
// authorized ONLY when the presented digest equals the row's digest and
// the row is still in an approvable state. Zero rows affected (foreign
// digest or a lost race) is ErrPlanState — the service layer reads the
// row first to give a foreign digest its own mismatch sentinel.
//
// The exclusion-set freeze (排除集首次批准后冻结) is pinned INSIDE the
// CAS: a row already authorized can only be re-approved with the SAME
// recorded excluded_json. A concurrent approval carrying a different set
// (it read the row before the first approval committed) therefore finds
// zero rows — first writer wins, the frozen decision is never silently
// rewritten.
func (s *PlanStore) ApprovePlan(ctx context.Context, tenantID uint64, planID, digest, actor, excludedJSON string, now time.Time) error {
	if planID == "" || digest == "" || actor == "" {
		return ErrPlanState
	}
	res := s.db.WithContext(ctx).Model(&ActionPlanRow{}).
		Where("tenant_id = ? AND id = ? AND digest = ? AND (state = ? OR excluded_json = ?)",
			tenantID, planID, digest, PlanStateAwaitingApproval, excludedJSON).
		Updates(map[string]interface{}{
			"state":         PlanStateAuthorized,
			"excluded_json": excludedJSON,
			"approved_by":   actor,
			"approved_at":   now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPlanState
	}
	return nil
}
