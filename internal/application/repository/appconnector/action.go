package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"gorm.io/gorm"
)

var (
	// ErrActionNotFound is returned when an action id is unknown.
	ErrActionNotFound = errors.New("action_not_found")
	// ErrActionState is returned on an invalid lifecycle transition or a
	// lost compare-and-swap on state/fence.
	ErrActionState = errors.New("action_state_conflict")
	// ErrApprovalExhausted is returned when no unconsumed, unexpired
	// approval remains for an action's digest (concurrent claim lost, count
	// consumed, or expired).
	ErrApprovalExhausted = errors.New("approval_exhausted")
)

// ActionRow persists one action with the exact snapshot it was approved
// under. ArgsSnapshot holds the NORMALIZED argument bytes — the same bytes
// covered by ArgsDigest and the only bytes a dispatch may ever send.
// Fence increments on every dispatch-intent write so a stale worker that
// lost a lease can never overwrite a newer dispatch outcome.
type ActionRow struct {
	ID             string `gorm:"primaryKey;column:id"`
	TenantID       uint64 `gorm:"column:tenant_id;not null"`
	ActorID        string `gorm:"column:actor_id;not null;default:''"`
	ConnectionID   string `gorm:"column:connection_id;not null"`
	AppVersion     string `gorm:"column:app_version;not null"`
	Target         string `gorm:"column:target;not null"`
	Risk           string `gorm:"column:risk;not null"`
	AuthVersion    int64  `gorm:"column:auth_version;not null;default:1"`
	ArgsSnapshot   string `gorm:"column:args_snapshot;not null"`
	ArgsDigest     string `gorm:"column:args_digest;not null"`
	State          string `gorm:"column:state;not null"`
	ProviderKey    string `gorm:"column:provider_key;not null;default:''"`
	ProviderResult string `gorm:"column:provider_result;not null;default:''"`
	ReservationID  string `gorm:"column:reservation_id;not null;default:''"`
	Fence          int64  `gorm:"column:fence;not null;default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (ActionRow) TableName() string { return "app_actions" }

// ApprovalRow binds ONE approval to ONE digest. Remaining is the number of
// dispatches this approval still authorizes; consuming below zero is
// structurally impossible because the consume path guards and decrements in
// the same transaction that writes the dispatch intent.
type ApprovalRow struct {
	ArgsDigest string    `gorm:"primaryKey;column:args_digest"`
	ActionID   string    `gorm:"column:action_id;not null"`
	Actor      string    `gorm:"column:actor;not null"`
	Expiry     time.Time `gorm:"column:expiry;not null"`
	Remaining  int64     `gorm:"column:remaining;not null;default:1"`
}

func (ApprovalRow) TableName() string { return "app_action_approvals" }

// PreAuthorizationRow persists a scope pre-authorization (its own scope —
// never interchangeable with a general write grant).
type PreAuthorizationRow struct {
	ID               string    `gorm:"primaryKey;column:id"`
	TenantID         uint64    `gorm:"column:tenant_id;not null"`
	AllowedRisksJSON string    `gorm:"column:allowed_risks_json;not null"`
	ConnectionID     string    `gorm:"column:connection_id;not null;default:'*';"`
	TargetScope      string    `gorm:"column:target_scope;not null;default:'*';"`
	ValidFrom        time.Time `gorm:"column:valid_from;not null"`
	ValidUntil       time.Time `gorm:"column:valid_until;not null"`
	BudgetCapMicro   int64     `gorm:"column:budget_cap_micro;not null;default:0"`
}

func (PreAuthorizationRow) TableName() string { return "app_action_preauthorizations" }

// ActionStore persists actions, their digest-bound approvals, and scope
// pre-authorizations.
type ActionStore struct{ db *gorm.DB }

// NewActionStore builds an ActionStore over a gorm DB.
func NewActionStore(db *gorm.DB) *ActionStore { return &ActionStore{db: db} }

// CreateAction persists a new action in the given initial state with its
// normalized snapshot and digest.
func (s *ActionStore) CreateAction(ctx context.Context, a appconnector.Action, snapshot, digest, state string) error {
	if a.ID == "" || a.TenantID == 0 || snapshot == "" || digest == "" || state == "" {
		return ErrActionState
	}
	row := ActionRow{
		ID: a.ID, TenantID: a.TenantID, ActorID: a.ActorID, ConnectionID: a.ConnectionID,
		AppVersion: a.Version, Target: a.Target, Risk: a.Risk, AuthVersion: a.AuthVersion,
		ArgsSnapshot: snapshot, ArgsDigest: digest, State: state,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// FindAction loads an action row by id.
func (s *ActionStore) FindAction(ctx context.Context, id string) (ActionRow, error) {
	var row ActionRow
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ActionRow{}, ErrActionNotFound
		}
		return ActionRow{}, err
	}
	return row, nil
}

// SaveApproval upserts the approval bound to a digest with the remaining
// consume count and expiry.
func (s *ActionStore) SaveApproval(ctx context.Context, actionID, digest, actor string, expiry time.Time, remaining int64) error {
	if actionID == "" || digest == "" || actor == "" || remaining <= 0 {
		return ErrApprovalExhausted
	}
	row := ApprovalRow{ArgsDigest: digest, ActionID: actionID, Actor: actor, Expiry: expiry, Remaining: remaining}
	return s.db.WithContext(ctx).Save(&row).Error
}

// SetActionState performs a guarded state transition (compare-and-swap on
// the expected state).
func (s *ActionStore) SetActionState(ctx context.Context, id, from, to string) error {
	res := s.db.WithContext(ctx).Model(&ActionRow{}).
		Where("id = ? AND state = ?", id, from).Update("state", to)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrActionState
	}
	return nil
}

// ClaimDispatch consumes one approval count and writes the dispatch intent
// in ONE transaction: the approval is decremented under a remaining-count
// guard (a concurrent racer that loses sees zero rows affected and the whole
// transaction aborts), then the action walks authorized→queued→dispatched
// with the provider key, reservation reference and a bumped fence. A crash
// before this transaction commits leaves the action authorized (no
// phantom dispatch); after it the action is dispatched and a provider query
// is the only unknown-resolution path.
func (s *ActionStore) ClaimDispatch(ctx context.Context, id, providerKey, reservationID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row ActionRow
		if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrActionNotFound
			}
			return err
		}
		if row.State != appconnector.ActionAuthorized {
			return ErrActionState
		}
		var ap ApprovalRow
		if err := tx.Where("args_digest = ?", row.ArgsDigest).First(&ap).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrApprovalExhausted
			}
			return err
		}
		now := time.Now()
		if ap.Remaining <= 0 || !now.Before(ap.Expiry) {
			return ErrApprovalExhausted
		}
		res := tx.Model(&ApprovalRow{}).
			Where("args_digest = ? AND remaining = ?", ap.ArgsDigest, ap.Remaining).
			Update("remaining", gorm.Expr("remaining - 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrApprovalExhausted
		}
		if err := tx.Model(&ActionRow{}).
			Where("id = ? AND state = ?", id, appconnector.ActionAuthorized).
			Update("state", appconnector.ActionQueued).Error; err != nil {
			return err
		}
		res = tx.Model(&ActionRow{}).
			Where("id = ? AND state = ?", id, appconnector.ActionQueued).
			Updates(map[string]interface{}{
				"state":          appconnector.ActionDispatched,
				"provider_key":   providerKey,
				"reservation_id": reservationID,
				"fence":          row.Fence + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrActionState
		}
		return nil
	})
}

// FinishDispatch settles a dispatched action into succeeded/failed/unknown
// under a fence guard, recording the provider result.
func (s *ActionStore) FinishDispatch(ctx context.Context, id string, fence int64, state, result string) error {
	if state != appconnector.ActionSucceeded && state != appconnector.ActionFailed && state != appconnector.ActionUnknown {
		return ErrActionState
	}
	res := s.db.WithContext(ctx).Model(&ActionRow{}).
		Where("id = ? AND state = ? AND fence = ?", id, appconnector.ActionDispatched, fence).
		Updates(map[string]interface{}{"state": state, "provider_result": result})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrActionState
	}
	return nil
}

// FinishUnknown settles an action parked in unknown into succeeded/failed
// after the provider query answered. It guards on the dispatch fence so a
// stale worker cannot overwrite a newer outcome.
func (s *ActionStore) FinishUnknown(ctx context.Context, id string, fence int64, state, result string) error {
	if state != appconnector.ActionSucceeded && state != appconnector.ActionFailed {
		return ErrActionState
	}
	res := s.db.WithContext(ctx).Model(&ActionRow{}).
		Where("id = ? AND state = ? AND fence = ?", id, appconnector.ActionUnknown, fence).
		Updates(map[string]interface{}{"state": state, "provider_result": result})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrActionState
	}
	return nil
}

// SavePreAuthorization persists a scope pre-authorization.
func (s *ActionStore) SavePreAuthorization(ctx context.Context, p appconnector.PreAuthorization) error {
	if p.ID == "" || p.TenantID == 0 || len(p.AllowedRisks) == 0 {
		return ErrActionState
	}
	risks, err := json.Marshal(p.AllowedRisks)
	if err != nil {
		return err
	}
	row := PreAuthorizationRow{
		ID: p.ID, TenantID: p.TenantID, AllowedRisksJSON: string(risks),
		ConnectionID: p.ConnectionID, TargetScope: p.TargetScope,
		ValidFrom: p.ValidFrom, ValidUntil: p.ValidUntil, BudgetCapMicro: p.BudgetCapMicro,
	}
	if row.ConnectionID == "" {
		row.ConnectionID = "*"
	}
	if row.TargetScope == "" {
		row.TargetScope = "*"
	}
	return s.db.WithContext(ctx).Save(&row).Error
}

// ListPreAuthorizations returns every pre-authorization of a tenant; the
// caller matches them with domain-scope rules (own risk category scope).
func (s *ActionStore) ListPreAuthorizations(ctx context.Context, tenant uint64) ([]appconnector.PreAuthorization, error) {
	var rows []PreAuthorizationRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenant).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]appconnector.PreAuthorization, 0, len(rows))
	for _, r := range rows {
		var risks []string
		if err := json.Unmarshal([]byte(r.AllowedRisksJSON), &risks); err != nil {
			return nil, err
		}
		out = append(out, appconnector.PreAuthorization{
			ID: r.ID, TenantID: r.TenantID, AllowedRisks: risks,
			ConnectionID: r.ConnectionID, TargetScope: r.TargetScope,
			ValidFrom: r.ValidFrom, ValidUntil: r.ValidUntil, BudgetCapMicro: r.BudgetCapMicro,
		})
	}
	return out, nil
}
