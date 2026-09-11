package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"gorm.io/gorm"
)

var ErrSandboxUnavailable = errors.New("sandbox_unavailable")
var ErrSandboxResourceConflict = errors.New("sandbox resource reference conflict")

type AgentRunResource struct {
	TenantID         uint64
	RunID, SessionID string
	Ref              sandbox.ExecutionRef
	State            string
	ExpiresAt        time.Time
}

func validateResource(v AgentRunResource) error {
	if v.TenantID == 0 || v.RunID == "" || v.SessionID == "" || v.SessionID != v.Ref.SessionID || v.Ref.TenantID != v.TenantID {
		return ErrSandboxResourceConflict
	}
	return v.Ref.Validate()
}

func (r AgentRunResource) Terminal() bool {
	return r.State == "succeeded" || r.State == "failed" || r.State == "cancelled" || r.State == "released"
}

type AgentRunResourceRepository interface {
	Put(context.Context, AgentRunResource) error
	Get(context.Context, uint64, string) (AgentRunResource, error)
	Delete(context.Context, uint64, string) error
	Protects(context.Context, uint64, string) (bool, error)
}

// GormAgentRunResourceRepository stores the reference in the existing
// agent_tool_calls.external_task_ref column, keeping resource identity in the
// same durable transaction as the tool journal.
type GormAgentRunResourceRepository struct{ db *gorm.DB }

func NewGormAgentRunResourceRepository(db *gorm.DB) *GormAgentRunResourceRepository {
	return &GormAgentRunResourceRepository{db: db}
}
func (r *GormAgentRunResourceRepository) Put(ctx context.Context, v AgentRunResource) error {
	if err := validateResource(v); err != nil {
		return err
	}
	ref, _ := json.Marshal(v.Ref)
	result, err := r.db.WithContext(ctx).Table("agent_tool_calls").Where("tenant_id=? AND run_id=? AND call_id=?", v.TenantID, v.RunID, v.Ref.TaskID).Updates(map[string]any{"external_task_ref": string(ref), "status": v.State, "updated_at": time.Now()}).RowsAffected, r.db.Error
	if err != nil {
		return err
	}
	if result != 1 {
		return sandbox.ErrNotFound
	}
	return nil
}
func (r *GormAgentRunResourceRepository) Get(ctx context.Context, tenant uint64, run string) (AgentRunResource, error) {
	var row struct {
		TenantID                               uint64
		RunID, CallID, Status, ExternalTaskRef string
	}
	err := r.db.WithContext(ctx).Table("agent_tool_calls").Where("tenant_id=? AND run_id=?", tenant, run).Where("external_task_ref <> ''").Order("updated_at desc").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AgentRunResource{}, sandbox.ErrNotFound
	}
	if err != nil {
		return AgentRunResource{}, err
	}
	var ref sandbox.ExecutionRef
	if err := json.Unmarshal([]byte(row.ExternalTaskRef), &ref); err != nil {
		return AgentRunResource{}, err
	}
	return AgentRunResource{TenantID: tenant, RunID: run, SessionID: ref.SessionID, Ref: ref, State: row.Status}, nil
}
func (r *GormAgentRunResourceRepository) Delete(ctx context.Context, tenant uint64, run string) error {
	return r.db.WithContext(ctx).Table("agent_tool_calls").Where("tenant_id=? AND run_id=?", tenant, run).Update("external_task_ref", "").Error
}
func (r *GormAgentRunResourceRepository) Protects(ctx context.Context, tenant uint64, run string) (bool, error) {
	v, e := r.Get(ctx, tenant, run)
	if errors.Is(e, sandbox.ErrNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	return !v.Terminal(), nil
}
func (r *GormAgentRunResourceRepository) ProtectsSandbox(ctx context.Context, summary sandbox.RemoteSandboxSummary) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("agent_tool_calls").Where("external_task_ref LIKE ? AND status NOT IN ?", "%\"instance_id\":\""+summary.ID+"\"%", []string{"succeeded", "failed", "cancelled", "released"}).Count(&n).Error
	return n > 0, err
}

// RegisterAgentRunResourceProtection wires the durable resource repository
// into Docker idle cleanup. The lookup is shared by newly created clients;
// repository errors make the sweeper fail closed.
func RegisterAgentRunResourceProtection(repo AgentRunResourceRepository) {
	if repo == nil {
		sandbox.ConfigureDockerResourceProtection(nil)
		return
	}
	if lookup, ok := repo.(interface {
		ProtectsSandbox(context.Context, sandbox.RemoteSandboxSummary) (bool, error)
	}); ok {
		sandbox.ConfigureDockerResourceProtection(lookup)
		return
	}
	sandbox.ConfigureDockerResourceProtection(sandbox.ProtectionLookupFunc(func(context.Context, sandbox.RemoteSandboxSummary) (bool, error) {
		return true, errors.New("sandbox resource protection lookup unavailable")
	}))
}

// MemoryAgentRunResourceRepository is suitable for process-local deployments
// and tests; production adapters can implement the same narrow contract.
type MemoryAgentRunResourceRepository struct {
	mu   sync.RWMutex
	rows map[string]AgentRunResource
}

func NewMemoryAgentRunResourceRepository() *MemoryAgentRunResourceRepository {
	return &MemoryAgentRunResourceRepository{rows: make(map[string]AgentRunResource)}
}
func resourceKey(tenant uint64, run string) string { return fmt.Sprintf("%d:%s", tenant, run) }
func (r *MemoryAgentRunResourceRepository) Put(ctx context.Context, v AgentRunResource) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if v.TenantID == 0 || v.RunID == "" {
		return ErrSandboxResourceConflict
	}
	if v.SessionID == "" || v.SessionID != v.Ref.SessionID {
		return ErrSandboxResourceConflict
	}
	if v.Ref.TenantID != v.TenantID {
		return ErrSandboxResourceConflict
	}
	if err := v.Ref.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := resourceKey(v.TenantID, v.RunID)
	if old, ok := r.rows[key]; ok && old.Ref != v.Ref {
		return ErrSandboxResourceConflict
	}
	r.rows[key] = v
	return nil
}
func (r *MemoryAgentRunResourceRepository) Get(ctx context.Context, tenant uint64, run string) (AgentRunResource, error) {
	if err := ctx.Err(); err != nil {
		return AgentRunResource{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.rows[resourceKey(tenant, run)]
	if !ok {
		return AgentRunResource{}, sandbox.ErrNotFound
	}
	return v, nil
}
func (r *MemoryAgentRunResourceRepository) Delete(ctx context.Context, tenant uint64, run string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, resourceKey(tenant, run))
	return nil
}
func (r *MemoryAgentRunResourceRepository) Protects(ctx context.Context, tenant uint64, run string) (bool, error) {
	v, e := r.Get(ctx, tenant, run)
	if errors.Is(e, sandbox.ErrNotFound) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	return !v.Terminal(), nil
}

// ProtectsSandbox is the error-capable hook consumed by provider sweepers.
// It fails closed when the repository cannot be read.
func (r *MemoryAgentRunResourceRepository) ProtectsSandbox(ctx context.Context, summary sandbox.RemoteSandboxSummary) (bool, error) {
	if err := ctx.Err(); err != nil {
		return true, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.rows {
		if v.Ref.InstanceID == summary.ID && !v.Terminal() {
			return true, nil
		}
	}
	return false, nil
}

// ReconcileExecution observes an existing task and records its state. A
// missing/unknown task is unavailable and never converted into success.
func ReconcileExecution(ctx context.Context, repo AgentRunResourceRepository, recovery sandbox.ExecutionRecovery, resource AgentRunResource) (sandbox.ExecutionObservation, error) {
	if repo == nil || recovery == nil {
		return sandbox.ExecutionObservation{State: "unknown"}, errors.New("sandbox recovery dependencies are required")
	}
	if resource.TenantID == 0 || resource.RunID == "" || resource.SessionID == "" || resource.Ref.TenantID != resource.TenantID || resource.SessionID != resource.Ref.SessionID {
		return sandbox.ExecutionObservation{State: "unknown"}, ErrSandboxResourceConflict
	}
	if err := resource.Ref.Validate(); err != nil {
		return sandbox.ExecutionObservation{State: "unknown"}, err
	}
	if existing, lookupErr := repo.Get(ctx, resource.TenantID, resource.RunID); lookupErr == nil {
		if existing.Ref != resource.Ref {
			return sandbox.ExecutionObservation{State: "unknown"}, ErrSandboxResourceConflict
		}
	} else if !errors.Is(lookupErr, sandbox.ErrNotFound) {
		return sandbox.ExecutionObservation{State: "unknown"}, lookupErr
	}
	obs, err := recovery.Observe(ctx, resource.Ref)
	if err != nil {
		return sandbox.ExecutionObservation{State: "unknown"}, err
	}
	obs = obs.Normalize()
	resource.State = obs.State
	if obs.State == "missing" || obs.State == "unknown" {
		resource.State = "sandbox_unavailable"
		if putErr := repo.Put(ctx, resource); putErr != nil {
			return obs, putErr
		}
		return obs, ErrSandboxUnavailable
	}
	if sandbox.CanImportObservation(obs) {
		if err := repo.Put(ctx, resource); err != nil {
			return obs, err
		}
		return obs, nil
	}
	if err := repo.Put(ctx, resource); err != nil {
		return obs, err
	}
	return obs, nil
}
