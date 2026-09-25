package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"gorm.io/gorm"
)

// CraftExecutionPolicyGate is the server-controlled command face of the T03
// uploaded-material execution policy (#122). Both seams run BEFORE any
// create, bind, claim or send, and a denial must surface the member-visible
// Refusal text on the error so the caller can project it into the
// member-facing response.
type CraftExecutionPolicyGate interface {
	// ReviewNormalExec screens one identity-carrying staged normal-exec
	// request before the service touches the coordinator or the engine.
	ReviewNormalExec(ctx context.Context, request repository.CraftDockerNormalInputRequest) error
	// ReviewOutputlessExec screens one restricted (outputless) exec before
	// the coordinator prepares the durable send.
	ReviewOutputlessExec(ctx context.Context, binding CraftCallBinding, request CraftDockerOutputlessRequest) error
}

// CraftDelegateExecutionPolicy adapts the gate onto
// CraftDelegateService.MaterialPolicy: it re-loads the admitted Run's frozen
// input manifest and workspace seed by durable identity (parameter-bound)
// and reviews the staged command inside the container workspace root.
//
// Symlink resolution: the reviewed targets live inside the container
// filesystem, which the server cannot walk, so ResolvedTargetPath stays
// empty here and the policy decides on lexical canonicalization plus
// canonical byte identity. Server-side execution surfaces that CAN resolve
// links (for example a host-side build entry) must call
// filepath.EvalSymlinks on the primary target and fill ResolvedTargetPath
// themselves before screening.
type CraftDelegateExecutionPolicy struct {
	delegate      *CraftDelegateService
	db            *gorm.DB
	workspaceRoot string
}

// NewCraftDelegateExecutionPolicy validates and assembles the gate adapter.
// workspaceRoot is the container workspace root (for example "/workspace").
func NewCraftDelegateExecutionPolicy(delegate *CraftDelegateService, db *gorm.DB, workspaceRoot string) (*CraftDelegateExecutionPolicy, error) {
	if delegate == nil || db == nil {
		return nil, fmt.Errorf("%w: execution policy adapter requires the delegation service and the database", craft.ErrInvalidInput)
	}
	if strings.TrimSpace(workspaceRoot) == "" {
		return nil, fmt.Errorf("%w: execution policy adapter requires the container workspace root", craft.ErrInvalidInput)
	}
	return &CraftDelegateExecutionPolicy{delegate: delegate, db: db, workspaceRoot: path.Clean(workspaceRoot)}, nil
}

// ReviewNormalExec screens the staged argv against the Run's admitted input
// manifest. Assembly, snapshot or identity failures fail closed.
func (g *CraftDelegateExecutionPolicy) ReviewNormalExec(ctx context.Context, request repository.CraftDockerNormalInputRequest) error {
	if g == nil {
		return fmt.Errorf("%w: execution policy gate is not assembled", craft.ErrForbidden)
	}
	execRequest := craft.InputExecutionRequest{
		Command:    append([]string(nil), request.Command...),
		WorkingDir: request.WorkingDir,
	}
	return g.review(ctx, request.TenantID, request.RunID, execRequest)
}

// ReviewOutputlessExec fails closed: the restricted request carries no
// durable Run identity (tenant/run) that the material policy could review
// uploaded code against, so an unreviewable command must never be sent.
// Mounting this face requires extending the restricted request with the
// durable identity first.
func (g *CraftDelegateExecutionPolicy) ReviewOutputlessExec(_ context.Context, _ CraftCallBinding, _ CraftDockerOutputlessRequest) error {
	if g == nil {
		return fmt.Errorf("%w: execution policy gate is not assembled", craft.ErrForbidden)
	}
	return fmt.Errorf("%w: restricted Docker exec carries no durable Run identity to review uploaded material against; refusing to send an unreviewable command. Allowed alternative: generated code inside the writable Workspace, staged through a normal exec that names the Run", craft.ErrForbidden)
}

func (g *CraftDelegateExecutionPolicy) review(ctx context.Context, tenantID uint64, runID string, execRequest craft.InputExecutionRequest) error {
	if tenantID == 0 || strings.TrimSpace(runID) == "" {
		return fmt.Errorf("%w: execution review requires the durable Run identity", craft.ErrInvalidInput)
	}
	var row struct {
		OwnerID   string
		SessionID string
		Snapshot  string
	}
	if err := g.db.WithContext(ctx).Table("agent_runs").
		Select("owner_id", "session_id", "snapshot").
		Where("tenant_id = ? AND run_id = ?", tenantID, runID).Take(&row).Error; err != nil {
		return err
	}
	snapshot, err := ParseDurableRunSnapshot(json.RawMessage(row.Snapshot))
	if err != nil {
		return err
	}
	inputs := []craft.Input{}
	if snapshot.CraftInputManifest != nil {
		inputs = *snapshot.CraftInputManifest
	}
	if snapshot.CraftWorkspaceSeed == nil {
		return fmt.Errorf("%w: run %s carries no craft workspace seed, so its command cannot be reviewed", craft.ErrConflict, runID)
	}
	task := craft.Task{
		Scope:       craft.Scope{TenantID: tenantID, UserID: row.OwnerID, SessionID: row.SessionID},
		WorkspaceID: snapshot.CraftWorkspaceSeed.WorkspaceID,
		Fence:       runtime.Fence{RunKey: runtime.RunKey{TenantID: tenantID, RunID: runID}},
		Inputs:      inputs,
	}
	policy, err := g.delegate.MaterialPolicy(g.workspaceRoot, task)
	if err != nil {
		return err
	}
	decision := policy.ReviewExecution(ctx, execRequest)
	if !decision.Allowed {
		return fmt.Errorf("%w: %s", craft.ErrForbidden, decision.Refusal())
	}
	return nil
}
