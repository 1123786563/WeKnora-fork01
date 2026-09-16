package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	workbench "github.com/Tencent/WeKnora/internal/workbench"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInteractionNotFound    = errors.New("interaction_not_found")
	ErrInteractionExpired     = errors.New("interaction_expired")
	ErrInteractionRevoked     = errors.New("interaction_revoked")
	ErrCapabilityUnavailable  = errors.New("capability_unavailable")
	ErrCommandRecoveryUnknown = errors.New("command_recovery_unknown")
)

// InteractionStore is the durable boundary. Implementations must use a
// tenant+owner predicate and an atomic decision_id/revision compare-and-swap.
// Keeping this port narrow lets the HTTP surface share the existing approval
// service without duplicating persistence or authorization logic.
type InteractionStore interface {
	List(ctx context.Context, tenantID uint64, ownerID, runID string) ([]workbench.InteractionDecision, error)
	Get(ctx context.Context, tenantID uint64, ownerID, id string) (workbench.InteractionDecision, error)
	Decide(ctx context.Context, tenantID uint64, ownerID, id string, decision workbench.InteractionDecision) (workbench.InteractionDecision, error)
}

type interactionRow struct {
	TenantID                           uint64
	ID, RunID, OwnerID, Kind, ArgsHash string
	DecisionID, Action, Status         string
	ExpectedRevision                   int64
	ExpiresAt                          *time.Time
	Revoked                            bool
	CreatedAt                          time.Time
	UpdatedAt                          time.Time
}

func (interactionRow) TableName() string { return "workbench_interactions" }

func (r interactionRow) decision() workbench.InteractionDecision {
	return workbench.InteractionDecision{ID: r.ID, DecisionID: r.DecisionID, Kind: r.Kind, Action: r.Action, ArgsHash: r.ArgsHash, ExpectedRevision: r.ExpectedRevision}
}

// GormInteractionStore is the production persistence adapter. All reads are
// scoped by tenant and owner, and Decide locks the row before checking the
// revision and decision id, making retries idempotent across replicas.
type GormInteractionStore struct{ db *gorm.DB }

type DecisionCompensator interface {
	RollbackDecision(ctx context.Context, tenantID uint64, ownerID, id, decisionID string, revision int64) error
}

func NewGormInteractionStore(db *gorm.DB) *GormInteractionStore { return &GormInteractionStore{db: db} }
func (s *GormInteractionStore) DB() *gorm.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *GormInteractionStore) CreatePending(ctx context.Context, req approval.PendingRequest, pendingID string) error {
	if s == nil || s.db == nil || req.TenantID == 0 || strings.TrimSpace(req.RunID) == "" || strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(pendingID) == "" {
		return ErrCapabilityUnavailable
	}
	hash := sha256.Sum256(req.Args)
	expires := time.Now().Add(10 * time.Minute)
	row := interactionRow{TenantID: req.TenantID, ID: pendingID, RunID: req.RunID, OwnerID: req.UserID, Kind: string(workbench.InteractionToolApproval), ArgsHash: hex.EncodeToString(hash[:]), Status: "pending", ExpiresAt: &expires}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (s *GormInteractionStore) DeletePending(ctx context.Context, req approval.PendingRequest, pendingID string) error {
	return s.db.WithContext(ctx).Model(&interactionRow{}).Where("tenant_id = ? AND owner_id = ? AND id = ? AND status = 'pending'", req.TenantID, req.UserID, pendingID).Updates(map[string]any{"status": "revoked", "revoked": true, "updated_at": gorm.Expr("CURRENT_TIMESTAMP")}).Error
}

func (s *GormInteractionStore) List(ctx context.Context, tenantID uint64, ownerID, runID string) ([]workbench.InteractionDecision, error) {
	var rows []interactionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND run_id = ?", tenantID, ownerID, runID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]workbench.InteractionDecision, 0, len(rows))
	for _, row := range rows {
		if err := validateInteractionRow(row); err != nil {
			return nil, err
		}
		out = append(out, row.decision())
	}
	return out, nil
}

func (s *GormInteractionStore) Get(ctx context.Context, tenantID uint64, ownerID, id string) (workbench.InteractionDecision, error) {
	var row interactionRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workbench.InteractionDecision{}, ErrInteractionNotFound
	}
	if err != nil {
		return workbench.InteractionDecision{}, err
	}
	if err := validateInteractionRow(row); err != nil {
		return workbench.InteractionDecision{}, err
	}
	return row.decision(), nil
}

func (s *GormInteractionStore) Decide(ctx context.Context, tenantID uint64, ownerID, id string, input workbench.InteractionDecision) (workbench.InteractionDecision, error) {
	if strings.TrimSpace(input.DecisionID) == "" || strings.TrimSpace(input.ArgsHash) == "" {
		return workbench.InteractionDecision{}, workbench.ErrInteractionActionMismatch
	}
	var result workbench.InteractionDecision
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row interactionRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_id = ? AND id = ?", tenantID, ownerID, id).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInteractionNotFound
			}
			return err
		}
		if err := validateInteractionRow(row); err != nil {
			return err
		}
		if input.ArgsHash != row.ArgsHash {
			return workbench.ErrInteractionActionMismatch
		}
		if input.DecisionID != "" && row.DecisionID == input.DecisionID {
			if row.Action != input.Action {
				return agentruntime.ErrConflict
			}
			result = row.decision()
			return nil
		}
		if input.ExpectedRevision != row.ExpectedRevision || row.DecisionID != "" {
			return agentruntime.ErrConflict
		}
		updated := tx.Model(&interactionRow{}).Where("tenant_id = ? AND owner_id = ? AND id = ? AND expected_revision = ? AND decision_id = ''", tenantID, ownerID, id, row.ExpectedRevision).Updates(map[string]any{
			"decision_id": input.DecisionID, "action": input.Action, "status": "resolved", "expected_revision": gorm.Expr("expected_revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP"),
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return agentruntime.ErrConflict
		}
		result = input
		result.ID = row.ID
		result.Kind = row.Kind
		result.ArgsHash = row.ArgsHash
		result.ExpectedRevision = row.ExpectedRevision + 1
		return nil
	})
	return result, err
}

func (s *GormInteractionStore) RollbackDecision(ctx context.Context, tenantID uint64, ownerID, id, decisionID string, revision int64) error {
	updated := s.db.WithContext(ctx).Model(&interactionRow{}).Where("tenant_id = ? AND owner_id = ? AND id = ? AND decision_id = ? AND expected_revision = ?", tenantID, ownerID, id, decisionID, revision).Updates(map[string]any{"decision_id": "", "action": "", "status": "pending", "expected_revision": gorm.Expr("expected_revision - 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return agentruntime.ErrConflict
	}
	return nil
}

func validateInteractionRow(row interactionRow) error {
	if row.Revoked {
		return ErrInteractionRevoked
	}
	if row.ExpiresAt != nil && time.Now().After(*row.ExpiresAt) {
		return ErrInteractionExpired
	}
	if row.Status == "resolved" {
		return nil
	}
	if err := workbench.ValidateInteractionAction(row.Kind, row.Action); err != nil && row.Action != "" {
		return err
	}
	return nil
}

type SteerPort interface {
	Steer(ctx context.Context, tenantID uint64, ownerID, runID, text string, expectedRevision int64) error
}

type CancelPort interface {
	Cancel(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64) error
}

// GormCancelPort is the durable cancel command. It only transitions the
// authenticated run and fences on its revision; unknown or already-terminal
// runs are conflicts and never mutate a different run.
type GormCancelPort struct{ db *gorm.DB }

func NewGormCancelPort(db *gorm.DB) *GormCancelPort { return &GormCancelPort{db: db} }
func (p *GormCancelPort) Cancel(ctx context.Context, tenantID uint64, ownerID, runID string, expectedRevision int64) error {
	if p == nil || p.db == nil {
		return ErrCapabilityUnavailable
	}
	updated := p.db.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND owner_id = ? AND run_id = ? AND revision = ? AND status IN ('queued','running','waiting_user','reconciling','recovering')", tenantID, ownerID, runID, expectedRevision).Updates(map[string]any{"status": "canceled", "revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return agentruntime.ErrConflict
	}
	return nil
}

// GormSteerPort resolves the run owner and assistant message from the durable
// run row before appending to the existing steer queue. It never marks a
// queued message executed; the engine consumes it at its round boundary.
type GormSteerPort struct {
	db      *gorm.DB
	streams interfaces.StreamManager
}

func NewGormSteerPort(db *gorm.DB, streams interfaces.StreamManager) *GormSteerPort {
	return &GormSteerPort{db: db, streams: streams}
}
func (p *GormSteerPort) Steer(ctx context.Context, tenantID uint64, ownerID, runID, text string, expectedRevision int64) error {
	if p == nil || p.db == nil || p.streams == nil {
		return ErrCapabilityUnavailable
	}
	var row struct {
		SessionID, AssistantMessageID string
		Revision                      int64
	}
	if err := p.db.WithContext(ctx).Table("agent_runs").Select("session_id, assistant_message_id, revision").Where("tenant_id = ? AND owner_id = ? AND run_id = ?", tenantID, ownerID, runID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return agentruntime.ErrNotFound
		}
		return err
	}
	if row.Revision != expectedRevision {
		return agentruntime.ErrConflict
	}
	claimed := p.db.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND owner_id = ? AND run_id = ? AND revision = ? AND status IN ('queued','running','waiting_user','reconciling','recovering')", tenantID, ownerID, runID, expectedRevision).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
	if claimed.Error != nil {
		return claimed.Error
	}
	if claimed.RowsAffected != 1 {
		return agentruntime.ErrConflict
	}
	err := p.streams.AppendSteerEvents(ctx, row.SessionID, row.AssistantMessageID, []interfaces.StreamEvent{{ID: uuid.NewString(), Type: types.ResponseTypeSteer, Content: text, Data: map[string]interface{}{"delivery": "inject"}, Timestamp: time.Now()}})
	if err != nil {
		rollback := p.db.WithContext(ctx).Table("agent_runs").Where("tenant_id = ? AND owner_id = ? AND run_id = ? AND revision = ?", tenantID, ownerID, runID, expectedRevision+1).Updates(map[string]any{"revision": gorm.Expr("revision - 1"), "updated_at": gorm.Expr("CURRENT_TIMESTAMP")})
		if rollback.Error != nil || rollback.RowsAffected != 1 {
			return fmt.Errorf("%w: steer revision rollback: %v", ErrCommandRecoveryUnknown, rollback.Error)
		}
		return err
	}
	return nil
}

type Service struct {
	store    InteractionStore
	steer    SteerPort
	cancel   CancelPort
	approval *approval.Gate
}

func NewInteractionService(store InteractionStore, steer SteerPort, cancel CancelPort) *Service {
	return &Service{store: store, steer: steer, cancel: cancel}
}

func NewInteractionServiceWithApproval(store InteractionStore, steer SteerPort, cancel CancelPort, gate *approval.Gate) *Service {
	if gate != nil {
		if durable, ok := store.(interface {
			CreatePending(context.Context, approval.PendingRequest, string) error
		}); ok {
			gate.SetPendingObserver(durable.CreatePending)
		}
		if rollback, ok := store.(interface {
			DeletePending(context.Context, approval.PendingRequest, string) error
		}); ok {
			gate.SetPendingRollback(rollback.DeletePending)
		}
	}
	return &Service{store: store, steer: steer, cancel: cancel, approval: gate}
}

func identity(ctx context.Context) (uint64, string, error) {
	tenant, ok := types.TenantIDFromContext(ctx)
	if !ok || tenant == 0 {
		return 0, "", errors.New("tenant context is required")
	}
	owner, ok := types.UserIDFromContext(ctx)
	if !ok || strings.TrimSpace(owner) == "" {
		return 0, "", errors.New("actor context is required")
	}
	return tenant, owner, nil
}

func (s *Service) List(ctx context.Context, runID string) ([]workbench.InteractionDecision, error) {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil || strings.TrimSpace(runID) == "" {
		return nil, ErrInteractionNotFound
	}
	return s.store.List(ctx, tenant, owner, strings.TrimSpace(runID))
}

func (s *Service) Decide(ctx context.Context, id string, input workbench.InteractionDecision) (workbench.InteractionDecision, error) {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return workbench.InteractionDecision{}, err
	}
	if s == nil || s.store == nil {
		return workbench.InteractionDecision{}, ErrInteractionNotFound
	}
	current, err := s.store.Get(ctx, tenant, owner, strings.TrimSpace(id))
	if err != nil {
		return workbench.InteractionDecision{}, err
	}
	if input.Kind != "" && input.Kind != current.Kind {
		return workbench.InteractionDecision{}, workbench.ErrInteractionActionMismatch
	}
	if err := workbench.ValidateInteractionAction(current.Kind, input.Action); err != nil {
		return workbench.InteractionDecision{}, err
	}
	if current.Kind == string(workbench.InteractionToolApproval) && s.approval == nil {
		return workbench.InteractionDecision{}, ErrCapabilityUnavailable
	}
	if strings.TrimSpace(input.DecisionID) == "" || strings.TrimSpace(input.ArgsHash) == "" {
		return workbench.InteractionDecision{}, workbench.ErrInteractionActionMismatch
	}
	if input.ArgsHash != current.ArgsHash {
		return workbench.InteractionDecision{}, workbench.ErrInteractionActionMismatch
	}
	input.ID = current.ID
	input.Kind = current.Kind
	input.ArgsHash = current.ArgsHash
	result, err := s.store.Decide(ctx, tenant, owner, current.ID, input)
	if err != nil {
		return workbench.InteractionDecision{}, err
	}
	newDecision := result.ExpectedRevision == input.ExpectedRevision+1
	if current.Kind == string(workbench.InteractionToolApproval) && s.approval != nil && newDecision {
		if err := s.approval.Resolve(tenant, owner, current.ID, approval.Decision{Approved: input.Action == "approve"}); err != nil {
			if compensator, ok := s.store.(DecisionCompensator); ok {
				_ = compensator.RollbackDecision(ctx, tenant, owner, current.ID, input.DecisionID, result.ExpectedRevision)
			}
			return workbench.InteractionDecision{}, err
		}
	}
	return result, nil
}

func (s *Service) Command(ctx context.Context, runID string, command workbench.ExecutionCommand) error {
	tenant, owner, err := identity(ctx)
	if err != nil {
		return err
	}
	if err := command.Validate(); err != nil {
		return err
	}
	if s == nil || strings.TrimSpace(runID) == "" {
		return ErrInteractionNotFound
	}
	switch command.Action {
	case "cancel":
		if s.cancel == nil {
			return ErrCapabilityUnavailable
		}
		return s.cancel.Cancel(ctx, tenant, owner, runID, command.ExpectedRevision)
	case "steer":
		if s.steer == nil {
			return ErrCapabilityUnavailable
		}
		return s.steer.Steer(ctx, tenant, owner, runID, command.Text, command.ExpectedRevision)
	default:
		return workbench.ErrCommandActionMismatch
	}
}
