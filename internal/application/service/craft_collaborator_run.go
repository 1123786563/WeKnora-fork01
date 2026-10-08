package service

// T09 (#135): Collaborator serialized-edit admission. One Craft Task keeps
// ONE persistent Workspace whose storage identity stays the Task owner
// (Task ID = Session ID); every Run — whoever initiates it — is one
// execution inside that Task behind the same T16 workspace writer lease.
// StartCollaboratorRun is the role-deriving entrance for a serialized edit:
// it derives the caller's CURRENT Task role through the T08 authority (a
// current Owner or Collaborator passes; Viewer, revoked or stranger members
// are refused with the audited denial), delegates the admission itself to
// the existing StartRun boundary (workspace resolution, T01 idempotent
// admission, T16 lease acquisition and conflict projection), and records
// the ACTUAL initiating member on the Task timeline.

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// craftTaskRoleReader is the optional role projection of the T08 access
// service. The production assembly injects *CraftAccessService, which
// implements it; the timeline audit then records the derived role verbatim.
type craftTaskRoleReader interface {
	Role(context.Context, craft.Scope) (craft.TaskRole, error)
}

// craftTaskRoleGate is the optional single-derivation seam: the production
// *CraftAccessService enforces the TaskWrite gate and hands back the derived
// role in one pass, so the timeline detail reuses the SAME derivation
// instead of re-querying the membership/grant facts a second time.
type craftTaskRoleGate interface {
	CheckTaskAccessWithRole(context.Context, craft.Scope, craft.TaskAction) (craft.TaskRole, error)
}

// StartCollaboratorRun admits one serialized-edit Run requested by the
// authenticated member under their own CURRENT grant. The Run stays inside
// the same Workspace (the owner's persisted binding), must acquire the T16
// writer lease through the StartRun boundary, and its durable actor is the
// actual initiating member — the identity the RunView knowledge/connector
// authority follows, so the initiator's own grants (never the owner's
// personal connections, keys or inaccessible knowledge) scope execution.
func (s *CraftSessionService) StartCollaboratorRun(ctx context.Context, scope craft.Scope, req CraftRunRequest) (agentruntime.Run, craft.WriterAcquisition, error) {
	if s == nil {
		return agentruntime.Run{}, craft.WriterAcquisition{}, craft.ErrForbidden
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID == "" || caller.UserID != scope.UserID {
		return agentruntime.Run{}, craft.WriterAcquisition{}, craft.ErrForbidden
	}
	// Fresh TaskWrite derivation through the T08 authority BEFORE any
	// admission work: a Viewer, a revoked Collaborator or a stranger member
	// is refused here and the refusal is audited by the access service. When
	// the ACL assembly is absent the seam stays fail-closed for non-owner
	// callers through StartRun's writeSession owner fallback, so this entrance
	// never widens authority beyond the pre-T08 owner behavior. ONE derivation
	// serves both the gate and the timeline detail (the production seam
	// returns the derived role); the derivation StartRun repeats inside its
	// own boundary stays by design — removing the pre-gate would flip the
	// observable order for a Viewer sending an invalid body (400 before the
	// audited 403) and weaken the owner-fallback contract.
	var initiatorRole craft.TaskRole
	if s.access != nil {
		if gate, ok := s.access.(craftTaskRoleGate); ok {
			role, err := gate.CheckTaskAccessWithRole(ctx, scope, craft.TaskWrite)
			if err != nil {
				return agentruntime.Run{}, craft.WriterAcquisition{}, err
			}
			initiatorRole = role
		} else {
			// A non-standard checker keeps the plain gate plus the separate
			// informational role read (both optional seams degrade honestly).
			if err := craft.RequireTaskAccess(ctx, s.access, scope, craft.TaskWrite); err != nil {
				return agentruntime.Run{}, craft.WriterAcquisition{}, err
			}
			initiatorRole = s.initiatingMemberRole(ctx, scope)
		}
	}
	run, acquisition, err := s.StartRun(ctx, scope, req)
	if err != nil {
		return run, acquisition, err
	}
	// Timeline: record the actual initiating member of the admitted Run.
	// The row never substitutes the storage owner — the actor is exactly the
	// authenticated caller whose current grant admitted the Run.
	s.auditCollaboratorRunStart(ctx, scope, initiatorRole, run, req)
	return run, acquisition, nil
}

// initiatingMemberRole re-derives the initiator's CURRENT Task role for the
// timeline audit detail. It is informational only: an unavailable role
// projection degrades to the empty detail and never fails admission, which
// is already authorized through RequireTaskAccess above.
func (s *CraftSessionService) initiatingMemberRole(ctx context.Context, scope craft.Scope) craft.TaskRole {
	if s == nil || s.access == nil {
		return ""
	}
	reader, ok := s.access.(craftTaskRoleReader)
	if !ok {
		return ""
	}
	role, err := reader.Role(ctx, scope)
	if err != nil {
		return ""
	}
	return role
}

// auditCollaboratorRunStart persists one craft.run_started timeline row for
// an admitted serialized edit. TargetID binds the stable Run ID; ActorUserID
// is the actual initiating member. A failed audit write is logged and never
// fails the already-committed admission (the run row itself keeps the actor
// identity in agent_runs.actor_user_id).
func (s *CraftSessionService) auditCollaboratorRunStart(ctx context.Context, scope craft.Scope, role craft.TaskRole, run agentruntime.Run, req CraftRunRequest) {
	if s == nil || s.db == nil || run.Key.RunID == "" {
		return
	}
	// Replay dedup (the auditTaskDenial discipline): a T01 idempotent retry
	// with the same request id replays the SAME admission, so the
	// run_started row for that Run already exists — re-writing it would
	// append one timeline row per retry (with retry-time role details that
	// can drift from the first admission). The probe keys on the Run
	// identity: a same-key replay is always the same actor (a cross-actor
	// replay is refused inside StartRun). A probe failure degrades to
	// writing a duplicate — never to skipping the audit for another reason.
	var existing int64
	if err := s.db.WithContext(ctx).Model(&craftAccessAudit{}).
		Where("tenant_id = ? AND action = ? AND scope_id = ? AND target_id = ?",
			scope.TenantID, "craft.run_started", scope.SessionID, run.Key.RunID).
		Count(&existing).Error; err == nil && existing > 0 {
		return
	}
	actor, actorFull := craftAuditActorUserID(scope.UserID)
	details := map[string]string{"request_id": strings.TrimSpace(req.RequestID), "role": string(role)}
	if actorFull != "" {
		details["actor_user_id_full"] = actorFull
	}
	raw, err := json.Marshal(details)
	if err != nil {
		raw = []byte(`{}`)
	}
	if err := s.db.WithContext(ctx).Create(&craftAccessAudit{
		TenantID: scope.TenantID, ActorUserID: actor, Action: "craft.run_started",
		ScopeType: "session", ScopeID: scope.SessionID, TargetType: "run", TargetID: run.Key.RunID,
		Outcome: "success", Details: types.JSON(raw), CreatedAt: time.Now(),
	}).Error; err != nil {
		logger.Warnf(ctx, "[CraftSession] run-start audit failed for run %s: %v", run.Key.RunID, err)
	}
}
