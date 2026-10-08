// Package service - Craft version-bound source bundle export (T12, #132).
//
// The export service projects one IMMUTABLE version's downloadable source
// bundle: exactly the version's own artifact members (source), the version's
// recorded checks (build metadata) and the citation/source manifest derived
// from the version's pinned evidence (T07). Every input is immutable: the
// Workspace, the current knowledge base and the Run's live record are never
// read, so a historical version's bundle digest stays stable after any later
// edit. Restricted originals are excluded by construction — original
// knowledge bytes never enter a version manifest — and every cited source
// appears only as a durable authenticated reference that resolves solely
// through the viewer's own fresh authorization (T10): the downloaded
// manifest grants no access.
//
// Every download re-checks CURRENT Task membership and audits the member,
// the Version and the manifest digest.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// CraftExportVersionReader reads one immutable version. The production
// adapter is the same version store the Craft session reads.
type CraftExportVersionReader interface {
	Get(context.Context, craft.Scope, string) (craft.Version, error)
}

// CraftExportEvidenceReader reads one version's pinned evidence (T07). A
// version without pinned evidence honestly answers ErrNotFound; the bundle
// is never rebuilt from mutable state.
type CraftExportEvidenceReader interface {
	VersionEvidence(context.Context, craft.Scope, string) (craft.VersionEvidence, error)
}

// CraftExportFileReader opens one stored version object by durable ref.
type CraftExportFileReader interface {
	GetFile(context.Context, string) (io.ReadCloser, error)
}

// CraftExportTitleSource resolves best-effort display titles for the cited
// originals from the CURRENT library rows. Titles are display-only: a
// missing row degrades to an empty title, never to a guess, and titles are
// deliberately outside the export manifest's identity (BuildExportManifest
// does not receive them), so library edits never move a bundle digest.
type CraftExportTitleSource func(ctx context.Context, tenantID uint64, knowledgeIDs []string) (map[string]string, error)

// CraftExportConfig assembles the export service. Every port except Titles
// is required and the service fails closed without it. Member BYTES never
// flow through the service: the HTTP download surface streams them through
// its own CraftExportFileReader keyed by the version's durable refs.
type CraftExportConfig struct {
	DB         *gorm.DB
	Versions   CraftExportVersionReader
	Evidence   CraftExportEvidenceReader
	Titles     CraftExportTitleSource
	TaskAccess craft.TaskAccessChecker
	Now        func() time.Time
}

// CraftExportService owns the version-bound source bundle projection.
type CraftExportService struct {
	db         *gorm.DB
	versions   CraftExportVersionReader
	evidence   CraftExportEvidenceReader
	titles     CraftExportTitleSource
	taskAccess craft.TaskAccessChecker
	now        func() time.Time
}

// NewCraftExportService validates the assembly.
func NewCraftExportService(cfg CraftExportConfig) (*CraftExportService, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("craft: export service requires the database")
	}
	if cfg.Versions == nil || cfg.Evidence == nil || cfg.TaskAccess == nil {
		return nil, fmt.Errorf("craft: export service requires versions, evidence and task access")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CraftExportService{
		db: cfg.DB, versions: cfg.Versions, evidence: cfg.Evidence,
		titles: cfg.Titles, taskAccess: cfg.TaskAccess, now: now,
	}, nil
}

// CraftExportBundle is the complete bundle projection handed to the HTTP
// surface: the immutable version (members + build checks), the export
// manifest (digest included) and the citation/source manifest. The member
// BYTES stream separately through the handler's CraftExportFileReader
// keyed by the version's durable refs.
type CraftExportBundle struct {
	Version          craft.Version
	Manifest         craft.ExportManifest
	CitationManifest craft.BundleCitationManifest
}

// CraftExportCheck is the wire projection of one verification inside
// build.json: snake_case keys, the exact shape the describe endpoint
// projects for the same checks (craft.Check itself carries no json tags).
type CraftExportCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// BundleBuildDocument is the build.json payload riding every bundle: the
// version's kind and its independently recorded checks, projected to the
// shared wire key shape.
type BundleBuildDocument struct {
	VersionID string             `json:"version_id"`
	RunID     string             `json:"run_id"`
	Kind      string             `json:"kind"`
	Checks    []CraftExportCheck `json:"checks"`
}

// The export denial audit actions fold the refusal REASON into the typed
// action column (the T11 discipline): the reason is part of the dedup key
// and the vocabulary is finite and server-controlled.
const (
	craftExportDenyActionTaskAccess     = "craft.export_denied:task_access"
	craftExportDenyActionCallerIdentity = "craft.export_denied:caller_identity"
)

var craftExportDeniedActionVocabulary = map[string]bool{
	craftExportDenyActionTaskAccess:     true,
	craftExportDenyActionCallerIdentity: true,
}

// ExportBundle projects one version's downloadable source bundle for a
// current Task member. The journey:
//
//  1. the version id must be the well-formed persisted identity — a
//     traversal or malformed id is refused before any lookup;
//  2. CURRENT Task membership is re-checked (a revoked member is refused,
//     and the refusal is audited);
//  3. the version and its pinned evidence answer from the immutable stores
//     only — a version without pinned evidence honestly refuses instead of
//     reconstructing history from the Workspace or the live record;
//  4. the member paths are defensively re-validated (a corrupted row
//     refuses the whole bundle before any byte is packaged);
//  5. the manifest digest is computed from immutable facts and the download
//     is audited with the member, the Version and the digest.
//
// Titles resolve best-effort from the current library rows and never enter
// the digest. The bundle grants no original-source access: opening a cited
// ref is a separate, freshly authorized act (T10).
func (s *CraftExportService) ExportBundle(ctx context.Context, scope craft.Scope, versionID string) (CraftExportBundle, error) {
	if s == nil || s.versions == nil || s.evidence == nil {
		return CraftExportBundle{}, craft.ErrForbidden
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return CraftExportBundle{}, fmt.Errorf("%w: incomplete export request", craft.ErrInvalidInput)
	}
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return CraftExportBundle{}, fmt.Errorf("%w: export requires the version id", craft.ErrInvalidInput)
	}
	if !craft.ValidVersionID(versionID) {
		// A malformed id is indistinguishable from a missing version: the
		// refusal is stable and non-leaking, and no lookup happens.
		return CraftExportBundle{}, craft.ErrNotFound
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
		s.auditExport(ctx, scope, versionID, craftExportDenyActionTaskAccess, "denied", nil)
		return CraftExportBundle{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		s.auditExport(ctx, scope, versionID, craftExportDenyActionCallerIdentity, "denied", nil)
		return CraftExportBundle{}, craft.ErrForbidden
	}

	// Immutable inputs only: the version store's own scope ACL already
	// answers a foreign task's version as missing.
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return CraftExportBundle{}, err
	}
	evidence, err := s.evidence.VersionEvidence(ctx, scope, versionID)
	if err != nil {
		// History is never reconstructed: a version promoted without
		// evidence has no provable citation manifest and refuses.
		logger.Warnf(ctx, "[CraftExport] bundle refused for version %s: no pinned evidence: %v", versionID, err)
		return CraftExportBundle{}, err
	}

	// Defensive member validation BEFORE any byte is packaged.
	if err := craft.ValidateExportBundleMembers(version.Files); err != nil {
		logger.Warnf(ctx, "[CraftExport] bundle refused for version %s: %v", versionID, err)
		return CraftExportBundle{}, err
	}

	manifest, err := craft.BuildExportManifest(version, evidence, scope.TenantID)
	if err != nil {
		return CraftExportBundle{}, err
	}
	// Titles resolve by knowledge coordinate; the citation manifest indexes
	// them by the citation identity, so translate here — the module stays a
	// pure function of its inputs.
	titlesByKnowledge := s.resolveTitles(ctx, scope.TenantID, evidence)
	titlesByCitation := map[string]string{}
	for _, source := range evidence.Sources {
		if title, ok := titlesByKnowledge[craftKnowledgeIDOfRef(source.Ref)]; ok {
			titlesByCitation[source.ID] = title
		}
	}
	citation, err := craft.BuildBundleCitationManifest(version, evidence, titlesByCitation)
	if err != nil {
		return CraftExportBundle{}, err
	}

	s.auditExport(ctx, scope, version.ID, "craft.export_bundle", "success", map[string]string{
		"manifest_digest": manifest.ManifestDigest,
		"members":         fmt.Sprintf("%d", len(manifest.Files)),
		"run_id":          version.RunID,
	})
	return CraftExportBundle{Version: version, Manifest: manifest, CitationManifest: citation}, nil
}

// resolveTitles resolves best-effort display titles for the cited originals.
// Titles resolve through the caller's own current read access; a failure or
// a missing row degrades to empty titles and never fails the bundle — the
// pinned identity facts are already complete without them.
func (s *CraftExportService) resolveTitles(ctx context.Context, tenantID uint64, evidence craft.VersionEvidence) map[string]string {
	if s.titles == nil || len(evidence.Sources) == 0 {
		return nil
	}
	ids := make([]string, 0, len(evidence.Sources))
	seen := map[string]bool{}
	for _, source := range evidence.Sources {
		id := craftKnowledgeIDOfRef(source.Ref)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	titles, err := s.titles(ctx, tenantID, ids)
	if err != nil {
		logger.Warnf(ctx, "[CraftExport] title resolution degraded for %d sources: %v", len(ids), err)
		return nil
	}
	return titles
}

// BundleBuildDocument projects the bundle's build.json payload (checks in
// the shared snake_case wire shape).
func (b CraftExportBundle) BundleBuildDocument() BundleBuildDocument {
	checks := make([]CraftExportCheck, 0, len(b.Version.Checks))
	for _, check := range b.Version.Checks {
		checks = append(checks, CraftExportCheck{Name: check.Name, Status: check.Status, Detail: check.Detail})
	}
	return BundleBuildDocument{VersionID: b.Version.ID, RunID: b.Version.RunID, Kind: b.Version.Kind, Checks: checks}
}

// auditExport records a durable export event into the shared audit trail
// with the T11 discipline: DENIAL rows carry the reason in the typed action
// and dedupe inside the shared sliding window; the write failure is logged,
// never propagated — authority was already decided.
func (s *CraftExportService) auditExport(ctx context.Context, scope craft.Scope, versionID, action, outcome string, details map[string]string) {
	if outcome == "denied" && !craftExportDeniedActionVocabulary[action] {
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
