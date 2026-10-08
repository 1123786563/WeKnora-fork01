// Package service - Craft restricted-share consent (T11, #128).
//
// The share service is the consent authority of the derived web result:
// it computes one immutable version's restricted-source contribution
// server-side from RECORDED evidence (the version's stored citation
// manifest plus the Run's immutable knowledge record), shows the owner the
// exact version and evidence digest they are consenting to, and records
// owner decisions bound to that identity. Sharing authority exists only
// while a live approved decision binds the CURRENT evidence; reject,
// expiry, replay (a decision of other evidence) and revocation never
// create it. Every projection carries only typed consent facts — sharing
// the derived result never grants original-source access — and every
// decision, revocation and proven refusal is audited.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftShareVersionReader reads one delivered immutable version. The
// production adapter is the same version store the Craft session reads.
type CraftShareVersionReader interface {
	Get(context.Context, craft.Scope, string) (craft.Version, error)
}

// CraftShareFileReader opens one stored version object by its durable ref.
// The production adapter is the existing file service.
type CraftShareFileReader interface {
	GetFile(context.Context, string) (io.ReadCloser, error)
}

// CraftShareConfig assembles the share service. Every port is required and
// the service fails closed without it: authority must never degrade into a
// permissive default.
type CraftShareConfig struct {
	DB         *gorm.DB
	Versions   CraftShareVersionReader
	Files      CraftShareFileReader
	Records    CraftCitationRecordReader
	TaskAccess craft.TaskAccessChecker
	Now        func() time.Time
}

// CraftShareService owns the restricted-share consent of web versions.
type CraftShareService struct {
	db         *gorm.DB
	versions   CraftShareVersionReader
	files      CraftShareFileReader
	records    CraftCitationRecordReader
	taskAccess craft.TaskAccessChecker
	now        func() time.Time
}

// NewCraftShareService validates the assembly.
func NewCraftShareService(cfg CraftShareConfig) (*CraftShareService, error) {
	if cfg.DB == nil {
		return nil, errors.New("craft: share service requires the database")
	}
	if cfg.Versions == nil || cfg.Files == nil || cfg.Records == nil || cfg.TaskAccess == nil {
		return nil, errors.New("craft: share service requires versions, files, records and task access")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CraftShareService{db: cfg.DB, versions: cfg.Versions, files: cfg.Files, records: cfg.Records, taskAccess: cfg.TaskAccess, now: now}, nil
}

// craftShareDecisionRow persists one task+version's latest owner decision.
// A fresh decision upserts the row; RevokedAt marks the row dead. The
// decision identity (Version ID + evidence digest) is stored with the row
// so authority checks re-derive the binding from recorded evidence rather
// than trusting the stored pair alone.
type craftShareDecisionRow struct {
	TenantID       uint64               `gorm:"primaryKey"`
	SessionID      string               `gorm:"primaryKey;column:session_id;type:varchar(128)"`
	VersionID      string               `gorm:"primaryKey;column:version_id;type:varchar(128)"`
	EvidenceDigest string               `gorm:"column:evidence_digest;type:char(64);not null"`
	OwnerID        string               `gorm:"column:owner_id;type:varchar(512);not null"`
	Decision       craft.DecisionStatus `gorm:"column:decision;type:varchar(16);not null"`
	DecidedAt      time.Time            `gorm:"column:decided_at;not null"`
	RevokedAt      *time.Time           `gorm:"column:revoked_at"`
	CreatedAt      time.Time            `gorm:"column:created_at"`
	UpdatedAt      time.Time            `gorm:"column:updated_at"`
}

func (craftShareDecisionRow) TableName() string { return "craft_share_decisions" }

// CraftShareView is the consent summary projected to members: the typed
// contribution, the current sharing state, the latest decision when one
// binds, and its expiry. It never carries source refs, excerpts or titles.
type CraftShareView struct {
	Contribution craft.RestrictedContribution `json:"contribution"`
	State        craft.ShareState             `json:"state"`
	Decision     *craft.ShareDecision         `json:"decision,omitempty"`
	ExpiresAt    *time.Time                   `json:"expires_at,omitempty"`
}

// contribution derives one version's restricted contribution from recorded
// evidence. The version must exist in the requesting scope; its citation
// manifest is read from the immutable version files (missing manifest = no
// declared facts), and the evidence record must be the published record of
// the exact Run that produced the version — the same server-side binding
// the citation gate enforces.
func (s *CraftShareService) contribution(ctx context.Context, scope craft.Scope, versionID string) (craft.RestrictedContribution, error) {
	if s == nil || s.versions == nil || s.files == nil || s.records == nil {
		return craft.RestrictedContribution{}, craft.ErrForbidden
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || strings.TrimSpace(versionID) == "" {
		return craft.RestrictedContribution{}, fmt.Errorf("%w: incomplete share request", craft.ErrInvalidInput)
	}
	// The only assembled VersionStore (repository.NewCraftVersionStore)
	// authorizes against the workspace OWNER: members who passed
	// RequireTaskAccess would still 403 under their own scope. Read with
	// the task owner's scope — the caller-facing TaskRead/TaskShare gates
	// ran before this point, and the owner scope is the store's authority
	// axis, not a privilege grant.
	ownerScope, err := s.taskOwnerScope(ctx, scope)
	if err != nil {
		return craft.RestrictedContribution{}, err
	}
	version, err := s.versions.Get(ctx, ownerScope, strings.TrimSpace(versionID))
	if err != nil {
		return craft.RestrictedContribution{}, err
	}
	// The version store is the scope authority: Get only answers versions
	// of the requesting task's workspace.
	manifest := craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: []craft.WebCitationEntry{}}
	for _, file := range version.Files {
		if file.Path != craft.WebCitationsPath {
			continue
		}
		reader, err := s.files.GetFile(ctx, file.Ref)
		if err != nil {
			return craft.RestrictedContribution{}, err
		}
		// Bounded read like every other GetFile consumer: the manifest's
		// recorded size caps a corrupted or mis-shelved object.
		raw, err := io.ReadAll(io.LimitReader(reader, file.Bytes+1))
		reader.Close()
		if int64(len(raw)) > file.Bytes {
			return craft.RestrictedContribution{}, fmt.Errorf("%w: citations manifest exceeds its recorded size", craft.ErrConflict)
		}
		if err != nil {
			return craft.RestrictedContribution{}, err
		}
		manifest, err = craft.DecodeWebCitationManifest(raw)
		if err != nil {
			return craft.RestrictedContribution{}, err
		}
		break
	}
	record, err := s.records.Load(ctx, scope, version.RunID)
	if err != nil {
		return craft.RestrictedContribution{}, err
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != version.RunID {
		return craft.RestrictedContribution{}, craft.ErrForbidden
	}
	if record.PublicationState != craft.KnowledgePublicationPublished {
		return craft.RestrictedContribution{}, craft.ErrConflict
	}
	return craft.RestrictedContributionFrom(version.ID, manifest, record)
}

// taskOwnerScope resolves the durable owner scope of a task's session: the
// sessions row's user_id is the workspace owner the version store authorizes
// against. It grants nothing — the caller already passed the task gates.
func (s *CraftShareService) taskOwnerScope(ctx context.Context, scope craft.Scope) (craft.Scope, error) {
	var owner string
	err := s.db.WithContext(ctx).Table("sessions").Select("user_id").
		Where("tenant_id = ? AND id = ?", scope.TenantID, scope.SessionID).Take(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craft.Scope{}, craft.ErrNotFound
	}
	if err != nil {
		return craft.Scope{}, err
	}
	if owner == "" {
		return craft.Scope{}, craft.ErrNotFound
	}
	return craft.Scope{TenantID: scope.TenantID, UserID: owner, SessionID: scope.SessionID}, nil
}

// decision loads the latest persisted decision row for one task+version.
func (s *CraftShareService) decision(ctx context.Context, scope craft.Scope, versionID string) (*craft.RecordedShareDecision, error) {
	var row craftShareDecisionRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND version_id = ?", scope.TenantID, scope.SessionID, strings.TrimSpace(versionID)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	recorded := &craft.RecordedShareDecision{
		Decision: craft.ShareDecision{
			VersionID:      row.VersionID,
			EvidenceDigest: row.EvidenceDigest,
			OwnerID:        row.OwnerID,
			Decision:       row.Decision,
		},
		DecidedAt: row.DecidedAt,
	}
	if row.RevokedAt != nil {
		recorded.RevokedAt = *row.RevokedAt
	}
	return recorded, nil
}

// projectCraftShareView projects the externally visible consent summary
// from a derived contribution and its latest recorded decision (nil when
// none was ever persisted). Both the read path (view) and the decision
// path (DecideShare) compose through this single projection: the immediate
// decision response and the later persisted projection are the same code,
// so gating, decision projection and TTL expiry can never drift apart.
func projectCraftShareView(contribution craft.RestrictedContribution, decision *craft.RecordedShareDecision, now time.Time) CraftShareView {
	state := craft.ShareStateOf(contribution, decision, now)
	view := CraftShareView{Contribution: contribution, State: state}
	// Only a RESTRICTED contribution carries a meaningful decision
	// projection. ShareStateOf reports non-restricted versions as
	// consented unconditionally; projecting a bound historical rejected
	// decision beside that would tell the client "consented + rejected"
	// (with a misleading expires_at), and downstream guards that downgrade
	// on decision!=approved would mislabel a shareable version private.
	if contribution.Restricted && decision != nil && craft.DecisionBinds(decision.Decision, contribution) {
		bound := decision.Decision
		view.Decision = &bound
		if state == craft.ShareStateConsented {
			expires := decision.DecidedAt.Add(craft.ShareDecisionTTL)
			view.ExpiresAt = &expires
		}
	}
	return view
}

// view assembles the externally visible consent summary.
func (s *CraftShareService) view(ctx context.Context, scope craft.Scope, versionID string) (CraftShareView, error) {
	contribution, err := s.contribution(ctx, scope, versionID)
	if err != nil {
		return CraftShareView{}, err
	}
	decision, err := s.decision(ctx, scope, versionID)
	if err != nil {
		return CraftShareView{}, err
	}
	return projectCraftShareView(contribution, decision, s.now()), nil
}

// ShareView projects one version's consent summary to a task member
// (a TaskRead fact). The projection is typed consent facts only.
func (s *CraftShareService) ShareView(ctx context.Context, scope craft.Scope, versionID string) (CraftShareView, error) {
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
		return CraftShareView{}, err
	}
	return s.view(ctx, scope, versionID)
}

// DecideShare records the owner's explicit decision. Only the CURRENT Task
// Owner can consent (a fresh TaskShare check on every submission — a
// revoked owner is refused like any non-owner). The decision binds the
// contribution computed server-side at submission time; seenDigest is the
// evidence digest the owner is deciding on, and a mismatch is a conflict:
// replaying a decision recorded for other evidence creates no authority.
func (s *CraftShareService) DecideShare(ctx context.Context, scope craft.Scope, versionID string, decision craft.DecisionStatus, seenDigest string) (CraftShareView, error) {
	if decision != craft.DecisionApproved && decision != craft.DecisionRejected {
		return CraftShareView{}, fmt.Errorf("%w: share decision must be approved or rejected", craft.ErrInvalidInput)
	}
	seenDigest = strings.TrimSpace(seenDigest)
	if seenDigest == "" {
		return CraftShareView{}, fmt.Errorf("%w: a share decision must state the evidence digest it binds", craft.ErrInvalidInput)
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskShare); err != nil {
		s.auditShare(ctx, scope, versionID, craftShareDenyActionTaskAccess, "denied", map[string]string{"attempted_decision": string(decision)})
		return CraftShareView{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		s.auditShare(ctx, scope, versionID, craftShareDenyActionCallerIdentity, "denied", map[string]string{"reason": "caller_identity_mismatch"})
		return CraftShareView{}, craft.ErrForbidden
	}
	contribution, err := s.contribution(ctx, scope, versionID)
	if err != nil {
		return CraftShareView{}, err
	}
	if seenDigest != contribution.EvidenceDigest {
		// A digest mismatch is a replay signal (a decision recorded for
		// OTHER evidence being replayed against this version): exactly the
		// proven refusal the package audit contract names.
		s.auditShare(ctx, scope, versionID, craftShareDenyActionEvidenceDigest, "denied", map[string]string{"reason": "evidence_digest_mismatch"})
		return CraftShareView{}, fmt.Errorf("%w: the decision binds other evidence than the version's current evidence", craft.ErrConflict)
	}
	now := s.now()
	row := craftShareDecisionRow{
		TenantID: scope.TenantID, SessionID: scope.SessionID, VersionID: contribution.VersionID,
		EvidenceDigest: contribution.EvidenceDigest, OwnerID: scope.UserID, Decision: decision,
		DecidedAt: now, RevokedAt: nil, CreatedAt: now, UpdatedAt: now,
	}
	// One row per task+version: a fresh decision replaces (and thereby
	// supersedes) whatever was decided before, including its timestamps.
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "session_id"}, {Name: "version_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"evidence_digest", "owner_id", "decision", "decided_at", "revoked_at", "updated_at"}),
	}).Create(&row).Error; err != nil {
		return CraftShareView{}, err
	}
	s.auditShare(ctx, scope, contribution.VersionID, "craft.share_decision_recorded", "success", map[string]string{
		"decision": string(decision), "evidence_digest": contribution.EvidenceDigest,
	})
	// The contribution was already derived above; composing the response
	// view from it and the row just written skips a full re-read
	// (versions.Get + files.GetFile + records.Load) on the hot path.
	recorded := &craft.RecordedShareDecision{
		Decision: craft.ShareDecision{
			VersionID:      row.VersionID,
			EvidenceDigest: row.EvidenceDigest,
			OwnerID:        row.OwnerID,
			Decision:       row.Decision,
		},
		DecidedAt: row.DecidedAt,
	}
	// The row was just upserted with revoked_at reset to NULL, so the
	// recorded decision's RevokedAt stays zero (never revoked) by design.
	// The response projects through the same shared projection the read
	// path uses, so it can never drift from the persisted view.
	return projectCraftShareView(contribution, recorded, s.now()), nil
}

// RevokeShare ends a live consent: the persisted decision row is marked
// revoked and can never grant again, even inside its TTL. Only the current
// Task Owner may revoke.
func (s *CraftShareService) RevokeShare(ctx context.Context, scope craft.Scope, versionID string) (CraftShareView, error) {
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskShare); err != nil {
		s.auditShare(ctx, scope, versionID, craftShareDenyActionTaskAccess, "denied", map[string]string{"attempted_decision": "revoke"})
		return CraftShareView{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		s.auditShare(ctx, scope, versionID, craftShareDenyActionCallerIdentity, "denied", map[string]string{"reason": "caller_identity_mismatch"})
		return CraftShareView{}, craft.ErrForbidden
	}
	versionID = strings.TrimSpace(versionID)
	// Derive the contribution BEFORE any mutation: a transient read failure
	// must not leave a persisted revocation that is reported to the caller
	// as a failed revoke.
	contribution, err := s.contribution(ctx, scope, versionID)
	if err != nil {
		return CraftShareView{}, err
	}
	now := s.now()
	result := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND version_id = ? AND revoked_at IS NULL", scope.TenantID, scope.SessionID, versionID).
		Updates(&craftShareDecisionRow{RevokedAt: &now, UpdatedAt: now})
	if result.Error != nil {
		return CraftShareView{}, result.Error
	}
	if result.RowsAffected > 0 {
		s.auditShare(ctx, scope, versionID, "craft.share_revoked", "success", nil)
	}
	// One single-row decision read projects the response from the
	// pre-derived immutable contribution. If that confirming read fails
	// after a durable revocation, the error says the revocation PERSISTED
	// instead of masquerading as a failed revoke; with no live row the
	// revocation is idempotent and the projection reports the current
	// state (never decided, or already revoked).
	decisionRow, derr := s.decision(ctx, scope, versionID)
	if derr != nil {
		if result.RowsAffected > 0 {
			return CraftShareView{}, fmt.Errorf("%w: revocation persisted for version %s but the confirming read failed: %v", craft.ErrConflict, versionID, derr)
		}
		return CraftShareView{}, derr
	}
	return projectCraftShareView(contribution, decisionRow, now), nil
}

// ShareAuthority is the gate downstream sharing surfaces (T13 export, T20
// composition) consult: it re-derives the contribution from recorded
// evidence and reports whether a live approved owner decision binds it
// right now. It grants no original-source access by construction — the
// answer is the typed contribution plus a boolean.
func (s *CraftShareService) ShareAuthority(ctx context.Context, scope craft.Scope, versionID string) (craft.RestrictedContribution, bool, error) {
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
		return craft.RestrictedContribution{}, false, err
	}
	contribution, err := s.contribution(ctx, scope, versionID)
	if err != nil {
		return craft.RestrictedContribution{}, false, err
	}
	if !contribution.Restricted {
		return contribution, true, nil
	}
	decision, err := s.decision(ctx, scope, versionID)
	if err != nil {
		return contribution, false, err
	}
	return contribution, craft.GrantsShareAuthority(contribution, decision, s.now()), nil
}

// The share denial audit actions fold the refusal REASON into the typed
// action column. The reason is part of the dedup key: inside the window
// only denials of the SAME reason collapse, so a proven refusal of another
// reason — in particular the evidence-digest replay signal — always lands
// its row ("every proven refusal is audited"). The typed-column discipline
// mirrors auditTaskDenial: the probe never depends on JSON comparison
// semantics. The vocabulary is finite and server-controlled; an unknown
// denial action never reaches the audit sink.
const (
	craftShareDenyActionTaskAccess     = "craft.share_denied:task_access"
	craftShareDenyActionCallerIdentity = "craft.share_denied:caller_identity"
	craftShareDenyActionEvidenceDigest = "craft.share_denied:evidence_digest"
)

var craftShareDeniedActionVocabulary = map[string]bool{
	craftShareDenyActionTaskAccess:     true,
	craftShareDenyActionCallerIdentity: true,
	craftShareDenyActionEvidenceDigest: true,
}

// auditShare records a durable sharing event into the shared audit trail.
// The write failure is logged, never propagated: authority was already
// decided and must not flip on the audit sink. DENIAL rows carry the same
// sliding-window dedup as craft_access.auditTaskDenial (shared
// craftDenyDedupWindow), keyed on the reason-carrying action: a probing
// client replaying the same denial of the same reason must not be able to
// flood audit_logs at request rate, while a refusal of another reason is
// never swallowed by an earlier row.
func (s *CraftShareService) auditShare(ctx context.Context, scope craft.Scope, versionID, action, outcome string, details map[string]string) {
	if outcome == "denied" && !craftShareDeniedActionVocabulary[action] {
		return
	}
	actor, full := craftAuditActorUserID(scope.UserID)
	if details == nil {
		details = map[string]string{}
	}
	if full != "" {
		details["actor_user_id_full"] = full
	}
	raw, err := json.Marshal(details)
	if err != nil {
		raw = []byte(`{}`)
	}
	versionID = strings.TrimSpace(versionID)
	// The injected clock keeps the deny-dedup window and the audit row's
	// CreatedAt deterministic under fake-clock tests, exactly like every
	// other time judgment in this service (and like auditExport's fixed
	// clock in the T12 lane).
	now := s.now()
	if outcome == "denied" {
		since := now.Add(-craftDenyDedupWindow)
		var recent int64
		if err := s.db.WithContext(ctx).Model(&craftAccessAudit{}).
			Where("tenant_id = ? AND actor_user_id = ? AND action = ? AND scope_id = ? AND target_id = ? AND outcome = ? AND created_at > ?",
				scope.TenantID, actor, action, scope.SessionID, versionID, "denied", since).
			Count(&recent).Error; err == nil && recent > 0 {
			return
		}
	}
	if err := s.db.WithContext(ctx).Create(&craftAccessAudit{
		TenantID: scope.TenantID, ActorUserID: actor, Action: action,
		ScopeType: "session", ScopeID: scope.SessionID, TargetType: "artifact_version",
		TargetID: versionID, Outcome: outcome,
		Details: types.JSON(raw), CreatedAt: now,
	}).Error; err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"audit_action": action})
	}
}
