// Package service - Craft restricted derived-export consent (T13, #133).
//
// The export consent service is the consent authority of the source
// bundle: it classifies one immutable version's derived members from the
// RECORDED origins the T12 export projection already pinned, shows the
// owner the exact files and origins with the manifest digest, and records
// owner decisions bound to that immutable identity. The consent-gated
// bundle projection (ConsentGatedExportService) wraps the T12 export
// service: without a live approved decision of the CURRENT owner no
// restricted derived byte streams — the safe fallback bundle carries only
// the unrestricted members with its own honestly distinct digest.
// Restricted originals never enter a version manifest at all, every
// decision, proven refusal and withholding is audited, and replayed
// decisions for another manifest are a proven conflict.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftExportConsentConfig assembles the consent service. Every port is
// required and the service fails closed without it.
type CraftExportConsentConfig struct {
	DB         *gorm.DB
	Versions   CraftExportVersionReader
	Evidence   CraftExportEvidenceReader
	TaskAccess craft.TaskAccessChecker
	Now        func() time.Time
}

// CraftExportConsentService owns the restricted derived-export consent.
type CraftExportConsentService struct {
	db         *gorm.DB
	versions   CraftExportVersionReader
	evidence   CraftExportEvidenceReader
	taskAccess craft.TaskAccessChecker
	now        func() time.Time
}

// NewCraftExportConsentService validates the assembly.
func NewCraftExportConsentService(cfg CraftExportConsentConfig) (*CraftExportConsentService, error) {
	if cfg.DB == nil {
		return nil, errors.New("craft: export consent service requires the database")
	}
	if cfg.Versions == nil || cfg.Evidence == nil || cfg.TaskAccess == nil {
		return nil, errors.New("craft: export consent service requires versions, evidence and task access")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CraftExportConsentService{
		db: cfg.DB, versions: cfg.Versions, evidence: cfg.Evidence,
		taskAccess: cfg.TaskAccess, now: now,
	}, nil
}

// craftExportDecisionRow persists one task+version's latest owner export
// decision. A fresh decision upserts the row. The decision identity
// (Version ID + Export Manifest digest) is stored with the row so
// authority checks re-derive the binding from the CURRENT manifest rather
// than trusting the stored pair alone — a manifest change therefore
// invalidates the old consent by construction.
type craftExportDecisionRow struct {
	TenantID       uint64               `gorm:"primaryKey"`
	SessionID      string               `gorm:"primaryKey;column:session_id;type:varchar(128)"`
	VersionID      string               `gorm:"primaryKey;column:version_id;type:varchar(128)"`
	ManifestDigest string               `gorm:"column:manifest_digest;type:char(64);not null"`
	OwnerID        string               `gorm:"column:owner_id;type:varchar(512);not null"`
	Decision       craft.DecisionStatus `gorm:"column:decision;type:varchar(16);not null"`
	DecidedAt      time.Time            `gorm:"column:decided_at;not null"`
	CreatedAt      time.Time            `gorm:"column:created_at"`
	UpdatedAt      time.Time            `gorm:"column:updated_at"`
}

func (craftExportDecisionRow) TableName() string { return "craft_export_decisions" }

// CraftExportConsentView is the consent summary projected to members: the
// exact export manifest (files with their recorded origins), the current
// state, the restricted derived classification and the latest decision
// when the CURRENT owner's decision binds this manifest. It never carries
// source bytes, excerpts or provider URLs.
type CraftExportConsentView struct {
	Manifest          craft.ExportManifest     `json:"manifest"`
	Decision          *craft.ExportDecision    `json:"decision,omitempty"`
	State             craft.ExportConsentState `json:"state"`
	RestrictedDerived []string                 `json:"restricted_derived"`
}

// manifest derives one version's export manifest from immutable facts
// only, exactly as the T12 bundle projection does: the version store's own
// scope ACL answers a foreign task's version as missing, a version without
// pinned evidence honestly refuses, and member paths are defensively
// re-validated before anything is classified. No titles are resolved — the
// consent identity is the manifest digest, and display names never enter
// it.
func (s *CraftExportConsentService) manifest(ctx context.Context, scope craft.Scope, versionID string) (craft.ExportManifest, error) {
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return craft.ExportManifest{}, err
	}
	evidence, err := s.evidence.VersionEvidence(ctx, scope, versionID)
	if err != nil {
		logger.Warnf(ctx, "[CraftExportConsent] manifest refused for version %s: no pinned evidence: %v", versionID, err)
		return craft.ExportManifest{}, err
	}
	if err := craft.ValidateExportBundleMembers(version.Files); err != nil {
		logger.Warnf(ctx, "[CraftExportConsent] manifest refused for version %s: %v", versionID, err)
		return craft.ExportManifest{}, err
	}
	return craft.BuildExportManifest(version, evidence, scope.TenantID)
}

// currentOwner resolves the task's CURRENT owner from the durable sessions
// row — the same fact the T11 share service reads. Authority follows the
// current owner only: a former owner's recorded decision stops granting
// the moment the row moves.
func (s *CraftExportConsentService) currentOwner(ctx context.Context, scope craft.Scope) (string, error) {
	var owner string
	err := s.db.WithContext(ctx).Table("sessions").Select("user_id").
		Where("tenant_id = ? AND id = ?", scope.TenantID, scope.SessionID).Take(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", craft.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if owner == "" {
		return "", craft.ErrNotFound
	}
	return owner, nil
}

// decision loads the latest persisted decision row for one task+version.
func (s *CraftExportConsentService) decision(ctx context.Context, scope craft.Scope, versionID string) (*craft.ExportDecision, error) {
	var row craftExportDecisionRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND version_id = ?", scope.TenantID, scope.SessionID, strings.TrimSpace(versionID)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	decision := &craft.ExportDecision{
		VersionID:      row.VersionID,
		ManifestDigest: row.ManifestDigest,
		OwnerID:        row.OwnerID,
		Decision:       row.Decision,
	}
	return decision, nil
}

// projectCraftExportConsentView is the single projection both the read
// path and the decision path compose through: the state reduces from the
// manifest's restricted derived members, the persisted decision and the
// CURRENT owner, and only the current owner's manifest-binding decision
// projects — a stale (other digest/version) or former-owner decision is
// history and never renders beside a state that claims authority.
func projectCraftExportConsentView(manifest craft.ExportManifest, restricted []string, decision *craft.ExportDecision, currentOwner string) CraftExportConsentView {
	state := craft.ExportConsentStateOf(restricted, decision, manifest.VersionID, manifest.ManifestDigest, currentOwner)
	view := CraftExportConsentView{Manifest: manifest, State: state, RestrictedDerived: restricted}
	if decision != nil && decision.OwnerID == currentOwner &&
		craft.ExportDecisionBinds(*decision, manifest.VersionID, manifest.ManifestDigest) {
		view.Decision = decision
	}
	return view
}

// view assembles the externally visible consent summary. The projected
// view satisfies the T00 frozen contract: a decision, when present, binds
// exactly the manifest it rides with.
func (s *CraftExportConsentService) view(ctx context.Context, scope craft.Scope, versionID string) (CraftExportConsentView, error) {
	if s == nil || s.versions == nil || s.evidence == nil {
		return CraftExportConsentView{}, craft.ErrForbidden
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return CraftExportConsentView{}, fmt.Errorf("%w: incomplete export consent request", craft.ErrInvalidInput)
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return CraftExportConsentView{}, fmt.Errorf("%w: export consent requires the version id", craft.ErrInvalidInput)
	}
	if !craft.ValidVersionID(versionID) {
		// A malformed id is indistinguishable from a missing version: the
		// refusal is stable and non-leaking, and no lookup happens.
		return CraftExportConsentView{}, craft.ErrNotFound
	}
	manifest, err := s.manifest(ctx, scope, versionID)
	if err != nil {
		return CraftExportConsentView{}, err
	}
	owner, err := s.currentOwner(ctx, scope)
	if err != nil {
		return CraftExportConsentView{}, err
	}
	decision, err := s.decision(ctx, scope, versionID)
	if err != nil {
		return CraftExportConsentView{}, err
	}
	view := projectCraftExportConsentView(manifest, craft.RestrictedDerivedPaths(manifest), decision, owner)
	if err := (craft.ExportConsentView{Manifest: view.Manifest, Decision: view.Decision}).Validate(); err != nil {
		return CraftExportConsentView{}, err
	}
	return view, nil
}

// ExportConsentView projects one version's consent summary to a task
// member (a TaskRead fact): the exact manifest with every member's
// recorded origins, the restricted derived classification, the state and
// the binding decision. The projection is typed consent facts only.
func (s *CraftExportConsentService) ExportConsentView(ctx context.Context, scope craft.Scope, versionID string) (CraftExportConsentView, error) {
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
		return CraftExportConsentView{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		s.auditExportConsentByCaller(ctx, scope, caller, versionID, craftExportDenyActionCallerIdentity, "denied", nil)
		return CraftExportConsentView{}, craft.ErrForbidden
	}
	return s.view(ctx, scope, versionID)
}

// DecideExport records the owner's explicit decision. Only the CURRENT
// Task Owner can decide (a fresh TaskShare check on every submission — a
// revoked owner is refused like any non-owner). The decision binds the
// manifest computed server-side at submission time; seenDigest is the
// manifest digest the owner is deciding on, and a mismatch is a conflict:
// replaying a decision recorded for another manifest creates no authority.
func (s *CraftExportConsentService) DecideExport(ctx context.Context, scope craft.Scope, versionID string, decision craft.DecisionStatus, seenDigest string) (CraftExportConsentView, error) {
	if decision != craft.DecisionApproved && decision != craft.DecisionRejected {
		return CraftExportConsentView{}, fmt.Errorf("%w: export decision must be approved or rejected", craft.ErrInvalidInput)
	}
	seenDigest = strings.TrimSpace(seenDigest)
	if seenDigest == "" {
		return CraftExportConsentView{}, fmt.Errorf("%w: an export decision must state the manifest digest it binds", craft.ErrInvalidInput)
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskShare); err != nil {
		s.auditExportConsent(ctx, scope, versionID, craftExportDenyActionTaskAccess, "denied", map[string]string{"attempted_decision": string(decision)})
		return CraftExportConsentView{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		s.auditExportConsentByCaller(ctx, scope, caller, versionID, craftExportDenyActionCallerIdentity, "denied", map[string]string{"reason": "caller_identity_mismatch"})
		return CraftExportConsentView{}, craft.ErrForbidden
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return CraftExportConsentView{}, fmt.Errorf("%w: incomplete export consent request", craft.ErrInvalidInput)
	}
	versionID = strings.TrimSpace(versionID)
	if !craft.ValidVersionID(versionID) {
		return CraftExportConsentView{}, craft.ErrNotFound
	}
	manifest, err := s.manifest(ctx, scope, versionID)
	if err != nil {
		return CraftExportConsentView{}, err
	}
	if seenDigest != manifest.ManifestDigest {
		// A digest mismatch is a replay signal (a decision recorded for
		// ANOTHER manifest being replayed against this version): exactly
		// the proven refusal the audit contract names.
		s.auditExportConsent(ctx, scope, versionID, craftExportConsentDenyActionManifestDigest, "denied", map[string]string{"reason": "manifest_digest_mismatch"})
		return CraftExportConsentView{}, fmt.Errorf("%w: the decision binds another manifest than the version's current export manifest", craft.ErrConflict)
	}
	owner, err := s.currentOwner(ctx, scope)
	if err != nil {
		return CraftExportConsentView{}, err
	}
	now := s.now()
	row := craftExportDecisionRow{
		TenantID: scope.TenantID, SessionID: scope.SessionID, VersionID: manifest.VersionID,
		ManifestDigest: manifest.ManifestDigest, OwnerID: scope.UserID, Decision: decision,
		DecidedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	// One row per task+version: a fresh decision replaces (and thereby
	// supersedes) whatever was decided before, including its timestamps —
	// a rejection is never final, and an approval can be withdrawn by a
	// fresh rejection.
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "session_id"}, {Name: "version_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"manifest_digest", "owner_id", "decision", "decided_at", "updated_at"}),
	}).Create(&row).Error; err != nil {
		return CraftExportConsentView{}, err
	}
	s.auditExportConsent(ctx, scope, manifest.VersionID, "craft.export_decision_recorded", "success", map[string]string{
		"decision": string(decision), "manifest_digest": manifest.ManifestDigest,
	})
	// The manifest was already derived above; composing the response view
	// from it and the row just written skips a full re-read on the hot
	// path, through the same shared projection the read path uses.
	recorded := &craft.ExportDecision{
		VersionID:      row.VersionID,
		ManifestDigest: row.ManifestDigest,
		OwnerID:        row.OwnerID,
		Decision:       row.Decision,
	}
	return projectCraftExportConsentView(manifest, craft.RestrictedDerivedPaths(manifest), recorded, owner), nil
}

// exportAuthority answers the gate's question for one already-projected
// manifest: the consent state and the restricted derived paths. The inner
// projection has already re-checked Task membership and the caller
// identity on THIS request, so only the decision row and the current
// owner remain to consult.
func (s *CraftExportConsentService) exportAuthority(ctx context.Context, scope craft.Scope, manifest craft.ExportManifest) (craft.ExportConsentState, []string, error) {
	restricted := craft.RestrictedDerivedPaths(manifest)
	if len(restricted) == 0 {
		return craft.ExportConsentNone, restricted, nil
	}
	owner, err := s.currentOwner(ctx, scope)
	if err != nil {
		return craft.ExportConsentAwaiting, restricted, err
	}
	decision, err := s.decision(ctx, scope, manifest.VersionID)
	if err != nil {
		return craft.ExportConsentAwaiting, restricted, err
	}
	return craft.ExportConsentStateOf(restricted, decision, manifest.VersionID, manifest.ManifestDigest, owner), restricted, nil
}

// The export consent denial audit actions fold the refusal REASON into the
// typed action column (the T11 discipline): the reason is part of the
// dedup key, and the vocabulary is finite and server-controlled. The
// withholding action carries its reason the same way — the restricted
// derived export was refused even though the safe bundle streamed.
const (
	craftExportConsentDenyActionManifestDigest = "craft.export_denied:manifest_digest"
	craftExportDerivedWithheldAction           = "craft.export_derived_withheld"
)

var craftExportConsentDeniedActionVocabulary = map[string]bool{
	craftExportDenyActionTaskAccess:            true,
	craftExportDenyActionCallerIdentity:        true,
	craftExportConsentDenyActionManifestDigest: true,
	craftExportDerivedWithheldAction:           true,
}

// auditExportConsent records a durable export-consent event into the
// shared audit trail with the T11/T12 discipline: DENIAL rows carry the
// reason in the typed action and dedupe inside the shared sliding window;
// the write failure is logged, never propagated — authority was already
// decided and must not flip on the audit sink.
func (s *CraftExportConsentService) auditExportConsent(ctx context.Context, scope craft.Scope, versionID, action, outcome string, details map[string]string) {
	s.auditExportConsentByCaller(ctx, scope, types.Caller{TenantID: scope.TenantID, UserID: scope.UserID}, versionID, action, outcome, details)
}

// auditExportConsentByCaller keeps the requested Task/version as the target
// while attributing caller-identity denials to the authenticated actor.
func (s *CraftExportConsentService) auditExportConsentByCaller(ctx context.Context, scope craft.Scope, caller types.Caller, versionID, action, outcome string, details map[string]string) {
	if outcome == "denied" && !craftExportConsentDeniedActionVocabulary[action] {
		return
	}
	actor, full := craftAuditActorUserID(caller.UserID)
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
	now := s.now()
	if outcome == "denied" {
		since := now.Add(-craftDenyDedupWindow)
		var recent int64
		if err := s.db.WithContext(ctx).Model(&craftAccessAudit{}).
			Where("tenant_id = ? AND actor_user_id = ? AND action = ? AND scope_id = ? AND target_id = ? AND outcome = ? AND created_at > ?",
				caller.TenantID, actor, action, scope.SessionID, versionID, "denied", since).
			Count(&recent).Error; err == nil && recent > 0 {
			return
		}
	}
	if err := s.db.WithContext(ctx).Create(&craftAccessAudit{
		TenantID: caller.TenantID, ActorUserID: actor, Action: action,
		ScopeType: "session", ScopeID: scope.SessionID, TargetType: "artifact_version",
		TargetID: versionID, Outcome: outcome,
		Details: types.JSON(raw), CreatedAt: now,
	}).Error; err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"audit_action": action})
	}
}

// CraftExportBundleProjector is the bundle projection the consent gate
// wraps. The production implementation is the T12 CraftExportService; the
// gate never re-derives the bundle, it only filters what may stream.
type CraftExportBundleProjector interface {
	ExportBundle(context.Context, craft.Scope, string) (CraftExportBundle, error)
}

// ConsentGatedExportService gates the T12 bundle projection behind the
// owner's export consent. It implements the same ExportBundle seam, so
// central assembly registers it where the raw export service stood and
// the T12 download surface keeps streaming — now of the bundle the gate
// hands it:
//
//   - a manifest without restricted derived members streams whole (there
//     is nothing to consent to);
//   - a live approved decision of the CURRENT owner binding exactly this
//     Version ID + manifest digest unlocks the full bundle;
//   - anything else — no decision, an explicit rejection, a stale replayed
//     decision, a former owner's decision — streams the SAFE bundle: the
//     unrestricted members with their own honestly distinct manifest
//     digest, and the withholding is audited. Restricted originals are
//     excluded from every branch by construction (T12).
type ConsentGatedExportService struct {
	inner   CraftExportBundleProjector
	consent *CraftExportConsentService
}

// NewConsentGatedExportService validates the assembly: a nil projector or
// consent service refuses every request instead of degrading into the
// ungated T12 projection.
func NewConsentGatedExportService(inner CraftExportBundleProjector, consent *CraftExportConsentService) (*ConsentGatedExportService, error) {
	if inner == nil || consent == nil {
		return nil, errors.New("craft: consent-gated export requires the bundle projector and the export consent service")
	}
	return &ConsentGatedExportService{inner: inner, consent: consent}, nil
}

// ExportBundle projects one version's bundle under the consent gate. The
// inner projection has already re-checked CURRENT Task membership, the
// caller identity, the member paths and the manifest digest (and audited
// the bundle event); this gate only decides which members may leave.
func (s *ConsentGatedExportService) ExportBundle(ctx context.Context, scope craft.Scope, versionID string) (CraftExportBundle, error) {
	if s == nil || s.inner == nil || s.consent == nil {
		return CraftExportBundle{}, craft.ErrForbidden
	}
	bundle, err := s.inner.ExportBundle(ctx, scope, versionID)
	if err != nil {
		return CraftExportBundle{}, err
	}
	state, restricted, err := s.consent.exportAuthority(ctx, scope, bundle.Manifest)
	if err != nil {
		// The authority read failed: no byte may leave on a doubt.
		return CraftExportBundle{}, err
	}
	if state == craft.ExportConsentNone || state == craft.ExportConsentConsented {
		return bundle, nil
	}
	// Without valid consent no restricted derived byte is returned: the
	// safe fallback bundle carries only the unrestricted members, with its
	// own freshly derived digest, and the withholding is audited.
	safe, err := craft.SafeExportManifest(bundle.Manifest)
	if err != nil {
		return CraftExportBundle{}, err
	}
	withheld := make(map[string]bool, len(restricted))
	for _, path := range restricted {
		withheld[path] = true
	}
	kept := make([]craft.File, 0, len(bundle.Version.Files))
	for _, member := range bundle.Version.Files {
		if !withheld[member.Path] {
			kept = append(kept, member)
		}
	}
	filtered := bundle
	filtered.Version.Files = kept
	filtered.Manifest = safe
	s.consent.auditExportConsent(ctx, scope, bundle.Manifest.VersionID, craftExportDerivedWithheldAction, "denied", map[string]string{
		"withheld": fmt.Sprintf("%d", len(restricted)), "manifest_digest": bundle.Manifest.ManifestDigest,
	})
	return filtered, nil
}
