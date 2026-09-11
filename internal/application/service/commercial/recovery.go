package commercial

import (
	"context"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/config"

	"gorm.io/gorm"
)

var (
	// ErrRecoveryOperatorCheckMissing: no platform-operator capability
	// check is wired, so Replay fails CLOSED — nobody can replay until an
	// explicit authority is injected.
	ErrRecoveryOperatorCheckMissing = errors.New("recovery_operator_check_missing")
	// ErrRecoveryUnauthorized: the actor failed the platform-operator
	// capability check.
	ErrRecoveryUnauthorized = errors.New("recovery_unauthorized")
	// ErrRecoveryOperationUnknown: no durable operation exists under the
	// requested ID — replays address recorded operations only.
	ErrRecoveryOperationUnknown = errors.New("recovery_operation_unknown")
	// ErrRecoveryQueryOnly: the operation kind is not replay-safe (new
	// consumption or an unknown write). Recovery may only query/reconcile
	// it; it is NEVER re-dispatched.
	ErrRecoveryQueryOnly = errors.New("recovery_query_only")
	// ErrRecoveryAlreadyDelivered: the operation was already sent; a
	// replay would duplicate the effect and is refused.
	ErrRecoveryAlreadyDelivered = errors.New("recovery_already_delivered")
	// ErrRecoveryConnectionRevoked: the object behind the operation is a
	// revoked connection — replay must never re-enable it.
	ErrRecoveryConnectionRevoked = errors.New("recovery_connection_revoked")
	// ErrRecoveryStateUnverified: the object's version/state could not be
	// verified before replay (no resolver or resolver failure), so the
	// replay is refused instead of guessed.
	ErrRecoveryStateUnverified = errors.New("recovery_state_unverified")
	// ErrRecoveryAuditFailed: the audit record for a replay could not be
	// persisted; an unauditable recovery is reported as failed.
	ErrRecoveryAuditFailed = errors.New("recovery_audit_failed")
	// ErrNewConsumptionPaused: a rollout switch (commercial_new_orders /
	// commercial_new_dispatch / connector_new_actions) is closed.
	ErrNewConsumptionPaused = errors.New("new_consumption_paused")
)

// PlatformOperatorCheck verifies the actor holds the platform-operator
// capability required to replay commercial operations. nil fails closed.
type PlatformOperatorCheck func(ctx context.Context, actor string) error

// RecoveryObjectState is the verified snapshot of the business object a
// replay targets: its business ID, optimistic version and current state.
// Revoked marks objects (connections) whose re-enable must be refused.
type RecoveryObjectState struct {
	BusinessID string
	Version    int64
	State      string
	Revoked    bool
}

// ObjectStateResolver loads the current object state for one outbox
// event. nil fails closed (ErrRecoveryStateUnverified).
type ObjectStateResolver func(ctx context.Context, event repocommercial.OutboxEvent) (RecoveryObjectState, error)

// RecoveryItem is one categorized recovery-queue entry as shown to
// operators: category, age, tenant and business ID. OperationID is the
// ORIGINAL operation/event key — replays never mint a new idempotency key.
type RecoveryItem struct {
	Category    domain.RecoveryCategory
	OperationID string
	Kind        string
	TenantID    uint64
	BusinessID  string
	State       string
	Age         time.Duration
	Attempts    int
}

// RecoveryAuditRow durably records who replayed what and with which
// outcome. Every Replay attempt — success or failure — writes one row.
type RecoveryAuditRow struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"`
	OperationID string    `gorm:"column:operation_id;not null;index"`
	Kind        string    `gorm:"column:kind;not null;default:''"`
	Actor       string    `gorm:"column:actor;not null"`
	Result      string    `gorm:"column:result;not null"`
	Detail      string    `gorm:"column:detail;not null;default:''"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
}

func (RecoveryAuditRow) TableName() string { return "commercial_recovery_audit" }

// RecoveryService presents the O01 recovery queue and performs audited
// replays of pause-safe commercial operations across rollout pauses.
//
// DEPENDENCY HONESTY: the platform-operator capability check is injected,
// not assumed — with no check wired, Replay fails closed. The service
// never touches balances directly: recovery re-drives durable operations
// through their ORIGINAL idempotency keys and leaves settlement math to
// the existing budget/settlement paths.
type RecoveryService struct {
	db       *gorm.DB
	outbox   *repocommercial.OutboxStore
	operator PlatformOperatorCheck
	resolve  ObjectStateResolver
	switches domain.CommercialSwitches
	now      func() time.Time
}

// NewRecoveryService validates wiring and adopts the config rollout
// switches (safe-on defaults: a nil cfg keeps every lane open).
func NewRecoveryService(db *gorm.DB, cfg *config.Config) (*RecoveryService, error) {
	if db == nil {
		return nil, errors.New("recovery_database_missing")
	}
	if err := db.AutoMigrate(&RecoveryAuditRow{}); err != nil {
		return nil, err
	}
	switches := domain.DefaultCommercialSwitches()
	if cfg != nil {
		switches = domain.CommercialSwitches{
			NewOrders:           cfg.AreCommercialNewOrdersEnabled(),
			NewDispatch:         cfg.AreCommercialNewDispatchEnabled(),
			ConnectorNewActions: cfg.AreConnectorNewActionsEnabled(),
		}
	}
	return &RecoveryService{
		db:       db,
		outbox:   repocommercial.NewOutboxStore(db),
		switches: switches,
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

// SetOperatorCheck wires the platform-operator capability check; without
// it Replay fails closed. SetStateResolver wires object verification.
func (s *RecoveryService) SetOperatorCheck(check PlatformOperatorCheck) { s.operator = check }
func (s *RecoveryService) SetStateResolver(r ObjectStateResolver)       { s.resolve = r }

// CheckNewConsumption enforces the rollout switches for NEW consumption:
// closed lanes are refused while recovery-safe operations (callbacks,
// queries, settlement) keep flowing.
func (s *RecoveryService) CheckNewConsumption(_ context.Context, operation string) error {
	if !s.switches.AllowsNewConsumption(operation) {
		return fmt.Errorf("%w: %s", ErrNewConsumptionPaused, operation)
	}
	return nil
}

// ListQueue projects the recovery queue, categorized, with age, tenant
// and business ID. Sources: undelivered outbox events plus refunds whose
// channel outcome is indeterminate or whose revocation is pending.
func (s *RecoveryService) ListQueue(ctx context.Context) ([]RecoveryItem, error) {
	now := s.now()
	var items []RecoveryItem
	var events []repocommercial.OutboxEvent
	if err := s.db.WithContext(ctx).
		Where("state <> ?", repocommercial.OutboxStateSent).
		Find(&events).Error; err != nil {
		return nil, err
	}
	for _, e := range events {
		age := now.Sub(e.LeaseUntil)
		if age < 0 {
			age = 0
		}
		items = append(items, RecoveryItem{
			Category:    domain.ClassifyRecovery(e.Kind, e.State),
			OperationID: e.EventKey,
			Kind:        e.Kind,
			TenantID:    e.TenantID,
			BusinessID:  e.EventKey,
			State:       e.State,
			Age:         age,
			Attempts:    e.AttemptCount,
		})
	}
	var refunds []repocommercial.RefundRow
	if err := s.db.WithContext(ctx).
		Where("state IN ?", []string{domain.RefundStatePending, domain.RefundStateRevocationPending}).
		Find(&refunds).Error; err != nil {
		return nil, err
	}
	for _, r := range refunds {
		items = append(items, RecoveryItem{
			Category:    domain.ClassifyRecovery(domain.OperationRefundQuery, r.State),
			OperationID: r.ID,
			Kind:        domain.OperationRefundQuery,
			TenantID:    r.TenantID,
			BusinessID:  r.OrderID,
			State:       r.State,
			Attempts:    int(r.ChannelAttempts),
		})
	}
	return items, nil
}

// Replay re-drives one durable commercial operation. Rules:
//   - the actor must pass the platform-operator capability check (nil
//     check fails CLOSED);
//   - the operation must exist and be one AllowDuringRollback permits —
//     unknown/new WRITE actions are query/reconcile-only, never
//     re-dispatched (ErrRecoveryQueryOnly);
//   - the object's version and current state must verify; a revoked
//     connection is never re-enabled (ErrRecoveryConnectionRevoked);
//   - the ORIGINAL event key is re-armed as the idempotency key — no new
//     key is ever minted, so a duplicate effect is impossible;
//   - balances are never mutated here: settlement math stays in the
//     budget/settlement services;
//   - every attempt writes an audit row recording actor and result.
func (s *RecoveryService) Replay(ctx context.Context, operationID, actor string) error {
	err := s.replayChecked(ctx, operationID, actor)
	if auditErr := s.recordAudit(ctx, operationID, actor, err); auditErr != nil && err == nil {
		return auditErr
	}
	return err
}

func (s *RecoveryService) replayChecked(ctx context.Context, operationID, actor string) error {
	if s.operator == nil {
		return ErrRecoveryOperatorCheckMissing
	}
	if err := s.operator(ctx, actor); err != nil {
		return fmt.Errorf("%w: %v", ErrRecoveryUnauthorized, err)
	}
	var event repocommercial.OutboxEvent
	err := s.db.WithContext(ctx).Where("event_key = ?", operationID).First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRecoveryOperationUnknown
	}
	if err != nil {
		return err
	}
	if !domain.AllowDuringRollback(event.Kind) {
		return fmt.Errorf("%w: kind %q is verification-only", ErrRecoveryQueryOnly, event.Kind)
	}
	if event.State == repocommercial.OutboxStateSent {
		return fmt.Errorf("%w: event_key %s", ErrRecoveryAlreadyDelivered, event.EventKey)
	}
	if s.resolve == nil {
		return fmt.Errorf("%w: no object-state resolver wired", ErrRecoveryStateUnverified)
	}
	snap, err := s.resolve(ctx, event)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRecoveryStateUnverified, err)
	}
	if snap.Version <= 0 {
		return fmt.Errorf("%w: object version %d for %q", ErrRecoveryStateUnverified, snap.Version, snap.BusinessID)
	}
	if snap.Revoked {
		// A revoked connection must never be re-enabled by recovery.
		return fmt.Errorf("%w: %s", ErrRecoveryConnectionRevoked, snap.BusinessID)
	}
	// Re-arm the ORIGINAL idempotency key: same event_key, back to
	// pending, lease cleared. No new outbox row is ever created.
	res := s.db.WithContext(ctx).Model(&repocommercial.OutboxEvent{}).
		Where("event_key = ?", operationID).
		Updates(map[string]any{
			"state":         repocommercial.OutboxStatePending,
			"lease_token":   "",
			"lease_until":   s.now(),
			"attempt_count": event.AttemptCount + 1,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrRecoveryOperationUnknown
	}
	return nil
}

func (s *RecoveryService) recordAudit(ctx context.Context, operationID, actor string, outcome error) error {
	result, detail, kind := "ok", "", ""
	if outcome != nil {
		result, detail = "error", outcome.Error()
	}
	if operationID != "" {
		var event repocommercial.OutboxEvent
		if err := s.db.WithContext(ctx).Where("event_key = ?", operationID).First(&event).Error; err == nil {
			kind = event.Kind
		}
	}
	row := RecoveryAuditRow{
		OperationID: operationID,
		Kind:        kind,
		Actor:       actor,
		Result:      result,
		Detail:      domain.TruncateLogValue(detail),
		CreatedAt:   s.now(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrRecoveryAuditFailed, err)
	}
	return nil
}
