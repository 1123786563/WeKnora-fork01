package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
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
// Evidence contract (explicit, per the OCR hardening round): the server
// cannot walk the container filesystem, so the symlink layer
// (ResolvedTargetPath) never fires here and file-target digests cannot be
// probed; the effective defenses on this face are the LEXICAL layers —
// exhaustive operand screening with wrapper- and version-aware interpreter
// detection, fail-closed shell expressions (a wrapped `sh -c "..."` is sent
// as Shell and denied without adapter evidence), environment-value
// containment, and stdin byte identity (the stdin digest is matched against
// the admitted manifest). A two-step copy-then-execute inside the container
// is therefore a KNOWN RESIDUAL at this face, to be closed by
// execution-receipt evidence when a component with container filesystem
// observability exists. Server-side surfaces that CAN resolve links must
// filepath.EvalSymlinks the primary target and fill ResolvedTargetPath
// themselves before screening.
type CraftDelegateExecutionPolicy struct {
	delegate      *CraftDelegateService
	db            *gorm.DB
	workspaceRoot string
	audit         interfaces.AuditLogService
}

// WithAuditLog attaches the durable audit sink for the gate's own
// fail-closed refusals (identity mismatch, missing run, unreviewable
// restricted exec): every denial persists one
// craft.input.execute_denied row (identities only, no payloads).
func (g *CraftDelegateExecutionPolicy) WithAuditLog(audit interfaces.AuditLogService) *CraftDelegateExecutionPolicy {
	if g != nil {
		g.audit = audit
	}
	return g
}

// writeDenied persists one denied audit row for a gate refusal that did not
// come from a policy decision (those are audited by CraftMaterialPolicy).
func (g *CraftDelegateExecutionPolicy) writeDenied(ctx context.Context, tenantID uint64, runID, detail string) {
	if g == nil || g.audit == nil {
		return
	}
	details, err := json.Marshal(map[string]string{"run": runID, "reason": detail})
	if err != nil {
		return
	}
	entry := &types.AuditLog{
		TenantID: tenantID, Action: types.AuditAction(craft.AuditKindInputExecuteDenied),
		Outcome: types.AuditOutcomeDenied, ScopeType: "craft_run", ScopeID: runID,
		Details: types.JSON(details),
	}
	if err := g.audit.Log(ctx, entry); err != nil {
		logger.Warnf(ctx, "[CraftExecutionPolicy] audit row write failed (denial stands): %v", err)
	}
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
		Command:     append([]string(nil), request.Command...),
		WorkingDir:  request.WorkingDir,
		Environment: request.Environment,
	}
	if request.StdinEnabled && len(request.Stdin) > 0 {
		// Byte identity for the stdin channel: the stdin bytes ride on the
		// reviewed request itself, so their digest can be matched against
		// the admitted manifest exactly like a file target.
		stdinSum := sha256.Sum256(request.Stdin)
		execRequest.TargetSHA256 = hex.EncodeToString(stdinSum[:])
	}
	if interpreter, offset := craft.InterpreterPrefix(request.Command); interpreter {
		rest := request.Command[offset:]
		// A wrapped shell reading its program from -c is a shell expression:
		// send it as Shell/CommandText, which the policy denies without
		// adapter-normalized evidence (lexical screening of shell forms is
		// bypassable by construction).
		if idx := shellCommandFlagIndex(rest); idx >= 0 && idx+1 < len(rest) {
			execRequest.Shell = true
			execRequest.CommandText = strings.Join(rest[idx+1:], " ")
			execRequest.Command = nil
			execRequest.Environment = nil
		} else if request.StdinEnabled && !hasScriptOperand(rest) {
			// A bare interpreter with stdin enabled reads its program from
			// stdin; the stdin digest screen above covers uploaded bytes,
			// but generated program bytes on stdin are indistinguishable
			// from an attempt to smuggle them, so refuse the form.
			g.writeDenied(ctx, request.TenantID, request.RunID, "interpreter reading its program from stdin")
			return fmt.Errorf("%w: an interpreter reading its program from stdin cannot be reviewed against the admitted material; stage the generated program as a file in the writable Workspace instead. Allowed alternative: execute a generated file, not stdin", craft.ErrForbidden)
		}
	}
	return g.review(ctx, request.TenantID, request.RunID, request.TaskID, execRequest)
}

// ReviewOutputlessExec fails closed: the restricted request carries no
// durable Run identity (tenant/run) that the material policy could review
// uploaded code against, so an unreviewable command must never be sent.
// Mounting this face requires extending the restricted request with the
// durable identity first.
func (g *CraftDelegateExecutionPolicy) ReviewOutputlessExec(ctx context.Context, _ CraftCallBinding, _ CraftDockerOutputlessRequest) error {
	if g == nil {
		return fmt.Errorf("%w: execution policy gate is not assembled", craft.ErrForbidden)
	}
	g.writeDenied(ctx, 0, "", "restricted exec carries no durable run identity")
	return fmt.Errorf("%w: restricted Docker exec carries no durable Run identity to review uploaded material against; refusing to send an unreviewable command. Allowed alternative: generated code inside the writable Workspace, staged through a normal exec that names the Run", craft.ErrForbidden)
}

// shellCommandFlagIndex returns the index of the -c flag in an interpreter
// operand list, or -1 when absent.
func shellCommandFlagIndex(rest []string) int {
	for i, arg := range rest {
		if arg == "-c" {
			return i
		}
	}
	return -1
}

// hasScriptOperand reports whether the interpreter operand list names a
// non-flag operand (a script path) as opposed to reading its program from
// stdin.
func hasScriptOperand(rest []string) bool {
	for _, arg := range rest {
		if !strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

func (g *CraftDelegateExecutionPolicy) review(ctx context.Context, tenantID uint64, runID, taskID string, execRequest craft.InputExecutionRequest) error {
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
		g.writeDenied(ctx, tenantID, runID, "run identity not resolvable")
		return err
	}
	// The reviewed identity must agree with the staged request's own task
	// binding, mirroring the coordinator's session/task consistency check:
	// a request may not review against a different, more permissive Run.
	if strings.TrimSpace(taskID) != "" && strings.TrimSpace(row.SessionID) != strings.TrimSpace(taskID) {
		g.writeDenied(ctx, tenantID, runID, "request task binding differs from the run session")
		return fmt.Errorf("%w: staged task %q is not the session of run %s", craft.ErrConflict, taskID, runID)
	}
	snapshot, err := ParseDurableRunSnapshot(json.RawMessage(row.Snapshot))
	if err != nil {
		g.writeDenied(ctx, tenantID, runID, "durable snapshot unparseable")
		return err
	}
	inputs := []craft.Input{}
	if snapshot.CraftInputManifest != nil {
		inputs = *snapshot.CraftInputManifest
	}
	if snapshot.CraftWorkspaceSeed == nil {
		g.writeDenied(ctx, tenantID, runID, "run carries no craft workspace seed")
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
		g.writeDenied(ctx, tenantID, runID, "material policy assembly failed")
		return err
	}
	decision := policy.ReviewExecution(ctx, execRequest)
	if !decision.Allowed {
		return fmt.Errorf("%w: %s", craft.ErrForbidden, decision.Refusal())
	}
	return nil
}
