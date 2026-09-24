package service

// Task compliance & retention (T13, #43).
//
// Compliance access is an INDEPENDENT flow (CONTEXT.md 合规访问): an
// administrator sees task METADATA by default; private content requires a
// reasoned, time-limited window that is fully audited BEFORE anything opens
// or reads. The service deliberately holds NO TaskGrantStorePort — a
// compliance window can never join the task collaboration list (AC1 is
// enforced at the type level).

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// MaxComplianceAccessTTL bounds one compliance window's lifetime (Spec story
// 54 "time-limited"; the concrete ceiling is a plan-level decision).
const MaxComplianceAccessTTL = 7 * 24 * time.Hour

// TaskComplianceStorePort is the persistence seam for the T13 lanes
// (implemented by repository.TaskComplianceStore).
type TaskComplianceStorePort interface {
	GetTaskPolicy(ctx context.Context, tenantID uint64) (*types.TenantTaskPolicy, error)
	UpsertTaskPolicy(ctx context.Context, policy types.TenantTaskPolicy) (types.TenantTaskPolicy, error)
	TaskMetadataFacts(ctx context.Context, tenantID uint64, taskID string) (*types.TaskMetadataFacts, error)
	OpenComplianceAccess(ctx context.Context, access types.TaskComplianceAccess) (types.TaskComplianceAccess, error)
	ActiveComplianceAccess(ctx context.Context, tenantID uint64, taskID, adminID string, now time.Time) (*types.TaskComplianceAccess, error)
	ListTaskMessages(ctx context.Context, taskID string, limit int) ([]types.TaskMessageFact, error)
	// PurgeTask is added back in Task 6 (retention purge); the port grows
	// with implemented capability (ruling via escalation, t13 #43 task 4:
	// a port method declared ahead of its only implementation is masked by
	// test stubs and explodes at the first real wiring point).
}

// TaskComplianceService carries the compliance surface and the deletion
// gates. audit may be nil only in mis-assembled deployments — every audited
// operation then fails closed with 503 instead of proceeding unrecorded.
type TaskComplianceService struct {
	store TaskComplianceStorePort
	audit interfaces.AuditLogService
	now   func() time.Time
}

// NewTaskComplianceService constructs the production service.
func NewTaskComplianceService(store TaskComplianceStorePort, audit interfaces.AuditLogService) *TaskComplianceService {
	return NewTaskComplianceServiceWithClock(store, audit, time.Now)
}

// NewTaskComplianceServiceWithClock is the test/e2e constructor with an
// injectable clock (retention-horizon time travel).
func NewTaskComplianceServiceWithClock(store TaskComplianceStorePort, audit interfaces.AuditLogService, now func() time.Time) *TaskComplianceService {
	return &TaskComplianceService{store: store, audit: audit, now: now}
}

// requireAdmin enforces the compliance lane's role floor: TenantRole admin
// or above (admin and owner both pass; contributors and viewers do not).
func (s *TaskComplianceService) requireAdmin(caller types.Caller) error {
	if caller.TenantID == 0 || strings.TrimSpace(caller.UserID) == "" {
		return apperrors.NewForbiddenError("authenticated tenant identity is required")
	}
	if caller.Role.Level() < types.TenantRoleAdmin.Level() {
		return apperrors.NewForbiddenError("compliance access requires the tenant admin role")
	}
	return nil
}

// audited writes one audit entry or fails closed.
func (s *TaskComplianceService) audited(ctx context.Context, entry *types.AuditLog) error {
	if s.audit == nil {
		return apperrors.NewServiceUnavailableError("compliance audit trail is required")
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = s.now().UTC()
	}
	return s.audit.Log(ctx, entry)
}

// GetTaskPolicy reads the tenant's policy (admin-only).
func (s *TaskComplianceService) GetTaskPolicy(ctx context.Context, caller types.Caller) (*types.TenantTaskPolicy, error) {
	if err := s.requireAdmin(caller); err != nil {
		return nil, err
	}
	return s.store.GetTaskPolicy(ctx, caller.TenantID)
}

// SetTaskPolicy rewrites the tenant's retention policy (admin-only,
// negative retention refused, audited before the write).
func (s *TaskComplianceService) SetTaskPolicy(ctx context.Context, caller types.Caller, retentionDays int, legalHold bool) (*types.TenantTaskPolicy, error) {
	if err := s.requireAdmin(caller); err != nil {
		return nil, err
	}
	if retentionDays < 0 {
		return nil, apperrors.NewBadRequestError("retention_days must be >= 0")
	}
	policyDetails, err := json.Marshal(map[string]any{"retention_days": retentionDays, "legal_hold": legalHold})
	if err != nil {
		return nil, err
	}
	if err := s.audited(ctx, &types.AuditLog{
		TenantID:    caller.TenantID,
		ActorUserID: caller.UserID,
		ActorRole:   string(caller.Role),
		Action:      types.AuditActionTaskPolicyUpdated,
		TargetType:  "tenant",
		TargetID:    "task-policy",
		Outcome:     types.AuditOutcomeSuccess,
		Details:     types.JSON(policyDetails),
	}); err != nil {
		return nil, err
	}
	policy, err := s.store.UpsertTaskPolicy(ctx, types.TenantTaskPolicy{
		TenantID: caller.TenantID, RetentionDays: retentionDays, LegalHold: legalHold, UpdatedBy: caller.UserID,
	})
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// TaskMetadata is the administrator's default view: metadata + policy, no
// content (the view type carries no content field at all).
func (s *TaskComplianceService) TaskMetadata(ctx context.Context, caller types.Caller, taskID string) (types.TaskMetadataView, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskMetadataView{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskMetadataView{}, apperrors.NewBadRequestError("task id is required")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskMetadataView{}, normalizeComplianceMiss(err)
	}
	policy, err := s.store.GetTaskPolicy(ctx, caller.TenantID)
	if err != nil {
		return types.TaskMetadataView{}, err
	}
	return types.TaskMetadataView{Metadata: *facts, Policy: policy}, nil
}

// RequestContentAccess opens one time-limited window after the audit row is
// durably written (reason + horizon in the entry). An audit failure refuses
// the window — fail closed.
func (s *TaskComplianceService) RequestContentAccess(ctx context.Context, caller types.Caller, taskID, reason string, ttl time.Duration) (types.TaskComplianceAccess, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskComplianceAccess{}, err
	}
	taskID = strings.TrimSpace(taskID)
	reason = strings.TrimSpace(reason)
	if taskID == "" {
		return types.TaskComplianceAccess{}, apperrors.NewBadRequestError("task id is required")
	}
	if reason == "" {
		return types.TaskComplianceAccess{}, apperrors.NewBadRequestError("an access reason is required")
	}
	if ttl <= 0 || ttl > MaxComplianceAccessTTL {
		return types.TaskComplianceAccess{}, apperrors.NewBadRequestError("ttl must be between 1 hour and 168 hours")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskComplianceAccess{}, normalizeComplianceMiss(err)
	}
	now := s.now().UTC()
	access := types.TaskComplianceAccess{
		TenantID: caller.TenantID, TaskID: facts.TaskID, AdminID: caller.UserID,
		Reason: reason, ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	details, _ := json.Marshal(map[string]any{"reason": reason, "expires_at": access.ExpiresAt})
	if err := s.audited(ctx, &types.AuditLog{
		TenantID: caller.TenantID, ActorUserID: caller.UserID, ActorRole: string(caller.Role),
		Action:     types.AuditActionComplianceAccessRequested,
		TargetType: "task", TargetID: facts.TaskID, TargetUserID: facts.OwnerID,
		Outcome: types.AuditOutcomeSuccess, Details: types.JSON(details),
	}); err != nil {
		return types.TaskComplianceAccess{}, err
	}
	return s.store.OpenComplianceAccess(ctx, access)
}

// ReadTaskContent returns the private content projection only under the
// caller's own unexpired window, and only after its read is audited.
func (s *TaskComplianceService) ReadTaskContent(ctx context.Context, caller types.Caller, taskID string) (types.TaskContentView, error) {
	if err := s.requireAdmin(caller); err != nil {
		return types.TaskContentView{}, err
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return types.TaskContentView{}, apperrors.NewBadRequestError("task id is required")
	}
	facts, err := s.store.TaskMetadataFacts(ctx, caller.TenantID, taskID)
	if err != nil {
		return types.TaskContentView{}, normalizeComplianceMiss(err)
	}
	window, err := s.store.ActiveComplianceAccess(ctx, caller.TenantID, facts.TaskID, caller.UserID, s.now().UTC())
	if err != nil {
		return types.TaskContentView{}, err
	}
	if window == nil {
		return types.TaskContentView{}, apperrors.NewForbiddenError("an active compliance access window is required")
	}
	readDetails, err := json.Marshal(map[string]any{"window_id": window.ID})
	if err != nil {
		return types.TaskContentView{}, err
	}
	if err := s.audited(ctx, &types.AuditLog{
		TenantID: caller.TenantID, ActorUserID: caller.UserID, ActorRole: string(caller.Role),
		Action:     types.AuditActionComplianceContentRead,
		TargetType: "task", TargetID: facts.TaskID, TargetUserID: facts.OwnerID,
		Outcome: types.AuditOutcomeSuccess,
		Details: types.JSON(readDetails),
	}); err != nil {
		return types.TaskContentView{}, err
	}
	messages, err := s.store.ListTaskMessages(ctx, facts.TaskID, 200)
	if err != nil {
		return types.TaskContentView{}, err
	}
	return types.TaskContentView{TaskID: facts.TaskID, Window: *window, Messages: messages}, nil
}

// normalizeComplianceMiss maps the store's uniform miss to a uniform 404 and
// passes everything else through untouched (a 5xx must never masquerade as
// a miss — B3-F82 convention).
func normalizeComplianceMiss(err error) error {
	if errors.Is(err, types.ErrTaskComplianceNotFound) {
		return apperrors.NewNotFoundError("task not found")
	}
	return err
}
